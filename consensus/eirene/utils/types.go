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

// Package utils는 Eirene 합의 알고리즘에서 사용하는 공통 유틸리티와 타입을 제공합니다.
// 이 패키지는 다른 패키지 간의 순환 참조를 방지하기 위해 공통 타입과 인터페이스를 정의합니다.
package utils

import (
	"math/big"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/state"
)

// 검증자 상태 상수
const (
	ValidatorStatusUnbonded  uint8 = 0 // 언본딩 상태
	ValidatorStatusUnbonding uint8 = 1 // 언본딩 중
	ValidatorStatusBonded    uint8 = 2 // 본딩 상태
	ValidatorStatusJailed    uint8 = 3 // 감금 상태
)

// 제안 유형 상수
const (
	ProposalTypeParameterChange = "parameter_change" // 매개변수 변경
	ProposalTypeParameter       = "parameter"        // 매개변수
	ProposalTypeUpgrade         = "upgrade"          // 업그레이드
	ProposalTypeFunding         = "funding"          // 자금 지원
	ProposalTypeText            = "text"             // 텍스트
)

// 제안 상태 상수
const (
	ProposalStatusDepositPeriod = "deposit_period" // 보증금 기간
	ProposalStatusVotingPeriod  = "voting_period"  // 투표 기간
	ProposalStatusPending       = "pending"        // 대기 중
	ProposalStatusPassed        = "passed"         // 통과됨
	ProposalStatusRejected      = "rejected"       // 거부됨
	ProposalStatusExecuted      = "executed"       // 실행됨
	ProposalStatusFailed        = "failed"         // 실패
)

// 투표 옵션 상수
const (
	VoteOptionYes      = "yes"             // 찬성
	VoteOptionNo       = "no"              // 반대
	VoteOptionAbstain  = "abstain"         // 기권
	VoteOptionVeto     = "veto"            // 거부권
	VoteWithConditions = "with_conditions" // 조건부 찬성
)

// 투표 가중치 유형 상수
const (
	VoteWeightTypeEqual     = 0 // 동등한 가중치
	VoteWeightTypeStake     = 1 // 스테이킹 기반 가중치
	VoteWeightTypeQuadratic = 2 // 이차 투표 가중치
)

// 기본 거버넌스 매개변수 상수
const (
	DefaultMinDeposit            = 100 * 1e18 // 100 토큰
	DefaultMinDepositEmergency   = 1000       // 긴급 제안 최소 보증금 (1000 토큰)
	DefaultDepositPeriod         = 86400 * 2  // 2일 (초 단위)
	DefaultVotingPeriod          = 86400 * 7  // 7일 (초 단위)
	DefaultEmergencyVotingPeriod = 86400      // 1일 (초 단위)
	DefaultQuorum                = 0.334      // 33.4%
	DefaultEmergencyQuorum       = 50         // 50% 쿼럼
	DefaultThreshold             = 0.5        // 50%
	DefaultEmergencyThreshold    = 67         // 67% 찬성 임계값
	DefaultVetoThreshold         = 0.334      // 33.4%
	DefaultExecutionDelay        = 86400 * 2  // 2일 (초 단위)
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
	GetValidatorsAtHeight(height int64) ([]ValidatorInterface, error)
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

// StandardProposal은 표준 제안 구조체입니다.
// 이 구조체는 모든 유형의 제안에 공통적으로 사용되는 필드를 포함합니다.
type StandardProposal struct {
	ID               uint64                      // 제안 ID
	Type             string                      // 제안 유형
	Title            string                      // 제안 제목
	Description      string                      // 제안 설명
	Proposer         common.Address              // 제안자 주소
	SubmitTime       time.Time                   // 제출 시간
	SubmitBlock      uint64                      // 제출 블록
	DepositEnd       time.Time                   // 보증금 기간 종료 시간
	VotingStart      time.Time                   // 투표 시작 시간
	VotingEnd        time.Time                   // 투표 종료 시간
	ExecuteTime      time.Time                   // 실행 시간
	VotingStartBlock uint64                      // 투표 시작 블록
	VotingEndBlock   uint64                      // 투표 종료 블록
	Status           string                      // 제안 상태
	TotalDeposit     *big.Int                    // 총 보증금
	Deposits         map[common.Address]*big.Int // 보증금 맵 (주소 -> 금액)
	YesVotes         *big.Int                    // 찬성 투표 수
	NoVotes          *big.Int                    // 반대 투표 수
	AbstainVotes     *big.Int                    // 기권 투표 수
	VetoVotes        *big.Int                    // 거부권 투표 수
	Votes            map[common.Address]string   // 투표 맵 (주소 -> 옵션)
	Content          ProposalContentInterface    `rlp:"-"` // 제안 내용 (직렬화 제외)
}

// GetID는 제안 ID를 반환합니다.
func (p *StandardProposal) GetID() uint64 {
	return p.ID
}

// GetType은 제안 유형을 반환합니다.
func (p *StandardProposal) GetType() string {
	return p.Type
}

// GetTitle은 제안 제목을 반환합니다.
func (p *StandardProposal) GetTitle() string {
	return p.Title
}

// GetDescription은 제안 설명을 반환합니다.
func (p *StandardProposal) GetDescription() string {
	return p.Description
}

// GetProposer는 제안자 주소를 반환합니다.
func (p *StandardProposal) GetProposer() common.Address {
	return p.Proposer
}

// GetStatus는 제안 상태를 반환합니다.
func (p *StandardProposal) GetStatus() string {
	return p.Status
}

// GetSubmitTime은 제안 제출 시간을 반환합니다.
func (p *StandardProposal) GetSubmitTime() time.Time {
	return p.SubmitTime
}

// GetVotingStartBlock은 투표 시작 블록을 반환합니다.
func (p *StandardProposal) GetVotingStartBlock() uint64 {
	return p.VotingStartBlock
}

// GetVotingEndBlock은 투표 종료 블록을 반환합니다.
func (p *StandardProposal) GetVotingEndBlock() uint64 {
	return p.VotingEndBlock
}

// GetTotalDeposit는 총 보증금을 반환합니다.
func (p *StandardProposal) GetTotalDeposit() *big.Int {
	return p.TotalDeposit
}

// ProposalContentInterface는 제안 내용 관련 기능을 제공하는 인터페이스입니다.
type ProposalContentInterface interface {
	GetType() string
	Validate() error
	GetParams() map[string]string
	Execute(state *state.StateDB) error
}

// GovernanceInterface는 거버넌스 관련 기능을 제공하는 인터페이스입니다.
// 이 인터페이스를 통해 다른 패키지에서 거버넌스 기능에 접근할 수 있습니다.
type GovernanceInterface interface {
	SubmitProposal(proposer common.Address, title string, description string, proposalType string, content ProposalContentInterface, initialDeposit *big.Int, state *state.StateDB) (uint64, error)
	Vote(proposalID uint64, voter common.Address, option string, state *state.StateDB) error
	GetProposal(proposalID uint64) (ProposalInterface, error)
	GetProposals() []ProposalInterface
	ExecuteProposal(proposalID uint64, state *state.StateDB) error
}

// UpgradeInfo는 업그레이드 정보를 나타냅니다.
type UpgradeInfo struct {
	Name                string       // 업그레이드 이름
	Height              uint64       // 업그레이드 높이
	Info                string       // 업그레이드 정보
	UpgradeTime         time.Time    // 업그레이드 시간
	CancelUpgradeHeight uint64       // 업그레이드 취소 높이
	Version             string       // 버전
	URL                 string       // 업그레이드 URL
	Hash                []byte       // 업그레이드 해시
	UpgradeHandler      func() error `rlp:"-"` // 업그레이드 핸들러 (직렬화 제외)
}

// FundingInfo는 자금 지원 정보를 나타냅니다.
type FundingInfo struct {
	Recipient common.Address // 수령인 주소
	Amount    *big.Int       // 금액
	Reason    string         // 이유
	Purpose   string         // 목적 (추가 설명)
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
	return v.Status == ValidatorStatusBonded
}
