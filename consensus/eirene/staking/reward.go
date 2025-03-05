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
	"math/big"
	"sync"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/core"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/rlp"
)

// 보상 관련 상수
const (
	// 블록 보상 (wei)
	baseBlockReward = 2e18 // 기본 블록 보상 (2 ETH)

	// 보상 분배 비율 (1000 단위)
	validatorRewardShare = 700 // 검증자 보상 비율 (70%)
	delegatorRewardShare = 200 // 위임자 보상 비율 (20%)
	communityRewardShare = 100 // 커뮤니티 기금 보상 비율 (10%)

	// 보상 감소 주기 (블록 수)
	rewardReductionPeriod = 4000000 // 약 2년 (15초 블록 기준)

	// 보상 감소 비율 (%)
	rewardReductionRatio = 20 // 20% 감소
)

// RewardAdapter는 보상 관리를 위한 어댑터입니다.
type RewardAdapter struct {
	eirene      *core.Eirene // Eirene 합의 엔진 인스턴스
	logger      log.Logger
	rewardState *RewardState
	lock        sync.RWMutex
}

// NewRewardAdapter는 새로운 RewardAdapter 인스턴스를 생성합니다.
func NewRewardAdapter(eirene *core.Eirene) *RewardAdapter {
	return &RewardAdapter{
		eirene:      eirene,
		logger:      log.New("module", "reward"),
		rewardState: newRewardState(),
	}
}

// RewardState는 보상 상태를 나타냅니다.
type RewardState struct {
	// 누적 보상
	AccumulatedRewards map[common.Address]*big.Int `json:"accumulatedRewards"` // 주소별 누적 보상

	// 커뮤니티 기금
	CommunityFund *big.Int `json:"communityFund"` // 커뮤니티 기금 잔액

	// 보상 매개변수
	CurrentBlockReward *big.Int `json:"currentBlockReward"` // 현재 블록 보상
	LastReductionBlock uint64   `json:"lastReductionBlock"` // 마지막 보상 감소 블록

	// 통계
	TotalDistributed *big.Int `json:"totalDistributed"` // 총 분배된 보상
}

// newRewardState는 새로운 RewardState 인스턴스를 생성합니다.
func newRewardState() *RewardState {
	return &RewardState{
		AccumulatedRewards: make(map[common.Address]*big.Int),
		CommunityFund:      new(big.Int),
		CurrentBlockReward: new(big.Int).SetUint64(baseBlockReward),
		LastReductionBlock: 0,
		TotalDistributed:   new(big.Int),
	}
}

// loadRewardState는 데이터베이스에서 RewardState를 로드합니다.
func loadRewardState(db ethdb.Database) (*RewardState, error) {
	data, err := db.Get([]byte("eirene-rewards"))
	if err != nil {
		return newRewardState(), nil
	}

	var rewardState RewardState
	if err := rlp.DecodeBytes(data, &rewardState); err != nil {
		return nil, err
	}

	return &rewardState, nil
}

// store는 RewardState를 데이터베이스에 저장합니다.
func (rs *RewardState) store(db ethdb.Database) error {
	data, err := rlp.EncodeToBytes(rs)
	if err != nil {
		return err
	}

	return db.Put([]byte("eirene-rewards"), data)
}

// calculateBlockReward는 블록 보상을 계산합니다.
func (rs *RewardState) calculateBlockReward(blockNumber uint64) *big.Int {
	// 보상 감소 확인
	if blockNumber > rs.LastReductionBlock+rewardReductionPeriod {
		// 감소 횟수 계산
		reductions := (blockNumber - rs.LastReductionBlock) / rewardReductionPeriod
		rs.LastReductionBlock += reductions * rewardReductionPeriod

		// 보상 감소 적용
		for i := uint64(0); i < reductions; i++ {
			reduction := new(big.Int).Mul(rs.CurrentBlockReward, big.NewInt(rewardReductionRatio))
			reduction = new(big.Int).Div(reduction, big.NewInt(100))
			rs.CurrentBlockReward = new(big.Int).Sub(rs.CurrentBlockReward, reduction)
		}
	}

	return new(big.Int).Set(rs.CurrentBlockReward)
}

// DistributeBlockReward는 블록 보상을 분배합니다.
func (a *RewardAdapter) DistributeBlockReward(header *types.Header) {
	blockNumber := header.Number.Uint64()

	// 블록 보상 계산
	blockReward := a.rewardState.calculateBlockReward(blockNumber)

	// 블록 제안자 가져오기
	proposer, err := a.eirene.Author(header)
	if err != nil {
		a.logger.Error("Failed to get block proposer", "err", err)
		return
	}

	// 검증자 가져오기
	// 실제 구현에서는 a.eirene.GetValidatorSet().GetValidator(proposer) 형태로 호출
	// 여기서는 간단히 구현
	validator := &Validator{
		Address:     proposer,
		VotingPower: big.NewInt(0),
		Delegations: make(map[common.Address]*ValidatorDelegation),
	}

	// 보상 분배
	// 1. 검증자 보상 (70%)
	validatorReward := new(big.Int).Mul(blockReward, big.NewInt(validatorRewardShare))
	validatorReward = new(big.Int).Div(validatorReward, big.NewInt(1000))

	// 2. 위임자 보상 (20%)
	delegatorReward := new(big.Int).Mul(blockReward, big.NewInt(delegatorRewardShare))
	delegatorReward = new(big.Int).Div(delegatorReward, big.NewInt(1000))

	// 3. 커뮤니티 기금 (10%)
	communityReward := new(big.Int).Mul(blockReward, big.NewInt(communityRewardShare))
	communityReward = new(big.Int).Div(communityReward, big.NewInt(1000))

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
		for delegator, delegation := range validator.Delegations {
			if totalShares.Sign() > 0 {
				// 위임자 지분에 비례하여 보상 계산
				delegatorShare := new(big.Int).Mul(delegatorReward, delegation.Amount)
				delegatorShare = new(big.Int).Div(delegatorShare, totalShares)

				// 위임자 보상 누적
				a.addReward(delegator, delegatorShare)
			}
		}
	}

	// 커뮤니티 기금에 보상 추가
	a.rewardState.CommunityFund = new(big.Int).Add(a.rewardState.CommunityFund, communityReward)

	// 총 분배된 보상 업데이트
	a.rewardState.TotalDistributed = new(big.Int).Add(a.rewardState.TotalDistributed, blockReward)
}

// addReward는 주소에 보상을 추가합니다.
func (a *RewardAdapter) addReward(addr common.Address, amount *big.Int) {
	if amount.Sign() <= 0 {
		return
	}

	if _, exists := a.rewardState.AccumulatedRewards[addr]; !exists {
		a.rewardState.AccumulatedRewards[addr] = new(big.Int)
	}

	a.rewardState.AccumulatedRewards[addr] = new(big.Int).Add(a.rewardState.AccumulatedRewards[addr], amount)
}

// ClaimRewards는 누적된 보상을 청구합니다.
func (a *RewardAdapter) ClaimRewards(claimer common.Address) (*big.Int, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	// 누적 보상 확인
	reward, exists := a.rewardState.AccumulatedRewards[claimer]
	if !exists || reward.Sign() <= 0 {
		return big.NewInt(0), errors.New("no rewards to claim")
	}

	// 보상 금액 복사
	amount := new(big.Int).Set(reward)

	// 보상 초기화
	a.rewardState.AccumulatedRewards[claimer] = big.NewInt(0)

	// 데이터베이스에 저장
	if err := a.rewardState.store(a.eirene.GetDB()); err != nil {
		return nil, err
	}

	return amount, nil
}

// GetAccumulatedRewards는 주소의 누적 보상을 반환합니다.
func (a *RewardAdapter) GetAccumulatedRewards(addr common.Address) *big.Int {
	a.lock.RLock()
	defer a.lock.RUnlock()

	reward, exists := a.rewardState.AccumulatedRewards[addr]
	if !exists {
		return big.NewInt(0)
	}

	return new(big.Int).Set(reward)
}

// GetCommunityFund는 커뮤니티 기금 잔액을 반환합니다.
func (a *RewardAdapter) GetCommunityFund() *big.Int {
	a.lock.RLock()
	defer a.lock.RUnlock()

	return new(big.Int).Set(a.rewardState.CommunityFund)
}

// WithdrawFromCommunityFund는 커뮤니티 기금에서 자금을 인출합니다.
func (a *RewardAdapter) WithdrawFromCommunityFund(recipient common.Address, amount *big.Int) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	// 잔액 확인
	if a.rewardState.CommunityFund.Cmp(amount) < 0 {
		return errors.New("insufficient community fund balance")
	}

	// 자금 인출
	a.rewardState.CommunityFund = new(big.Int).Sub(a.rewardState.CommunityFund, amount)

	// 수령인에게 보상 추가
	a.addReward(recipient, amount)

	// 데이터베이스에 저장
	if err := a.rewardState.store(a.eirene.GetDB()); err != nil {
		return err
	}

	return nil
}
