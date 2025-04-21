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

package eth

import (
	"fmt"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/common/hexutil"
)

// ZenanetAPI provides an API to access Zenanet full node-related information.
type ZenanetAPI struct {
	e *Zenanet
}

// NewZenanetAPI creates a new Zenanet protocol API for full nodes.
func NewZenanetAPI(e *Zenanet) *ZenanetAPI {
	return &ZenanetAPI{e}
}

// Zenbase is the address that mining rewards will be sent to.
func (api *ZenanetAPI) Zenbase() (common.Address, error) {
	return api.e.Zenbase()
}

// Coinbase is the address that mining rewards will be sent to (alias for Zenbase).
func (api *ZenanetAPI) Coinbase() (common.Address, error) {
	return api.Zenbase()
}

// Hashrate returns the POW hashrate.
func (api *ZenanetAPI) Hashrate() hexutil.Uint64 {
	return hexutil.Uint64(api.e.Miner().Hashrate())
}

// Mining returns an indication if this node is currently mining.
func (api *ZenanetAPI) Mining() bool {
	return api.e.IsMining()
}

func getFinalizedBlockNumber(eth *Zenanet) (uint64, error) {
	currentBlockNum := eth.BlockChain().CurrentBlock()

	doExist, number, hash := eth.Downloader().GetWhitelistedMilestone()
	if doExist && number <= currentBlockNum.Number.Uint64() {
		block := eth.BlockChain().GetBlockByNumber(number)

		if block.Hash() == hash {
			return number, nil
		}
	}

	doExist, number, hash = eth.Downloader().GetWhitelistedCheckpoint()
	if doExist && number <= currentBlockNum.Number.Uint64() {
		block := eth.BlockChain().GetBlockByNumber(number)

		if block.Hash() == hash {
			return number, nil
		}
	}

	return 0, fmt.Errorf("No finalized block")
}
