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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/params"
)

// TestNewBlockTimeOptimizer는 BlockTimeOptimizer 생성을 테스트합니다.
func TestNewBlockTimeOptimizer(t *testing.T) {
	// 기본 설정으로 생성
	optimizer := NewBlockTimeOptimizer(nil)
	assert.NotNil(t, optimizer)
	assert.Equal(t, uint64(4), optimizer.GetOptimalBlockTime())
	
	// 사용자 정의 설정으로 생성
	config := &BlockTimeOptimizerConfig{
		DefaultBlockTime:        3,
		MinBlockTime:            1,
		MaxBlockTime:            5,
		AdjustmentInterval:      50,
		AdjustmentFactor:        0.1,
		NetworkLoadThreshold:    0.7,
		TxThroughputThreshold:   500,
		ValidatorPerformanceMin: 0.8,
		HistorySize:             500,
	}
	
	optimizer = NewBlockTimeOptimizer(config)
	assert.NotNil(t, optimizer)
	assert.Equal(t, uint64(3), optimizer.GetOptimalBlockTime())
}

// TestRecordBlockTime는 블록 시간 기록 기능을 테스트합니다.
func TestRecordBlockTime(t *testing.T) {
	optimizer := NewBlockTimeOptimizer(nil)
	
	// 블록 시간 기록
	optimizer.RecordBlockTime(3)
	optimizer.RecordBlockTime(4)
	optimizer.RecordBlockTime(5)
	
	// 블록 시간 통계 확인
	stats := optimizer.GetBlockTimeStats()
	history, ok := stats["blockTimeHistory"].([]uint64)
	assert.True(t, ok)
	assert.Equal(t, 3, len(history))
	assert.Equal(t, uint64(3), history[0])
	assert.Equal(t, uint64(4), history[1])
	assert.Equal(t, uint64(5), history[2])
	
	// 평균 블록 시간 확인
	avgTime, ok := stats["averageBlockTime"].(uint64)
	assert.True(t, ok)
	assert.Equal(t, uint64(4), avgTime)
}

// TestOptimizeBlockTime은 블록 시간 최적화 기능을 테스트합니다.
func TestOptimizeBlockTime(t *testing.T) {
	config := &BlockTimeOptimizerConfig{
		DefaultBlockTime:        4,
		MinBlockTime:            2,
		MaxBlockTime:            6,
		AdjustmentInterval:      10,
		AdjustmentFactor:        0.5,
		NetworkLoadThreshold:    0.5,
		TxThroughputThreshold:   1000,
		ValidatorPerformanceMin: 0.7,
		HistorySize:             100,
	}
	
	optimizer := NewBlockTimeOptimizer(config)
	
	// 네트워크 부하가 높은 경우 (블록 시간 증가 예상)
	networkStats := &NetworkStats{
		NetworkLoad: 0.8, // 임계값(0.5)보다 높음
	}
	optimizer.UpdateNetworkStats(networkStats)
	
	// 트랜잭션 처리량이 낮은 경우 (블록 시간 증가 예상)
	txStats := &TxThroughputStats{
		CurrentTPS: 500, // 임계값(1000)보다 낮음
	}
	optimizer.UpdateTxThroughputStats(txStats)
	
	// 검증자 성능이 좋은 경우 (블록 시간 감소 예상)
	validatorStats := &ValidatorStats{
		Address:     common.HexToAddress("0x1"),
		Performance: 0.9, // 임계값(0.7)보다 높음
	}
	optimizer.UpdateValidatorStats(validatorStats.Address, validatorStats)
	
	// 블록 번호가 조정 간격의 배수인 경우 최적화 수행
	blockTime := optimizer.OptimizeBlockTime(10)
	
	// 네트워크 부하와 트랜잭션 처리량이 블록 시간을 증가시키고,
	// 검증자 성능이 블록 시간을 감소시키므로, 전체적으로는 약간 증가 예상
	assert.True(t, blockTime >= 4, "Block time should increase due to high network load and low tx throughput")
	assert.True(t, blockTime <= 6, "Block time should not exceed max block time")
	
	// 블록 번호가 조정 간격의 배수가 아닌 경우 최적화 수행하지 않음
	currentOptimal := optimizer.GetOptimalBlockTime()
	blockTime = optimizer.OptimizeBlockTime(11)
	assert.Equal(t, currentOptimal, blockTime)
}

// TestCalculateNetworkLoadFactor는 네트워크 부하 기반 조정 계수 계산을 테스트합니다.
func TestCalculateNetworkLoadFactor(t *testing.T) {
	optimizer := NewBlockTimeOptimizer(nil)
	
	// 네트워크 부하가 임계값보다 높은 경우 (양수 반환 예상)
	networkStats := &NetworkStats{
		NetworkLoad: 0.9, // 임계값(0.8)보다 높음
	}
	optimizer.UpdateNetworkStats(networkStats)
	factor := optimizer.calculateNetworkLoadFactor()
	assert.True(t, factor > 0, "Network load factor should be positive when load is above threshold")
	
	// 네트워크 부하가 임계값보다 낮은 경우 (음수 반환 예상)
	networkStats.NetworkLoad = 0.7 // 임계값(0.8)보다 낮음
	optimizer.UpdateNetworkStats(networkStats)
	factor = optimizer.calculateNetworkLoadFactor()
	assert.True(t, factor < 0, "Network load factor should be negative when load is below threshold")
	
	// 네트워크 부하가 임계값과 같은 경우 (0 반환 예상)
	networkStats.NetworkLoad = 0.8 // 임계값(0.8)과 같음
	optimizer.UpdateNetworkStats(networkStats)
	factor = optimizer.calculateNetworkLoadFactor()
	assert.Equal(t, float64(0), factor, "Network load factor should be zero when load equals threshold")
}

// TestCalculateTxThroughputFactor는 트랜잭션 처리량 기반 조정 계수 계산을 테스트합니다.
func TestCalculateTxThroughputFactor(t *testing.T) {
	optimizer := NewBlockTimeOptimizer(nil)
	
	// 트랜잭션 처리량이 임계값보다 높은 경우 (음수 반환 예상)
	txStats := &TxThroughputStats{
		CurrentTPS: 1500, // 임계값(1000)보다 높음
	}
	optimizer.UpdateTxThroughputStats(txStats)
	factor := optimizer.calculateTxThroughputFactor()
	assert.True(t, factor < 0, "Tx throughput factor should be negative when TPS is above threshold")
	
	// 트랜잭션 처리량이 임계값보다 낮은 경우 (양수 반환 예상)
	txStats.CurrentTPS = 500 // 임계값(1000)보다 낮음
	optimizer.UpdateTxThroughputStats(txStats)
	factor = optimizer.calculateTxThroughputFactor()
	assert.True(t, factor > 0, "Tx throughput factor should be positive when TPS is below threshold")
	
	// 트랜잭션 처리량이 임계값과 같은 경우 (0 반환 예상)
	txStats.CurrentTPS = 1000 // 임계값(1000)과 같음
	optimizer.UpdateTxThroughputStats(txStats)
	factor = optimizer.calculateTxThroughputFactor()
	assert.Equal(t, float64(0), factor, "Tx throughput factor should be zero when TPS equals threshold")
}

// TestCalculateValidatorPerformanceFactor는 검증자 성능 기반 조정 계수 계산을 테스트합니다.
func TestCalculateValidatorPerformanceFactor(t *testing.T) {
	optimizer := NewBlockTimeOptimizer(nil)
	
	// 검증자 성능이 임계값보다 높은 경우 (음수 반환 예상)
	validatorStats := &ValidatorStats{
		Address:     common.HexToAddress("0x1"),
		Performance: 0.8, // 임계값(0.7)보다 높음
	}
	optimizer.UpdateValidatorStats(validatorStats.Address, validatorStats)
	factor := optimizer.calculateValidatorPerformanceFactor()
	assert.True(t, factor < 0, "Validator performance factor should be negative when performance is above threshold")
	
	// 검증자 성능이 임계값보다 낮은 경우 (양수 반환 예상)
	validatorStats.Performance = 0.6 // 임계값(0.7)보다 낮음
	optimizer.UpdateValidatorStats(validatorStats.Address, validatorStats)
	factor = optimizer.calculateValidatorPerformanceFactor()
	assert.True(t, factor > 0, "Validator performance factor should be positive when performance is below threshold")
	
	// 검증자 성능이 임계값과 같은 경우 (0 반환 예상)
	validatorStats.Performance = 0.7 // 임계값(0.7)과 같음
	optimizer.UpdateValidatorStats(validatorStats.Address, validatorStats)
	factor = optimizer.calculateValidatorPerformanceFactor()
	assert.Equal(t, float64(0), factor, "Validator performance factor should be zero when performance equals threshold")
}

// TestCalculateNewBlockTime은 새로운 블록 시간 계산을 테스트합니다.
func TestCalculateNewBlockTime(t *testing.T) {
	optimizer := NewBlockTimeOptimizer(nil)
	
	// 양수 조정 계수 (블록 시간 증가 예상)
	newTime := optimizer.calculateNewBlockTime(0.5)
	assert.True(t, newTime > 4, "Block time should increase with positive adjustment factor")
	assert.True(t, newTime <= 8, "Block time should not exceed max block time")
	
	// 음수 조정 계수 (블록 시간 감소 예상)
	newTime = optimizer.calculateNewBlockTime(-0.5)
	assert.True(t, newTime < 4, "Block time should decrease with negative adjustment factor")
	assert.True(t, newTime >= 2, "Block time should not be less than min block time")
	
	// 0 조정 계수 (블록 시간 유지 예상)
	newTime = optimizer.calculateNewBlockTime(0)
	assert.Equal(t, uint64(4), newTime, "Block time should not change with zero adjustment factor")
}

// TestApplyToConfig는 최적화된 블록 시간을 Eirene 설정에 적용하는 기능을 테스트합니다.
func TestApplyToConfig(t *testing.T) {
	optimizer := NewBlockTimeOptimizer(nil)
	
	// 블록 시간 최적화
	optimizer.RecordBlockTime(3)
	optimizer.RecordBlockTime(5)
	optimizer.OptimizeBlockTime(10)
	
	// Eirene 설정에 적용
	config := &params.EireneConfig{
		Period: 15, // 기존 블록 시간
	}
	optimizer.ApplyToConfig(config)
	
	// 설정이 업데이트되었는지 확인
	assert.Equal(t, optimizer.GetOptimalBlockTime(), config.Period)
} 