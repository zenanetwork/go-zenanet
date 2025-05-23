// Copyright 2022 The go-zenanet Authors
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
// along with the go-zenanet library. If not, see <http://www.gnu.org/licenses/>

package miner

import (
	"reflect"
	"testing"
	"time"

	"github.com/zenanetwork/go-zenanet/beacon/engine"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/ethash"
	"github.com/zenanetwork/go-zenanet/core/rawdb"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/params"
)

func TestBuildPayload(t *testing.T) {
	var (
		db        = rawdb.NewMemoryDatabase()
		recipient = common.HexToAddress("0xdeadbeef")
	)

	w, b, _ := newTestWorker(t, params.TestChainConfig, ethash.NewFaker(), db, false, 0, 0)
	defer w.close()

	timestamp := uint64(time.Now().Unix())
	args := &BuildPayloadArgs{
		Parent:       b.chain.CurrentBlock().Hash(),
		Timestamp:    timestamp,
		Random:       common.Hash{},
		FeeRecipient: recipient,
	}

	payload, err := w.buildPayload(args)
	if err != nil {
		t.Fatalf("Failed to build payload %v", err)
	}

	verify := func(outer *engine.ExecutionPayloadEnvelope, txs int) {
		payload := outer.ExecutionPayload
		if payload.ParentHash != b.chain.CurrentBlock().Hash() {
			t.Fatal("Unexpected parent hash")
		}

		if payload.Random != (common.Hash{}) {
			t.Fatal("Unexpected random value")
		}

		if payload.Timestamp != timestamp {
			t.Fatal("Unexpected timestamp")
		}

		if payload.FeeRecipient != recipient {
			t.Fatal("Unexpected fee recipient")
		}

		if len(payload.Transactions) != txs {
			t.Fatal("Unexpected transaction set")
		}
	}
	empty := payload.ResolveEmpty()
	verify(empty, 0)

	full := payload.ResolveFull()
	verify(full, len(pendingTxs))

	// Ensure resolve can be called multiple times and the
	// result should be unchanged
	dataOne := payload.Resolve()
	dataTwo := payload.Resolve()

	if !reflect.DeepEqual(dataOne, dataTwo) {
		t.Fatal("Unexpected payload data")
	}
}

func TestPayloadId(t *testing.T) {
	t.Parallel()
	ids := make(map[string]int)

	for i, tt := range []*BuildPayloadArgs{
		{
			Parent:       common.Hash{1},
			Timestamp:    1,
			Random:       common.Hash{0x1},
			FeeRecipient: common.Address{0x1},
		},
		// Different parent
		{
			Parent:       common.Hash{2},
			Timestamp:    1,
			Random:       common.Hash{0x1},
			FeeRecipient: common.Address{0x1},
		},
		// Different timestamp
		{
			Parent:       common.Hash{2},
			Timestamp:    2,
			Random:       common.Hash{0x1},
			FeeRecipient: common.Address{0x1},
		},
		// Different Random
		{
			Parent:       common.Hash{2},
			Timestamp:    2,
			Random:       common.Hash{0x2},
			FeeRecipient: common.Address{0x1},
		},
		// Different fee-recipient
		{
			Parent:       common.Hash{2},
			Timestamp:    2,
			Random:       common.Hash{0x2},
			FeeRecipient: common.Address{0x2},
		},
		// Different withdrawals (non-empty)
		{
			Parent:       common.Hash{2},
			Timestamp:    2,
			Random:       common.Hash{0x2},
			FeeRecipient: common.Address{0x2},
			Withdrawals: []*types.Withdrawal{
				{
					Index:     0,
					Validator: 0,
					Address:   common.Address{},
					Amount:    0,
				},
			},
		},
		// Different withdrawals (non-empty)
		{
			Parent:       common.Hash{2},
			Timestamp:    2,
			Random:       common.Hash{0x2},
			FeeRecipient: common.Address{0x2},
			Withdrawals: []*types.Withdrawal{
				{
					Index:     2,
					Validator: 0,
					Address:   common.Address{},
					Amount:    0,
				},
			},
		},
	} {
		id := tt.Id().String()
		if prev, exists := ids[id]; exists {
			t.Errorf("ID collision, case %d and case %d: id %v", prev, i, id)
		}

		ids[id] = i
	}
}
