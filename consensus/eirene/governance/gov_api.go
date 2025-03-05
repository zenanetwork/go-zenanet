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

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/types"
)

// GovAPI는 거버넌스 관련 RPC API를 제공합니다.
type GovAPI struct {
	chain      consensus.ChainHeaderReader
	govAdapter *GovAdapter
}

// ProposalResponse는 제안 정보를 반환하는 응답 구조체입니다.
type ProposalResponse struct {
	ID                     uint64            `json:"id"`
	Title                  string            `json:"title"`
	Description            string            `json:"description"`
	ProposalType           string            `json:"proposal_type"`
	ProposerAddress        common.Address    `json:"proposer_address"`
	Status                 string            `json:"status"`
	SubmitTime             uint64            `json:"submit_time"`
	DepositEndTime         uint64            `json:"deposit_end_time"`
	TotalDeposit           string            `json:"total_deposit"`
	VotingStartTime        uint64            `json:"voting_start_time"`
	VotingEndTime          uint64            `json:"voting_end_time"`
	ExecutionTime          uint64            `json:"execution_time"`
	Params                 map[string]string `json:"params,omitempty"`
	UpgradeInfo            *UpgradeResponse  `json:"upgrade_info,omitempty"`
	CommunityPoolSpendInfo *SpendResponse    `json:"community_pool_spend_info,omitempty"`
}

// UpgradeResponse는 업그레이드 정보를 반환하는 응답 구조체입니다.
type UpgradeResponse struct {
	Name   string `json:"name"`
	Height uint64 `json:"height"`
	Info   string `json:"info"`
}

// SpendResponse는 커뮤니티 풀 지출 정보를 반환하는 응답 구조체입니다.
type SpendResponse struct {
	Recipient common.Address `json:"recipient"`
	Amount    string         `json:"amount"`
}

// VoteResponse는 투표 정보를 반환하는 응답 구조체입니다.
type VoteResponse struct {
	ProposalID uint64         `json:"proposal_id"`
	Voter      common.Address `json:"voter"`
	Option     string         `json:"option"`
	Timestamp  uint64         `json:"timestamp"`
}

// DepositResponse는 보증금 정보를 반환하는 응답 구조체입니다.
type DepositResponse struct {
	ProposalID uint64         `json:"proposal_id"`
	Depositor  common.Address `json:"depositor"`
	Amount     string         `json:"amount"`
	Timestamp  uint64         `json:"timestamp"`
}

// NewGovAPI는 새로운 GovAPI 인스턴스를 생성합니다.
func NewGovAPI(govAdapter *GovAdapter, chain consensus.ChainHeaderReader) *GovAPI {
	return &GovAPI{
		chain:      chain,
		govAdapter: govAdapter,
	}
}

// GetProposal은 제안 정보를 반환합니다.
func (api *GovAPI) GetProposal(proposalID uint64) (*ProposalResponse, error) {
	proposal, err := api.govAdapter.GetProposal(proposalID)
	if err != nil {
		return nil, err
	}

	return api.convertProposalToResponse(proposal), nil
}

// GetProposals는 모든 제안 정보를 반환합니다.
func (api *GovAPI) GetProposals() ([]*ProposalResponse, error) {
	proposals := api.govAdapter.GetProposals()
	responses := make([]*ProposalResponse, len(proposals))

	for i, proposal := range proposals {
		responses[i] = api.convertProposalToResponse(proposal)
	}

	return responses, nil
}

// GetProposalsByStatus는 특정 상태의 제안 정보를 반환합니다.
func (api *GovAPI) GetProposalsByStatus(status string) ([]*ProposalResponse, error) {
	var statusEnum GovProposalStatus
	switch status {
	case "deposit_period":
		statusEnum = GovProposalStatusDepositPeriod
	case "voting_period":
		statusEnum = GovProposalStatusVotingPeriod
	case "passed":
		statusEnum = GovProposalStatusPassed
	case "rejected":
		statusEnum = GovProposalStatusRejected
	case "failed":
		statusEnum = GovProposalStatusFailed
	case "executed":
		statusEnum = GovProposalStatusExecuted
	default:
		return nil, fmt.Errorf("invalid proposal status: %s", status)
	}

	proposals := api.govAdapter.GetProposalsByStatus(statusEnum)
	responses := make([]*ProposalResponse, len(proposals))

	for i, proposal := range proposals {
		responses[i] = api.convertProposalToResponse(proposal)
	}

	return responses, nil
}

// SubmitProposal은 새로운 제안을 제출합니다.
func (api *GovAPI) SubmitProposal(args SubmitProposalArgs) (uint64, error) {
	// 상태 DB 가져오기
	state, _, err := api.getStateDB()
	if err != nil {
		return 0, err
	}

	// 제안 유형 변환
	var proposalType GovProposalType
	switch args.ProposalType {
	case "parameter_change":
		proposalType = GovProposalTypeParameterChange
	case "software_upgrade":
		proposalType = GovProposalTypeSoftwareUpgrade
	case "community_pool_spend":
		proposalType = GovProposalTypeCommunityPoolSpend
	case "text":
		proposalType = GovProposalTypeText
	default:
		return 0, fmt.Errorf("invalid proposal type: %s", args.ProposalType)
	}

	// 초기 보증금 변환
	initialDeposit, ok := new(big.Int).SetString(args.InitialDeposit, 10)
	if !ok {
		return 0, fmt.Errorf("invalid initial deposit: %s", args.InitialDeposit)
	}

	// 업그레이드 정보 변환
	var upgradeInfo *GovUpgradeInfo
	if args.UpgradeInfo != nil {
		upgradeInfo = &GovUpgradeInfo{
			Name:   args.UpgradeInfo.Name,
			Height: args.UpgradeInfo.Height,
			Info:   args.UpgradeInfo.Info,
		}
	}

	// 커뮤니티 풀 지출 정보 변환
	var communityPoolSpendInfo *GovCommunityPoolSpendInfo
	if args.CommunityPoolSpendInfo != nil {
		amount, ok := new(big.Int).SetString(args.CommunityPoolSpendInfo.Amount, 10)
		if !ok {
			return 0, fmt.Errorf("invalid community pool spend amount: %s", args.CommunityPoolSpendInfo.Amount)
		}
		communityPoolSpendInfo = &GovCommunityPoolSpendInfo{
			Recipient: args.CommunityPoolSpendInfo.Recipient,
			Amount:    amount,
		}
	}

	// 제안 제출
	return api.govAdapter.SubmitProposal(state, args.ProposerAddress, args.Title, args.Description, proposalType, initialDeposit, args.Params, upgradeInfo, communityPoolSpendInfo)
}

// Deposit은 제안에 보증금을 예치합니다.
func (api *GovAPI) Deposit(args DepositArgs) error {
	// 상태 DB 가져오기
	state, _, err := api.getStateDB()
	if err != nil {
		return err
	}

	// 보증금 변환
	amount, ok := new(big.Int).SetString(args.Amount, 10)
	if !ok {
		return fmt.Errorf("invalid deposit amount: %s", args.Amount)
	}

	// 보증금 예치
	return api.govAdapter.Deposit(state, args.Depositor, args.ProposalID, amount)
}

// Vote는 제안에 투표합니다.
func (api *GovAPI) Vote(args VoteArgs) error {
	// 투표 옵션 변환
	var option GovVoteOption
	switch args.Option {
	case "yes":
		option = GovOptionYes
	case "no":
		option = GovOptionNo
	case "no_with_veto":
		option = GovOptionNoWithVeto
	case "abstain":
		option = GovOptionAbstain
	default:
		return fmt.Errorf("invalid vote option: %s", args.Option)
	}

	// 투표
	return api.govAdapter.Vote(args.Voter, args.ProposalID, option)
}

// SubmitProposalArgs는 제안 제출 인자를 나타내는 구조체입니다.
type SubmitProposalArgs struct {
	ProposerAddress        common.Address          `json:"proposer_address"`
	Title                  string                  `json:"title"`
	Description            string                  `json:"description"`
	ProposalType           string                  `json:"proposal_type"`
	InitialDeposit         string                  `json:"initial_deposit"`
	Params                 map[string]string       `json:"params,omitempty"`
	UpgradeInfo            *UpgradeArgs            `json:"upgrade_info,omitempty"`
	CommunityPoolSpendInfo *CommunityPoolSpendArgs `json:"community_pool_spend_info,omitempty"`
}

// UpgradeArgs는 업그레이드 인자를 나타내는 구조체입니다.
type UpgradeArgs struct {
	Name   string `json:"name"`
	Height uint64 `json:"height"`
	Info   string `json:"info"`
}

// CommunityPoolSpendArgs는 커뮤니티 풀 지출 인자를 나타내는 구조체입니다.
type CommunityPoolSpendArgs struct {
	Recipient common.Address `json:"recipient"`
	Amount    string         `json:"amount"`
}

// DepositArgs는 보증금 예치 인자를 나타내는 구조체입니다.
type DepositArgs struct {
	Depositor  common.Address `json:"depositor"`
	ProposalID uint64         `json:"proposal_id"`
	Amount     string         `json:"amount"`
}

// VoteArgs는 투표 인자를 나타내는 구조체입니다.
type VoteArgs struct {
	Voter      common.Address `json:"voter"`
	ProposalID uint64         `json:"proposal_id"`
	Option     string         `json:"option"`
}

// convertProposalToResponse는 내부 Proposal 구조체를 응답 구조체로 변환합니다.
func (api *GovAPI) convertProposalToResponse(proposal *GovProposal) *ProposalResponse {
	// 제안 유형 변환
	var proposalType string
	switch proposal.ProposalType {
	case GovProposalTypeParameterChange:
		proposalType = "parameter_change"
	case GovProposalTypeSoftwareUpgrade:
		proposalType = "software_upgrade"
	case GovProposalTypeCommunityPoolSpend:
		proposalType = "community_pool_spend"
	case GovProposalTypeText:
		proposalType = "text"
	}

	// 제안 상태 변환
	var status string
	switch proposal.Status {
	case GovProposalStatusDepositPeriod:
		status = "deposit_period"
	case GovProposalStatusVotingPeriod:
		status = "voting_period"
	case GovProposalStatusPassed:
		status = "passed"
	case GovProposalStatusRejected:
		status = "rejected"
	case GovProposalStatusFailed:
		status = "failed"
	case GovProposalStatusExecuted:
		status = "executed"
	}

	// 응답 생성
	response := &ProposalResponse{
		ID:              proposal.ID,
		Title:           proposal.Title,
		Description:     proposal.Description,
		ProposalType:    proposalType,
		ProposerAddress: proposal.ProposerAddress,
		Status:          status,
		SubmitTime:      uint64(proposal.SubmitTime.Unix()),
		DepositEndTime:  uint64(proposal.DepositEndTime.Unix()),
		TotalDeposit:    proposal.TotalDeposit.String(),
		VotingStartTime: uint64(proposal.VotingStartTime.Unix()),
		VotingEndTime:   uint64(proposal.VotingEndTime.Unix()),
		ExecutionTime:   uint64(proposal.ExecutionTime.Unix()),
		Params:          proposal.Params,
	}

	// 업그레이드 정보 추가
	if proposal.UpgradeInfo != nil {
		response.UpgradeInfo = &UpgradeResponse{
			Name:   proposal.UpgradeInfo.Name,
			Height: proposal.UpgradeInfo.Height,
			Info:   proposal.UpgradeInfo.Info,
		}
	}

	// 커뮤니티 풀 지출 정보 추가
	if proposal.CommunityPoolSpendInfo != nil {
		response.CommunityPoolSpendInfo = &SpendResponse{
			Recipient: proposal.CommunityPoolSpendInfo.Recipient,
			Amount:    proposal.CommunityPoolSpendInfo.Amount.String(),
		}
	}

	return response
}

// getStateDB는 현재 블록의 상태 DB를 가져옵니다.
func (api *GovAPI) getStateDB() (*state.StateDB, *types.Header, error) {
	// 현재 블록 헤더 가져오기
	header := api.chain.CurrentHeader()
	if header == nil {
		return nil, nil, fmt.Errorf("current header is nil")
	}

	// 상태 DB 가져오기
	// 참고: 실제 구현에서는 StateDB에 접근하는 적절한 방법을 사용해야 합니다.
	// 여기서는 임시로 nil을 반환합니다.
	return nil, header, fmt.Errorf("StateDB access not implemented")
}

// GetVotes는 제안에 대한 투표 목록을 반환합니다.
func (api *GovAPI) GetVotes(proposalID uint64) ([]*VoteResponse, error) {
	proposal, err := api.govAdapter.GetProposal(proposalID)
	if err != nil {
		return nil, err
	}

	responses := make([]*VoteResponse, len(proposal.Votes))
	for i, vote := range proposal.Votes {
		// 투표 옵션 변환
		var option string
		switch vote.Option {
		case GovOptionYes:
			option = "yes"
		case GovOptionNo:
			option = "no"
		case GovOptionNoWithVeto:
			option = "no_with_veto"
		case GovOptionAbstain:
			option = "abstain"
		}

		responses[i] = &VoteResponse{
			ProposalID: vote.ProposalID,
			Voter:      vote.Voter,
			Option:     option,
			Timestamp:  uint64(vote.Timestamp.Unix()),
		}
	}

	return responses, nil
}

// GetDeposits는 제안에 대한 보증금 목록을 반환합니다.
func (api *GovAPI) GetDeposits(proposalID uint64) ([]*DepositResponse, error) {
	proposal, err := api.govAdapter.GetProposal(proposalID)
	if err != nil {
		return nil, err
	}

	responses := make([]*DepositResponse, len(proposal.Deposits))
	for i, deposit := range proposal.Deposits {
		responses[i] = &DepositResponse{
			ProposalID: deposit.ProposalID,
			Depositor:  deposit.Depositor,
			Amount:     deposit.Amount.String(),
			Timestamp:  uint64(deposit.Timestamp.Unix()),
		}
	}

	return responses, nil
}
