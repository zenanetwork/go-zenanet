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
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/core"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/cosmos"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/log"
)

// DistributionParams는 보상 분배 관련 매개변수를 정의합니다.
type DistributionParams struct {
	ValidatorRewardRatio  float64 // 검증자 보상 비율 (0-1)
	DelegatorRewardRatio  float64 // 위임자 보상 비율 (0-1)
	CommunityRewardRatio  float64 // 커뮤니티 기금 보상 비율 (0-1)
	BaseReward            *big.Int // 기본 블록 보상
	RewardReductionPeriod uint64   // 보상 감소 주기 (블록 수)
	RewardReductionRatio  float64  // 보상 감소 비율 (0-1)
}

// DefaultDistributionParams는 기본 보상 분배 매개변수를 반환합니다.
func DefaultDistributionParams() DistributionParams {
	return DistributionParams{
		ValidatorRewardRatio:  0.7,                  // 70%
		DelegatorRewardRatio:  0.2,                  // 20%
		CommunityRewardRatio:  0.1,                  // 10%
		BaseReward:            big.NewInt(2e18),     // 2 ETH
		RewardReductionPeriod: 4000000,              // 약 2년 (15초 블록 기준)
		RewardReductionRatio:  0.2,                  // 20%
	}
}

// DistributionRecord는 보상 분배 정보를 나타냅니다.
type DistributionRecord struct {
	BlockNumber     uint64         // 블록 번호
	Timestamp       time.Time      // 타임스탬프
	TotalReward     *big.Int       // 총 보상
	ValidatorReward *big.Int       // 검증자 보상
	DelegatorReward *big.Int       // 위임자 보상
	CommunityReward *big.Int       // 커뮤니티 기금 보상
	Validator       common.Address // 검증자 주소
}

// CosmosDistributionAdapter는 Cosmos SDK의 distribution 모듈과 연동하는 어댑터입니다.
type CosmosDistributionAdapter struct {
	eirene           *core.Eirene
	logger           log.Logger
	storeAdapter     *cosmos.StateDBAdapter
	validatorSet     *ValidatorSet
	params           DistributionParams
	communityFund    *big.Int
	accumulatedRewards map[common.Address]*big.Int
	distributionHistory []DistributionRecord
	currentBlockReward *big.Int
	lastReductionBlock uint64
}

// NewCosmosDistributionAdapter는 새로운 CosmosDistributionAdapter 인스턴스를 생성합니다.
func NewCosmosDistributionAdapter(eirene *core.Eirene, storeAdapter *cosmos.StateDBAdapter, validatorSet *ValidatorSet) *CosmosDistributionAdapter {
	return &CosmosDistributionAdapter{
		eirene:             eirene,
		logger:             log.New("module", "cosmos_distribution"),
		storeAdapter:       storeAdapter,
		validatorSet:       validatorSet,
		params:             DefaultDistributionParams(),
		communityFund:      big.NewInt(0),
		accumulatedRewards: make(map[common.Address]*big.Int),
		distributionHistory: []DistributionRecord{},
		currentBlockReward: big.NewInt(0).Set(DefaultDistributionParams().BaseReward),
		lastReductionBlock: 0,
	}
}

// DistributeBlockReward는 블록 보상을 분배합니다.
func (a *CosmosDistributionAdapter) DistributeBlockReward(header *types.Header) error {
	blockNumber := header.Number.Uint64()
	a.logger.Debug("Distributing block reward", "height", blockNumber)

	// 블록 제안자 가져오기
	proposer, err := a.eirene.Author(header)
	if err != nil {
		return fmt.Errorf("failed to get block proposer: %v", err)
	}

	// 검증자 확인
	validator := a.validatorSet.GetValidator(proposer)
	if validator == nil {
		return fmt.Errorf("validator not found: %s", proposer.Hex())
	}

	// 블록 보상 계산
	blockReward := a.calculateBlockReward(blockNumber)

	// 보상 분배
	// 1. 검증자 보상 (70%)
	validatorReward := new(big.Int).Mul(blockReward, big.NewInt(int64(a.params.ValidatorRewardRatio * 100)))
	validatorReward = new(big.Int).Div(validatorReward, big.NewInt(100))

	// 2. 위임자 보상 (20%)
	delegatorReward := new(big.Int).Mul(blockReward, big.NewInt(int64(a.params.DelegatorRewardRatio * 100)))
	delegatorReward = new(big.Int).Div(delegatorReward, big.NewInt(100))

	// 3. 커뮤니티 기금 (10%)
	communityReward := new(big.Int).Mul(blockReward, big.NewInt(int64(a.params.CommunityRewardRatio * 100)))
	communityReward = new(big.Int).Div(communityReward, big.NewInt(100))

	// 검증자 보상 누적
	a.addReward(proposer, validatorReward)

	// 위임자 보상 분배
	if len(validator.Delegations) > 0 {
		// 위임자별 지분 계산
		totalShares := new(big.Int)
		for _, delegation := range validator.Delegations {
			totalShares = new(big.Int).Add(totalShares, delegation.Amount)
		}

		// 위임자별 보상 분배
		for _, delegation := range validator.Delegations {
			if totalShares.Sign() > 0 {
				// 위임자 지분에 비례하여 보상 계산
				delegatorShare := new(big.Int).Mul(delegatorReward, delegation.Amount)
				delegatorShare = new(big.Int).Div(delegatorShare, totalShares)

				// 위임자 보상 누적
				a.addReward(delegation.Delegator, delegatorShare)
			}
		}
	}

	// 커뮤니티 기금에 보상 추가
	a.communityFund = new(big.Int).Add(a.communityFund, communityReward)

	// 보상 분배 이력 추가
	distribution := DistributionRecord{
		BlockNumber:     blockNumber,
		Timestamp:       time.Now(),
		TotalReward:     new(big.Int).Set(blockReward),
		ValidatorReward: new(big.Int).Set(validatorReward),
		DelegatorReward: new(big.Int).Set(delegatorReward),
		CommunityReward: new(big.Int).Set(communityReward),
		Validator:       proposer,
	}
	a.distributionHistory = append(a.distributionHistory, distribution)

	a.logger.Info("Block reward distributed",
		"height", blockNumber,
		"proposer", proposer.Hex(),
		"totalReward", blockReward.String(),
		"validatorReward", validatorReward.String(),
		"delegatorReward", delegatorReward.String(),
		"communityReward", communityReward.String())

	return nil
}

// calculateBlockReward는 블록 보상을 계산합니다.
func (a *CosmosDistributionAdapter) calculateBlockReward(blockNumber uint64) *big.Int {
	// 보상 감소 확인
	if blockNumber > a.lastReductionBlock+a.params.RewardReductionPeriod {
		// 감소 횟수 계산
		reductions := (blockNumber - a.lastReductionBlock) / a.params.RewardReductionPeriod
		a.lastReductionBlock += reductions * a.params.RewardReductionPeriod

		// 보상 감소 적용
		for i := uint64(0); i < reductions; i++ {
			reduction := new(big.Int).Mul(a.currentBlockReward, big.NewInt(int64(a.params.RewardReductionRatio * 100)))
			reduction = new(big.Int).Div(reduction, big.NewInt(100))
			a.currentBlockReward = new(big.Int).Sub(a.currentBlockReward, reduction)
		}

		a.logger.Info("Block reward reduced",
			"newReward", a.currentBlockReward.String(),
			"reductions", reductions,
			"lastReductionBlock", a.lastReductionBlock)
	}

	return new(big.Int).Set(a.currentBlockReward)
}

// addReward는 주소에 보상을 추가합니다.
func (a *CosmosDistributionAdapter) addReward(addr common.Address, amount *big.Int) {
	if amount.Sign() <= 0 {
		return
	}

	if _, exists := a.accumulatedRewards[addr]; !exists {
		a.accumulatedRewards[addr] = new(big.Int)
	}

	a.accumulatedRewards[addr] = new(big.Int).Add(a.accumulatedRewards[addr], amount)
}

// ClaimRewards는 누적된 보상을 청구합니다.
func (a *CosmosDistributionAdapter) ClaimRewards(claimer common.Address) (*big.Int, error) {
	// 누적 보상 확인
	reward, exists := a.accumulatedRewards[claimer]
	if !exists || reward.Sign() <= 0 {
		return big.NewInt(0), errors.New("no rewards to claim")
	}

	// 보상 금액 복사
	amount := new(big.Int).Set(reward)

	// 보상 초기화
	a.accumulatedRewards[claimer] = big.NewInt(0)

	a.logger.Info("Rewards claimed", "claimer", claimer.Hex(), "amount", amount.String())

	return amount, nil
}

// GetAccumulatedRewards는 주소의 누적 보상을 반환합니다.
func (a *CosmosDistributionAdapter) GetAccumulatedRewards(addr common.Address) *big.Int {
	if reward, exists := a.accumulatedRewards[addr]; exists {
		return new(big.Int).Set(reward)
	}
	return big.NewInt(0)
}

// GetCommunityFund는 커뮤니티 기금의 잔액을 반환합니다.
func (a *CosmosDistributionAdapter) GetCommunityFund() *big.Int {
	return new(big.Int).Set(a.communityFund)
}

// WithdrawFromCommunityFund는 커뮤니티 기금에서 자금을 인출합니다.
func (a *CosmosDistributionAdapter) WithdrawFromCommunityFund(recipient common.Address, amount *big.Int) error {
	// 잔액 확인
	if a.communityFund.Cmp(amount) < 0 {
		return fmt.Errorf("insufficient community fund: %s < %s", a.communityFund.String(), amount.String())
	}

	// 자금 인출
	a.communityFund = new(big.Int).Sub(a.communityFund, amount)

	// 수령인에게 자금 추가
	a.addReward(recipient, amount)

	a.logger.Info("Withdrawn from community fund",
		"recipient", recipient.Hex(),
		"amount", amount.String(),
		"remainingFund", a.communityFund.String())

	return nil
}

// GetRewardDistributionHistory는 보상 분배 이력을 반환합니다.
func (a *CosmosDistributionAdapter) GetRewardDistributionHistory(count int) []DistributionRecord {
	historyLen := len(a.distributionHistory)
	if count <= 0 || count > historyLen {
		count = historyLen
	}

	result := make([]DistributionRecord, count)
	for i := 0; i < count; i++ {
		result[i] = a.distributionHistory[historyLen-count+i]
	}
	return result
}

// SaveToState는 보상 상태를 상태 DB에 저장합니다.
func (a *CosmosDistributionAdapter) SaveToState(state *state.StateDB) error {
	// 상태 저장 로직 구현
	// 실제 구현에서는 상태 DB에 보상 상태를 저장해야 함
	// 여기서는 간단히 구현
	return nil
}

// LoadFromState는 상태 DB에서 보상 상태를 로드합니다.
func (a *CosmosDistributionAdapter) LoadFromState(state *state.StateDB) error {
	// 상태 로드 로직 구현
	// 실제 구현에서는 상태 DB에서 보상 상태를 로드해야 함
	// 여기서는 간단히 구현
	return nil
} 