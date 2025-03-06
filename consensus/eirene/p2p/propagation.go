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
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/log"
)

// 블록 전파 관련 상수
const (
	// 초기 전파 피어 수
	initialPropagationPeers = 4

	// 최대 전파 피어 수
	maxPropagationPeers = 16

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

	// 네트워크 혼잡도 측정 간격
	congestionMeasureInterval = 5 * time.Second

	// 네트워크 혼잡도 임계값
	congestionThreshold = 0.8

	// 적응형 전파 조정 간격
	adaptiveAdjustmentInterval = 30 * time.Second

	// 헤더 전파 비율 (전체 피어 중 헤더만 전파할 피어 비율)
	headerOnlyPropagationRatio = 0.7
)

// 전파 모드
const (
	PropagationModeNormal       = 0 // 일반 전파 모드
	PropagationModeAggressive   = 1 // 적극적 전파 모드 (중요 블록)
	PropagationModeConservative = 2 // 보수적 전파 모드 (네트워크 혼잡 시)
)

// BlockPropagator는 블록 전파 알고리즘을 구현합니다.
type BlockPropagator struct {
	peerSet      *PeerSet                    // 피어 집합
	validatorSet utils.ValidatorSetInterface // 검증자 집합

	// 블록 전파 상태 추적
	propagatedBlocks map[common.Hash]*propagationState // 전파된 블록 맵

	// 블록 해시 캐시
	blockHashCache *lruCache

	// 통계
	totalPropagated  uint64          // 총 전파된 블록 수
	totalPeers       uint64          // 총 전파된 피어 수
	propagationTimes []time.Duration // 전파 시간 통계

	// 네트워크 상태
	networkCongestion float64 // 네트워크 혼잡도 (0-1)
	propagationMode   int     // 전파 모드
	adaptivePeerCount int     // 적응형 피어 수

	quit chan struct{}  // 종료 채널
	wg   sync.WaitGroup // 대기 그룹

	lock sync.RWMutex // 동시성 제어를 위한 락

	logger log.Logger // 로거
}

// propagationState는 블록 전파 상태를 나타냅니다.
type propagationState struct {
	block           *types.Block    // 블록
	header          *types.Header   // 블록 헤더
	td              *big.Int        // 총 난이도
	peers           map[string]bool // 전파된 피어 맵
	headerOnlyPeers map[string]bool // 헤더만 전파된 피어 맵
	startTime       time.Time       // 전파 시작 시간
	endTime         time.Time       // 전파 완료 시간
	completed       bool            // 전파 완료 여부
	isUrgent        bool            // 긴급 전파 여부
}

// lruCache는 LRU 캐시를 구현합니다.
type lruCache struct {
	capacity int
	items    map[interface{}]interface{}
	order    []interface{}
	lock     sync.RWMutex
}

// NewBlockPropagator는 새로운 블록 전파기를 생성합니다.
func NewBlockPropagator(peerSet *PeerSet, validatorSet utils.ValidatorSetInterface) *BlockPropagator {
	return &BlockPropagator{
		peerSet:           peerSet,
		validatorSet:      validatorSet,
		propagatedBlocks:  make(map[common.Hash]*propagationState),
		blockHashCache:    newLRUCache(blockHashCacheSize),
		propagationTimes:  make([]time.Duration, 0, 100),
		quit:              make(chan struct{}),
		logger:            log.New("module", "p2p/propagator"),
		propagationMode:   PropagationModeNormal,
		adaptivePeerCount: initialPropagationPeers,
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

	// 이미 존재하는 키인 경우 순서 업데이트
	if _, exists := c.items[key]; exists {
		// 기존 항목 제거
		for i, k := range c.order {
			if k == key {
				c.order = append(c.order[:i], c.order[i+1:]...)
				break
			}
		}
	} else if len(c.order) >= c.capacity {
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

	value, found := c.items[key]
	return value, found
}

// Contains는 캐시에 항목이 있는지 확인합니다.
func (c *lruCache) Contains(key interface{}) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	_, found := c.items[key]
	return found
}

// Start는 블록 전파기를 시작합니다.
func (bp *BlockPropagator) Start() {
	bp.wg.Add(2)
	go bp.cleanupLoop()
	go bp.networkMonitorLoop()
	bp.logger.Info("Block propagator started")
}

// Stop은 블록 전파기를 중지합니다.
func (bp *BlockPropagator) Stop() {
	close(bp.quit)
	bp.wg.Wait()
	bp.logger.Info("Block propagator stopped")
}

// cleanupLoop는 오래된 전파 상태를 정리하는 루프입니다.
func (bp *BlockPropagator) cleanupLoop() {
	defer bp.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-bp.quit:
			return
		case <-ticker.C:
			bp.cleanup()
		}
	}
}

// cleanup은 오래된 전파 상태를 정리합니다.
func (bp *BlockPropagator) cleanup() {
	bp.lock.Lock()
	defer bp.lock.Unlock()

	now := time.Now()
	for hash, state := range bp.propagatedBlocks {
		// 완료된 전파 상태 중 1시간 이상 지난 것 제거
		if state.completed && now.Sub(state.endTime) > time.Hour {
			delete(bp.propagatedBlocks, hash)
		}

		// 미완료 전파 상태 중 1일 이상 지난 것 제거 (비정상 상태)
		if !state.completed && now.Sub(state.startTime) > 24*time.Hour {
			delete(bp.propagatedBlocks, hash)
		}
	}
}

// networkMonitorLoop는 네트워크 상태를 모니터링하는 루프입니다.
func (bp *BlockPropagator) networkMonitorLoop() {
	defer bp.wg.Done()

	congestionTicker := time.NewTicker(congestionMeasureInterval)
	adaptiveTicker := time.NewTicker(adaptiveAdjustmentInterval)

	for {
		select {
		case <-bp.quit:
			congestionTicker.Stop()
			adaptiveTicker.Stop()
			return
		case <-congestionTicker.C:
			bp.measureNetworkCongestion()
		case <-adaptiveTicker.C:
			bp.adjustPropagationStrategy()
		}
	}
}

// measureNetworkCongestion은 네트워크 혼잡도를 측정합니다.
func (bp *BlockPropagator) measureNetworkCongestion() {
	// 피어 응답 시간, 대기 중인 요청 수 등을 기반으로 혼잡도 계산
	// 여기서는 간단한 구현으로 피어 수와 활성 전파 수만 사용

	peers := bp.peerSet.AllPeers()
	peerCount := len(peers)

	bp.lock.RLock()
	activePropagations := 0
	for _, state := range bp.propagatedBlocks {
		if !state.completed {
			activePropagations++
		}
	}
	bp.lock.RUnlock()

	// 혼잡도 계산 (피어당 활성 전파 수 기준)
	var congestion float64
	if peerCount > 0 {
		congestion = float64(activePropagations) / float64(peerCount)
		if congestion > 1.0 {
			congestion = 1.0
		}
	}

	bp.lock.Lock()
	bp.networkCongestion = congestion
	bp.lock.Unlock()

	bp.logger.Debug("Network congestion measured",
		"congestion", congestion,
		"active_propagations", activePropagations,
		"peer_count", peerCount)

	// 혼잡도에 따라 전파 모드 조정
	if congestion > congestionThreshold {
		bp.setPropagationMode(PropagationModeConservative)
	} else {
		bp.setPropagationMode(PropagationModeNormal)
	}
}

// adjustPropagationStrategy는 전파 전략을 조정합니다.
func (bp *BlockPropagator) adjustPropagationStrategy() {
	bp.lock.Lock()
	defer bp.lock.Unlock()

	// 최근 전파 성능을 기반으로 피어 수 조정
	var totalTime time.Duration
	recentTimes := bp.propagationTimes
	if len(recentTimes) > 20 {
		recentTimes = recentTimes[len(recentTimes)-20:]
	}

	if len(recentTimes) == 0 {
		return
	}

	for _, t := range recentTimes {
		totalTime += t
	}

	avgTime := totalTime / time.Duration(len(recentTimes))

	// 평균 전파 시간이 너무 길면 피어 수 증가
	if avgTime > 500*time.Millisecond && bp.adaptivePeerCount < maxPropagationPeers {
		bp.adaptivePeerCount++
		bp.logger.Debug("Increased propagation peer count", "count", bp.adaptivePeerCount, "avg_time", avgTime)
	}

	// 평균 전파 시간이 충분히 짧으면 피어 수 감소
	if avgTime < 100*time.Millisecond && bp.adaptivePeerCount > initialPropagationPeers {
		bp.adaptivePeerCount--
		bp.logger.Debug("Decreased propagation peer count", "count", bp.adaptivePeerCount, "avg_time", avgTime)
	}
}

// setPropagationMode는 전파 모드를 설정합니다.
func (bp *BlockPropagator) setPropagationMode(mode int) {
	bp.lock.Lock()
	defer bp.lock.Unlock()

	if bp.propagationMode != mode {
		bp.propagationMode = mode
		bp.logger.Info("Propagation mode changed", "mode", mode)
	}
}

// markPropagationCompleted는 블록 전파를 완료로 표시합니다.
func (bp *BlockPropagator) markPropagationCompleted(hash common.Hash) {
	bp.lock.Lock()
	defer bp.lock.Unlock()

	state, exists := bp.propagatedBlocks[hash]
	if !exists {
		return
	}

	if !state.completed {
		state.completed = true
		state.endTime = time.Now()

		// 전파 시간 통계 업데이트
		propagationTime := state.endTime.Sub(state.startTime)
		bp.propagationTimes = append(bp.propagationTimes, propagationTime)
		if len(bp.propagationTimes) > 100 {
			bp.propagationTimes = bp.propagationTimes[1:]
		}

		bp.logger.Debug("Block propagation completed",
			"hash", hash.Hex(),
			"time", propagationTime,
			"peers", len(state.peers),
			"header_only_peers", len(state.headerOnlyPeers))
	}
}

// PropagateBlock은 블록을 네트워크에 전파합니다.
func (bp *BlockPropagator) PropagateBlock(block *types.Block, td *big.Int) {
	hash := block.Hash()

	// 이미 전파 중인 블록인지 확인
	bp.lock.Lock()
	if _, exists := bp.propagatedBlocks[hash]; exists {
		bp.lock.Unlock()
		return
	}

	// 블록 해시 캐시에 있는지 확인
	if bp.blockHashCache.Contains(hash) {
		bp.lock.Unlock()
		return
	}

	// 블록 해시 캐시에 추가
	bp.blockHashCache.Add(hash, struct{}{})

	// 블록 높이에 따라 긴급 여부 결정
	isUrgent := block.NumberU64()%100 == 0 // 100 블록마다 긴급 처리

	// 전파 상태 생성
	state := &propagationState{
		block:           block,
		header:          block.Header(),
		td:              td,
		peers:           make(map[string]bool),
		headerOnlyPeers: make(map[string]bool),
		startTime:       time.Now(),
		completed:       false,
		isUrgent:        isUrgent,
	}

	bp.propagatedBlocks[hash] = state
	bp.lock.Unlock()

	// 긴급 블록이면 적극적 전파 모드로 설정
	if isUrgent {
		bp.setPropagationMode(PropagationModeAggressive)
	}

	// 초기 피어에게 전파
	bp.propagateToInitialPeers(hash)

	// 추가 피어에게 전파 (고루틴으로 실행)
	go bp.propagateToAdditionalPeers(hash)

	bp.logger.Debug("Started block propagation", "hash", hash.Hex(), "number", block.NumberU64(), "urgent", isUrgent)
}

// propagateToInitialPeers는 초기 피어 집합에게 블록을 전파합니다.
func (bp *BlockPropagator) propagateToInitialPeers(hash common.Hash) {
	bp.lock.RLock()
	state, exists := bp.propagatedBlocks[hash]
	if !exists {
		bp.lock.RUnlock()
		return
	}

	// 전파 모드에 따라 피어 수 결정
	peerCount := bp.adaptivePeerCount
	switch bp.propagationMode {
	case PropagationModeAggressive:
		peerCount = maxPropagationPeers
	case PropagationModeConservative:
		peerCount = initialPropagationPeers / 2
		if peerCount < 2 {
			peerCount = 2
		}
	}
	bp.lock.RUnlock()

	// 모든 피어 가져오기
	peers := bp.peerSet.AllPeers()
	if len(peers) == 0 {
		return
	}

	// 검증자 노드 우선 선택
	var validatorPeers []*Peer
	var normalPeers []*Peer

	for _, peer := range peers {
		if bp.isValidatorNode(peer) {
			validatorPeers = append(validatorPeers, peer)
		} else {
			normalPeers = append(normalPeers, peer)
		}
	}

	// 선택할 피어 수 결정
	validatorCount := len(validatorPeers)
	remainingCount := peerCount - validatorCount

	// 검증자 노드가 충분하지 않으면 일반 노드로 보충
	selectedPeers := make([]*Peer, 0, peerCount)

	// 검증자 노드 추가
	if validatorCount > 0 {
		if validatorCount <= peerCount {
			selectedPeers = append(selectedPeers, validatorPeers...)
		} else {
			// 검증자가 너무 많으면 무작위로 선택
			rand.Shuffle(validatorCount, func(i, j int) {
				validatorPeers[i], validatorPeers[j] = validatorPeers[j], validatorPeers[i]
			})
			selectedPeers = append(selectedPeers, validatorPeers[:peerCount]...)
			remainingCount = 0
		}
	}

	// 일반 노드 추가
	if remainingCount > 0 && len(normalPeers) > 0 {
		rand.Shuffle(len(normalPeers), func(i, j int) {
			normalPeers[i], normalPeers[j] = normalPeers[j], normalPeers[i]
		})

		count := remainingCount
		if count > len(normalPeers) {
			count = len(normalPeers)
		}

		selectedPeers = append(selectedPeers, normalPeers[:count]...)
	}

	// 선택된 피어에게 블록 전파
	for i, peer := range selectedPeers {
		// 헤더만 전파할지 결정 (검증자는 항상 전체 블록 전파)
		headerOnly := !bp.isValidatorNode(peer) && float64(i)/float64(len(selectedPeers)) < headerOnlyPropagationRatio

		// 전파 지연 계산
		delay := time.Duration(rand.Intn(maxPropagationDelay-minPropagationDelay)+minPropagationDelay) * time.Millisecond

		// 검증자 노드는 지연 감소
		if bp.isValidatorNode(peer) {
			delay = delay / 2
		}

		// 긴급 블록은 지연 감소
		if state.isUrgent {
			delay = delay / 2
		}

		go func(p *Peer, d time.Duration, headerOnly bool) {
			// 지연 적용
			time.Sleep(d)

			// 블록 전파
			bp.propagateBlockToPeer(hash, p, headerOnly)
		}(peer, delay, headerOnly)
	}
}

// isValidatorNode는 피어가 검증자 노드인지 확인합니다.
func (bp *BlockPropagator) isValidatorNode(peer *Peer) bool {
	if bp.validatorSet == nil {
		return false
	}

	// 피어 ID를 주소로 변환 (실제 구현에서는 피어의 노드 ID를 주소로 변환하는 로직 필요)
	// 여기서는 임의의 주소 사용
	addr := common.Address{}

	// 검증자 집합에 포함되어 있는지 확인
	return bp.validatorSet.Contains(addr)
}

// propagateBlockToPeer는 특정 피어에게 블록을 전파합니다.
func (bp *BlockPropagator) propagateBlockToPeer(hash common.Hash, peer *Peer, headerOnly bool) {
	bp.lock.Lock()
	state, exists := bp.propagatedBlocks[hash]
	if !exists {
		bp.lock.Unlock()
		return
	}

	// 이미 전파된 피어인지 확인
	peerID := peer.ID().String()
	if state.peers[peerID] || state.headerOnlyPeers[peerID] {
		bp.lock.Unlock()
		return
	}

	// 전파 상태 업데이트
	if headerOnly {
		state.headerOnlyPeers[peerID] = true
	} else {
		state.peers[peerID] = true
	}
	bp.lock.Unlock()

	// 블록 또는 헤더 전송
	var err error
	if headerOnly {
		// 헤더만 전송 (실제 구현에서는 헤더 전송 메서드 필요)
		// 여기서는 임시로 블록 전송 메서드 사용
		err = peer.SendNewBlock(state.block, state.td)
	} else {
		err = peer.SendNewBlock(state.block, state.td)
	}

	if err != nil {
		bp.logger.Debug("Failed to propagate block", "peer", peer.ID().String(), "hash", hash.Hex(), "error", err)

		// 전파 실패 시 상태 업데이트
		bp.lock.Lock()
		if headerOnly {
			delete(state.headerOnlyPeers, peerID)
		} else {
			delete(state.peers, peerID)
		}
		bp.lock.Unlock()
	} else {
		bp.logger.Trace("Propagated block", "peer", peer.ID().String(), "hash", hash.Hex(), "header_only", headerOnly)
	}
}

// propagateToAdditionalPeers는 추가 피어에게 블록을 전파합니다.
func (bp *BlockPropagator) propagateToAdditionalPeers(hash common.Hash) {
	// 전파 완료 대기
	timer := time.NewTimer(propagationTimeout)
	defer timer.Stop()

	// 추가 전파 간격
	ticker := time.NewTicker(propagationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-bp.quit:
			return
		case <-timer.C:
			// 타임아웃 - 전파 완료 처리
			bp.markPropagationCompleted(hash)
			return
		case <-ticker.C:
			// 추가 피어에게 전파
			completed := bp.propagateToMorePeers(hash)
			if completed {
				bp.markPropagationCompleted(hash)
				return
			}
		}
	}
}

// propagateToMorePeers는 더 많은 피어에게 블록을 전파합니다.
func (bp *BlockPropagator) propagateToMorePeers(hash common.Hash) bool {
	bp.lock.Lock()
	state, exists := bp.propagatedBlocks[hash]
	if !exists || state.completed {
		bp.lock.Unlock()
		return true
	}

	// 이미 전파된 피어 수 확인
	propagatedCount := len(state.peers) + len(state.headerOnlyPeers)

	// 전파 모드에 따라 목표 피어 수 결정
	targetPeerCount := bp.adaptivePeerCount * 2
	switch bp.propagationMode {
	case PropagationModeAggressive:
		targetPeerCount = maxPropagationPeers * 2
	case PropagationModeConservative:
		targetPeerCount = bp.adaptivePeerCount
	}

	// 모든 피어에게 전파 완료 확인
	allPeers := bp.peerSet.AllPeers()
	if propagatedCount >= len(allPeers) || propagatedCount >= targetPeerCount {
		state.completed = true
		bp.lock.Unlock()
		return true
	}
	bp.lock.Unlock()

	// 아직 전파되지 않은 피어 선택
	var candidates []*Peer
	for _, peer := range allPeers {
		bp.lock.RLock()
		_, propagated := state.peers[peer.ID().String()]
		_, headerOnly := state.headerOnlyPeers[peer.ID().String()]
		bp.lock.RUnlock()

		if !propagated && !headerOnly {
			candidates = append(candidates, peer)
		}
	}

	if len(candidates) == 0 {
		return false
	}

	// 추가로 전파할 피어 수 결정
	additionalCount := targetPeerCount - propagatedCount
	if additionalCount > len(candidates) {
		additionalCount = len(candidates)
	}

	// 무작위로 피어 선택
	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	// 선택된 피어에게 전파
	for i := 0; i < additionalCount; i++ {
		peer := candidates[i]

		// 헤더만 전파할지 결정
		headerOnly := float64(i)/float64(additionalCount) < headerOnlyPropagationRatio

		// 전파 지연 계산
		delay := time.Duration(rand.Intn(maxPropagationDelay-minPropagationDelay)+minPropagationDelay) * time.Millisecond

		go func(p *Peer, d time.Duration, headerOnly bool) {
			time.Sleep(d)
			bp.propagateBlockToPeer(hash, p, headerOnly)
		}(peer, delay, headerOnly)
	}

	return false
}

// GetPropagationStats는 전파 통계를 반환합니다.
func (bp *BlockPropagator) GetPropagationStats() map[string]interface{} {
	bp.lock.RLock()
	defer bp.lock.RUnlock()

	// 활성 전파 수 계산
	var activePropagations int
	for _, state := range bp.propagatedBlocks {
		if !state.completed {
			activePropagations++
		}
	}

	// 평균 전파 시간 계산
	var totalTime time.Duration
	for _, t := range bp.propagationTimes {
		totalTime += t
	}

	var avgTime time.Duration
	if len(bp.propagationTimes) > 0 {
		avgTime = totalTime / time.Duration(len(bp.propagationTimes))
	}

	return map[string]interface{}{
		"total_propagated":     bp.totalPropagated,
		"total_peers":          bp.totalPeers,
		"avg_propagation_time": avgTime.String(),
		"active_propagations":  activePropagations,
		"network_congestion":   bp.networkCongestion,
		"propagation_mode":     bp.propagationMode,
		"adaptive_peer_count":  bp.adaptivePeerCount,
	}
}
