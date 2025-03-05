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

// Package eirene implements the Proof-of-Stake consensus algorithm.
package staking

import (
	"fmt"
	"math/big"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/params"
)

// StakingAdapter는 Cosmos SDK의 staking 모듈과 go-zenanet의 검증자 관리 시스템을 연결하는 어댑터입니다.
type StakingAdapter struct {
	logger        log.Logger
	minStake      *big.Int // 최소 스테이킹 양
	maxValidators int      // 최대 검증자 수
	validatorSet  *ValidatorSet // 검증자 집합
}

// NewStakingAdapter는 새로운 StakingAdapter 인스턴스를 생성합니다.
func NewStakingAdapter(validatorSet *ValidatorSet, config *params.EireneConfig) *StakingAdapter {
	maxValidatorCount := 100 // 기본값
	if config != nil && config.SlashingThreshold > 0 {
		// SlashingThreshold를 임시로 사용 (MaxValidators가 없으므로)
		maxValidatorCount = int(config.SlashingThreshold)
	}

	return &StakingAdapter{
		logger:        log.New("module", "staking"),
		minStake:      big.NewInt(1000000000000000000), // 1 ETH
		maxValidators: maxValidatorCount,
		validatorSet:  validatorSet,
	}
}

// ValidatorDescription은 검증자 설명 정보를 나타내는 구조체입니다.
type ValidatorDescription struct {
	Moniker         string // 검증자 이름
	Identity        string // 신원 정보
	Website         string // 웹사이트
	SecurityContact string // 보안 연락처
	Details         string // 상세 정보
}

// Delegation은 위임 정보를 나타내는 구조체입니다.
type Delegation struct {
	DelegatorAddress common.Address // 위임자 주소
	ValidatorAddress common.Address // 검증자 주소
	Shares           *big.Int       // 위임 지분
}

// Stake는 토큰을 스테이킹하여 검증자가 됩니다.
func (a *StakingAdapter) Stake(state *state.StateDB, address common.Address, amount *big.Int, pubKey []byte, description ValidatorDescription, commission *big.Int) error {
	a.logger.Info("Staking tokens", "address", address.Hex(), "amount", amount.String())

	// 최소 스테이킹 양 확인
	if amount.Cmp(a.minStake) < 0 {
		return fmt.Errorf("staking amount is less than minimum required: %s < %s", amount.String(), a.minStake.String())
	}

	// 이미 검증자인지 확인
	if a.isValidator(address) {
		return fmt.Errorf("address is already a validator: %s", address.Hex())
	}

	// 계정 잔액 확인
	balance := state.GetBalance(address)
	if balance.Uint64() < amount.Uint64() {
		return fmt.Errorf("insufficient balance for staking: %s < %s", balance.String(), amount.String())
	}

	// 검증자 수 제한 확인
	validators, err := a.GetValidators()
	if err != nil {
		return err
	}
	if len(validators) >= a.maxValidators {
		return fmt.Errorf("maximum validator count reached: %d", a.maxValidators)
	}

	// 검증자 생성
	validator := &Validator{
		Address:     address,
		PubKey:      pubKey,
		VotingPower: amount,
		Status:      ValidatorStatusActive,
		Commission:  uint64(commission.Uint64()),
		SelfStake:   amount,
		Delegations: make(map[common.Address]*ValidatorDelegation),
		// 기타 필드 초기화
		BlocksProposed:     0,
		BlocksSigned:       0,
		BlocksMissed:       0,
		Uptime:             100, // 초기 업타임 100%
		GovernanceVotes:    0,
		AccumulatedRewards: new(big.Int),
		LastRewardBlock:    0,
		JailedUntil:        0,
		SlashingCount:      0,
		LastSlashedBlock:   0,
	}

	// 자기 위임 생성
	selfDelegation := &ValidatorDelegation{
		Delegator:          address,
		Amount:             amount,
		AccumulatedRewards: new(big.Int),
		StartBlock:         0, // 현재 블록 높이 사용 (간단히 0으로 설정)
		EndBlock:           0, // 종료 블록 없음
	}
	validator.Delegations[address] = selfDelegation

	// 검증자 추가
	err = a.addValidator(validator)
	if err != nil {
		return err
	}

	// 토큰 스테이킹 (잔액에서 차감)
	// 실제 구현에서는 state.SubBalance(address, amount, reason) 형태로 호출
	// 여기서는 간단히 구현
	// state.SubBalance(address, amount)

	a.logger.Info("New validator staked", "address", address.Hex(), "amount", amount.String())

	return nil
}

// Unstake는 스테이킹된 토큰을 해제합니다.
func (a *StakingAdapter) Unstake(state *state.StateDB, address common.Address) error {
	a.logger.Info("Unstaking tokens", "address", address.Hex())

	// 검증자 확인
	validator, err := a.GetValidator(address)
	if err != nil {
		return err
	}

	// 검증자 제거
	err = a.removeValidator(address)
	if err != nil {
		return err
	}

	// 언스테이킹 기간 설정 (실제 구현에서는 언스테이킹 기간 후 토큰 반환)
	// 여기서는 간단히 바로 반환
	// 실제 구현에서는 state.AddBalance(address, amount, reason) 형태로 호출
	// 여기서는 간단히 구현
	// state.AddBalance(address, validator.VotingPower)

	a.logger.Info("Validator unstaked", "address", address.Hex(), "amount", validator.VotingPower.String())

	return nil
}

// Delegate는 토큰을 검증자에게 위임합니다.
func (a *StakingAdapter) Delegate(state *state.StateDB, delegator common.Address, validator common.Address, amount *big.Int) error {
	a.logger.Info("Delegating tokens", "delegator", delegator.Hex(), "validator", validator.Hex(), "amount", amount.String())

	// 검증자 확인
	validatorInfo, err := a.GetValidator(validator)
	if err != nil {
		return err
	}

	// 계정 잔액 확인
	balance := state.GetBalance(delegator)
	// 실제 구현에서는 balance.Cmp(amount) 형태로 호출
	// 여기서는 간단히 구현
	if balance.Uint64() < amount.Uint64() {
		return fmt.Errorf("insufficient balance for delegation: %s < %s", balance.String(), amount.String())
	}

	// 기존 위임이 있는지 확인
	for addr, delegation := range validatorInfo.Delegations {
		if addr == delegator {
			// 기존 위임에 추가
			delegation.Amount = new(big.Int).Add(delegation.Amount, amount)

			// 검증자 투표력 업데이트
			validatorInfo.VotingPower = new(big.Int).Add(validatorInfo.VotingPower, amount)

			// 검증자 업데이트
			err = a.updateValidator(validatorInfo)
			if err != nil {
				return err
			}

			// 토큰 위임 (잔액에서 차감)
			// 실제 구현에서는 state.SubBalance(delegator, amount, reason) 형태로 호출
			// 여기서는 간단히 구현
			// state.SubBalance(delegator, amount)

			a.logger.Info("Delegation added", "delegator", delegator.Hex(), "validator", validator.Hex(), "amount", amount.String())

			return nil
		}
	}

	// 새 위임 추가
	newDelegation := &ValidatorDelegation{
		Delegator:          delegator,
		Amount:             amount,
		AccumulatedRewards: new(big.Int),
		StartBlock:         0, // 현재 블록 높이 사용 (간단히 0으로 설정)
		EndBlock:           0, // 종료 블록 없음
	}
	validatorInfo.Delegations[delegator] = newDelegation

	// 검증자 투표력 업데이트
	validatorInfo.VotingPower = new(big.Int).Add(validatorInfo.VotingPower, amount)

	// 검증자 업데이트
	err = a.updateValidator(validatorInfo)
	if err != nil {
		return err
	}

	// 토큰 위임 (잔액에서 차감)
	// 실제 구현에서는 state.SubBalance(delegator, amount, reason) 형태로 호출
	// 여기서는 간단히 구현
	// state.SubBalance(delegator, amount)

	a.logger.Info("New delegation created", "delegator", delegator.Hex(), "validator", validator.Hex(), "amount", amount.String())

	return nil
}

// Undelegate는 위임된 토큰을 해제합니다.
func (a *StakingAdapter) Undelegate(state *state.StateDB, delegator common.Address, validator common.Address, amount *big.Int) error {
	a.logger.Info("Undelegating tokens", "delegator", delegator.Hex(), "validator", validator.Hex(), "amount", amount.String())

	// 검증자 확인
	validatorInfo, err := a.GetValidator(validator)
	if err != nil {
		return err
	}

	// 위임 확인
	delegation, exists := validatorInfo.Delegations[delegator]
	if !exists {
		return fmt.Errorf("delegation not found for delegator %s and validator %s", delegator.Hex(), validator.Hex())
	}

	// 위임 금액 확인
	if delegation.Amount.Cmp(amount) < 0 {
		return fmt.Errorf("insufficient delegation amount: %s < %s", delegation.Amount.String(), amount.String())
	}

	// 검증자 투표력 업데이트
	validatorInfo.VotingPower = new(big.Int).Sub(validatorInfo.VotingPower, amount)

	// 위임 업데이트 또는 삭제
	if delegation.Amount.Cmp(amount) == 0 {
		// 위임 삭제
		delete(validatorInfo.Delegations, delegator)
	} else {
		// 위임 감소
		delegation.Amount = new(big.Int).Sub(delegation.Amount, amount)
	}

	// 검증자 업데이트
	err = a.updateValidator(validatorInfo)
	if err != nil {
		return err
	}

	// 언위임 기간 설정 (실제 구현에서는 언위임 기간 후 토큰 반환)
	// 여기서는 간단히 바로 반환
	// 실제 구현에서는 state.AddBalance(delegator, amount, reason) 형태로 호출
	// 여기서는 간단히 구현
	// state.AddBalance(delegator, amount)

	a.logger.Info("Delegation removed", "delegator", delegator.Hex(), "validator", validator.Hex(), "amount", amount.String())

	return nil
}

// Redelegate는 위임된 토큰을 다른 검증자에게 재위임합니다.
func (a *StakingAdapter) Redelegate(state *state.StateDB, delegator common.Address, srcValidator common.Address, dstValidator common.Address, amount *big.Int) error {
	a.logger.Info("Redelegating tokens", "delegator", delegator.Hex(), "srcValidator", srcValidator.Hex(), "dstValidator", dstValidator.Hex(), "amount", amount.String())

	// 소스 검증자 확인
	srcValidatorInfo, err := a.GetValidator(srcValidator)
	if err != nil {
		return err
	}

	// 대상 검증자 확인
	dstValidatorInfo, err := a.GetValidator(dstValidator)
	if err != nil {
		return err
	}

	// 소스 위임 확인
	srcDelegation, exists := srcValidatorInfo.Delegations[delegator]
	if !exists {
		return fmt.Errorf("delegation not found for delegator %s and validator %s", delegator.Hex(), srcValidator.Hex())
	}

	// 위임 금액 확인
	if srcDelegation.Amount.Cmp(amount) < 0 {
		return fmt.Errorf("insufficient delegation amount: %s < %s", srcDelegation.Amount.String(), amount.String())
	}

	// 소스 검증자 투표력 업데이트
	srcValidatorInfo.VotingPower = new(big.Int).Sub(srcValidatorInfo.VotingPower, amount)

	// 소스 위임 업데이트 또는 삭제
	if srcDelegation.Amount.Cmp(amount) == 0 {
		// 위임 삭제
		delete(srcValidatorInfo.Delegations, delegator)
	} else {
		// 위임 감소
		srcDelegation.Amount = new(big.Int).Sub(srcDelegation.Amount, amount)
	}

	// 소스 검증자 업데이트
	err = a.updateValidator(srcValidatorInfo)
	if err != nil {
		return err
	}

	// 대상 위임 확인
	dstDelegation, exists := dstValidatorInfo.Delegations[delegator]
	if exists {
		// 기존 위임에 추가
		dstDelegation.Amount = new(big.Int).Add(dstDelegation.Amount, amount)
	} else {
		// 새 위임 추가
		newDelegation := &ValidatorDelegation{
			Delegator:          delegator,
			Amount:             amount,
			AccumulatedRewards: new(big.Int),
			StartBlock:         0, // 현재 블록 높이 사용 (간단히 0으로 설정)
			EndBlock:           0, // 종료 블록 없음
		}
		dstValidatorInfo.Delegations[delegator] = newDelegation
	}

	// 대상 검증자 투표력 업데이트
	dstValidatorInfo.VotingPower = new(big.Int).Add(dstValidatorInfo.VotingPower, amount)

	// 대상 검증자 업데이트
	err = a.updateValidator(dstValidatorInfo)
	if err != nil {
		return err
	}

	a.logger.Info("Delegation redelegated", "delegator", delegator.Hex(), "srcValidator", srcValidator.Hex(), "dstValidator", dstValidator.Hex(), "amount", amount.String())

	return nil
}

// GetValidator는 검증자 정보를 반환합니다.
func (a *StakingAdapter) GetValidator(address common.Address) (*Validator, error) {
	// 실제 구현에서는 a.eirene.GetValidatorSet().GetValidator(address) 형태로 호출
	// 여기서는 간단히 구현
	return &Validator{
		Address:     address,
		VotingPower: big.NewInt(0),
		Delegations: make(map[common.Address]*ValidatorDelegation),
	}, nil
}

// GetValidators는 모든 검증자 정보를 반환합니다.
func (a *StakingAdapter) GetValidators() ([]*Validator, error) {
	// 실제 구현에서는 a.eirene.GetValidatorSet().GetValidators() 형태로 호출
	// 여기서는 간단히 구현
	return []*Validator{}, nil
}

// GetDelegation은 위임 정보를 반환합니다.
func (a *StakingAdapter) GetDelegation(delegator common.Address, validator common.Address) (*ValidatorDelegation, error) {
	// 검증자 확인
	validatorInfo, err := a.GetValidator(validator)
	if err != nil {
		return nil, err
	}

	// 위임 확인
	delegation, exists := validatorInfo.Delegations[delegator]
	if !exists {
		return nil, fmt.Errorf("delegation not found for delegator %s and validator %s", delegator.Hex(), validator.Hex())
	}

	return delegation, nil
}

// GetDelegations는 위임자의 모든 위임 정보를 반환합니다.
func (a *StakingAdapter) GetDelegations(delegator common.Address) ([]*ValidatorDelegation, error) {
	// 모든 검증자 가져오기
	validators, err := a.GetValidators()
	if err != nil {
		return nil, err
	}

	// 위임 목록 생성
	delegations := []*ValidatorDelegation{}

	// 모든 검증자의 위임 확인
	for _, v := range validators {
		if delegation, exists := v.Delegations[delegator]; exists {
			delegations = append(delegations, delegation)
		}
	}

	return delegations, nil
}

// isValidator는 주소가 검증자인지 확인합니다.
func (a *StakingAdapter) isValidator(address common.Address) bool {
	// 실제 구현에서는 a.eirene.GetValidatorSet().GetValidatorByAddress(address) 형태로 호출
	// 여기서는 간단히 구현
	_, err := a.GetValidator(address)
	return err == nil
}

// addValidator는 검증자를 추가합니다.
func (a *StakingAdapter) addValidator(validator *Validator) error {
	// 실제 구현에서는 a.eirene.GetValidatorSet().AddValidator(validator) 형태로 호출
	// 여기서는 간단히 구현
	return nil
}

// updateValidator는 검증자 정보를 업데이트합니다.
func (a *StakingAdapter) updateValidator(validator *Validator) error {
	// 실제 구현에서는 a.eirene.GetValidatorSet().UpdateValidator(validator) 형태로 호출
	// 여기서는 간단히 구현
	return nil
}

// removeValidator는 검증자를 제거합니다.
func (a *StakingAdapter) removeValidator(address common.Address) error {
	// 실제 구현에서는 a.eirene.GetValidatorSet().RemoveValidator(address) 형태로 호출
	// 여기서는 간단히 구현
	return nil
}
