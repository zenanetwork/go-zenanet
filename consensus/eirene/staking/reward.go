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
	"time"

	"github.com/holiman/uint256"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/core"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/tracing"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/params"
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

	// 보상 비율
	ValidatorRewardRatio  = 0.70 // 검증자 보상 비율 (70%)
	DelegatorRewardRatio  = 0.20 // 위임자 보상 비율 (20%)
	CommunityRewardRatio  = 0.10 // 커뮤니티 기금 보상 비율 (10%)

	// 보상 감소 주기
	RewardHalvingBlocks = 1000000 // 100만 블록마다 보상 반감
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

// RewardManager는 보상 분배 시스템을 관리합니다
type RewardManager struct {
	config       *params.EireneConfig // 합의 엔진 구성
	validatorSet *ValidatorSet        // 검증자 집합

	// 보상 추적
	accumulatedRewards map[common.Address]*big.Int // 검증자별 누적 보상
	communityFund      *big.Int                    // 커뮤니티 기금

	// 보상 분배 이력
	distributionHistory []RewardDistribution // 보상 분배 이력

	lock sync.RWMutex // 동시성 제어를 위한 잠금
}

// RewardDistribution은 보상 분배 정보를 나타냅니다
type RewardDistribution struct {
	BlockNumber    uint64         // 블록 번호
	Timestamp      time.Time      // 타임스탬프
	TotalReward    *big.Int       // 총 보상
	ValidatorReward *big.Int      // 검증자 보상
	DelegatorReward *big.Int      // 위임자 보상
	CommunityReward *big.Int      // 커뮤니티 기금 보상
	Validator      common.Address // 검증자 주소
}

// NewRewardManager는 새로운 보상 관리자를 생성합니다
func NewRewardManager(config *params.EireneConfig, validatorSet *ValidatorSet) *RewardManager {
	return &RewardManager{
		config:             config,
		validatorSet:       validatorSet,
		accumulatedRewards: make(map[common.Address]*big.Int),
		communityFund:      big.NewInt(0),
		distributionHistory: make([]RewardDistribution, 0),
	}
}

// CalculateBlockReward는 블록 번호에 따라 블록 보상을 계산합니다
func (rm *RewardManager) CalculateBlockReward(blockNumber uint64) *big.Int {
	// 기본 보상
	baseReward := new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18)) // 1 토큰

	// 블록 보상 감소 로직 (매 100만 블록마다 반감)
	halvings := blockNumber / RewardHalvingBlocks
	
	if halvings > 0 {
		// 2^halvings로 나누기
		divisor := new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(halvings)), nil)
		baseReward = baseReward.Div(baseReward, divisor)
	}

	return baseReward
}

// DistributeRewards는 블록 생성 및 검증에 대한 보상을 분배합니다
func (rm *RewardManager) DistributeRewards(blockNumber uint64, validator common.Address, state *state.StateDB) error {
	rm.lock.Lock()
	defer rm.lock.Unlock()

	// 블록 보상 계산
	blockReward := rm.CalculateBlockReward(blockNumber)

	// 검증자 보상 (70%)
	validatorReward := new(big.Int).Mul(blockReward, big.NewInt(int64(ValidatorRewardRatio*100)))
	validatorReward = validatorReward.Div(validatorReward, big.NewInt(100))

	// 위임자 보상 (20%)
	delegatorReward := new(big.Int).Mul(blockReward, big.NewInt(int64(DelegatorRewardRatio*100)))
	delegatorReward = delegatorReward.Div(delegatorReward, big.NewInt(100))

	// 커뮤니티 기금 보상 (10%)
	communityReward := new(big.Int).Mul(blockReward, big.NewInt(int64(CommunityRewardRatio*100)))
	communityReward = communityReward.Div(communityReward, big.NewInt(100))

	// 검증자 보상 지급
	v := rm.validatorSet.GetValidatorByAddress(validator)
	if v == nil {
		return ErrValidatorNotFound
	}

	// 검증자 수수료 계산
	validator_obj, ok := v.(*Validator)
	if !ok {
		return errors.New("invalid validator type")
	}
	commissionRate := float64(validator_obj.Commission) / 10000

	// 검증자에게 보상 지급 (검증자 보상 + 수수료)
	totalValidatorReward := new(big.Int).Add(validatorReward, new(big.Int).Mul(delegatorReward, big.NewInt(int64(commissionRate*10000))))
	if _, ok := rm.accumulatedRewards[validator]; !ok {
		rm.accumulatedRewards[validator] = big.NewInt(0)
	}
	rm.accumulatedRewards[validator] = new(big.Int).Add(rm.accumulatedRewards[validator], totalValidatorReward)
	
	// big.Int를 uint256.Int로 변환
	uint256TotalValidatorReward := new(uint256.Int).SetBytes(totalValidatorReward.Bytes())
	state.AddBalance(validator, uint256TotalValidatorReward, tracing.BalanceChangeUnspecified)

	// 위임자 보상 분배
	rm.distributeDelegatorRewards(validator, delegatorReward, state)

	// 커뮤니티 기금에 보상 추가
	rm.communityFund = new(big.Int).Add(rm.communityFund, communityReward)
	communityFundAddress := common.HexToAddress("0x0000000000000000000000000000000000000100") // 예시 주소
	
	// big.Int를 uint256.Int로 변환
	uint256CommunityReward := new(uint256.Int).SetBytes(communityReward.Bytes())
	state.AddBalance(communityFundAddress, uint256CommunityReward, tracing.BalanceChangeUnspecified)

	// 보상 분배 이력 추가
	rm.distributionHistory = append(rm.distributionHistory, RewardDistribution{
		BlockNumber:     blockNumber,
		Timestamp:       time.Now(),
		TotalReward:     blockReward,
		ValidatorReward: totalValidatorReward,
		DelegatorReward: delegatorReward,
		CommunityReward: communityReward,
		Validator:       validator,
	})

	log.Debug("Rewards distributed", 
		"block", blockNumber, 
		"validator", validator, 
		"total", blockReward, 
		"validatorReward", totalValidatorReward, 
		"delegatorReward", delegatorReward, 
		"communityReward", communityReward)

	return nil
}

// distributeDelegatorRewards는 위임자들에게 보상을 분배합니다
func (rm *RewardManager) distributeDelegatorRewards(validator common.Address, totalReward *big.Int, state *state.StateDB) {
	v := rm.validatorSet.GetValidatorByAddress(validator)
	if v == nil {
		return
	}
	
	validator_obj, ok := v.(*Validator)
	if !ok {
		log.Error("Invalid validator type", "validator", validator)
		return
	}

	// 총 위임 금액 계산
	totalDelegation := big.NewInt(0)
	for _, delegation := range validator_obj.Delegations {
		totalDelegation = new(big.Int).Add(totalDelegation, delegation.Amount)
	}
	
	// 위임이 없으면 검증자에게 모든 보상 지급
	if totalDelegation.Cmp(big.NewInt(0)) == 0 {
		if _, ok := rm.accumulatedRewards[validator]; !ok {
			rm.accumulatedRewards[validator] = big.NewInt(0)
		}
		rm.accumulatedRewards[validator] = new(big.Int).Add(rm.accumulatedRewards[validator], totalReward)
		
		// big.Int를 uint256.Int로 변환
		uint256TotalReward := new(uint256.Int).SetBytes(totalReward.Bytes())
		state.AddBalance(validator, uint256TotalReward, tracing.BalanceChangeUnspecified)
		return
	}

	// 위임자 보상 분배
	for _, delegation := range validator_obj.Delegations {
		// 위임 비율 계산
		ratio := new(big.Float).Quo(
			new(big.Float).SetInt(delegation.Amount),
			new(big.Float).SetInt(totalDelegation),
		)

		// 위임자 보상 계산
		rewardFloat := new(big.Float).Mul(ratio, new(big.Float).SetInt(totalReward))
		delegatorReward, _ := rewardFloat.Int(nil)

		// 위임자 보상 누적
		if delegation.AccumulatedRewards == nil {
			delegation.AccumulatedRewards = big.NewInt(0)
		}
		delegation.AccumulatedRewards = new(big.Int).Add(delegation.AccumulatedRewards, delegatorReward)

		log.Debug("Delegator reward distributed", "delegator", delegation.Delegator, "validator", validator, "amount", delegatorReward)
	}
}

// ClaimRewards는 검증자의 누적 보상을 청구합니다.
func (rm *RewardManager) ClaimRewards(validator common.Address, state *state.StateDB) (*big.Int, error) {
	rm.lock.Lock()
	defer rm.lock.Unlock()

	// 누적 보상 확인
	rewards, ok := rm.accumulatedRewards[validator]
	if !ok || rewards.Cmp(big.NewInt(0)) <= 0 {
		return big.NewInt(0), nil
	}

	// 보상 지급
	claimedRewards := new(big.Int).Set(rewards)
	rm.accumulatedRewards[validator] = big.NewInt(0)
	
	// big.Int를 uint256.Int로 변환
	uint256ClaimedRewards := new(uint256.Int).SetBytes(claimedRewards.Bytes())
	state.AddBalance(validator, uint256ClaimedRewards, tracing.BalanceChangeUnspecified)

	log.Debug("Validator rewards claimed", "validator", validator, "amount", claimedRewards)
	return claimedRewards, nil
}

// ClaimDelegatorRewards는 위임자의 누적 보상을 청구합니다.
func (rm *RewardManager) ClaimDelegatorRewards(delegator common.Address, validator common.Address, state *state.StateDB) (*big.Int, error) {
	rm.lock.Lock()
	defer rm.lock.Unlock()

	// 검증자 확인
	v := rm.validatorSet.GetValidatorByAddress(validator)
	if v == nil {
		return nil, ErrValidatorNotFound
	}
	
	validator_obj, ok := v.(*Validator)
	if !ok {
		return nil, errors.New("invalid validator type")
	}

	// 위임 정보 확인
	delegation, exists := validator_obj.Delegations[delegator]
	if !exists {
		return nil, errors.New("delegation not found")
	}

	// 보상 청구
	claimedRewards := new(big.Int).Set(delegation.AccumulatedRewards)
	delegation.AccumulatedRewards = big.NewInt(0)
	
	// big.Int를 uint256.Int로 변환
	uint256ClaimedRewards := new(uint256.Int).SetBytes(claimedRewards.Bytes())
	state.AddBalance(delegator, uint256ClaimedRewards, tracing.BalanceChangeUnspecified)

	log.Debug("Delegator rewards claimed", "delegator", delegator, "validator", validator, "amount", claimedRewards)
	return claimedRewards, nil
}

// GetAccumulatedRewards는 검증자의 누적 보상을 반환합니다
func (rm *RewardManager) GetAccumulatedRewards(validator common.Address) *big.Int {
	rm.lock.RLock()
	defer rm.lock.RUnlock()

	rewards, ok := rm.accumulatedRewards[validator]
	if !ok {
		return big.NewInt(0)
	}
	return new(big.Int).Set(rewards)
}

// GetDelegatorRewards는 위임자의 누적 보상을 반환합니다.
func (rm *RewardManager) GetDelegatorRewards(delegator common.Address, validator common.Address) (*big.Int, error) {
	rm.lock.RLock()
	defer rm.lock.RUnlock()

	// 검증자 확인
	v := rm.validatorSet.GetValidatorByAddress(validator)
	if v == nil {
		return nil, ErrValidatorNotFound
	}
	
	validator_obj, ok := v.(*Validator)
	if !ok {
		return nil, errors.New("invalid validator type")
	}

	// 위임 정보 확인
	delegation, exists := validator_obj.Delegations[delegator]
	if !exists {
		return nil, errors.New("delegation not found")
	}

	return new(big.Int).Set(delegation.AccumulatedRewards), nil
}

// GetCommunityFund는 커뮤니티 기금의 잔액을 반환합니다
func (rm *RewardManager) GetCommunityFund() *big.Int {
	rm.lock.RLock()
	defer rm.lock.RUnlock()

	return new(big.Int).Set(rm.communityFund)
}

// GetRewardDistributionHistory는 보상 분배 이력을 반환합니다
func (rm *RewardManager) GetRewardDistributionHistory(count int) []RewardDistribution {
	rm.lock.RLock()
	defer rm.lock.RUnlock()

	historyLen := len(rm.distributionHistory)
	if count <= 0 || count > historyLen {
		count = historyLen
	}

	result := make([]RewardDistribution, count)
	for i := 0; i < count; i++ {
		result[i] = rm.distributionHistory[historyLen-count+i]
	}
	return result
}

// SaveToState는 보상 상태를 상태 DB에 저장합니다
func (rm *RewardManager) SaveToState(state *state.StateDB) error {
	rm.lock.Lock()
	defer rm.lock.Unlock()
	
	// 누적 보상 저장
	for addr, amount := range rm.accumulatedRewards {
		rewardKey := append([]byte("reward-"), addr.Bytes()...)
		state.SetState(common.HexToAddress("0x0000000000000000000000000000000000000200"), common.BytesToHash(rewardKey), common.BytesToHash(amount.Bytes()))
	}
	
	// 커뮤니티 풀 금액 저장
	communityPoolKey := []byte("community-pool")
	state.SetState(common.HexToAddress("0x0000000000000000000000000000000000000200"), common.BytesToHash(communityPoolKey), common.BytesToHash(rm.communityFund.Bytes()))
	
	return nil
}

// LoadFromState는 상태 DB에서 보상 상태를 로드합니다
func (rm *RewardManager) LoadFromState(state *state.StateDB) error {
	rm.lock.Lock()
	defer rm.lock.Unlock()
	
	// 커뮤니티 풀 금액 로드
	communityPoolKey := []byte("community-pool")
	communityPoolHash := state.GetState(common.HexToAddress("0x0000000000000000000000000000000000000200"), common.BytesToHash(communityPoolKey))
	if communityPoolHash != (common.Hash{}) {
		rm.communityFund = new(big.Int).SetBytes(communityPoolHash.Bytes())
	}
	
	// 실제 구현에서는 모든 검증자의 누적 보상을 로드해야 함
	// 여기서는 간단한 예시만 제공
	
	return nil
}
