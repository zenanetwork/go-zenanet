//go:build integration

package zena

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"sort"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"github.com/zenanetwork/go-zenanet/accounts"
	"github.com/zenanetwork/go-zenanet/accounts/keystore"
	"github.com/zenanetwork/go-zenanet/cmd/utils"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus"
	"github.com/zenanetwork/go-zenanet/consensus/misc/eip1559"
	"github.com/zenanetwork/go-zenanet/consensus/zena" //nolint:typecheck
	"github.com/zenanetwork/go-zenanet/consensus/zena/clerk"
	"github.com/zenanetwork/go-zenanet/consensus/zena/iris/span"
	"github.com/zenanetwork/go-zenanet/consensus/zena/valset"
	"github.com/zenanetwork/go-zenanet/core"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/txpool/legacypool"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/core/vm"
	"github.com/zenanetwork/go-zenanet/crypto"
	"github.com/zenanetwork/go-zenanet/crypto/secp256k1"
	"github.com/zenanetwork/go-zenanet/eth"
	"github.com/zenanetwork/go-zenanet/eth/downloader"
	"github.com/zenanetwork/go-zenanet/eth/ethconfig"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/miner"
	"github.com/zenanetwork/go-zenanet/node"
	"github.com/zenanetwork/go-zenanet/p2p"
	"github.com/zenanetwork/go-zenanet/p2p/enode"
	"github.com/zenanetwork/go-zenanet/params"
	"github.com/zenanetwork/go-zenanet/tests/zena/mocks"
	"github.com/zenanetwork/go-zenanet/triedb"
	"github.com/zenanetwork/zenaproto/iris"
)

var (

	// Only this account is a validator for the 0th span
	key, _ = crypto.HexToECDSA(privKey)
	addr   = crypto.PubkeyToAddress(key.PublicKey) // 0x71562b71999873DB5b286dF957af199Ec94617F7

	// This account is one the validators for 1st span (0-indexed)
	key2, _ = crypto.HexToECDSA(privKey2)
	addr2   = crypto.PubkeyToAddress(key2.PublicKey) // 0x9fB29AAc15b9A4B7F17c3385939b007540f4d791

	keys = []*ecdsa.PrivateKey{key, key2}
)

const (
	privKey  = "b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291"
	privKey2 = "9b28f36fbd67381120752d6172ecdcf10e06ab2d9a1367aac00cdcd6ac7855d3"

	// The genesis for tests was generated with following parameters
	extraSeal = 65 // Fixed number of extra-data suffix bytes reserved for signer seal

	sprintSize uint64 = 4
	spanSize   uint64 = 8

	validatorHeaderBytesLength = common.AddressLength + 20 // address + power
)

type initializeData struct {
	genesis *core.Genesis
	zenanet *eth.Zenanet
}

func setupMiner(t *testing.T, n int, genesis *core.Genesis) ([]*node.Node, []*eth.Zenanet, []*enode.Node) {
	t.Helper()

	// Create an Ethash network based off of the Ropsten config
	var (
		stacks []*node.Node
		nodes  []*eth.Zenanet
		enodes []*enode.Node
	)

	for i := 0; i < n; i++ {
		// Start the node and wait until it's up
		stack, ethBackend, err := InitMiner(genesis, keys[i], true)
		if err != nil {
			t.Fatal("Error occurred while initialising miner", "error", err)
		}

		for stack.Server().NodeInfo().Ports.Listener == 0 {
			time.Sleep(250 * time.Millisecond)
		}
		// Connect the node to all the previous ones
		for _, n := range enodes {
			stack.Server().AddPeer(n)
		}
		// Start tracking the node and its enode
		stacks = append(stacks, stack)
		nodes = append(nodes, ethBackend)
		enodes = append(enodes, stack.Server().Self())
	}

	return stacks, nodes, enodes
}

func buildZenanetInstance(t *testing.T, db ethdb.Database) *initializeData {
	genesisData, err := ioutil.ReadFile("./testdata/genesis.json")
	if err != nil {
		t.Fatalf("%s", err)
	}

	gen := &core.Genesis{}

	if err := json.Unmarshal(genesisData, gen); err != nil {
		t.Fatalf("%s", err)
	}

	ethConf := &eth.Config{
		Genesis:     gen,
		ZenaLogs:    true,
		StateScheme: "hash",
	}

	ethConf.Genesis.MustCommit(db, triedb.NewDatabase(db, triedb.HashDefaults))

	zenanet := utils.CreateZenaZenanet(ethConf)
	if err != nil {
		t.Fatalf("failed to register Zenanet protocol: %v", err)
	}

	ethConf.Genesis.MustCommit(zenanet.ChainDb(), triedb.NewDatabase(zenanet.ChainDb(), triedb.HashDefaults))

	zenanet.Engine().(*zena.Zena).Authorize(addr, func(account accounts.Account, s string, data []byte) ([]byte, error) {
		return crypto.Sign(crypto.Keccak256(data), key)
	})

	return &initializeData{
		genesis: gen,
		zenanet: zenanet,
	}
}

func insertNewBlock(t *testing.T, chain *core.BlockChain, block *types.Block) {
	t.Helper()

	if _, err := chain.InsertChain([]*types.Block{block}); err != nil {
		t.Fatalf("%s", err)
	}
}

type Option func(header *types.Header)

func buildNextBlock(t *testing.T, _zena consensus.Engine, chain *core.BlockChain, parentBlock *types.Block, signer []byte, zenaConfig *params.ZenaConfig, txs []*types.Transaction, currentValidators []*valset.Validator, opts ...Option) *types.Block {
	t.Helper()

	header := &types.Header{
		Number:     big.NewInt(int64(parentBlock.Number().Uint64() + 1)),
		Difficulty: big.NewInt(int64(parentBlock.Difficulty().Uint64())),
		GasLimit:   parentBlock.GasLimit(),
		ParentHash: parentBlock.Hash(),
	}
	number := header.Number.Uint64()

	if signer == nil {
		signer = getSignerKey(header.Number.Uint64())
	}

	header.Time = parentBlock.Time() + zena.CalcProducerDelay(header.Number.Uint64(), 0, zenaConfig)
	header.Extra = make([]byte, 32+65) // vanity + extraSea

	isSpanStart := IsSpanStart(number)
	isSprintEnd := IsSprintEnd(number)

	if isSpanStart {
		header.Difficulty = new(big.Int).SetInt64(int64(len(currentValidators)))
	}

	if isSprintEnd {
		sort.Sort(valset.ValidatorsByAddress(currentValidators))

		validatorBytes := make([]byte, len(currentValidators)*validatorHeaderBytesLength)
		header.Extra = make([]byte, 32+len(validatorBytes)+65) // vanity + validatorBytes + extraSeal

		for i, val := range currentValidators {
			copy(validatorBytes[i*validatorHeaderBytesLength:], val.HeaderBytes())
		}

		copy(header.Extra[32:], validatorBytes)
	}

	if chain.Config().IsLondon(header.Number) {
		header.BaseFee = eip1559.CalcBaseFee(chain.Config(), parentBlock.Header())

		if !chain.Config().IsLondon(parentBlock.Number()) {
			parentGasLimit := parentBlock.GasLimit() * params.ElasticityMultiplier
			header.GasLimit = core.CalcGasLimit(parentGasLimit, parentGasLimit)
		}
	}

	for _, opt := range opts {
		opt(header)
	}

	state, err := chain.State()
	if err != nil {
		t.Fatalf("%s", err)
	}

	b := &blockGen{header: header}
	for _, tx := range txs {
		b.addTxWithChain(chain, state, tx, addr)
	}

	// Finalize and seal the block
	block, err := _zena.FinalizeAndAssemble(chain, b.header, state, &types.Body{
		Transactions: b.txs,
	}, b.receipts)

	if err != nil {
		panic(fmt.Sprintf("error finalizing block: %v", err))
	}

	// Write state changes to db
	root, err := state.Commit(block.NumberU64(), chain.Config().IsEIP158(b.header.Number))
	if err != nil {
		panic(fmt.Sprintf("state write error: %v", err))
	}

	if err := state.Database().TrieDB().Commit(root, false); err != nil {
		panic(fmt.Sprintf("trie write error: %v", err))
	}

	res := make(chan *types.Block, 1)

	err = _zena.Seal(chain, block, res, nil)
	if err != nil {
		// an error case - sign manually
		sign(t, header, signer, zenaConfig)
		return types.NewBlockWithHeader(header)
	}

	return <-res
}

type blockGen struct {
	txs      []*types.Transaction
	receipts []*types.Receipt
	gasPool  *core.GasPool
	header   *types.Header
}

func (b *blockGen) addTxWithChain(bc *core.BlockChain, statedb *state.StateDB, tx *types.Transaction, coinbase common.Address) {
	if b.gasPool == nil {
		b.setCoinbase(coinbase)
	}

	statedb.SetTxContext(tx.Hash(), len(b.txs))

	receipt, err := core.ApplyTransaction(bc.Config(), bc, &b.header.Coinbase, b.gasPool, statedb, b.header, tx, &b.header.GasUsed, vm.Config{}, nil)
	if err != nil {
		panic(err)
	}

	b.txs = append(b.txs, tx)
	b.receipts = append(b.receipts, receipt)
}

func (b *blockGen) setCoinbase(addr common.Address) {
	if b.gasPool != nil {
		if len(b.txs) > 0 {
			panic("coinbase must be set before adding transactions")
		}

		panic("coinbase can only be set once")
	}

	b.header.Coinbase = addr
	b.gasPool = new(core.GasPool).AddGas(b.header.GasLimit)
}

func sign(t *testing.T, header *types.Header, signer []byte, c *params.ZenaConfig) {
	t.Helper()

	sig, err := secp256k1.Sign(crypto.Keccak256(zena.ZenaRLP(header, c)), signer)
	if err != nil {
		t.Fatalf("%s", err)
	}

	copy(header.Extra[len(header.Extra)-extraSeal:], sig)
}

//nolint:unused,deadcode
func stateSyncEventsPayload(t *testing.T) *iris.StateSyncEventsResponse {
	t.Helper()

	stateData, err := ioutil.ReadFile("./testdata/states.json")
	if err != nil {
		t.Fatalf("%s", err)
	}

	res := &iris.StateSyncEventsResponse{}
	if err := json.Unmarshal(stateData, res); err != nil {
		t.Fatalf("%s", err)
	}

	return res
}

//nolint:unused,deadcode
func loadSpanFromFile(t *testing.T) (*iris.SpanResponse, *span.IrisSpan) {
	t.Helper()

	spanData, err := ioutil.ReadFile("./testdata/span.json")
	if err != nil {
		t.Fatalf("%s", err)
	}

	res := &iris.SpanResponse{}

	if err := json.Unmarshal(spanData, res); err != nil {
		t.Fatalf("%s", err)
	}

	return res, &res.Result
}

func getSignerKey(number uint64) []byte {
	signerKey := privKey

	if IsSpanStart(number) {
		// validator set in the new span has changed
		signerKey = privKey2
	}

	newKey, _ := hex.DecodeString(signerKey)

	return newKey
}

func getMockedIrisClient(t *testing.T, irisSpan *span.IrisSpan) (*mocks.MockIIrisClient, *gomock.Controller) {
	t.Helper()

	ctrl := gomock.NewController(t)
	h := mocks.NewMockIIrisClient(ctrl)

	h.EXPECT().Span(gomock.Any(), uint64(1)).Return(irisSpan, nil).AnyTimes()

	h.EXPECT().StateSyncEvents(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*clerk.EventRecordWithTime{getSampleEventRecord(t)}, nil).AnyTimes()

	return h, ctrl
}

func getMockedSpanner(t *testing.T, validators []*valset.Validator) *zena.MockSpanner {
	t.Helper()

	spanner := zena.NewMockSpanner(gomock.NewController(t))
	spanner.EXPECT().GetCurrentValidatorsByHash(gomock.Any(), gomock.Any(), gomock.Any()).Return(validators, nil).AnyTimes()
	spanner.EXPECT().GetCurrentValidatorsByBlockNrOrHash(gomock.Any(), gomock.Any(), gomock.Any()).Return(validators, nil).AnyTimes()
	spanner.EXPECT().GetCurrentSpan(gomock.Any(), gomock.Any()).Return(&span.Span{0, 0, 0}, nil).AnyTimes()
	spanner.EXPECT().CommitSpan(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	return spanner
}

func generateFakeStateSyncEvents(sample *clerk.EventRecordWithTime, count int) []*clerk.EventRecordWithTime {
	events := make([]*clerk.EventRecordWithTime, count)
	event := *sample
	event.ID = 1
	events[0] = &clerk.EventRecordWithTime{}
	*events[0] = event

	for i := 1; i < count; i++ {
		event.ID = uint64(i + 1)
		event.Time = event.Time.Add(1 * time.Second)
		events[i] = &clerk.EventRecordWithTime{}
		*events[i] = event
	}

	return events
}

func buildStateEvent(sample *clerk.EventRecordWithTime, id uint64, timeStamp int64) *clerk.EventRecordWithTime {
	event := *sample
	event.ID = id
	event.Time = time.Unix(timeStamp, 0)

	return &event
}

func getSampleEventRecord(t *testing.T) *clerk.EventRecordWithTime {
	t.Helper()

	eventRecords := stateSyncEventsPayload(t)
	eventRecords.Result[0].Time = time.Unix(1, 0)

	return eventRecords.Result[0]
}

func newGwei(n int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(n), big.NewInt(params.GWei))
}

func IsSpanEnd(number uint64) bool {
	return (number+1)%spanSize == 0
}

func IsSpanStart(number uint64) bool {
	return number%spanSize == 0
}

func IsSprintStart(number uint64) bool {
	return number%sprintSize == 0
}

func IsSprintEnd(number uint64) bool {
	return (number+1)%sprintSize == 0
}

func InitGenesis(t *testing.T, faucets []*ecdsa.PrivateKey, fileLocation string, sprintSize uint64) *core.Genesis {
	t.Helper()

	// sprint size = 8 in genesis
	genesisData, err := ioutil.ReadFile(fileLocation)
	if err != nil {
		t.Fatalf("%s", err)
	}

	genesis := &core.Genesis{}

	if err := json.Unmarshal(genesisData, genesis); err != nil {
		t.Fatalf("%s", err)
	}

	genesis.Config.ChainID = big.NewInt(15001)
	genesis.Config.EIP150Block = big.NewInt(0)

	genesis.Config.Zena.Sprint["0"] = sprintSize

	return genesis
}

func InitMiner(genesis *core.Genesis, privKey *ecdsa.PrivateKey, withoutIris bool) (*node.Node, *eth.Zenanet, error) {
	// Define the basic configurations for the Zenanet node
	datadir, _ := ioutil.TempDir("", "")

	config := &node.Config{
		Name:    "gzen",
		Version: params.Version,
		DataDir: datadir,
		P2P: p2p.Config{
			ListenAddr:  "0.0.0.0:0",
			NoDiscovery: true,
			MaxPeers:    25,
		},
		UseLightweightKDF: true,
	}
	// Create the node and configure a full Zenanet node on it
	stack, err := node.New(config)
	if err != nil {
		return nil, nil, err
	}

	ethBackend, err := eth.New(stack, &ethconfig.Config{
		Genesis:         genesis,
		NetworkId:       genesis.Config.ChainID.Uint64(),
		SyncMode:        downloader.FullSync,
		DatabaseCache:   256,
		DatabaseHandles: 256,
		TxPool:          legacypool.DefaultConfig,
		GPO:             ethconfig.Defaults.GPO,
		Miner: miner.Config{
			Zenbase:  crypto.PubkeyToAddress(privKey.PublicKey),
			GasCeil:  genesis.GasLimit * 11 / 10,
			GasPrice: big.NewInt(1),
			Recommit: time.Second,
		},
		WithoutIris: withoutIris,
	})

	if err != nil {
		return nil, nil, err
	}

	// register backend to account manager with keystore for signing
	keydir := stack.KeyStoreDir()

	n, p := keystore.StandardScryptN, keystore.StandardScryptP
	kStore := keystore.NewKeyStore(keydir, n, p)

	_, err = kStore.ImportECDSA(privKey, "")

	if err != nil {
		return nil, nil, err
	}

	acc := kStore.Accounts()[0]
	err = kStore.Unlock(acc, "")

	if err != nil {
		return nil, nil, err
	}

	// proceed to authorize the local account manager in any case
	ethBackend.AccountManager().AddBackend(kStore)

	err = stack.Start()

	return stack, ethBackend, err
}
