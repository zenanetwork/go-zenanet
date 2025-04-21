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

package filters

import (
	"context"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/params"
	"github.com/zenanetwork/go-zenanet/rpc"
)

// ZenaBlockLogsFilter can be used to retrieve and filter logs.
type ZenaBlockLogsFilter struct {
	backend    Backend
	zenaConfig *params.ZenaConfig

	db        ethdb.Database
	addresses []common.Address
	topics    [][]common.Hash

	block      common.Hash // Block hash if filtering a single block
	begin, end int64       // Range interval if filtering multiple blocks
}

// NewZenaBlockLogsRangeFilter creates a new filter which uses a bloom filter on blocks to
// figure out whether a particular block is interesting or not.
func NewZenaBlockLogsRangeFilter(backend Backend, zenaConfig *params.ZenaConfig, begin, end int64, addresses []common.Address, topics [][]common.Hash) *ZenaBlockLogsFilter {
	// Create a generic filter and convert it into a range filter
	filter := newZenaBlockLogsFilter(backend, zenaConfig, addresses, topics)
	filter.begin = begin
	filter.end = end

	return filter
}

// NewZenaBlockLogsFilter creates a new filter which directly inspects the contents of
// a block to figure out whether it is interesting or not.
func NewZenaBlockLogsFilter(backend Backend, zenaConfig *params.ZenaConfig, block common.Hash, addresses []common.Address, topics [][]common.Hash) *ZenaBlockLogsFilter {
	// Create a generic filter and convert it into a block filter
	filter := newZenaBlockLogsFilter(backend, zenaConfig, addresses, topics)
	filter.block = block

	return filter
}

// newZenaBlockLogsFilter creates a generic filter that can either filter based on a block hash,
// or based on range queries. The search criteria needs to be explicitly set.
func newZenaBlockLogsFilter(backend Backend, zenaConfig *params.ZenaConfig, addresses []common.Address, topics [][]common.Hash) *ZenaBlockLogsFilter {
	return &ZenaBlockLogsFilter{
		backend:    backend,
		zenaConfig: zenaConfig,
		addresses:  addresses,
		topics:     topics,
		db:         backend.ChainDb(),
	}
}

// Logs searches the blockchain for matching log entries, returning all from the
// first block that contains matches, updating the start of the filter accordingly.
func (f *ZenaBlockLogsFilter) Logs(ctx context.Context) ([]*types.Log, error) {
	// If we're doing singleton block filtering, execute and return
	if f.block != (common.Hash{}) {
		receipt, _ := f.backend.GetZenaBlockReceipt(ctx, f.block)
		if receipt == nil {
			return nil, nil
		}

		return f.zenaBlockLogs(ctx, receipt)
	}

	// Figure out the limits of the filter range
	header, _ := f.backend.HeaderByNumber(ctx, rpc.LatestBlockNumber)
	if header == nil {
		return nil, nil
	}

	head := header.Number.Uint64()

	if f.begin == -1 {
		f.begin = int64(head)
	}

	// adjust begin for sprint
	f.begin = currentSprintEnd(f.zenaConfig.CalculateSprint(uint64(f.begin)), f.begin)

	end := f.end
	if f.end == -1 {
		end = int64(head)
	}

	// Gather all indexed logs, and finish with non indexed ones
	return f.unindexedLogs(ctx, uint64(end))
}

// unindexedLogs returns the logs matching the filter criteria based on raw block
// iteration and bloom matching.
func (f *ZenaBlockLogsFilter) unindexedLogs(ctx context.Context, end uint64) ([]*types.Log, error) {
	var logs []*types.Log

	sprintLength := f.zenaConfig.CalculateSprint(uint64(f.begin))

	for ; f.begin <= int64(end); f.begin = f.begin + int64(sprintLength) {
		header, err := f.backend.HeaderByNumber(ctx, rpc.BlockNumber(f.begin))
		if header == nil || err != nil {
			return logs, err
		}

		// get zena block receipt
		receipt, err := f.backend.GetZenaBlockReceipt(ctx, header.Hash())
		if receipt == nil || err != nil {
			continue
		}

		// filter zena block logs
		found, err := f.zenaBlockLogs(ctx, receipt)
		if err != nil {
			return logs, err
		}

		logs = append(logs, found...)
		sprintLength = f.zenaConfig.CalculateSprint(uint64(f.begin))
	}

	return logs, nil
}

// zenaBlockLogs returns the logs matching the filter criteria within a single block.
func (f *ZenaBlockLogsFilter) zenaBlockLogs(ctx context.Context, receipt *types.Receipt) (logs []*types.Log, err error) {
	if bloomFilter(receipt.Bloom, f.addresses, f.topics) {
		logs = filterLogs(receipt.Logs, nil, nil, f.addresses, f.topics)
	}

	return logs, nil
}

func currentSprintEnd(sprint uint64, n int64) int64 {
	m := n % int64(sprint)
	if m == 0 {
		return n
	}

	return n + int64(sprint) - m
}
