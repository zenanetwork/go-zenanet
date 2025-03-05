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

// Package utils는 Eirene 합의 알고리즘의 여러 모듈에서 공통으로 사용하는 유틸리티 함수와 타입을 제공합니다.
package utils

import (
	"math/big"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/state"
)

// 검증자 상태 상수
const (
	ValidatorStatusActive    = 0 // 활성 상태
	ValidatorStatusJailed    = 1 // 감금 상태
	ValidatorStatusUnbonding = 2 // 언본딩 상태
)

// 제안 유형 상수
const (
	ProposalTypeParameterChange = "parameter_change" // 매개변수 변경 제안
	ProposalTypeParameter       = "parameter"        // 매개변수 제안
	ProposalTypeUpgrade         = "upgrade"          // 업그레이드 제안
	ProposalTypeFunding         = "funding"          // 자금 지원 제안
	ProposalTypeText            = "text"             // 텍스트 제안
)

// 제안 상태 상수
const (
	ProposalStatusDepositPeriod = "deposit_period" // 보증금 기간
	ProposalStatusVotingPeriod  = "voting_period"  // 투표 기간
	ProposalStatusPending       = "pending"        // 대기 중
	ProposalStatusPassed        = "passed"         // 통과됨
	ProposalStatusRejected      = "rejected"       // 거부됨
	ProposalStatusExecuted      = "executed"       // 실행됨
)

// 투표 옵션 상수
const (
	VoteOptionYes     = "yes"     // 찬성
	VoteOptionNo      = "no"      // 반대
	VoteOptionAbstain = "abstain" // 기권
	VoteOptionVeto    = "veto"    // 거부권
)

// 기본 거버넌스 매개변수 상수
const (
	DefaultMinDeposit        = 100 * 1e18 // 100 토큰
	DefaultDepositPeriod     = 86400 * 2  // 2일 (초 단위)
	DefaultVotingPeriod      = 86400 * 7  // 7일 (초 단위)
	DefaultQuorum            = 0.334      // 33.4%
	DefaultThreshold         = 0.5        // 50%
	DefaultVetoThreshold     = 0.334      // 33.4%
	DefaultExecutionDelay    = 86400 * 2  // 2일 (초 단위)
)

// ValidatorInterface는 검증자 관련 기능을 제공하는 인터페이스입니다.
// 이 인터페이스를 통해 다른 패키지에서 검증자 정보에 접근할 수 있습니다.
type ValidatorInterface interface {
	GetAddress() common.Address
	GetVotingPower() *big.Int
	GetStatus() uint8
	IsActive() bool
}

// ValidatorSetInterface는 검증자 집합 관련 기능을 제공하는 인터페이스입니다.
// 이 인터페이스를 통해 다른 패키지에서 검증자 집합 정보에 접근할 수 있습니다.
type ValidatorSetInterface interface {
	GetValidatorCount() int
	GetActiveValidatorCount() int
	GetTotalStake() *big.Int
	GetValidatorByAddress(address common.Address) ValidatorInterface
	GetActiveValidators() []ValidatorInterface
	Contains(address common.Address) bool
}

// ProposalInterface는 제안 관련 기능을 제공하는 인터페이스입니다.
// 이 인터페이스를 통해 다른 패키지에서 제안 정보에 접근할 수 있습니다.
type ProposalInterface interface {
	GetID() uint64
	GetType() string
	GetTitle() string
	GetDescription() string
	GetProposer() common.Address
	GetStatus() string
	GetVotingStartBlock() uint64
	GetVotingEndBlock() uint64
}

// ProposalContentInterface는 제안 내용 관련 기능을 제공하는 인터페이스입니다.
type ProposalContentInterface interface {
	GetType() string
	Execute(state *state.StateDB) error
}

// GovernanceInterface는 거버넌스 관련 기능을 제공하는 인터페이스입니다.
type GovernanceInterface interface {
	SubmitProposal(proposer common.Address, title string, description string, proposalType string, content ProposalContentInterface, initialDeposit *big.Int, state *state.StateDB) (uint64, error)
	Vote(proposalID uint64, voter common.Address, option string) error
	GetProposal(proposalID uint64) (ProposalInterface, error)
	GetProposals() []ProposalInterface
	ExecuteProposal(proposalID uint64, state *state.StateDB) error
}

// UpgradeInfo는 업그레이드 정보를 나타냅니다.
type UpgradeInfo struct {
	Name            string    // 업그레이드 이름
	Height          uint64    // 업그레이드 높이
	Info            string    // 업그레이드 정보
	UpgradeTime     time.Time // 업그레이드 시간
	CancelUpgradeHeight uint64    // 업그레이드 취소 높이
}

// FundingInfo는 자금 지원 정보를 나타냅니다.
type FundingInfo struct {
	Recipient common.Address // 수령인 주소
	Amount    *big.Int       // 금액
	Reason    string         // 이유
}

// UpgradeEvent는 업그레이드 이벤트를 나타냅니다.
type UpgradeEvent struct {
	Name        string    // 업그레이드 이름
	Height      uint64    // 업그레이드 높이
	Info        string    // 업그레이드 정보
	UpgradeTime time.Time // 업그레이드 시간
}

// BasicValidator는 기본적인 검증자 정보를 담는 구조체입니다.
// 이 구조체는 다른 패키지에서 검증자 정보를 교환할 때 사용됩니다.
type BasicValidator struct {
	Address     common.Address `json:"address"`     // 검증자 주소
	VotingPower *big.Int       `json:"votingPower"` // 투표 파워 (스테이킹 양)
	Status      uint8          `json:"status"`      // 검증자 상태
}

// GetAddress는 검증자의 주소를 반환합니다.
func (v *BasicValidator) GetAddress() common.Address {
	return v.Address
}

// GetVotingPower는 검증자의 투표 파워를 반환합니다.
func (v *BasicValidator) GetVotingPower() *big.Int {
	return v.VotingPower
}

// GetStatus는 검증자의 상태를 반환합니다.
func (v *BasicValidator) GetStatus() uint8 {
	return v.Status
}

// IsActive는 검증자가 활성 상태인지 여부를 반환합니다.
func (v *BasicValidator) IsActive() bool {
	return v.Status == ValidatorStatusActive
} 