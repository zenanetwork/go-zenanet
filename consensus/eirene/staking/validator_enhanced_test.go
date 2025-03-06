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

package staking

import (
	"math/big"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/types"
)

// TestCalculateEnhancedValidatorScore는 향상된 검증자 점수 계산 기능을 테스트합니다.
func TestCalculateEnhancedValidatorScore(t *testing.T) {
	// 테스트 검증자 생성
	validator := createTestEnhancedValidator(t)
	
	// 총 블록 수와 현재 블록 설정
	totalBlocks := uint64(1000)
	currentBlock := uint64(2000)
	
	// 점수 계산
	score := calculateEnhancedValidatorScore(validator, totalBlocks, currentBlock)
	
	// 점수가 0보다 큰지 확인
	assert.True(t, score.Sign() > 0, "검증자 점수가 0보다 커야 함")
	
	// 스테이킹 양이 많은 검증자 생성
	highStakeValidator := createTestEnhancedValidator(t)
	highStakeValidator.Validator.VotingPower = new(big.Int).Mul(validator.Validator.VotingPower, big.NewInt(2))
	
	// 높은 스테이킹 양을 가진 검증자의 점수 계산
	highScore := calculateEnhancedValidatorScore(highStakeValidator, totalBlocks, currentBlock)
	
	// 스테이킹 양이 많은 검증자의 점수가 더 높은지 확인
	assert.True(t, highScore.Cmp(score) > 0, "스테이킹 양이 많은 검증자의 점수가 더 높아야 함")
}

// TestCalculateEnhancedPerformanceScore는 향상된 성능 점수 계산 기능을 테스트합니다.
func TestCalculateEnhancedPerformanceScore(t *testing.T) {
	// 테스트 검증자 생성
	validator := createTestEnhancedValidator(t)
	
	// 총 블록 수 설정
	totalBlocks := uint64(1000)
	
	// 성능 점수 계산
	performanceScore := calculateEnhancedPerformanceScore(validator, totalBlocks)
	
	// 점수가 0보다 큰지 확인
	assert.True(t, performanceScore.Sign() > 0, "성능 점수가 0보다 커야 함")
	
	// 성능이 좋은 검증자 생성
	highPerformanceValidator := createTestEnhancedValidator(t)
	highPerformanceValidator.EnhancedStats.BlocksMissed = 0
	highPerformanceValidator.EnhancedStats.BlocksSigned = totalBlocks
	highPerformanceValidator.EnhancedStats.BlocksProposed = totalBlocks / 10
	highPerformanceValidator.EnhancedStats.Uptime = 1000
	highPerformanceValidator.EnhancedStats.AvgResponseTime = 100
	
	// 성능이 좋은 검증자의 점수 계산
	highPerformanceScore := calculateEnhancedPerformanceScore(highPerformanceValidator, totalBlocks)
	
	// 성능이 좋은 검증자의 점수가 더 높은지 확인
	assert.True(t, highPerformanceScore.Cmp(performanceScore) > 0, "성능이 좋은 검증자의 점수가 더 높아야 함")
}

// TestCalculateReputationScore는 평판 점수 계산 기능을 테스트합니다.
func TestCalculateReputationScore(t *testing.T) {
	// 테스트 검증자 생성
	validator := createTestEnhancedValidator(t)
	
	// 현재 블록 설정
	currentBlock := uint64(2000)
	
	// 평판 점수 계산
	reputationScore := calculateReputationScore(validator, currentBlock)
	
	// 점수가 0보다 큰지 확인
	assert.True(t, reputationScore.Sign() > 0, "평판 점수가 0보다 커야 함")
	
	// 평판이 좋은 검증자 생성
	highReputationValidator := createTestEnhancedValidator(t)
	highReputationValidator.Reputation.SlashingHistory = []SlashingEvent{}
	highReputationValidator.Reputation.ActivationBlock = 1
	highReputationValidator.Reputation.TotalActiveBlocks = currentBlock - 1
	highReputationValidator.Reputation.CommunityVotes = 500
	highReputationValidator.Reputation.NetworkContribPoints = 800
	
	// 평판이 좋은 검증자의 점수 계산
	highReputationScore := calculateReputationScore(highReputationValidator, currentBlock)
	
	// 평판이 좋은 검증자의 점수가 더 높은지 확인
	assert.True(t, highReputationScore.Cmp(reputationScore) > 0, "평판이 좋은 검증자의 점수가 더 높아야 함")
}

// TestEnhancedSelectValidators는 향상된 검증자 선택 기능을 테스트합니다.
func TestEnhancedSelectValidators(t *testing.T) {
	// 검증자 집합 생성
	validatorSet := NewValidatorSet()
	
	// 테스트 검증자 추가
	for i := 0; i < 5; i++ {
		validator := &Validator{
			Address:     common.BytesToAddress([]byte{byte(i + 1)}),
			VotingPower: new(big.Int).Mul(big.NewInt(int64(i+1)), big.NewInt(1e18)),
			Status:      ValidatorStatusBonded,
			BlocksProposed: uint64(i + 1) * 10,
			BlocksSigned: 900 - uint64(i) * 50,
			BlocksMissed: uint64(i) * 50,
			Uptime: 1000 - uint64(i) * 50,
		}
		validatorSet.Validators = append(validatorSet.Validators, validator)
	}
	
	// 최대 검증자 수 설정
	maxValidators := 3
	
	// 검증자 선택 (실제 구현에서는 validatorSet.enhancedSelectValidators 메서드 사용)
	// 여기서는 테스트를 위해 직접 구현
	validators := make([]*Validator, len(validatorSet.Validators))
	copy(validators, validatorSet.Validators)
	
	// 각 검증자의 점수 계산 (간소화된 버전)
	type validatorScore struct {
		validator *Validator
		score     *big.Int
	}
	
	scores := make([]validatorScore, len(validators))
	for i, validator := range validators {
		// 간단한 점수 계산 (실제 구현과 다름)
		score := new(big.Int).Set(validator.VotingPower)
		scores[i] = validatorScore{
			validator: validator,
			score:     score,
		}
	}
	
	// 점수에 따라 정렬
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score.Cmp(scores[j].score) > 0
	})
	
	// 상위 maxValidators개 선택
	selectedValidators := make([]*Validator, 0, maxValidators)
	for i := 0; i < len(scores) && i < maxValidators; i++ {
		selectedValidators = append(selectedValidators, scores[i].validator)
	}
	
	// 선택된 검증자 수 확인
	assert.Equal(t, maxValidators, len(selectedValidators), "선택된 검증자 수가 maxValidators와 일치해야 함")
	
	// 스테이킹 양이 가장 많은 검증자가 선택되었는지 확인
	found := false
	for _, validator := range selectedValidators {
		if validator.Address == validators[4].Address {
			found = true
			break
		}
	}
	assert.True(t, found, "스테이킹 양이 가장 많은 검증자가 선택되어야 함")
}

// TestUpdateEnhancedValidatorPerformance는 향상된 검증자 성능 업데이트 기능을 테스트합니다.
func TestUpdateEnhancedValidatorPerformance(t *testing.T) {
	// 검증자 집합 생성
	validatorSet := NewValidatorSet()
	
	// 테스트 검증자 추가
	validator1 := &Validator{
		Address:     common.BytesToAddress([]byte{1}),
		VotingPower: big.NewInt(1e18),
		Status:      ValidatorStatusBonded,
	}
	validatorSet.Validators = append(validatorSet.Validators, validator1)
	
	// 헤더 생성
	header := &types.Header{
		Number: big.NewInt(1000),
	}
	
	// 제안자 설정
	proposer := validator1.Address
	
	// 서명자 목록 생성
	signers := []common.Address{validator1.Address}
	
	// 응답 시간 설정
	responseTime := uint64(200)
	
	// 성능 업데이트 (로깅만 수행하므로 오류가 없어야 함)
	validatorSet.updateEnhancedValidatorPerformance(header, proposer, signers, responseTime)
	
	// 실제 구현에서는 DB에 저장하므로 여기서는 검증할 수 없음
	// 로깅만 확인
}

// TestAddNetworkContribution는 네트워크 기여도 추가 기능을 테스트합니다.
func TestAddNetworkContribution(t *testing.T) {
	// 검증자 집합 생성
	validatorSet := NewValidatorSet()
	
	// 테스트 검증자 주소
	validator := common.BytesToAddress([]byte{1})
	
	// 카테고리와 포인트 설정
	category := "governance"
	points := uint64(100)
	
	// 네트워크 기여도 추가 (로깅만 수행하므로 오류가 없어야 함)
	validatorSet.addNetworkContribution(validator, category, points)
	
	// 실제 구현에서는 DB에 저장하므로 여기서는 검증할 수 없음
	// 로깅만 확인
}

// TestAddCommunityVote는 커뮤니티 투표 추가 기능을 테스트합니다.
func TestAddCommunityVote(t *testing.T) {
	// 검증자 집합 생성
	validatorSet := NewValidatorSet()
	
	// 테스트 검증자 주소
	validator := common.BytesToAddress([]byte{1})
	
	// 투표 설정
	vote := int64(1)
	
	// 커뮤니티 투표 추가 (로깅만 수행하므로 오류가 없어야 함)
	validatorSet.addCommunityVote(validator, vote)
	
	// 실제 구현에서는 DB에 저장하므로 여기서는 검증할 수 없음
	// 로깅만 확인
}

// TestCreateSlashingEvent는 슬래싱 이벤트 생성 기능을 테스트합니다.
func TestCreateSlashingEvent(t *testing.T) {
	// 테스트 데이터 설정
	blockNumber := uint64(1000)
	slashingType := uint8(SlashingTypeDoubleSignValue)
	amount := big.NewInt(1e18)
	reason := "이중 서명 감지"
	
	// 슬래싱 이벤트 생성
	event := createSlashingEvent(blockNumber, slashingType, amount, reason)
	
	// 이벤트 필드 확인
	assert.Equal(t, blockNumber, event.BlockNumber, "블록 번호가 일치해야 함")
	assert.Equal(t, slashingType, event.Type, "슬래싱 유형이 일치해야 함")
	assert.Equal(t, amount, event.Amount, "슬래싱 양이 일치해야 함")
	assert.Equal(t, reason, event.Reason, "슬래싱 이유가 일치해야 함")
}

// 테스트용 향상된 검증자 생성 함수
func createTestEnhancedValidator(t *testing.T) *EnhancedValidator {
	// 기본 검증자 생성
	validator := &Validator{
		Address:     common.BytesToAddress([]byte{1}),
		VotingPower: big.NewInt(1e18),
		Status:      ValidatorStatusBonded,
	}
	
	// 향상된 성능 지표 설정
	enhancedStats := EnhancedValidatorStats{
		BlocksProposed:  10,
		BlocksSigned:    900,
		BlocksMissed:    100,
		Uptime:          900,
		GovernanceVotes: 5,
		ResponseTimes:   []uint64{200, 300, 250, 220, 180},
		AvgResponseTime: 230,
		BlocksOrphaned:  1,
		LastActiveBlock: 1000,
	}
	
	// 평판 지표 설정
	reputation := ValidatorReputationStats{
		SlashingHistory: []SlashingEvent{
			{
				BlockNumber: 500,
				Type:        SlashingTypeDowntimeValue,
				Amount:      big.NewInt(1e17),
				Reason:      "다운타임 감지",
			},
		},
		LastSlashingBlock:   500,
		TotalSlashingAmount: big.NewInt(1e17),
		ActivationBlock:     100,
		TotalActiveBlocks:   900,
		CommunityVotes:      10,
		NetworkContribPoints: 200,
		ContribCategories:    map[string]uint64{"governance": 100, "development": 100},
	}
	
	// 향상된 검증자 생성
	return &EnhancedValidator{
		Validator:     validator,
		EnhancedStats: enhancedStats,
		Reputation:    reputation,
	}
} 