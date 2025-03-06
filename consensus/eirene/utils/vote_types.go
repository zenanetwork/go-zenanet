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

package utils

import (
	"math/big"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
)

// VoteInterface는 모든 투표 타입이 구현해야 하는 공통 인터페이스입니다.
type VoteInterface interface {
	GetProposalID() uint64
	GetVoter() common.Address
	GetOption() string
	GetWeight() *big.Int
	GetTimestamp() time.Time
}

// StandardVote는 기본 투표 구현체입니다.
type StandardVote struct {
	ProposalID uint64         `json:"proposal_id"`
	Voter      common.Address `json:"voter"`
	Option     string         `json:"option"`
	Weight     *big.Int       `json:"weight"`
	Timestamp  time.Time      `json:"timestamp"`
}

// GetProposalID는 제안 ID를 반환합니다.
func (v *StandardVote) GetProposalID() uint64 {
	return v.ProposalID
}

// GetVoter는 투표자 주소를 반환합니다.
func (v *StandardVote) GetVoter() common.Address {
	return v.Voter
}

// GetOption은 투표 옵션을 반환합니다.
func (v *StandardVote) GetOption() string {
	return v.Option
}

// GetWeight는 투표 가중치를 반환합니다.
func (v *StandardVote) GetWeight() *big.Int {
	return v.Weight
}

// GetTimestamp는 투표 시간을 반환합니다.
func (v *StandardVote) GetTimestamp() time.Time {
	return v.Timestamp
}

// ConditionalVote는 조건부 투표를 나타냅니다.
type ConditionalVote struct {
	StandardVote          // 기본 투표 정보 임베딩
	Conditions   []string `json:"conditions"` // 조건 목록
}

// VoteArgs는 투표 API 요청 인자를 나타냅니다.
type VoteArgs struct {
	ProposalID uint64         `json:"proposal_id"`
	Voter      common.Address `json:"voter"`
	Option     string         `json:"option"`
}

// NewStandardVote는 새로운 표준 투표를 생성합니다.
func NewStandardVote(proposalID uint64, voter common.Address, option string, weight *big.Int) *StandardVote {
	return &StandardVote{
		ProposalID: proposalID,
		Voter:      voter,
		Option:     option,
		Weight:     weight,
		Timestamp:  time.Now(),
	}
}

// NewConditionalVote는 새로운 조건부 투표를 생성합니다.
func NewConditionalVote(proposalID uint64, voter common.Address, option string, weight *big.Int, conditions []string) *ConditionalVote {
	return &ConditionalVote{
		StandardVote: *NewStandardVote(proposalID, voter, option, weight),
		Conditions:   conditions,
	}
}

// IsValidVoteOption은 주어진 투표 옵션이 유효한지 확인합니다.
func IsValidVoteOption(option string) bool {
	switch option {
	case VoteOptionYes, VoteOptionNo, VoteOptionAbstain, VoteOptionVeto, VoteWithConditions:
		return true
	default:
		return false
	}
}
