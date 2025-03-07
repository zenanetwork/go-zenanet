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
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
)

// TestNewValidatorSelector는 ValidatorSelector 생성을 테스트합니다.
func TestNewValidatorSelector(t *testing.T) {
	// 기본 설정으로 생성
	selector := NewValidatorSelector(nil)
	assert.NotNil(t, selector)
	assert.NotNil(t, selector.config)
	assert.NotNil(t, selector.validatorStats)
	assert.NotNil(t, selector.selectionHistory)
	
	// 사용자 정의 설정으로 생성
	config := &ValidatorSelectorConfig{
		MaxValidators:        50,
		MinVotingPower:       big.NewInt(500),
		PerformanceWeight:    0.4,
		RandomnessWeight:     0.1,
		StakeWeight:          0.4,
		HistoricalWeight:     0.1,
		MissedBlockPenalty:   0.2,
		LatePropagationPenalty: 0.1,
		PerformanceDecay:     0.9,
		RandomSeed:           12345,
		HistorySize:          500,
		HistoryDecay:         0.8,
	}
	
	selector = NewValidatorSelector(config)
	assert.NotNil(t, selector)
	assert.Equal(t, config, selector.config)
}

// TestSelectNextProposer는 다음 블록 생성자 선택을 테스트합니다.
func TestSelectNextProposer(t *testing.T) {
	selector := NewValidatorSelector(nil)
	
	// 검증자 생성
	validators := []utils.ValidatorInterface{
		createTestValidator("0x1111111111111111111111111111111111111111", 1000),
		createTestValidator("0x2222222222222222222222222222222222222222", 2000),
		createTestValidator("0x3333333333333333333333333333333333333333", 3000),
	}
	
	// 빈 검증자 목록으로 테스트
	proposer := selector.SelectNextProposer([]utils.ValidatorInterface{}, 1, common.Hash{})
	assert.Nil(t, proposer)
	
	// 검증자가 1명인 경우 테스트
	proposer = selector.SelectNextProposer(validators[:1], 1, common.Hash{})
	assert.Equal(t, validators[0].GetAddress(), proposer.GetAddress())
	
	// 여러 검증자가 있는 경우 테스트
	proposer = selector.SelectNextProposer(validators, 1, common.Hash{})
	assert.NotNil(t, proposer)
	
	// 검증자 성능 통계 업데이트
	for _, v := range validators {
		selector.UpdateValidatorPerformance(v.GetAddress(), false, time.Millisecond*100)
	}
	
	// 두 번째 블록 생성자 선택
	proposer2 := selector.SelectNextProposer(validators, 2, common.Hash{})
	assert.NotNil(t, proposer2)
	
	// 세 번째 블록 생성자 선택
	proposer3 := selector.SelectNextProposer(validators, 3, common.Hash{})
	assert.NotNil(t, proposer3)
	
	// 선택 히스토리 확인
	stats := selector.GetAllValidatorPerformances()
	assert.Equal(t, uint64(3), stats[proposer3.GetAddress()].LastSelected)
}

// TestSelectValidatorsForCommittee는 위원회 검증자 선택을 테스트합니다.
func TestSelectValidatorsForCommittee(t *testing.T) {
	selector := NewValidatorSelector(nil)
	
	// 검증자 생성
	validators := []utils.ValidatorInterface{
		createTestValidator("0x1111111111111111111111111111111111111111", 1000),
		createTestValidator("0x2222222222222222222222222222222222222222", 2000),
		createTestValidator("0x3333333333333333333333333333333333333333", 3000),
		createTestValidator("0x4444444444444444444444444444444444444444", 4000),
		createTestValidator("0x5555555555555555555555555555555555555555", 5000),
	}
	
	// 빈 검증자 목록으로 테스트
	committee := selector.SelectValidatorsForCommittee([]utils.ValidatorInterface{}, 1, common.Hash{}, 3)
	assert.Empty(t, committee)
	
	// 위원회 크기보다 작은 검증자 목록으로 테스트
	committee = selector.SelectValidatorsForCommittee(validators[:2], 1, common.Hash{}, 3)
	assert.Equal(t, 2, len(committee))
	
	// 위원회 크기보다 큰 검증자 목록으로 테스트
	committee = selector.SelectValidatorsForCommittee(validators, 1, common.Hash{}, 3)
	assert.Equal(t, 3, len(committee))
	
	// 검증자 성능 통계 업데이트
	for _, v := range validators {
		selector.UpdateValidatorPerformance(v.GetAddress(), false, time.Millisecond*100)
	}
	
	// 두 번째 위원회 선택
	committee2 := selector.SelectValidatorsForCommittee(validators, 2, common.Hash{}, 3)
	assert.Equal(t, 3, len(committee2))
}

// TestUpdateValidatorPerformance는 검증자 성능 통계 업데이트를 테스트합니다.
func TestUpdateValidatorPerformance(t *testing.T) {
	selector := NewValidatorSelector(nil)
	
	// 검증자 주소
	address := common.HexToAddress("0x1111111111111111111111111111111111111111")
	
	// 초기 성능 통계 확인
	stats := selector.GetValidatorPerformance(address)
	assert.Nil(t, stats)
	
	// 블록 생성 성공 업데이트
	selector.UpdateValidatorPerformance(address, false, time.Millisecond*100)
	
	// 업데이트된 성능 통계 확인
	stats = selector.GetValidatorPerformance(address)
	assert.NotNil(t, stats)
	assert.Equal(t, address, stats.Address)
	assert.Equal(t, 1, stats.BlocksProposed)
	assert.Equal(t, 0, stats.BlocksMissed)
	assert.Equal(t, time.Millisecond*100, stats.AvgPropagationTime)
	assert.InDelta(t, 1.0, stats.PerformanceScore, 0.01)
	
	// 블록 생성 실패 업데이트
	selector.UpdateValidatorPerformance(address, true, time.Duration(0))
	
	// 업데이트된 성능 통계 확인
	stats = selector.GetValidatorPerformance(address)
	assert.Equal(t, 1, stats.BlocksProposed)
	assert.Equal(t, 1, stats.BlocksMissed)
	assert.Less(t, stats.PerformanceScore, 1.0)
	
	// 블록 전파 지연 업데이트
	selector.UpdateValidatorPerformance(address, false, time.Second*5)
	
	// 업데이트된 성능 통계 확인
	stats = selector.GetValidatorPerformance(address)
	assert.Equal(t, 2, stats.BlocksProposed)
	assert.Equal(t, 1, stats.BlocksMissed)
	assert.Less(t, stats.PerformanceScore, 0.9)
}

// TestResetValidatorPerformance는 검증자 성능 통계 초기화를 테스트합니다.
func TestResetValidatorPerformance(t *testing.T) {
	selector := NewValidatorSelector(nil)
	
	// 검증자 주소
	address := common.HexToAddress("0x1111111111111111111111111111111111111111")
	
	// 성능 통계 업데이트
	selector.UpdateValidatorPerformance(address, false, time.Millisecond*100)
	selector.UpdateValidatorPerformance(address, true, time.Duration(0))
	
	// 업데이트된 성능 통계 확인
	stats := selector.GetValidatorPerformance(address)
	assert.Equal(t, 1, stats.BlocksProposed)
	assert.Equal(t, 1, stats.BlocksMissed)
	assert.Less(t, stats.PerformanceScore, 1.0)
	
	// 성능 통계 초기화
	selector.ResetValidatorPerformance(address)
	
	// 초기화된 성능 통계 확인
	stats = selector.GetValidatorPerformance(address)
	assert.Equal(t, 0, stats.BlocksProposed)
	assert.Equal(t, 0, stats.BlocksMissed)
	assert.Equal(t, time.Duration(0), stats.AvgPropagationTime)
	assert.Equal(t, 1.0, stats.PerformanceScore)
}

// TestCalculateStakeScore는 스테이크 점수 계산을 테스트합니다.
func TestCalculateStakeScore(t *testing.T) {
	// 검증자 생성
	validators := []utils.ValidatorInterface{
		createTestValidator("0x1111111111111111111111111111111111111111", 1000),
		createTestValidator("0x2222222222222222222222222222222222222222", 2000),
		createTestValidator("0x3333333333333333333333333333333333333333", 3000),
		createTestValidator("0x4444444444444444444444444444444444444444", 4000),
	}
	
	// 총 스테이크: 10000
	
	// 첫 번째 검증자 점수 계산 (1000/10000 = 0.1)
	score1 := calculateStakeScore(validators[0], validators)
	assert.InDelta(t, 0.1, score1, 0.001)
	
	// 두 번째 검증자 점수 계산 (2000/10000 = 0.2)
	score2 := calculateStakeScore(validators[1], validators)
	assert.InDelta(t, 0.2, score2, 0.001)
	
	// 세 번째 검증자 점수 계산 (3000/10000 = 0.3)
	score3 := calculateStakeScore(validators[2], validators)
	assert.InDelta(t, 0.3, score3, 0.001)
	
	// 네 번째 검증자 점수 계산 (4000/10000 = 0.4)
	score4 := calculateStakeScore(validators[3], validators)
	assert.InDelta(t, 0.4, score4, 0.001)
}

// TestCalculateRandomScore는 무작위 점수 계산을 테스트합니다.
func TestCalculateRandomScore(t *testing.T) {
	// 검증자 주소
	address1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
	address2 := common.HexToAddress("0x2222222222222222222222222222222222222222")
	
	// 블록 번호와 부모 해시
	blockNumber := uint64(1)
	parentHash := common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001")
	
	// 시드
	seed := int64(12345)
	
	// 첫 번째 검증자 점수 계산
	score1 := calculateRandomScore(address1, blockNumber, parentHash, seed)
	assert.GreaterOrEqual(t, score1, 0.0)
	assert.LessOrEqual(t, score1, 1.0)
	
	// 두 번째 검증자 점수 계산
	score2 := calculateRandomScore(address2, blockNumber, parentHash, seed)
	assert.GreaterOrEqual(t, score2, 0.0)
	assert.LessOrEqual(t, score2, 1.0)
	
	// 같은 입력에 대해 같은 점수가 나오는지 확인
	score1Again := calculateRandomScore(address1, blockNumber, parentHash, seed)
	assert.Equal(t, score1, score1Again)
	
	// 다른 블록 번호에 대해 다른 점수가 나오는지 확인
	score1Different := calculateRandomScore(address1, blockNumber+1, parentHash, seed)
	assert.NotEqual(t, score1, score1Different)
}

// TestCalculateHistoryScore는 히스토리 점수 계산을 테스트합니다.
func TestCalculateHistoryScore(t *testing.T) {
	// 검증자 성능 통계 생성
	stats1 := &ValidatorPerformance{
		Address:        common.HexToAddress("0x1111111111111111111111111111111111111111"),
		LastSelected:   0, // 한 번도 선택되지 않음
		SelectionCount: 0,
	}
	
	stats2 := &ValidatorPerformance{
		Address:        common.HexToAddress("0x2222222222222222222222222222222222222222"),
		LastSelected:   900, // 최근에 선택됨
		SelectionCount: 1,
	}
	
	stats3 := &ValidatorPerformance{
		Address:        common.HexToAddress("0x3333333333333333333333333333333333333333"),
		LastSelected:   100, // 오래 전에 선택됨
		SelectionCount: 10, // 자주 선택됨
	}
	
	// 현재 블록 번호
	currentBlock := uint64(1000)
	
	// 감쇠 계수
	decayFactor := 0.9
	
	// 첫 번째 검증자 점수 계산 (한 번도 선택되지 않음 -> 최대 점수)
	score1 := calculateHistoryScore(stats1, currentBlock, decayFactor)
	assert.Equal(t, 1.0, score1)
	
	// 두 번째 검증자 점수 계산 (최근에 선택됨 -> 낮은 점수)
	score2 := calculateHistoryScore(stats2, currentBlock, decayFactor)
	assert.Less(t, score2, 0.5)
	
	// 세 번째 검증자 점수 계산 (오래 전에 선택됨 -> 높은 점수, 자주 선택됨 -> 페널티)
	score3 := calculateHistoryScore(stats3, currentBlock, decayFactor)
	assert.Greater(t, score3, score2) // 오래 전에 선택되어 점수가 높지만
	assert.Less(t, score3, 1.0)       // 자주 선택되어 페널티가 적용됨
}

// TestSelectByScore는 점수에 따른 검증자 선택을 테스트합니다.
func TestSelectByScore(t *testing.T) {
	// 검증자 생성
	validators := []utils.ValidatorInterface{
		createTestValidator("0x1111111111111111111111111111111111111111", 1000),
		createTestValidator("0x2222222222222222222222222222222222222222", 2000),
		createTestValidator("0x3333333333333333333333333333333333333333", 3000),
	}
	
	// 점수 맵 생성
	scores := map[common.Address]float64{
		validators[0].GetAddress(): 0.1,
		validators[1].GetAddress(): 0.2,
		validators[2].GetAddress(): 0.7,
	}
	
	// 총 점수
	totalScore := 1.0
	
	// 여러 번 선택하여 확률 검증
	selections := make(map[common.Address]int)
	
	for i := 0; i < 1000; i++ {
		selected := selectByScore(validators, scores, totalScore)
		selections[selected.GetAddress()]++
	}
	
	// 점수가 높은 검증자가 더 자주 선택되는지 확인
	assert.Greater(t, selections[validators[2].GetAddress()], selections[validators[1].GetAddress()])
	assert.Greater(t, selections[validators[1].GetAddress()], selections[validators[0].GetAddress()])
	
	// 총 점수가 0인 경우 테스트
	selected := selectByScore(validators, scores, 0.0)
	assert.NotNil(t, selected)
}

// TestCalculateNewAverage는 새로운 평균값 계산을 테스트합니다.
func TestCalculateNewAverage(t *testing.T) {
	// 초기 평균값
	oldAvg := time.Millisecond * 100
	
	// 새로운 값
	newValue := time.Millisecond * 200
	
	// 첫 번째 값인 경우
	newAvg := calculateNewAverage(oldAvg, newValue, 1)
	assert.Equal(t, newValue, newAvg)
	
	// 두 번째 값인 경우 (100 + 200) / 2 = 150
	newAvg = calculateNewAverage(oldAvg, newValue, 2)
	assert.Equal(t, time.Millisecond*150, newAvg)
	
	// 세 번째 값인 경우 (100*2 + 200) / 3 = 133.33...
	newAvg = calculateNewAverage(oldAvg, newValue, 3)
	assert.InDelta(t, float64(time.Millisecond*133), float64(newAvg), float64(time.Millisecond))
} 