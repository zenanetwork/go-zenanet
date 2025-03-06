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

	"github.com/holiman/uint256"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/params"
)

// StakingAdapter는 Cosmos SDK의 staking 모듈과 go-zenanet의 검증자 관리 시스템을 연결하는 어댑터입니다.
// StakingAdapterInterface를 구현합니다.
type StakingAdapter struct {
	*BaseStakingAdapter
	logger log.Logger
}

// 컴파일 타임에 인터페이스 구현 여부 확인
var _ StakingAdapterInterface = (*StakingAdapter)(nil)

// NewStakingAdapter는 새로운 StakingAdapter 인스턴스를 생성합니다.
func NewStakingAdapter(validatorSet *ValidatorSet, config *params.EireneConfig) *StakingAdapter {
	baseAdapter := NewBaseStakingAdapter(validatorSet, config)
	
	return &StakingAdapter{
		BaseStakingAdapter: baseAdapter,
		logger:             log.New("module", "staking"),
	}
}

// ValidatorDescription은 검증자 설명 정보를 나타내는 구조체입니다.
//
// 이 구조체는 검증자의 이름, 웹사이트, 신원 정보, 세부 정보 등을 포함합니다.
// 검증자 생성 시 제공되며, 검증자 정보 조회 시 사용됩니다.
// 사용자들이 검증자를 식별하고 평가하는 데 도움이 되는 정보를 제공합니다.
type ValidatorDescription struct {
	Moniker         string // 검증자 이름
	Identity        string // 신원 정보 (예: Keybase 식별자)
	Website         string // 웹사이트 URL
	SecurityContact string // 보안 연락처
	Details         string // 상세 설명
}

// Delegation은 위임 정보를 나타내는 구조체입니다.
type Delegation struct {
	DelegatorAddress common.Address // 위임자 주소
	ValidatorAddress common.Address // 검증자 주소
	Shares           *big.Int       // 위임 지분
}

// Stake는 검증자로 스테이킹합니다.
//
// 매개변수:
//   - stateDB: 상태 데이터베이스
//   - operator: 검증자 운영자 주소
//   - amount: 스테이킹할 금액
//   - pubKey: 검증자 공개 키
//   - description: 검증자 설명 정보
//   - commission: 커미션 비율 (1e18 = 100%)
//
// 반환값:
//   - error: 스테이킹 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 새로운 검증자를 생성하거나 기존 검증자의 스테이킹 금액을 증가시킵니다.
// 최소 스테이킹 금액보다 적은 금액으로 스테이킹할 경우 오류를 반환합니다.
// 검증자 생성 시 공개 키와 설명 정보를 함께 등록합니다.
// 스테이킹 성공 시 검증자 집합에 추가되고 투표 권한이 계산됩니다.
func (a *StakingAdapter) Stake(stateDB *state.StateDB, operator common.Address, amount *big.Int, pubKey []byte, description ValidatorDescription, commission *big.Int) error {
	a.logger.Info("Staking tokens", "address", operator.Hex(), "amount", amount.String())

	// 스테이킹 요구사항 확인
	err := a.CheckStakingRequirements(stateDB, operator, amount)
	if err != nil {
		return err
	}

	// 잔액 차감
	err = a.DeductBalance(stateDB, operator, amount)
	if err != nil {
		return err
	}

	// 검증자 생성
	validator := a.CreateValidatorObject(operator, pubKey, amount, description, commission)

	// 검증자 추가
	err = a.AddValidator(validator)
	if err != nil {
		// 실패 시 잔액 반환
		a.AddBalance(stateDB, operator, amount)
		return utils.WrapError(err, fmt.Sprintf("failed to add validator: %s", operator.Hex()))
	}

	a.logger.Info("New validator created", "address", operator.Hex(), "amount", amount.String())

	return nil
}

// Unstake는 검증자의 스테이킹을 해제합니다.
//
// 매개변수:
//   - stateDB: 상태 데이터베이스
//   - operator: 검증자 운영자 주소
//
// 반환값:
//   - error: 언스테이킹 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 검증자의 스테이킹을 해제하고 스테이킹된 금액을 반환합니다.
// 검증자가 존재하지 않거나 이미 언스테이킹 중인 경우 오류를 반환합니다.
// 언스테이킹은 즉시 처리되지 않고 언바운딩 기간 후에 완료됩니다.
// 언스테이킹 시작 시 검증자 상태가 언바운딩으로 변경되고 투표 권한이 0으로 설정됩니다.
func (a *StakingAdapter) Unstake(stateDB *state.StateDB, operator common.Address) error {
	a.logger.Info("Unstaking tokens", "address", operator.Hex())

	// 검증자 확인
	validator, err := a.GetValidator(operator)
	if err != nil {
		return err
	}

	// 자기 위임 금액 가져오기
	selfStake := validator.SelfStake

	// 검증자 제거
	err = a.RemoveValidator(operator)
	if err != nil {
		return err
	}

	// 잔액 반환
	err = a.AddBalance(stateDB, operator, selfStake)
	if err != nil {
		return err
	}

	a.logger.Info("Validator unstaked", "address", operator.Hex(), "amount", selfStake.String())
	return nil
}

// Delegate는 검증자에게 토큰을 위임합니다.
//
// 매개변수:
//   - stateDB: 상태 데이터베이스
//   - delegator: 위임자 주소
//   - validator: 검증자 주소
//   - amount: 위임할 금액
//
// 반환값:
//   - error: 위임 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 위임자가 검증자에게 토큰을 위임하는 기능을 제공합니다.
// 검증자가 존재하지 않거나 활성 상태가 아닌 경우 오류를 반환합니다.
// 최소 위임 금액보다 적은 금액으로 위임할 경우 오류를 반환합니다.
// 위임 성공 시 검증자의 위임 금액이 증가하고 투표 권한이 재계산됩니다.
func (a *StakingAdapter) Delegate(stateDB *state.StateDB, delegator common.Address, validator common.Address, amount *big.Int) error {
	a.logger.Info("Delegating tokens", "delegator", delegator.Hex(), "validator", validator.Hex(), "amount", amount.String())

	// 계정 잔액 확인
	balance := stateDB.GetBalance(delegator)
	amountUint256, _ := uint256.FromBig(amount)
	if balance.Cmp(amountUint256) < 0 {
		return utils.WrapError(utils.ErrInsufficientBalance, 
			fmt.Sprintf("insufficient balance for delegation: %s < %s", balance.String(), amount.String()))
	}

	// 잔액 차감
	err := a.DeductBalance(stateDB, delegator, amount)
	if err != nil {
		return err
	}

	// 검증자 확인
	validatorInfo, err := a.GetValidator(validator)
	if err != nil {
		// 실패 시 잔액 반환
		a.AddBalance(stateDB, delegator, amount)
		return err
	}

	// 기존 위임이 있는지 확인
	for _, delegation := range validatorInfo.Delegations {
		if delegation.Delegator == delegator {
			// 기존 위임에 추가
			delegation.Amount = new(big.Int).Add(delegation.Amount, amount)

			// 검증자 투표력 업데이트
			validatorInfo.VotingPower = new(big.Int).Add(validatorInfo.VotingPower, amount)

			// 검증자 업데이트
			err = a.UpdateValidator(validatorInfo)
			if err != nil {
				// 실패 시 잔액 반환
				a.AddBalance(stateDB, delegator, amount)
				return err
			}

			a.logger.Info("Delegation added to existing", "delegator", delegator.Hex(), "validator", validator.Hex(), "amount", amount.String())
			return nil
		}
	}

	// 새 위임 생성
	newDelegation := &ValidatorDelegation{
		Delegator:          delegator,
		Amount:             amount,
		AccumulatedRewards: new(big.Int),
		StartBlock:         0, // 현재 블록 높이 사용 (간단히 0으로 설정)
		EndBlock:           0, // 종료 블록 없음
	}
	validatorInfo.Delegations = append(validatorInfo.Delegations, newDelegation)

	// 검증자 투표력 업데이트
	validatorInfo.VotingPower = new(big.Int).Add(validatorInfo.VotingPower, amount)

	// 검증자 업데이트
	err = a.UpdateValidator(validatorInfo)
	if err != nil {
		// 실패 시 잔액 반환
		a.AddBalance(stateDB, delegator, amount)
		return err
	}

	a.logger.Info("New delegation created", "delegator", delegator.Hex(), "validator", validator.Hex(), "amount", amount.String())
	return nil
}

// Undelegate는 검증자로부터 위임을 해제합니다.
//
// 매개변수:
//   - stateDB: 상태 데이터베이스
//   - delegator: 위임자 주소
//   - validator: 검증자 주소
//   - amount: 언위임할 금액
//
// 반환값:
//   - error: 언위임 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 위임자가 검증자에게 위임한 토큰을 회수하는 기능을 제공합니다.
// 검증자가 존재하지 않거나 위임 기록이 없는 경우 오류를 반환합니다.
// 언위임은 즉시 처리되지 않고 언바운딩 기간 후에 완료됩니다.
// 언위임 시작 시 위임 금액이 감소하고 검증자의 투표 권한이 재계산됩니다.
func (a *StakingAdapter) Undelegate(stateDB *state.StateDB, delegator common.Address, validator common.Address, amount *big.Int) error {
	a.logger.Info("Undelegating tokens", "delegator", delegator.Hex(), "validator", validator.Hex(), "amount", amount.String())

	// 검증자 확인
	validatorInfo, err := a.GetValidator(validator)
	if err != nil {
		return err
	}

	// 위임 찾기
	for i, delegation := range validatorInfo.Delegations {
		if delegation.Delegator == delegator {
			// 위임 금액 확인
			if delegation.Amount.Cmp(amount) < 0 {
				return utils.WrapError(utils.ErrInvalidStakeAmount, 
					fmt.Sprintf("undelegation amount exceeds delegation: %s > %s", amount.String(), delegation.Amount.String()))
			}

			// 위임 금액 감소
			delegation.Amount = new(big.Int).Sub(delegation.Amount, amount)

			// 잔액 반환
			err = a.AddBalance(stateDB, delegator, amount)
			if err != nil {
				return err
			}

			// 검증자 투표력 업데이트
			validatorInfo.VotingPower = new(big.Int).Sub(validatorInfo.VotingPower, amount)

			// 위임 금액이 0이면 위임 제거
			if delegation.Amount.Cmp(big.NewInt(0)) == 0 {
				validatorInfo.Delegations = append(validatorInfo.Delegations[:i], validatorInfo.Delegations[i+1:]...)
			}

			// 검증자 업데이트
			err = a.UpdateValidator(validatorInfo)
			if err != nil {
				// 실패 시 위임 복원 및 잔액 차감
				delegation.Amount = new(big.Int).Add(delegation.Amount, amount)
				validatorInfo.VotingPower = new(big.Int).Add(validatorInfo.VotingPower, amount)
				a.DeductBalance(stateDB, delegator, amount)
				return err
			}

			a.logger.Info("Tokens undelegated", "delegator", delegator.Hex(), "validator", validator.Hex(), "amount", amount.String())
			return nil
		}
	}

	return utils.WrapError(utils.ErrDelegationNotFound, 
		fmt.Sprintf("delegation not found: delegator=%s, validator=%s", delegator.Hex(), validator.Hex()))
}

// Redelegate는 한 검증자에서 다른 검증자로 위임을 재배치합니다.
//
// 매개변수:
//   - stateDB: 상태 데이터베이스
//   - delegator: 위임자 주소
//   - srcValidator: 소스 검증자 주소
//   - dstValidator: 대상 검증자 주소
//   - amount: 재위임할 금액
//
// 반환값:
//   - error: 재위임 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 위임자가 한 검증자에서 다른 검증자로 위임을 이동하는 기능을 제공합니다.
// 소스 또는 대상 검증자가 존재하지 않거나 활성 상태가 아닌 경우 오류를 반환합니다.
// 소스 검증자에 대한 위임 기록이 없거나 위임 금액이 부족한 경우 오류를 반환합니다.
// 재위임 성공 시 소스 검증자의 위임 금액이 감소하고 대상 검증자의 위임 금액이 증가합니다.
// 두 검증자의 투표 권한이 재계산됩니다.
func (a *StakingAdapter) Redelegate(stateDB *state.StateDB, delegator common.Address, srcValidator common.Address, dstValidator common.Address, amount *big.Int) error {
	a.logger.Info("Redelegating tokens", 
		"delegator", delegator.Hex(), 
		"src_validator", srcValidator.Hex(), 
		"dst_validator", dstValidator.Hex(), 
		"amount", amount.String())

	// 소스 검증자에서 위임 철회
	err := a.Undelegate(stateDB, delegator, srcValidator, amount)
	if err != nil {
		return err
	}

	// 대상 검증자에게 위임
	err = a.Delegate(stateDB, delegator, dstValidator, amount)
	if err != nil {
		// 실패 시 원래 검증자에게 다시 위임
		a.Delegate(stateDB, delegator, srcValidator, amount)
		return err
	}

	return nil
}

// BeginBlock은 블록 처리 시작 시 호출됩니다.
//
// 매개변수:
//   - height: 블록 높이
//   - time: 블록 타임스탬프
//
// 반환값:
//   - error: 처리 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 각 블록 처리 시작 시 호출되어 스테이킹 모듈의 상태를 업데이트합니다.
// 블록 높이와 타임스탬프를 기록하고, 언바운딩 기간이 완료된 스테이킹과 위임을 처리합니다.
// 검증자 성능 지표를 업데이트하고, 슬래싱 조건을 확인합니다.
// 에포크 전환 시 검증자 집합을 재구성합니다.
func (a *StakingAdapter) BeginBlock(height uint64, time uint64) error {
	a.logger.Debug("Begin block", "height", height, "time", time)
	// 블록 시작 시 필요한 작업 수행
	return nil
}

// EndBlock은 블록 처리 종료 시 호출됩니다.
//
// 매개변수:
//   - height: 블록 높이
//
// 반환값:
//   - []ValidatorUpdate: 검증자 업데이트 목록
//   - error: 처리 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 각 블록 처리 종료 시 호출되어 검증자 집합의 변경사항을 반환합니다.
// 블록 높이에 따라 에포크 전환 여부를 확인하고, 필요 시 검증자 집합을 재구성합니다.
// 검증자 순위를 재계산하고 활성 검증자 목록을 업데이트합니다.
// 변경된 검증자 정보를 ValidatorUpdate 형태로 반환하여 합의 엔진에 전달합니다.
func (a *StakingAdapter) EndBlock(height uint64) ([]ValidatorUpdate, error) {
	a.logger.Debug("End block", "height", height)

	// 검증자 업데이트 목록 생성
	validators := a.GetValidators()

	updates := make([]ValidatorUpdate, 0, len(validators))
	for _, validator := range validators {
		if validator.Status == ValidatorStatusBonded {
			updates = append(updates, ValidatorUpdate{
				Address:     validator.Address,
				VotingPower: validator.VotingPower,
			})
		}
	}

	return updates, nil
}

// GetRewards는 위임자의 모든 보상을 조회합니다.
func (a *StakingAdapter) GetRewards(delegator common.Address) (*big.Int, error) {
	a.logger.Info("Getting rewards", "delegator", delegator.Hex())

	// 모든 위임 조회
	delegations, err := a.GetDelegations(delegator)
	if err != nil {
		return nil, err
	}

	// 모든 보상 합산
	totalRewards := big.NewInt(0)
	for _, delegation := range delegations {
		totalRewards = new(big.Int).Add(totalRewards, delegation.AccumulatedRewards)
	}

	return totalRewards, nil
}

// WithdrawRewards는 위임자의 보상을 인출합니다.
func (a *StakingAdapter) WithdrawRewards(delegator common.Address, validator common.Address) (*big.Int, error) {
	a.logger.Info("Withdrawing rewards", "delegator", delegator.Hex(), "validator", validator.Hex())

	// 검증자 확인
	validatorInfo, err := a.GetValidator(validator)
	if err != nil {
		return nil, err
	}

	// 위임 찾기
	for _, delegation := range validatorInfo.Delegations {
		if delegation.Delegator == delegator {
			// 보상 금액 가져오기
			rewards := delegation.AccumulatedRewards

			// 보상 초기화
			delegation.AccumulatedRewards = big.NewInt(0)

			// 검증자 업데이트
			err = a.UpdateValidator(validatorInfo)
			if err != nil {
				return nil, err
			}

			a.logger.Info("Rewards withdrawn", "delegator", delegator.Hex(), "validator", validator.Hex(), "amount", rewards.String())
			return rewards, nil
		}
	}

	return nil, utils.WrapError(utils.ErrDelegationNotFound, 
		fmt.Sprintf("delegation not found: delegator=%s, validator=%s", delegator.Hex(), validator.Hex()))
}

// GetValidators는 모든 검증자 목록을 반환합니다.
func (a *StakingAdapter) GetValidators() []*Validator {
	return a.BaseStakingAdapter.GetValidators()
}

// GetState는 상태 DB에서 스테이킹 상태를 로드합니다.
func (a *StakingAdapter) GetState(stateDB *state.StateDB) error {
	// 상태 DB에서 스테이킹 상태 로드 구현
	// 실제 구현에서는 stateDB에서 검증자 정보를 로드해야 함
	return nil
}

// SaveState는 스테이킹 상태를 상태 DB에 저장합니다.
func (a *StakingAdapter) SaveState(stateDB *state.StateDB) error {
	// 스테이킹 상태를 상태 DB에 저장 구현
	// 실제 구현에서는 검증자 정보를 stateDB에 저장해야 함
	return nil
}

// GetValidatorsFromState는 상태 DB에서 검증자 목록을 가져옵니다.
func (a *StakingAdapter) GetValidatorsFromState(state *state.StateDB) ([]*Validator, error) {
	// 상태 DB에서 검증자 목록 로드 구현
	// 실제 구현에서는 state에서 검증자 정보를 로드해야 함
	return nil, nil
}

// CreateValidator는 새로운 검증자를 생성합니다.
func (a *StakingAdapter) CreateValidator(operator common.Address, pubKey []byte, amount *big.Int) error {
	a.logger.Info("Creating validator", "address", operator.Hex(), "amount", amount.String())

	// 검증자 생성 로직 구현
	// 실제 구현에서는 검증자 생성 및 등록 로직이 필요함
	
	// 기본 설명 정보 생성
	description := ValidatorDescription{
		Moniker:         "Validator " + operator.Hex()[:8],
		Identity:        "",
		Website:         "",
		SecurityContact: "",
		Details:         "Created by StakingAdapter",
	}
	
	// 기본 커미션 설정 (10%)
	commission := big.NewInt(10)
	
	// 검증자 객체 생성
	validator := a.CreateValidatorObject(operator, pubKey, amount, description, commission)
	
	// 검증자 추가
	err := a.AddValidator(validator)
	if err != nil {
		return utils.WrapError(err, fmt.Sprintf("failed to add validator: %s", operator.Hex()))
	}
	
	a.logger.Info("Validator created", "address", operator.Hex(), "voting_power", amount.String())
	return nil
}

// EditValidator는 검증자 정보를 수정합니다.
func (a *StakingAdapter) EditValidator(operator common.Address, description string, commission *big.Int) error {
	a.logger.Info("Editing validator", "address", operator.Hex())

	// 검증자 확인
	validator, err := a.GetValidator(operator)
	if err != nil {
		return err
	}
	
	// 설명 정보 업데이트
	validator.Description.Details = description
	
	// 커미션 업데이트 (제공된 경우)
	if commission != nil {
		validator.Commission = new(big.Int).Set(commission)
	}
	
	// 검증자 업데이트
	err = a.UpdateValidator(validator)
	if err != nil {
		return utils.WrapError(err, fmt.Sprintf("failed to update validator: %s", operator.Hex()))
	}
	
	a.logger.Info("Validator edited", "address", operator.Hex())
	return nil
}
