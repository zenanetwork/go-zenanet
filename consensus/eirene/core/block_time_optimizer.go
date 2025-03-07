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
	"math"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/params"
)

// BlockTimeOptimizer는 블록 생성 시간을 최적화하는 모듈입니다.
// 이 모듈은 네트워크 상태, 트랜잭션 처리량, 검증자 성능 등을 모니터링하여
// 최적의 블록 생성 시간을 동적으로 조정합니다.
type BlockTimeOptimizer struct {
	// 설정
	config *BlockTimeOptimizerConfig

	// 상태 데이터
	networkStats      *NetworkStats
	validatorStats    map[common.Address]*ValidatorStats
	blockTimeHistory  []uint64
	txThroughputStats *TxThroughputStats

	// 동적 조정 관련
	currentOptimalBlockTime uint64
	minBlockTime            uint64
	maxBlockTime            uint64
	adjustmentFactor        float64

	// 동시성 제어
	lock sync.RWMutex

	// 로깅
	logger log.Logger
}

// BlockTimeOptimizerConfig는 블록 시간 최적화 모듈의 설정입니다.
type BlockTimeOptimizerConfig struct {
	// 기본 설정
	DefaultBlockTime uint64 `json:"defaultBlockTime"` // 기본 블록 생성 시간 (초)
	MinBlockTime     uint64 `json:"minBlockTime"`     // 최소 블록 생성 시간 (초)
	MaxBlockTime     uint64 `json:"maxBlockTime"`     // 최대 블록 생성 시간 (초)

	// 조정 관련 설정
	AdjustmentInterval      uint64  `json:"adjustmentInterval"`      // 블록 시간 조정 간격 (블록 수)
	AdjustmentFactor        float64 `json:"adjustmentFactor"`        // 조정 계수 (0.0 ~ 1.0)
	NetworkLoadThreshold    float64 `json:"networkLoadThreshold"`    // 네트워크 부하 임계값 (0.0 ~ 1.0)
	TxThroughputThreshold   uint64  `json:"txThroughputThreshold"`   // 트랜잭션 처리량 임계값 (TPS)
	ValidatorPerformanceMin float64 `json:"validatorPerformanceMin"` // 최소 검증자 성능 임계값 (0.0 ~ 1.0)

	// 히스토리 관련 설정
	HistorySize uint64 `json:"historySize"` // 블록 시간 히스토리 크기
}

// NetworkStats는 네트워크 상태 통계를 나타냅니다.
type NetworkStats struct {
	// 네트워크 부하
	NetworkLoad float64 // 네트워크 부하 (0.0 ~ 1.0)

	// 네트워크 지연
	AverageLatency    time.Duration // 평균 네트워크 지연 시간
	LatencyPercentile map[int]time.Duration // 지연 시간 백분위수 (50, 90, 95, 99)

	// 네트워크 처리량
	InboundBandwidth  uint64 // 인바운드 대역폭 (bytes/s)
	OutboundBandwidth uint64 // 아웃바운드 대역폭 (bytes/s)

	// 블록 전파 통계
	BlockPropagationTime time.Duration // 블록 전파 시간
	BlockPropagationRate float64       // 블록 전파 성공률 (0.0 ~ 1.0)
}

// ValidatorStats는 검증자 성능 통계를 나타냅니다.
type ValidatorStats struct {
	// 검증자 정보
	Address     common.Address // 검증자 주소
	Performance float64        // 검증자 성능 점수 (0.0 ~ 1.0)

	// 블록 생성 통계
	BlocksProposed  uint64 // 제안한 블록 수
	BlocksMissed    uint64 // 놓친 블록 수
	AverageBlockTime uint64 // 평균 블록 생성 시간 (ms)

	// 트랜잭션 처리 통계
	TxProcessed      uint64 // 처리한 트랜잭션 수
	TxProcessingTime uint64 // 평균 트랜잭션 처리 시간 (ms)
}

// TxThroughputStats는 트랜잭션 처리량 통계를 나타냅니다.
type TxThroughputStats struct {
	// 트랜잭션 처리량
	CurrentTPS      uint64 // 현재 초당 트랜잭션 처리량
	AverageTPS      uint64 // 평균 초당 트랜잭션 처리량
	PeakTPS         uint64 // 최대 초당 트랜잭션 처리량
	TPSHistory      []uint64 // TPS 히스토리

	// 트랜잭션 풀 상태
	PendingTxCount  uint64 // 대기 중인 트랜잭션 수
	QueuedTxCount   uint64 // 대기열에 있는 트랜잭션 수
	DiscardedTxCount uint64 // 폐기된 트랜잭션 수

	// 트랜잭션 처리 시간
	AverageTxTime   uint64 // 평균 트랜잭션 처리 시간 (ms)
	TxTimePercentile map[int]uint64 // 트랜잭션 처리 시간 백분위수 (50, 90, 95, 99)
}

// NewBlockTimeOptimizer는 새로운 블록 시간 최적화 모듈을 생성합니다.
func NewBlockTimeOptimizer(config *BlockTimeOptimizerConfig) *BlockTimeOptimizer {
	if config == nil {
		config = &BlockTimeOptimizerConfig{
			DefaultBlockTime:        4,  // 기본 4초
			MinBlockTime:            2,  // 최소 2초
			MaxBlockTime:            8,  // 최대 8초
			AdjustmentInterval:      100, // 100블록마다 조정
			AdjustmentFactor:        0.2, // 20% 조정
			NetworkLoadThreshold:    0.8, // 80% 네트워크 부하 임계값
			TxThroughputThreshold:   1000, // 1000 TPS 임계값
			ValidatorPerformanceMin: 0.7, // 70% 최소 검증자 성능
			HistorySize:             1000, // 1000개 블록 히스토리
		}
	}

	return &BlockTimeOptimizer{
		config:                config,
		networkStats:          &NetworkStats{},
		validatorStats:        make(map[common.Address]*ValidatorStats),
		blockTimeHistory:      make([]uint64, 0, config.HistorySize),
		txThroughputStats:     &TxThroughputStats{},
		currentOptimalBlockTime: config.DefaultBlockTime,
		minBlockTime:          config.MinBlockTime,
		maxBlockTime:          config.MaxBlockTime,
		adjustmentFactor:      config.AdjustmentFactor,
		logger:                log.New("module", "block-time-optimizer"),
	}
}

// UpdateNetworkStats는 네트워크 상태 통계를 업데이트합니다.
func (bto *BlockTimeOptimizer) UpdateNetworkStats(stats *NetworkStats) {
	bto.lock.Lock()
	defer bto.lock.Unlock()

	bto.networkStats = stats
}

// UpdateValidatorStats는 검증자 성능 통계를 업데이트합니다.
func (bto *BlockTimeOptimizer) UpdateValidatorStats(address common.Address, stats *ValidatorStats) {
	bto.lock.Lock()
	defer bto.lock.Unlock()

	bto.validatorStats[address] = stats
}

// UpdateTxThroughputStats는 트랜잭션 처리량 통계를 업데이트합니다.
func (bto *BlockTimeOptimizer) UpdateTxThroughputStats(stats *TxThroughputStats) {
	bto.lock.Lock()
	defer bto.lock.Unlock()

	bto.txThroughputStats = stats
}

// RecordBlockTime은 블록 생성 시간을 기록합니다.
func (bto *BlockTimeOptimizer) RecordBlockTime(blockTime uint64) {
	bto.lock.Lock()
	defer bto.lock.Unlock()

	// 블록 시간 히스토리 업데이트
	bto.blockTimeHistory = append(bto.blockTimeHistory, blockTime)
	if uint64(len(bto.blockTimeHistory)) > bto.config.HistorySize {
		bto.blockTimeHistory = bto.blockTimeHistory[1:]
	}
}

// GetOptimalBlockTime은 현재 최적의 블록 생성 시간을 반환합니다.
func (bto *BlockTimeOptimizer) GetOptimalBlockTime() uint64 {
	bto.lock.RLock()
	defer bto.lock.RUnlock()

	return bto.currentOptimalBlockTime
}

// OptimizeBlockTime은 네트워크 상태, 트랜잭션 처리량, 검증자 성능 등을 고려하여
// 최적의 블록 생성 시간을 계산합니다.
func (bto *BlockTimeOptimizer) OptimizeBlockTime(blockNumber uint64) uint64 {
	// 조정 간격마다 블록 시간 최적화
	if blockNumber%bto.config.AdjustmentInterval != 0 {
		return bto.GetOptimalBlockTime()
	}

	bto.lock.Lock()
	defer bto.lock.Unlock()

	// 네트워크 부하 기반 조정
	networkLoadFactor := bto.calculateNetworkLoadFactor()

	// 트랜잭션 처리량 기반 조정
	txThroughputFactor := bto.calculateTxThroughputFactor()

	// 검증자 성능 기반 조정
	validatorPerformanceFactor := bto.calculateValidatorPerformanceFactor()

	// 종합 조정 계수 계산
	adjustmentFactor := (networkLoadFactor + txThroughputFactor + validatorPerformanceFactor) / 3.0

	// 블록 시간 조정
	newBlockTime := bto.calculateNewBlockTime(adjustmentFactor)

	// 로그 기록
	bto.logger.Info("Block time optimized",
		"blockNumber", blockNumber,
		"oldBlockTime", bto.currentOptimalBlockTime,
		"newBlockTime", newBlockTime,
		"networkLoadFactor", networkLoadFactor,
		"txThroughputFactor", txThroughputFactor,
		"validatorPerformanceFactor", validatorPerformanceFactor,
		"adjustmentFactor", adjustmentFactor)

	// 최적 블록 시간 업데이트
	bto.currentOptimalBlockTime = newBlockTime

	return newBlockTime
}

// calculateNetworkLoadFactor는 네트워크 부하 기반 조정 계수를 계산합니다.
// 네트워크 부하가 높을수록 블록 시간을 늘리고, 낮을수록 줄입니다.
func (bto *BlockTimeOptimizer) calculateNetworkLoadFactor() float64 {
	if bto.networkStats == nil {
		return 0.0
	}

	// 네트워크 부하가 임계값보다 높으면 양수 반환 (블록 시간 증가)
	// 네트워크 부하가 임계값보다 낮으면 음수 반환 (블록 시간 감소)
	loadDiff := bto.networkStats.NetworkLoad - bto.config.NetworkLoadThreshold
	return math.Max(-1.0, math.Min(1.0, loadDiff*2.0))
}

// calculateTxThroughputFactor는 트랜잭션 처리량 기반 조정 계수를 계산합니다.
// 트랜잭션 처리량이 높을수록 블록 시간을 줄이고, 낮을수록 늘립니다.
func (bto *BlockTimeOptimizer) calculateTxThroughputFactor() float64 {
	if bto.txThroughputStats == nil {
		return 0.0
	}

	// 트랜잭션 처리량이 임계값보다 높으면 음수 반환 (블록 시간 감소)
	// 트랜잭션 처리량이 임계값보다 낮으면 양수 반환 (블록 시간 증가)
	tpsDiff := float64(bto.config.TxThroughputThreshold) - float64(bto.txThroughputStats.CurrentTPS)
	normalizedDiff := tpsDiff / float64(bto.config.TxThroughputThreshold)
	return math.Max(-1.0, math.Min(1.0, normalizedDiff))
}

// calculateValidatorPerformanceFactor는 검증자 성능 기반 조정 계수를 계산합니다.
// 검증자 성능이 좋을수록 블록 시간을 줄이고, 나쁠수록 늘립니다.
func (bto *BlockTimeOptimizer) calculateValidatorPerformanceFactor() float64 {
	if len(bto.validatorStats) == 0 {
		return 0.0
	}

	// 모든 검증자의 평균 성능 계산
	var totalPerformance float64
	for _, stats := range bto.validatorStats {
		totalPerformance += stats.Performance
	}
	avgPerformance := totalPerformance / float64(len(bto.validatorStats))

	// 검증자 성능이 임계값보다 높으면 음수 반환 (블록 시간 감소)
	// 검증자 성능이 임계값보다 낮으면 양수 반환 (블록 시간 증가)
	perfDiff := bto.config.ValidatorPerformanceMin - avgPerformance
	return math.Max(-1.0, math.Min(1.0, perfDiff*2.0))
}

// calculateNewBlockTime은 조정 계수를 기반으로 새로운 블록 시간을 계산합니다.
func (bto *BlockTimeOptimizer) calculateNewBlockTime(adjustmentFactor float64) uint64 {
	// 현재 블록 시간
	currentTime := bto.currentOptimalBlockTime

	// 조정 계수에 따라 블록 시간 조정
	// 양수: 블록 시간 증가, 음수: 블록 시간 감소
	adjustment := float64(currentTime) * adjustmentFactor * bto.adjustmentFactor
	newTime := float64(currentTime) + adjustment

	// 최소/최대 블록 시간 범위 내로 제한
	newTime = math.Max(float64(bto.minBlockTime), math.Min(float64(bto.maxBlockTime), newTime))

	return uint64(math.Round(newTime))
}

// GetBlockTimeStats는 블록 시간 통계를 반환합니다.
func (bto *BlockTimeOptimizer) GetBlockTimeStats() map[string]interface{} {
	bto.lock.RLock()
	defer bto.lock.RUnlock()

	// 블록 시간 히스토리가 비어있으면 기본값 반환
	if len(bto.blockTimeHistory) == 0 {
		return map[string]interface{}{
			"currentOptimalBlockTime": bto.currentOptimalBlockTime,
			"minBlockTime":           bto.minBlockTime,
			"maxBlockTime":           bto.maxBlockTime,
			"averageBlockTime":       bto.currentOptimalBlockTime,
			"blockTimeHistory":       []uint64{},
		}
	}

	// 평균 블록 시간 계산
	var sum uint64
	for _, time := range bto.blockTimeHistory {
		sum += time
	}
	avgBlockTime := sum / uint64(len(bto.blockTimeHistory))

	return map[string]interface{}{
		"currentOptimalBlockTime": bto.currentOptimalBlockTime,
		"minBlockTime":           bto.minBlockTime,
		"maxBlockTime":           bto.maxBlockTime,
		"averageBlockTime":       avgBlockTime,
		"blockTimeHistory":       bto.blockTimeHistory,
	}
}

// ApplyToConfig는 최적화된 블록 시간을 Eirene 설정에 적용합니다.
func (bto *BlockTimeOptimizer) ApplyToConfig(config *params.EireneConfig) {
	optimalTime := bto.GetOptimalBlockTime()
	
	// 설정 업데이트
	config.Period = optimalTime
	
	bto.logger.Info("Applied optimal block time to config", "blockTime", optimalTime)
} 