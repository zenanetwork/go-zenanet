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

// Package staking implements the staking module for the Eirene consensus algorithm.
package staking

import (
	"fmt"
	"math/big"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/cosmos"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/params"
)

// CosmosStakingAdapter는 Cosmos SDK의 staking 모듈과 go-zenanet을 연결하는 어댑터입니다.
// StakingAdapterInterface를 구현합니다.
type CosmosStakingAdapter struct {
	*BaseStakingAdapter
	logger         log.Logger
	stakingAdapter *StakingAdapter // 기존 스테이킹 어댑터
	cosmosAdapter  *cosmos.CosmosAdapter // Cosmos SDK 어댑터
}

// 컴파일 타임에 인터페이스 구현 여부 확인
var _ StakingAdapterInterface = (*CosmosStakingAdapter)(nil)

// NewCosmosStakingAdapter는 새로운 CosmosStakingAdapter 인스턴스를 생성합니다.
func NewCosmosStakingAdapter(validatorSet *ValidatorSet, stakingAdapter *StakingAdapter, cosmosAdapter *cosmos.CosmosAdapter, config *params.EireneConfig) *CosmosStakingAdapter {
	baseAdapter := NewBaseStakingAdapter(validatorSet, config)
	
	return &CosmosStakingAdapter{
		BaseStakingAdapter: baseAdapter,
		logger:             log.New("module", "cosmos-staking"),
		stakingAdapter:     stakingAdapter,
		cosmosAdapter:      cosmosAdapter,
	}
}

// CreateValidator는 새로운 검증자를 생성합니다.
func (a *CosmosStakingAdapter) CreateValidator(operator common.Address, pubKey []byte, amount *big.Int) error {
	a.logger.Info("Creating validator", "address", operator.Hex(), "amount", amount.String())

	// 기존 StakingAdapter의 CreateValidator 함수 호출
	err := a.stakingAdapter.CreateValidator(operator, pubKey, amount)
	if err != nil {
		return utils.WrapError(err, fmt.Sprintf("failed to create validator via StakingAdapter: %s", operator.Hex()))
	}

	// Cosmos SDK의 staking 모듈과 연동하는 로직 구현
	storeAdapter := a.cosmosAdapter.GetStoreAdapter()
	if storeAdapter == nil {
		a.logger.Error("Store adapter is nil")
		return nil
	}

	// 검증자 주소를 Cosmos SDK 형식으로 변환
	valAddr := sdk.ValAddress(operator.Bytes()).String()
	
	// 스테이킹 금액을 Cosmos SDK 형식으로 변환
	stakeAmount := sdk.NewCoin("zena", math.NewIntFromBigInt(amount))
	
	// 최소 자기 위임량 설정
	minSelfDelegation := math.NewIntFromBigInt(big.NewInt(1))
	
	// 커미션 비율 설정 (0-100%)
	commissionRate := math.LegacyNewDecFromBigInt(big.NewInt(10)) // 10%
	
	// 검증자 생성 메시지 생성
	// 실제 구현에서는 적절한 PubKey 타입으로 변환 필요
	// 임시 구현: 실제 코드에서는 적절한 타입으로 변환해야 함
	a.logger.Info("Preparing validator creation message", 
		"validator", valAddr,
		"commission", commissionRate.String(),
		"amount", stakeAmount.String(),
		"min_self_delegation", minSelfDelegation.String())
	
	return nil
}

// EditValidator는 검증자 정보를 수정합니다.
func (a *CosmosStakingAdapter) EditValidator(operator common.Address, description string, commission *big.Int) error {
	a.logger.Info("Editing validator", "address", operator.Hex())

	// 기존 StakingAdapter의 EditValidator 함수 호출
	err := a.stakingAdapter.EditValidator(operator, description, commission)
	if err != nil {
		return utils.WrapError(err, fmt.Sprintf("failed to edit validator via StakingAdapter: %s", operator.Hex()))
	}

	// Cosmos SDK의 staking 모듈과 연동하는 로직 구현
	storeAdapter := a.cosmosAdapter.GetStoreAdapter()
	if storeAdapter == nil {
		a.logger.Error("Store adapter is nil")
		return nil
	}

	// 검증자 주소를 Cosmos SDK 형식으로 변환
	valAddr := sdk.ValAddress(operator.Bytes()).String()
	
	// 커미션 비율 설정 (0-100%)
	var commissionRate math.LegacyDec
	if commission != nil {
		commissionRate = math.LegacyNewDecFromBigIntWithPrec(commission, 2) // 2자리 소수점 정밀도
	}
	
	// 검증자 수정 메시지 생성
	// 실제 구현에서는 적절한 메시지 타입으로 변환 필요
	// 임시 구현: 실제 코드에서는 적절한 타입으로 변환해야 함
	a.logger.Info("Preparing validator edit message", 
		"validator", valAddr,
		"description", description,
		"commission", commissionRate.String())
	
	return nil
}

// Stake는 토큰을 스테이킹하고 검증자가 됩니다.
func (a *CosmosStakingAdapter) Stake(stateDB *state.StateDB, operator common.Address, amount *big.Int, pubKey []byte, description ValidatorDescription, commission *big.Int) error {
	a.logger.Info("Staking tokens via Cosmos adapter", "address", operator.Hex(), "amount", amount.String())

	// 기존 StakingAdapter의 Stake 함수 호출
	err := a.stakingAdapter.Stake(stateDB, operator, amount, pubKey, description, commission)
	if err != nil {
		return utils.WrapError(err, fmt.Sprintf("failed to stake via StakingAdapter: %s", operator.Hex()))
	}

	// Cosmos SDK의 staking 모듈과 연동하는 로직 구현
	// 실제 구현에서는 Cosmos SDK의 MsgCreateValidator 메시지를 생성하고 처리해야 함
	
	return nil
}

// Unstake는 스테이킹된 토큰을 철회하고 검증자 상태를 종료합니다.
func (a *CosmosStakingAdapter) Unstake(stateDB *state.StateDB, operator common.Address) error {
	a.logger.Info("Unstaking tokens via Cosmos adapter", "address", operator.Hex())

	// 기존 StakingAdapter의 Unstake 함수 호출
	err := a.stakingAdapter.Unstake(stateDB, operator)
	if err != nil {
		return utils.WrapError(err, fmt.Sprintf("failed to unstake via StakingAdapter: %s", operator.Hex()))
	}

	// Cosmos SDK의 staking 모듈과 연동하는 로직 구현
	// 실제 구현에서는 Cosmos SDK의 MsgUnbond 메시지를 생성하고 처리해야 함
	
	return nil
}

// Delegate는 토큰을 검증자에게 위임합니다.
func (a *CosmosStakingAdapter) Delegate(stateDB *state.StateDB, delegator common.Address, validator common.Address, amount *big.Int) error {
	a.logger.Info("Delegating tokens via Cosmos adapter", "delegator", delegator.Hex(), "validator", validator.Hex(), "amount", amount.String())

	// 기존 StakingAdapter의 Delegate 함수 호출
	err := a.stakingAdapter.Delegate(stateDB, delegator, validator, amount)
	if err != nil {
		return utils.WrapError(err, fmt.Sprintf("failed to delegate via StakingAdapter: delegator=%s, validator=%s", delegator.Hex(), validator.Hex()))
	}

	// Cosmos SDK의 staking 모듈과 연동하는 로직 구현
	// 실제 구현에서는 Cosmos SDK의 MsgDelegate 메시지를 생성하고 처리해야 함
	
	return nil
}

// Undelegate는 위임된 토큰을 철회합니다.
func (a *CosmosStakingAdapter) Undelegate(stateDB *state.StateDB, delegator common.Address, validator common.Address, amount *big.Int) error {
	a.logger.Info("Undelegating tokens via Cosmos adapter", "delegator", delegator.Hex(), "validator", validator.Hex(), "amount", amount.String())

	// 기존 StakingAdapter의 Undelegate 함수 호출
	err := a.stakingAdapter.Undelegate(stateDB, delegator, validator, amount)
	if err != nil {
		return utils.WrapError(err, fmt.Sprintf("failed to undelegate via StakingAdapter: delegator=%s, validator=%s", delegator.Hex(), validator.Hex()))
	}

	// Cosmos SDK의 staking 모듈과 연동하는 로직 구현
	// 실제 구현에서는 Cosmos SDK의 MsgUndelegate 메시지를 생성하고 처리해야 함
	
	return nil
}

// Redelegate는 위임을 한 검증자에서 다른 검증자로 이동합니다.
func (a *CosmosStakingAdapter) Redelegate(stateDB *state.StateDB, delegator common.Address, srcValidator common.Address, dstValidator common.Address, amount *big.Int) error {
	a.logger.Info("Redelegating tokens via Cosmos adapter", 
		"delegator", delegator.Hex(), 
		"src_validator", srcValidator.Hex(), 
		"dst_validator", dstValidator.Hex(), 
		"amount", amount.String())

	// 기존 StakingAdapter의 Redelegate 함수 호출
	err := a.stakingAdapter.Redelegate(stateDB, delegator, srcValidator, dstValidator, amount)
	if err != nil {
		return utils.WrapError(err, fmt.Sprintf("failed to redelegate via StakingAdapter: delegator=%s, src=%s, dst=%s", 
			delegator.Hex(), srcValidator.Hex(), dstValidator.Hex()))
	}

	// Cosmos SDK의 staking 모듈과 연동하는 로직 구현
	// 실제 구현에서는 Cosmos SDK의 MsgBeginRedelegate 메시지를 생성하고 처리해야 함
	
	return nil
}

// GetRewards는 위임자의 모든 보상을 조회합니다.
func (a *CosmosStakingAdapter) GetRewards(delegator common.Address) (*big.Int, error) {
	a.logger.Info("Getting rewards via Cosmos adapter", "delegator", delegator.Hex())

	// 기존 StakingAdapter의 GetRewards 함수 호출
	rewards, err := a.stakingAdapter.GetRewards(delegator)
	if err != nil {
		return nil, utils.WrapError(err, fmt.Sprintf("failed to get rewards via StakingAdapter: %s", delegator.Hex()))
	}

	// Cosmos SDK의 distribution 모듈과 연동하는 로직 구현
	// 실제 구현에서는 Cosmos SDK의 distribution 모듈에서 보상 정보를 가져와야 함
	
	return rewards, nil
}

// WithdrawRewards는 위임자의 보상을 인출합니다.
func (a *CosmosStakingAdapter) WithdrawRewards(delegator common.Address, validator common.Address) (*big.Int, error) {
	a.logger.Info("Withdrawing rewards via Cosmos adapter", "delegator", delegator.Hex(), "validator", validator.Hex())

	// 기존 StakingAdapter의 WithdrawRewards 함수 호출
	rewards, err := a.stakingAdapter.WithdrawRewards(delegator, validator)
	if err != nil {
		return nil, utils.WrapError(err, fmt.Sprintf("failed to withdraw rewards via StakingAdapter: delegator=%s, validator=%s", delegator.Hex(), validator.Hex()))
	}

	// Cosmos SDK의 distribution 모듈과 연동하는 로직 구현
	// 실제 구현에서는 Cosmos SDK의 MsgWithdrawDelegatorReward 메시지를 생성하고 처리해야 함
	
	return rewards, nil
}

// BeginBlock은 블록 시작 시 호출됩니다.
func (a *CosmosStakingAdapter) BeginBlock(height uint64, time uint64) error {
	a.logger.Debug("Begin block via Cosmos adapter", "height", height, "time", time)

	// 기존 StakingAdapter의 BeginBlock 함수 호출
	err := a.stakingAdapter.BeginBlock(height, time)
	if err != nil {
		return utils.WrapError(err, fmt.Sprintf("failed to begin block via StakingAdapter: height=%d", height))
	}

	// Cosmos SDK의 staking 모듈과 연동하는 로직 구현
	// 실제 구현에서는 Cosmos SDK의 BeginBlock 핸들러를 호출해야 함
	
	return nil
}

// BeginBlockWithABCI는 ABCI 요청으로 블록 시작 시 호출됩니다.
func (a *CosmosStakingAdapter) BeginBlockWithABCI(ctx sdk.Context, req abci.RequestBeginBlock, stateDB *state.StateDB) error {
	a.logger.Debug("Begin block with ABCI", "height", req.Header.Height)

	// Cosmos SDK의 staking 모듈의 BeginBlock 핸들러 호출
	// 실제 구현에서는 Cosmos SDK의 staking 모듈의 BeginBlock 핸들러를 호출해야 함
	
	return nil
}

// EndBlock은 블록 종료 시 호출됩니다.
func (a *CosmosStakingAdapter) EndBlock(height uint64) ([]ValidatorUpdate, error) {
	a.logger.Debug("End block via Cosmos adapter", "height", height)

	// 기존 StakingAdapter의 EndBlock 함수 호출
	updates, err := a.stakingAdapter.EndBlock(height)
	if err != nil {
		return nil, utils.WrapError(err, fmt.Sprintf("failed to end block via StakingAdapter: height=%d", height))
	}

	// Cosmos SDK의 staking 모듈과 연동하는 로직 구현
	// 실제 구현에서는 Cosmos SDK의 EndBlock 핸들러를 호출하고 검증자 업데이트를 가져와야 함
	
	// Cosmos SDK의 검증자 업데이트를 go-zenanet 형식으로 변환
	// 실제 구현에서는 Cosmos SDK의 검증자 업데이트를 go-zenanet 형식으로 변환해야 함
	
	return updates, nil
}

// EndBlockWithABCI는 ABCI 요청으로 블록 종료 시 호출됩니다.
func (a *CosmosStakingAdapter) EndBlockWithABCI(ctx sdk.Context, req interface{}, stateDB *state.StateDB) ([]ValidatorUpdate, error) {
	a.logger.Debug("End block with ABCI")

	// Cosmos SDK의 staking 모듈의 EndBlock 핸들러 호출
	// 실제 구현에서는 Cosmos SDK의 staking 모듈의 EndBlock 핸들러를 호출해야 함
	
	// Cosmos SDK의 검증자 업데이트를 go-zenanet 형식으로 변환
	// 실제 구현에서는 Cosmos SDK의 검증자 업데이트를 go-zenanet 형식으로 변환해야 함
	
	return nil, nil
}

// GetValidator는 주소로 검증자를 조회합니다.
func (a *CosmosStakingAdapter) GetValidator(address common.Address) (*Validator, error) {
	return a.BaseStakingAdapter.GetValidator(address)
}

// GetValidators는 모든 검증자 목록을 반환합니다.
func (a *CosmosStakingAdapter) GetValidators() []*Validator {
	return a.stakingAdapter.GetValidators()
}

// GetAllValidators는 모든 검증자를 반환합니다.
func (a *CosmosStakingAdapter) GetAllValidators(stateDB *state.StateDB) ([]*Validator, error) {
	return a.GetValidators(), nil
}

// GetValidatorSet은 검증자 집합을 반환합니다.
func (a *CosmosStakingAdapter) GetValidatorSet() *ValidatorSet {
	return a.BaseStakingAdapter.GetValidatorSet()
}

// SetValidatorSet은 검증자 집합을 설정합니다.
func (a *CosmosStakingAdapter) SetValidatorSet(validatorSet *ValidatorSet) {
	a.BaseStakingAdapter.SetValidatorSet(validatorSet)
}

// GetState는 상태 DB에서 스테이킹 상태를 로드합니다.
func (a *CosmosStakingAdapter) GetState(stateDB *state.StateDB) error {
	return a.stakingAdapter.GetState(stateDB)
}

// SaveState는 스테이킹 상태를 상태 DB에 저장합니다.
func (a *CosmosStakingAdapter) SaveState(stateDB *state.StateDB) error {
	return a.stakingAdapter.SaveState(stateDB)
}

// InitGenesis는 제네시스 상태를 초기화합니다.
func (a *CosmosStakingAdapter) InitGenesis(ctx sdk.Context, genesisState *state.StateDB) error {
	a.logger.Info("Initializing genesis state")

	// Cosmos SDK의 staking 모듈의 InitGenesis 핸들러 호출
	// 실제 구현에서는 Cosmos SDK의 staking 모듈의 InitGenesis 핸들러를 호출해야 함
	
	// 검증자 초기화
	validators, err := a.stakingAdapter.GetValidatorsFromState(genesisState)
	if err != nil {
		return utils.WrapError(err, "failed to get validators from state")
	}
	
	// 검증자 집합 초기화
	for _, validator := range validators {
		err = a.AddValidator(validator)
		if err != nil {
			return utils.WrapError(err, fmt.Sprintf("failed to add validator: %s", validator.Address.Hex()))
		}
	}
	
	return nil
}

// UpdateValidator는 검증자 정보를 업데이트합니다.
func (a *CosmosStakingAdapter) UpdateValidator(validator *Validator, stateDB *state.StateDB) error {
	err := a.BaseStakingAdapter.UpdateValidator(validator)
	if err != nil {
		return err
	}
	
	// 상태 DB에 저장
	err = a.SaveState(stateDB)
	if err != nil {
		return err
	}
	
	return nil
}

// RemoveValidator는 검증자를 제거합니다.
func (a *CosmosStakingAdapter) RemoveValidator(address common.Address, stateDB *state.StateDB) error {
	err := a.BaseStakingAdapter.RemoveValidator(address)
	if err != nil {
		return err
	}
	
	// 상태 DB에 저장
	err = a.SaveState(stateDB)
	if err != nil {
		return err
	}
	
	return nil
} 