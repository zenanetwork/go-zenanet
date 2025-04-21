package miner

import (
	"errors"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus"
	"github.com/zenanetwork/go-zenanet/consensus/zena/api"
	"github.com/zenanetwork/go-zenanet/consensus/zena/valset"
	"github.com/zenanetwork/go-zenanet/consensus/zena"
	"github.com/zenanetwork/go-zenanet/core"
	"github.com/zenanetwork/go-zenanet/core/rawdb"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/txpool"
	"github.com/zenanetwork/go-zenanet/core/txpool/legacypool"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/core/vm"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/ethdb/memorydb"
	"github.com/zenanetwork/go-zenanet/event"
	"github.com/zenanetwork/go-zenanet/params"
	"github.com/zenanetwork/go-zenanet/tests/zena/mocks"
	"github.com/zenanetwork/go-zenanet/triedb"
)

type DefaultZenaMiner struct {
	Miner   *Miner
	Mux     *event.TypeMux //nolint:staticcheck
	Cleanup func(skipMiner bool)

	Ctrl           *gomock.Controller
	EthAPIMock     api.Caller
	IrisClientMock zena.IIrisClient
	ContractMock   zena.GenesisContract
}

func NewZenaDefaultMiner(t *testing.T) *DefaultZenaMiner {
	t.Helper()

	ctrl := gomock.NewController(t)

	ethAPI := api.NewMockCaller(ctrl)
	ethAPI.EXPECT().Call(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	spanner := zena.NewMockSpanner(ctrl)
	spanner.EXPECT().GetCurrentValidatorsByHash(gomock.Any(), gomock.Any(), gomock.Any()).Return([]*valset.Validator{
		{
			ID:               0,
			Address:          common.Address{0x1},
			VotingPower:      100,
			ProposerPriority: 0,
		},
	}, nil).AnyTimes()

	irisClient := mocks.NewMockIIrisClient(ctrl)
	irisClient.EXPECT().Close().Times(1)

	genesisContracts := zena.NewMockGenesisContract(ctrl)

	miner, mux, cleanup := createZenaMiner(t, ethAPI, spanner, irisClient, genesisContracts)

	return &DefaultZenaMiner{
		Miner:          miner,
		Mux:            mux,
		Cleanup:        cleanup,
		Ctrl:           ctrl,
		EthAPIMock:     ethAPI,
		IrisClientMock: irisClient,
		ContractMock:   genesisContracts,
	}
}

// //nolint:staticcheck
func createZenaMiner(t *testing.T, ethAPIMock api.Caller, spanner zena.Spanner, irisClientMock zena.IIrisClient, contractMock zena.GenesisContract) (*Miner, *event.TypeMux, func(skipMiner bool)) {
	t.Helper()

	// Create Ethash config
	chainDB, genspec, chainConfig := NewDBForFakes(t)

	engine := NewFakeZena(t, chainDB, chainConfig, ethAPIMock, spanner, irisClientMock, contractMock)

	// Create Zenanet backend
	bc, err := core.NewBlockChain(chainDB, nil, genspec, nil, engine, vm.Config{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("can't create new chain %v", err)
	}

	statedb, _ := state.New(common.Hash{}, state.NewDatabase(chainDB), nil)
	blockchain := &testBlockChainZena{chainConfig, statedb, 10000000, new(event.Feed)}

	pool := legacypool.New(testTxPoolConfigZena, blockchain)
	txpool, _ := txpool.New(testTxPoolConfigZena.PriceLimit, blockchain, []txpool.SubPool{pool})

	backend := NewMockBackendZena(bc, txpool)

	// Create event Mux
	mux := new(event.TypeMux)

	config := Config{
		Zenbase: common.HexToAddress("123456789"),
	}

	// Create Miner
	miner := New(backend, &config, chainConfig, mux, engine, nil)

	cleanup := func(skipMiner bool) {
		bc.Stop()
		engine.Close()

		if !skipMiner {
			miner.Close()
		}
	}

	return miner, mux, cleanup
}

type TensingObject interface {
	Helper()
	Fatalf(format string, args ...any)
}

func NewDBForFakes(t TensingObject) (ethdb.Database, *core.Genesis, *params.ChainConfig) {
	t.Helper()

	memdb := memorydb.New()
	chainDB := rawdb.NewDatabase(memdb)
	addr := common.HexToAddress("12345")
	genesis := core.DeveloperGenesisBlock(11_500_000, &addr)

	chainConfig, _, err := core.SetupGenesisBlock(chainDB, triedb.NewDatabase(chainDB, triedb.HashDefaults), genesis)
	if err != nil {
		t.Fatalf("can't create new chain config: %v", err)
	}

	chainConfig.Zena.Period = map[string]uint64{
		"0": 1,
	}
	chainConfig.Zena.Sprint = map[string]uint64{
		"0": 64,
	}

	return chainDB, genesis, chainConfig
}

func NewFakeZena(t TensingObject, chainDB ethdb.Database, chainConfig *params.ChainConfig, ethAPIMock api.Caller, spanner zena.Spanner, irisClientMock zena.IIrisClient, contractMock zena.GenesisContract) consensus.Engine {
	t.Helper()

	if chainConfig.Zena == nil {
		chainConfig.Zena = params.ZenaUnittestChainConfig.Zena
	}

	return zena.New(chainConfig, chainDB, ethAPIMock, spanner, irisClientMock, contractMock, false)
}

var (
	// Test chain configurations
	testTxPoolConfigZena legacypool.Config
)

// TODO - Arpit, Duplicate Functions
type mockBackendZena struct {
	bc     *core.BlockChain
	txPool *txpool.TxPool
}

func NewMockBackendZena(bc *core.BlockChain, txPool *txpool.TxPool) *mockBackendZena {
	return &mockBackendZena{
		bc:     bc,
		txPool: txPool,
	}
}

func (m *mockBackendZena) BlockChain() *core.BlockChain {
	return m.bc
}

// PeerCount implements Backend.
func (*mockBackendZena) PeerCount() int {
	panic("unimplemented")
}

func (m *mockBackendZena) TxPool() *txpool.TxPool {
	return m.txPool
}

func (m *mockBackendZena) StateAtBlock(block *types.Block, reexec uint64, base *state.StateDB, checkLive bool, preferDisk bool) (statedb *state.StateDB, err error) {
	return nil, errors.New("not supported")
}

// TODO - Arpit, Duplicate Functions
type testBlockChainZena struct {
	config        *params.ChainConfig
	statedb       *state.StateDB
	gasLimit      uint64
	chainHeadFeed *event.Feed
}

func (bc *testBlockChainZena) Config() *params.ChainConfig {
	return bc.config
}

func (bc *testBlockChainZena) CurrentBlock() *types.Header {
	return &types.Header{
		Number:   new(big.Int),
		GasLimit: bc.gasLimit,
	}
}

func (bc *testBlockChainZena) GetBlock(hash common.Hash, number uint64) *types.Block {
	return types.NewBlock(bc.CurrentBlock(), nil, nil, nil)
}

func (bc *testBlockChainZena) StateAt(common.Hash) (*state.StateDB, error) {
	return bc.statedb, nil
}

func (bc *testBlockChainZena) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return bc.chainHeadFeed.Subscribe(ch)
}
