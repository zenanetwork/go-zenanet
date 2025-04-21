// Copyright 2015 The go-zenanet Authors
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

// Package ethapi implements the general Zenanet API functions.
package ethapi

import (
	"context"
	"math/big"
	"time"

	"github.com/zenanetwork/go-zenanet"
	"github.com/zenanetwork/go-zenanet/accounts"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus"
	"github.com/zenanetwork/go-zenanet/core"
	"github.com/zenanetwork/go-zenanet/core/bloombits"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/core/vm"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/event"
	"github.com/zenanetwork/go-zenanet/params"
	"github.com/zenanetwork/go-zenanet/rpc"
)

// Backend interface provides the common API services (that are provided by
// both full and light clients) with access to necessary functions.
type Backend interface {
	// General Zenanet API
	SyncProgress() zenanet.SyncProgress

	SuggestGasTipCap(ctx context.Context) (*big.Int, error)
	FeeHistory(ctx context.Context, blockCount uint64, lastBlock rpc.BlockNumber, rewardPercentiles []float64) (*big.Int, [][]*big.Int, []*big.Int, []float64, []*big.Int, []float64, error)
	BlobBaseFee(ctx context.Context) *big.Int
	ChainDb() ethdb.Database
	AccountManager() *accounts.Manager
	ExtRPCEnabled() bool
	RPCGasCap() uint64             // global gas cap for eth_call over rpc: DoS protection
	RPCRpcReturnDataLimit() uint64 // Maximum size (in bytes) a result of an rpc request could have
	RPCEVMTimeout() time.Duration  // global timeout for eth_call over rpc: DoS protection
	RPCTxFeeCap() float64          // global tx fee cap for all transaction related APIs
	UnprotectedAllowed() bool      // allows only for EIP155 transactions.

	// Blockchain API
	SetHead(number uint64)
	HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error)
	HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error)
	HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error)
	CurrentHeader() *types.Header
	CurrentBlock() *types.Header
	BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error)
	BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error)
	BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error)
	StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error)
	StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error)
	Pending() (*types.Block, types.Receipts, *state.StateDB)
	GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error)
	GetTd(ctx context.Context, hash common.Hash) *big.Int
	GetEVM(ctx context.Context, msg *core.Message, state *state.StateDB, header *types.Header, vmConfig *vm.Config, blockCtx *vm.BlockContext) *vm.EVM
	SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription
	SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription
	SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription

	// Transaction pool API
	SendTx(ctx context.Context, signedTx *types.Transaction) error
	GetTransaction(ctx context.Context, txHash common.Hash) (bool, *types.Transaction, common.Hash, uint64, uint64, error)
	GetPoolTransactions() (types.Transactions, error)
	GetPoolTransaction(txHash common.Hash) *types.Transaction
	GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error)
	Stats() (pending int, queued int)
	TxPoolContent() (map[common.Address][]*types.Transaction, map[common.Address][]*types.Transaction)
	TxPoolContentFrom(addr common.Address) ([]*types.Transaction, []*types.Transaction)
	SubscribeNewTxsEvent(chan<- core.NewTxsEvent) event.Subscription

	ChainConfig() *params.ChainConfig
	Engine() consensus.Engine

	// This is copied from filters.Backend
	// eth/filters needs to be initialized from this backend type, so methods needed by
	// it must also be included here.
	GetBody(ctx context.Context, hash common.Hash, number rpc.BlockNumber) (*types.Body, error)
	GetLogs(ctx context.Context, blockHash common.Hash, number uint64) ([][]*types.Log, error)
	SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription
	SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription
	SubscribePendingLogsEvent(ch chan<- []*types.Log) event.Subscription
	BloomStatus() (uint64, uint64)
	ServiceFilter(ctx context.Context, session *bloombits.MatcherSession)

	// Zena related APIs
	SubscribeStateSyncEvent(ch chan<- core.StateSyncEvent) event.Subscription
	GetRootHash(ctx context.Context, starBlockNr uint64, endBlockNr uint64) (string, error)
	GetVoteOnHash(ctx context.Context, startBlockNumber uint64, endBlockNumber uint64, hash string, milestoneID string) (bool, error)
	GetZenaBlockReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error)
	GetZenaBlockLogs(ctx context.Context, hash common.Hash) ([]*types.Log, error)
	GetZenaBlockTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error)
	GetZenaBlockTransactionWithBlockHash(ctx context.Context, txHash common.Hash, blockHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error)
	SubscribeChain2HeadEvent(ch chan<- core.Chain2HeadEvent) event.Subscription
	GetWhitelistedCheckpoint() (bool, uint64, common.Hash)
	PurgeWhitelistedCheckpoint()
	GetWhitelistedMilestone() (bool, uint64, common.Hash)
	PurgeWhitelistedMilestone()

	// Networking related APIs
	PeerStats() interface{}
}

func GetAPIs(apiBackend Backend) []rpc.API {
	nonceLock := new(AddrLocker)

	return []rpc.API{
		{
			Namespace: "eth",
			Service:   NewZenanetAPI(apiBackend),
		}, {
			Namespace: "eth",
			Service:   NewBlockChainAPI(apiBackend),
		}, {
			Namespace: "eth",
			Service:   NewTransactionAPI(apiBackend, nonceLock),
		}, {
			Namespace: "txpool",
			Service:   NewTxPoolAPI(apiBackend),
		}, {
			Namespace: "debug",
			Service:   NewDebugAPI(apiBackend),
		}, {
			Namespace: "eth",
			Service:   NewZenanetAccountAPI(apiBackend.AccountManager()),
		}, {
			Namespace: "personal",
			Service:   NewPersonalAccountAPI(apiBackend, nonceLock),
		}, {
			Namespace: "zena",
			Service:   NewZenaAPI(apiBackend),
		},
	}
}
