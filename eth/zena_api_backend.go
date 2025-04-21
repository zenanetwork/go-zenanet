package eth

import (
	"context"
	"errors"
	"fmt"

	"github.com/zenanetwork/go-zenanet"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core"
	"github.com/zenanetwork/go-zenanet/core/rawdb"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/event"
	"github.com/zenanetwork/go-zenanet/rpc"
)

var errZenaEngineNotAvailable error = errors.New("Only available in Zena engine")

// GetRootHash returns root hash for given start and end block
func (b *EthAPIBackend) GetRootHash(ctx context.Context, starBlockNr uint64, endBlockNr uint64) (string, error) {
	var api *zena.API

	for _, _api := range b.eth.Engine().APIs(b.eth.BlockChain()) {
		if _api.Namespace == "zena" {
			api = _api.Service.(*zena.API)
		}
	}

	if api == nil {
		return "", errZenaEngineNotAvailable
	}

	root, err := api.GetRootHash(starBlockNr, endBlockNr)
	if err != nil {
		return "", err
	}

	return root, nil
}

// GetVoteOnHash returns the vote on hash
func (b *EthAPIBackend) GetVoteOnHash(ctx context.Context, starBlockNr uint64, endBlockNr uint64, hash string, milestoneId string) (bool, error) {
	var api *zena.API

	for _, _api := range b.eth.Engine().APIs(b.eth.BlockChain()) {
		if _api.Namespace == "zena" {
			api = _api.Service.(*zena.API)
		}
	}

	if api == nil {
		return false, errZenaEngineNotAvailable
	}

	// Confirmation of 16 blocks on the endblock
	tipConfirmationBlockNr := endBlockNr + uint64(16)

	// Check if tipConfirmation block exit
	_, err := b.BlockByNumber(ctx, rpc.BlockNumber(tipConfirmationBlockNr))
	if err != nil {
		return false, errTipConfirmationBlock
	}

	// Check if end block exist
	localEndBlock, err := b.BlockByNumber(ctx, rpc.BlockNumber(endBlockNr))
	if err != nil {
		return false, errEndBlock
	}

	localEndBlockHash := localEndBlock.Hash().String()

	downloader := b.eth.handler.downloader
	isLocked := downloader.LockMutex(endBlockNr)

	if !isLocked {
		downloader.UnlockMutex(false, "", endBlockNr, common.Hash{})
		return false, errors.New("whitelisted number or locked sprint number is more than the received end block number")
	}

	if localEndBlockHash != hash {
		downloader.UnlockMutex(false, "", endBlockNr, common.Hash{})
		return false, fmt.Errorf("hash mismatch: localChainHash %s, milestoneHash %s", localEndBlockHash, hash)
	}

	downloader.UnlockMutex(true, milestoneId, endBlockNr, localEndBlock.Hash())

	return true, nil
}

// GetZenaBlockReceipt returns zena block receipt
func (b *EthAPIBackend) GetZenaBlockReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	receipt := b.eth.blockchain.GetZenaReceiptByHash(hash)
	if receipt == nil {
		return nil, zenanet.NotFound
	}

	return receipt, nil
}

// GetZenaBlockLogs returns zena block logs
func (b *EthAPIBackend) GetZenaBlockLogs(ctx context.Context, hash common.Hash) ([]*types.Log, error) {
	receipt := b.eth.blockchain.GetZenaReceiptByHash(hash)
	if receipt == nil {
		return nil, nil
	}

	return receipt.Logs, nil
}

// GetZenaBlockTransaction returns zena block tx
func (b *EthAPIBackend) GetZenaBlockTransaction(ctx context.Context, hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	tx, blockHash, blockNumber, index := rawdb.ReadZenaTransaction(b.eth.ChainDb(), hash)
	return tx, blockHash, blockNumber, index, nil
}

func (b *EthAPIBackend) GetZenaBlockTransactionWithBlockHash(ctx context.Context, txHash common.Hash, blockHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	tx, blockHash, blockNumber, index := rawdb.ReadZenaTransactionWithBlockHash(b.eth.ChainDb(), txHash, blockHash)
	return tx, blockHash, blockNumber, index, nil
}

// SubscribeStateSyncEvent subscribes to state sync event
func (b *EthAPIBackend) SubscribeStateSyncEvent(ch chan<- core.StateSyncEvent) event.Subscription {
	return b.eth.BlockChain().SubscribeStateSyncEvent(ch)
}

// SubscribeChain2HeadEvent subscribes to reorg/head/fork event
func (b *EthAPIBackend) SubscribeChain2HeadEvent(ch chan<- core.Chain2HeadEvent) event.Subscription {
	return b.eth.BlockChain().SubscribeChain2HeadEvent(ch)
}
