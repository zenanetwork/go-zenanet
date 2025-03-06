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

package core

import (
	"math/big"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/event"
)

// BlockChain은 블록체인 인터페이스를 정의합니다.
type BlockChain interface {
	// CurrentBlock은 현재 블록을 반환합니다.
	CurrentBlock() *types.Block
	
	// GetBlock은 해시와 번호로 블록을 검색합니다.
	GetBlock(hash common.Hash, number uint64) *types.Block
	
	// GetBlockByHash는 해시로 블록을 검색합니다.
	GetBlockByHash(hash common.Hash) *types.Block
	
	// GetBlockByNumber는 번호로 블록을 검색합니다.
	GetBlockByNumber(number uint64) *types.Block
	
	// GetHeaderByHash는 해시로 헤더를 검색합니다.
	GetHeaderByHash(hash common.Hash) *types.Header
	
	// GetHeaderByNumber는 번호로 헤더를 검색합니다.
	GetHeaderByNumber(number uint64) *types.Header
	
	// GetTd는 해시와 번호로 총 난이도를 검색합니다.
	GetTd(hash common.Hash, number uint64) *big.Int
	
	// InsertChain은 블록 체인을 삽입합니다.
	InsertChain(chain []*types.Block) (int, error)
	
	// SubscribeChainEvent는 체인 이벤트를 구독합니다.
	SubscribeChainEvent(ch chan<- ChainEvent) event.Subscription
	
	// SubscribeChainHeadEvent는 체인 헤드 이벤트를 구독합니다.
	SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription
}

// TxPool은 트랜잭션 풀 인터페이스를 정의합니다.
type TxPool interface {
	// AddRemotes는 새 트랜잭션을 풀에 추가합니다.
	AddRemotes([]*types.Transaction) []error
	
	// Pending은 대기 중인 트랜잭션을 반환합니다.
	Pending(bool) (map[common.Address]types.Transactions, error)
	
	// SubscribeNewTxsEvent는 새 트랜잭션 이벤트를 구독합니다.
	SubscribeNewTxsEvent(chan<- NewTxsEvent) event.Subscription
}

// ChainEvent는 블록체인 이벤트를 나타냅니다.
type ChainEvent struct {
	Block *types.Block
	Hash  common.Hash
	Logs  []*types.Log
}

// ChainHeadEvent는 체인 헤드 이벤트를 나타냅니다.
type ChainHeadEvent struct {
	Block *types.Block
}

// NewTxsEvent는 새 트랜잭션 이벤트를 나타냅니다.
type NewTxsEvent struct {
	Txs []*types.Transaction
}

// NewMinedBlockEvent는 새로 채굴된 블록 이벤트를 나타냅니다.
type NewMinedBlockEvent struct {
	Block *types.Block
}

// EireneValidator는 검증자 인터페이스를 정의합니다.
type EireneValidator interface {
	// ValidateBlock은 블록을 검증합니다.
	ValidateBlock(block *types.Block) error
	
	// ValidateHeader는 헤더를 검증합니다.
	ValidateHeader(header *types.Header, parent *types.Header) error
	
	// ValidateHeaders는 헤더 목록을 검증합니다.
	ValidateHeaders(headers []*types.Header, parents []*types.Header) (chan<- struct{}, <-chan error)
}

// StateDB는 상태 데이터베이스 인터페이스를 정의합니다.
type StateDB interface {
	// 상태 DB 인터페이스 메서드
	GetBalance(addr common.Address) *big.Int
	GetNonce(addr common.Address) uint64
	GetCode(addr common.Address) []byte
	GetState(addr common.Address, hash common.Hash) common.Hash
	SetState(addr common.Address, key, value common.Hash)
	SetBalance(addr common.Address, amount *big.Int)
	SetNonce(addr common.Address, nonce uint64)
	SetCode(addr common.Address, code []byte)
	Suicide(addr common.Address) bool
	HasSuicided(addr common.Address) bool
	Exist(addr common.Address) bool
	Empty(addr common.Address) bool
	AddRefund(gas uint64)
	GetRefund() uint64
	GetCommittedState(addr common.Address, hash common.Hash) common.Hash
	Snapshot() int
	RevertToSnapshot(id int)
	AddLog(*types.Log)
	AddPreimage(hash common.Hash, preimage []byte)
	ForEachStorage(addr common.Address, cb func(key, value common.Hash) bool) error
}

// EireneEngine은 합의 엔진 인터페이스를 정의합니다.
type EireneEngine interface {
	// Author는 헤더의 작성자를 반환합니다.
	Author(header *types.Header) (common.Address, error)
	
	// VerifyHeader는 헤더를 검증합니다.
	VerifyHeader(chain BlockChain, header *types.Header, seal bool) error
	
	// VerifyHeaders는 헤더 목록을 검증합니다.
	VerifyHeaders(chain BlockChain, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error)
	
	// Prepare는 합의 필드를 준비합니다.
	Prepare(chain BlockChain, header *types.Header) error
	
	// Finalize는 블록을 마무리합니다.
	Finalize(chain BlockChain, header *types.Header, state StateDB, txs []*types.Transaction, uncles []*types.Header) (*types.Block, error)
	
	// Seal은 블록을 봉인합니다.
	Seal(chain BlockChain, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error
	
	// SealHash는 봉인할 블록의 해시를 반환합니다.
	SealHash(header *types.Header) common.Hash
	
	// CalcDifficulty는 난이도를 계산합니다.
	CalcDifficulty(chain BlockChain, time uint64, parent *types.Header) *big.Int
	
	// APIs는 RPC API를 반환합니다.
	APIs(chain BlockChain) []EireneAPI
	
	// Close는 엔진을 닫습니다.
	Close() error
}

// EireneAPI는 RPC API를 나타냅니다.
type EireneAPI struct {
	Namespace string
	Version   string
	Service   interface{}
	Public    bool
} 