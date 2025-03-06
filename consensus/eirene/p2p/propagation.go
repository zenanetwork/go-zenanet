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

package p2p

import (
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/log"
)

// 블록 전파 관련 상수
const (
	// 초기 전파 피어 수
	initialPropagationPeers = 4
	
	// 최대 전파 피어 수
	maxPropagationPeers = 8
	
	// 전파 간격
	propagationInterval = 100 * time.Millisecond
	
	// 전파 타임아웃
	propagationTimeout = 10 * time.Second
	
	// 블록 전파 지연 (밀리초)
	minPropagationDelay = 50
	maxPropagationDelay = 200
	
	// 블록 해시 캐시 크기
	blockHashCacheSize = 1024
	
	// 블록 전파 로그 간격
	propagationLogInterval = 100
)

// BlockPropagator는 블록 전파 알고리즘을 구현합니다.
type BlockPropagator struct {
	peerSet     *PeerSet           // 피어 집합
	
	// 블록 전파 상태 추적
	propagatedBlocks map[common.Hash]*propagationState // 전파된 블록 맵
	
	// 블록 해시 캐시
	blockHashCache *lruCache
	
	// 통계
	totalPropagated  uint64         // 총 전파된 블록 수
	totalPeers       uint64         // 총 전파된 피어 수
	propagationTimes []time.Duration // 전파 시간 통계
	
	quit       chan struct{}      // 종료 채널
	wg         sync.WaitGroup     // 대기 그룹
	
	lock       sync.RWMutex       // 동시성 제어를 위한 락
	
	logger     log.Logger         // 로거
}

// propagationState는 블록 전파 상태를 나타냅니다.
type propagationState struct {
	block      *types.Block       // 블록
	td         *big.Int           // 총 난이도
	peers      map[string]bool    // 전파된 피어 맵
	startTime  time.Time          // 전파 시작 시간
	endTime    time.Time          // 전파 완료 시간
	completed  bool               // 전파 완료 여부
}

// lruCache는 LRU 캐시를 구현합니다.
type lruCache struct {
	capacity int
	items    map[interface{}]interface{}
	order    []interface{}
	lock     sync.RWMutex
}

// NewBlockPropagator는 새로운 블록 전파기를 생성합니다.
func NewBlockPropagator(peerSet *PeerSet) *BlockPropagator {
	return &BlockPropagator{
		peerSet:          peerSet,
		propagatedBlocks: make(map[common.Hash]*propagationState),
		blockHashCache:   newLRUCache(blockHashCacheSize),
		propagationTimes: make([]time.Duration, 0, 100),
		quit:             make(chan struct{}),
		logger:           log.New("module", "eirene/p2p/propagation"),
	}
}

// newLRUCache는 새로운 LRU 캐시를 생성합니다.
func newLRUCache(capacity int) *lruCache {
	return &lruCache{
		capacity: capacity,
		items:    make(map[interface{}]interface{}),
		order:    make([]interface{}, 0, capacity),
	}
}

// Add는 캐시에 항목을 추가합니다.
func (c *lruCache) Add(key, value interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	
	// 이미 있는 항목인 경우 순서 업데이트
	if _, ok := c.items[key]; ok {
		// 기존 항목 제거
		for i, k := range c.order {
			if k == key {
				c.order = append(c.order[:i], c.order[i+1:]...)
				break
			}
		}
	} else if len(c.items) >= c.capacity {
		// 캐시가 가득 찬 경우 가장 오래된 항목 제거
		oldestKey := c.order[0]
		delete(c.items, oldestKey)
		c.order = c.order[1:]
	}
	
	// 새 항목 추가
	c.items[key] = value
	c.order = append(c.order, key)
}

// Get은 캐시에서 항목을 가져옵니다.
func (c *lruCache) Get(key interface{}) (interface{}, bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	
	value, ok := c.items[key]
	return value, ok
}

// Contains는 캐시에 항목이 있는지 확인합니다.
func (c *lruCache) Contains(key interface{}) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	
	_, ok := c.items[key]
	return ok
}

// Start는 블록 전파기를 시작합니다.
func (bp *BlockPropagator) Start() {
	bp.logger.Info("Starting block propagator")
	
	bp.wg.Add(1)
	go bp.cleanupLoop()
}

// Stop은 블록 전파기를 중지합니다.
func (bp *BlockPropagator) Stop() {
	bp.logger.Info("Stopping block propagator")
	close(bp.quit)
	bp.wg.Wait()
}

// cleanupLoop는 주기적으로 오래된 전파 상태를 정리합니다.
func (bp *BlockPropagator) cleanupLoop() {
	defer bp.wg.Done()
	
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			bp.cleanup()
		case <-bp.quit:
			return
		}
	}
}

// cleanup은 오래된 전파 상태를 정리합니다.
func (bp *BlockPropagator) cleanup() {
	bp.lock.Lock()
	defer bp.lock.Unlock()
	
	now := time.Now()
	
	// 오래된 전파 상태 제거
	for hash, state := range bp.propagatedBlocks {
		// 완료된 전파 상태이고 일정 시간이 지난 경우 제거
		if state.completed && now.Sub(state.endTime) > time.Hour {
			delete(bp.propagatedBlocks, hash)
		}
		
		// 시작했지만 완료되지 않은 전파 상태가 타임아웃된 경우 제거
		if !state.completed && now.Sub(state.startTime) > propagationTimeout {
			delete(bp.propagatedBlocks, hash)
		}
	}
}

// PropagateBlock은 블록을 전파합니다.
func (bp *BlockPropagator) PropagateBlock(block *types.Block, td *big.Int) {
	hash := block.Hash()
	
	// 이미 전파 중인 블록인지 확인
	bp.lock.Lock()
	if _, ok := bp.propagatedBlocks[hash]; ok {
		bp.lock.Unlock()
		return
	}
	
	// 이미 전파된 블록인지 확인
	if bp.blockHashCache.Contains(hash) {
		bp.lock.Unlock()
		return
	}
	
	// 블록 해시 캐시에 추가
	bp.blockHashCache.Add(hash, struct{}{})
	
	// 새 전파 상태 생성
	state := &propagationState{
		block:     block,
		td:        td,
		peers:     make(map[string]bool),
		startTime: time.Now(),
	}
	bp.propagatedBlocks[hash] = state
	bp.lock.Unlock()
	
	// 로그
	bp.logger.Debug("Starting block propagation", "hash", hash.Hex(), "number", block.NumberU64())
	
	// 초기 전파
	bp.propagateToInitialPeers(hash)
	
	// 통계 업데이트
	bp.totalPropagated++
	if bp.totalPropagated%propagationLogInterval == 0 {
		bp.logPropagationStats()
	}
}

// propagateToInitialPeers는 초기 피어들에게 블록을 전파합니다.
func (bp *BlockPropagator) propagateToInitialPeers(hash common.Hash) {
	// 전파 상태 가져오기
	bp.lock.RLock()
	state, ok := bp.propagatedBlocks[hash]
	bp.lock.RUnlock()
	
	if !ok {
		return
	}
	
	// 모든 피어 가져오기
	peers := bp.peerSet.AllPeers()
	
	// 피어가 없으면 전파 완료 표시
	if len(peers) == 0 {
		bp.markPropagationCompleted(hash)
		return
	}
	
	// 피어 셔플
	rand.Shuffle(len(peers), func(i, j int) {
		peers[i], peers[j] = peers[j], peers[i]
	})
	
	// 초기 전파 피어 수 결정
	numPeers := initialPropagationPeers
	if numPeers > len(peers) {
		numPeers = len(peers)
	}
	
	// 선택된 피어들에게 전파
	for i := 0; i < numPeers; i++ {
		peer := peers[i]
		
		// 이미 전파된 피어인지 확인
		bp.lock.RLock()
		if state.peers[peer.ID().String()] {
			bp.lock.RUnlock()
			continue
		}
		bp.lock.RUnlock()
		
		// 전파 지연 계산
		delay := time.Duration(rand.Intn(maxPropagationDelay-minPropagationDelay)+minPropagationDelay) * time.Millisecond
		
		// 고루틴으로 전파
		go func(p *Peer, d time.Duration) {
			// 지연 적용
			time.Sleep(d)
			
			// 블록 전송
			err := p.SendNewBlock(state.block, state.td)
			
			// 전파 결과 처리
			bp.lock.Lock()
			if err == nil {
				state.peers[p.ID().String()] = true
				bp.totalPeers++
			}
			bp.lock.Unlock()
			
			// 추가 피어에게 전파
			bp.propagateToAdditionalPeers(hash)
		}(peer, delay)
	}
}

// propagateToAdditionalPeers는 추가 피어들에게 블록을 전파합니다.
func (bp *BlockPropagator) propagateToAdditionalPeers(hash common.Hash) {
	// 전파 상태 가져오기
	bp.lock.RLock()
	state, ok := bp.propagatedBlocks[hash]
	if !ok {
		bp.lock.RUnlock()
		return
	}
	
	// 이미 충분히 전파되었는지 확인
	if len(state.peers) >= maxPropagationPeers {
		bp.lock.RUnlock()
		bp.markPropagationCompleted(hash)
		return
	}
	bp.lock.RUnlock()
	
	// 모든 피어 가져오기
	allPeers := bp.peerSet.AllPeers()
	
	// 아직 전파되지 않은 피어 필터링
	var candidates []*Peer
	bp.lock.RLock()
	for _, peer := range allPeers {
		if !state.peers[peer.ID().String()] {
			candidates = append(candidates, peer)
		}
	}
	bp.lock.RUnlock()
	
	// 전파할 피어가 없으면 전파 완료 표시
	if len(candidates) == 0 {
		bp.markPropagationCompleted(hash)
		return
	}
	
	// 피어 셔플
	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})
	
	// 추가로 전파할 피어 수 결정
	bp.lock.RLock()
	numPropagated := len(state.peers)
	bp.lock.RUnlock()
	
	numAdditional := maxPropagationPeers - numPropagated
	if numAdditional > len(candidates) {
		numAdditional = len(candidates)
	}
	
	// 선택된 피어들에게 전파
	for i := 0; i < numAdditional; i++ {
		peer := candidates[i]
		
		// 전파 지연 계산
		delay := time.Duration(rand.Intn(maxPropagationDelay-minPropagationDelay)+minPropagationDelay) * time.Millisecond
		
		// 고루틴으로 전파
		go func(p *Peer, d time.Duration) {
			// 지연 적용
			time.Sleep(d)
			
			// 블록 전송
			err := p.SendNewBlock(state.block, state.td)
			
			// 전파 결과 처리
			bp.lock.Lock()
			if err == nil {
				state.peers[p.ID().String()] = true
				bp.totalPeers++
			}
			bp.lock.Unlock()
			
			// 모든 피어에게 전파되었는지 확인
			bp.lock.RLock()
			allPropagated := len(state.peers) >= maxPropagationPeers
			bp.lock.RUnlock()
			
			if allPropagated {
				bp.markPropagationCompleted(hash)
			}
		}(peer, delay)
	}
}

// markPropagationCompleted는 블록 전파가 완료되었음을 표시합니다.
func (bp *BlockPropagator) markPropagationCompleted(hash common.Hash) {
	bp.lock.Lock()
	defer bp.lock.Unlock()
	
	state, ok := bp.propagatedBlocks[hash]
	if !ok {
		return
	}
	
	// 이미 완료된 경우 무시
	if state.completed {
		return
	}
	
	// 완료 표시
	state.completed = true
	state.endTime = time.Now()
	
	// 전파 시간 통계 업데이트
	propagationTime := state.endTime.Sub(state.startTime)
	bp.propagationTimes = append(bp.propagationTimes, propagationTime)
	if len(bp.propagationTimes) > 100 {
		bp.propagationTimes = bp.propagationTimes[1:]
	}
	
	// 로그
	bp.logger.Debug("Block propagation completed", 
		"hash", hash.Hex(), 
		"number", state.block.NumberU64(),
		"peers", len(state.peers),
		"time", propagationTime)
}

// logPropagationStats는 전파 통계를 로깅합니다.
func (bp *BlockPropagator) logPropagationStats() {
	bp.lock.RLock()
	defer bp.lock.RUnlock()
	
	// 평균 전파 시간 계산
	var totalTime time.Duration
	for _, t := range bp.propagationTimes {
		totalTime += t
	}
	
	var avgTime time.Duration
	if len(bp.propagationTimes) > 0 {
		avgTime = totalTime / time.Duration(len(bp.propagationTimes))
	}
	
	// 평균 피어 수 계산
	var avgPeers float64
	if bp.totalPropagated > 0 {
		avgPeers = float64(bp.totalPeers) / float64(bp.totalPropagated)
	}
	
	bp.logger.Info("Block propagation statistics", 
		"blocks", bp.totalPropagated,
		"avg_peers", avgPeers,
		"avg_time", avgTime)
}

// GetPropagationStats는 전파 통계를 반환합니다.
func (bp *BlockPropagator) GetPropagationStats() map[string]interface{} {
	bp.lock.RLock()
	defer bp.lock.RUnlock()
	
	// 평균 전파 시간 계산
	var totalTime time.Duration
	for _, t := range bp.propagationTimes {
		totalTime += t
	}
	
	var avgTime time.Duration
	if len(bp.propagationTimes) > 0 {
		avgTime = totalTime / time.Duration(len(bp.propagationTimes))
	}
	
	// 평균 피어 수 계산
	var avgPeers float64
	if bp.totalPropagated > 0 {
		avgPeers = float64(bp.totalPeers) / float64(bp.totalPropagated)
	}
	
	// 현재 전파 중인 블록 수
	var activePropagations int
	for _, state := range bp.propagatedBlocks {
		if !state.completed {
			activePropagations++
		}
	}
	
	return map[string]interface{}{
		"total_blocks":       bp.totalPropagated,
		"total_peers":        bp.totalPeers,
		"avg_peers_per_block": avgPeers,
		"avg_propagation_time": avgTime.String(),
		"active_propagations": activePropagations,
	}
} 