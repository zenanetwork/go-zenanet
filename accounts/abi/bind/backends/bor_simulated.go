package backends

import (
	"context"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core"
	"github.com/zenanetwork/go-zenanet/core/rawdb"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/event"
)

func (fb *filterBackend) GetZenaBlockReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error) {
	number := rawdb.ReadHeaderNumber(fb.db, hash)
	if number == nil {
		return nil, nil
	}

	receipt := rawdb.ReadRawZenaReceipt(fb.db, hash, *number)
	if receipt == nil {
		return nil, nil
	}

	return receipt, nil
}

func (fb *filterBackend) GetVoteOnHash(ctx context.Context, starBlockNr uint64, endBlockNr uint64, hash string, milestoneId string) (bool, error) {
	return false, nil
}

func (fb *filterBackend) GetZenaBlockLogs(ctx context.Context, hash common.Hash) ([]*types.Log, error) {
	receipt, err := fb.GetZenaBlockReceipt(ctx, hash)
	if err != nil || receipt == nil {
		return nil, err
	}

	return receipt.Logs, nil
}

// SubscribeStateSyncEvent subscribes to state sync events
func (fb *filterBackend) SubscribeStateSyncEvent(ch chan<- core.StateSyncEvent) event.Subscription {
	return fb.bc.SubscribeStateSyncEvent(ch)
}
