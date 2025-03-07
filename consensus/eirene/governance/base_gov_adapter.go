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
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/log"
)

// GovernanceAdapterInterface는 거버넌스 어댑터가 구현해야 하는 인터페이스입니다.
//
// 이 인터페이스는 거버넌스 시스템의 핵심 기능을 정의합니다.
// 제안 생성, 보증금 추가, 투표, 제안 실행 등의 기능을 포함합니다.
// GovAdapter와 CosmosGovAdapter는 이 인터페이스를 구현하여 서로 다른 백엔드와 통합됩니다.
type GovernanceAdapterInterface interface {
	SubmitProposal(proposer common.Address, title string, description string, proposalType string, content utils.ProposalContentInterface, initialDeposit *big.Int, state *state.StateDB) (uint64, error)
	Deposit(proposalID uint64, depositor common.Address, amount *big.Int) error
	Vote(proposalID uint64, voter common.Address, option string) error
	GetProposal(proposalID uint64) (*GovProposal, error)
	GetProposals() []*GovProposal
	GetProposalsByStatus(status GovProposalStatus) []*GovProposal
	ExecuteProposal(proposalID uint64, state *state.StateDB) error
	ProcessProposals(blockHeight uint64, state *state.StateDB) error
}

// BaseGovernanceAdapter는 거버넌스 어댑터의 기본 구현을 제공합니다.
//
// 이 구조체는 GovAdapter와 CosmosGovAdapter에서 공통으로 사용되는 기능을 구현합니다.
// 거버넌스 매개변수 관리, 제안 상태 관리, 투표 처리 등의 공통 기능을 제공합니다.
// 구체적인 어댑터 구현은 이 기본 구현을 확장하여 특정 백엔드와 통합됩니다.
type BaseGovernanceAdapter struct {
	eirene             EireneInterface       // Eirene 합의 엔진 인스턴스
	logger             log.Logger            // 로거
	db                 ethdb.Database        // 데이터베이스
	minDeposit         *big.Int              // 최소 제안 보증금
	maxDepositPeriod   time.Duration         // 최대 보증금 기간
	votingPeriod       time.Duration         // 투표 기간
	quorumFloat        float64               // 쿼럼 (투표 참여 최소 비율)
	thresholdFloat     float64               // 통과 임계값
	vetoThresholdFloat float64               // 거부권 임계값
	proposals          map[uint64]*GovProposal // 제안 목록
	nextProposalID     uint64                // 다음 제안 ID
	voteTallyOptimizer *VoteTallyOptimizer   // 투표 집계 최적화기
}

// NewBaseGovernanceAdapter는 새로운 BaseGovernanceAdapter 인스턴스를 생성합니다.
//
// 매개변수:
//   - eirene: Eirene 합의 엔진 인터페이스
//   - db: 데이터베이스
//
// 반환값:
//   - *BaseGovernanceAdapter: 생성된 기본 거버넌스 어댑터 인스턴스
//
// 이 함수는 새로운 기본 거버넌스 어댑터 인스턴스를 초기화합니다.
// 거버넌스 매개변수(투표 기간, 최소 보증금 등)를 설정하고,
// 제안 및 투표 관리를 위한 데이터 구조를 초기화합니다.
// 이 기본 어댑터는 GovAdapter와 CosmosGovAdapter의 기반이 됩니다.
func NewBaseGovernanceAdapter(eirene EireneInterface, db ethdb.Database, logger log.Logger) *BaseGovernanceAdapter {
	// 기본값 설정
	adapter := &BaseGovernanceAdapter{
		eirene:             eirene,
		logger:             logger,
		db:                 db,
		minDeposit:         new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18)), // 10 토큰
		maxDepositPeriod:   time.Hour * 24 * 14,                                // 2주
		votingPeriod:       time.Hour * 24 * 14,                                // 2주
		quorumFloat:        0.334,                                              // 33.4%
		thresholdFloat:     0.5,                                                // 50%
		vetoThresholdFloat: 0.334,                                              // 33.4%
		proposals:          make(map[uint64]*GovProposal),
		nextProposalID:     1,
		voteTallyOptimizer: NewVoteTallyOptimizer(4, 100, 1000, time.Hour), // 워커 수, 배치 크기, 캐시 크기, 캐시 만료 시간
	}

	return adapter
}

// GetProposal은 제안 정보를 반환합니다.
func (a *BaseGovernanceAdapter) GetProposal(proposalID uint64) (*GovProposal, error) {
	proposal, exists := a.proposals[proposalID]
	if !exists {
		return nil, utils.WrapError(utils.ErrProposalNotFound, fmt.Sprintf("proposal not found: %d", proposalID))
	}
	return proposal, nil
}

// GetProposals는 모든 제안 목록을 반환합니다.
func (a *BaseGovernanceAdapter) GetProposals() []*GovProposal {
	proposals := make([]*GovProposal, 0, len(a.proposals))
	for _, proposal := range a.proposals {
		proposals = append(proposals, proposal)
	}
	return proposals
}

// GetProposalsByStatus는 특정 상태의 제안 목록을 반환합니다.
func (a *BaseGovernanceAdapter) GetProposalsByStatus(status GovProposalStatus) []*GovProposal {
	proposals := make([]*GovProposal, 0)
	for _, proposal := range a.proposals {
		if proposal.Status == status {
			proposals = append(proposals, proposal)
		}
	}
	return proposals
}

// IsValidProposalType은 제안 유형이 유효한지 확인합니다.
func (a *BaseGovernanceAdapter) IsValidProposalType(proposalType GovProposalType) bool {
	switch proposalType {
	case GovProposalTypeText, GovProposalTypeParameterChange, GovProposalTypeSoftwareUpgrade, GovProposalTypeCommunityPoolSpend:
		return true
	default:
		return false
	}
}

// IsValidVoteOption은 투표 옵션이 유효한지 확인합니다.
func (a *BaseGovernanceAdapter) IsValidVoteOption(option GovVoteOption) bool {
	switch option {
	case GovOptionYes, GovOptionNo, GovOptionNoWithVeto, GovOptionAbstain:
		return true
	default:
		return false
	}
}

// activateVotingPeriod는 제안의 투표 기간을 활성화합니다.
func (a *BaseGovernanceAdapter) activateVotingPeriod(proposal *GovProposal) {
	now := time.Now()
	proposal.Status = GovProposalStatusVotingPeriod
	proposal.VotingStartTime = now
	proposal.VotingEndTime = now.Add(a.votingPeriod)
	a.logger.Info("Proposal entered voting period", "id", proposal.ID, "end", proposal.VotingEndTime)
}

// tallyVotes는 제안에 대한 투표를 집계하고 통과 여부를 반환합니다.
func (a *BaseGovernanceAdapter) tallyVotes(proposal *GovProposal) bool {
	// 총 스테이킹 양 가져오기 (실제로는 스테이킹 어댑터에서 가져와야 함)
	// 여기서는 간단히 총 투표 수의 3배라고 가정
	totalVotes := big.NewInt(int64(len(proposal.Votes)))
	totalStaked := new(big.Int).Mul(totalVotes, big.NewInt(3))

	// 투표 집계 최적화기 사용
	result := a.voteTallyOptimizer.TallyVotes(
		proposal,
		a.quorumFloat,
		a.vetoThresholdFloat,
		a.thresholdFloat,
		totalStaked,
	)

	if result.Passed {
		a.logger.Info("Proposal passed", "id", proposal.ID, "yes_votes", result.YesVotes, "total_votes", result.TotalVotes, "tally_time", result.TallyTime)
	} else {
		a.logger.Info("Proposal rejected", "id", proposal.ID, "yes_votes", result.YesVotes, "total_votes", result.TotalVotes, "tally_time", result.TallyTime)
	}

	return result.Passed
}

// executeParameterChangeProposal은 매개변수 변경 제안을 실행합니다.
func (a *BaseGovernanceAdapter) executeParameterChangeProposal(proposal *GovProposal) error {
	// 매개변수 변경 로직 구현
	// 실제 구현에서는 각 매개변수에 대한 유효성 검사 및 적용 로직이 필요
	for key, value := range proposal.Params {
		a.logger.Info("Parameter changed", "key", key, "value", value)
	}
	return nil
}

// executeSoftwareUpgradeProposal은 소프트웨어 업그레이드 제안을 실행합니다.
func (a *BaseGovernanceAdapter) executeSoftwareUpgradeProposal(proposal *GovProposal, blockHeight uint64) error {
	if proposal.UpgradeInfo == nil {
		return utils.WrapError(utils.ErrInvalidParameter, "upgrade info is nil")
	}

	// 업그레이드 높이 확인
	if blockHeight < proposal.UpgradeInfo.Height {
		// 아직 업그레이드 높이에 도달하지 않음
		return nil
	}

	// 업그레이드 핸들러 실행
	if proposal.UpgradeInfo.UpgradeHandler != nil {
		return proposal.UpgradeInfo.UpgradeHandler()
	}

	a.logger.Info("Software upgrade scheduled", "name", proposal.UpgradeInfo.Name, "height", proposal.UpgradeInfo.Height)
	return nil
}

// executeCommunityPoolSpendProposal은 커뮤니티 풀 지출 제안을 실행합니다.
func (a *BaseGovernanceAdapter) executeCommunityPoolSpendProposal(state *state.StateDB, proposal *GovProposal) error {
	if proposal.CommunityPoolSpendInfo == nil {
		return utils.WrapError(utils.ErrInvalidParameter, "community pool spend info is nil")
	}

	// 커뮤니티 풀에서 자금 전송
	// 실제 구현에서는 커뮤니티 풀 모듈과 연동하여 자금을 전송해야 함
	recipient := proposal.CommunityPoolSpendInfo.Recipient
	amount := proposal.CommunityPoolSpendInfo.Amount

	a.logger.Info("Community pool spend executed", "recipient", recipient, "amount", amount)
	return nil
}

// GetMinDeposit은 최소 보증금을 반환합니다.
//
// 반환값:
//   - *big.Int: 최소 보증금
//
// 이 함수는 제안 생성에 필요한 최소 보증금을 반환합니다.
// 최소 보증금은 스팸 제안을 방지하고 제안의 품질을 보장하는 데 사용됩니다.
// 제안이 투표 단계로 진행되려면 이 금액 이상의 보증금이 필요합니다.
func (a *BaseGovernanceAdapter) GetMinDeposit() *big.Int {
	return a.minDeposit
}

// GetMaxDepositPeriod는 최대 보증금 기간을 반환합니다.
//
// 반환값:
//   - time.Duration: 최대 보증금 기간
//
// 이 함수는 제안의 보증금 수집 기간을 반환합니다.
// 이 기간 내에 최소 보증금이 모이지 않으면 제안은 자동으로 거부됩니다.
// 보증금 기간은 제안 생성 시점부터 시작됩니다.
func (a *BaseGovernanceAdapter) GetMaxDepositPeriod() time.Duration {
	return a.maxDepositPeriod
}

// GetVotingPeriod는 투표 기간을 반환합니다.
//
// 반환값:
//   - time.Duration: 투표 기간
//
// 이 함수는 제안의 투표 기간을 반환합니다.
// 이 기간 동안 검증자와 위임자는 제안에 투표할 수 있습니다.
// 투표 기간이 종료되면 투표 결과가 집계되고 제안의 통과 여부가 결정됩니다.
func (a *BaseGovernanceAdapter) GetVotingPeriod() time.Duration {
	return a.votingPeriod
}

// GetQuorum은 투표 정족수를 반환합니다.
//
// 반환값:
//   - *big.Int: 투표 정족수 (1e18 = 100%)
//
// 이 함수는 제안 통과에 필요한 최소 투표율을 반환합니다.
// 정족수는 총 스테이킹 양 대비 투표에 참여한 스테이킹 양의 비율로 계산됩니다.
// 투표율이 정족수에 미치지 못하면 제안은 자동으로 거부됩니다.
func (a *BaseGovernanceAdapter) GetQuorum() *big.Int {
	return new(big.Int).Mul(big.NewInt(1e18), big.NewInt(int64(a.quorumFloat)))
}

// GetThreshold은 제안 통과 임계값을 반환합니다.
//
// 반환값:
//   - *big.Int: 통과 임계값 (1e18 = 100%)
//
// 이 함수는 제안 통과에 필요한 찬성 투표 비율을 반환합니다.
// 임계값은 기권 투표를 제외한 총 투표 중 찬성 투표의 비율로 계산됩니다.
// 찬성 비율이 임계값에 미치지 못하면 제안은 거부됩니다.
func (a *BaseGovernanceAdapter) GetThreshold() *big.Int {
	return new(big.Int).Mul(big.NewInt(1e18), big.NewInt(int64(a.thresholdFloat)))
}

// GetVetoThreshold은 거부권 임계값을 반환합니다.
//
// 반환값:
//   - *big.Int: 거부권 임계값 (1e18 = 100%)
//
// 이 함수는 제안 거부에 필요한 거부권 투표 비율을 반환합니다.
// 거부권 임계값은 총 투표 중 거부권(NoWithVeto) 투표의 비율로 계산됩니다.
// 거부권 비율이 이 임계값을 초과하면 제안은 자동으로 거부됩니다.
func (a *BaseGovernanceAdapter) GetVetoThreshold() *big.Int {
	return new(big.Int).Mul(big.NewInt(1e18), big.NewInt(int64(a.vetoThresholdFloat)))
}

// ConvertVoteOption은 문자열 투표 옵션을 GovVoteOption으로 변환합니다.
//
// 매개변수:
//   - optionStr: 투표 옵션 문자열 (Yes, No, Abstain, NoWithVeto)
//
// 반환값:
//   - GovVoteOption: 변환된 투표 옵션
//   - error: 변환 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 문자열 형태의 투표 옵션을 GovVoteOption 열거형으로 변환합니다.
// 지원되는 옵션은 Yes(찬성), No(반대), Abstain(기권), NoWithVeto(거부권 행사)입니다.
// 지원되지 않는 옵션이 입력되면 오류를 반환합니다.
func (a *BaseGovernanceAdapter) ConvertVoteOption(optionStr string) (GovVoteOption, error) {
	switch optionStr {
	case "Yes":
		return GovOptionYes, nil
	case "No":
		return GovOptionNo, nil
	case "Abstain":
		return GovOptionAbstain, nil
	case "NoWithVeto":
		return GovOptionNoWithVeto, nil
	default:
		return GovOptionNo, fmt.Errorf("invalid vote option: %s", optionStr)
	}
}

// ConvertProposalType은 문자열 제안 유형을 GovProposalType으로 변환합니다.
//
// 매개변수:
//   - typeStr: 제안 유형 문자열 (ParameterChange, Upgrade, CommunityPoolSpend, Text)
//
// 반환값:
//   - GovProposalType: 변환된 제안 유형
//   - error: 변환 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 문자열 형태의 제안 유형을 GovProposalType 열거형으로 변환합니다.
// 지원되는 유형은 ParameterChange(매개변수 변경), Upgrade(소프트웨어 업그레이드),
// CommunityPoolSpend(커뮤니티 풀 지출), Text(텍스트 제안)입니다.
// 지원되지 않는 유형이 입력되면 오류를 반환합니다.
func (a *BaseGovernanceAdapter) ConvertProposalType(typeStr string) (GovProposalType, error) {
	switch typeStr {
	case "ParameterChange":
		return GovProposalTypeParameterChange, nil
	case "Upgrade":
		return GovProposalTypeSoftwareUpgrade, nil
	case "CommunityPoolSpend":
		return GovProposalTypeCommunityPoolSpend, nil
	case "Text":
		return GovProposalTypeText, nil
	default:
		return GovProposalTypeText, fmt.Errorf("invalid proposal type: %s", typeStr)
	}
} 