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
	"github.com/zenanetwork/go-zenanet/consensus"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/log"
)

// API는 거버넌스 시스템의 RPC API를 구현합니다
type API struct {
	chain       consensus.ChainHeaderReader
	governance  *GovernanceManager
	stateAt     func(common.Hash) (*state.StateDB, error)
	currentBlock func() *types.Block
}

// NewAPI는 새로운 거버넌스 API를 생성합니다
func NewAPI(chain consensus.ChainHeaderReader, governance *GovernanceManager, stateAt func(common.Hash) (*state.StateDB, error), currentBlock func() *types.Block) *API {
	return &API{
		chain:       chain,
		governance:  governance,
		stateAt:     stateAt,
		currentBlock: currentBlock,
	}
}

// ProposalResponse는 제안 정보 응답을 나타냅니다
type ProposalResponse struct {
	ID          uint64         `json:"id"`
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Proposer    common.Address `json:"proposer"`
	SubmitTime  time.Time      `json:"submit_time"`
	DepositEnd  time.Time      `json:"deposit_end"`
	VotingStart time.Time      `json:"voting_start"`
	VotingEnd   time.Time      `json:"voting_end"`
	ExecuteTime time.Time      `json:"execute_time"`
	Status      string         `json:"status"`
	
	TotalDeposit string                 `json:"total_deposit"`
	Deposits     map[string]string      `json:"deposits"`
	
	YesVotes     string                 `json:"yes_votes"`
	NoVotes      string                 `json:"no_votes"`
	AbstainVotes string                 `json:"abstain_votes"`
	VetoVotes    string                 `json:"veto_votes"`
	Votes        map[string]string      `json:"votes"`
	
	Content      interface{}            `json:"content"`
}

// convertProposalToResponse는 제안을 응답 형식으로 변환합니다
func convertProposalToResponse(proposal *utils.StandardProposal) ProposalResponse {
	// 보증금 변환
	deposits := make(map[string]string)
	for addr, amount := range proposal.Deposits {
		deposits[addr.Hex()] = amount.String()
	}
	
	// 투표 변환
	votes := make(map[string]string)
	for addr, vote := range proposal.Votes {
		votes[addr.Hex()] = vote
	}
	
	// 제안 내용 변환
	var content interface{}
	if proposal.Content != nil {
		switch proposal.Type {
		case ProposalTypeParameterChange:
			if paramChange, ok := proposal.Content.(ParameterChangeProposal); ok {
				content = paramChange.Changes
			} else {
				// 안전한 타입 변환을 위한 추가 처리
				log.Warn("Failed to convert proposal content to ParameterChangeProposal", "id", proposal.ID)
			}
		case ProposalTypeUpgrade:
			if upgrade, ok := proposal.Content.(UpgradeProposal); ok {
				content = map[string]interface{}{
					"name":                 upgrade.Name,
					"height":               upgrade.Height,
					"info":                 upgrade.Info,
					"upgrade_time":         upgrade.UpgradeTime,
					"cancel_upgrade_height": upgrade.CancelUpgradeHeight,
				}
			} else {
				// 안전한 타입 변환을 위한 추가 처리
				log.Warn("Failed to convert proposal content to UpgradeProposal", "id", proposal.ID)
			}
		case ProposalTypeFunding:
			if funding, ok := proposal.Content.(FundingProposal); ok {
				content = map[string]interface{}{
					"recipient": funding.Recipient.Hex(),
					"amount":    funding.Amount.String(),
					"reason":    funding.Reason,
				}
			} else {
				// 안전한 타입 변환을 위한 추가 처리
				log.Warn("Failed to convert proposal content to FundingProposal", "id", proposal.ID)
			}
		case ProposalTypeText:
			if text, ok := proposal.Content.(TextProposal); ok {
				content = map[string]interface{}{
					"text": text.Text,
				}
			} else {
				// 안전한 타입 변환을 위한 추가 처리
				log.Warn("Failed to convert proposal content to TextProposal", "id", proposal.ID)
			}
		default:
			log.Warn("Unknown proposal type", "type", proposal.Type, "id", proposal.ID)
		}
	}
	
	// 응답 생성
	return ProposalResponse{
		ID:          proposal.ID,
		Type:        proposal.Type,
		Title:       proposal.Title,
		Description: proposal.Description,
		Proposer:    proposal.Proposer,
		SubmitTime:  proposal.SubmitTime,
		DepositEnd:  proposal.DepositEnd,
		VotingStart: proposal.VotingStart,
		VotingEnd:   proposal.VotingEnd,
		ExecuteTime: proposal.ExecuteTime,
		Status:      proposal.Status,
		
		TotalDeposit: proposal.TotalDeposit.String(),
		Deposits:     deposits,
		
		YesVotes:     proposal.YesVotes.String(),
		NoVotes:      proposal.NoVotes.String(),
		AbstainVotes: proposal.AbstainVotes.String(),
		VetoVotes:    proposal.VetoVotes.String(),
		Votes:        votes,
		
		Content:      content,
	}
}

// GetProposal은 제안 정보를 반환합니다
func (api *API) GetProposal(proposalID uint64) (*ProposalResponse, error) {
	proposal, err := api.governance.GetProposal(proposalID)
	if err != nil {
		return nil, err
	}
	
	response := convertProposalToResponse(proposal)
	return &response, nil
}

// GetProposals은 모든 제안 목록을 반환합니다
func (api *API) GetProposals() ([]ProposalResponse, error) {
	proposals := api.governance.GetProposals()
	
	responses := make([]ProposalResponse, len(proposals))
	for i, proposal := range proposals {
		responses[i] = convertProposalToResponse(proposal)
	}
	
	return responses, nil
}

// GetProposalsByStatus는 특정 상태의 제안 목록을 반환합니다
func (api *API) GetProposalsByStatus(status string) ([]ProposalResponse, error) {
	// 상태 유효성 검사
	if status != ProposalStatusDepositPeriod && 
	   status != ProposalStatusVotingPeriod && 
	   status != ProposalStatusPassed && 
	   status != ProposalStatusRejected && 
	   status != ProposalStatusExecuted {
		return nil, errors.New("invalid proposal status")
	}
	
	proposals := api.governance.GetProposalsByStatus(status)
	
	responses := make([]ProposalResponse, len(proposals))
	for i, proposal := range proposals {
		responses[i] = convertProposalToResponse(proposal)
	}
	
	return responses, nil
}

// SubmitProposalArgs는 제안 제출 인자를 나타냅니다
type SubmitProposalArgs struct {
	Type           string         `json:"type"`
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	Proposer       common.Address `json:"proposer"`
	InitialDeposit string         `json:"initial_deposit"`
	Content        interface{}    `json:"content"`
}

// SubmitProposal은 새로운 제안을 제출합니다
func (api *API) SubmitProposal(args SubmitProposalArgs) (uint64, error) {
	// 현재 상태 가져오기
	currentBlock := api.currentBlock()
	if currentBlock == nil {
		return 0, errors.New("current block not available")
	}
	
	state, err := api.stateAt(currentBlock.Root())
	if err != nil {
		return 0, err
	}
	
	// 초기 보증금 파싱
	initialDeposit, ok := new(big.Int).SetString(args.InitialDeposit, 10)
	if !ok {
		return 0, errors.New("invalid initial deposit")
	}
	
	// 제안 내용 생성
	var content utils.ProposalContentInterface
	switch args.Type {
	case ProposalTypeParameterChange:
		// 매개변수 변경 제안
		contentMap, ok := args.Content.(map[string]interface{})
		if !ok {
			return 0, errors.New("invalid parameter change content")
		}
		
		changesRaw, ok := contentMap["changes"].([]interface{})
		if !ok {
			return 0, errors.New("invalid parameter changes")
		}
		
		changes := make([]ParamChange, len(changesRaw))
		for i, changeRaw := range changesRaw {
			changeMap, ok := changeRaw.(map[string]interface{})
			if !ok {
				return 0, errors.New("invalid parameter change")
			}
			
			subspace, ok := changeMap["subspace"].(string)
			if !ok {
				return 0, errors.New("invalid parameter subspace")
			}
			
			key, ok := changeMap["key"].(string)
			if !ok {
				return 0, errors.New("invalid parameter key")
			}
			
			value, ok := changeMap["value"].(string)
			if !ok {
				return 0, errors.New("invalid parameter value")
			}
			
			changes[i] = ParamChange{
				Subspace: subspace,
				Key:      key,
				Value:    value,
			}
		}
		
		content = ParameterChangeProposal{
			Changes: changes,
		}
		
	case ProposalTypeUpgrade:
		// 업그레이드 제안
		contentMap, ok := args.Content.(map[string]interface{})
		if !ok {
			return 0, errors.New("invalid upgrade content")
		}
		
		name, ok := contentMap["name"].(string)
		if !ok {
			return 0, errors.New("invalid upgrade name")
		}
		
		heightFloat, ok := contentMap["height"].(float64)
		if !ok {
			return 0, errors.New("invalid upgrade height")
		}
		height := uint64(heightFloat)
		
		info, ok := contentMap["info"].(string)
		if !ok {
			return 0, errors.New("invalid upgrade info")
		}
		
		upgradeTimeStr, ok := contentMap["upgrade_time"].(string)
		if !ok {
			return 0, errors.New("invalid upgrade time")
		}
		upgradeTime, err := time.Parse(time.RFC3339, upgradeTimeStr)
		if err != nil {
			return 0, errors.New("invalid upgrade time format")
		}
		
		cancelHeightFloat, ok := contentMap["cancel_upgrade_height"].(float64)
		if !ok {
			cancelHeightFloat = 0
		}
		cancelHeight := uint64(cancelHeightFloat)
		
		content = UpgradeProposal{
			Name:                name,
			Height:              height,
			Info:                info,
			UpgradeTime:         upgradeTime,
			CancelUpgradeHeight: cancelHeight,
		}
		
	case ProposalTypeFunding:
		// 자금 지원 제안
		contentMap, ok := args.Content.(map[string]interface{})
		if !ok {
			return 0, errors.New("invalid funding content")
		}
		
		recipientStr, ok := contentMap["recipient"].(string)
		if !ok {
			return 0, errors.New("invalid recipient")
		}
		recipient := common.HexToAddress(recipientStr)
		
		amountStr, ok := contentMap["amount"].(string)
		if !ok {
			return 0, errors.New("invalid amount")
		}
		amount, ok := new(big.Int).SetString(amountStr, 10)
		if !ok {
			return 0, errors.New("invalid amount format")
		}
		
		reason, ok := contentMap["reason"].(string)
		if !ok {
			return 0, errors.New("invalid reason")
		}
		
		content = FundingProposal{
			Recipient: recipient,
			Amount:    amount,
			Reason:    reason,
		}
		
	case ProposalTypeText:
		// 텍스트 제안
		contentMap, ok := args.Content.(map[string]interface{})
		if !ok {
			return 0, errors.New("invalid text content")
		}
		
		text, ok := contentMap["text"].(string)
		if !ok {
			return 0, errors.New("invalid text")
		}
		
		content = TextProposal{
			Text: text,
		}
		
	default:
		return 0, errors.New("invalid proposal type")
	}
	
	// 제안 제출
	proposalID, err := api.governance.SubmitProposal(
		args.Proposer,
		args.Title,
		args.Description,
		args.Type,
		content,
		initialDeposit,
		state,
	)
	if err != nil {
		return 0, err
	}
	
	log.Info("Proposal submitted via API", "id", proposalID, "type", args.Type, "proposer", args.Proposer)
	return proposalID, nil
}

// DepositArgs는 보증금 추가 인자를 나타냅니다
type DepositArgs struct {
	ProposalID uint64         `json:"proposal_id"`
	Depositor  common.Address `json:"depositor"`
	Amount     string         `json:"amount"`
}

// Deposit는 제안에 보증금을 추가합니다
func (api *API) Deposit(args DepositArgs) (bool, error) {
	// 현재 상태 가져오기
	currentBlock := api.currentBlock()
	if currentBlock == nil {
		return false, errors.New("current block not available")
	}
	
	state, err := api.stateAt(currentBlock.Root())
	if err != nil {
		return false, err
	}
	
	// 보증금 파싱
	amount, ok := new(big.Int).SetString(args.Amount, 10)
	if !ok {
		return false, errors.New("invalid amount")
	}
	
	// 보증금 추가
	err = api.governance.Deposit(args.ProposalID, args.Depositor, amount, state)
	if err != nil {
		return false, err
	}
	
	log.Info("Deposit added via API", "proposal", args.ProposalID, "depositor", args.Depositor, "amount", args.Amount)
	return true, nil
}

// VoteArgs는 투표 인자를 나타냅니다
type VoteArgs struct {
	ProposalID uint64         `json:"proposal_id"`
	Voter      common.Address `json:"voter"`
	Option     string         `json:"option"`
}

// Vote는 제안에 투표합니다
func (api *API) Vote(args VoteArgs) (bool, error) {
	// 투표 옵션 유효성 검사
	if args.Option != VoteOptionYes && 
	   args.Option != VoteOptionNo && 
	   args.Option != VoteOptionAbstain && 
	   args.Option != VoteOptionVeto {
		return false, errors.New("invalid vote option")
	}
	
	// 투표
	err := api.governance.Vote(args.ProposalID, args.Voter, args.Option)
	if err != nil {
		return false, err
	}
	
	log.Info("Vote cast via API", "proposal", args.ProposalID, "voter", args.Voter, "option", args.Option)
	return true, nil
}

// EndVotingArgs는 투표 종료 인자를 나타냅니다
type EndVotingArgs struct {
	ProposalID uint64 `json:"proposal_id"`
}

// EndVoting은 투표 기간이 종료된 제안을 처리합니다
func (api *API) EndVoting(args EndVotingArgs) (bool, error) {
	// 현재 상태 가져오기
	currentBlock := api.currentBlock()
	if currentBlock == nil {
		return false, errors.New("current block not available")
	}
	
	state, err := api.stateAt(currentBlock.Root())
	if err != nil {
		return false, err
	}
	
	// 투표 종료
	err = api.governance.EndVoting(args.ProposalID, state)
	if err != nil {
		return false, err
	}
	
	log.Info("Voting ended via API", "proposal", args.ProposalID)
	return true, nil
}

// ExecuteProposalArgs는 제안 실행 인자를 나타냅니다
type ExecuteProposalArgs struct {
	ProposalID uint64 `json:"proposal_id"`
}

// ExecuteProposal은 통과된 제안을 실행합니다
func (api *API) ExecuteProposal(args ExecuteProposalArgs) (bool, error) {
	// 현재 상태 가져오기
	currentBlock := api.currentBlock()
	if currentBlock == nil {
		return false, errors.New("current block not available")
	}
	
	state, err := api.stateAt(currentBlock.Root())
	if err != nil {
		return false, err
	}
	
	// 제안 실행
	err = api.governance.ExecuteProposal(args.ProposalID, state)
	if err != nil {
		return false, err
	}
	
	log.Info("Proposal executed via API", "proposal", args.ProposalID)
	return true, nil
}

// GetParams는 거버넌스 매개변수를 반환합니다
func (api *API) GetParams() map[string]interface{} {
	params := api.governance.GetParams()
	
	return map[string]interface{}{
		"min_deposit":     params.MinDeposit.String(),
		"deposit_period":  params.DepositPeriod,
		"voting_period":   params.VotingPeriod,
		"quorum":          params.Quorum,
		"threshold":       params.Threshold,
		"veto_threshold":  params.VetoThreshold,
		"execution_delay": params.ExecutionDelay,
	}
}

// GetGovernanceManager는 거버넌스 매니저를 반환합니다
func (api *API) GetGovernanceManager() *GovernanceManager {
	return api.governance
}

// SetParamsArgs는 거버넌스 매개변수 설정 인자를 나타냅니다
type SetParamsArgs struct {
	MinDeposit     string  `json:"min_deposit"`
	DepositPeriod  uint64  `json:"deposit_period"`
	VotingPeriod   uint64  `json:"voting_period"`
	Quorum         float64 `json:"quorum"`
	Threshold      float64 `json:"threshold"`
	VetoThreshold  float64 `json:"veto_threshold"`
	ExecutionDelay uint64  `json:"execution_delay"`
}

// SetParams는 거버넌스 매개변수를 설정합니다
func (api *API) SetParams(args SetParamsArgs) (bool, error) {
	// 매개변수 파싱
	minDeposit, ok := new(big.Int).SetString(args.MinDeposit, 10)
	if !ok {
		return false, errors.New("invalid min deposit")
	}
	
	// 매개변수 유효성 검사
	if args.DepositPeriod == 0 {
		return false, errors.New("deposit period must be positive")
	}
	if args.VotingPeriod == 0 {
		return false, errors.New("voting period must be positive")
	}
	if args.Quorum <= 0 || args.Quorum > 1 {
		return false, errors.New("quorum must be between 0 and 1")
	}
	if args.Threshold <= 0 || args.Threshold > 1 {
		return false, errors.New("threshold must be between 0 and 1")
	}
	if args.VetoThreshold <= 0 || args.VetoThreshold > 1 {
		return false, errors.New("veto threshold must be between 0 and 1")
	}
	
	// 매개변수 설정
	params := &GovernanceParams{
		MinDeposit:     minDeposit,
		DepositPeriod:  args.DepositPeriod,
		VotingPeriod:   args.VotingPeriod,
		Quorum:         args.Quorum,
		Threshold:      args.Threshold,
		VetoThreshold:  args.VetoThreshold,
		ExecutionDelay: args.ExecutionDelay,
	}
	
	api.governance.SetParams(params)
	
	log.Info("Governance parameters updated via API")
	return true, nil
}
