// Copyright 2020 The go-zenanet Authors
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

// Package miner implements Zenanet block creation and mining.
package miner

import (
	"math/big"
	"testing"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/txpool"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/eth/downloader"
	"github.com/zenanetwork/go-zenanet/event"
	"github.com/zenanetwork/go-zenanet/params"
	"github.com/zenanetwork/go-zenanet/trie"
)

type mockBackend struct {
	bc     *core.BlockChain
	txPool *txpool.TxPool
}

func NewMockBackend(bc *core.BlockChain, txPool *txpool.TxPool) *mockBackend {
	return &mockBackend{
		bc:     bc,
		txPool: txPool,
	}
}

func (m *mockBackend) BlockChain() *core.BlockChain {
	return m.bc
}

// PeerCount implements Backend.
func (*mockBackend) PeerCount() int {
	panic("unimplemented")
}

func (m *mockBackend) TxPool() *txpool.TxPool {
	return m.txPool
}

// nolint : unused
type testBlockChain struct {
	root          common.Hash
	config        *params.ChainConfig
	statedb       *state.StateDB
	gasLimit      uint64
	chainHeadFeed *event.Feed
}

// nolint : unused
func (bc *testBlockChain) Config() *params.ChainConfig {
	return bc.config
}

// nolint : unused
func (bc *testBlockChain) CurrentBlock() *types.Header {
	return &types.Header{
		Number:   new(big.Int),
		GasLimit: bc.gasLimit,
	}
}

// nolint : unused
func (bc *testBlockChain) GetBlock(hash common.Hash, number uint64) *types.Block {
	return types.NewBlock(bc.CurrentBlock(), nil, nil, trie.NewStackTrie(nil))
}

// nolint : unused
func (bc *testBlockChain) StateAt(common.Hash) (*state.StateDB, error) {
	return bc.statedb, nil
}

// nolint : unused
func (bc *testBlockChain) HasState(root common.Hash) bool {
	return bc.root == root
}

// nolint : unused
func (bc *testBlockChain) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return bc.chainHeadFeed.Subscribe(ch)
}

func TestMiner(t *testing.T) {
	t.Parallel()

	minerZena := NewZenaDefaultMiner(t)
	defer func() {
		minerZena.Cleanup(false)
		minerZena.Ctrl.Finish()
	}()

	miner := minerZena.Miner
	mux := minerZena.Mux

	miner.Start()
	waitForMiningState(t, miner, true)

	// Start the downloader
	mux.Post(downloader.StartEvent{})
	waitForMiningState(t, miner, false)

	// Stop the downloader and wait for the update loop to run
	mux.Post(downloader.DoneEvent{})
	waitForMiningState(t, miner, true)

	// Subsequent downloader events after a successful DoneEvent should not cause the
	// miner to start or stop. This prevents a security vulnerability
	// that would allow entities to present fake high blocks that would
	// stop mining operations by causing a downloader sync
	// until it was discovered they were invalid, whereon mining would resume.
	mux.Post(downloader.StartEvent{})
	waitForMiningState(t, miner, true)

	mux.Post(downloader.FailedEvent{})
	waitForMiningState(t, miner, true)
}

// TestMinerDownloaderFirstFails tests that mining is only
// permitted to run indefinitely once the downloader sees a DoneEvent (success).
// An initial FailedEvent should allow mining to stop on a subsequent
// downloader StartEvent.
func TestMinerDownloaderFirstFails(t *testing.T) {
	t.Parallel()

	minerZena := NewZenaDefaultMiner(t)
	defer func() {
		minerZena.Cleanup(false)
		minerZena.Ctrl.Finish()
	}()

	miner := minerZena.Miner
	mux := minerZena.Mux

	miner.Start()
	waitForMiningState(t, miner, true)

	// Start the downloader
	mux.Post(downloader.StartEvent{})
	waitForMiningState(t, miner, false)

	// Stop the downloader and wait for the update loop to run
	mux.Post(downloader.FailedEvent{})
	waitForMiningState(t, miner, true)

	// Since the downloader hasn't yet emitted a successful DoneEvent,
	// we expect the miner to stop on next StartEvent.
	mux.Post(downloader.StartEvent{})
	waitForMiningState(t, miner, false)

	// Downloader finally succeeds.
	mux.Post(downloader.DoneEvent{})
	waitForMiningState(t, miner, true)

	// Downloader starts again.
	// Since it has achieved a DoneEvent once, we expect miner
	// state to be unchanged.
	mux.Post(downloader.StartEvent{})
	waitForMiningState(t, miner, true)

	mux.Post(downloader.FailedEvent{})
	waitForMiningState(t, miner, true)
}

func TestMinerStartStopAfterDownloaderEvents(t *testing.T) {
	t.Parallel()

	minerZena := NewZenaDefaultMiner(t)
	defer func() {
		minerZena.Cleanup(false)
		minerZena.Ctrl.Finish()
	}()

	miner := minerZena.Miner
	mux := minerZena.Mux

	miner.Start()
	waitForMiningState(t, miner, true)

	// Start the downloader
	mux.Post(downloader.StartEvent{})
	waitForMiningState(t, miner, false)

	// Downloader finally succeeds.
	mux.Post(downloader.DoneEvent{})
	waitForMiningState(t, miner, true)

	ch := make(chan struct{})
	miner.Stop(ch)
	waitForMiningState(t, miner, false)

	miner.Start()
	waitForMiningState(t, miner, true)

	ch = make(chan struct{})
	miner.Stop(ch)
	waitForMiningState(t, miner, false)
}

func TestStartWhileDownload(t *testing.T) {
	t.Parallel()

	minerZena := NewZenaDefaultMiner(t)
	defer func() {
		minerZena.Cleanup(false)
		minerZena.Ctrl.Finish()
	}()

	miner := minerZena.Miner
	mux := minerZena.Mux

	waitForMiningState(t, miner, false)
	miner.Start()
	waitForMiningState(t, miner, true)

	// Stop the downloader and wait for the update loop to run
	mux.Post(downloader.StartEvent{})
	waitForMiningState(t, miner, false)

	// Starting the miner after the downloader should not work
	miner.Start()
	waitForMiningState(t, miner, false)
}

func TestStartStopMiner(t *testing.T) {
	t.Parallel()

	minerZena := NewZenaDefaultMiner(t)
	defer func() {
		minerZena.Cleanup(false)
		minerZena.Ctrl.Finish()
	}()

	miner := minerZena.Miner

	waitForMiningState(t, miner, false)
	miner.Start()
	waitForMiningState(t, miner, true)

	ch := make(chan struct{})
	miner.Stop(ch)

	waitForMiningState(t, miner, false)
}

func TestCloseMiner(t *testing.T) {
	t.Parallel()

	minerZena := NewZenaDefaultMiner(t)
	defer func() {
		minerZena.Cleanup(true)
		minerZena.Ctrl.Finish()
	}()

	miner := minerZena.Miner

	waitForMiningState(t, miner, false)
	miner.Start()

	miner.Start()

	waitForMiningState(t, miner, true)

	// Terminate the miner and wait for the update loop to run
	miner.Close()

	waitForMiningState(t, miner, false)
}

// // TestMinerSetZenbase checks that zenbase becomes set even if mining isn't
// // possible at the moment
func TestMinerSetZenbase(t *testing.T) {
	t.Parallel()

	minerZena := NewZenaDefaultMiner(t)
	defer func() {
		minerZena.Cleanup(false)
		minerZena.Ctrl.Finish()
	}()

	miner := minerZena.Miner
	mux := minerZena.Mux

	// Start with a 'bad' mining address
	miner.Start()
	waitForMiningState(t, miner, true)

	// Start the downloader
	mux.Post(downloader.StartEvent{})
	waitForMiningState(t, miner, false)

	// Now user tries to configure proper mining address
	miner.Start()
	// Stop the downloader and wait for the update loop to run
	mux.Post(downloader.DoneEvent{})
	waitForMiningState(t, miner, true)

	coinbase := common.HexToAddress("0xdeedbeef")
	miner.SetZenbase(coinbase)

	if addr := miner.worker.zenbase(); addr != coinbase {
		t.Fatalf("Unexpected zenbase want %x got %x", coinbase, addr)
	}
}

// waitForMiningState waits until either
// * the desired mining state was reached
// * a timeout was reached which fails the test
func waitForMiningState(t *testing.T, m *Miner, mining bool) {
	t.Helper()

	var state bool

	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)

		if state = m.Mining(); state == mining {
			return
		}
	}
	t.Fatalf("Mining() == %t, want %t", state, mining)
}

// func TestBuildPendingBlocks(t *testing.T) {
// 	miner := createMiner(t)
// 	var wg sync.WaitGroup
// 	wg.Add(1)
// 	go func() {
// 		defer wg.Done()
// 		block, _, _ := miner.Pending()
// 		if block == nil {
// 			t.Error("Pending failed")
// 		}
// 	}()
// 	wg.Wait()
// }

// func minerTestGenesisBlock(period uint64, gasLimit uint64, faucet common.Address) *core.Genesis {
// 	config := *params.AllCliqueProtocolChanges
// 	config.Clique = &params.CliqueConfig{
// 		Period: period,
// 		Epoch:  config.Clique.Epoch,
// 	}

// 	// Assemble and return the genesis with the precompiles and faucet pre-funded
// 	return &core.Genesis{
// 		Config:     &config,
// 		ExtraData:  append(append(make([]byte, 32), faucet[:]...), make([]byte, crypto.SignatureLength)...),
// 		GasLimit:   gasLimit,
// 		BaseFee:    big.NewInt(params.InitialBaseFee),
// 		Difficulty: big.NewInt(1),
// 		Alloc: map[common.Address]types.Account{
// 			common.BytesToAddress([]byte{1}): {Balance: big.NewInt(1)}, // ECRecover
// 			common.BytesToAddress([]byte{2}): {Balance: big.NewInt(1)}, // SHA256
// 			common.BytesToAddress([]byte{3}): {Balance: big.NewInt(1)}, // RIPEMD
// 			common.BytesToAddress([]byte{4}): {Balance: big.NewInt(1)}, // Identity
// 			common.BytesToAddress([]byte{5}): {Balance: big.NewInt(1)}, // ModExp
// 			common.BytesToAddress([]byte{6}): {Balance: big.NewInt(1)}, // ECAdd
// 			common.BytesToAddress([]byte{7}): {Balance: big.NewInt(1)}, // ECScalarMul
// 			common.BytesToAddress([]byte{8}): {Balance: big.NewInt(1)}, // ECPairing
// 			common.BytesToAddress([]byte{9}): {Balance: big.NewInt(1)}, // BLAKE2b
// 			faucet:                           {Balance: new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(9))},
// 		},
// 	}
// }

// func createMiner(t *testing.T) (*Miner, *event.TypeMux, func(skipMiner bool)) {
// 	t.Helper()

// 	// Create Ethash config
// 	config := Config{
// 		Zenbase: common.HexToAddress("123456789"),
// 	}
// 	// Create chainConfig
// 	chainDB := rawdb.NewMemoryDatabase()
// 	genesis := minerTestGenesisBlock(15, 11_500_000, common.HexToAddress("12345"))
// 	chainConfig, _, err := core.SetupGenesisBlock(chainDB, trie.NewDatabase(chainDB), genesis)
// 	if err != nil {
// 		t.Fatalf("can't create new chain config: %v", err)
// 	}
// 	// Create consensus engine
// 	engine := clique.New(chainConfig.Clique, chainDB)
// 	// Create Zenanet backend
// 	bc, err := core.NewBlockChain(chainDB, nil, genesis, nil, engine, vm.Config{}, nil, nil)
// 	if err != nil {
// 		t.Fatalf("can't create new chain %v", err)
// 	}
// 	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabase(chainDB), nil)
// 	blockchain := &testBlockChain{chainConfig, statedb, 10000000, new(event.Feed)}

// 	pool := legacypool.New(testTxPoolConfig, blockchain)
// 	txpool, _ := txpool.New(new(big.Int).SetUint64(testTxPoolConfig.PriceLimit), blockchain, []txpool.SubPool{pool})

// 	backend := NewMockBackend(bc, txpool)
// 	// Create event Mux
// 	// nolint:staticcheck
// 	mux := new(event.TypeMux)
// 	// Create Miner
// 	miner := New(backend, &config, chainConfig, mux, engine, nil)
// 	cleanup := func(skipMiner bool) {
// 		bc.Stop()
// 		engine.Close()
// 		txpool.Close()
// 		if !skipMiner {
// 			miner.Close()
// 		}
// 	}

// 	return miner, mux, cleanup
// }
