// Copyright 2023 The go-zenanet Authors
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

// Package cosmos implements the Cosmos SDK adapter for the Eirene consensus algorithm.
package cosmos

import (
	"github.com/zenanetwork/go-zenanet/consensus/eirene/core"
	"github.com/zenanetwork/go-zenanet/log"
)

// CosmosAdapter는 Cosmos SDK의 핵심 기능과 go-zenanet을 연결하는 어댑터입니다.
type CosmosAdapter struct {
	eirene      *core.Eirene
	logger      log.Logger
	storeAdapter *StateDBAdapter
}

// NewCosmosAdapter는 새로운 CosmosAdapter 인스턴스를 생성합니다.
func NewCosmosAdapter(eirene *core.Eirene) *CosmosAdapter {
	return &CosmosAdapter{
		eirene:      eirene,
		logger:      log.New("module", "cosmos"),
		storeAdapter: NewStateDBAdapter(nil), // 초기에는 nil로 설정하고 나중에 설정
	}
}

// SetStateDBAdapter는 상태 DB 어댑터를 설정합니다.
func (a *CosmosAdapter) SetStateDBAdapter(storeAdapter *StateDBAdapter) {
	a.storeAdapter = storeAdapter
}

// InitSDK는 Cosmos SDK를 초기화합니다.
func (a *CosmosAdapter) InitSDK() error {
	a.logger.Info("Initializing Cosmos SDK")
	// TODO: Cosmos SDK 초기화 로직 구현
	return nil
}

// GetEirene는 Eirene 인스턴스를 반환합니다.
func (a *CosmosAdapter) GetEirene() *core.Eirene {
	return a.eirene
}

// GetStoreAdapter는 상태 DB 어댑터를 반환합니다.
func (a *CosmosAdapter) GetStoreAdapter() *StateDBAdapter {
	return a.storeAdapter
} 