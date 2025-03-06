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

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/cosmos"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/rawdb"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/tracing"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/params"
)

// MockEirene는 테스트를 위한 Eirene 인터페이스 모의 구현체입니다.
type MockEirene struct {
	db           ethdb.Database
	config       *params.EireneConfig
	validatorSet *MockValidatorSet
}

func (m *MockEirene) GetDB() ethdb.Database {
	return m.db
}

func (m *MockEirene) GetConfig() *params.EireneConfig {
	return m.config
}

func (m *MockEirene) GetValidatorSet() utils.ValidatorSetInterface {
	return m.validatorSet
}

// MockValidatorSet은 테스트를 위한 ValidatorSetInterface 모의 구현체입니다.
type MockValidatorSet struct {
	validators map[common.Address]bool
}

func NewMockValidatorSet() *MockValidatorSet {
	return &MockValidatorSet{
		validators: make(map[common.Address]bool),
	}
}

func (m *MockValidatorSet) Contains(address common.Address) bool {
	return m.validators[address]
}

func (m *MockValidatorSet) AddValidator(address common.Address) {
	m.validators[address] = true
}

func (m *MockValidatorSet) RemoveValidator(address common.Address) {
	delete(m.validators, address)
}

func (m *MockValidatorSet) GetValidatorCount() int {
	return len(m.validators)
}

func (m *MockValidatorSet) GetActiveValidatorCount() int {
	return len(m.validators)
}

func (m *MockValidatorSet) GetTotalStake() *big.Int {
	return big.NewInt(int64(len(m.validators) * 100))
}

func (m *MockValidatorSet) GetValidatorByAddress(address common.Address) utils.ValidatorInterface {
	if !m.validators[address] {
		return nil
	}
	return &MockValidator{
		address: address,
	}
}

func (m *MockValidatorSet) GetActiveValidators() []utils.ValidatorInterface {
	validators := make([]utils.ValidatorInterface, 0, len(m.validators))
	for addr := range m.validators {
		validators = append(validators, &MockValidator{
			address: addr,
		})
	}
	return validators
}

func (m *MockValidatorSet) GetValidatorsAtHeight(height int64) ([]utils.ValidatorInterface, error) {
	return m.GetActiveValidators(), nil
}

// MockValidator는 테스트를 위한 ValidatorInterface 모의 구현체입니다.
type MockValidator struct {
	address common.Address
}

func (v *MockValidator) GetAddress() common.Address {
	return v.address
}

func (v *MockValidator) GetVotingPower() *big.Int {
	return big.NewInt(100)
}

func (v *MockValidator) GetStatus() uint8 {
	return 1 // 활성 상태
}

func (v *MockValidator) IsActive() bool {
	return true
}

// MockProposalContent는 테스트를 위한 ProposalContentInterface 모의 구현체입니다.
type MockProposalContent struct {
	params map[string]string
}

func (m *MockProposalContent) GetType() string {
	return "parameter_change"
}

func (m *MockProposalContent) Validate() error {
	return nil
}

func (m *MockProposalContent) GetParams() map[string]string {
	return m.params
}

func (m *MockProposalContent) Execute(state *state.StateDB) error {
	// 테스트용 구현체이므로 아무 작업도 수행하지 않음
	return nil
}

// TestCosmosGovAdapterInit은 CosmosGovAdapter 초기화를 테스트합니다.
func TestCosmosGovAdapterInit(t *testing.T) {
	// 테스트 환경 설정
	db := rawdb.NewMemoryDatabase()
	stateDB, _ := state.New(common.Hash{}, state.NewDatabaseForTesting())
	
	// MockEirene 생성
	mockEirene := &MockEirene{
		db:     db,
		config: &params.EireneConfig{},
	}
	
	// StateDBAdapter 생성
	storeAdapter := cosmos.NewStateDBAdapter(stateDB)
	
	// CosmosGovAdapter 생성
	cosmosGovAdapter := NewCosmosGovAdapter(mockEirene, storeAdapter)
	
	// 어댑터가 올바르게 초기화되었는지 확인
	assert.NotNil(t, cosmosGovAdapter, "CosmosGovAdapter가 nil입니다")
	assert.NotNil(t, cosmosGovAdapter.logger, "Logger가 nil입니다")
	assert.Equal(t, storeAdapter, cosmosGovAdapter.storeAdapter, "StateDBAdapter가 올바르게 설정되지 않았습니다")
	
	// 기본 매개변수 확인
	params := cosmosGovAdapter.params
	expectedMinDeposit := new(big.Int).Mul(big.NewInt(100), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	assert.Equal(t, 0, expectedMinDeposit.Cmp(params.MinDeposit), "MinDeposit이 올바르게 설정되지 않았습니다")
	assert.Equal(t, 2*24*time.Hour, params.MaxDepositPeriod, "MaxDepositPeriod가 올바르게 설정되지 않았습니다")
	assert.Equal(t, 7*24*time.Hour, params.VotingPeriod, "VotingPeriod가 올바르게 설정되지 않았습니다")
	assert.Equal(t, 0.334, params.Quorum, "Quorum이 올바르게 설정되지 않았습니다")
	assert.Equal(t, 0.5, params.Threshold, "Threshold가 올바르게 설정되지 않았습니다")
	assert.Equal(t, 0.334, params.VetoThreshold, "VetoThreshold이 올바르게 설정되지 않았습니다")
	assert.Equal(t, 2*24*time.Hour, params.ExecutionDelay, "ExecutionDelay가 올바르게 설정되지 않았습니다")
}

// TestSubmitProposalCosmos는 제안 제출 기능을 테스트합니다.
func TestSubmitProposalCosmos(t *testing.T) {
	// 테스트 환경 설정
	db := rawdb.NewMemoryDatabase()
	stateDB, _ := state.New(common.Hash{}, state.NewDatabaseForTesting())
	
	// 테스트 계정에 잔액 추가
	testProposer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	initialBalance := new(big.Int).Mul(big.NewInt(1000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	initialBalanceUint256 := uint256.MustFromBig(initialBalance)
	stateDB.AddBalance(testProposer, initialBalanceUint256, tracing.BalanceChangeUnspecified)
	
	// MockEirene 생성
	mockEirene := &MockEirene{
		db:     db,
		config: &params.EireneConfig{},
	}
	
	// StateDBAdapter 생성
	storeAdapter := cosmos.NewStateDBAdapter(stateDB)
	
	// CosmosGovAdapter 생성
	cosmosGovAdapter := NewCosmosGovAdapter(mockEirene, storeAdapter)
	
	// 제안 제출 테스트
	title := "테스트 제안"
	description := "이것은 테스트 제안입니다."
	proposalTypeStr := "parameter_change"
	initialDeposit := new(big.Int).Mul(big.NewInt(100), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	
	// 제안 컨텐츠 생성
	content := &MockProposalContent{
		params: map[string]string{
			"votingPeriod": "604800", // 7일 (초 단위)
		},
	}
	
	proposalID, err := cosmosGovAdapter.SubmitProposal(
		testProposer,
		title,
		description,
		proposalTypeStr,
		content,
		initialDeposit,
		stateDB,
	)
	
	// 결과 확인
	assert.NoError(t, err, "제안 제출 중 오류 발생")
	assert.Equal(t, uint64(1), proposalID, "제안 ID가 1이 아닙니다")
	
	// 제안 확인
	proposal, err := cosmosGovAdapter.GetProposal(proposalID)
	assert.NoError(t, err, "제안 조회 중 오류 발생")
	assert.Equal(t, title, proposal.Title, "제안 제목이 일치하지 않습니다")
	assert.Equal(t, description, proposal.Description, "제안 설명이 일치하지 않습니다")
	assert.Equal(t, GovProposalTypeParameterChange, proposal.ProposalType, "제안 유형이 일치하지 않습니다")
	assert.Equal(t, testProposer, proposal.ProposerAddress, "제안자가 일치하지 않습니다")
	assert.Equal(t, 0, initialDeposit.Cmp(proposal.TotalDeposit), "총 보증금이 일치하지 않습니다")
}

// TestDepositCosmos는 제안 보증금 예치 기능을 테스트합니다.
func TestDepositCosmos(t *testing.T) {
	// 테스트 환경 설정
	db := rawdb.NewMemoryDatabase()
	stateDB, _ := state.New(common.Hash{}, state.NewDatabaseForTesting())
	
	// 테스트 계정에 잔액 추가
	testProposer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	testDepositor := common.HexToAddress("0x2222222222222222222222222222222222222222")
	initialBalance := new(big.Int).Mul(big.NewInt(1000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	initialBalanceUint256 := uint256.MustFromBig(initialBalance)
	stateDB.AddBalance(testProposer, initialBalanceUint256, tracing.BalanceChangeUnspecified)
	stateDB.AddBalance(testDepositor, initialBalanceUint256, tracing.BalanceChangeUnspecified)
	
	// MockEirene 생성
	mockEirene := &MockEirene{
		db:     db,
		config: &params.EireneConfig{},
	}
	
	// StateDBAdapter 생성
	storeAdapter := cosmos.NewStateDBAdapter(stateDB)
	
	// CosmosGovAdapter 생성
	cosmosGovAdapter := NewCosmosGovAdapter(mockEirene, storeAdapter)
	
	// 제안 제출 (최소 보증금보다 적게 설정)
	title := "테스트 제안"
	description := "이것은 테스트 제안입니다."
	proposalType := GovProposalTypeParameterChange
	initialDeposit := new(big.Int).Mul(big.NewInt(50), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)) // 최소 요구 금액의 절반
	
	// 제안 직접 생성
	proposalID := cosmosGovAdapter.nextProposalID
	now := time.Now()
	proposal := &GovProposal{
		ID:              proposalID,
		Title:           title,
		Description:     description,
		ProposalType:    proposalType,
		ProposerAddress: testProposer,
		Status:          GovProposalStatusDepositPeriod,
		SubmitTime:      now,
		DepositEndTime:  now.Add(14 * 24 * time.Hour), // 14일
		TotalDeposit:    initialDeposit,
		Votes:           make([]*GovVote, 0),
		Deposits:        make([]*GovDeposit, 0),
	}
	
	// 초기 보증금 예치
	deposit := &GovDeposit{
		ProposalID: proposal.ID,
		Depositor:  testProposer,
		Amount:     initialDeposit,
		Timestamp:  now,
	}
	proposal.Deposits = append(proposal.Deposits, deposit)
	
	// 제안 저장
	cosmosGovAdapter.proposals[proposal.ID] = proposal
	cosmosGovAdapter.nextProposalID++
	
	// 추가 보증금 예치 (최소 요구 금액을 충족하도록)
	additionalDeposit := new(big.Int).Mul(big.NewInt(50), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	err := cosmosGovAdapter.Deposit(proposalID, testDepositor, additionalDeposit)
	assert.NoError(t, err, "보증금 예치 중 오류 발생")
	
	// 제안 확인
	proposal, err = cosmosGovAdapter.GetProposal(proposalID)
	assert.NoError(t, err, "제안 조회 중 오류 발생")
	expectedTotalDeposit := new(big.Int).Add(initialDeposit, additionalDeposit)
	assert.Equal(t, 0, expectedTotalDeposit.Cmp(proposal.TotalDeposit), "총 보증금이 일치하지 않습니다")
	
	// 제안이 투표 기간으로 전환되었는지 확인
	assert.Equal(t, GovProposalStatusVotingPeriod, proposal.Status, "제안이 투표 기간으로 전환되지 않았습니다")
}

// TestVoteCosmos는 제안 투표 기능을 테스트합니다.
func TestVoteCosmos(t *testing.T) {
	// 테스트 환경 설정
	db := rawdb.NewMemoryDatabase()
	stateDB, _ := state.New(common.Hash{}, state.NewDatabaseForTesting())
	
	// 테스트 계정에 잔액 추가
	testProposer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	testVoter := common.HexToAddress("0x2222222222222222222222222222222222222222")
	initialBalance := new(big.Int).Mul(big.NewInt(1000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	initialBalanceUint256 := uint256.MustFromBig(initialBalance)
	stateDB.AddBalance(testProposer, initialBalanceUint256, tracing.BalanceChangeUnspecified)
	
	// MockValidatorSet 생성 및 투표자 추가
	mockValidatorSet := NewMockValidatorSet()
	mockValidatorSet.AddValidator(testVoter)
	
	// MockEirene 생성
	mockEirene := &MockEirene{
		db:           db,
		config:       &params.EireneConfig{},
		validatorSet: mockValidatorSet,
	}
	
	// StateDBAdapter 생성
	storeAdapter := cosmos.NewStateDBAdapter(stateDB)
	
	// CosmosGovAdapter 생성
	cosmosGovAdapter := NewCosmosGovAdapter(mockEirene, storeAdapter)
	
	// 제안 직접 생성
	title := "테스트 제안"
	description := "이것은 테스트 제안입니다."
	proposalType := GovProposalTypeParameterChange
	initialDeposit := new(big.Int).Mul(big.NewInt(100), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	
	// 제안 직접 생성
	proposalID := cosmosGovAdapter.nextProposalID
	now := time.Now()
	proposal := &GovProposal{
		ID:              proposalID,
		Title:           title,
		Description:     description,
		ProposalType:    proposalType,
		ProposerAddress: testProposer,
		Status:          GovProposalStatusVotingPeriod, // 투표 기간으로 설정
		SubmitTime:      now,
		DepositEndTime:  now.Add(14 * 24 * time.Hour),
		TotalDeposit:    initialDeposit,
		VotingStartTime: now,
		VotingEndTime:   now.Add(14 * 24 * time.Hour),
		Votes:           make([]*GovVote, 0),
		Deposits:        make([]*GovDeposit, 0),
	}
	
	// 제안 저장
	cosmosGovAdapter.proposals[proposal.ID] = proposal
	cosmosGovAdapter.nextProposalID++
	
	// 투표
	err := cosmosGovAdapter.Vote(proposalID, testVoter, "yes")
	assert.NoError(t, err, "투표 중 오류 발생")
	
	// 제안 확인
	proposal, err = cosmosGovAdapter.GetProposal(proposalID)
	assert.NoError(t, err, "제안 조회 중 오류 발생")
	
	// 투표 확인
	found := false
	for _, vote := range proposal.Votes {
		if vote.Voter == testVoter {
			found = true
			assert.Equal(t, GovOptionYes, vote.Option, "투표 옵션이 일치하지 않습니다")
		}
	}
	assert.True(t, found, "투표가 기록되지 않았습니다")
}

// TestExecuteProposalCosmos는 제안 실행 기능을 테스트합니다.
func TestExecuteProposalCosmos(t *testing.T) {
	// 테스트 환경 설정
	db := rawdb.NewMemoryDatabase()
	stateDB, _ := state.New(common.Hash{}, state.NewDatabaseForTesting())
	
	// 테스트 계정에 잔액 추가
	testProposer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	testVoter1 := common.HexToAddress("0x2222222222222222222222222222222222222222")
	testVoter2 := common.HexToAddress("0x3333333333333333333333333333333333333333")
	testVoter3 := common.HexToAddress("0x4444444444444444444444444444444444444444")
	initialBalance := new(big.Int).Mul(big.NewInt(1000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	initialBalanceUint256 := uint256.MustFromBig(initialBalance)
	stateDB.AddBalance(testProposer, initialBalanceUint256, tracing.BalanceChangeUnspecified)
	
	// MockValidatorSet 생성 및 투표자 추가
	mockValidatorSet := NewMockValidatorSet()
	mockValidatorSet.AddValidator(testVoter1)
	mockValidatorSet.AddValidator(testVoter2)
	mockValidatorSet.AddValidator(testVoter3)
	
	// MockEirene 생성
	mockEirene := &MockEirene{
		db:           db,
		config:       &params.EireneConfig{},
		validatorSet: mockValidatorSet,
	}
	
	// StateDBAdapter 생성
	storeAdapter := cosmos.NewStateDBAdapter(stateDB)
	
	// CosmosGovAdapter 생성
	cosmosGovAdapter := NewCosmosGovAdapter(mockEirene, storeAdapter)
	
	// 제안 직접 생성
	title := "테스트 제안"
	description := "이것은 테스트 제안입니다."
	proposalType := GovProposalTypeParameterChange
	initialDeposit := new(big.Int).Mul(big.NewInt(100), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	params := map[string]string{
		"votingPeriod": "604800", // 7일 (초 단위)
	}
	
	// 제안 직접 생성
	proposalID := cosmosGovAdapter.nextProposalID
	now := time.Now()
	proposal := &GovProposal{
		ID:              proposalID,
		Title:           title,
		Description:     description,
		ProposalType:    proposalType,
		ProposerAddress: testProposer,
		Status:          GovProposalStatusVotingPeriod, // 투표 기간으로 설정
		SubmitTime:      now,
		DepositEndTime:  now.Add(14 * 24 * time.Hour),
		TotalDeposit:    initialDeposit,
		VotingStartTime: now,
		VotingEndTime:   now.Add(14 * 24 * time.Hour),
		Params:          params,
		Votes:           make([]*GovVote, 0),
		Deposits:        make([]*GovDeposit, 0),
	}
	
	// 제안 저장
	cosmosGovAdapter.proposals[proposal.ID] = proposal
	cosmosGovAdapter.nextProposalID++
	
	// 제안자 잔액 차감
	initialDepositUint256 := uint256.MustFromBig(initialDeposit)
	stateDB.SubBalance(testProposer, initialDepositUint256, tracing.BalanceChangeUnspecified)
	
	// 투표
	err := cosmosGovAdapter.Vote(proposalID, testVoter1, "yes")
	assert.NoError(t, err, "투표 중 오류 발생")
	err = cosmosGovAdapter.Vote(proposalID, testVoter2, "yes")
	assert.NoError(t, err, "투표 중 오류 발생")
	err = cosmosGovAdapter.Vote(proposalID, testVoter3, "no")
	assert.NoError(t, err, "투표 중 오류 발생")
	
	// 제안 상태 업데이트 (투표 기간 종료)
	proposal, err = cosmosGovAdapter.GetProposal(proposalID)
	assert.NoError(t, err, "제안 조회 중 오류 발생")
	proposal.Status = GovProposalStatusPassed
	proposal.VotingEndTime = time.Now().Add(-24 * time.Hour) // 투표 종료
	
	// 제안 실행
	err = cosmosGovAdapter.ExecuteProposal(proposalID, stateDB)
	assert.NoError(t, err, "제안 실행 중 오류 발생")
	
	// 제안 상태 확인
	proposal, err = cosmosGovAdapter.GetProposal(proposalID)
	assert.NoError(t, err, "제안 조회 중 오류 발생")
	assert.Equal(t, GovProposalStatusExecuted, proposal.Status, "제안 상태가 실행됨으로 변경되지 않았습니다")
} 