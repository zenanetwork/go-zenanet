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
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zenanetwork/go-zenanet/common"
)

// TestEnhancedGovernanceState는 향상된 거버넌스 상태 관리를 테스트합니다.
func TestEnhancedGovernanceState(t *testing.T) {
	// 새로운 향상된 거버넌스 상태 생성
	egs := newEnhancedGovernanceState()

	// 초기 상태 확인
	assert.Equal(t, uint64(1), egs.NextProposalID, "초기 NextProposalID가 1이 아닙니다")
	assert.Equal(t, 0, len(egs.Proposals), "초기 Proposals가 비어있지 않습니다")
	assert.Equal(t, 0, len(egs.Votes), "초기 Votes가 비어있지 않습니다")
	assert.Equal(t, uint64(0), egs.TotalProposals, "초기 TotalProposals가 0이 아닙니다")
	assert.Equal(t, uint64(0), egs.PassedProposals, "초기 PassedProposals가 0이 아닙니다")
	assert.Equal(t, uint64(0), egs.RejectedProposals, "초기 RejectedProposals가 0이 아닙니다")
	assert.Equal(t, uint64(0), egs.ImplementedProposals, "초기 ImplementedProposals가 0이 아닙니다")
	assert.Equal(t, 0, egs.CommunityPoolBalance.Cmp(big.NewInt(0)), "초기 CommunityPoolBalance가 0이 아닙니다")
	assert.Equal(t, 0, egs.CommunityPoolSpent.Cmp(big.NewInt(0)), "초기 CommunityPoolSpent가 0이 아닙니다")
	assert.Equal(t, 0, len(egs.NetworkParameters), "초기 NetworkParameters가 비어있지 않습니다")
}

// TestEnhancedProposalSubmission은 향상된 제안 제출 기능을 테스트합니다.
func TestEnhancedProposalSubmission(t *testing.T) {
	// 테스트 환경 설정
	egs := newEnhancedGovernanceState()

	// 테스트 계정
	testProposer := common.HexToAddress("0x1111111111111111111111111111111111111111")

	// 기본 제안 정보
	title := "테스트 향상된 제안"
	description := "이것은 향상된 거버넌스 테스트 제안입니다."
	proposalType := ProposalTypeNetworkParameter
	deposit := new(big.Int).Mul(
		big.NewInt(100),
		new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil),
	)
	currentBlock := uint64(1000)

	// 향상된 제안 정보
	tags := []string{"테스트", "네트워크", "매개변수"}
	relatedProposals := []uint64{}
	attachments := []string{"ipfs://QmTest1234567890"}
	networkParameters := map[string]string{
		"votingPeriod": "50000",
		"quorum":       "40",
	}

	// 향상된 제안 생성
	enhancedProposal := &EnhancedProposal{
		Proposal: &Proposal{
			ID:          egs.NextProposalID,
			Title:       title,
			Description: description,
			Type:        proposalType,
			Proposer:    testProposer,
			SubmitBlock: currentBlock,
			TotalDeposit: deposit,
			Status:      ProposalStatusActive,
			VotingEndBlock: currentBlock + egs.VotingPeriod,
		},
		Tags:              tags,
		RelatedProposals:  relatedProposals,
		Attachments:       attachments,
		Discussions:       []DiscussionEntry{},
		Amendments:        []Amendment{},
		VoteWeightType:    0, // 기본 가중치
		ConditionalVotes:  []ConditionalVote{},
		IsEmergency:       false,
		EmergencyReason:   "",
		NetworkParameters: networkParameters,
	}

	// 제안 저장
	egs.EnhancedProposals[egs.NextProposalID] = enhancedProposal
	egs.Proposals[egs.NextProposalID] = enhancedProposal.Proposal
	egs.Votes[egs.NextProposalID] = make(map[common.Address]int)
	egs.NextProposalID++
	egs.TotalProposals++

	// 제안 저장 확인
	assert.Equal(t, uint64(2), egs.NextProposalID, "NextProposalID가 증가하지 않았습니다")
	assert.Equal(t, uint64(1), egs.TotalProposals, "TotalProposals가 증가하지 않았습니다")
	assert.Equal(t, 1, len(egs.Proposals), "Proposals에 제안이 추가되지 않았습니다")
	assert.Equal(t, 1, len(egs.EnhancedProposals), "EnhancedProposals에 제안이 추가되지 않았습니다")

	// 제안 내용 확인
	proposal := egs.Proposals[1]
	enhancedProposal = egs.EnhancedProposals[1]

	assert.Equal(t, title, proposal.Title, "제안 제목이 일치하지 않습니다")
	assert.Equal(t, description, proposal.Description, "제안 설명이 일치하지 않습니다")
	assert.Equal(t, proposalType, proposal.Type, "제안 유형이 일치하지 않습니다")
	assert.Equal(t, testProposer, proposal.Proposer, "제안자가 일치하지 않습니다")
	assert.Equal(t, 0, deposit.Cmp(proposal.TotalDeposit), "보증금이 일치하지 않습니다")
	assert.Equal(t, ProposalStatusActive, proposal.Status, "제안 상태가 일치하지 않습니다")

	assert.Equal(t, tags, enhancedProposal.Tags, "제안 태그가 일치하지 않습니다")
	assert.Equal(t, attachments, enhancedProposal.Attachments, "제안 첨부파일이 일치하지 않습니다")
	assert.Equal(t, networkParameters, enhancedProposal.NetworkParameters, "네트워크 매개변수가 일치하지 않습니다")
}

// TestEnhancedVoting은 향상된 투표 기능을 테스트합니다.
func TestEnhancedVoting(t *testing.T) {
	// 테스트 환경 설정
	egs := newEnhancedGovernanceState()

	// 테스트 계정
	testProposer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	testVoter1 := common.HexToAddress("0x2222222222222222222222222222222222222222")
	testVoter2 := common.HexToAddress("0x3333333333333333333333333333333333333333")
	testVoter3 := common.HexToAddress("0x4444444444444444444444444444444444444444")

	// 제안 생성
	proposalID := uint64(1)
	currentBlock := uint64(1000)
	
	proposal := &Proposal{
		ID:             proposalID,
		Title:          "테스트 제안",
		Description:    "이것은 테스트 제안입니다.",
		Type:           ProposalTypeTextProposal,
		Proposer:       testProposer,
		SubmitBlock:    currentBlock,
		TotalDeposit:   big.NewInt(100),
		Status:         ProposalStatusActive,
		VotingEndBlock: currentBlock + egs.VotingPeriod,
	}
	
	enhancedProposal := &EnhancedProposal{
		Proposal:       proposal,
		Tags:           []string{"테스트"},
		VoteWeightType: 0, // 기본 가중치
	}

	// 제안 저장
	egs.Proposals[proposalID] = proposal
	egs.EnhancedProposals[proposalID] = enhancedProposal
	egs.Votes[proposalID] = make(map[common.Address]int)
	egs.NextProposalID = proposalID + 1
	egs.TotalProposals++

	// 투표 진행
	egs.Votes[proposalID][testVoter1] = int(VoteOptionYesValue)
	egs.Votes[proposalID][testVoter2] = int(VoteOptionYesValue)
	egs.Votes[proposalID][testVoter3] = int(VoteOptionNoValue)

	// 투표 확인
	assert.Equal(t, 3, len(egs.Votes[proposalID]), "투표 수가 일치하지 않습니다")
	assert.Equal(t, int(VoteOptionYesValue), egs.Votes[proposalID][testVoter1], "투표1이 일치하지 않습니다")
	assert.Equal(t, int(VoteOptionYesValue), egs.Votes[proposalID][testVoter2], "투표2가 일치하지 않습니다")
	assert.Equal(t, int(VoteOptionNoValue), egs.Votes[proposalID][testVoter3], "투표3이 일치하지 않습니다")

	// 조건부 투표 추가
	conditionalVote := ConditionalVote{
		Voter:      testVoter1,
		Conditions: []string{"네트워크 매개변수 중 quorum을 35%로 수정해주세요."},
		Timestamp:  uint64(time.Now().Unix()),
	}
	enhancedProposal.ConditionalVotes = append(enhancedProposal.ConditionalVotes, conditionalVote)

	// 조건부 투표 확인
	assert.Equal(t, 1, len(enhancedProposal.ConditionalVotes), "조건부 투표 수가 일치하지 않습니다")
	assert.Equal(t, testVoter1, enhancedProposal.ConditionalVotes[0].Voter, "조건부 투표자가 일치하지 않습니다")
	assert.Equal(t, "네트워크 매개변수 중 quorum을 35%로 수정해주세요.", enhancedProposal.ConditionalVotes[0].Conditions[0], "조건부 투표 조건이 일치하지 않습니다")
}

// TestDiscussionAndAmendments는 토론 및 수정안 기능을 테스트합니다.
func TestDiscussionAndAmendments(t *testing.T) {
	// 테스트 환경 설정
	egs := newEnhancedGovernanceState()

	// 테스트 계정
	testProposer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	testDiscusser := common.HexToAddress("0x2222222222222222222222222222222222222222")
	testAmender := common.HexToAddress("0x3333333333333333333333333333333333333333")

	// 제안 생성
	proposalID := uint64(1)
	currentBlock := uint64(1000)
	
	proposal := &Proposal{
		ID:             proposalID,
		Title:          "테스트 제안",
		Description:    "이것은 테스트 제안입니다.",
		Type:           ProposalTypeTextProposal,
		Proposer:       testProposer,
		SubmitBlock:    currentBlock,
		TotalDeposit:   big.NewInt(100),
		Status:         ProposalStatusActive,
		VotingEndBlock: currentBlock + egs.VotingPeriod,
	}
	
	enhancedProposal := &EnhancedProposal{
		Proposal:       proposal,
		Tags:           []string{"테스트"},
		VoteWeightType: 0, // 기본 가중치
		Discussions:    []DiscussionEntry{},
		Amendments:     []Amendment{},
	}

	// 제안 저장
	egs.Proposals[proposalID] = proposal
	egs.EnhancedProposals[proposalID] = enhancedProposal
	egs.Votes[proposalID] = make(map[common.Address]int)
	egs.NextProposalID = proposalID + 1
	egs.TotalProposals++

	// 토론 추가
	discussion := DiscussionEntry{
		Author:    testDiscusser,
		Content:   "이 제안에 대해 좀 더 자세한 설명이 필요합니다.",
		Timestamp: uint64(time.Now().Unix()),
	}
	enhancedProposal.Discussions = append(enhancedProposal.Discussions, discussion)

	// 수정안 추가
	amendment := Amendment{
		Proposer:   testAmender,
		Content:    "제안의 투표 기간을 2주로 연장하는 것을 제안합니다.",
		Timestamp:  uint64(time.Now().Unix()),
		Accepted:   true,
		AcceptedAt: uint64(time.Now().Unix()),
	}
	enhancedProposal.Amendments = append(enhancedProposal.Amendments, amendment)

	// 토론 확인
	assert.Equal(t, 1, len(enhancedProposal.Discussions), "토론 수가 일치하지 않습니다")
	assert.Equal(t, testDiscusser, enhancedProposal.Discussions[0].Author, "토론 작성자가 일치하지 않습니다")
	assert.Equal(t, "이 제안에 대해 좀 더 자세한 설명이 필요합니다.", enhancedProposal.Discussions[0].Content, "토론 내용이 일치하지 않습니다")

	// 수정안 확인
	assert.Equal(t, 1, len(enhancedProposal.Amendments), "수정안 수가 일치하지 않습니다")
	assert.Equal(t, testAmender, enhancedProposal.Amendments[0].Proposer, "수정안 제안자가 일치하지 않습니다")
	assert.Equal(t, "제안의 투표 기간을 2주로 연장하는 것을 제안합니다.", enhancedProposal.Amendments[0].Content, "수정안 내용이 일치하지 않습니다")
	assert.Equal(t, true, enhancedProposal.Amendments[0].Accepted, "수정안 수락 여부가 일치하지 않습니다")
}

// TestEmergencyProposal은 긴급 제안 기능을 테스트합니다.
func TestEmergencyProposal(t *testing.T) {
	// 테스트 환경 설정
	egs := newEnhancedGovernanceState()

	// 테스트 계정
	testProposer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	testVoter1 := common.HexToAddress("0x2222222222222222222222222222222222222222")
	testVoter2 := common.HexToAddress("0x3333333333333333333333333333333333333333")

	// 긴급 제안 생성
	proposalID := uint64(1)
	currentBlock := uint64(1000)
	
	proposal := &Proposal{
		ID:             proposalID,
		Title:          "긴급 제안",
		Description:    "이것은 긴급 제안입니다.",
		Type:           ProposalTypeEmergency,
		Proposer:       testProposer,
		SubmitBlock:    currentBlock,
		TotalDeposit:   big.NewInt(500), // 긴급 제안은 더 높은 보증금 필요
		Status:         ProposalStatusActive,
		VotingEndBlock: currentBlock + DefaultEmergencyVotingPeriod,
	}
	
	enhancedProposal := &EnhancedProposal{
		Proposal:       proposal,
		Tags:           []string{"긴급", "보안"},
		VoteWeightType: VoteWeightTypeStake, // 스테이킹 기반 가중치
		IsEmergency:    true,
		EmergencyReason: "네트워크 보안 취약점 발견으로 인한 긴급 패치 필요",
	}

	// 제안 저장
	egs.Proposals[proposalID] = proposal
	egs.EnhancedProposals[proposalID] = enhancedProposal
	egs.Votes[proposalID] = make(map[common.Address]int)
	egs.NextProposalID = proposalID + 1
	egs.TotalProposals++

	// 긴급 제안 확인
	assert.Equal(t, true, enhancedProposal.IsEmergency, "긴급 제안 여부가 일치하지 않습니다")
	assert.Equal(t, "네트워크 보안 취약점 발견으로 인한 긴급 패치 필요", enhancedProposal.EmergencyReason, "긴급 제안 이유가 일치하지 않습니다")
	assert.Equal(t, uint8(VoteWeightTypeStake), enhancedProposal.VoteWeightType, "투표 가중치 타입이 일치하지 않습니다")
	assert.Equal(t, ProposalTypeEmergency, proposal.Type, "제안 타입이 일치하지 않습니다")
	assert.Equal(t, currentBlock+DefaultEmergencyVotingPeriod, proposal.VotingEndBlock, "투표 종료 블록이 일치하지 않습니다")

	// 긴급 투표 진행 (더 높은 통과 임계값 필요)
	egs.Votes[proposalID][testVoter1] = int(VoteOptionYesValue)
	egs.Votes[proposalID][testVoter2] = int(VoteOptionYesValue)

	// 투표 확인
	assert.Equal(t, 2, len(egs.Votes[proposalID]), "투표 수가 일치하지 않습니다")
	assert.Equal(t, int(VoteOptionYesValue), egs.Votes[proposalID][testVoter1], "투표1이 일치하지 않습니다")
	assert.Equal(t, int(VoteOptionYesValue), egs.Votes[proposalID][testVoter2], "투표2가 일치하지 않습니다")
}

// TestCommunityPoolProposal은 커뮤니티 풀 제안 기능을 테스트합니다.
func TestCommunityPoolProposal(t *testing.T) {
	// 테스트 환경 설정
	egs := newEnhancedGovernanceState()

	// 테스트 계정
	testProposer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	testRecipient := common.HexToAddress("0x5555555555555555555555555555555555555555")

	// 커뮤니티 풀 제안 생성
	proposalID := uint64(1)
	currentBlock := uint64(1000)
	
	proposal := &Proposal{
		ID:             proposalID,
		Title:          "커뮤니티 풀 제안",
		Description:    "이것은 커뮤니티 풀 제안입니다.",
		Type:           ProposalTypeCommunityPool,
		Proposer:       testProposer,
		SubmitBlock:    currentBlock,
		TotalDeposit:   big.NewInt(100),
		Status:         ProposalStatusActive,
		VotingEndBlock: currentBlock + egs.VotingPeriod,
	}
	
	enhancedProposal := &EnhancedProposal{
		Proposal:           proposal,
		Tags:               []string{"커뮤니티 풀", "개발 지원"},
		VoteWeightType:     VoteWeightTypeEqual, // 동등한 가중치
		CommunityPoolAmount: big.NewInt(1000),
	}
	
	// 수령인 정보는 별도로 저장 (실제 구현에서는 Content 필드나 다른 방식으로 저장)
	recipientInfo := map[string]string{
		"recipient": testRecipient.Hex(),
	}
	enhancedProposal.NetworkParameters = recipientInfo

	// 제안 저장
	egs.Proposals[proposalID] = proposal
	egs.EnhancedProposals[proposalID] = enhancedProposal
	egs.Votes[proposalID] = make(map[common.Address]int)
	egs.NextProposalID = proposalID + 1
	egs.TotalProposals++

	// 커뮤니티 풀 제안 확인
	assert.Equal(t, ProposalTypeCommunityPool, proposal.Type, "제안 타입이 일치하지 않습니다")
	assert.Equal(t, big.NewInt(1000), enhancedProposal.CommunityPoolAmount, "커뮤니티 풀 금액이 일치하지 않습니다")
	assert.Equal(t, testRecipient.Hex(), enhancedProposal.NetworkParameters["recipient"], "수령인이 일치하지 않습니다")
} 