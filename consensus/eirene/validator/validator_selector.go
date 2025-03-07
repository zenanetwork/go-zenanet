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

package validator

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"math/big"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/log"
)

// ValidatorSelector는 검증자 선택 알고리즘을 구현합니다.
// 이 모듈은 다양한 요소를 고려하여 블록 생성자 및 투표 검증자를 선택합니다.
type ValidatorSelector struct {
	// 설정
	config *ValidatorSelectorConfig

	// 검증자 성능 통계
	validatorStats map[common.Address]*ValidatorPerformance

	// 검증자 선택 히스토리
	selectionHistory map[uint64][]common.Address

	// 동시성 제어
	lock sync.RWMutex

	// 로깅
	logger log.Logger
}

// ValidatorSelectorConfig는 검증자 선택 알고리즘의 설정을 정의합니다.
type ValidatorSelectorConfig struct {
	// 기본 설정
	MaxValidators        int     `json:"maxValidators"`        // 최대 검증자 수
	MinVotingPower       *big.Int `json:"minVotingPower"`      // 최소 투표력
	PerformanceWeight    float64 `json:"performanceWeight"`    // 성능 가중치 (0.0 ~ 1.0)
	RandomnessWeight     float64 `json:"randomnessWeight"`     // 무작위성 가중치 (0.0 ~ 1.0)
	StakeWeight          float64 `json:"stakeWeight"`          // 스테이크 가중치 (0.0 ~ 1.0)
	HistoricalWeight     float64 `json:"historicalWeight"`     // 과거 선택 가중치 (0.0 ~ 1.0)
	
	// 성능 관련 설정
	MissedBlockPenalty   float64 `json:"missedBlockPenalty"`   // 블록 생성 실패 페널티
	LatePropagationPenalty float64 `json:"latePropagationPenalty"` // 블록 전파 지연 페널티
	PerformanceDecay     float64 `json:"performanceDecay"`     // 성능 점수 감쇠 계수
	
	// 무작위성 관련 설정
	RandomSeed           int64   `json:"randomSeed"`           // 무작위 시드
	
	// 히스토리 관련 설정
	HistorySize          int     `json:"historySize"`          // 히스토리 크기
	HistoryDecay         float64 `json:"historyDecay"`         // 히스토리 가중치 감쇠 계수
}

// ValidatorPerformance는 검증자의 성능 통계를 나타냅니다.
type ValidatorPerformance struct {
	// 검증자 정보
	Address     common.Address // 검증자 주소
	
	// 성능 지표
	PerformanceScore float64   // 성능 점수 (0.0 ~ 1.0)
	BlocksProposed   int       // 제안한 블록 수
	BlocksMissed     int       // 놓친 블록 수
	AvgPropagationTime time.Duration // 평균 블록 전파 시간
	
	// 선택 히스토리
	LastSelected     uint64    // 마지막으로 선택된 블록 번호
	SelectionCount   int       // 선택된 횟수
}

// NewValidatorSelector는 새로운 검증자 선택기를 생성합니다.
func NewValidatorSelector(config *ValidatorSelectorConfig) *ValidatorSelector {
	if config == nil {
		config = &ValidatorSelectorConfig{
			MaxValidators:        100,
			MinVotingPower:       big.NewInt(1000),
			PerformanceWeight:    0.3,
			RandomnessWeight:     0.2,
			StakeWeight:          0.4,
			HistoricalWeight:     0.1,
			MissedBlockPenalty:   0.1,
			LatePropagationPenalty: 0.05,
			PerformanceDecay:     0.95,
			RandomSeed:           time.Now().UnixNano(),
			HistorySize:          1000,
			HistoryDecay:         0.9,
		}
	}
	
	return &ValidatorSelector{
		config:           config,
		validatorStats:   make(map[common.Address]*ValidatorPerformance),
		selectionHistory: make(map[uint64][]common.Address),
		logger:           log.New("module", "validator-selector"),
	}
}

// SelectNextProposer는 다음 블록 생성자를 선택합니다.
func (vs *ValidatorSelector) SelectNextProposer(
	validators []utils.ValidatorInterface,
	blockNumber uint64,
	parentHash common.Hash,
) utils.ValidatorInterface {
	vs.lock.Lock()
	defer vs.lock.Unlock()
	
	// 검증자가 없으면 nil 반환
	if len(validators) == 0 {
		return nil
	}
	
	// 검증자가 1명이면 그 검증자 반환
	if len(validators) == 1 {
		return validators[0]
	}
	
	// 검증자 점수 계산
	scores := make(map[common.Address]float64)
	totalScore := 0.0
	
	for _, validator := range validators {
		address := validator.GetAddress()
		
		// 검증자 성능 통계 가져오기 (없으면 생성)
		stats, ok := vs.validatorStats[address]
		if !ok {
			stats = &ValidatorPerformance{
				Address:          address,
				PerformanceScore: 1.0, // 초기 성능 점수는 1.0 (최대)
			}
			vs.validatorStats[address] = stats
		}
		
		// 스테이크 점수 계산
		stakeScore := calculateStakeScore(validator, validators)
		
		// 성능 점수 계산
		perfScore := stats.PerformanceScore
		
		// 무작위 점수 계산
		randomScore := calculateRandomScore(address, blockNumber, parentHash, vs.config.RandomSeed)
		
		// 히스토리 점수 계산
		historyScore := calculateHistoryScore(stats, blockNumber, vs.config.HistoryDecay)
		
		// 최종 점수 계산
		finalScore := (stakeScore * vs.config.StakeWeight) +
			(perfScore * vs.config.PerformanceWeight) +
			(randomScore * vs.config.RandomnessWeight) +
			(historyScore * vs.config.HistoricalWeight)
		
		scores[address] = finalScore
		totalScore += finalScore
	}
	
	// 점수에 따라 검증자 선택
	selectedValidator := selectByScore(validators, scores, totalScore)
	
	// 선택된 검증자의 통계 업데이트
	if selectedValidator != nil {
		address := selectedValidator.GetAddress()
		stats := vs.validatorStats[address]
		stats.LastSelected = blockNumber
		stats.SelectionCount++
		
		// 선택 히스토리 업데이트
		vs.updateSelectionHistory(blockNumber, address)
	}
	
	return selectedValidator
}

// SelectValidatorsForCommittee는 위원회 검증자를 선택합니다.
func (vs *ValidatorSelector) SelectValidatorsForCommittee(
	validators []utils.ValidatorInterface,
	blockNumber uint64,
	parentHash common.Hash,
	committeeSize int,
) []utils.ValidatorInterface {
	vs.lock.Lock()
	defer vs.lock.Unlock()
	
	// 검증자가 없으면 빈 슬라이스 반환
	if len(validators) == 0 {
		return []utils.ValidatorInterface{}
	}
	
	// 검증자 수가 위원회 크기보다 작으면 모든 검증자 반환
	if len(validators) <= committeeSize {
		return validators
	}
	
	// 검증자 점수 계산
	scores := make(map[common.Address]float64)
	
	for _, validator := range validators {
		address := validator.GetAddress()
		
		// 검증자 성능 통계 가져오기 (없으면 생성)
		stats, ok := vs.validatorStats[address]
		if !ok {
			stats = &ValidatorPerformance{
				Address:          address,
				PerformanceScore: 1.0, // 초기 성능 점수는 1.0 (최대)
			}
			vs.validatorStats[address] = stats
		}
		
		// 스테이크 점수 계산
		stakeScore := calculateStakeScore(validator, validators)
		
		// 성능 점수 계산
		perfScore := stats.PerformanceScore
		
		// 무작위 점수 계산
		randomScore := calculateRandomScore(address, blockNumber, parentHash, vs.config.RandomSeed)
		
		// 히스토리 점수 계산
		historyScore := calculateHistoryScore(stats, blockNumber, vs.config.HistoryDecay)
		
		// 최종 점수 계산
		finalScore := (stakeScore * vs.config.StakeWeight) +
			(perfScore * vs.config.PerformanceWeight) +
			(randomScore * vs.config.RandomnessWeight) +
			(historyScore * vs.config.HistoricalWeight)
		
		scores[address] = finalScore
	}
	
	// 점수에 따라 검증자 정렬
	sortedValidators := make([]utils.ValidatorInterface, len(validators))
	copy(sortedValidators, validators)
	
	sort.Slice(sortedValidators, func(i, j int) bool {
		return scores[sortedValidators[i].GetAddress()] > scores[sortedValidators[j].GetAddress()]
	})
	
	// 상위 N개 검증자 선택
	committee := sortedValidators
	if len(sortedValidators) > committeeSize {
		committee = sortedValidators[:committeeSize]
	}
	
	// 선택된 검증자의 통계 업데이트
	for _, validator := range committee {
		address := validator.GetAddress()
		stats := vs.validatorStats[address]
		stats.LastSelected = blockNumber
		stats.SelectionCount++
	}
	
	return committee
}

// UpdateValidatorPerformance는 검증자의 성능 통계를 업데이트합니다.
func (vs *ValidatorSelector) UpdateValidatorPerformance(
	address common.Address,
	missedBlock bool,
	propagationTime time.Duration,
) {
	vs.lock.Lock()
	defer vs.lock.Unlock()
	
	// 검증자 성능 통계 가져오기 (없으면 생성)
	stats, ok := vs.validatorStats[address]
	if !ok {
		stats = &ValidatorPerformance{
			Address:          address,
			PerformanceScore: 1.0, // 초기 성능 점수는 1.0 (최대)
		}
		vs.validatorStats[address] = stats
	}
	
	// 블록 생성 통계 업데이트
	if missedBlock {
		stats.BlocksMissed++
		// 블록 생성 실패 페널티 적용
		stats.PerformanceScore *= (1.0 - vs.config.MissedBlockPenalty)
	} else {
		stats.BlocksProposed++
		
		// 블록 전파 시간 업데이트
		stats.AvgPropagationTime = calculateNewAverage(
			stats.AvgPropagationTime,
			propagationTime,
			stats.BlocksProposed,
		)
		
		// 블록 전파 지연 페널티 적용 (전파 시간이 너무 길면)
		if propagationTime > time.Second*3 {
			stats.PerformanceScore *= (1.0 - vs.config.LatePropagationPenalty)
		}
	}
	
	// 성능 점수 범위 제한 (0.0 ~ 1.0)
	stats.PerformanceScore = math.Max(0.0, math.Min(1.0, stats.PerformanceScore))
	
	// 성능 점수 감쇠 적용 (시간이 지남에 따라 점수가 회복되도록)
	stats.PerformanceScore = stats.PerformanceScore*vs.config.PerformanceDecay + (1.0-vs.config.PerformanceDecay)
}

// GetValidatorPerformance는 검증자의 성능 통계를 반환합니다.
func (vs *ValidatorSelector) GetValidatorPerformance(address common.Address) *ValidatorPerformance {
	vs.lock.RLock()
	defer vs.lock.RUnlock()
	
	stats, ok := vs.validatorStats[address]
	if !ok {
		return nil
	}
	
	return stats
}

// GetAllValidatorPerformances는 모든 검증자의 성능 통계를 반환합니다.
func (vs *ValidatorSelector) GetAllValidatorPerformances() map[common.Address]*ValidatorPerformance {
	vs.lock.RLock()
	defer vs.lock.RUnlock()
	
	// 맵 복사
	result := make(map[common.Address]*ValidatorPerformance)
	for addr, stats := range vs.validatorStats {
		result[addr] = stats
	}
	
	return result
}

// ResetValidatorPerformance는 검증자의 성능 통계를 초기화합니다.
func (vs *ValidatorSelector) ResetValidatorPerformance(address common.Address) {
	vs.lock.Lock()
	defer vs.lock.Unlock()
	
	stats, ok := vs.validatorStats[address]
	if !ok {
		return
	}
	
	// 성능 점수 초기화
	stats.PerformanceScore = 1.0
	stats.BlocksProposed = 0
	stats.BlocksMissed = 0
	stats.AvgPropagationTime = 0
}

// updateSelectionHistory는 검증자 선택 히스토리를 업데이트합니다.
func (vs *ValidatorSelector) updateSelectionHistory(blockNumber uint64, address common.Address) {
	// 히스토리 크기 제한
	if len(vs.selectionHistory) > vs.config.HistorySize {
		// 가장 오래된 히스토리 삭제
		var oldestBlock uint64 = math.MaxUint64
		for block := range vs.selectionHistory {
			if block < oldestBlock {
				oldestBlock = block
			}
		}
		delete(vs.selectionHistory, oldestBlock)
	}
	
	// 히스토리 업데이트
	vs.selectionHistory[blockNumber] = []common.Address{address}
}

// calculateStakeScore는 검증자의 스테이크 점수를 계산합니다.
func calculateStakeScore(validator utils.ValidatorInterface, allValidators []utils.ValidatorInterface) float64 {
	// 총 스테이크 계산
	totalStake := big.NewInt(0)
	for _, v := range allValidators {
		totalStake = new(big.Int).Add(totalStake, v.GetVotingPower())
	}
	
	// 스테이크 비율 계산
	if totalStake.Cmp(big.NewInt(0)) == 0 {
		return 0.0
	}
	
	stakeRatio := new(big.Float).Quo(
		new(big.Float).SetInt(validator.GetVotingPower()),
		new(big.Float).SetInt(totalStake),
	)
	
	score, _ := stakeRatio.Float64()
	return score
}

// calculateRandomScore는 검증자의 무작위 점수를 계산합니다.
func calculateRandomScore(
	address common.Address,
	blockNumber uint64,
	parentHash common.Hash,
	seed int64,
) float64 {
	// 블록 번호, 부모 해시, 주소, 시드를 조합하여 무작위 점수 생성
	hasher := sha256.New()
	hasher.Write(parentHash.Bytes())
	hasher.Write(address.Bytes())
	binary.Write(hasher, binary.BigEndian, blockNumber)
	binary.Write(hasher, binary.BigEndian, seed)
	
	hash := hasher.Sum(nil)
	
	// 해시의 첫 8바이트를 uint64로 변환
	randomValue := binary.BigEndian.Uint64(hash[:8])
	
	// 0.0 ~ 1.0 범위로 정규화
	return float64(randomValue) / float64(math.MaxUint64)
}

// calculateHistoryScore는 검증자의 히스토리 점수를 계산합니다.
func calculateHistoryScore(stats *ValidatorPerformance, currentBlock uint64, decayFactor float64) float64 {
	if stats.LastSelected == 0 {
		// 한 번도 선택되지 않은 경우 최대 점수 부여
		return 1.0
	}
	
	// 마지막 선택 이후 경과 블록 수
	blocksSinceLastSelection := currentBlock - stats.LastSelected
	
	// 경과 블록 수에 따라 점수 계산 (오래 전에 선택될수록 높은 점수)
	score := 1.0 - math.Exp(-float64(blocksSinceLastSelection)/1000.0)
	
	// 선택 횟수에 따른 페널티 (자주 선택될수록 낮은 점수)
	selectionPenalty := math.Min(0.5, float64(stats.SelectionCount)/1000.0)
	
	return score * (1.0 - selectionPenalty)
}

// selectByScore는 점수에 따라 검증자를 선택합니다.
func selectByScore(
	validators []utils.ValidatorInterface,
	scores map[common.Address]float64,
	totalScore float64,
) utils.ValidatorInterface {
	if totalScore <= 0 {
		// 총 점수가 0 이하면 무작위 선택
		return validators[rand.Intn(len(validators))]
	}
	
	// 룰렛 휠 선택 알고리즘
	r := rand.Float64() * totalScore
	cumulativeScore := 0.0
	
	for _, validator := range validators {
		address := validator.GetAddress()
		cumulativeScore += scores[address]
		if r <= cumulativeScore {
			return validator
		}
	}
	
	// 여기까지 오면 마지막 검증자 선택
	return validators[len(validators)-1]
}

// calculateNewAverage는 새로운 평균값을 계산합니다.
func calculateNewAverage(oldAvg time.Duration, newValue time.Duration, count int) time.Duration {
	if count <= 1 {
		return newValue
	}
	
	// 가중 평균 계산
	newAvg := time.Duration(
		(float64(oldAvg)*float64(count-1) + float64(newValue)) / float64(count),
	)
	
	return newAvg
} 