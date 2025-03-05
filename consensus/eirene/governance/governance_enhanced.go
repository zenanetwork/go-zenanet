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

// 확장된 거버넌스 관련 상수
const (
	// 추가 제안 유형
	ProposalTypeTextProposal     = 3 // 텍스트 제안
	ProposalTypeNetworkParameter = 4 // 네트워크 매개변수 변경
	ProposalTypeCommunityPool    = 5 // 커뮤니티 풀 사용
	ProposalTypeEmergency        = 6 // 긴급 제안

	// 추가 투표 옵션
	VoteVeto           = 3 // 거부권
	VoteWithConditions = 4 // 조건부 찬성

	// 투표 가중치 유형
	VoteWeightTypeEqual     = 0 // 동등한 가중치
	VoteWeightTypeStake     = 1 // 스테이킹 기반 가중치
	VoteWeightTypeQuadratic = 2 // 이차 투표 가중치

	// 거버넌스 매개변수
	DefaultMinDepositEmergency    = 1000 // 긴급 제안 최소 보증금 (1000 토큰)
	DefaultEmergencyVotingPeriod  = 2880 // 약 12시간 (15초 블록 기준)
	DefaultEmergencyQuorum        = 50   // 50% 쿼럼
	DefaultEmergencyThreshold     = 67   // 67% 찬성 임계값
	DefaultTextProposalDeposit    = 10   // 10 토큰
	DefaultCommunityPoolThreshold = 75   // 75% 찬성 임계값 (커뮤니티 풀 사용)
)

// EnhancedProposal은 확장된 거버넌스 제안을 나타냅니다.
type EnhancedProposal struct {
	// 기본 제안 정보
	Proposal *Proposal `json:"proposal"` // 기본 제안 정보

	// 확장된 제안 정보
	Tags             []string          `json:"tags"`             // 제안 태그
	RelatedProposals []uint64          `json:"relatedProposals"` // 관련 제안 ID
	Attachments      []string          `json:"attachments"`      // 첨부 파일 (IPFS 해시 등)
	Discussions      []DiscussionEntry `json:"discussions"`      // 토론 내용
	Amendments       []Amendment       `json:"amendments"`       // 수정안
	VoteWeightType   uint8             `json:"voteWeightType"`   // 투표 가중치 유형

	// 조건부 투표 관련
	ConditionalVotes []ConditionalVote `json:"conditionalVotes"` // 조건부 투표 목록

	// 긴급 제안 관련
	IsEmergency     bool   `json:"isEmergency"`     // 긴급 제안 여부
	EmergencyReason string `json:"emergencyReason"` // 긴급 제안 이유

	// 커뮤니티 풀 관련
	CommunityPoolAmount *big.Int `json:"communityPoolAmount"` // 커뮤니티 풀 사용 금액

	// 네트워크 매개변수 관련
	NetworkParameters map[string]string `json:"networkParameters"` // 네트워크 매개변수

	// 투표 결과 확장
	ConditionalYesVotes *big.Int          `json:"conditionalYesVotes"` // 조건부 찬성 투표 수
	VoteDistribution    map[string]uint64 `json:"voteDistribution"`    // 투표 분포 (주소 그룹별)

	// 이행 상태
	ImplementationStatus  uint8  `json:"implementationStatus"`  // 이행 상태
	ImplementationDetails string `json:"implementationDetails"` // 이행 세부 정보
}

// DiscussionEntry는 제안에 대한 토론 항목을 나타냅니다.
type DiscussionEntry struct {
	Author    common.Address    `json:"author"`    // 작성자 주소
	Timestamp uint64            `json:"timestamp"` // 타임스탬프
	Content   string            `json:"content"`   // 내용
	Replies   []DiscussionEntry `json:"replies"`   // 답글
}

// Amendment는 제안에 대한 수정안을 나타냅니다.
type Amendment struct {
	Proposer   common.Address `json:"proposer"`   // 제안자 주소
	Timestamp  uint64         `json:"timestamp"`  // 타임스탬프
	Content    string         `json:"content"`    // 내용
	Accepted   bool           `json:"accepted"`   // 수락 여부
	AcceptedAt uint64         `json:"acceptedAt"` // 수락 시간
}

// ConditionalVote는 조건부 투표를 나타냅니다.
type ConditionalVote struct {
	Voter      common.Address `json:"voter"`      // 투표자 주소
	Conditions []string       `json:"conditions"` // 조건 목록
	Weight     *big.Int       `json:"weight"`     // 투표 가중치
	Timestamp  uint64         `json:"timestamp"`  // 타임스탬프
}

// EnhancedGovernanceState는 확장된 거버넌스 상태를 나타냅니다.
type EnhancedGovernanceState struct {
	// 기본 거버넌스 상태
	GovernanceState *GovernanceState `json:"governanceState"` // 기본 거버넌스 상태

	// 확장된 제안 관리
	EnhancedProposals map[uint64]*EnhancedProposal `json:"enhancedProposals"` // 확장된 제안 맵

	// 거버넌스 통계
	TotalProposals       uint64 `json:"totalProposals"`       // 총 제안 수
	PassedProposals      uint64 `json:"passedProposals"`      // 통과된 제안 수
	RejectedProposals    uint64 `json:"rejectedProposals"`    // 거부된 제안 수
	ImplementedProposals uint64 `json:"implementedProposals"` // 이행된 제안 수

	// 거버넌스 참여 통계
	UniqueVoters         uint64 `json:"uniqueVoters"`         // 고유 투표자 수
	TotalVotesCast       uint64 `json:"totalVotesCast"`       // 총 투표 수
	AverageParticipation uint64 `json:"averageParticipation"` // 평균 참여율 (1000 단위)

	// 커뮤니티 풀 관리
	CommunityPoolBalance *big.Int `json:"communityPoolBalance"` // 커뮤니티 풀 잔액
	CommunityPoolSpent   *big.Int `json:"communityPoolSpent"`   // 커뮤니티 풀 지출액

	// 네트워크 매개변수
	NetworkParameters map[string]string `json:"networkParameters"` // 네트워크 매개변수
}

// newEnhancedGovernanceState는 새로운 확장 거버넌스 상태를 생성합니다.
func newEnhancedGovernanceState() *EnhancedGovernanceState {
	return &EnhancedGovernanceState{
		GovernanceState:      newGovernanceState(),
		EnhancedProposals:    make(map[uint64]*EnhancedProposal),
		CommunityPoolBalance: new(big.Int),
		CommunityPoolSpent:   new(big.Int),
		NetworkParameters:    make(map[string]string),
	}
}

// loadEnhancedGovernanceState는 데이터베이스에서 확장 거버넌스 상태를 로드합니다.
func loadEnhancedGovernanceState(db ethdb.Database) (*EnhancedGovernanceState, error) {
	data, err := db.Get([]byte("eirene-enhanced-governance"))
	if err != nil {
		// 데이터가 없으면 새로운 거버넌스 상태 생성
		return newEnhancedGovernanceState(), nil
	}

	var governanceState EnhancedGovernanceState
	if err := rlp.DecodeBytes(data, &governanceState); err != nil {
		return nil, err
	}

	return &governanceState, nil
}

// store는 확장 거버넌스 상태를 데이터베이스에 저장합니다.
func (gs *EnhancedGovernanceState) store(db ethdb.Database) error {
	data, err := rlp.EncodeToBytes(gs)
	if err != nil {
		return err
	}

	return db.Put([]byte("eirene-enhanced-governance"), data)
}

// submitEnhancedProposal은 확장된 제안을 제출합니다.
func (gs *EnhancedGovernanceState) submitEnhancedProposal(
	proposer common.Address,
	title string,
	description string,
	proposalType uint8,
	deposit *big.Int,
	submitBlock uint64,
	votingPeriod uint64,
	tags []string,
	attachments []string,
	voteWeightType uint8,
	isEmergency bool,
	emergencyReason string,
	communityPoolAmount *big.Int,
	networkParameters map[string]string,
) (uint64, error) {
	// 기본 제안 생성
	proposalID, err := gs.GovernanceState.submitProposal(
		proposer,
		title,
		description,
		proposalType,
		networkParameters,
		nil,
		nil,
		deposit,
		submitBlock,
	)
	if err != nil {
		return 0, err
	}

	// 확장 제안 생성
	proposal, err := gs.GovernanceState.getProposal(proposalID)
	if err != nil {
		return 0, err
	}
	enhancedProposal := &EnhancedProposal{
		Proposal:              proposal,
		Tags:                  tags,
		RelatedProposals:      []uint64{},
		Attachments:           attachments,
		Discussions:           []DiscussionEntry{},
		Amendments:            []Amendment{},
		VoteWeightType:        voteWeightType,
		ConditionalVotes:      []ConditionalVote{},
		IsEmergency:           isEmergency,
		EmergencyReason:       emergencyReason,
		CommunityPoolAmount:   communityPoolAmount,
		NetworkParameters:     networkParameters,
		ConditionalYesVotes:   new(big.Int),
		VoteDistribution:      make(map[string]uint64),
		ImplementationStatus:  0,
		ImplementationDetails: "",
	}

	// 긴급 제안인 경우 투표 기간 조정
	if isEmergency {
		// 투표 종료 블록을 조정 (시작 블록 + 긴급 투표 기간)
		proposal.VotingEndBlock = proposal.VotingStartBlock + DefaultEmergencyVotingPeriod
	}

	// 제안 저장
	gs.EnhancedProposals[proposalID] = enhancedProposal
	gs.TotalProposals++

	log.Info("Enhanced proposal submitted",
		"id", proposalID,
		"type", proposalType,
		"isEmergency", isEmergency,
		"proposer", proposer)

	return proposalID, nil
}

// voteOnEnhancedProposal은 확장된 제안에 투표합니다.
func (gs *EnhancedGovernanceState) voteOnEnhancedProposal(
	proposalID uint64,
	voter common.Address,
	voteOption uint8,
	voteWeight *big.Int,
	conditions []string,
	blockNumber uint64,
) error {
	// 기본 투표 처리
	if voteOption <= VoteAbstain {
		err := gs.GovernanceState.vote(proposalID, voter, voteOption, voteWeight, blockNumber)
		if err != nil {
			return err
		}
	}

	// 확장 제안 가져오기
	enhancedProposal, exists := gs.EnhancedProposals[proposalID]
	if !exists {
		return errors.New("enhanced proposal not found")
	}

	// 조건부 찬성 투표 처리
	if voteOption == VoteWithConditions {
		conditionalVote := ConditionalVote{
			Voter:      voter,
			Conditions: conditions,
			Weight:     new(big.Int).Set(voteWeight),
			Timestamp:  blockNumber,
		}
		enhancedProposal.ConditionalVotes = append(enhancedProposal.ConditionalVotes, conditionalVote)
		enhancedProposal.ConditionalYesVotes = new(big.Int).Add(enhancedProposal.ConditionalYesVotes, voteWeight)
	}

	// 투표 통계 업데이트
	gs.TotalVotesCast++

	// 투표자 그룹 분류 (예: 검증자, 위임자, 일반 사용자)
	voterGroup := "regular"
	// 실제 구현에서는 투표자 유형에 따라 분류

	enhancedProposal.VoteDistribution[voterGroup]++

	log.Info("Vote cast on enhanced proposal",
		"proposalID", proposalID,
		"voter", voter,
		"option", voteOption,
		"weight", voteWeight,
		"hasConditions", len(conditions) > 0)

	return nil
}

// addDiscussion은 제안에 토론 항목을 추가합니다.
func (gs *EnhancedGovernanceState) addDiscussion(
	proposalID uint64,
	author common.Address,
	content string,
	timestamp uint64,
	parentIndex int, // -1이면 최상위 토론
) error {
	enhancedProposal, exists := gs.EnhancedProposals[proposalID]
	if !exists {
		return errors.New("enhanced proposal not found")
	}

	discussion := DiscussionEntry{
		Author:    author,
		Timestamp: timestamp,
		Content:   content,
		Replies:   []DiscussionEntry{},
	}

	if parentIndex == -1 {
		// 최상위 토론 추가
		enhancedProposal.Discussions = append(enhancedProposal.Discussions, discussion)
	} else if parentIndex >= 0 && parentIndex < len(enhancedProposal.Discussions) {
		// 답글 추가
		enhancedProposal.Discussions[parentIndex].Replies = append(
			enhancedProposal.Discussions[parentIndex].Replies,
			discussion,
		)
	} else {
		return errors.New("invalid parent discussion index")
	}

	log.Info("Discussion added to proposal",
		"proposalID", proposalID,
		"author", author,
		"parentIndex", parentIndex)

	return nil
}

// proposeAmendment는 제안에 수정안을 제안합니다.
func (gs *EnhancedGovernanceState) proposeAmendment(
	proposalID uint64,
	proposer common.Address,
	content string,
	timestamp uint64,
) error {
	enhancedProposal, exists := gs.EnhancedProposals[proposalID]
	if !exists {
		return errors.New("enhanced proposal not found")
	}

	// 제안 상태 확인
	if enhancedProposal.Proposal.Status != ProposalStatusActive {
		return errors.New("proposal is not in active state")
	}

	amendment := Amendment{
		Proposer:   proposer,
		Timestamp:  timestamp,
		Content:    content,
		Accepted:   false,
		AcceptedAt: 0,
	}

	enhancedProposal.Amendments = append(enhancedProposal.Amendments, amendment)

	log.Info("Amendment proposed",
		"proposalID", proposalID,
		"proposer", proposer)

	return nil
}

// acceptAmendment는 제안의 수정안을 수락합니다.
func (gs *EnhancedGovernanceState) acceptAmendment(
	proposalID uint64,
	amendmentIndex int,
	acceptedAt uint64,
) error {
	enhancedProposal, exists := gs.EnhancedProposals[proposalID]
	if !exists {
		return errors.New("enhanced proposal not found")
	}

	if amendmentIndex < 0 || amendmentIndex >= len(enhancedProposal.Amendments) {
		return errors.New("invalid amendment index")
	}

	// 제안 상태 확인
	if enhancedProposal.Proposal.Status != ProposalStatusActive {
		return errors.New("proposal is not in active state")
	}

	// 수정안 수락
	enhancedProposal.Amendments[amendmentIndex].Accepted = true
	enhancedProposal.Amendments[amendmentIndex].AcceptedAt = acceptedAt

	log.Info("Amendment accepted",
		"proposalID", proposalID,
		"amendmentIndex", amendmentIndex)

	return nil
}

// processEnhancedProposal은 확장된 제안을 처리합니다.
func (gs *EnhancedGovernanceState) processEnhancedProposal(proposalID uint64, blockNumber uint64) error {
	// 기본 제안 처리
	gs.GovernanceState.finalizeProposal(proposalID, blockNumber)

	enhancedProposal, exists := gs.EnhancedProposals[proposalID]
	if !exists {
		return errors.New("enhanced proposal not found")
	}

	// 제안 결과에 따른 통계 업데이트
	if enhancedProposal.Proposal.Status == ProposalStatusPassed {
		gs.PassedProposals++

		// 커뮤니티 풀 제안 처리
		if enhancedProposal.Proposal.Type == ProposalTypeCommunityPool && enhancedProposal.CommunityPoolAmount != nil {
			// 커뮤니티 풀 잔액 확인
			if gs.CommunityPoolBalance.Cmp(enhancedProposal.CommunityPoolAmount) < 0 {
				log.Error("Community pool balance insufficient",
					"proposalID", proposalID,
					"required", enhancedProposal.CommunityPoolAmount,
					"available", gs.CommunityPoolBalance)
				return errors.New("community pool balance insufficient")
			}

			// 커뮤니티 풀 잔액 업데이트
			gs.CommunityPoolBalance = new(big.Int).Sub(gs.CommunityPoolBalance, enhancedProposal.CommunityPoolAmount)
			gs.CommunityPoolSpent = new(big.Int).Add(gs.CommunityPoolSpent, enhancedProposal.CommunityPoolAmount)

			log.Info("Community pool funds allocated",
				"proposalID", proposalID,
				"amount", enhancedProposal.CommunityPoolAmount,
				"newBalance", gs.CommunityPoolBalance)
		}

		// 네트워크 매개변수 제안 처리
		if enhancedProposal.Proposal.Type == ProposalTypeNetworkParameter {
			// 네트워크 매개변수 업데이트
			for key, value := range enhancedProposal.NetworkParameters {
				gs.NetworkParameters[key] = value
				log.Info("Network parameter updated",
					"proposalID", proposalID,
					"key", key,
					"value", value)
			}
		}
	} else if enhancedProposal.Proposal.Status == ProposalStatusRejected {
		gs.RejectedProposals++
	}

	log.Info("Enhanced proposal processed",
		"proposalID", proposalID,
		"status", enhancedProposal.Proposal.Status)

	return nil
}

// updateImplementationStatus는 제안의 이행 상태를 업데이트합니다.
func (gs *EnhancedGovernanceState) updateImplementationStatus(
	proposalID uint64,
	status uint8,
	details string,
) error {
	enhancedProposal, exists := gs.EnhancedProposals[proposalID]
	if !exists {
		return errors.New("enhanced proposal not found")
	}

	// 제안 상태 확인
	if enhancedProposal.Proposal.Status != ProposalStatusPassed {
		return errors.New("proposal is not in passed state")
	}

	// 이행 상태 업데이트
	enhancedProposal.ImplementationStatus = status
	enhancedProposal.ImplementationDetails = details

	// 완전히 이행된 경우 통계 업데이트
	if status == 100 { // 100%
		gs.ImplementedProposals++
	}

	log.Info("Proposal implementation status updated",
		"proposalID", proposalID,
		"status", status,
		"details", details)

	return nil
}

// getEnhancedProposal은 확장된 제안을 조회합니다.
func (gs *EnhancedGovernanceState) getEnhancedProposal(proposalID uint64) *EnhancedProposal {
	return gs.EnhancedProposals[proposalID]
}

// getEnhancedProposalsByType은 유형별로 확장된 제안을 조회합니다.
func (gs *EnhancedGovernanceState) getEnhancedProposalsByType(proposalType uint8) []*EnhancedProposal {
	proposals := make([]*EnhancedProposal, 0)

	for _, proposal := range gs.EnhancedProposals {
		if proposal.Proposal.Type == proposalType {
			proposals = append(proposals, proposal)
		}
	}

	return proposals
}

// getEnhancedProposalsByTags는 태그별로 확장된 제안을 조회합니다.
func (gs *EnhancedGovernanceState) getEnhancedProposalsByTags(tags []string) []*EnhancedProposal {
	proposals := make([]*EnhancedProposal, 0)

	for _, proposal := range gs.EnhancedProposals {
		// 태그 일치 여부 확인
		matches := false
		for _, tag := range tags {
			for _, proposalTag := range proposal.Tags {
				if tag == proposalTag {
					matches = true
					break
				}
			}
			if matches {
				break
			}
		}

		if matches {
			proposals = append(proposals, proposal)
		}
	}

	return proposals
}

// addCommunityPoolFunds는 커뮤니티 풀에 자금을 추가합니다.
func (gs *EnhancedGovernanceState) addCommunityPoolFunds(amount *big.Int) {
	gs.CommunityPoolBalance = new(big.Int).Add(gs.CommunityPoolBalance, amount)

	log.Info("Community pool funds added",
		"amount", amount,
		"newBalance", gs.CommunityPoolBalance)
}

// getNetworkParameter는 네트워크 매개변수를 조회합니다.
func (gs *EnhancedGovernanceState) getNetworkParameter(key string) (string, bool) {
	value, exists := gs.NetworkParameters[key]
	return value, exists
}

// setNetworkParameter는 네트워크 매개변수를 설정합니다.
func (gs *EnhancedGovernanceState) setNetworkParameter(key string, value string) {
	gs.NetworkParameters[key] = value

	log.Info("Network parameter set",
		"key", key,
		"value", value)
}

// calculateQuadraticVoteWeight는 이차 투표 가중치를 계산합니다.
func calculateQuadraticVoteWeight(stake *big.Int) *big.Int {
	// 이차 투표 가중치 = sqrt(stake)
	// 간단한 구현을 위해 stake의 제곱근을 계산
	// 실제 구현에서는 더 정확한 제곱근 계산 알고리즘 사용

	// stake를 float64로 변환
	stakeFloat := new(big.Float).SetInt(stake)
	stakeFloat64, _ := stakeFloat.Float64()

	// 제곱근 계산
	sqrtStake := new(big.Float).SetFloat64(stakeFloat64)
	sqrtStake.Sqrt(sqrtStake)

	// 결과를 big.Int로 변환
	result := new(big.Int)
	sqrtStake.Int(result)

	return result
}
