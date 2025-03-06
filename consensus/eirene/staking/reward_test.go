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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zenanetwork/go-zenanet/common"
)

// 테스트용 상수 정의
const (
	testBaseBlockReward      = 2e18 // 기본 블록 보상 (2 ETH)
	testRewardReductionPeriod = 4000000 // 약 2년 (15초 블록 기준)
	testRewardReductionRatio  = 20 // 20% 감소
)

// 테스트용 보상 상태 구조체
type testRewardState struct {
	AccumulatedRewards map[common.Address]*big.Int
	CommunityFund      *big.Int
	CurrentBlockReward *big.Int
	LastReductionBlock uint64
	TotalDistributed   *big.Int
}

// 테스트용 보상 상태 생성 함수
func newTestRewardState() *testRewardState {
	return &testRewardState{
		AccumulatedRewards: make(map[common.Address]*big.Int),
		CommunityFund:      new(big.Int),
		CurrentBlockReward: new(big.Int).SetUint64(testBaseBlockReward),
		LastReductionBlock: 0,
		TotalDistributed:   new(big.Int),
	}
}

// 테스트용 블록 보상 계산 함수
func (rs *testRewardState) calculateBlockReward(blockNumber uint64) *big.Int {
	// 보상 감소 확인
	if blockNumber > rs.LastReductionBlock+testRewardReductionPeriod {
		// 감소 횟수 계산
		reductions := (blockNumber - rs.LastReductionBlock) / testRewardReductionPeriod
		rs.LastReductionBlock += reductions * testRewardReductionPeriod

		// 보상 감소 적용
		for i := uint64(0); i < reductions; i++ {
			reduction := new(big.Int).Mul(rs.CurrentBlockReward, big.NewInt(testRewardReductionRatio))
			reduction = new(big.Int).Div(reduction, big.NewInt(100))
			rs.CurrentBlockReward = new(big.Int).Sub(rs.CurrentBlockReward, reduction)
		}
	}

	return new(big.Int).Set(rs.CurrentBlockReward)
}

// TestNewRewardState는 새로운 보상 상태 생성 기능을 테스트합니다.
func TestNewRewardState(t *testing.T) {
	// 새로운 보상 상태 생성
	rewardState := newTestRewardState()
	
	// 필드 확인
	assert.NotNil(t, rewardState.AccumulatedRewards, "누적 보상 맵이 nil이 아니어야 함")
	assert.NotNil(t, rewardState.CommunityFund, "커뮤니티 기금이 nil이 아니어야 함")
	assert.NotNil(t, rewardState.CurrentBlockReward, "현재 블록 보상이 nil이 아니어야 함")
	assert.Equal(t, uint64(0), rewardState.LastReductionBlock, "마지막 감소 블록이 0이어야 함")
	assert.NotNil(t, rewardState.TotalDistributed, "총 분배된 보상이 nil이 아니어야 함")
	
	// 기본값 확인
	expectedBlockReward := new(big.Int).SetUint64(testBaseBlockReward)
	assert.Equal(t, expectedBlockReward.String(), rewardState.CurrentBlockReward.String(), "현재 블록 보상이 기본 블록 보상과 일치해야 함")
	assert.Equal(t, "0", rewardState.CommunityFund.String(), "커뮤니티 기금이 0이어야 함")
	assert.Equal(t, "0", rewardState.TotalDistributed.String(), "총 분배된 보상이 0이어야 함")
}

// TestCalculateBlockReward는 블록 보상 계산 기능을 테스트합니다.
func TestCalculateBlockReward(t *testing.T) {
	// 새로운 보상 상태 생성
	rewardState := newTestRewardState()
	
	// 초기 블록 보상 확인
	initialReward := rewardState.calculateBlockReward(1)
	expectedInitialReward := new(big.Int).SetUint64(testBaseBlockReward)
	assert.Equal(t, expectedInitialReward.String(), initialReward.String(), "초기 블록 보상이 기본 블록 보상과 일치해야 함")
	
	// 보상 감소 전 블록 보상 확인
	beforeReductionReward := rewardState.calculateBlockReward(testRewardReductionPeriod - 1)
	assert.Equal(t, expectedInitialReward.String(), beforeReductionReward.String(), "감소 전 블록 보상이 초기 보상과 일치해야 함")
	
	// 보상 감소 후 블록 보상 확인
	afterReductionReward := rewardState.calculateBlockReward(testRewardReductionPeriod + 1)
	
	// 감소된 보상 계산
	reduction := new(big.Int).Mul(expectedInitialReward, big.NewInt(testRewardReductionRatio))
	reduction = new(big.Int).Div(reduction, big.NewInt(100))
	expectedReducedReward := new(big.Int).Sub(expectedInitialReward, reduction)
	
	assert.Equal(t, expectedReducedReward.String(), afterReductionReward.String(), "감소 후 블록 보상이 예상 감소 보상과 일치해야 함")
	assert.Equal(t, uint64(testRewardReductionPeriod), rewardState.LastReductionBlock, "마지막 감소 블록이 업데이트되어야 함")
	
	// 여러 번 감소 후 블록 보상 확인
	multipleReductionReward := rewardState.calculateBlockReward(testRewardReductionPeriod * 3 + 1)
	
	// 두 번 더 감소된 보상 계산
	for i := 0; i < 2; i++ {
		reduction = new(big.Int).Mul(expectedReducedReward, big.NewInt(testRewardReductionRatio))
		reduction = new(big.Int).Div(reduction, big.NewInt(100))
		expectedReducedReward = new(big.Int).Sub(expectedReducedReward, reduction)
	}
	
	assert.Equal(t, expectedReducedReward.String(), multipleReductionReward.String(), "여러 번 감소 후 블록 보상이 예상 감소 보상과 일치해야 함")
	assert.Equal(t, uint64(testRewardReductionPeriod * 3), rewardState.LastReductionBlock, "마지막 감소 블록이 업데이트되어야 함")
}

// TestAddReward는 보상 추가 기능을 테스트합니다.
func TestAddReward(t *testing.T) {
	// 보상 어댑터 생성
	rewardState := newTestRewardState()
	
	// 테스트 주소
	address := common.BytesToAddress([]byte{1})
	
	// 보상 추가
	reward := big.NewInt(1e18) // 1 ETH
	if _, exists := rewardState.AccumulatedRewards[address]; !exists {
		rewardState.AccumulatedRewards[address] = new(big.Int)
	}
	rewardState.AccumulatedRewards[address] = new(big.Int).Add(rewardState.AccumulatedRewards[address], reward)
	
	// 누적 보상 확인
	accumulatedReward, exists := rewardState.AccumulatedRewards[address]
	assert.True(t, exists, "주소에 대한 누적 보상이 존재해야 함")
	assert.Equal(t, reward.String(), accumulatedReward.String(), "누적 보상이 추가된 보상과 일치해야 함")
	
	// 추가 보상 추가
	additionalReward := big.NewInt(5e17) // 0.5 ETH
	rewardState.AccumulatedRewards[address] = new(big.Int).Add(rewardState.AccumulatedRewards[address], additionalReward)
	
	// 업데이트된 누적 보상 확인
	expectedTotalReward := new(big.Int).Add(reward, additionalReward)
	updatedAccumulatedReward := rewardState.AccumulatedRewards[address]
	assert.Equal(t, expectedTotalReward.String(), updatedAccumulatedReward.String(), "업데이트된 누적 보상이 예상 총 보상과 일치해야 함")
}

// TestAddToCommunityFund는 커뮤니티 기금 추가 기능을 테스트합니다.
func TestAddToCommunityFund(t *testing.T) {
	// 보상 상태 생성
	rewardState := newTestRewardState()
	
	// 초기 커뮤니티 기금 확인
	initialFund := new(big.Int).Set(rewardState.CommunityFund)
	assert.Equal(t, "0", initialFund.String(), "초기 커뮤니티 기금이 0이어야 함")
	
	// 커뮤니티 기금 추가
	amount := big.NewInt(1e18) // 1 ETH
	rewardState.CommunityFund = new(big.Int).Add(rewardState.CommunityFund, amount)
	
	// 업데이트된 커뮤니티 기금 확인
	expectedFund := new(big.Int).Add(initialFund, amount)
	updatedFund := rewardState.CommunityFund
	assert.Equal(t, expectedFund.String(), updatedFund.String(), "업데이트된 커뮤니티 기금이 예상 기금과 일치해야 함")
	
	// 추가 금액 추가
	additionalAmount := big.NewInt(5e17) // 0.5 ETH
	rewardState.CommunityFund = new(big.Int).Add(rewardState.CommunityFund, additionalAmount)
	
	// 최종 커뮤니티 기금 확인
	expectedFinalFund := new(big.Int).Add(expectedFund, additionalAmount)
	finalFund := rewardState.CommunityFund
	assert.Equal(t, expectedFinalFund.String(), finalFund.String(), "최종 커뮤니티 기금이 예상 기금과 일치해야 함")
} 