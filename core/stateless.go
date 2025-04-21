// Copyright 2024 The go-zenanet Authors
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

package core

import (
	"context"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/common/lru"
	"github.com/zenanetwork/go-zenanet/consensus/beacon"
	"github.com/zenanetwork/go-zenanet/consensus/ethash"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/stateless"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/core/vm"
	"github.com/zenanetwork/go-zenanet/params"
	"github.com/zenanetwork/go-zenanet/trie"
	"github.com/zenanetwork/go-zenanet/triedb"
)

// ExecuteStateless runs a stateless execution based on a witness, verifies
// everything it can locally and returns the two computed fields that need the
// other side to explicitly check.
//
// This method is a bit of a sore thumb here, but:
//   - It cannot be placed in core/stateless, because state.New prodces a circular dep
//   - It cannot be placed outside of core, because it needs to construct a dud headerchain
//
// TODO(karalabe): Would be nice to resolve both issues above somehow and move it.
func ExecuteStateless(config *params.ChainConfig, witness *stateless.Witness) (common.Hash, common.Hash, error) {
	// Create and populate the state database to serve as the stateless backend
	memdb := witness.MakeHashDB()

	db, err := state.New(witness.Root(), state.NewDatabaseWithConfig(memdb, triedb.HashDefaults), nil)
	if err != nil {
		return common.Hash{}, common.Hash{}, err
	}
	// Create a blockchain that is idle, but can be used to access headers through
	headerChain := &HeaderChain{
		config:      config,
		chainDb:     memdb,
		headerCache: lru.NewCache[common.Hash, *types.Header](256),
		engine:      beacon.New(ethash.NewFaker()),
	}
	processor := NewStateProcessor(config, nil, headerChain)
	validator := NewBlockValidator(config, nil) // No chain, we only validate the state, not the block

	// Run the stateless blocks processing and self-validate certain fields
	receipts, _, usedGas, err := processor.Process(witness.Block, db, vm.Config{}, context.Background())
	if err != nil {
		return common.Hash{}, common.Hash{}, err
	}
	if err = validator.ValidateState(witness.Block, db, receipts, usedGas, true); err != nil {
		return common.Hash{}, common.Hash{}, err
	}
	// Almost everything validated, but receipt and state root needs to be returned
	receiptRoot := types.DeriveSha(receipts, trie.NewStackTrie(nil))
	stateRoot := db.IntermediateRoot(config.IsEIP158(witness.Block.Number()))

	return receiptRoot, stateRoot, nil
}
