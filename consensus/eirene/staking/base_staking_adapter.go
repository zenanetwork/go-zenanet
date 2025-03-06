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
	"fmt"
	"math/big"

	"github.com/holiman/uint256"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/tracing"
	"github.com/zenanetwork/go-zenanet/params"
)

// BaseStakingAdapter는 스테이킹 어댑터의 공통 기능을 제공하는 기본 구현체입니다.
// StakingAdapter와 CosmosStakingAdapter에서 공통으로 사용되는 기능을 포함합니다.
type BaseStakingAdapter struct {
	logger        utils.Logger
	minStake      *big.Int // 최소 스테이킹 양
	maxValidators int      // 최대 검증자 수
	validatorSet  *ValidatorSet // 검증자 집합
}

// NewBaseStakingAdapter는 새로운 BaseStakingAdapter 인스턴스를 생성합니다.
func NewBaseStakingAdapter(validatorSet *ValidatorSet, config *params.EireneConfig) *BaseStakingAdapter {
	maxValidatorCount := 100 // 기본값
	if config != nil && config.SlashingThreshold > 0 {
		// SlashingThreshold를 임시로 사용 (MaxValidators가 없으므로)
		maxValidatorCount = int(config.SlashingThreshold)
	}

	return &BaseStakingAdapter{
		logger:        utils.NewLogger("base-staking"),
		minStake:      big.NewInt(1000000000000000000), // 1 ETH
		maxValidators: maxValidatorCount,
		validatorSet:  validatorSet,
	}
}

// GetValidatorSet은 검증자 집합을 반환합니다.
func (a *BaseStakingAdapter) GetValidatorSet() *ValidatorSet {
	return a.validatorSet
}

// SetValidatorSet은 검증자 집합을 설정합니다.
func (a *BaseStakingAdapter) SetValidatorSet(validatorSet *ValidatorSet) {
	a.validatorSet = validatorSet
}

// GetValidator는 주소로 검증자를 조회합니다.
func (a *BaseStakingAdapter) GetValidator(address common.Address) (*Validator, error) {
	validator := a.validatorSet.GetValidator(address)
	if validator == nil {
		return nil, utils.WrapError(utils.ErrValidatorNotFound, fmt.Sprintf("validator not found: %s", address.Hex()))
	}
	return validator, nil
}

// GetValidators는 모든 검증자 목록을 반환합니다.
func (a *BaseStakingAdapter) GetValidators() []*Validator {
	return a.validatorSet.GetValidators()
}

// IsValidator는 주소가 검증자인지 확인합니다.
func (a *BaseStakingAdapter) IsValidator(address common.Address) bool {
	validator := a.validatorSet.GetValidator(address)
	return validator != nil
}

// AddValidator는 검증자를 추가합니다.
func (a *BaseStakingAdapter) AddValidator(validator *Validator) error {
	if validator == nil {
		return utils.ErrInvalidParameter
	}
	
	// 검증자 추가
	a.validatorSet.AddValidator(validator)
	
	a.logger.Info("Validator added", "address", validator.Address.Hex(), "voting_power", validator.VotingPower.String())
	return nil
}

// UpdateValidator는 검증자 정보를 업데이트합니다.
func (a *BaseStakingAdapter) UpdateValidator(validator *Validator) error {
	if validator == nil {
		return utils.ErrInvalidParameter
	}
	
	// 기존 검증자 확인
	existingValidator := a.validatorSet.GetValidator(validator.Address)
	if existingValidator == nil {
		return utils.WrapError(utils.ErrValidatorNotFound, fmt.Sprintf("validator not found: %s", validator.Address.Hex()))
	}
	
	// 검증자 정보 업데이트 (기존 검증자 제거 후 새로 추가)
	removedValidator := a.validatorSet.RemoveValidator(validator.Address)
	if removedValidator == nil {
		return utils.WrapError(utils.ErrValidatorNotFound, fmt.Sprintf("failed to remove validator: %s", validator.Address.Hex()))
	}
	
	a.validatorSet.AddValidator(validator)
	
	a.logger.Info("Validator updated", "address", validator.Address.Hex(), "voting_power", validator.VotingPower.String())
	return nil
}

// RemoveValidator는 검증자를 제거합니다.
func (a *BaseStakingAdapter) RemoveValidator(address common.Address) error {
	// 검증자 제거
	removedValidator := a.validatorSet.RemoveValidator(address)
	if removedValidator == nil {
		return utils.WrapError(utils.ErrValidatorNotFound, fmt.Sprintf("validator not found: %s", address.Hex()))
	}
	
	a.logger.Info("Validator removed", "address", address.Hex())
	return nil
}

// CheckStakingRequirements는 스테이킹 요구사항을 확인합니다.
func (a *BaseStakingAdapter) CheckStakingRequirements(stateDB *state.StateDB, operator common.Address, amount *big.Int) error {
	// 최소 스테이킹 양 확인
	if amount.Cmp(a.minStake) < 0 {
		return utils.WrapError(utils.ErrInsufficientStake, 
			fmt.Sprintf("staking amount is less than minimum required: %s < %s", amount.String(), a.minStake.String()))
	}

	// 이미 검증자인지 확인
	if a.IsValidator(operator) {
		return utils.WrapError(utils.ErrValidatorExists, 
			fmt.Sprintf("address is already a validator: %s", operator.Hex()))
	}

	// 계정 잔액 확인
	balance := stateDB.GetBalance(operator)
	amountUint256, _ := uint256.FromBig(amount)
	if balance.Cmp(amountUint256) < 0 {
		return utils.WrapError(utils.ErrInsufficientBalance, 
			fmt.Sprintf("insufficient balance for staking: %s < %s", balance.String(), amount.String()))
	}

	// 검증자 수 제한 확인
	validators := a.GetValidators()
	if len(validators) >= a.maxValidators {
		return utils.FormatError(utils.ErrOperationFailed, 
			"maximum validator count reached: %d", a.maxValidators)
	}
	
	return nil
}

// DeductBalance는 계정에서 잔액을 차감합니다.
func (a *BaseStakingAdapter) DeductBalance(stateDB *state.StateDB, address common.Address, amount *big.Int) error {
	amountUint256, _ := uint256.FromBig(amount)
	stateDB.SubBalance(address, amountUint256, tracing.BalanceChangeUnspecified)
	return nil
}

// AddBalance는 계정에 잔액을 추가합니다.
func (a *BaseStakingAdapter) AddBalance(stateDB *state.StateDB, address common.Address, amount *big.Int) error {
	amountUint256, _ := uint256.FromBig(amount)
	stateDB.AddBalance(address, amountUint256, tracing.BalanceChangeUnspecified)
	return nil
}

// CreateValidatorObject는 검증자 객체를 생성합니다.
func (a *BaseStakingAdapter) CreateValidatorObject(operator common.Address, pubKey []byte, amount *big.Int, description ValidatorDescription, commission *big.Int) *Validator {
	// 검증자 생성
	validator := &Validator{
		Address:     operator,
		PubKey:      pubKey,
		VotingPower: amount,
		Status:      ValidatorStatusBonded,
		Commission:  new(big.Int).Set(commission),
		SelfStake:   amount,
		Delegations: []*ValidatorDelegation{},
		Description: description,
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
		Delegator:          operator,
		Amount:             amount,
		AccumulatedRewards: new(big.Int),
		StartBlock:         0, // 현재 블록 높이 사용 (간단히 0으로 설정)
		EndBlock:           0, // 종료 블록 없음
	}
	validator.Delegations = append(validator.Delegations, selfDelegation)
	
	return validator
}

// GetDelegation은 위임 정보를 조회합니다.
func (a *BaseStakingAdapter) GetDelegation(delegator common.Address, validator common.Address) (*ValidatorDelegation, error) {
	validatorInfo, err := a.GetValidator(validator)
	if err != nil {
		return nil, err
	}

	for _, delegation := range validatorInfo.Delegations {
		if delegation.Delegator == delegator {
			return delegation, nil
		}
	}

	return nil, utils.WrapError(utils.ErrDelegationNotFound, 
		fmt.Sprintf("delegation not found: delegator=%s, validator=%s", delegator.Hex(), validator.Hex()))
}

// GetDelegations은 위임자의 모든 위임 정보를 조회합니다.
func (a *BaseStakingAdapter) GetDelegations(delegator common.Address) ([]*ValidatorDelegation, error) {
	validators := a.GetValidators()

	delegations := []*ValidatorDelegation{}
	for _, validator := range validators {
		for _, delegation := range validator.Delegations {
			if delegation.Delegator == delegator {
				delegations = append(delegations, delegation)
			}
		}
	}

	return delegations, nil
} 