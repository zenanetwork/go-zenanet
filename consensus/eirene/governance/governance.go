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

package governance

import (
	"errors"
	"math/big"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/rlp"
)

// 거버넌스 관련 상수
const (
	// 제안 상태
	ProposalStatusPending   = 0 // 대기 중
	ProposalStatusActive    = 1 // 활성 상태
	ProposalStatusPassed    = 2 // 통과됨
	ProposalStatusRejected  = 3 // 거부됨
	ProposalStatusExecuted  = 4 // 실행됨
	ProposalStatusCancelled = 5 // 취소됨

	// 제안 유형
	ProposalTypeParameter = 0 // 매개변수 변경
	ProposalTypeUpgrade   = 1 // 업그레이드
	ProposalTypeFunding   = 2 // 자금 지원

	// 투표 옵션
	VoteYes     = 0 // 찬성
	VoteNo      = 1 // 반대
	VoteAbstain = 2 // 기권

	// 거버넌스 매개변수
	DefaultVotingPeriod    = 40320 // 약 1주일 (15초 블록 기준)
	DefaultProposalDeposit = 100   // 100 토큰 (실제 구현에서는 10^18 단위로 변환)
	DefaultQuorum          = 33    // 33% 쿼럼
	DefaultThreshold       = 50    // 50% 찬성 임계값
	DefaultVetoThreshold   = 33    // 33% 거부권 임계값
	DefaultMinProposalAge  = 1440  // 약 6시간 (15초 블록 기준)
)

// Proposal은 거버넌스 제안을 나타냅니다.
type Proposal struct {
	ID          uint64         `json:"id"`          // 제안 ID
	Proposer    common.Address `json:"proposer"`    // 제안자 주소
	Title       string         `json:"title"`       // 제안 제목
	Description string         `json:"description"` // 제안 설명
	Type        uint8          `json:"type"`        // 제안 유형
	Status      uint8          `json:"status"`      // 제안 상태

	// 제안 내용 (유형에 따라 다름)
	Parameters map[string]string `json:"parameters"` // 매개변수 변경 제안의 경우
	Upgrade    *UpgradeInfo      `json:"upgrade"`    // 업그레이드 제안의 경우
	Funding    *FundingInfo      `json:"funding"`    // 자금 지원 제안의 경우

	// 제안 메타데이터
	SubmitBlock      uint64 `json:"submitBlock"`      // 제안 제출 블록
	VotingStartBlock uint64 `json:"votingStartBlock"` // 투표 시작 블록
	VotingEndBlock   uint64 `json:"votingEndBlock"`   // 투표 종료 블록
	ExecutionBlock   uint64 `json:"executionBlock"`   // 실행 블록 (통과된 경우)

	// 투표 결과
	YesVotes     *big.Int `json:"yesVotes"`     // 찬성 투표 수
	NoVotes      *big.Int `json:"noVotes"`      // 반대 투표 수
	AbstainVotes *big.Int `json:"abstainVotes"` // 기권 투표 수
	VetoVotes    *big.Int `json:"vetoVotes"`    // 거부권 투표 수
	TotalVotes   *big.Int `json:"totalVotes"`   // 총 투표 수

	// 제안 보증금
	Deposit *big.Int `json:"deposit"` // 제안 보증금
}

// UpgradeInfo는 업그레이드 제안에 대한 정보를 포함합니다.
type UpgradeInfo struct {
	Name        string `json:"name"`        // 업그레이드 이름
	Height      uint64 `json:"height"`      // 업그레이드 적용 블록 높이
	Info        string `json:"info"`        // 업그레이드 정보 (URL 등)
	ActivateMsg []byte `json:"activateMsg"` // 업그레이드 활성화 메시지
}

// FundingInfo는 자금 지원 제안에 대한 정보를 포함합니다.
type FundingInfo struct {
	Recipient common.Address `json:"recipient"` // 수령인 주소
	Amount    *big.Int       `json:"amount"`    // 지원 금액
	Reason    string         `json:"reason"`    // 지원 이유
}

// ProposalVote는 제안에 대한 투표를 나타냅니다.
type ProposalVote struct {
	ProposalID uint64         `json:"proposalId"` // 제안 ID
	Voter      common.Address `json:"voter"`      // 투표자 주소
	Option     uint8          `json:"option"`     // 투표 옵션
	Weight     *big.Int       `json:"weight"`     // 투표 가중치
	Block      uint64         `json:"block"`      // 투표 블록
}

// GovernanceState는 거버넌스 시스템의 상태를 나타냅니다.
type GovernanceState struct {
	Proposals      map[uint64]*Proposal                       `json:"proposals"`      // 제안 목록
	Votes          map[uint64]map[common.Address]ProposalVote `json:"votes"`          // 제안별 투표 목록
	NextProposalID uint64                                     `json:"nextProposalId"` // 다음 제안 ID

	// 거버넌스 매개변수
	VotingPeriod    uint64   `json:"votingPeriod"`    // 투표 기간 (블록 수)
	ProposalDeposit *big.Int `json:"proposalDeposit"` // 제안 보증금
	Quorum          uint8    `json:"quorum"`          // 쿼럼 (%)
	Threshold       uint8    `json:"threshold"`       // 통과 임계값 (%)
	VetoThreshold   uint8    `json:"vetoThreshold"`   // 거부권 임계값 (%)
	MinProposalAge  uint64   `json:"minProposalAge"`  // 최소 제안 나이 (블록 수)
}

// newGovernanceState는 새로운 거버넌스 상태를 생성합니다.
func newGovernanceState() *GovernanceState {
	// 100 토큰을 10^18 단위로 변환 (wei)
	depositAmount := new(big.Int).Mul(
		big.NewInt(DefaultProposalDeposit),
		new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil),
	)

	return &GovernanceState{
		Proposals:       make(map[uint64]*Proposal),
		Votes:           make(map[uint64]map[common.Address]ProposalVote),
		NextProposalID:  1,
		VotingPeriod:    DefaultVotingPeriod,
		ProposalDeposit: depositAmount,
		Quorum:          DefaultQuorum,
		Threshold:       DefaultThreshold,
		VetoThreshold:   DefaultVetoThreshold,
		MinProposalAge:  DefaultMinProposalAge,
	}
}

// loadGovernanceState는 데이터베이스에서 거버넌스 상태를 로드합니다.
func loadGovernanceState(db ethdb.Database) (*GovernanceState, error) {
	data, err := db.Get([]byte("eirene-governance"))
	if err != nil {
		// 데이터가 없으면 새로운 상태 생성
		return newGovernanceState(), nil
	}

	var state GovernanceState
	if err := rlp.DecodeBytes(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// store는 거버넌스 상태를 데이터베이스에 저장합니다.
func (gs *GovernanceState) store(db ethdb.Database) error {
	data, err := rlp.EncodeToBytes(gs)
	if err != nil {
		return err
	}

	return db.Put([]byte("eirene-governance"), data)
}

// submitProposal은 새로운 제안을 제출합니다.
func (gs *GovernanceState) submitProposal(
	proposer common.Address,
	title string,
	description string,
	proposalType uint8,
	parameters map[string]string,
	upgrade *UpgradeInfo,
	funding *FundingInfo,
	deposit *big.Int,
	currentBlock uint64,
) (uint64, error) {
	// 보증금 확인
	if deposit.Cmp(gs.ProposalDeposit) < 0 {
		return 0, errors.New("insufficient proposal deposit")
	}

	// 제안 생성
	proposal := &Proposal{
		ID:               gs.NextProposalID,
		Proposer:         proposer,
		Title:            title,
		Description:      description,
		Type:             proposalType,
		Status:           ProposalStatusPending,
		Parameters:       parameters,
		Upgrade:          upgrade,
		Funding:          funding,
		SubmitBlock:      currentBlock,
		VotingStartBlock: currentBlock + gs.MinProposalAge,
		VotingEndBlock:   currentBlock + gs.MinProposalAge + gs.VotingPeriod,
		YesVotes:         new(big.Int),
		NoVotes:          new(big.Int),
		AbstainVotes:     new(big.Int),
		VetoVotes:        new(big.Int),
		TotalVotes:       new(big.Int),
		Deposit:          new(big.Int).Set(deposit),
	}

	// 제안 저장
	gs.Proposals[gs.NextProposalID] = proposal
	gs.Votes[gs.NextProposalID] = make(map[common.Address]ProposalVote)

	// 다음 제안 ID 증가
	proposalID := gs.NextProposalID
	gs.NextProposalID++

	return proposalID, nil
}

// vote는 제안에 투표합니다.
func (gs *GovernanceState) vote(
	proposalID uint64,
	voter common.Address,
	option uint8,
	weight *big.Int,
	currentBlock uint64,
) error {
	// 제안 확인
	proposal, ok := gs.Proposals[proposalID]
	if !ok {
		return errors.New("proposal not found")
	}

	// 제안 상태 확인
	if proposal.Status != ProposalStatusActive {
		return errors.New("proposal is not active")
	}

	// 투표 기간 확인
	if currentBlock < proposal.VotingStartBlock || currentBlock > proposal.VotingEndBlock {
		return errors.New("voting period has not started or has ended")
	}

	// 이미 투표했는지 확인
	if _, voted := gs.Votes[proposalID][voter]; voted {
		return errors.New("already voted")
	}

	// 투표 옵션 확인
	if option > VoteAbstain {
		return errors.New("invalid vote option")
	}

	// 투표 저장
	vote := ProposalVote{
		ProposalID: proposalID,
		Voter:      voter,
		Option:     option,
		Weight:     new(big.Int).Set(weight),
		Block:      currentBlock,
	}

	gs.Votes[proposalID][voter] = vote

	// 투표 집계 업데이트
	switch option {
	case VoteYes:
		proposal.YesVotes = new(big.Int).Add(proposal.YesVotes, weight)
	case VoteNo:
		proposal.NoVotes = new(big.Int).Add(proposal.NoVotes, weight)
	case VoteAbstain:
		proposal.AbstainVotes = new(big.Int).Add(proposal.AbstainVotes, weight)
	}

	proposal.TotalVotes = new(big.Int).Add(proposal.TotalVotes, weight)

	return nil
}

// processProposals는 현재 블록에서 제안을 처리합니다.
func (gs *GovernanceState) processProposals(currentBlock uint64) {
	for id, proposal := range gs.Proposals {
		// 대기 중인 제안이 투표 시작 블록에 도달했는지 확인
		if proposal.Status == ProposalStatusPending && currentBlock >= proposal.VotingStartBlock {
			proposal.Status = ProposalStatusActive
			log.Info("Proposal activated", "id", id, "title", proposal.Title)
		}

		// 활성 제안이 투표 종료 블록에 도달했는지 확인
		if proposal.Status == ProposalStatusActive && currentBlock >= proposal.VotingEndBlock {
			gs.finalizeProposal(id, currentBlock)
		}

		// 통과된 제안이 실행 블록에 도달했는지 확인
		if proposal.Status == ProposalStatusPassed && currentBlock >= proposal.ExecutionBlock {
			gs.executeProposal(id, currentBlock)
		}
	}
}

// finalizeProposal은 투표가 종료된 제안을 최종 처리합니다.
func (gs *GovernanceState) finalizeProposal(proposalID uint64, currentBlock uint64) {
	proposal := gs.Proposals[proposalID]

	// 총 투표 가중치 계산
	totalStake := new(big.Int) // 실제 구현에서는 총 스테이킹 양을 가져와야 함

	// 쿼럼 확인
	quorum := new(big.Int).Mul(totalStake, big.NewInt(int64(gs.Quorum)))
	quorum = new(big.Int).Div(quorum, big.NewInt(100))

	if proposal.TotalVotes.Cmp(quorum) < 0 {
		// 쿼럼 미달
		proposal.Status = ProposalStatusRejected
		log.Info("Proposal rejected due to insufficient quorum",
			"id", proposalID,
			"votes", proposal.TotalVotes,
			"quorum", quorum)
		return
	}

	// 거부권 확인
	vetoThreshold := new(big.Int).Mul(totalStake, big.NewInt(int64(gs.VetoThreshold)))
	vetoThreshold = new(big.Int).Div(vetoThreshold, big.NewInt(100))

	if proposal.VetoVotes.Cmp(vetoThreshold) >= 0 {
		// 거부권 행사
		proposal.Status = ProposalStatusRejected
		log.Info("Proposal vetoed",
			"id", proposalID,
			"vetoVotes", proposal.VetoVotes,
			"threshold", vetoThreshold)
		return
	}

	// 통과 임계값 확인
	threshold := new(big.Int).Mul(proposal.TotalVotes, big.NewInt(int64(gs.Threshold)))
	threshold = new(big.Int).Div(threshold, big.NewInt(100))

	if proposal.YesVotes.Cmp(threshold) >= 0 {
		// 제안 통과
		proposal.Status = ProposalStatusPassed
		proposal.ExecutionBlock = currentBlock + 1440 // 약 6시간 후 실행 (15초 블록 기준)
		log.Info("Proposal passed",
			"id", proposalID,
			"yesVotes", proposal.YesVotes,
			"threshold", threshold,
			"executionBlock", proposal.ExecutionBlock)
	} else {
		// 제안 거부
		proposal.Status = ProposalStatusRejected
		log.Info("Proposal rejected",
			"id", proposalID,
			"yesVotes", proposal.YesVotes,
			"threshold", threshold)
	}
}

// executeProposal은 통과된 제안을 실행합니다.
func (gs *GovernanceState) executeProposal(proposalID uint64, currentBlock uint64) {
	proposal := gs.Proposals[proposalID]

	// 제안 유형에 따라 실행
	switch proposal.Type {
	case ProposalTypeParameter:
		// 매개변수 변경 제안 실행
		gs.executeParameterProposal(proposal)
	case ProposalTypeUpgrade:
		// 업그레이드 제안 실행
		gs.executeUpgradeProposal(proposal)
	case ProposalTypeFunding:
		// 자금 지원 제안 실행
		gs.executeFundingProposal(proposal)
	}

	// 제안 상태 업데이트
	proposal.Status = ProposalStatusExecuted
	log.Info("Proposal executed", "id", proposalID, "type", proposal.Type)
}

// executeParameterProposal은 매개변수 변경 제안을 실행합니다.
func (gs *GovernanceState) executeParameterProposal(proposal *Proposal) {
	// 매개변수 변경
	for key, value := range proposal.Parameters {
		switch key {
		case "votingPeriod":
			if period, ok := new(big.Int).SetString(value, 10); ok {
				gs.VotingPeriod = period.Uint64()
			}
		case "proposalDeposit":
			if deposit, ok := new(big.Int).SetString(value, 10); ok {
				gs.ProposalDeposit = deposit
			}
		case "quorum":
			if quorum, ok := new(big.Int).SetString(value, 10); ok {
				gs.Quorum = uint8(quorum.Uint64())
			}
		case "threshold":
			if threshold, ok := new(big.Int).SetString(value, 10); ok {
				gs.Threshold = uint8(threshold.Uint64())
			}
		case "vetoThreshold":
			if vetoThreshold, ok := new(big.Int).SetString(value, 10); ok {
				gs.VetoThreshold = uint8(vetoThreshold.Uint64())
			}
		case "minProposalAge":
			if age, ok := new(big.Int).SetString(value, 10); ok {
				gs.MinProposalAge = age.Uint64()
			}
		}
	}
}

// executeUpgradeProposal은 업그레이드 제안을 실행합니다.
func (gs *GovernanceState) executeUpgradeProposal(proposal *Proposal) {
	// 업그레이드 제안 실행 로직
	// 실제 구현에서는 업그레이드 정보를 저장하고 적절한 시점에 업그레이드를 수행
	if proposal.Upgrade != nil {
		log.Info("Upgrade scheduled",
			"name", proposal.Upgrade.Name,
			"height", proposal.Upgrade.Height,
			"info", proposal.Upgrade.Info)
	}
}

// executeFundingProposal은 자금 지원 제안을 실행합니다.
func (gs *GovernanceState) executeFundingProposal(proposal *Proposal) {
	// 자금 지원 제안 실행 로직
	// 실제 구현에서는 커뮤니티 기금에서 지정된 주소로 자금을 전송
	if proposal.Funding != nil {
		log.Info("Funding executed",
			"recipient", proposal.Funding.Recipient,
			"amount", proposal.Funding.Amount,
			"reason", proposal.Funding.Reason)
	}
}

// getProposal은 제안 정보를 반환합니다.
func (gs *GovernanceState) getProposal(proposalID uint64) (*Proposal, error) {
	proposal, ok := gs.Proposals[proposalID]
	if !ok {
		return nil, errors.New("proposal not found")
	}

	return proposal, nil
}

// getVotes는 제안에 대한 투표 목록을 반환합니다.
func (gs *GovernanceState) getVotes(proposalID uint64) ([]ProposalVote, error) {
	votes, ok := gs.Votes[proposalID]
	if !ok {
		return nil, errors.New("proposal not found")
	}

	result := make([]ProposalVote, 0, len(votes))
	for _, vote := range votes {
		result = append(result, vote)
	}

	return result, nil
}

// getActiveProposals는 활성 상태인 제안 목록을 반환합니다.
func (gs *GovernanceState) getActiveProposals() []*Proposal {
	result := make([]*Proposal, 0)

	for _, proposal := range gs.Proposals {
		if proposal.Status == ProposalStatusActive {
			result = append(result, proposal)
		}
	}

	return result
}

// getAllProposals는 모든 제안 목록을 반환합니다.
func (gs *GovernanceState) getAllProposals() []*Proposal {
	result := make([]*Proposal, 0, len(gs.Proposals))

	for _, proposal := range gs.Proposals {
		result = append(result, proposal)
	}

	return result
}

// getGovernanceParams는 현재 거버넌스 매개변수를 반환합니다.
func (gs *GovernanceState) getGovernanceParams() map[string]interface{} {
	return map[string]interface{}{
		"votingPeriod":    gs.VotingPeriod,
		"proposalDeposit": gs.ProposalDeposit,
		"quorum":          gs.Quorum,
		"threshold":       gs.Threshold,
		"vetoThreshold":   gs.VetoThreshold,
		"minProposalAge":  gs.MinProposalAge,
	}
}
