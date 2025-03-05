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

	"github.com/zenanetwork/go-zenanet/common"
	core "github.com/zenanetwork/go-zenanet/consensus/eirene/core"
	"github.com/zenanetwork/go-zenanet/core/rawdb"
	"github.com/zenanetwork/go-zenanet/params"
)

// 테스트용 주소 생성
var (
	testProposer  = common.HexToAddress("0x1111111111111111111111111111111111111111")
	testVoter1    = common.HexToAddress("0x2222222222222222222222222222222222222222")
	testVoter2    = common.HexToAddress("0x3333333333333333333333333333333333333333")
	testVoter3    = common.HexToAddress("0x4444444444444444444444444444444444444444")
	testRecipient = common.HexToAddress("0x5555555555555555555555555555555555555555")
)

// TestGovernanceState는 거버넌스 상태 관리를 테스트합니다.
func TestGovernanceState(t *testing.T) {
	// 새로운 거버넌스 상태 생성
	gs := newGovernanceState()

	// 초기 상태 확인
	if gs.NextProposalID != 1 {
		t.Errorf("초기 NextProposalID가 1이 아님: %d", gs.NextProposalID)
	}

	if len(gs.Proposals) != 0 {
		t.Errorf("초기 Proposals가 비어있지 않음: %d", len(gs.Proposals))
	}

	if len(gs.Votes) != 0 {
		t.Errorf("초기 Votes가 비어있지 않음: %d", len(gs.Votes))
	}
}

// TestSubmitProposal은 제안 제출 기능을 테스트합니다.
func TestSubmitProposal(t *testing.T) {
	// 새로운 거버넌스 상태 생성
	gs := newGovernanceState()

	// 제안 제출
	title := "테스트 제안"
	description := "이것은 테스트 제안입니다."
	parameters := map[string]string{
		"votingPeriod": "50000",
	}
	deposit := new(big.Int).Mul(
		big.NewInt(100),
		new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil),
	)
	currentBlock := uint64(1000)

	proposalID, err := gs.submitProposal(
		testProposer,
		title,
		description,
		ProposalTypeParameter,
		parameters,
		nil,
		nil,
		deposit,
		currentBlock,
	)

	if err != nil {
		t.Fatalf("제안 제출 실패: %v", err)
	}

	if proposalID != 1 {
		t.Errorf("제안 ID가 1이 아님: %d", proposalID)
	}

	// 제안 확인
	proposal, err := gs.getProposal(proposalID)
	if err != nil {
		t.Fatalf("제안 조회 실패: %v", err)
	}

	if proposal.Title != title {
		t.Errorf("제안 제목이 일치하지 않음: %s != %s", proposal.Title, title)
	}

	if proposal.Description != description {
		t.Errorf("제안 설명이 일치하지 않음: %s != %s", proposal.Description, description)
	}

	if proposal.Type != ProposalTypeParameter {
		t.Errorf("제안 유형이 일치하지 않음: %d != %d", proposal.Type, ProposalTypeParameter)
	}

	if proposal.Status != ProposalStatusPending {
		t.Errorf("제안 상태가 대기 중이 아님: %d", proposal.Status)
	}

	if proposal.SubmitBlock != currentBlock {
		t.Errorf("제안 제출 블록이 일치하지 않음: %d != %d", proposal.SubmitBlock, currentBlock)
	}

	if proposal.VotingStartBlock != currentBlock+gs.MinProposalAge {
		t.Errorf("투표 시작 블록이 일치하지 않음: %d != %d", proposal.VotingStartBlock, currentBlock+gs.MinProposalAge)
	}

	if proposal.VotingEndBlock != currentBlock+gs.MinProposalAge+gs.VotingPeriod {
		t.Errorf("투표 종료 블록이 일치하지 않음: %d != %d", proposal.VotingEndBlock, currentBlock+gs.MinProposalAge+gs.VotingPeriod)
	}
}

// TestVote는 투표 기능을 테스트합니다.
func TestVote(t *testing.T) {
	// 새로운 거버넌스 상태 생성
	gs := newGovernanceState()

	// 제안 제출
	title := "테스트 제안"
	description := "이것은 테스트 제안입니다."
	parameters := map[string]string{
		"votingPeriod": "50000",
	}
	deposit := new(big.Int).Mul(
		big.NewInt(100),
		new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil),
	)
	currentBlock := uint64(1000)

	proposalID, err := gs.submitProposal(
		testProposer,
		title,
		description,
		ProposalTypeParameter,
		parameters,
		nil,
		nil,
		deposit,
		currentBlock,
	)

	if err != nil {
		t.Fatalf("제안 제출 실패: %v", err)
	}

	// 제안 활성화
	proposal := gs.Proposals[proposalID]
	proposal.Status = ProposalStatusActive

	// 투표
	weight := new(big.Int).Mul(
		big.NewInt(10),
		new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil),
	)

	err = gs.vote(
		proposalID,
		testVoter1,
		VoteYes,
		weight,
		currentBlock+gs.MinProposalAge+1,
	)

	if err != nil {
		t.Fatalf("투표 실패: %v", err)
	}

	// 투표 확인
	votes, err := gs.getVotes(proposalID)
	if err != nil {
		t.Fatalf("투표 조회 실패: %v", err)
	}

	if len(votes) != 1 {
		t.Errorf("투표 수가 1이 아님: %d", len(votes))
	}

	if votes[0].Voter != testVoter1 {
		t.Errorf("투표자가 일치하지 않음: %s != %s", votes[0].Voter.Hex(), testVoter1.Hex())
	}

	if votes[0].Option != VoteYes {
		t.Errorf("투표 옵션이 일치하지 않음: %d != %d", votes[0].Option, VoteYes)
	}

	if votes[0].Weight.Cmp(weight) != 0 {
		t.Errorf("투표 가중치가 일치하지 않음: %s != %s", votes[0].Weight.String(), weight.String())
	}

	// 제안 투표 결과 확인
	if proposal.YesVotes.Cmp(weight) != 0 {
		t.Errorf("찬성 투표 수가 일치하지 않음: %s != %s", proposal.YesVotes.String(), weight.String())
	}

	if proposal.TotalVotes.Cmp(weight) != 0 {
		t.Errorf("총 투표 수가 일치하지 않음: %s != %s", proposal.TotalVotes.String(), weight.String())
	}
}

// TestGovernanceAPI는 거버넌스 API를 테스트합니다.
func TestGovernanceAPI(t *testing.T) {
	// 새로운 Eirene 엔진 생성
	db := rawdb.NewMemoryDatabase()
	config := params.EireneConfig{
		Period: 15,
		Epoch:  30000,
	}
	eirene := core.New(&config, db)

	// 거버넌스 API 생성
	api := core.NewGovernanceAPI(nil, eirene)

	// 거버넌스 매개변수 확인
	params := api.GetGovernanceParams()

	if params["votingPeriod"].(uint64) != DefaultVotingPeriod {
		t.Errorf("투표 기간이 일치하지 않음: %d != %d", params["votingPeriod"], DefaultVotingPeriod)
	}

	if params["quorum"].(uint8) != DefaultQuorum {
		t.Errorf("쿼럼이 일치하지 않음: %d != %d", params["quorum"], DefaultQuorum)
	}

	if params["threshold"].(uint8) != DefaultThreshold {
		t.Errorf("임계값이 일치하지 않음: %d != %d", params["threshold"], DefaultThreshold)
	}
}

// TestGovernanceStorage는 거버넌스 상태 저장 및 로드를 테스트합니다.
func TestGovernanceStorage(t *testing.T) {
	// 새로운 데이터베이스 생성
	db := rawdb.NewMemoryDatabase()

	// 새로운 거버넌스 상태 생성
	gs := newGovernanceState()

	// 제안 제출
	title := "테스트 제안"
	description := "이것은 테스트 제안입니다."
	parameters := map[string]string{
		"votingPeriod": "50000",
	}
	deposit := new(big.Int).Mul(
		big.NewInt(100),
		new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil),
	)
	currentBlock := uint64(1000)

	_, err := gs.submitProposal(
		testProposer,
		title,
		description,
		ProposalTypeParameter,
		parameters,
		nil,
		nil,
		deposit,
		currentBlock,
	)

	if err != nil {
		t.Fatalf("제안 제출 실패: %v", err)
	}

	// 거버넌스 상태 저장
	err = gs.store(db)
	if err != nil {
		t.Fatalf("거버넌스 상태 저장 실패: %v", err)
	}

	// 거버넌스 상태 로드
	gs2, err := loadGovernanceState(db)
	if err != nil {
		t.Fatalf("거버넌스 상태 로드 실패: %v", err)
	}

	// 로드된 상태 확인
	if gs2.NextProposalID != 2 {
		t.Errorf("NextProposalID가 일치하지 않음: %d != 2", gs2.NextProposalID)
	}

	if len(gs2.Proposals) != 1 {
		t.Errorf("Proposals 수가 일치하지 않음: %d != 1", len(gs2.Proposals))
	}

	proposal := gs2.Proposals[1]
	if proposal.Title != title {
		t.Errorf("제안 제목이 일치하지 않음: %s != %s", proposal.Title, title)
	}

	if proposal.Description != description {
		t.Errorf("제안 설명이 일치하지 않음: %s != %s", proposal.Description, description)
	}
}
