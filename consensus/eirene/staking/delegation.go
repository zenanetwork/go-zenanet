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
	"math/big"

	"github.com/zenanetwork/go-zenanet/common"
)

// ValidatorDelegation은 검증자에게 위임된 정보를 나타냅니다.
type ValidatorDelegation struct {
	Delegator          common.Address // 위임자 주소
	Validator          common.Address // 검증자 주소
	Amount             *big.Int       // 위임 양
	AccumulatedRewards *big.Int       // 누적 보상
	StartBlock         uint64         // 위임 시작 블록
	EndBlock           uint64         // 위임 종료 블록 (언본딩 중인 경우)
	Status             DelegationStatus // 위임 상태
}

// DelegationStatus는 위임의 상태를 나타냅니다.
type DelegationStatus int

const (
	DelegationStatusBonded    DelegationStatus = iota // 본딩 상태 (활성화됨)
	DelegationStatusUnbonding                         // 언본딩 중 (활성화 해제 중)
)

// String은 DelegationStatus를 문자열로 변환합니다.
func (s DelegationStatus) String() string {
	switch s {
	case DelegationStatusBonded:
		return "Bonded"
	case DelegationStatusUnbonding:
		return "Unbonding"
	default:
		return "Unknown"
	}
}

// NewValidatorDelegation은 새로운 ValidatorDelegation 인스턴스를 생성합니다.
func NewValidatorDelegation(delegator, validator common.Address, amount *big.Int, startBlock uint64) *ValidatorDelegation {
	return &ValidatorDelegation{
		Delegator:          delegator,
		Validator:          validator,
		Amount:             amount,
		AccumulatedRewards: new(big.Int),
		StartBlock:         startBlock,
		EndBlock:           0,
		Status:             DelegationStatusBonded,
	}
}

// Unbond는 위임을 언본딩 상태로 변경합니다.
func (d *ValidatorDelegation) Unbond(endBlock uint64) {
	d.Status = DelegationStatusUnbonding
	d.EndBlock = endBlock
}

// IsActive는 위임이 활성 상태인지 확인합니다.
func (d *ValidatorDelegation) IsActive() bool {
	return d.Status == DelegationStatusBonded
}

// IsUnbonding은 위임이 언본딩 중인지 확인합니다.
func (d *ValidatorDelegation) IsUnbonding() bool {
	return d.Status == DelegationStatusUnbonding
}

// AddReward는 위임에 보상을 추가합니다.
func (d *ValidatorDelegation) AddReward(reward *big.Int) {
	if d.AccumulatedRewards == nil {
		d.AccumulatedRewards = new(big.Int)
	}
	d.AccumulatedRewards = new(big.Int).Add(d.AccumulatedRewards, reward)
}

// ClaimReward는 위임의 보상을 청구합니다.
func (d *ValidatorDelegation) ClaimReward() *big.Int {
	reward := new(big.Int).Set(d.AccumulatedRewards)
	d.AccumulatedRewards = new(big.Int)
	return reward
}

// ToCosmosSDK는 ValidatorDelegation을 Cosmos SDK의 Delegation 형식으로 변환합니다.
func (d *ValidatorDelegation) ToCosmosSDK() interface{} {
	// Cosmos SDK의 Delegation 형식으로 변환하는 로직 구현
	// 실제 구현에서는 Cosmos SDK의 타입을 임포트하여 변환해야 함
	return nil
}

// FromCosmosSDK는 Cosmos SDK의 Delegation을 ValidatorDelegation으로 변환합니다.
func FromCosmosSDK(cosmosDelegation interface{}) *ValidatorDelegation {
	// Cosmos SDK의 Delegation을 ValidatorDelegation으로 변환하는 로직 구현
	// 실제 구현에서는 Cosmos SDK의 타입을 임포트하여 변환해야 함
	return nil
} 