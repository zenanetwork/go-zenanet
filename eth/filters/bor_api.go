package filters

import (
	"context"
	"errors"

	zenanet "github.com/zenanetwork/go-zenanet"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/params"
	"github.com/zenanetwork/go-zenanet/rpc"
)

// SetChainConfig sets chain config
func (api *FilterAPI) SetChainConfig(chainConfig *params.ChainConfig) {
	api.chainConfig = chainConfig
}

func (api *FilterAPI) GetZenaBlockLogs(ctx context.Context, crit FilterCriteria) ([]*types.Log, error) {
	if api.chainConfig == nil {
		return nil, errors.New("no chain config found. Proper PublicFilterAPI initialization required")
	}

	// get sprint from zena config
	zenaConfig := api.chainConfig.Zena

	var filter *ZenaBlockLogsFilter
	if crit.BlockHash != nil {
		// Block filter requested, construct a single-shot filter
		filter = NewZenaBlockLogsFilter(api.sys.backend, zenaConfig, *crit.BlockHash, crit.Addresses, crit.Topics)
	} else {
		// Convert the RPC block numbers into internal representations
		begin := rpc.LatestBlockNumber.Int64()
		if crit.FromBlock != nil {
			begin = crit.FromBlock.Int64()
		}

		end := rpc.LatestBlockNumber.Int64()
		if crit.ToBlock != nil {
			end = crit.ToBlock.Int64()
		}
		// Construct the range filter
		filter = NewZenaBlockLogsRangeFilter(api.sys.backend, zenaConfig, begin, end, crit.Addresses, crit.Topics)
	}

	// Run the filter and return all the logs
	logs, err := filter.Logs(ctx)
	if err != nil {
		return nil, err
	}

	return returnLogs(logs), err
}

// NewDeposits send a notification each time a new deposit received from bridge.
func (api *FilterAPI) NewDeposits(ctx context.Context, crit zenanet.StateSyncFilter) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		stateSyncData := make(chan *types.StateSyncData, 10)
		stateSyncSub := api.events.SubscribeNewDeposits(stateSyncData)

		// nolint: gosimple
		for {
			select {
			case h := <-stateSyncData:
				if h != nil && (crit.ID == h.ID || crit.Contract == h.Contract ||
					(crit.ID == 0 && crit.Contract == common.Address{})) {
					notifier.Notify(rpcSub.ID, h)
				}
			case <-rpcSub.Err():
				stateSyncSub.Unsubscribe()
				return
			case <-notifier.Closed():
				stateSyncSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}
