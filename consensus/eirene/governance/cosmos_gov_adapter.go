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
	"fmt"
	"math/big"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/cosmos"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/log"
)

// GovParams는 거버넌스 관련 매개변수를 정의합니다.
type GovParams struct {
	MinDeposit        *big.Int      // 최소 제안 보증금
	MaxDepositPeriod  time.Duration // 최대 보증금 기간
	VotingPeriod      time.Duration // 투표 기간
	Quorum            float64       // 쿼럼 (투표 참여 최소 비율)
	Threshold         float64       // 통과 임계값
	VetoThreshold     float64       // 거부권 임계값
	ExecutionDelay    time.Duration // 실행 지연 시간
}

// DefaultGovParams는 기본 거버넌스 매개변수를 반환합니다.
func DefaultGovParams() GovParams {
	// 100 * 10^18 (100 토큰)
	minDeposit := new(big.Int).Mul(big.NewInt(100), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	
	return GovParams{
		MinDeposit:        minDeposit,
		MaxDepositPeriod:  2 * 24 * time.Hour,                // 2일
		VotingPeriod:      7 * 24 * time.Hour,                // 7일
		Quorum:            0.334,                             // 33.4%
		Threshold:         0.5,                               // 50%
		VetoThreshold:     0.334,                             // 33.4%
		ExecutionDelay:    2 * 24 * time.Hour,                // 2일
	}
}

// CosmosGovAdapter는 Cosmos SDK의 gov 모듈과 연동하는 어댑터입니다.
type CosmosGovAdapter struct {
	*BaseGovernanceAdapter // 기본 거버넌스 어댑터 상속
	storeAdapter *cosmos.StateDBAdapter
	params       GovParams
}

// NewCosmosGovAdapter는 새로운 CosmosGovAdapter 인스턴스를 생성합니다.
func NewCosmosGovAdapter(eirene EireneInterface, storeAdapter *cosmos.StateDBAdapter) *CosmosGovAdapter {
	logger := log.New("module", "cosmos_gov")
	baseAdapter := NewBaseGovernanceAdapter(eirene, eirene.GetDB(), logger)
	
	return &CosmosGovAdapter{
		BaseGovernanceAdapter: baseAdapter,
		storeAdapter: storeAdapter,
		params: DefaultGovParams(),
	}
}

// SubmitProposal은 새로운 제안을 제출합니다.
func (a *CosmosGovAdapter) SubmitProposal(proposer common.Address, title string, description string, proposalTypeStr string, content utils.ProposalContentInterface, initialDeposit *big.Int, state *state.StateDB) (uint64, error) {
	// Cosmos SDK 관련 로직 처리
	a.logger.Info("Submitting proposal via Cosmos SDK", "proposer", proposer.Hex())
	
	// 제안 유형 변환
	var proposalType GovProposalType
	switch proposalTypeStr {
	case "parameter_change":
		proposalType = GovProposalTypeParameterChange
	case "software_upgrade":
		proposalType = GovProposalTypeSoftwareUpgrade
	case "community_pool_spend":
		proposalType = GovProposalTypeCommunityPoolSpend
	case "text":
		proposalType = GovProposalTypeText
	default:
		return 0, utils.WrapError(utils.ErrInvalidParameter, fmt.Sprintf("invalid proposal type: %s", proposalTypeStr))
	}

	// 제안 유효성 검사
	if !a.IsValidProposalType(proposalType) {
		return 0, utils.WrapError(utils.ErrInvalidParameter, fmt.Sprintf("invalid proposal type: %d", proposalType))
	}

	if len(title) == 0 {
		return 0, utils.WrapError(utils.ErrInvalidParameter, "title cannot be empty")
	}

	if len(description) == 0 {
		return 0, utils.WrapError(utils.ErrInvalidParameter, "description cannot be empty")
	}

	// 초기 보증금 검사
	if initialDeposit == nil {
		initialDeposit = big.NewInt(0)
	}

	// 최소 보증금 검사
	if initialDeposit.Cmp(a.params.MinDeposit) < 0 {
		return 0, utils.WrapError(utils.ErrInsufficientFunds, fmt.Sprintf("initial deposit %s is less than minimum required %s", initialDeposit.String(), a.params.MinDeposit.String()))
	}

	// 제안 생성
	now := time.Now()
	proposal := &GovProposal{
		ID:              a.nextProposalID,
		Title:           title,
		Description:     description,
		ProposalType:    proposalType,
		ProposerAddress: proposer,
		Status:          GovProposalStatusDepositPeriod,
		SubmitTime:      now,
		DepositEndTime:  now.Add(a.params.MaxDepositPeriod),
		TotalDeposit:    initialDeposit,
		Votes:           make([]*GovVote, 0),
		Deposits:        make([]*GovDeposit, 0),
	}

	// 초기 보증금 추가
	if initialDeposit.Sign() > 0 {
		deposit := &GovDeposit{
			ProposalID: proposal.ID,
			Depositor:  proposer,
			Amount:     initialDeposit,
			Timestamp:  now,
		}
		proposal.Deposits = append(proposal.Deposits, deposit)
	}

	// 제안 저장
	a.proposals[proposal.ID] = proposal
	a.nextProposalID++

	// 초기 보증금이 최소 보증금 이상이면 바로 투표 기간으로 전환
	if initialDeposit.Cmp(a.params.MinDeposit) >= 0 {
		a.activateVotingPeriod(proposal)
	}

	a.logger.Info("Proposal submitted", "id", proposal.ID, "type", proposalType, "proposer", proposer.Hex())
	return proposal.ID, nil
}

// Deposit은 제안에 보증금을 추가합니다.
func (a *CosmosGovAdapter) Deposit(proposalID uint64, depositor common.Address, amount *big.Int) error {
	// Cosmos SDK 관련 로직 처리
	a.logger.Info("Adding deposit via Cosmos SDK", "proposalID", proposalID, "depositor", depositor.Hex())
	
	// 제안 조회
	proposal, exists := a.proposals[proposalID]
	if !exists {
		return utils.WrapError(utils.ErrProposalNotFound, fmt.Sprintf("proposal not found: %d", proposalID))
	}

	// 제안 상태 확인
	if proposal.Status != GovProposalStatusDepositPeriod {
		return utils.WrapError(utils.ErrInvalidProposalStatus, fmt.Sprintf("proposal is not in deposit period: %d", proposalID))
	}

	// 보증금 마감 시간 확인
	now := time.Now()
	if now.After(proposal.DepositEndTime) {
		return utils.WrapError(utils.ErrDepositPeriodEnded, fmt.Sprintf("deposit period ended: %d", proposalID))
	}

	// 보증금 추가
	deposit := &GovDeposit{
		ProposalID: proposalID,
		Depositor:  depositor,
		Amount:     amount,
		Timestamp:  now,
	}
	proposal.Deposits = append(proposal.Deposits, deposit)
	proposal.TotalDeposit = new(big.Int).Add(proposal.TotalDeposit, amount)

	// 최소 보증금 도달 시 투표 기간으로 전환
	if proposal.TotalDeposit.Cmp(a.params.MinDeposit) >= 0 && proposal.Status == GovProposalStatusDepositPeriod {
		a.activateVotingPeriod(proposal)
	}

	a.logger.Info("Deposit added to proposal", "id", proposalID, "depositor", depositor.Hex(), "amount", amount)
	return nil
}

// Vote는 제안에 투표합니다.
func (a *CosmosGovAdapter) Vote(proposalID uint64, voter common.Address, optionStr string) error {
	// Cosmos SDK 관련 로직 처리
	a.logger.Info("Voting via Cosmos SDK", "proposalID", proposalID, "voter", voter.Hex())
	
	// 제안 조회
	proposal, exists := a.proposals[proposalID]
	if !exists {
		return utils.WrapError(utils.ErrProposalNotFound, fmt.Sprintf("proposal not found: %d", proposalID))
	}

	// 제안 상태 확인
	if proposal.Status != GovProposalStatusVotingPeriod {
		return utils.WrapError(utils.ErrInvalidProposalStatus, fmt.Sprintf("proposal is not in voting period: %d", proposalID))
	}

	// 투표 마감 시간 확인
	now := time.Now()
	if now.After(proposal.VotingEndTime) {
		return utils.WrapError(utils.ErrVotingPeriodEnded, fmt.Sprintf("voting period ended: %d", proposalID))
	}

	// 투표 옵션 변환
	var option GovVoteOption
	switch optionStr {
	case "yes":
		option = GovOptionYes
	case "no":
		option = GovOptionNo
	case "no_with_veto":
		option = GovOptionNoWithVeto
	case "abstain":
		option = GovOptionAbstain
	default:
		return utils.WrapError(utils.ErrInvalidParameter, fmt.Sprintf("invalid vote option: %s", optionStr))
	}

	// 중복 투표 확인
	for _, vote := range proposal.Votes {
		if vote.Voter == voter {
			return utils.WrapError(utils.ErrDuplicateVote, fmt.Sprintf("voter already voted: %s", voter.Hex()))
		}
	}

	// 투표 추가
	vote := &GovVote{
		ProposalID: proposalID,
		Voter:      voter,
		Option:     option,
		Timestamp:  now,
	}
	proposal.Votes = append(proposal.Votes, vote)

	a.logger.Info("Vote cast", "id", proposalID, "voter", voter.Hex(), "option", option)
	return nil
}

// ExecuteProposal은 제안을 실행합니다.
func (a *CosmosGovAdapter) ExecuteProposal(proposalID uint64, state *state.StateDB) error {
	// Cosmos SDK 관련 로직 처리
	a.logger.Info("Executing proposal via Cosmos SDK", "proposalID", proposalID)
	
	// 제안 조회
	proposal, exists := a.proposals[proposalID]
	if !exists {
		return utils.WrapError(utils.ErrProposalNotFound, fmt.Sprintf("proposal not found: %d", proposalID))
	}

	// 제안 상태 확인
	if proposal.Status != GovProposalStatusPassed {
		return utils.WrapError(utils.ErrInvalidProposalStatus, fmt.Sprintf("proposal is not passed: %d", proposalID))
	}

	// 제안 유형별 실행
	var err error
	switch proposal.ProposalType {
	case GovProposalTypeParameterChange:
		err = a.executeParameterChangeProposal(proposal)
	case GovProposalTypeSoftwareUpgrade:
		// 소프트웨어 업그레이드 제안은 ProcessProposals에서 처리
		err = nil
	case GovProposalTypeCommunityPoolSpend:
		err = a.executeCommunityPoolSpendProposal(state, proposal)
	case GovProposalTypeText:
		// 텍스트 제안은 실행할 내용이 없음
		err = nil
	default:
		err = utils.WrapError(utils.ErrInvalidParameter, fmt.Sprintf("invalid proposal type: %d", proposal.ProposalType))
	}

	if err != nil {
		proposal.Status = GovProposalStatusFailed
		a.logger.Error("Proposal execution failed", "id", proposalID, "error", err)
		return err
	}

	// 제안 상태 업데이트
	proposal.Status = GovProposalStatusExecuted
	proposal.ExecutionTime = time.Now()

	a.logger.Info("Proposal executed", "id", proposalID)
	return nil
}

// ProcessProposals는 블록 생성 시 제안을 처리합니다.
func (a *CosmosGovAdapter) ProcessProposals(blockHeight uint64, state *state.StateDB) error {
	// Cosmos SDK 관련 로직 처리
	a.logger.Info("Processing proposals via Cosmos SDK", "blockHeight", blockHeight)
	
	now := time.Now()

	// 모든 제안 처리
	for _, proposal := range a.proposals {
		// 보증금 기간 종료 확인
		if proposal.Status == GovProposalStatusDepositPeriod && now.After(proposal.DepositEndTime) {
			// 최소 보증금 미달 시 제안 거부
			if proposal.TotalDeposit.Cmp(a.params.MinDeposit) < 0 {
				proposal.Status = GovProposalStatusRejected
				a.logger.Info("Proposal rejected due to insufficient deposit", "id", proposal.ID)
			}
		}

		// 투표 기간 종료 확인
		if proposal.Status == GovProposalStatusVotingPeriod && now.After(proposal.VotingEndTime) {
			// 투표 집계
			if a.tallyVotes(proposal) {
				proposal.Status = GovProposalStatusPassed
				a.logger.Info("Proposal passed", "id", proposal.ID)
			} else {
				proposal.Status = GovProposalStatusRejected
				a.logger.Info("Proposal rejected", "id", proposal.ID)
			}
		}

		// 통과된 소프트웨어 업그레이드 제안 처리
		if proposal.Status == GovProposalStatusPassed && proposal.ProposalType == GovProposalTypeSoftwareUpgrade {
			err := a.executeSoftwareUpgradeProposal(proposal, blockHeight)
			if err != nil {
				a.logger.Error("Software upgrade proposal execution failed", "id", proposal.ID, "error", err)
			}
		}
	}

	return nil
}

// GetProposal은 제안 정보를 반환합니다.
func (a *CosmosGovAdapter) GetProposal(proposalID uint64) (*GovProposal, error) {
	return a.BaseGovernanceAdapter.GetProposal(proposalID)
}

// GetProposals는 모든 제안 목록을 반환합니다.
func (a *CosmosGovAdapter) GetProposals() []*GovProposal {
	return a.BaseGovernanceAdapter.GetProposals()
}

// GetProposalsByStatus는 특정 상태의 제안 목록을 반환합니다.
func (a *CosmosGovAdapter) GetProposalsByStatus(status GovProposalStatus) []*GovProposal {
	return a.BaseGovernanceAdapter.GetProposalsByStatus(status)
}

// executeParameterChangeProposal은 매개변수 변경 제안을 실행합니다.
func (a *CosmosGovAdapter) executeParameterChangeProposal(proposal *GovProposal) error {
	a.logger.Info("Executing parameter change proposal", "proposalID", proposal.ID)
	return a.BaseGovernanceAdapter.executeParameterChangeProposal(proposal)
}

// executeSoftwareUpgradeProposal은 소프트웨어 업그레이드 제안을 실행합니다.
func (a *CosmosGovAdapter) executeSoftwareUpgradeProposal(proposal *GovProposal, blockHeight uint64) error {
	a.logger.Info("Executing software upgrade proposal", "proposalID", proposal.ID, "blockHeight", blockHeight)
	return a.BaseGovernanceAdapter.executeSoftwareUpgradeProposal(proposal, blockHeight)
}

// executeCommunityPoolSpendProposal은 커뮤니티 풀 지출 제안을 실행합니다.
func (a *CosmosGovAdapter) executeCommunityPoolSpendProposal(state *state.StateDB, proposal *GovProposal) error {
	a.logger.Info("Executing community pool spend proposal", "proposalID", proposal.ID)
	return a.BaseGovernanceAdapter.executeCommunityPoolSpendProposal(state, proposal)
}

// isValidProposalType은 제안 유형이 유효한지 확인합니다.
func isValidProposalType(proposalType GovProposalType) bool {
	switch proposalType {
	case GovProposalTypeText, GovProposalTypeParameterChange, GovProposalTypeSoftwareUpgrade, GovProposalTypeCommunityPoolSpend:
		return true
	default:
		return false
	}
}

// isValidVoteOption은 투표 옵션이 유효한지 확인합니다.
func isValidVoteOption(option GovVoteOption) bool {
	switch option {
	case GovOptionYes, GovOptionNo, GovOptionNoWithVeto, GovOptionAbstain:
		return true
	default:
		return false
	}
} 