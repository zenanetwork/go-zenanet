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

//go:build ignore
// +build ignore

package miner

// nolint : unused

import (
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
)

// pendingTTL indicates the period of time a generated pending block should
// exist to serve RPC requests before being discarded if the parent block
// has not changed yet. The value is chosen to align with the recommit interval.
const pendingTTL = 2 * time.Second

// pending wraps a pending block with additional metadata.
type pending struct {
	created    time.Time
	parentHash common.Hash
	result     *newPayloadResult
	lock       sync.Mutex
}

// resolve retrieves the cached pending result if it's available. Nothing will be
// returned if the parentHash is not matched or the result is already too old.
//
// Note, don't modify the returned payload result.
func (p *pending) resolve(parentHash common.Hash) *newPayloadResult {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.result == nil {
		return nil
	}
	if parentHash != p.parentHash {
		return nil
	}
	if time.Since(p.created) > pendingTTL {
		return nil
	}
	return p.result
}

// update refreshes the cached pending block with newly created one.
func (p *pending) update(parent common.Hash, result *newPayloadResult) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.parentHash = parent
	p.result = result
	p.created = time.Now()
}
