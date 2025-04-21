// Copyright 2014 The go-zenanet Authors
// This file is part of the go-zenanet library.
//
// The go-zenanet library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-zenanet library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-zenanet library. If not, see <http://www.gnu.org/licenses/>.

// Package eth implements the Zenanet protocol.
package eth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/accounts"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/common/hexutil"
	"github.com/zenanetwork/go-zenanet/consensus"
	"github.com/zenanetwork/go-zenanet/consensus/beacon"
	"github.com/zenanetwork/go-zenanet/consensus/clique"
	"github.com/zenanetwork/go-zenanet/consensus/zena"
	"github.com/zenanetwork/go-zenanet/consensus/zena/iris"
	"github.com/zenanetwork/go-zenanet/core"
	"github.com/zenanetwork/go-zenanet/core/bloombits"
	"github.com/zenanetwork/go-zenanet/core/rawdb"
	"github.com/zenanetwork/go-zenanet/core/state/pruner"
	"github.com/zenanetwork/go-zenanet/core/txpool"
	"github.com/zenanetwork/go-zenanet/core/txpool/legacypool"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/core/vm"
	"github.com/zenanetwork/go-zenanet/eth/downloader"
	"github.com/zenanetwork/go-zenanet/eth/downloader/whitelist"
	"github.com/zenanetwork/go-zenanet/eth/ethconfig"
	"github.com/zenanetwork/go-zenanet/eth/filters"
	"github.com/zenanetwork/go-zenanet/eth/gasprice"
	"github.com/zenanetwork/go-zenanet/eth/protocols/eth"
	"github.com/zenanetwork/go-zenanet/eth/protocols/snap"
	"github.com/zenanetwork/go-zenanet/eth/tracers"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/event"
	"github.com/zenanetwork/go-zenanet/internal/ethapi"
	"github.com/zenanetwork/go-zenanet/internal/shutdowncheck"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/miner"
	"github.com/zenanetwork/go-zenanet/node"
	"github.com/zenanetwork/go-zenanet/p2p"
	"github.com/zenanetwork/go-zenanet/p2p/dnsdisc"
	"github.com/zenanetwork/go-zenanet/p2p/enode"
	"github.com/zenanetwork/go-zenanet/params"
	"github.com/zenanetwork/go-zenanet/rlp"
	"github.com/zenanetwork/go-zenanet/rpc"
	"github.com/zenanetwork/go-zenanet/triedb"
)

// Config contains the configuration options of the ETH protocol.
// Deprecated: use ethconfig.Config instead.
type Config = ethconfig.Config

// Zenanet implements the Zenanet full node service.
type Zenanet struct {
	config *ethconfig.Config

	// Handlers
	txPool *txpool.TxPool

	blockchain         *core.BlockChain
	handler            *handler
	ethDialCandidates  enode.Iterator
	snapDialCandidates enode.Iterator

	// DB interfaces
	chainDb ethdb.Database // Block chain database

	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager
	authorized     bool // If consensus engine is authorized with keystore

	bloomRequests     chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer      *core.ChainIndexer             // Bloom indexer operating during block imports
	closeBloomHandler chan struct{}

	APIBackend *EthAPIBackend

	miner    *miner.Miner
	gasPrice *big.Int
	zenbase  common.Address

	networkID     uint64
	netRPCService *ethapi.NetAPI

	p2pServer *p2p.Server

	lock sync.RWMutex // Protects the variadic fields (e.g. gas price and zenbase)

	closeCh chan struct{} // Channel to signal the background processes to exit

	shutdownTracker *shutdowncheck.ShutdownTracker // Tracks if and when the node has shutdown ungracefully
}

// New creates a new Zenanet object (including the initialisation of the common Zenanet object),
// whose lifecycle will be managed by the provided node.
func New(stack *node.Node, config *ethconfig.Config) (*Zenanet, error) {
	// Ensure configuration values are compatible and sane
	if !config.SyncMode.IsValid() {
		return nil, fmt.Errorf("invalid sync mode %d", config.SyncMode)
	}

	// PIP-35: Enforce min gas price to 25 gwei
	if config.Miner.GasPrice == nil || config.Miner.GasPrice.Cmp(big.NewInt(params.ZenaDefaultMinerGasPrice)) != 0 {
		log.Warn("Sanitizing invalid miner gas price", "provided", config.Miner.GasPrice, "updated", ethconfig.Defaults.Miner.GasPrice)
		config.Miner.GasPrice = ethconfig.Defaults.Miner.GasPrice
	}

	if config.NoPruning && config.TrieDirtyCache > 0 {
		if config.SnapshotCache > 0 {
			config.TrieCleanCache += config.TrieDirtyCache * 3 / 5
			config.SnapshotCache += config.TrieDirtyCache * 2 / 5
		} else {
			config.TrieCleanCache += config.TrieDirtyCache
		}
		config.TrieDirtyCache = 0
	}
	log.Info("Allocated trie memory caches", "clean", common.StorageSize(config.TrieCleanCache)*1024*1024, "dirty", common.StorageSize(config.TrieDirtyCache)*1024*1024)

	// Assemble the Zenanet object
	chainDb, err := stack.OpenDatabaseWithFreezer("chaindata", config.DatabaseCache, config.DatabaseHandles, config.DatabaseFreezer, "zenanet/db/chaindata/", false, false, false)
	if err != nil {
		return nil, err
	}
	scheme, err := rawdb.ParseStateScheme(config.StateScheme, chainDb)
	if err != nil {
		return nil, err
	}
	// Try to recover offline state pruning only in hash-based.
	if scheme == rawdb.HashScheme {
		if err := pruner.RecoverPruning(stack.ResolvePath(""), chainDb); err != nil {
			log.Error("Failed to recover state", "error", err)
		}
	}

	// START: Zena changes
	eth := &Zenanet{
		config:            config,
		chainDb:           chainDb,
		eventMux:          stack.EventMux(),
		accountManager:    stack.AccountManager(),
		authorized:        false,
		closeBloomHandler: make(chan struct{}),
		networkID:         config.NetworkId,
		gasPrice:          config.Miner.GasPrice,
		zenbase:           config.Miner.Zenbase,
		bloomRequests:     make(chan chan *bloombits.Retrieval),
		bloomIndexer:      core.NewBloomIndexer(chainDb, params.BloomBitsBlocks, params.BloomConfirms),
		p2pServer:         stack.Server(),
		shutdownTracker:   shutdowncheck.NewShutdownTracker(chainDb),
		closeCh:           make(chan struct{}),
	}

	eth.APIBackend = &EthAPIBackend{stack.Config().ExtRPCEnabled(), stack.Config().AllowUnprotectedTxs, eth, nil}
	if eth.APIBackend.allowUnprotectedTxs {
		log.Info("------Unprotected transactions allowed-------")
		config.TxPool.AllowUnprotectedTxs = true
	}

	gpoParams := config.GPO

	// Override the chain config with provided settings.
	var overrides core.ChainOverrides
	if config.OverrideCancun != nil {
		overrides.OverrideCancun = config.OverrideCancun
	}
	if config.OverrideVerkle != nil {
		overrides.OverrideVerkle = config.OverrideVerkle
	}

	chainConfig, _, genesisErr := core.SetupGenesisBlockWithOverride(chainDb, triedb.NewDatabase(chainDb, triedb.HashDefaults), config.Genesis, &overrides)
	if _, isCompat := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !isCompat {
		return nil, genesisErr
	}

	blockChainAPI := ethapi.NewBlockChainAPI(eth.APIBackend)
	engine, err := ethconfig.CreateConsensusEngine(chainConfig, config, chainDb, blockChainAPI)
	eth.engine = engine
	if err != nil {
		return nil, err
	}
	// END: Zena changes

	bcVersion := rawdb.ReadDatabaseVersion(chainDb)
	var dbVer = "<nil>"
	if bcVersion != nil {
		dbVer = fmt.Sprintf("%d", *bcVersion)
	}
	log.Info("Initialising Zenanet protocol", "network", config.NetworkId, "dbversion", dbVer)

	if !config.SkipBcVersionCheck {
		if bcVersion != nil && *bcVersion > core.BlockChainVersion {
			return nil, fmt.Errorf("database version is v%d, Gzen %s only supports v%d", *bcVersion, params.VersionWithMeta, core.BlockChainVersion)
		} else if bcVersion == nil || *bcVersion < core.BlockChainVersion {
			if bcVersion != nil { // only print warning on upgrade, not on init
				log.Warn("Upgrade blockchain database version", "from", dbVer, "to", core.BlockChainVersion)
			}
			rawdb.WriteDatabaseVersion(chainDb, core.BlockChainVersion)
		}
	}
	var (
		vmConfig = vm.Config{
			EnablePreimageRecording: config.EnablePreimageRecording,
			EnableWitnessCollection: config.EnableWitnessCollection,
		}
		cacheConfig = &core.CacheConfig{
			TrieCleanLimit:      config.TrieCleanCache,
			TrieCleanNoPrefetch: config.NoPrefetch,
			TrieDirtyLimit:      config.TrieDirtyCache,
			TrieDirtyDisabled:   config.NoPruning,
			TrieTimeLimit:       config.TrieTimeout,
			SnapshotLimit:       config.SnapshotCache,
			Preimages:           config.Preimages,
			StateHistory:        config.StateHistory,
			StateScheme:         scheme,
			TriesInMemory:       config.TriesInMemory,
		}
	)

	if config.VMTrace != "" {
		var traceConfig json.RawMessage
		if config.VMTraceJsonConfig != "" {
			traceConfig = json.RawMessage(config.VMTraceJsonConfig)
		}
		t, err := tracers.LiveDirectory.New(config.VMTrace, traceConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create tracer %s: %v", config.VMTrace, err)
		}
		vmConfig.Tracer = t
	}

	checker := whitelist.NewService(chainDb)

	// check if Parallel EVM is enabled
	// if enabled, use parallel state processor
	if config.ParallelEVM.Enable {
		eth.blockchain, err = core.NewParallelBlockChain(chainDb, cacheConfig, config.Genesis, &overrides, eth.engine, vmConfig, eth.shouldPreserve, &config.TxLookupLimit, checker, config.ParallelEVM.SpeculativeProcesses, config.ParallelEVM.Enforce)
	} else {
		eth.blockchain, err = core.NewBlockChain(chainDb, cacheConfig, config.Genesis, &overrides, eth.engine, vmConfig, eth.shouldPreserve, &config.TxLookupLimit, checker)
	}

	// 1.14.8: NewOracle function definition was changed to accept (startPrice *big.Int) param.
	eth.APIBackend.gpo = gasprice.NewOracle(eth.APIBackend, gpoParams, config.Miner.GasPrice)
	if err != nil {
		return nil, err
	}

	_ = eth.engine.VerifyHeader(eth.blockchain, eth.blockchain.CurrentHeader()) // TODO think on it

	// ZENA changes
	eth.APIBackend.gpo.ProcessCache()
	// ZENA changes

	eth.bloomIndexer.Start(eth.blockchain)

	if config.BlobPool.Datadir != "" {
		config.BlobPool.Datadir = stack.ResolvePath(config.BlobPool.Datadir)
	}

	if config.TxPool.Journal != "" {
		config.TxPool.Journal = stack.ResolvePath(config.TxPool.Journal)
	}
	legacyPool := legacypool.New(config.TxPool, eth.blockchain)

	// ZENA changes
	// Blob pool is removed from Subpool for Zena
	eth.txPool, err = txpool.New(config.TxPool.PriceLimit, eth.blockchain, []txpool.SubPool{legacyPool})
	if err != nil {
		return nil, err
	}

	// The `config.TxPool.PriceLimit` used above doesn't reflect the sanitized/enforced changes
	// made in the txpool. Update the `gasTip` explicitly to reflect the enforced value.
	eth.txPool.SetGasTip(new(big.Int).SetUint64(params.ZenaDefaultTxPoolPriceLimit))

	// Permit the downloader to use the trie cache allowance during fast sync
	cacheLimit := cacheConfig.TrieCleanLimit + cacheConfig.TrieDirtyLimit + cacheConfig.SnapshotLimit
	if eth.handler, err = newHandler(&handlerConfig{
		Database:            chainDb,
		Chain:               eth.blockchain,
		TxPool:              eth.txPool,
		Network:             config.NetworkId,
		Sync:                config.SyncMode,
		BloomCache:          uint64(cacheLimit),
		EventMux:            eth.eventMux,
		RequiredBlocks:      config.RequiredBlocks,
		EthAPI:              blockChainAPI,
		checker:             checker,
		enableBlockTracking: eth.config.EnableBlockTracking,
	}); err != nil {
		return nil, err
	}

	eth.miner = miner.New(eth, &config.Miner, eth.blockchain.Config(), eth.EventMux(), eth.engine, eth.isLocalBlock)
	eth.miner.SetExtra(makeExtraData(config.Miner.ExtraData))

	eth.APIBackend = &EthAPIBackend{stack.Config().ExtRPCEnabled(), stack.Config().AllowUnprotectedTxs, eth, nil}
	if eth.APIBackend.allowUnprotectedTxs {
		log.Info("Unprotected transactions allowed")
	}
	// 1.14.8: NewOracle function definition was changed to accept (startPrice *big.Int) param.
	eth.APIBackend.gpo = gasprice.NewOracle(eth.APIBackend, config.GPO, config.Miner.GasPrice)

	// Setup DNS discovery iterators.
	dnsclient := dnsdisc.NewClient(dnsdisc.Config{})
	eth.ethDialCandidates, err = dnsclient.NewIterator(eth.config.EthDiscoveryURLs...)
	if err != nil {
		return nil, err
	}
	eth.snapDialCandidates, err = dnsclient.NewIterator(eth.config.SnapDiscoveryURLs...)
	if err != nil {
		return nil, err
	}

	// Start the RPC service
	eth.netRPCService = ethapi.NewNetAPI(eth.p2pServer, config.NetworkId)

	// Register the backend on the node
	stack.RegisterAPIs(eth.APIs())
	stack.RegisterProtocols(eth.Protocols())
	stack.RegisterLifecycle(eth)

	// Successful startup; push a marker and check previous unclean shutdowns.
	eth.shutdownTracker.MarkStartup()

	return eth, nil
}

func makeExtraData(extra []byte) []byte {
	if len(extra) == 0 {
		// create default extradata
		extra, _ = rlp.EncodeToBytes([]interface{}{
			uint(params.VersionMajor<<16 | params.VersionMinor<<8 | params.VersionPatch),
			"zena",
			runtime.Version(),
			runtime.GOOS,
		})
	}

	if uint64(len(extra)) > params.MaximumExtraDataSize {
		log.Warn("Miner extra data exceed limit", "extra", hexutil.Bytes(extra), "limit", params.MaximumExtraDataSize)
		extra = nil
	}

	return extra
}

// PeerCount returns the number of connected peers.
func (s *Zenanet) PeerCount() int {
	return s.p2pServer.PeerCount()
}

// APIs return the collection of RPC services the zenanet package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *Zenanet) APIs() []rpc.API {
	apis := ethapi.GetAPIs(s.APIBackend)

	// Append any APIs exposed explicitly by the consensus engine
	apis = append(apis, s.engine.APIs(s.BlockChain())...)

	// ZENA change starts
	filterSystem := filters.NewFilterSystem(s.APIBackend, filters.Config{})
	// set genesis to public filter api
	publicFilterAPI := filters.NewFilterAPI(filterSystem, s.config.ZenaLogs)
	// avoiding constructor changed by introducing new method to set genesis
	publicFilterAPI.SetChainConfig(s.blockchain.Config())
	// ZENA change ends

	// Append all the local APIs and return
	return append(apis, []rpc.API{
		{
			Namespace: "miner",
			Service:   NewMinerAPI(s),
		}, {
			Namespace: "eth",
			Service:   publicFilterAPI, // ZENA related change
		}, {
			Namespace: "admin",
			Service:   NewAdminAPI(s),
		}, {
			Namespace: "debug",
			Service:   NewDebugAPI(s),
		}, {
			Namespace: "net",
			Service:   s.netRPCService,
		},
	}...)
}

func (s *Zenanet) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *Zenanet) PublicBlockChainAPI() *ethapi.BlockChainAPI {
	return s.handler.ethAPI
}

func (s *Zenanet) Zenbase() (eb common.Address, err error) {
	s.lock.RLock()
	zenbase := s.zenbase
	s.lock.RUnlock()

	if zenbase != (common.Address{}) {
		return zenbase, nil
	}
	return common.Address{}, errors.New("zenbase must be explicitly specified")
}

// isLocalBlock checks whether the specified block is mined
// by local miner accounts.
//
// We regard two types of accounts as local miner account: zenbase
// and accounts specified via `txpool.locals` flag.
func (s *Zenanet) isLocalBlock(header *types.Header) bool {
	author, err := s.engine.Author(header)
	if err != nil {
		log.Warn("Failed to retrieve block author", "number", header.Number.Uint64(), "hash", header.Hash(), "err", err)
		return false
	}
	// Check whether the given address is zenbase.
	s.lock.RLock()
	zenbase := s.zenbase
	s.lock.RUnlock()

	if author == zenbase {
		return true
	}
	// Check whether the given address is specified by `txpool.local`
	// CLI flag.
	for _, account := range s.config.TxPool.Locals {
		if account == author {
			return true
		}
	}

	return false
}

// shouldPreserve checks whether we should preserve the given block
// during the chain reorg depending on whether the author of block
// is a local account.
func (s *Zenanet) shouldPreserve(header *types.Header) bool {
	// The reason we need to disable the self-reorg preserving for clique
	// is it can be probable to introduce a deadlock.
	//
	// e.g. If there are 7 available signers
	//
	// r1   A
	// r2     B
	// r3       C
	// r4         D
	// r5   A      [X] F G
	// r6    [X]
	//
	// In the round5, the in-turn signer E is offline, so the worst case
	// is A, F and G sign the block of round5 and reject the block of opponents
	// and in the round6, the last available signer B is offline, the whole
	// network is stuck.
	if _, ok := s.engine.(*clique.Clique); ok {
		return false
	}

	return s.isLocalBlock(header)
}

// SetZenbase sets the mining reward address.
func (s *Zenanet) SetZenbase(zenbase common.Address) {
	s.lock.Lock()
	s.zenbase = zenbase
	s.lock.Unlock()

	s.miner.SetZenbase(zenbase)
}

// StartMining starts the miner with the given number of CPU threads. If mining
// is already running, this method adjust the number of threads allowed to use
// and updates the minimum price required by the transaction pool.
func (s *Zenanet) StartMining() error {
	// If the miner was not running, initialize it
	if !s.IsMining() {
		// Propagate the initial price point to the transaction pool
		s.lock.RLock()
		price := s.gasPrice
		s.lock.RUnlock()
		s.txPool.SetGasTip(price)

		// Configure the local mining address
		eb, err := s.Zenbase()
		if err != nil {
			log.Error("Cannot start mining without zenbase", "err", err)
			return fmt.Errorf("zenbase missing: %v", err)
		}
		// If personal endpoints are disabled, the server creating
		// this Zenanet instance has already Authorized consensus.
		if !s.authorized {
			var cli *clique.Clique
			if c, ok := s.engine.(*clique.Clique); ok {
				cli = c
			} else if cl, ok := s.engine.(*beacon.Beacon); ok {
				if c, ok := cl.InnerEngine().(*clique.Clique); ok {
					cli = c
				}
			}

			if cli != nil {
				wallet, err := s.accountManager.Find(accounts.Account{Address: eb})
				if wallet == nil || err != nil {
					log.Error("Zenbase account unavailable locally", "err", err)
					return fmt.Errorf("signer missing: %v", err)
				}

				cli.Authorize(eb, wallet.SignData)
			}

			if zena, ok := s.engine.(*zena.Zena); ok {
				wallet, err := s.accountManager.Find(accounts.Account{Address: eb})
				if wallet == nil || err != nil {
					log.Error("Zenbase account unavailable locally", "err", err)

					return fmt.Errorf("signer missing: %v", err)
				}

				zena.Authorize(eb, wallet.SignData)
			}
		}

		// If mining is started, we can disable the transaction rejection mechanism
		// introduced to speed sync times.
		s.handler.enableSyncedFeatures()

		go s.miner.Start()
	}

	return nil
}

// StopMining terminates the miner, both at the consensus engine level as well as
// at the block creation level.
func (s *Zenanet) StopMining() {
	// Update the thread count within the consensus engine
	type threaded interface {
		SetThreads(threads int)
	}

	if th, ok := s.engine.(threaded); ok {
		th.SetThreads(-1)
	}
	// Stop the block creating itself
	ch := make(chan struct{})
	s.miner.Stop(ch)
}

func (s *Zenanet) IsMining() bool      { return s.miner.Mining() }
func (s *Zenanet) Miner() *miner.Miner { return s.miner }

func (s *Zenanet) AccountManager() *accounts.Manager { return s.accountManager }
func (s *Zenanet) BlockChain() *core.BlockChain      { return s.blockchain }
func (s *Zenanet) TxPool() *txpool.TxPool            { return s.txPool }
func (s *Zenanet) EventMux() *event.TypeMux          { return s.eventMux }
func (s *Zenanet) Engine() consensus.Engine          { return s.engine }
func (s *Zenanet) ChainDb() ethdb.Database {
	return s.chainDb
}
func (s *Zenanet) IsListening() bool                  { return true } // Always listening
func (s *Zenanet) Downloader() *downloader.Downloader { return s.handler.downloader }
func (s *Zenanet) Synced() bool                       { return s.handler.synced.Load() }
func (s *Zenanet) SetSynced()                         { s.handler.enableSyncedFeatures() }
func (s *Zenanet) ArchiveMode() bool                  { return s.config.NoPruning }
func (s *Zenanet) BloomIndexer() *core.ChainIndexer   { return s.bloomIndexer }

// SetAuthorized sets the authorized bool variable
// denoting that consensus has been authorized while creation
func (s *Zenanet) SetAuthorized(authorized bool) {
	s.lock.Lock()
	s.authorized = authorized
	s.lock.Unlock()
}

// Protocols returns all the currently configured
// network protocols to start.
func (s *Zenanet) Protocols() []p2p.Protocol {
	protos := eth.MakeProtocols((*ethHandler)(s.handler), s.networkID, s.ethDialCandidates)
	if s.config.SnapshotCache > 0 {
		protos = append(protos, snap.MakeProtocols((*snapHandler)(s.handler), s.snapDialCandidates)...)
	}

	return protos
}

// Start implements node.Lifecycle, starting all internal goroutines needed by the
// Zenanet protocol implementation.
func (s *Zenanet) Start() error {
	eth.StartENRUpdater(s.blockchain, s.p2pServer.LocalNode())

	// Start the bloom bits servicing goroutines
	s.startBloomHandlers(params.BloomBitsBlocks)

	// Regularly update shutdown marker
	s.shutdownTracker.Start()

	// Figure out a max peers count based on the server limits
	maxPeers := s.p2pServer.MaxPeers

	if s.config.LightServ > 0 {
		if s.config.LightPeers >= s.p2pServer.MaxPeers {
			return fmt.Errorf("invalid peer config: light peer count (%d) >= total peer count (%d)", s.config.LightPeers, s.p2pServer.MaxPeers)
		}

		maxPeers -= s.config.LightPeers
	}

	// Start the networking layer and the light server if requested
	s.handler.Start(maxPeers)

	go s.startCheckpointWhitelistService()
	go s.startMilestoneWhitelistService()
	go s.startNoAckMilestoneService()
	go s.startNoAckMilestoneByIDService()

	return nil
}

var (
	ErrNotZenaConsensus         = errors.New("not zena consensus was given")
	ErrZenaConsensusWithoutIris = errors.New("zena consensus without iris")
)

const (
	whitelistTimeout      = 30 * time.Second
	noAckMilestoneTimeout = 4 * time.Second
)

// StartCheckpointWhitelistService starts the goroutine to fetch checkpoints and update the
// checkpoint whitelist map.
func (s *Zenanet) startCheckpointWhitelistService() {
	const (
		tickerDuration = 100 * time.Second
		fnName         = "whitelist checkpoint"
	)

	s.retryIrisHandler(s.handleWhitelistCheckpoint, tickerDuration, whitelistTimeout, fnName)
}

// startMilestoneWhitelistService starts the goroutine to fetch milestiones and update the
// milestone whitelist map.
func (s *Zenanet) startMilestoneWhitelistService() {
	const (
		tickerDuration = 12 * time.Second
		fnName         = "whitelist milestone"
	)

	s.retryIrisHandler(s.handleMilestone, tickerDuration, whitelistTimeout, fnName)
}

func (s *Zenanet) startNoAckMilestoneService() {
	const (
		tickerDuration = 6 * time.Second
		fnName         = "no-ack-milestone service"
	)

	s.retryIrisHandler(s.handleNoAckMilestone, tickerDuration, noAckMilestoneTimeout, fnName)
}

func (s *Zenanet) startNoAckMilestoneByIDService() {
	const (
		tickerDuration = 1 * time.Minute
		fnName         = "no-ack-milestone-by-id service"
	)

	s.retryIrisHandler(s.handleNoAckMilestoneByID, tickerDuration, noAckMilestoneTimeout, fnName)
}

func (s *Zenanet) retryIrisHandler(fn irisHandler, tickerDuration time.Duration, timeout time.Duration, fnName string) {
	retryIrisHandler(fn, tickerDuration, timeout, fnName, s.closeCh, s.getHandler)
}

func retryIrisHandler(fn irisHandler, tickerDuration time.Duration, timeout time.Duration, fnName string, closeCh chan struct{}, getHandler func() (*ethHandler, *zena.Zena, error)) {
	// a shortcut helps with tests and early exit
	select {
	case <-closeCh:
		return
	default:
	}

	ethHandler, zena, err := getHandler()
	if err != nil {
		log.Error("error while getting the ethHandler", "err", err)
		return
	}

	// first run
	firstCtx, cancel := context.WithTimeout(context.Background(), timeout)
	_ = fn(firstCtx, ethHandler, zena)

	cancel()

	ticker := time.NewTicker(tickerDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), timeout)

			// Skip any error reporting here as it's handled in respective functions
			_ = fn(ctx, ethHandler, zena)

			cancel()
		case <-closeCh:
			return
		}
	}
}

// handleWhitelistCheckpoint handles the checkpoint whitelist mechanism.
func (s *Zenanet) handleWhitelistCheckpoint(ctx context.Context, ethHandler *ethHandler, zena *zena.Zena) error {
	// Create a new zena verifier, which will be used to verify checkpoints and milestones
	verifier := newZenaVerifier()

	blockNum, blockHash, err := ethHandler.fetchWhitelistCheckpoint(ctx, zena, s, verifier)
	// If the array is empty, we're bound to receive an error. Non-nill error and non-empty array
	// means that array has partial elements and it failed for some block. We'll add those partial
	// elements anyway.
	if err != nil {
		return err
	}

	ethHandler.downloader.ProcessCheckpoint(blockNum, blockHash)

	return nil
}

type irisHandler func(ctx context.Context, ethHandler *ethHandler, zena *zena.Zena) error

// handleMilestone handles the milestone mechanism.
func (s *Zenanet) handleMilestone(ctx context.Context, ethHandler *ethHandler, zena *zena.Zena) error {
	// Create a new zena verifier, which will be used to verify checkpoints and milestones
	verifier := newZenaVerifier()
	num, hash, err := ethHandler.fetchWhitelistMilestone(ctx, zena, s, verifier)

	// If the current chain head is behind the received milestone, add it to the future milestone
	// list. Also, the hash mismatch (end block hash) error will lead to rewind so also
	// add that milestone to the future milestone list.
	if errors.Is(err, errChainOutOfSync) || errors.Is(err, errHashMismatch) {
		ethHandler.downloader.ProcessFutureMilestone(num, hash)
	}

	if errors.Is(err, iris.ErrServiceUnavailable) {
		return nil
	}

	if err != nil {
		return err
	}

	ethHandler.downloader.ProcessMilestone(num, hash)

	return nil
}

func (s *Zenanet) handleNoAckMilestone(ctx context.Context, ethHandler *ethHandler, zena *zena.Zena) error {
	milestoneID, err := ethHandler.fetchNoAckMilestone(ctx, zena)

	if errors.Is(err, iris.ErrServiceUnavailable) {
		return nil
	}

	if err != nil {
		return err
	}

	ethHandler.downloader.RemoveMilestoneID(milestoneID)

	return nil
}

func (s *Zenanet) handleNoAckMilestoneByID(ctx context.Context, ethHandler *ethHandler, zena *zena.Zena) error {
	milestoneIDs := ethHandler.downloader.GetMilestoneIDsList()

	for _, milestoneID := range milestoneIDs {
		// todo: check if we can ignore the error
		err := ethHandler.fetchNoAckMilestoneByID(ctx, zena, milestoneID)
		if err == nil {
			ethHandler.downloader.RemoveMilestoneID(milestoneID)
		}
	}

	return nil
}

func (s *Zenanet) getHandler() (*ethHandler, *zena.Zena, error) {
	ethHandler := (*ethHandler)(s.handler)

	zena, ok := ethHandler.chain.Engine().(*zena.Zena)
	if !ok {
		return nil, nil, ErrNotZenaConsensus
	}

	if zena.IrisClient == nil {
		return nil, nil, ErrZenaConsensusWithoutIris
	}

	return ethHandler, zena, nil
}

// Stop implements node.Lifecycle, terminating all internal goroutines used by the
// Zenanet protocol.
func (s *Zenanet) Stop() error {
	// Stop all the peer-related stuff first.
	s.ethDialCandidates.Close()
	s.snapDialCandidates.Close()

	// Close the engine before handler else it may cause a deadlock where
	// the iris is unresponsive and the syncing loop keeps waiting
	// for a response and is unable to proceed to exit `Finalize` during
	// block processing.
	s.engine.Close()
	s.handler.Stop()

	// Then stop everything else.
	s.bloomIndexer.Close()
	close(s.closeBloomHandler)

	// Close all bg processes
	close(s.closeCh)

	s.txPool.Close()
	s.miner.Close()
	s.blockchain.Stop()

	// Clean shutdown marker as the last thing before closing db
	s.shutdownTracker.Stop()

	s.chainDb.Close()
	s.eventMux.Stop()

	return nil
}

//
// Zena related methods
//

// SetBlockchain set blockchain while testing
func (s *Zenanet) SetBlockchain(blockchain *core.BlockChain) {
	s.blockchain = blockchain
}

// SyncMode retrieves the current sync mode, either explicitly set, or derived
// from the chain status.
func (s *Zenanet) SyncMode() downloader.SyncMode {
	// If we're in snap sync mode, return that directly
	if s.handler.snapSync.Load() {
		return downloader.SnapSync
	}
	// We are probably in full sync, but we might have rewound to before the
	// snap sync pivot, check if we should re-enable snap sync.
	head := s.blockchain.CurrentBlock()
	if pivot := rawdb.ReadLastPivotNumber(s.chainDb); pivot != nil {
		if head.Number.Uint64() < *pivot {
			return downloader.SnapSync
		}
	}
	// We are in a full sync, but the associated head state is missing. To complete
	// the head state, forcefully rerun the snap sync. Note it doesn't mean the
	// persistent state is corrupted, just mismatch with the head block.
	if !s.blockchain.HasState(head.Root) {
		log.Info("Reenabled snap sync as chain is stateless")
		return downloader.SnapSync
	}
	// Nope, we're really full syncing
	return downloader.FullSync
}
