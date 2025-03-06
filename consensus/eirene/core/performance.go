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
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/log"
)

// 성능 최적화 관련 상수
const (
	// 병렬 처리 관련
	defaultWorkerCount = 4  // 기본 워커 수
	minWorkerCount     = 2  // 최소 워커 수
	maxWorkerCount     = 16 // 최대 워커 수

	// 캐싱 관련
	defaultCacheSize = 1024 // 기본 캐시 크기
	minCacheSize     = 128  // 최소 캐시 크기
	maxCacheSize     = 8192 // 최대 캐시 크기

	// 성능 모니터링 관련
	perfMonitorInterval = 5 * time.Minute // 성능 모니터링 간격

	// 배치 처리 관련
	defaultBatchSize = 100  // 기본 배치 크기
	minBatchSize     = 10   // 최소 배치 크기
	maxBatchSize     = 1000 // 최대 배치 크기
)

// PerformanceOptimizer는 블록 처리 성능을 최적화합니다.
type PerformanceOptimizer struct {
	// 설정
	workerCount int // 워커 수
	cacheSize   int // 캐시 크기
	batchSize   int // 배치 크기

	// 캐시
	validatorCache *sync.Map // 검증자 캐시 (주소 -> 검증자)
	stateCache     *sync.Map // 상태 캐시 (해시 -> 상태)

	// 성능 통계
	txProcessingTimes    []time.Duration // 트랜잭션 처리 시간
	blockProcessingTimes []time.Duration // 블록 처리 시간
	batchProcessingTimes []time.Duration // 배치 처리 시간

	// 워커 풀
	workerPool chan struct{} // 워커 풀

	// 종료 채널
	quit chan struct{}  // 종료 채널
	wg   sync.WaitGroup // 대기 그룹

	// 로거
	logger log.Logger // 로거

	// 배치 처리 관련
	stateBatchProcessor *StateBatchProcessor // 상태 DB 배치 처리기

	// 프로파일링 관련
	profiler          *PerformanceProfiler // 성능 프로파일러
	lastProfilingTime time.Time            // 마지막 프로파일링 시간
	highLoadDetected  bool                 // 높은 부하 감지 여부
}

// NewPerformanceOptimizer는 새로운 성능 최적화기를 생성합니다.
func NewPerformanceOptimizer() *PerformanceOptimizer {
	// 워커 수 결정 (CPU 코어 수 기반)
	workerCount := runtime.NumCPU()
	if workerCount < minWorkerCount {
		workerCount = minWorkerCount
	} else if workerCount > maxWorkerCount {
		workerCount = maxWorkerCount
	}

	// 성능 프로파일러 생성
	profiler := NewPerformanceProfiler()

	// 성능 최적화기 생성
	po := &PerformanceOptimizer{
		workerCount:          workerCount,
		cacheSize:            defaultCacheSize,
		batchSize:            defaultBatchSize,
		validatorCache:       &sync.Map{},
		stateCache:           &sync.Map{},
		txProcessingTimes:    make([]time.Duration, 0),
		blockProcessingTimes: make([]time.Duration, 0),
		batchProcessingTimes: make([]time.Duration, 0),
		workerPool:           make(chan struct{}, workerCount),
		quit:                 make(chan struct{}),
		logger:               log.New("module", "performance"),
		profiler:             profiler,
		lastProfilingTime:    time.Now(),
		highLoadDetected:     false,
	}

	// 상태 배치 처리기 생성
	po.stateBatchProcessor = NewStateBatchProcessor(po.batchSize)

	return po
}

// Start는 성능 최적화기를 시작합니다.
func (po *PerformanceOptimizer) Start() {
	po.logger.Info("Starting performance optimizer", "workers", po.workerCount, "cache_size", po.cacheSize, "batch_size", po.batchSize)

	// 워커 풀 초기화
	for i := 0; i < po.workerCount; i++ {
		po.workerPool <- struct{}{}
	}

	// 성능 모니터링 시작
	po.wg.Add(1)
	go po.monitorPerformance()

	// 성능 프로파일러 시작
	if err := po.profiler.Start(); err != nil {
		po.logger.Error("Failed to start performance profiler", "error", err)
	}
}

// Stop은 성능 최적화기를 중지합니다.
func (po *PerformanceOptimizer) Stop() {
	po.logger.Info("Stopping performance optimizer")

	// 성능 프로파일러 중지
	po.profiler.Stop()

	close(po.quit)
	po.wg.Wait()
}

// monitorPerformance는 성능을 모니터링합니다.
func (po *PerformanceOptimizer) monitorPerformance() {
	defer po.wg.Done()

	ticker := time.NewTicker(perfMonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			po.analyzePerformance()
		case <-po.quit:
			return
		}
	}
}

// analyzePerformance는 성능을 분석하고 최적화합니다.
func (po *PerformanceOptimizer) analyzePerformance() {
	// 메모리 사용량 분석
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	// 트랜잭션 처리 시간 분석
	var avgTxTime time.Duration
	if len(po.txProcessingTimes) > 0 {
		var total time.Duration
		for _, t := range po.txProcessingTimes {
			total += t
		}
		avgTxTime = total / time.Duration(len(po.txProcessingTimes))
	}

	// 블록 처리 시간 분석
	var avgBlockTime time.Duration
	if len(po.blockProcessingTimes) > 0 {
		var total time.Duration
		for _, t := range po.blockProcessingTimes {
			total += t
		}
		avgBlockTime = total / time.Duration(len(po.blockProcessingTimes))
	}

	// 배치 처리 시간 분석
	var avgBatchTime time.Duration
	if len(po.batchProcessingTimes) > 0 {
		var total time.Duration
		for _, t := range po.batchProcessingTimes {
			total += t
		}
		avgBatchTime = total / time.Duration(len(po.batchProcessingTimes))
	}

	// 워커 수 조정
	if avgBlockTime > 500*time.Millisecond && po.workerCount < maxWorkerCount {
		// 블록 처리가 느리면 워커 수 증가
		po.increaseWorkers()
	} else if avgBlockTime < 100*time.Millisecond && po.workerCount > minWorkerCount {
		// 블록 처리가 빠르면 워커 수 감소
		po.decreaseWorkers()
	}

	// 캐시 크기 조정
	cacheHitRate := po.calculateCacheHitRate()
	if cacheHitRate < 0.5 && po.cacheSize < maxCacheSize {
		// 캐시 적중률이 낮으면 캐시 크기 증가
		po.increaseCacheSize()
	} else if cacheHitRate > 0.9 && po.cacheSize > minCacheSize {
		// 캐시 적중률이 높으면 캐시 크기 감소
		po.decreaseCacheSize()
	}

	// 배치 크기 조정
	if avgBatchTime > 50*time.Millisecond && po.batchSize > minBatchSize {
		// 배치 처리가 느리면 배치 크기 감소
		po.decreaseBatchSize()
	} else if avgBatchTime < 10*time.Millisecond && po.batchSize < maxBatchSize {
		// 배치 처리가 빠르면 배치 크기 증가
		po.increaseBatchSize()
	}

	// 높은 부하 감지
	highLoad := false
	if avgBlockTime > 1*time.Second || memStats.Alloc > 1024*1024*1024 {
		highLoad = true
		po.highLoadDetected = true
	} else {
		po.highLoadDetected = false
	}

	// 메모리 사용량이 높으면 GC 유도 및 캐시 정리
	if memStats.Alloc > 1024*1024*1024 { // 1GB 이상 사용 중이면
		po.logger.Info("High memory usage detected, triggering GC and cache cleanup")
		runtime.GC()
		po.cleanupCache()
	}

	// 높은 부하가 감지되고 마지막 프로파일링으로부터 충분한 시간이 지났으면 수동 프로파일링 실행
	if highLoad && time.Since(po.lastProfilingTime) > 10*time.Minute {
		po.logger.Info("High load detected, triggering manual profiling")
		if err := po.profiler.RunManualProfiling(); err != nil {
			po.logger.Error("Failed to run manual profiling", "error", err)
		} else {
			po.lastProfilingTime = time.Now()
		}
	}

	// 병목 지점 분석
	bottlenecks := po.profiler.GetBottlenecks()
	if len(bottlenecks) > 0 {
		po.logger.Info("Current bottlenecks", "count", len(bottlenecks))
		for i, b := range bottlenecks {
			if i >= 3 { // 상위 3개만 로깅
				break
			}
			po.logger.Info(fmt.Sprintf("Bottleneck #%d", i+1),
				"function", filepath.Base(b.Function),
				"cpu", fmt.Sprintf("%.1f%%", b.CPUPercent),
				"score", fmt.Sprintf("%.1f", b.Score))
		}
	}

	// 로그 출력
	po.logger.Info("Performance analysis",
		"mem_alloc", memStats.Alloc/1024/1024,
		"mem_sys", memStats.Sys/1024/1024,
		"avg_tx_time", avgTxTime,
		"avg_block_time", avgBlockTime,
		"avg_batch_time", avgBatchTime,
		"cache_hit_rate", cacheHitRate,
		"validator_cache_hit_rate", validatorCacheStats.hitRate(),
		"state_cache_hit_rate", stateCacheStats.hitRate(),
		"workers", po.workerCount,
		"cache_size", po.cacheSize,
		"batch_size", po.batchSize,
		"high_load", highLoad)
}

// calculateCacheHitRate는 캐시 적중률을 계산합니다.
func (po *PerformanceOptimizer) calculateCacheHitRate() float64 {
	validatorHitRate := validatorCacheStats.hitRate()
	stateHitRate := stateCacheStats.hitRate()

	// 두 캐시의 평균 적중률 계산
	avgHitRate := (validatorHitRate + stateHitRate) / 2.0

	// 통계 초기화 (주기적으로 초기화하여 최근 경향 반영)
	if len(po.blockProcessingTimes)%20 == 0 {
		validatorCacheStats.resetStats()
		stateCacheStats.resetStats()
	}

	return avgHitRate
}

// increaseWorkers는 워커 수를 증가시킵니다.
func (po *PerformanceOptimizer) increaseWorkers() {
	if po.workerCount >= maxWorkerCount {
		return
	}

	po.workerCount++
	po.workerPool <- struct{}{}

	po.logger.Info("Increased worker count", "workers", po.workerCount)
}

// decreaseWorkers는 워커 수를 감소시킵니다.
func (po *PerformanceOptimizer) decreaseWorkers() {
	if po.workerCount <= minWorkerCount {
		return
	}

	po.workerCount--
	<-po.workerPool

	po.logger.Info("Decreased worker count", "workers", po.workerCount)
}

// SetWorkerCount는 워커 수를 설정합니다.
func (po *PerformanceOptimizer) SetWorkerCount(count int) {
	// 유효한 범위로 조정
	if count < minWorkerCount {
		count = minWorkerCount
	} else if count > maxWorkerCount {
		count = maxWorkerCount
	}

	// 현재 워커 수와 같으면 변경 없음
	if po.workerCount == count {
		return
	}

	// 워커 풀 재생성
	oldCount := po.workerCount
	po.workerCount = count

	// 워커 풀 크기 조정
	if len(po.workerPool) > 0 {
		// 기존 워커 풀 비우기
		for len(po.workerPool) > 0 {
			<-po.workerPool
		}
	}

	// 새 워커 풀 생성
	po.workerPool = make(chan struct{}, count)
	for i := 0; i < count; i++ {
		po.workerPool <- struct{}{}
	}

	po.logger.Info("Worker count set", "old_count", oldCount, "new_count", count)
}

// increaseCacheSize는 캐시 크기를 증가시킵니다.
func (po *PerformanceOptimizer) increaseCacheSize() {
	if po.cacheSize >= maxCacheSize {
		return
	}

	po.cacheSize *= 2
	if po.cacheSize > maxCacheSize {
		po.cacheSize = maxCacheSize
	}

	po.logger.Info("Increased cache size", "cache_size", po.cacheSize)
}

// decreaseCacheSize는 캐시 크기를 감소시킵니다.
func (po *PerformanceOptimizer) decreaseCacheSize() {
	if po.cacheSize <= minCacheSize {
		return
	}

	po.cacheSize /= 2
	if po.cacheSize < minCacheSize {
		po.cacheSize = minCacheSize
	}

	po.logger.Info("Decreased cache size", "cache_size", po.cacheSize)
}

// increaseBatchSize는 배치 크기를 증가시킵니다.
func (po *PerformanceOptimizer) increaseBatchSize() {
	if po.batchSize >= maxBatchSize {
		return
	}

	oldBatchSize := po.batchSize
	po.batchSize = min(po.batchSize*2, maxBatchSize)
	po.stateBatchProcessor.SetBatchSize(po.batchSize)

	po.logger.Info("Increased batch size", "old_size", oldBatchSize, "new_size", po.batchSize)
}

// decreaseBatchSize는 배치 크기를 감소시킵니다.
func (po *PerformanceOptimizer) decreaseBatchSize() {
	if po.batchSize <= minBatchSize {
		return
	}

	oldBatchSize := po.batchSize
	po.batchSize = max(po.batchSize/2, minBatchSize)
	po.stateBatchProcessor.SetBatchSize(po.batchSize)

	po.logger.Info("Decreased batch size", "old_size", oldBatchSize, "new_size", po.batchSize)
}

// min은 두 정수 중 작은 값을 반환합니다.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max는 두 정수 중 큰 값을 반환합니다.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ProcessTransactionsParallel은 트랜잭션을 병렬로 처리합니다.
func (po *PerformanceOptimizer) ProcessTransactionsParallel(txs []*types.Transaction, state *state.StateDB) error {
	if len(txs) == 0 {
		return nil
	}

	if state == nil {
		return utils.ErrInvalidParameter
	}

	// 트랜잭션 처리 시작 시간
	startTime := time.Now()

	// 트랜잭션 의존성 그래프 구축
	graph, err := po.buildTxDependencyGraph(txs)
	if err != nil {
		return utils.WrapError(err, "failed to build transaction dependency graph")
	}

	// 결과 채널
	results := make(chan error, len(txs))

	// 처리 완료된 트랜잭션 수
	processedCount := 0

	// 상태 DB 배치 처리기 초기화
	if err := po.stateBatchProcessor.Reset(state); err != nil {
		return utils.WrapError(err, "failed to reset state batch processor")
	}

	// 모든 트랜잭션이 처리될 때까지 반복
	for processedCount < len(txs) {
		// 독립적인 트랜잭션 가져오기
		independentTxs := graph.getIndependentTransactions()

		if len(independentTxs) == 0 && processedCount < len(txs) {
			// 의존성 사이클이 있는 경우 처리
			po.logger.Warn("Dependency cycle detected in transactions, processing sequentially")

			// 남은 트랜잭션을 순차적으로 처리
			for _, node := range graph.nodes {
				if !node.processed {
					independentTxs = append(independentTxs, node.tx)
					node.processed = true
					break
				}
			}

			// 의존성 사이클 오류 기록
			results <- utils.ErrTxDependencyCycle
		}

		if len(independentTxs) == 0 {
			break
		}

		// 독립적인 트랜잭션을 청크로 나누기
		chunks, err := po.splitTransactions(independentTxs)
		if err != nil {
			return utils.WrapError(err, "failed to split transactions")
		}

		// 각 청크를 병렬로 처리
		for _, chunk := range chunks {
			// 워커 풀에서 워커 가져오기
			select {
			case <-po.workerPool:
				// 워커 획득 성공
			case <-time.After(5 * time.Second):
				// 타임아웃 - 워커 풀이 가득 찬 경우
				return utils.ErrWorkerPoolFull
			}

			go func(txs []*types.Transaction) {
				defer func() {
					// 워커 반환
					po.workerPool <- struct{}{}

					// 패닉 복구
					if r := recover(); r != nil {
						po.logger.Error("Panic in transaction processing", "error", r)
						results <- utils.FormatError(utils.ErrInternalError, "panic in transaction processing: %v", r)
					}
				}()

				// 트랜잭션 처리
				for _, tx := range txs {
					// 실제 구현에서는 트랜잭션 처리 로직 구현
					// 여기서는 간단히 로그만 출력하고 배치 처리기에 추가
					po.logger.Debug("Processing transaction", "hash", tx.Hash().Hex())

					// 트랜잭션을 배치 처리기에 추가
					if err := po.stateBatchProcessor.AddTransaction(tx); err != nil {
						results <- utils.WrapError(err, fmt.Sprintf("failed to add transaction %s to batch", tx.Hash().Hex()))
						continue
					}

					// 트랜잭션 처리 완료 표시
					graph.markProcessed(tx)
					processedCount++
					results <- nil
				}
			}(chunk)
		}

		// 결과 수집
		for i := 0; i < len(independentTxs); i++ {
			if err := <-results; err != nil {
				// 오류 발생 시 로그 기록 후 계속 진행
				po.logger.Error("Error processing transaction", "error", err)
			}
		}
	}

	// 남은 트랜잭션 처리
	if err := po.stateBatchProcessor.Flush(); err != nil {
		return utils.WrapError(err, "failed to flush batch processor")
	}

	// 처리 시간 기록
	processingTime := time.Since(startTime)
	po.txProcessingTimes = append(po.txProcessingTimes, processingTime)
	if len(po.txProcessingTimes) > 100 {
		po.txProcessingTimes = po.txProcessingTimes[1:]
	}

	po.logger.Info("Transactions processed", "count", len(txs), "time", processingTime)

	return nil
}

// ProcessBlockParallel은 블록을 병렬로 처리합니다.
func (po *PerformanceOptimizer) ProcessBlockParallel(block *types.Block, state *state.StateDB) error {
	// 블록 처리 시작 시간
	startTime := time.Now()

	// 트랜잭션 처리
	err := po.ProcessTransactionsParallel(block.Transactions(), state)

	// 처리 시간 기록
	processingTime := time.Since(startTime)
	po.blockProcessingTimes = append(po.blockProcessingTimes, processingTime)
	if len(po.blockProcessingTimes) > 100 {
		po.blockProcessingTimes = po.blockProcessingTimes[1:]
	}

	po.logger.Debug("Processed block", "number", block.NumberU64(), "hash", block.Hash().Hex(), "time", processingTime)

	return err
}

// splitTransactions은 트랜잭션을 청크로 나눕니다.
func (po *PerformanceOptimizer) splitTransactions(txs []*types.Transaction) ([][]*types.Transaction, error) {
	if txs == nil {
		return nil, utils.ErrInvalidParameter
	}

	if len(txs) == 0 {
		return [][]*types.Transaction{}, nil
	}

	// 워커 수에 따라 청크 크기 결정
	chunkSize := (len(txs) + po.workerCount - 1) / po.workerCount
	if chunkSize < 1 {
		chunkSize = 1
	}

	// 트랜잭션을 청크로 나누기
	var chunks [][]*types.Transaction
	for i := 0; i < len(txs); i += chunkSize {
		end := i + chunkSize
		if end > len(txs) {
			end = len(txs)
		}
		chunks = append(chunks, txs[i:end])
	}

	return chunks, nil
}

// GetValidatorFromCache는 캐시에서 검증자를 가져옵니다.
func (po *PerformanceOptimizer) GetValidatorFromCache(address common.Address) (interface{}, bool) {
	value, found := po.validatorCache.Load(address)
	if found {
		validatorCacheStats.recordHit()
	} else {
		validatorCacheStats.recordMiss()
	}
	return value, found
}

// StoreValidatorInCache는 검증자를 캐시에 저장합니다.
func (po *PerformanceOptimizer) StoreValidatorInCache(address common.Address, validator interface{}) {
	po.validatorCache.Store(address, validator)
}

// GetStateFromCache는 캐시에서 상태를 가져옵니다.
func (po *PerformanceOptimizer) GetStateFromCache(hash common.Hash) (interface{}, bool) {
	value, found := po.stateCache.Load(hash)
	if found {
		stateCacheStats.recordHit()
	} else {
		stateCacheStats.recordMiss()
	}
	return value, found
}

// StoreStateInCache는 상태를 캐시에 저장합니다.
func (po *PerformanceOptimizer) StoreStateInCache(hash common.Hash, state interface{}) {
	po.stateCache.Store(hash, state)
}

// ClearCache는 캐시를 비웁니다.
func (po *PerformanceOptimizer) ClearCache() {
	po.validatorCache = &sync.Map{}
	po.stateCache = &sync.Map{}
	po.logger.Info("Cache cleared")
}

// 트랜잭션 의존성 분석을 위한 구조체
type txDependencyGraph struct {
	nodes map[common.Hash]*txNode
}

type txNode struct {
	tx           *types.Transaction
	dependencies []*txNode
	dependents   []*txNode
	processed    bool
}

// newTxDependencyGraph는 새로운 트랜잭션 의존성 그래프를 생성합니다.
func newTxDependencyGraph() *txDependencyGraph {
	return &txDependencyGraph{
		nodes: make(map[common.Hash]*txNode),
	}
}

// addTransaction은 트랜잭션을 그래프에 추가합니다.
func (g *txDependencyGraph) addTransaction(tx *types.Transaction) {
	hash := tx.Hash()
	if _, exists := g.nodes[hash]; exists {
		return
	}

	g.nodes[hash] = &txNode{
		tx:           tx,
		dependencies: make([]*txNode, 0),
		dependents:   make([]*txNode, 0),
		processed:    false,
	}
}

// addDependency는 트랜잭션 간의 의존성을 추가합니다.
func (g *txDependencyGraph) addDependency(tx, dependency *types.Transaction) {
	txHash := tx.Hash()
	depHash := dependency.Hash()

	txNode, txExists := g.nodes[txHash]
	depNode, depExists := g.nodes[depHash]

	if !txExists || !depExists {
		return
	}

	// 의존성 추가
	txNode.dependencies = append(txNode.dependencies, depNode)
	depNode.dependents = append(depNode.dependents, txNode)
}

// getIndependentTransactions은 의존성이 없는 트랜잭션들을 반환합니다.
func (g *txDependencyGraph) getIndependentTransactions() []*types.Transaction {
	var result []*types.Transaction

	for _, node := range g.nodes {
		if !node.processed && len(node.dependencies) == 0 {
			result = append(result, node.tx)
			node.processed = true
		}
	}

	return result
}

// markProcessed는 트랜잭션을 처리 완료로 표시하고 의존성을 업데이트합니다.
func (g *txDependencyGraph) markProcessed(tx *types.Transaction) {
	hash := tx.Hash()
	node, exists := g.nodes[hash]
	if !exists {
		return
	}

	node.processed = true

	// 이 트랜잭션에 의존하는 다른 트랜잭션들의 의존성 제거
	for _, dependent := range node.dependents {
		for i, dep := range dependent.dependencies {
			if dep == node {
				// 의존성 제거
				dependent.dependencies = append(dependent.dependencies[:i], dependent.dependencies[i+1:]...)
				break
			}
		}
	}
}

// buildTxDependencyGraph는 트랜잭션 의존성 그래프를 구축합니다.
func (po *PerformanceOptimizer) buildTxDependencyGraph(txs []*types.Transaction) (*txDependencyGraph, error) {
	if txs == nil {
		return nil, utils.ErrInvalidParameter
	}

	graph := newTxDependencyGraph()

	// 모든 트랜잭션을 그래프에 추가
	for _, tx := range txs {
		if tx == nil {
			return nil, utils.FormatError(utils.ErrInvalidParameter, "nil transaction in list")
		}
		graph.addTransaction(tx)
	}

	// 의존성 분석 (같은 발신자의 트랜잭션은 논스 순서대로 처리되어야 함)
	senderMap := make(map[common.Address][]*types.Transaction)

	for _, tx := range txs {
		// 실제 구현에서는 트랜잭션에서 발신자 주소를 가져오는 로직 필요
		// 여기서는 임의의 주소 사용
		sender := common.Address{}

		senderMap[sender] = append(senderMap[sender], tx)
	}

	// 같은 발신자의 트랜잭션 간 의존성 추가
	for _, senderTxs := range senderMap {
		for i := 1; i < len(senderTxs); i++ {
			graph.addDependency(senderTxs[i], senderTxs[i-1])
		}
	}

	return graph, nil
}

// 캐시 통계 정보
type cacheStats struct {
	hits   int64
	misses int64
	mutex  sync.Mutex
}

// 캐시 통계
var (
	validatorCacheStats = &cacheStats{}
	stateCacheStats     = &cacheStats{}
)

// recordCacheHit은 캐시 적중을 기록합니다.
func (cs *cacheStats) recordHit() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.hits++
}

// recordCacheMiss는 캐시 미스를 기록합니다.
func (cs *cacheStats) recordMiss() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	cs.misses++
}

// hitRate는 캐시 적중률을 계산합니다.
func (cs *cacheStats) hitRate() float64 {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	total := cs.hits + cs.misses
	if total == 0 {
		return 0.0
	}

	return float64(cs.hits) / float64(total)
}

// resetStats는 캐시 통계를 초기화합니다.
func (cs *cacheStats) resetStats() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	cs.hits = 0
	cs.misses = 0
}

// cleanupCache는 오래된 캐시 항목을 정리합니다.
func (po *PerformanceOptimizer) cleanupCache() {
	// 여기서는 간단히 캐시를 비우는 방식으로 구현
	// 실제 구현에서는 LRU 알고리즘 등을 사용하여 오래된 항목만 제거
	po.validatorCache = &sync.Map{}
	po.stateCache = &sync.Map{}
	po.logger.Info("Cache cleaned up due to high memory usage")
}

// StateBatchProcessor는 상태 DB 배치 처리를 담당합니다.
type StateBatchProcessor struct {
	batchSize       int                  // 배치 크기
	pendingTxs      []*types.Transaction // 대기 중인 트랜잭션
	stateDB         *state.StateDB       // 상태 DB
	processingTimes []time.Duration      // 처리 시간
	mutex           sync.Mutex           // 뮤텍스
	logger          log.Logger           // 로거
}

// NewStateBatchProcessor는 새로운 상태 DB 배치 처리기를 생성합니다.
func NewStateBatchProcessor(batchSize int) *StateBatchProcessor {
	return &StateBatchProcessor{
		batchSize:       batchSize,
		pendingTxs:      make([]*types.Transaction, 0, batchSize),
		processingTimes: make([]time.Duration, 0, 100),
		logger:          log.New("module", "eirene/batch"),
	}
}

// Reset은 배치 처리기를 초기화합니다.
func (bp *StateBatchProcessor) Reset(stateDB *state.StateDB) error {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()

	if stateDB == nil {
		return utils.ErrInvalidParameter
	}

	bp.stateDB = stateDB
	bp.pendingTxs = make([]*types.Transaction, 0, bp.batchSize)

	return nil
}

// SetBatchSize는 배치 크기를 설정합니다.
func (bp *StateBatchProcessor) SetBatchSize(batchSize int) error {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()

	if batchSize < 1 {
		return utils.FormatError(utils.ErrInvalidParameter, "batch size must be at least 1, got %d", batchSize)
	}

	bp.batchSize = batchSize
	return nil
}

// AddTransaction은 트랜잭션을 배치에 추가합니다.
func (bp *StateBatchProcessor) AddTransaction(tx *types.Transaction) error {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()

	if tx == nil {
		return utils.ErrInvalidParameter
	}

	bp.pendingTxs = append(bp.pendingTxs, tx)

	// 배치가 가득 차면 처리
	if len(bp.pendingTxs) >= bp.batchSize {
		return bp.processBatchInternal()
	}

	return nil
}

// ProcessBatch는 현재 배치를 처리합니다.
func (bp *StateBatchProcessor) ProcessBatch() error {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()

	if len(bp.pendingTxs) > 0 {
		return bp.processBatchInternal()
	}

	return nil
}

// Flush는 남은 모든 트랜잭션을 처리합니다.
func (bp *StateBatchProcessor) Flush() error {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()

	if len(bp.pendingTxs) > 0 {
		return bp.processBatchInternal()
	}

	return nil
}

// processBatchInternal은 내부적으로 배치를 처리합니다.
func (bp *StateBatchProcessor) processBatchInternal() error {
	if bp.stateDB == nil {
		return utils.ErrInternalError
	}

	if len(bp.pendingTxs) == 0 {
		return nil
	}

	startTime := time.Now()

	// 실제 구현에서는 트랜잭션을 일괄 처리하는 로직 구현
	// 여기서는 간단히 로그만 출력
	bp.logger.Debug("Processing batch", "count", len(bp.pendingTxs))

	// 배치 처리 로직 (실제 구현에서는 상태 DB에 일괄 적용)
	// 예: bp.stateDB.ApplyBatch(bp.pendingTxs)

	// 배치 처리 시간 기록
	processingTime := time.Since(startTime)
	bp.processingTimes = append(bp.processingTimes, processingTime)
	if len(bp.processingTimes) > 100 {
		bp.processingTimes = bp.processingTimes[1:]
	}

	// 처리된 트랜잭션 초기화
	bp.pendingTxs = make([]*types.Transaction, 0, bp.batchSize)

	bp.logger.Debug("Batch processed", "time", processingTime)

	return nil
}

// GetAverageProcessingTime은 평균 배치 처리 시간을 반환합니다.
func (bp *StateBatchProcessor) GetAverageProcessingTime() time.Duration {
	bp.mutex.Lock()
	defer bp.mutex.Unlock()

	if len(bp.processingTimes) == 0 {
		return 0
	}

	var total time.Duration
	for _, t := range bp.processingTimes {
		total += t
	}

	return total / time.Duration(len(bp.processingTimes))
}
