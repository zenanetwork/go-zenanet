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

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/state"
)

// StakingAdapterInterface는 스테이킹 어댑터가 구현해야 하는 인터페이스입니다.
type StakingAdapterInterface interface {
	// 검증자 관련 메서드
	GetValidator(address common.Address) (*Validator, error)
	GetValidators() []*Validator
	CreateValidator(operator common.Address, pubKey []byte, amount *big.Int) error
	EditValidator(operator common.Address, description string, commission *big.Int) error
	
	// 스테이킹 관련 메서드
	Stake(stateDB *state.StateDB, operator common.Address, amount *big.Int, pubKey []byte, description ValidatorDescription, commission *big.Int) error
	Unstake(stateDB *state.StateDB, operator common.Address) error
	
	// 위임 관련 메서드
	Delegate(stateDB *state.StateDB, delegator common.Address, validator common.Address, amount *big.Int) error
	Undelegate(stateDB *state.StateDB, delegator common.Address, validator common.Address, amount *big.Int) error
	Redelegate(stateDB *state.StateDB, delegator common.Address, srcValidator common.Address, dstValidator common.Address, amount *big.Int) error
	
	// 보상 관련 메서드
	GetRewards(delegator common.Address) (*big.Int, error)
	WithdrawRewards(delegator common.Address, validator common.Address) (*big.Int, error)
	
	// 블록 처리 관련 메서드
	BeginBlock(height uint64, time uint64) error
	EndBlock(height uint64) ([]ValidatorUpdate, error)
	
	// 상태 관리 메서드
	GetValidatorSet() *ValidatorSet
	SetValidatorSet(validatorSet *ValidatorSet)
	GetState(stateDB *state.StateDB) error
	SaveState(stateDB *state.StateDB) error
}

// ValidatorUpdate는 검증자 업데이트 정보를 나타냅니다.
type ValidatorUpdate struct {
	Address     common.Address
	VotingPower *big.Int
} 