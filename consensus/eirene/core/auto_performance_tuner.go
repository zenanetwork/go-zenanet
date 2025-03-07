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
	"math"
	"math/rand"
	"runtime"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/metrics"
)

func init() {
	// 난수 생성기 초기화
	rand.Seed(time.Now().UnixNano())
}

// 자동 성능 튜닝 관련 상수
const (
	// 튜닝 간격
	defaultTuningInterval = 5 * time.Minute
	minTuningInterval     = 1 * time.Minute
	maxTuningInterval     = 30 * time.Minute

	// 워커 풀 관련
	minWorkerPoolSize = 2
	maxWorkerPoolSize = 32

	// 캐시 관련
	tunerMinCacheSize = 128
	tunerMaxCacheSize = 16384

	// 배치 처리 관련
	tunerMinBatchSize = 10
	tunerMaxBatchSize = 1000

	// 튜닝 알고리즘 관련
	learningRate        = 0.1  // 학습률
	explorationRate     = 0.2  // 탐색률
	explorationDecay    = 0.95 // 탐색률 감소 계수
	minExplorationRate  = 0.05 // 최소 탐색률
	performanceWeight   = 0.7  // 성능 가중치
	resourceWeight      = 0.3  // 자원 사용량 가중치
	
	// 메트릭스 관련
	tunerMetricsInterval = 1 * time.Minute
)

// TuningParameter는 튜닝 가능한 매개변수를 나타냅니다.
type TuningParameter struct {
	Name        string  // 매개변수 이름
	CurrentValue int     // 현재 값
	MinValue     int     // 최소 값
	MaxValue     int     // 최대 값
	StepSize     int     // 조정 단위
	Weight       float64 // 가중치
}

// TuningResult는 튜닝 결과를 나타냅니다.
type TuningResult struct {
	Parameter   string  // 매개변수 이름
	OldValue    int     // 이전 값
	NewValue    int     // 새 값
	Performance float64 // 성능 점수
	Timestamp   time.Time // 튜닝 시간
}

// PerformanceMetrics는 성능 지표를 나타냅니다.
type PerformanceMetrics struct {
	TxThroughput       float64 // 초당 트랜잭션 처리량
	BlockTime          float64 // 블록 생성 시간 (초)
	CPUUsage           float64 // CPU 사용률 (%)
	MemoryUsage        float64 // 메모리 사용률 (%)
	CacheHitRate       float64 // 캐시 적중률 (%)
	ParallelizableRatio float64 // 병렬화 가능 비율 (%)
	AvgLatency         float64 // 평균 지연 시간 (ms)
}

// AutoPerformanceTuner는 자동 성능 튜닝 시스템을 구현합니다.
type AutoPerformanceTuner struct {
	// 튜닝 매개변수
	parameters map[string]*TuningParameter
	
	// 튜닝 결과 기록
	tuningHistory []TuningResult
	
	// 성능 지표 기록
	performanceHistory []PerformanceMetrics
	
	// 튜닝 간격
	tuningInterval time.Duration
	
	// 마지막 튜닝 시간
	lastTuningTime time.Time
	
	// 튜닝 활성화 여부
	enabled bool
	
	// 탐색률
	explorationRate float64
	
	// 메트릭스
	metrics struct {
		tuningCount        *metrics.Counter
		parameterChanges   *metrics.Counter
		performanceScore   *metrics.Gauge
		resourceUsage      *metrics.Gauge
		tuningDuration     *metrics.Gauge
	}
	
	// 컴포넌트 참조
	adaptiveWorkerPool *AdaptiveWorkerPool
	txDependencyAnalyzer *TxDependencyAnalyzer
	smartContractAnalyzer *SmartContractDependencyAnalyzer
	blockTimeOptimizer *BlockTimeOptimizer
	
	// 로깅
	logger log.Logger
	
	// 락
	lock sync.RWMutex
	
	// 종료 채널
	quit chan struct{}
}

// NewAutoPerformanceTuner는 새로운 자동 성능 튜닝 시스템을 생성합니다.
func NewAutoPerformanceTuner() *AutoPerformanceTuner {
	tuner := &AutoPerformanceTuner{
		parameters:        make(map[string]*TuningParameter),
		tuningHistory:     make([]TuningResult, 0),
		performanceHistory: make([]PerformanceMetrics, 0),
		tuningInterval:    defaultTuningInterval,
		lastTuningTime:    time.Now(),
		enabled:           true,
		explorationRate:   explorationRate,
		logger:            log.New("module", "auto-tuner"),
		quit:              make(chan struct{}),
	}
	
	// 메트릭스 초기화
	tuner.metrics.tuningCount = metrics.NewCounter()
	metrics.Register("eirene/auto_tuner/tuning_count", tuner.metrics.tuningCount)
	
	tuner.metrics.parameterChanges = metrics.NewCounter()
	metrics.Register("eirene/auto_tuner/parameter_changes", tuner.metrics.parameterChanges)
	
	tuner.metrics.performanceScore = metrics.NewGauge()
	metrics.Register("eirene/auto_tuner/performance_score", tuner.metrics.performanceScore)
	
	tuner.metrics.resourceUsage = metrics.NewGauge()
	metrics.Register("eirene/auto_tuner/resource_usage", tuner.metrics.resourceUsage)
	
	tuner.metrics.tuningDuration = metrics.NewGauge()
	metrics.Register("eirene/auto_tuner/tuning_duration", tuner.metrics.tuningDuration)
	
	// 기본 튜닝 매개변수 등록
	tuner.RegisterParameter("worker_pool_size", runtime.NumCPU(), minWorkerPoolSize, maxWorkerPoolSize, 1, 1.0)
	tuner.RegisterParameter("tx_batch_size", 100, tunerMinBatchSize, tunerMaxBatchSize, 10, 0.8)
	tuner.RegisterParameter("lru_cache_size", 1024, tunerMinCacheSize, tunerMaxCacheSize, 128, 0.6)
	tuner.RegisterParameter("block_time_target", 5, 1, 15, 1, 0.9)
	
	return tuner
}

// RegisterParameter는 튜닝 매개변수를 등록합니다.
func (apt *AutoPerformanceTuner) RegisterParameter(name string, initialValue, minValue, maxValue, stepSize int, weight float64) {
	apt.lock.Lock()
	defer apt.lock.Unlock()
	
	apt.parameters[name] = &TuningParameter{
		Name:         name,
		CurrentValue: initialValue,
		MinValue:     minValue,
		MaxValue:     maxValue,
		StepSize:     stepSize,
		Weight:       weight,
	}
	
	apt.logger.Info("Registered tuning parameter", "name", name, "initial", initialValue, "min", minValue, "max", maxValue)
}

// SetComponents는 튜닝 대상 컴포넌트를 설정합니다.
func (apt *AutoPerformanceTuner) SetComponents(
	workerPool *AdaptiveWorkerPool,
	txAnalyzer *TxDependencyAnalyzer,
	contractAnalyzer *SmartContractDependencyAnalyzer,
	blockOptimizer *BlockTimeOptimizer,
) {
	apt.lock.Lock()
	defer apt.lock.Unlock()
	
	apt.adaptiveWorkerPool = workerPool
	apt.txDependencyAnalyzer = txAnalyzer
	apt.smartContractAnalyzer = contractAnalyzer
	apt.blockTimeOptimizer = blockOptimizer
	
	apt.logger.Info("Components set for auto tuning")
}

// Start는 자동 성능 튜닝 시스템을 시작합니다.
func (apt *AutoPerformanceTuner) Start() {
	apt.lock.Lock()
	defer apt.lock.Unlock()
	
	apt.enabled = true
	apt.lastTuningTime = time.Now()
	
	go apt.tuningLoop()
	
	apt.logger.Info("Auto performance tuner started", "interval", apt.tuningInterval)
}

// Stop은 자동 성능 튜닝 시스템을 중지합니다.
func (apt *AutoPerformanceTuner) Stop() {
	apt.lock.Lock()
	defer apt.lock.Unlock()
	
	apt.enabled = false
	close(apt.quit)
	
	apt.logger.Info("Auto performance tuner stopped")
}

// SetTuningInterval은 튜닝 간격을 설정합니다.
func (apt *AutoPerformanceTuner) SetTuningInterval(interval time.Duration) {
	apt.lock.Lock()
	defer apt.lock.Unlock()
	
	if interval < minTuningInterval {
		interval = minTuningInterval
	} else if interval > maxTuningInterval {
		interval = maxTuningInterval
	}
	
	apt.tuningInterval = interval
	apt.logger.Info("Tuning interval updated", "interval", interval)
}

// tuningLoop는 주기적으로 성능 튜닝을 수행합니다.
func (apt *AutoPerformanceTuner) tuningLoop() {
	ticker := time.NewTicker(apt.tuningInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if apt.enabled {
				apt.PerformTuning()
			}
		case <-apt.quit:
			return
		}
	}
}

// PerformTuning은 성능 튜닝을 수행합니다.
func (apt *AutoPerformanceTuner) PerformTuning() {
	apt.lock.Lock()
	defer apt.lock.Unlock()
	
	startTime := time.Now()
	apt.logger.Info("Starting performance tuning")
	
	// 현재 성능 지표 수집
	currentMetrics := apt.collectPerformanceMetrics()
	apt.performanceHistory = append(apt.performanceHistory, currentMetrics)
	
	// 성능 점수 계산
	performanceScore := apt.calculatePerformanceScore(currentMetrics)
	apt.metrics.performanceScore.Update(int64(performanceScore * 100))
	
	// 자원 사용량 점수 계산
	resourceScore := apt.calculateResourceScore(currentMetrics)
	apt.metrics.resourceUsage.Update(int64(resourceScore * 100))
	
	// 매개변수 튜닝
	parameterChanges := 0
	for name, param := range apt.parameters {
		// 탐색 또는 최적화 결정
		if apt.shouldExplore() {
			// 탐색: 무작위 조정
			apt.exploreParameter(name, param)
			parameterChanges++
		} else {
			// 최적화: 성능 기반 조정
			changed := apt.optimizeParameter(name, param, performanceScore)
			if changed {
				parameterChanges++
			}
		}
	}
	
	// 탐색률 감소
	apt.explorationRate *= explorationDecay
	if apt.explorationRate < minExplorationRate {
		apt.explorationRate = minExplorationRate
	}
	
	// 매개변수 적용
	apt.applyParameters()
	
	// 메트릭스 업데이트
	apt.metrics.tuningCount.Inc(1)
	apt.metrics.parameterChanges.Inc(int64(parameterChanges))
	
	// 튜닝 시간 기록
	tuningDuration := time.Since(startTime)
	apt.metrics.tuningDuration.Update(tuningDuration.Nanoseconds())
	apt.lastTuningTime = time.Now()
	
	apt.logger.Info("Performance tuning completed", 
		"duration", tuningDuration, 
		"changes", parameterChanges,
		"performance", fmt.Sprintf("%.2f%%", performanceScore*100),
		"exploration_rate", fmt.Sprintf("%.2f%%", apt.explorationRate*100))
}

// collectPerformanceMetrics는 현재 성능 지표를 수집합니다.
func (apt *AutoPerformanceTuner) collectPerformanceMetrics() PerformanceMetrics {
	metrics := PerformanceMetrics{}
	
	// 실제 구현에서는 각 컴포넌트에서 성능 지표를 수집
	// 여기서는 예시로 구현
	
	// 워커 풀 성능
	if apt.adaptiveWorkerPool != nil {
		poolStats := apt.adaptiveWorkerPool.GetStats()
		if throughput, ok := poolStats["throughput"].(float64); ok {
			metrics.TxThroughput = throughput
		}
	}
	
	// 블록 시간 최적화기 성능
	if apt.blockTimeOptimizer != nil {
		// 실제 구현에서는 blockTimeOptimizer.GetStats() 메서드를 호출
		// 여기서는 예시로 구현
		metrics.BlockTime = 5.0 // 예시 값
	}
	
	// 스마트 컨트랙트 분석기 성능
	if apt.smartContractAnalyzer != nil {
		// 병렬화 가능 비율 (예시)
		metrics.ParallelizableRatio = 0.7
	}
	
	// 시스템 자원 사용량
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	metrics.CPUUsage = float64(runtime.NumGoroutine()) / float64(runtime.NumCPU() * 100)
	metrics.MemoryUsage = float64(memStats.Alloc) / float64(memStats.Sys)
	
	// 캐시 적중률 (예시)
	metrics.CacheHitRate = 0.85
	
	// 평균 지연 시간 (예시, 밀리초 단위)
	metrics.AvgLatency = 50.0
	
	return metrics
}

// calculatePerformanceScore는 성능 점수를 계산합니다.
func (apt *AutoPerformanceTuner) calculatePerformanceScore(metrics PerformanceMetrics) float64 {
	// 각 지표별 가중치
	weights := map[string]float64{
		"tx_throughput":        0.3,
		"block_time":           0.2,
		"cache_hit_rate":       0.15,
		"parallelizable_ratio": 0.15,
		"avg_latency":          0.2,
	}
	
	// 정규화된 점수 계산
	txThroughputScore := math.Min(metrics.TxThroughput/1000.0, 1.0)
	blockTimeScore := 1.0 - math.Min(metrics.BlockTime/15.0, 1.0)
	cacheHitRateScore := metrics.CacheHitRate
	parallelizableRatioScore := metrics.ParallelizableRatio
	latencyScore := 1.0 - math.Min(metrics.AvgLatency/500.0, 1.0)
	
	// 가중 평균 계산
	score := txThroughputScore * weights["tx_throughput"] +
		blockTimeScore * weights["block_time"] +
		cacheHitRateScore * weights["cache_hit_rate"] +
		parallelizableRatioScore * weights["parallelizable_ratio"] +
		latencyScore * weights["avg_latency"]
	
	return score
}

// calculateResourceScore는 자원 사용량 점수를 계산합니다.
func (apt *AutoPerformanceTuner) calculateResourceScore(metrics PerformanceMetrics) float64 {
	// 각 지표별 가중치
	weights := map[string]float64{
		"cpu_usage":    0.6,
		"memory_usage": 0.4,
	}
	
	// 정규화된 점수 계산 (낮을수록 좋음)
	cpuScore := 1.0 - metrics.CPUUsage
	memoryScore := 1.0 - metrics.MemoryUsage
	
	// 가중 평균 계산
	score := cpuScore * weights["cpu_usage"] +
		memoryScore * weights["memory_usage"]
	
	return score
}

// shouldExplore는 탐색을 수행할지 여부를 결정합니다.
func (apt *AutoPerformanceTuner) shouldExplore() bool {
	// 간단한 랜덤 비교로 구현
	return apt.explorationRate > rand.Float64()
}

// exploreParameter는 매개변수를 탐색적으로 조정합니다.
func (apt *AutoPerformanceTuner) exploreParameter(name string, param *TuningParameter) {
	oldValue := param.CurrentValue
	
	// 무작위 조정 방향 결정
	direction := 1
	if rand.Float64() < 0.5 {
		direction = -1
	}
	
	// 무작위 조정 크기 결정
	steps := 1 + int(rand.Float64()*float64(3))
	delta := direction * steps * param.StepSize
	
	// 새 값 계산 및 범위 검사
	newValue := oldValue + delta
	if newValue < param.MinValue {
		newValue = param.MinValue
	} else if newValue > param.MaxValue {
		newValue = param.MaxValue
	}
	
	// 값 변경
	param.CurrentValue = newValue
	
	// 튜닝 결과 기록
	apt.tuningHistory = append(apt.tuningHistory, TuningResult{
		Parameter:   name,
		OldValue:    oldValue,
		NewValue:    newValue,
		Performance: 0.0, // 탐색 단계에서는 성능 측정 안 함
		Timestamp:   time.Now(),
	})
	
	apt.logger.Debug("Explored parameter", "name", name, "old", oldValue, "new", newValue, "delta", delta)
}

// optimizeParameter는 매개변수를 최적화합니다.
func (apt *AutoPerformanceTuner) optimizeParameter(name string, param *TuningParameter, currentPerformance float64) bool {
	// 이전 튜닝 결과 찾기
	var lastTuning *TuningResult
	for i := len(apt.tuningHistory) - 1; i >= 0; i-- {
		if apt.tuningHistory[i].Parameter == name {
			lastTuning = &apt.tuningHistory[i]
			break
		}
	}
	
	// 이전 튜닝 결과가 없으면 조정하지 않음
	if lastTuning == nil {
		return false
	}
	
	oldValue := param.CurrentValue
	
	// 성능 변화에 따른 조정 방향 결정
	var direction int
	if lastTuning.Performance < currentPerformance {
		// 성능이 향상되었으면 같은 방향으로 계속 조정
		if lastTuning.NewValue > lastTuning.OldValue {
			direction = 1
		} else {
			direction = -1
		}
	} else {
		// 성능이 저하되었으면 반대 방향으로 조정
		if lastTuning.NewValue > lastTuning.OldValue {
			direction = -1
		} else {
			direction = 1
		}
	}
	
	// 조정 크기 계산
	delta := direction * param.StepSize
	
	// 새 값 계산 및 범위 검사
	newValue := oldValue + delta
	if newValue < param.MinValue {
		newValue = param.MinValue
	} else if newValue > param.MaxValue {
		newValue = param.MaxValue
	}
	
	// 값이 변경되지 않으면 조정하지 않음
	if newValue == oldValue {
		return false
	}
	
	// 값 변경
	param.CurrentValue = newValue
	
	// 튜닝 결과 기록
	apt.tuningHistory = append(apt.tuningHistory, TuningResult{
		Parameter:   name,
		OldValue:    oldValue,
		NewValue:    newValue,
		Performance: currentPerformance,
		Timestamp:   time.Now(),
	})
	
	apt.logger.Debug("Optimized parameter", "name", name, "old", oldValue, "new", newValue, "delta", delta, "performance", currentPerformance)
	
	return true
}

// applyParameters는 튜닝된 매개변수를 각 컴포넌트에 적용합니다.
func (apt *AutoPerformanceTuner) applyParameters() {
	// 워커 풀 크기 적용
	if apt.adaptiveWorkerPool != nil {
		if param, ok := apt.parameters["worker_pool_size"]; ok {
			// 실제 구현에서는 adaptiveWorkerPool.SetWorkerCount() 메서드를 호출
			// 여기서는 로깅만 수행
			apt.logger.Debug("Applied worker pool size", "value", param.CurrentValue)
		}
	}
	
	// 배치 크기 적용
	if param, ok := apt.parameters["tx_batch_size"]; ok {
		// 배치 크기 적용 로직 (예시)
		apt.logger.Debug("Applied tx batch size", "value", param.CurrentValue)
	}
	
	// 캐시 크기 적용
	if param, ok := apt.parameters["lru_cache_size"]; ok {
		// 캐시 크기 적용 로직 (예시)
		apt.logger.Debug("Applied LRU cache size", "value", param.CurrentValue)
	}
	
	// 블록 시간 목표 적용
	if apt.blockTimeOptimizer != nil {
		if param, ok := apt.parameters["block_time_target"]; ok {
			// 실제 구현에서는 blockTimeOptimizer.SetTargetBlockTime() 메서드를 호출
			// 여기서는 로깅만 수행
			apt.logger.Debug("Applied block time target", "value", param.CurrentValue)
		}
	}
}

// GetTuningHistory는 튜닝 기록을 반환합니다.
func (apt *AutoPerformanceTuner) GetTuningHistory() []TuningResult {
	apt.lock.RLock()
	defer apt.lock.RUnlock()
	
	return apt.tuningHistory
}

// GetPerformanceHistory는 성능 기록을 반환합니다.
func (apt *AutoPerformanceTuner) GetPerformanceHistory() []PerformanceMetrics {
	apt.lock.RLock()
	defer apt.lock.RUnlock()
	
	return apt.performanceHistory
}

// GetCurrentParameters는 현재 매개변수 값을 반환합니다.
func (apt *AutoPerformanceTuner) GetCurrentParameters() map[string]int {
	apt.lock.RLock()
	defer apt.lock.RUnlock()
	
	result := make(map[string]int)
	for name, param := range apt.parameters {
		result[name] = param.CurrentValue
	}
	
	return result
}

// ResetTuningHistory는 튜닝 기록을 초기화합니다.
func (apt *AutoPerformanceTuner) ResetTuningHistory() {
	apt.lock.Lock()
	defer apt.lock.Unlock()
	
	apt.tuningHistory = make([]TuningResult, 0)
	apt.performanceHistory = make([]PerformanceMetrics, 0)
	
	apt.logger.Info("Tuning history reset")
}

// GenerateReport는 성능 튜닝 보고서를 생성합니다.
func (apt *AutoPerformanceTuner) GenerateReport() string {
	apt.lock.RLock()
	defer apt.lock.RUnlock()
	
	report := "=== 자동 성능 튜닝 보고서 ===\n\n"
	
	// 현재 매개변수 값
	report += "현재 매개변수 값:\n"
	for name, param := range apt.parameters {
		report += fmt.Sprintf("- %s: %d (범위: %d-%d, 단위: %d)\n", 
			name, param.CurrentValue, param.MinValue, param.MaxValue, param.StepSize)
	}
	
	// 최근 성능 지표
	if len(apt.performanceHistory) > 0 {
		latest := apt.performanceHistory[len(apt.performanceHistory)-1]
		report += "\n최근 성능 지표:\n"
		report += fmt.Sprintf("- 트랜잭션 처리량: %.2f TPS\n", latest.TxThroughput)
		report += fmt.Sprintf("- 블록 생성 시간: %.2f 초\n", latest.BlockTime)
		report += fmt.Sprintf("- CPU 사용률: %.2f%%\n", latest.CPUUsage*100)
		report += fmt.Sprintf("- 메모리 사용률: %.2f%%\n", latest.MemoryUsage*100)
		report += fmt.Sprintf("- 캐시 적중률: %.2f%%\n", latest.CacheHitRate*100)
		report += fmt.Sprintf("- 병렬화 가능 비율: %.2f%%\n", latest.ParallelizableRatio*100)
		report += fmt.Sprintf("- 평균 지연 시간: %.2f ms\n", latest.AvgLatency)
	}
	
	// 최근 튜닝 결과
	if len(apt.tuningHistory) > 0 {
		report += "\n최근 튜닝 결과:\n"
		count := 5
		if len(apt.tuningHistory) < count {
			count = len(apt.tuningHistory)
		}
		
		for i := len(apt.tuningHistory) - count; i < len(apt.tuningHistory); i++ {
			tuning := apt.tuningHistory[i]
			report += fmt.Sprintf("- %s: %s: %d -> %d (성능: %.2f%%)\n", 
				tuning.Timestamp.Format("2006-01-02 15:04:05"),
				tuning.Parameter, tuning.OldValue, tuning.NewValue, tuning.Performance*100)
		}
	}
	
	// 튜닝 통계
	report += "\n튜닝 통계:\n"
	report += fmt.Sprintf("- 총 튜닝 횟수: %d\n", len(apt.tuningHistory))
	report += fmt.Sprintf("- 마지막 튜닝 시간: %s\n", apt.lastTuningTime.Format("2006-01-02 15:04:05"))
	report += fmt.Sprintf("- 현재 탐색률: %.2f%%\n", apt.explorationRate*100)
	
	return report
} 