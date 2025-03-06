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
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/rawdb"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/params"
)

// MockChainReader는 테스트를 위한 ChainHeaderReader 인터페이스 모의 구현체입니다.
type MockChainReader struct {
	currentHeader *types.Header
	currentBlock  *types.Block
}

func (m *MockChainReader) CurrentHeader() *types.Header {
	return m.currentHeader
}

func (m *MockChainReader) GetHeaderByNumber(number uint64) *types.Header {
	if m.currentHeader != nil && m.currentHeader.Number.Uint64() == number {
		return m.currentHeader
	}
	return nil
}

func (m *MockChainReader) GetHeaderByHash(hash common.Hash) *types.Header {
	if m.currentHeader != nil && m.currentHeader.Hash() == hash {
		return m.currentHeader
	}
	return nil
}

func (m *MockChainReader) GetHeader(hash common.Hash, number uint64) *types.Header {
	if m.currentHeader != nil && m.currentHeader.Hash() == hash && m.currentHeader.Number.Uint64() == number {
		return m.currentHeader
	}
	return nil
}

func (m *MockChainReader) Config() *params.ChainConfig {
	return &params.ChainConfig{}
}

// setupTestAPI는 테스트를 위한 API 인스턴스를 설정합니다.
func setupTestAPI(t *testing.T) (*API, *GovernanceManager, *state.StateDB, ethdb.Database) {
	// 메모리 데이터베이스 생성
	db := rawdb.NewMemoryDatabase()
	
	// 상태 DB 생성
	stateDB, _ := state.New(common.Hash{}, state.NewDatabaseForTesting())
	
	// 검증자 집합 생성
	validatorSet := NewMockValidatorSet()
	
	// 검증자 추가
	addr1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
	addr2 := common.HexToAddress("0x2222222222222222222222222222222222222222")
	addr3 := common.HexToAddress("0x3333333333333333333333333333333333333333")
	
	validatorSet.AddValidator(addr1)
	validatorSet.AddValidator(addr2)
	validatorSet.AddValidator(addr3)
	
	// 거버넌스 매개변수 생성
	params := NewDefaultGovernanceParams()
	
	// 거버넌스 매니저 생성
	govManager := NewGovernanceManager(params, validatorSet)
	
	// 현재 블록 헤더 생성
	header := &types.Header{
		Number:     big.NewInt(100),
		Time:       uint64(time.Now().Unix()),
		Difficulty: big.NewInt(1),
		GasLimit:   8000000,
	}
	
	// 체인 리더 생성
	chainReader := &MockChainReader{
		currentHeader: header,
		currentBlock:  types.NewBlockWithHeader(header),
	}
	
	// API 생성
	api := NewAPI(
		chainReader,
		govManager,
		func(hash common.Hash) (*state.StateDB, error) { return stateDB, nil },
		func() *types.Block { return chainReader.currentBlock },
	)
	
	return api, govManager, stateDB, db
}

// TestGetProposals는 GetProposals API 메서드를 테스트합니다.
func TestGetProposals(t *testing.T) {
	// 테스트 API 설정
	api, _, _, _ := setupTestAPI(t)
	
	// 초기 상태에서는 제안이 없어야 함
	proposals, err := api.GetProposals()
	assert.NoError(t, err, "GetProposals should not return an error")
	assert.Equal(t, 0, len(proposals), "Initial proposals list should be empty")
	
	// 테스트 제안 생성
	proposer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	
	// 제안 제출
	args := SubmitProposalArgs{
		Type:           utils.ProposalTypeText,
		Title:          "Test Proposal",
		Description:    "This is a test proposal",
		Proposer:       proposer,
		InitialDeposit: "100",
		Content: &TestProposalContent{
			ProposalType: utils.ProposalTypeText,
			Params:       make(map[string]string),
		},
	}
	
	proposalID, err := api.SubmitProposal(args)
	assert.NoError(t, err, "SubmitProposal should not return an error")
	assert.Equal(t, uint64(1), proposalID, "First proposal ID should be 1")
	
	// 제안 목록 다시 조회
	proposals, err = api.GetProposals()
	assert.NoError(t, err, "GetProposals should not return an error")
	assert.Equal(t, 1, len(proposals), "Should have 1 proposal after submission")
	assert.Equal(t, "Test Proposal", proposals[0].Title, "Proposal title should match")
}

// TestGetProposal은 GetProposal API 메서드를 테스트합니다.
func TestGetProposal(t *testing.T) {
	// 테스트 API 설정
	api, _, _, _ := setupTestAPI(t)
	
	// 존재하지 않는 제안 조회
	proposal, err := api.GetProposal(1)
	assert.Error(t, err, "GetProposal should return an error for non-existent proposal")
	assert.Nil(t, proposal, "Proposal should be nil for non-existent proposal")
	
	// 테스트 제안 생성
	proposer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	
	// 제안 제출
	args := SubmitProposalArgs{
		Type:           utils.ProposalTypeText,
		Title:          "Test Proposal",
		Description:    "This is a test proposal",
		Proposer:       proposer,
		InitialDeposit: "100",
		Content: &TestProposalContent{
			ProposalType: utils.ProposalTypeText,
			Params:       make(map[string]string),
		},
	}
	
	proposalID, err := api.SubmitProposal(args)
	assert.NoError(t, err, "SubmitProposal should not return an error")
	
	// 제안 조회
	proposal, err = api.GetProposal(proposalID)
	assert.NoError(t, err, "GetProposal should not return an error")
	assert.NotNil(t, proposal, "Proposal should not be nil")
	assert.Equal(t, "Test Proposal", proposal.Title, "Proposal title should match")
	assert.Equal(t, "This is a test proposal", proposal.Description, "Proposal description should match")
	assert.Equal(t, proposer, proposal.Proposer, "Proposal proposer should match")
}

// TestSubmitProposal은 SubmitProposal API 메서드를 테스트합니다.
func TestSubmitProposal(t *testing.T) {
	// 테스트 API 설정
	api, _, _, _ := setupTestAPI(t)
	
	// 유효한 제안 제출
	proposer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	args := SubmitProposalArgs{
		Type:           utils.ProposalTypeText,
		Title:          "Test Proposal",
		Description:    "This is a test proposal",
		Proposer:       proposer,
		InitialDeposit: "100",
		Content: &TestProposalContent{
			ProposalType: utils.ProposalTypeText,
			Params:       make(map[string]string),
		},
	}
	
	proposalID, err := api.SubmitProposal(args)
	assert.NoError(t, err, "SubmitProposal should not return an error")
	assert.Equal(t, uint64(1), proposalID, "First proposal ID should be 1")
	
	// 잘못된 초기 보증금으로 제안 제출
	args.InitialDeposit = "0"
	_, err = api.SubmitProposal(args)
	assert.Error(t, err, "SubmitProposal should return an error for zero initial deposit")
	
	// 잘못된 제안 유형으로 제안 제출
	args.InitialDeposit = "100"
	args.Type = "invalid_type"
	_, err = api.SubmitProposal(args)
	assert.Error(t, err, "SubmitProposal should return an error for invalid proposal type")
}

// TestVote는 Vote API 메서드를 테스트합니다.
func TestVote(t *testing.T) {
	// 테스트 API 설정
	api, govManager, _, _ := setupTestAPI(t)
	
	// 테스트 제안 생성
	proposer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	args := SubmitProposalArgs{
		Type:           utils.ProposalTypeText,
		Title:          "Test Proposal",
		Description:    "This is a test proposal",
		Proposer:       proposer,
		InitialDeposit: "100",
		Content: &TestProposalContent{
			ProposalType: utils.ProposalTypeText,
			Params:       make(map[string]string),
		},
	}
	
	proposalID, err := api.SubmitProposal(args)
	assert.NoError(t, err, "SubmitProposal should not return an error")
	
	// 제안 상태를 투표 기간으로 변경
	proposal, _ := govManager.GetProposal(proposalID)
	proposal.Status = utils.ProposalStatusVotingPeriod
	
	// 유효한 투표
	voter := common.HexToAddress("0x2222222222222222222222222222222222222222")
	voteArgs := VoteArgs{
		ProposalID: proposalID,
		Voter:      voter,
		Option:     utils.VoteOptionYes,
	}
	
	success, err := api.Vote(voteArgs)
	assert.NoError(t, err, "Vote should not return an error")
	assert.True(t, success, "Vote should be successful")
	
	// 이미 투표한 검증자의 투표
	_, err = api.Vote(voteArgs)
	assert.Error(t, err, "Vote should return an error for already voted validator")
	
	// 잘못된 투표 옵션
	voteArgs.Voter = common.HexToAddress("0x3333333333333333333333333333333333333333")
	voteArgs.Option = "invalid_option"
	_, err = api.Vote(voteArgs)
	assert.Error(t, err, "Vote should return an error for invalid vote option")
}

// TestDeposit는 Deposit API 메서드를 테스트합니다.
func TestDeposit(t *testing.T) {
	// 테스트 API 설정
	api, _, _, _ := setupTestAPI(t)
	
	// 테스트 제안 생성
	proposer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	args := SubmitProposalArgs{
		Type:           utils.ProposalTypeText,
		Title:          "Test Proposal",
		Description:    "This is a test proposal",
		Proposer:       proposer,
		InitialDeposit: "50", // 최소 보증금보다 작게 설정
		Content: &TestProposalContent{
			ProposalType: utils.ProposalTypeText,
			Params:       make(map[string]string),
		},
	}
	
	proposalID, err := api.SubmitProposal(args)
	assert.NoError(t, err, "SubmitProposal should not return an error")
	
	// 유효한 보증금 예치
	depositor := common.HexToAddress("0x2222222222222222222222222222222222222222")
	depositArgs := DepositArgs{
		ProposalID: proposalID,
		Depositor:  depositor,
		Amount:     "50",
	}
	
	success, err := api.Deposit(depositArgs)
	assert.NoError(t, err, "Deposit should not return an error")
	assert.True(t, success, "Deposit should be successful")
	
	// 잘못된 금액으로 보증금 예치
	depositArgs.Amount = "0"
	_, err = api.Deposit(depositArgs)
	assert.Error(t, err, "Deposit should return an error for zero amount")
	
	// 존재하지 않는 제안에 보증금 예치
	depositArgs.Amount = "50"
	depositArgs.ProposalID = 999
	_, err = api.Deposit(depositArgs)
	assert.Error(t, err, "Deposit should return an error for non-existent proposal")
}

// TestGetParams는 GetParams API 메서드를 테스트합니다.
func TestGetParams(t *testing.T) {
	// 테스트 API 설정
	api, _, _, _ := setupTestAPI(t)
	
	// 거버넌스 매개변수 조회
	params := api.GetParams()
	assert.NotNil(t, params, "GetParams should not return nil")
	assert.Contains(t, params, "min_deposit", "Params should contain min_deposit")
	assert.Contains(t, params, "deposit_period", "Params should contain deposit_period")
	assert.Contains(t, params, "voting_period", "Params should contain voting_period")
	assert.Contains(t, params, "quorum", "Params should contain quorum")
	assert.Contains(t, params, "threshold", "Params should contain threshold")
	assert.Contains(t, params, "veto_threshold", "Params should contain veto_threshold")
	assert.Contains(t, params, "execution_delay", "Params should contain execution_delay")
}

// TestSetParams는 SetParams API 메서드를 테스트합니다.
func TestSetParams(t *testing.T) {
	// 테스트 API 설정
	api, _, _, _ := setupTestAPI(t)
	
	// 유효한 매개변수 설정
	args := SetParamsArgs{
		MinDeposit:     "200",
		DepositPeriod:  72 * 3600, // 72시간
		VotingPeriod:   7 * 24 * 3600, // 7일
		Quorum:         0.4,
		Threshold:      0.6,
		VetoThreshold:  0.334,
		ExecutionDelay: 24 * 3600, // 24시간
	}
	
	success, err := api.SetParams(args)
	assert.NoError(t, err, "SetParams should not return an error")
	assert.True(t, success, "SetParams should be successful")
	
	// 매개변수 확인
	params := api.GetParams()
	assert.Equal(t, "200", params["min_deposit"], "min_deposit should be updated")
	assert.Equal(t, uint64(72*3600), params["deposit_period"], "deposit_period should be updated")
	assert.Equal(t, uint64(7*24*3600), params["voting_period"], "voting_period should be updated")
	assert.Equal(t, 0.4, params["quorum"], "quorum should be updated")
	assert.Equal(t, 0.6, params["threshold"], "threshold should be updated")
	assert.Equal(t, 0.334, params["veto_threshold"], "veto_threshold should be updated")
	assert.Equal(t, uint64(24*3600), params["execution_delay"], "execution_delay should be updated")
	
	// 잘못된 매개변수 설정
	args.Quorum = 1.5 // 1.0보다 큰 값은 유효하지 않음
	_, err = api.SetParams(args)
	assert.Error(t, err, "SetParams should return an error for invalid quorum")
	
	args.Quorum = 0.4
	args.Threshold = -0.1 // 음수 값은 유효하지 않음
	_, err = api.SetParams(args)
	assert.Error(t, err, "SetParams should return an error for invalid threshold")
}

// TestExecuteProposal은 ExecuteProposal API 메서드를 테스트합니다.
func TestExecuteProposal(t *testing.T) {
	// 테스트 API 설정
	api, govManager, _, _ := setupTestAPI(t)
	
	// 테스트 제안 생성
	proposer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	args := SubmitProposalArgs{
		Type:           utils.ProposalTypeText,
		Title:          "Test Proposal",
		Description:    "This is a test proposal",
		Proposer:       proposer,
		InitialDeposit: "100",
		Content: &TestProposalContent{
			ProposalType: utils.ProposalTypeText,
			Params:       make(map[string]string),
		},
	}
	
	proposalID, err := api.SubmitProposal(args)
	assert.NoError(t, err, "SubmitProposal should not return an error")
	
	// 제안 상태를 통과로 변경
	proposal, _ := govManager.GetProposal(proposalID)
	proposal.Status = utils.ProposalStatusPassed
	
	// 제안 실행
	executeArgs := ExecuteProposalArgs{
		ProposalID: proposalID,
	}
	
	success, err := api.ExecuteProposal(executeArgs)
	assert.NoError(t, err, "ExecuteProposal should not return an error")
	assert.True(t, success, "ExecuteProposal should be successful")
	
	// 이미 실행된 제안 실행
	proposal.Status = utils.ProposalStatusExecuted
	_, err = api.ExecuteProposal(executeArgs)
	assert.Error(t, err, "ExecuteProposal should return an error for already executed proposal")
	
	// 존재하지 않는 제안 실행
	executeArgs.ProposalID = 999
	_, err = api.ExecuteProposal(executeArgs)
	assert.Error(t, err, "ExecuteProposal should return an error for non-existent proposal")
}

// TestGetProposalsByStatus는 GetProposalsByStatus API 메서드를 테스트합니다.
func TestGetProposalsByStatus(t *testing.T) {
	// 테스트 API 설정
	api, govManager, _, _ := setupTestAPI(t)
	
	// 여러 제안 생성
	proposer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	
	// 첫 번째 제안 (보증금 기간)
	args1 := SubmitProposalArgs{
		Type:           utils.ProposalTypeText,
		Title:          "Proposal 1",
		Description:    "This is proposal 1",
		Proposer:       proposer,
		InitialDeposit: "100",
		Content: &TestProposalContent{
			ProposalType: utils.ProposalTypeText,
			Params:       make(map[string]string),
		},
	}
	
	api.SubmitProposal(args1)
	
	// 두 번째 제안 (투표 기간)
	args2 := SubmitProposalArgs{
		Type:           utils.ProposalTypeText,
		Title:          "Proposal 2",
		Description:    "This is proposal 2",
		Proposer:       proposer,
		InitialDeposit: "100",
		Content: &TestProposalContent{
			ProposalType: utils.ProposalTypeText,
			Params:       make(map[string]string),
		},
	}
	
	proposalID2, _ := api.SubmitProposal(args2)
	proposal2, _ := govManager.GetProposal(proposalID2)
	proposal2.Status = utils.ProposalStatusVotingPeriod
	
	// 세 번째 제안 (통과)
	args3 := SubmitProposalArgs{
		Type:           utils.ProposalTypeText,
		Title:          "Proposal 3",
		Description:    "This is proposal 3",
		Proposer:       proposer,
		InitialDeposit: "100",
		Content: &TestProposalContent{
			ProposalType: utils.ProposalTypeText,
			Params:       make(map[string]string),
		},
	}
	
	proposalID3, _ := api.SubmitProposal(args3)
	proposal3, _ := govManager.GetProposal(proposalID3)
	proposal3.Status = utils.ProposalStatusPassed
	
	// 보증금 기간 제안 조회
	depositProposals, err := api.GetProposalsByStatus(utils.ProposalStatusDepositPeriod)
	assert.NoError(t, err, "GetProposalsByStatus should not return an error")
	assert.Equal(t, 1, len(depositProposals), "Should have 1 proposal in deposit period")
	assert.Equal(t, "Proposal 1", depositProposals[0].Title, "Proposal title should match")
	
	// 투표 기간 제안 조회
	votingProposals, err := api.GetProposalsByStatus(utils.ProposalStatusVotingPeriod)
	assert.NoError(t, err, "GetProposalsByStatus should not return an error")
	assert.Equal(t, 1, len(votingProposals), "Should have 1 proposal in voting period")
	assert.Equal(t, "Proposal 2", votingProposals[0].Title, "Proposal title should match")
	
	// 통과 제안 조회
	passedProposals, err := api.GetProposalsByStatus(utils.ProposalStatusPassed)
	assert.NoError(t, err, "GetProposalsByStatus should not return an error")
	assert.Equal(t, 1, len(passedProposals), "Should have 1 passed proposal")
	assert.Equal(t, "Proposal 3", passedProposals[0].Title, "Proposal title should match")
	
	// 잘못된 상태로 제안 조회
	_, err = api.GetProposalsByStatus("invalid_status")
	assert.Error(t, err, "GetProposalsByStatus should return an error for invalid status")
} 