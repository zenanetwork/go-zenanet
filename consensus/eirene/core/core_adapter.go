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

package core

import (
	"math/big"

	"github.com/pkg/errors"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/params"
)

// CoreAdapter는 Eirene 합의 알고리즘의 코어 어댑터 구현체입니다.
// 이 구조체는 BaseCoreAdapter를 상속하여 확장된 기능을 제공합니다.
type CoreAdapter struct {
	*BaseCoreAdapter
	chain         consensus.ChainHeaderReader
	currentBlock  func() *types.Block
	stateAt       func(common.Hash) (*state.StateDB, error)
}

// 컴파일 타임에 인터페이스 구현 여부 확인
var _ CoreAdapterInterface = (*CoreAdapter)(nil)

// NewCoreAdapter는 새로운 CoreAdapter 인스턴스를 생성합니다.
func NewCoreAdapter(
	db ethdb.Database,
	validatorSet utils.ValidatorSetInterface,
	governance utils.GovernanceInterface,
	config *params.EireneConfig,
	chain consensus.ChainHeaderReader,
	currentBlock func() *types.Block,
	stateAt func(common.Hash) (*state.StateDB, error),
) *CoreAdapter {
	baseAdapter := NewBaseCoreAdapter(db, validatorSet, governance, config)
	
	return &CoreAdapter{
		BaseCoreAdapter: baseAdapter,
		chain:           chain,
		currentBlock:    currentBlock,
		stateAt:         stateAt,
	}
}

// SubmitProposal은 새로운 제안을 제출합니다.
func (a *CoreAdapter) SubmitProposal(proposer common.Address, title string, description string, proposalType string, parameters map[string]string, upgrade *utils.UpgradeInfo, funding *utils.FundingInfo, deposit *big.Int) (uint64, error) {
	// 거버넌스 인터페이스가 없는 경우 오류 반환
	if a.governance == nil {
		a.logger.Error("governance interface not set")
		return 0, errors.New("governance interface not set")
	}
	
	// 제안 내용 생성
	var content utils.ProposalContentInterface
	switch proposalType {
	case utils.ProposalTypeParameterChange:
		content = &parameterChangeProposal{
			Changes: parameters,
		}
	case utils.ProposalTypeUpgrade:
		if upgrade == nil {
			a.logger.Error("upgrade info is required for upgrade proposal")
			return 0, errors.New("upgrade info is required for upgrade proposal")
		}
		content = &upgradeProposal{
			Info: *upgrade,
		}
	case utils.ProposalTypeFunding:
		if funding == nil {
			a.logger.Error("funding info is required for funding proposal")
			return 0, errors.New("funding info is required for funding proposal")
		}
		content = &fundingProposal{
			Info: *funding,
		}
	case utils.ProposalTypeText:
		content = &textProposal{}
	default:
		a.logger.Error("invalid proposal type")
		return 0, errors.New("invalid proposal type")
	}
	
	// 현재 블록 헤더 가져오기
	header := a.chain.CurrentHeader()
	if header == nil {
		a.logger.Error("current header not found")
		return 0, errors.New("current header not found")
	}
	
	// 상태 DB 가져오기
	stateDB, err := a.stateAt(header.Root)
	if err != nil {
		a.logger.Error("failed to get state", "error", err)
		return 0, err
	}
	
	// 제안 제출
	a.logger.Info("Submitting proposal", "proposer", proposer.Hex(), "type", proposalType, "title", title)
	return a.governance.SubmitProposal(proposer, title, description, proposalType, content, deposit, stateDB)
}

// ExecuteProposal은 제안을 실행합니다.
func (a *CoreAdapter) ExecuteProposal(proposalID uint64) error {
	// 거버넌스 인터페이스가 없는 경우 오류 반환
	if a.governance == nil {
		a.logger.Error("governance interface not set")
		return errors.New("governance interface not set")
	}
	
	// 현재 블록 헤더 가져오기
	header := a.chain.CurrentHeader()
	if header == nil {
		a.logger.Error("current header not found")
		return errors.New("current header not found")
	}
	
	// 상태 DB 가져오기
	stateDB, err := a.stateAt(header.Root)
	if err != nil {
		a.logger.Error("failed to get state", "error", err)
		return err
	}
	
	// 제안 실행
	a.logger.Info("Executing proposal", "id", proposalID)
	return a.governance.ExecuteProposal(proposalID, stateDB)
}

// ProcessProposals는 현재 블록에서 제안을 처리합니다.
func (a *CoreAdapter) ProcessProposals(currentBlock uint64) error {
	// 거버넌스 인터페이스가 없는 경우 오류 반환
	if a.governance == nil {
		a.logger.Error("governance interface not set")
		return errors.New("governance interface not set")
	}
	
	// 현재 블록 헤더 가져오기
	header := a.chain.CurrentHeader()
	if header == nil {
		a.logger.Error("current header not found")
		return errors.New("current header not found")
	}
	
	// 상태 DB 가져오기
	stateDB, err := a.stateAt(header.Root)
	if err != nil {
		a.logger.Error("failed to get state", "error", err)
		return err
	}
	
	// 모든 제안 가져오기
	proposals := a.governance.GetProposals()
	
	// 각 제안 처리
	for _, proposal := range proposals {
		// 투표 기간이 끝난 제안 처리
		if proposal.GetStatus() == utils.ProposalStatusVotingPeriod && currentBlock > proposal.GetVotingEndBlock() {
			// 제안 실행
			err := a.governance.ExecuteProposal(proposal.GetID(), stateDB)
			if err != nil {
				a.logger.Error("Failed to execute proposal", "id", proposal.GetID(), "error", err)
			}
		}
	}
	
	return nil
}

// GetVotes는 지정된 제안의 투표를 반환합니다.
func (a *CoreAdapter) GetVotes(proposalID uint64) ([]ProposalVote, error) {
	// 거버넌스 인터페이스가 없는 경우 오류 반환
	if a.governance == nil {
		a.logger.Error("governance interface not set")
		return nil, errors.New("governance interface not set")
	}
	
	// 제안 가져오기
	proposal, err := a.governance.GetProposal(proposalID)
	if err != nil {
		a.logger.Error("failed to get proposal", "id", proposalID, "error", err)
		return nil, err
	}
	
	// 임시 구현: 빈 배열 반환
	// 실제 구현에서는 제안의 투표 정보를 가져와야 함
	a.logger.Info("Getting votes for proposal", "id", proposalID, "status", proposal.GetStatus())
	return []ProposalVote{}, nil
} 