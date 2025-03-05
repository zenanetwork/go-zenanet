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
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/rlp"
)

// 확장 거버넌스 상수
const (
	// 긴급 제안 관련
	DefaultEmergencyVotingPeriod = 86400 // 1일 (초 단위)
	
	// 투표 가중치 유형
	VoteWeightTypeEqual     = 0 // 동등한 가중치
	VoteWeightTypeStake     = 1 // 스테이킹 기반 가중치
	VoteWeightTypeQuadratic = 2 // 이차 투표 가중치
	
	// 거버넌스 매개변수
	DefaultMinDepositEmergency = 1000 // 긴급 제안 최소 보증금 (1000 토큰)
	DefaultEmergencyQuorum     = 50   // 50% 쿼럼
	DefaultEmergencyThreshold  = 67   // 67% 찬성 임계값
	
	// 추가 투표 옵션
	VoteWithConditions = "with_conditions" // 조건부 찬성
)

// EnhancedGovernanceState는 확장된 거버넌스 상태를 나타냅니다.
type EnhancedGovernanceState struct {
	*EnhancedGovernanceStateBase                     // 기본 거버넌스 상태 포함
	EnhancedProposals map[uint64]*EnhancedProposal // 확장된 제안 목록
	TotalProposals    uint64             // 총 제안 수
	PassedProposals   uint64             // 통과된 제안 수
	RejectedProposals uint64             // 거부된 제안 수
	ImplementedProposals uint64          // 구현된 제안 수
	
	// 커뮤니티 풀 관리
	CommunityPoolBalance *big.Int        // 커뮤니티 풀 잔액
	CommunityPoolSpent   *big.Int        // 커뮤니티 풀 지출액
	
	// 네트워크 매개변수
	NetworkParameters map[string]string  // 네트워크 매개변수
}

// DiscussionEntry는 제안에 대한 토론 항목을 나타냅니다.
type DiscussionEntry struct {
	Author    common.Address `json:"author"`    // 작성자
	Content   string         `json:"content"`   // 내용
	Timestamp uint64         `json:"timestamp"` // 타임스탬프
	ParentIdx int            `json:"parentIdx"` // 부모 인덱스 (-1이면 최상위)
	Likes     uint64         `json:"likes"`     // 좋아요 수
	Dislikes  uint64         `json:"dislikes"`  // 싫어요 수
	Replies   []int          `json:"replies"`   // 답글 인덱스 목록
}

// Amendment는 제안에 대한 수정안을 나타냅니다.
type Amendment struct {
	Proposer   common.Address `json:"proposer"`   // 제안자
	Content    string         `json:"content"`    // 내용
	Timestamp  uint64         `json:"timestamp"`  // 타임스탬프
	Accepted   bool           `json:"accepted"`   // 수락 여부
	AcceptedAt uint64         `json:"acceptedAt"` // 수락 시간
}

// ConditionalVote는 조건부 투표를 나타냅니다.
type ConditionalVote struct {
	Voter      common.Address `json:"voter"`      // 투표자
	VoteOption uint8          `json:"voteOption"` // 투표 옵션
	Weight     *big.Int       `json:"weight"`     // 투표 가중치
	Conditions []string       `json:"conditions"` // 조건 목록
	Timestamp  uint64         `json:"timestamp"`  // 타임스탬프
}

// newEnhancedGovernanceState는 새로운 확장 거버넌스 상태를 생성합니다.
func newEnhancedGovernanceState() *EnhancedGovernanceState {
	governanceState := newEnhancedGovernanceStateBase()
	
	return &EnhancedGovernanceState{
		EnhancedGovernanceStateBase: governanceState,
		EnhancedProposals:    make(map[uint64]*EnhancedProposal),
		TotalProposals:       0,
		PassedProposals:      0,
		RejectedProposals:    0,
		ImplementedProposals: 0,
		CommunityPoolBalance: big.NewInt(0),
		CommunityPoolSpent:   big.NewInt(0),
		NetworkParameters:    make(map[string]string),
	}
}

// GovernanceState는 기본 거버넌스 상태를 나타냅니다.
type EnhancedGovernanceStateBase struct {
	Proposals       map[uint64]*Proposal              // 제안 ID -> 제안
	Votes           map[uint64]map[common.Address]int // 제안 ID -> 투표자 -> 투표 옵션
	NextProposalID  uint64                            // 다음 제안 ID
	VotingPeriod    uint64                            // 투표 기간 (블록 수)
	QuorumThreshold uint64                            // 정족수 임계값 (%)
	PassThreshold   uint64                            // 통과 임계값 (%)
	MinProposalAge  uint64                            // 최소 제안 나이 (블록 수)
}

// newGovernanceState는 새로운 거버넌스 상태를 생성합니다.
func newEnhancedGovernanceStateBase() *EnhancedGovernanceStateBase {
	return &EnhancedGovernanceStateBase{
		Proposals:       make(map[uint64]*Proposal),
		Votes:           make(map[uint64]map[common.Address]int),
		NextProposalID:  1,
		VotingPeriod:    20160, // 약 1주일 (15초 블록 기준)
		QuorumThreshold: 33, // 33%
		PassThreshold:   50, // 50%
		MinProposalAge:  100,   // 약 25분 (15초 블록 기준)
	}
}

// 확장된 거버넌스 관련 상수
const (
	// 추가 제안 유형 (문자열)
	ProposalTypeTextProposal     = "text_proposal"     // 텍스트 제안
	ProposalTypeNetworkParameter = "network_parameter" // 네트워크 매개변수 변경
	ProposalTypeCommunityPool    = "community_pool"    // 커뮤니티 풀 사용
	ProposalTypeEmergency        = "emergency"         // 긴급 제안

	// 제안 상태
	ProposalStatusActive = "active" // 활성 상태
	
	// 투표 옵션 값 (uint8)
	VoteOptionYesValue        uint8 = 1 // 찬성
	VoteOptionNoValue         uint8 = 2 // 반대
	VoteOptionAbstainValue    uint8 = 3 // 기권
	VoteOptionVetoValue       uint8 = 4 // 거부권
	VoteWithConditionsValue   uint8 = 5 // 조건부 찬성
	
	// 제안 유형 값 (uint8)
	ProposalTypeTextProposalValue     uint8 = 1 // 텍스트 제안
	ProposalTypeNetworkParameterValue uint8 = 2 // 네트워크 매개변수 변경
	ProposalTypeCommunityPoolValue    uint8 = 3 // 커뮤니티 풀 사용
	ProposalTypeEmergencyValue        uint8 = 4 // 긴급 제안
)

// 투표 옵션 uint8을 문자열로 변환
func voteOptionToString(voteOption uint8) string {
	switch voteOption {
	case VoteOptionYesValue:
		return "yes"
	case VoteOptionNoValue:
		return "no"
	case VoteOptionAbstainValue:
		return "abstain"
	case VoteOptionVetoValue:
		return "veto"
	case VoteWithConditionsValue:
		return VoteWithConditions
	default:
		return ""
	}
}

// 제안 유형 uint8을 문자열로 변환
func proposalTypeToString(proposalType uint8) string {
	switch proposalType {
	case ProposalTypeTextProposalValue:
		return ProposalTypeTextProposal
	case ProposalTypeNetworkParameterValue:
		return ProposalTypeNetworkParameter
	case ProposalTypeCommunityPoolValue:
		return ProposalTypeCommunityPool
	case ProposalTypeEmergencyValue:
		return ProposalTypeEmergency
	default:
		return ""
	}
}

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
	ImplementationStatus  uint8  `json:"implementationStatus"`  // 이행 상태 (0: 미이행, 1: 이행 중, 2: 이행 완료)
	ImplementationDetails string `json:"implementationDetails"` // 이행 세부 정보
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
	proposalID, err := gs.EnhancedGovernanceStateBase.submitProposal(
		proposer,
		title,
		description,
		proposalTypeToString(proposalType),
		deposit,
		submitBlock,
		votingPeriod,
	)
	if err != nil {
		return 0, err
	}

	// 확장 제안 생성
	proposal, err := gs.EnhancedGovernanceStateBase.getProposal(proposalID)
	if err != nil {
		return 0, err
	}

	// 확장 제안 생성
	enhancedProposal := &EnhancedProposal{
		Proposal:            proposal,
		Tags:                tags,
		RelatedProposals:    []uint64{},
		Attachments:         attachments,
		Discussions:         []DiscussionEntry{},
		Amendments:          []Amendment{},
		VoteWeightType:      voteWeightType,
		ConditionalVotes:    []ConditionalVote{},
		IsEmergency:         isEmergency,
		EmergencyReason:     emergencyReason,
		CommunityPoolAmount: communityPoolAmount,
		NetworkParameters:   networkParameters,
		ConditionalYesVotes: big.NewInt(0),
		VoteDistribution:    make(map[string]uint64),
	}

	// 긴급 제안인 경우 투표 기간 조정
	if isEmergency {
		// 투표 종료 시간을 조정 (시작 시간 + 긴급 투표 기간)
		emergencyVotingPeriod := time.Duration(DefaultEmergencyVotingPeriod) * time.Second
		proposal.VotingEnd = proposal.VotingStart.Add(emergencyVotingPeriod)
	}

	// 제안 저장
	gs.EnhancedProposals[proposalID] = enhancedProposal

	// 통계 업데이트
	gs.TotalProposals++

	log.Info("Enhanced proposal submitted",
		"id", proposalID,
		"type", proposalTypeToString(proposalType),
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
	enhancedProposal, exists := gs.EnhancedProposals[proposalID]
	if !exists {
		return errors.New("enhanced proposal not found")
	}

	// 기본 투표 처리
	if voteOption <= VoteOptionAbstainValue {
		err := gs.EnhancedGovernanceStateBase.vote(proposalID, voter, voteOptionToString(voteOption), voteWeight, blockNumber)
		if err != nil {
			return err
		}
	}

	// 조건부 찬성 투표 처리
	if voteOption == VoteWithConditionsValue {
		conditionalVote := ConditionalVote{
			Voter:      voter,
			Conditions: conditions,
			Weight:     voteWeight,
			Timestamp:  blockNumber,
		}
		enhancedProposal.ConditionalVotes = append(enhancedProposal.ConditionalVotes, conditionalVote)
		enhancedProposal.ConditionalYesVotes = new(big.Int).Add(enhancedProposal.ConditionalYesVotes, voteWeight)
	}

	log.Info("Vote cast on enhanced proposal",
		"proposalID", proposalID,
		"voter", voter,
		"option", voteOptionToString(voteOption),
		"weight", voteWeight,
		"hasConditions", len(conditions) > 0)

	return nil
}

// addDiscussion은 제안에 대한 토론을 추가합니다.
func (gs *EnhancedGovernanceState) addDiscussion(
	proposalID uint64,
	author common.Address,
	content string,
	timestamp uint64,
	parentIndex int, // -1이면 최상위 토론
) error {
	// 제안 확인
	enhancedProposal, exists := gs.EnhancedProposals[proposalID]
	if !exists {
		return errors.New("enhanced proposal not found")
	}

	// 제안 상태 확인
	if enhancedProposal.Proposal.Status != "voting_period" && enhancedProposal.Proposal.Status != "deposit_period" {
		return errors.New("proposal is not in active state")
	}

	// 토론 항목 생성
	discussion := DiscussionEntry{
		Author:    author,
		Content:   content,
		Timestamp: timestamp,
		ParentIdx: parentIndex,
		Likes:     0,
		Dislikes:  0,
		Replies:   []int{},
	}

	// 토론 추가
	if parentIndex == -1 {
		// 최상위 토론
		enhancedProposal.Discussions = append(enhancedProposal.Discussions, discussion)
	} else {
		// 답글
		if parentIndex >= len(enhancedProposal.Discussions) {
			return errors.New("parent discussion not found")
		}
		
		// 현재 토론 인덱스
		currentIndex := len(enhancedProposal.Discussions)
		
		// 부모 토론에 답글 인덱스 추가
		enhancedProposal.Discussions[parentIndex].Replies = append(
			enhancedProposal.Discussions[parentIndex].Replies, 
			currentIndex,
		)
		
		// 토론 추가
		enhancedProposal.Discussions = append(enhancedProposal.Discussions, discussion)
	}

	log.Info("Discussion added to proposal", "id", proposalID, "author", author)

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
	gs.EnhancedGovernanceStateBase.finalizeProposal(proposalID, blockNumber)

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
		if proposal.Proposal.Type == proposalTypeToString(proposalType) {
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

// GovernanceState에 필요한 메서드 추가
func (gs *EnhancedGovernanceStateBase) submitProposal(
	proposer common.Address,
	title string,
	description string,
	proposalType string,
	deposit *big.Int,
	submitBlock uint64,
	votingPeriod uint64,
) (uint64, error) {
	// 제안 ID 생성
	proposalID := gs.NextProposalID
	gs.NextProposalID++

	// 제안 생성
	proposal := &Proposal{
		ID:          proposalID,
		Type:        proposalType,
		Title:       title,
		Description: description,
		Proposer:    proposer,
		Status:      ProposalStatusActive,
		TotalDeposit: deposit,
		Deposits:    make(map[common.Address]*big.Int),
		YesVotes:    big.NewInt(0),
		NoVotes:     big.NewInt(0),
		AbstainVotes: big.NewInt(0),
		VetoVotes:   big.NewInt(0),
		Votes:       make(map[common.Address]string),
	}

	// 보증금 추가
	proposal.Deposits[proposer] = deposit

	// 제안 저장
	gs.Proposals[proposalID] = proposal

	return proposalID, nil
}

func (gs *EnhancedGovernanceStateBase) getProposal(proposalID uint64) (*Proposal, error) {
	proposal, exists := gs.Proposals[proposalID]
	if !exists {
		return nil, errors.New("proposal not found")
	}
	return proposal, nil
}

func (gs *EnhancedGovernanceStateBase) vote(
	proposalID uint64,
	voter common.Address,
	voteOption string,
	voteWeight *big.Int,
	blockNumber uint64,
) error {
	proposal, exists := gs.Proposals[proposalID]
	if !exists {
		return errors.New("proposal not found")
	}

	// 이전 투표 확인
	prevOption, voted := proposal.Votes[voter]
	if voted {
		// 이전 투표 취소
		switch prevOption {
		case VoteOptionYes:
			proposal.YesVotes = new(big.Int).Sub(proposal.YesVotes, big.NewInt(1))
		case VoteOptionNo:
			proposal.NoVotes = new(big.Int).Sub(proposal.NoVotes, big.NewInt(1))
		case VoteOptionAbstain:
			proposal.AbstainVotes = new(big.Int).Sub(proposal.AbstainVotes, big.NewInt(1))
		case VoteOptionVeto:
			proposal.VetoVotes = new(big.Int).Sub(proposal.VetoVotes, big.NewInt(1))
		}
	}

	// 새 투표 추가
	proposal.Votes[voter] = voteOption
	
	// 투표 집계
	switch voteOption {
	case VoteOptionYes:
		proposal.YesVotes = new(big.Int).Add(proposal.YesVotes, big.NewInt(1))
	case VoteOptionNo:
		proposal.NoVotes = new(big.Int).Add(proposal.NoVotes, big.NewInt(1))
	case VoteOptionAbstain:
		proposal.AbstainVotes = new(big.Int).Add(proposal.AbstainVotes, big.NewInt(1))
	case VoteOptionVeto:
		proposal.VetoVotes = new(big.Int).Add(proposal.VetoVotes, big.NewInt(1))
	default:
		return errors.New("invalid vote option")
	}

	return nil
}

func (gs *EnhancedGovernanceStateBase) finalizeProposal(proposalID uint64, blockNumber uint64) error {
	proposal, exists := gs.Proposals[proposalID]
	if !exists {
		return errors.New("proposal not found")
	}

	// 제안 상태 업데이트
	proposal.Status = "executed"

	return nil
}
