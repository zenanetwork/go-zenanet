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

	"github.com/holiman/uint256"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/tracing"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/params"
)

// EireneInterface는 GovAdapter가 필요로 하는 Eirene 기능을 정의합니다.
type EireneInterface interface {
	GetDB() ethdb.Database
	GetConfig() *params.EireneConfig
	GetValidatorSet() utils.ValidatorSetInterface
}

// GovAdapter는 Cosmos SDK의 gov 모듈과 go-zenanet의 Eirene 합의 알고리즘을 연결하는 어댑터입니다.
type GovAdapter struct {
	*BaseGovernanceAdapter // 기본 거버넌스 어댑터 상속
}

// GovProposal은 거버넌스 제안 정보를 나타내는 구조체입니다.
type GovProposal struct {
	ID                     uint64                     // 제안 ID
	Title                  string                     // 제안 제목
	Description            string                     // 제안 설명
	ProposalType           GovProposalType            // 제안 유형
	ProposerAddress        common.Address             // 제안자 주소
	Status                 GovProposalStatus          // 제안 상태
	SubmitTime             time.Time                  // 제출 시간
	DepositEndTime         time.Time                  // 보증금 마감 시간
	TotalDeposit           *big.Int                   // 총 보증금
	VotingStartTime        time.Time                  // 투표 시작 시간
	VotingEndTime          time.Time                  // 투표 종료 시간
	Votes                  []*GovVote                 // 투표 목록
	Deposits               []*GovDeposit              // 보증금 목록
	ExecutionTime          time.Time                  // 실행 시간
	Params                 map[string]string          // 매개변수 변경 제안의 경우 변경할 매개변수
	UpgradeInfo            *GovUpgradeInfo            // 업그레이드 제안의 경우 업그레이드 정보
	CommunityPoolSpendInfo *GovCommunityPoolSpendInfo // 커뮤니티 풀 지출 제안의 경우 지출 정보
}

// GovProposalType은 제안 유형을 나타내는 열거형입니다.
type GovProposalType uint8

const (
	GovProposalTypeParameterChange GovProposalType = iota
	GovProposalTypeSoftwareUpgrade
	GovProposalTypeCommunityPoolSpend
	GovProposalTypeText
)

// GovProposalStatus는 제안 상태를 나타내는 열거형입니다.
type GovProposalStatus uint8

const (
	GovProposalStatusDepositPeriod GovProposalStatus = iota
	GovProposalStatusVotingPeriod
	GovProposalStatusPassed
	GovProposalStatusRejected
	GovProposalStatusFailed
	GovProposalStatusExecuted
)

// GovVote는 투표 정보를 나타내는 구조체입니다.
// 참고: 이 타입은 utils.StandardVote로 대체되었습니다.
// 하위 호환성을 위해 유지되며, 내부적으로 utils.StandardVote를 사용합니다.
type GovVote struct {
	ProposalID uint64         // 제안 ID
	Voter      common.Address // 투표자 주소
	Option     GovVoteOption  // 투표 옵션
	Timestamp  time.Time      // 투표 시간
}

// ToStandardVote는 GovVote를 utils.StandardVote로 변환합니다.
func (v *GovVote) ToStandardVote() *utils.StandardVote {
	return &utils.StandardVote{
		ProposalID: v.ProposalID,
		Voter:      v.Voter,
		Option:     string(v.Option),
		Weight:     big.NewInt(1), // 기본 가중치 1
		Timestamp:  v.Timestamp,
	}
}

// FromStandardVote는 utils.StandardVote를 GovVote로 변환합니다.
func GovVoteFromStandardVote(sv *utils.StandardVote) *GovVote {
	return &GovVote{
		ProposalID: sv.ProposalID,
		Voter:      sv.Voter,
		Option:     GovVoteOption(sv.Option),
		Timestamp:  sv.Timestamp,
	}
}

// GovVoteOption은 투표 옵션을 나타내는 열거형입니다.
// 참고: 이 타입은 string 상수로 대체되었습니다.
// 하위 호환성을 위해 유지됩니다.
type GovVoteOption string

const (
	GovOptionYes        GovVoteOption = "yes"
	GovOptionNo         GovVoteOption = "no"
	GovOptionNoWithVeto GovVoteOption = "no_with_veto"
	GovOptionAbstain    GovVoteOption = "abstain"
)

// GovDeposit는 보증금 정보를 나타내는 구조체입니다.
type GovDeposit struct {
	ProposalID uint64         // 제안 ID
	Depositor  common.Address // 보증금 예치자 주소
	Amount     *big.Int       // 보증금 양
	Timestamp  time.Time      // 예치 시간
}

// GovUpgradeInfo는 업그레이드 정보를 나타내는 구조체입니다.
type GovUpgradeInfo struct {
	Name           string       // 업그레이드 이름
	Height         uint64       // 업그레이드 높이
	Info           string       // 업그레이드 정보
	UpgradeHandler func() error // 업그레이드 핸들러
}

// GovCommunityPoolSpendInfo는 커뮤니티 풀 지출 정보를 나타내는 구조체입니다.
type GovCommunityPoolSpendInfo struct {
	Recipient common.Address // 수령인 주소
	Amount    *big.Int       // 지출 양
}

// NewGovAdapter는 새로운 GovAdapter 인스턴스를 생성합니다.
func NewGovAdapter(eirene EireneInterface, db ethdb.Database, config *params.EireneConfig) *GovAdapter {
	logger := log.New("module", "eirene/gov")
	baseAdapter := NewBaseGovernanceAdapter(eirene, db, logger)
	return &GovAdapter{
		BaseGovernanceAdapter: baseAdapter,
	}
}

// SubmitProposal은 새로운 거버넌스 제안을 제출합니다.
//
// 매개변수:
//   - proposer: 제안자 주소
//   - title: 제안 제목
//   - description: 제안 설명
//   - proposalTypeStr: 제안 유형 문자열 (ParameterChange, Upgrade, CommunityPoolSpend, Text)
//   - content: 제안 내용 인터페이스
//   - initialDeposit: 초기 보증금
//   - state: 상태 데이터베이스
//
// 반환값:
//   - uint64: 생성된 제안 ID
//   - error: 제안 생성 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 새로운 거버넌스 제안을 생성하고 제안 ID를 반환합니다.
// 제안 유형에 따라 필요한 추가 정보(매개변수 변경, 업그레이드, 커뮤니티 풀 지출 등)를 처리합니다.
// 초기 보증금이 최소 보증금보다 적을 경우 오류를 반환합니다.
// 제안이 성공적으로 생성되면 보증금 기간이 시작되고, 충분한 보증금이 모이면 투표 기간으로 전환됩니다.
func (a *GovAdapter) SubmitProposal(proposer common.Address, title string, description string, proposalTypeStr string, content utils.ProposalContentInterface, initialDeposit *big.Int, state *state.StateDB) (uint64, error) {
	a.logger.Info("Submitting proposal", "proposer", proposer.Hex(), "title", title)

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
	if initialDeposit.Cmp(a.minDeposit) < 0 {
		return 0, utils.WrapError(utils.ErrInsufficientFunds, fmt.Sprintf("initial deposit %s is less than minimum required %s", initialDeposit.String(), a.minDeposit.String()))
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
		DepositEndTime:  now.Add(a.maxDepositPeriod),
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

	// 제안 유형별 추가 정보 설정
	switch proposalType {
	case GovProposalTypeParameterChange:
		proposal.Params = make(map[string]string)
		// 매개변수 변경 제안의 경우 content에서 매개변수 정보 추출
		// 실제 구현에서는 content의 타입을 확인하고 적절히 처리해야 함
	case GovProposalTypeSoftwareUpgrade:
		// 업그레이드 제안의 경우 content에서 업그레이드 정보 추출
		// 실제 구현에서는 content의 타입을 확인하고 적절히 처리해야 함
	case GovProposalTypeCommunityPoolSpend:
		// 커뮤니티 풀 지출 제안의 경우 content에서 지출 정보 추출
		// 실제 구현에서는 content의 타입을 확인하고 적절히 처리해야 함
	}

	// 제안 저장
	a.proposals[proposal.ID] = proposal
	a.nextProposalID++

	// 초기 보증금이 최소 보증금 이상이면 바로 투표 기간으로 전환
	if initialDeposit.Cmp(a.minDeposit) >= 0 {
		a.activateVotingPeriod(proposal)
	}

	a.logger.Info("Proposal submitted", "id", proposal.ID, "type", proposalType, "proposer", proposer.Hex())
	return proposal.ID, nil
}

// Deposit은 제안에 보증금을 추가합니다.
//
// 매개변수:
//   - proposalID: 제안 ID
//   - depositor: 보증금 예치자 주소
//   - amount: 보증금 금액
//
// 반환값:
//   - error: 보증금 추가 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 지정된 제안에 보증금을 추가합니다.
// 제안이 존재하지 않거나 이미 보증금 기간이 종료된 경우 오류를 반환합니다.
// 보증금이 추가되면 제안의 총 보증금이 업데이트되고, 최소 보증금에 도달하면
// 제안 상태가 투표 기간으로 전환됩니다.
// 보증금은 제안이 통과되면 반환되고, 거부되면 커뮤니티 풀로 이동합니다.
func (a *GovAdapter) Deposit(proposalID uint64, depositor common.Address, amount *big.Int) error {
	a.logger.Info("Adding deposit", "proposalID", proposalID, "depositor", depositor.Hex())

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
	if proposal.TotalDeposit.Cmp(a.minDeposit) >= 0 && proposal.Status == GovProposalStatusDepositPeriod {
		a.activateVotingPeriod(proposal)
	}

	a.logger.Info("Deposit added to proposal", "id", proposalID, "depositor", depositor.Hex(), "amount", amount)
	return nil
}

// Vote는 제안에 투표합니다.
//
// 매개변수:
//   - proposalID: 제안 ID
//   - voter: 투표자 주소
//   - optionStr: 투표 옵션 문자열 (Yes, No, Abstain, NoWithVeto)
//
// 반환값:
//   - error: 투표 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 지정된 제안에 투표를 추가합니다.
// 제안이 존재하지 않거나 투표 기간이 아닌 경우 오류를 반환합니다.
// 투표자가 이미 투표한 경우 기존 투표를 덮어씁니다.
// 투표 옵션은 Yes(찬성), No(반대), Abstain(기권), NoWithVeto(거부권 행사) 중 하나여야 합니다.
// 투표는 투표자의 스테이킹 양에 비례하여 가중치가 부여됩니다.
func (a *GovAdapter) Vote(proposalID uint64, voter common.Address, optionStr string) error {
	a.logger.Info("Voting on proposal", "proposalID", proposalID, "voter", voter.Hex())

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

// ExecuteProposal은 통과된 제안을 실행합니다.
//
// 매개변수:
//   - proposalID: 제안 ID
//   - state: 상태 데이터베이스
//
// 반환값:
//   - error: 제안 실행 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 통과된 제안을 실행합니다.
// 제안이 존재하지 않거나 통과 상태가 아닌 경우 오류를 반환합니다.
// 제안 유형에 따라 다른 실행 로직이 적용됩니다:
//   - 매개변수 변경: 시스템 매개변수를 업데이트합니다.
//   - 소프트웨어 업그레이드: 지정된 블록 높이에서 업그레이드를 예약합니다.
//   - 커뮤니티 풀 지출: 커뮤니티 풀에서 지정된 수령인에게 자금을 전송합니다.
//   - 텍스트 제안: 실행 로직이 없으며 단순히 상태만 업데이트합니다.
// 제안이 성공적으로 실행되면 상태가 'Executed'로 변경됩니다.
func (a *GovAdapter) ExecuteProposal(proposalID uint64, state *state.StateDB) error {
	a.logger.Info("Executing proposal", "proposalID", proposalID)

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

// ProcessProposals는 현재 블록 높이에서 모든 활성 제안을 처리합니다.
//
// 매개변수:
//   - blockHeight: 현재 블록 높이
//   - state: 상태 데이터베이스
//
// 반환값:
//   - error: 처리 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 각 블록 처리 시 호출되어 모든 활성 제안의 상태를 업데이트합니다.
// 다음과 같은 작업을 수행합니다:
//   - 보증금 기간이 종료된 제안을 처리합니다. 최소 보증금에 도달하지 못한 제안은 거부됩니다.
//   - 투표 기간이 종료된 제안을 처리합니다. 투표 결과에 따라 제안이 통과되거나 거부됩니다.
//   - 통과된 제안을 실행합니다.
// 이 함수는 블록체인의 거버넌스 상태를 최신 상태로 유지하는 데 중요합니다.
func (a *GovAdapter) ProcessProposals(blockHeight uint64, state *state.StateDB) error {
	a.logger.Debug("Processing proposals", "blockHeight", blockHeight)

	now := time.Now()

	// 모든 제안 처리
	for _, proposal := range a.proposals {
		// 보증금 기간 종료 확인
		if proposal.Status == GovProposalStatusDepositPeriod && now.After(proposal.DepositEndTime) {
			// 최소 보증금 미달 시 제안 거부
			if proposal.TotalDeposit.Cmp(a.minDeposit) < 0 {
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

// activateVotingPeriod는 제안을 투표 기간으로 전환합니다.
func (a *GovAdapter) activateVotingPeriod(proposal *GovProposal) {
	now := time.Now()
	proposal.Status = GovProposalStatusVotingPeriod
	proposal.VotingStartTime = now
	proposal.VotingEndTime = now.Add(a.votingPeriod)
	a.logger.Info("Proposal entered voting period", "id", proposal.ID, "end", proposal.VotingEndTime)
}

// tallyVotes는 제안의 투표를 집계하고 통과 여부를 결정합니다.
//
// 매개변수:
//   - proposal: 집계할 제안
//
// 반환값:
//   - bool: 제안 통과 여부 (true: 통과, false: 거부)
//
// 이 함수는 제안의 모든 투표를 집계하고 통과 여부를 결정합니다.
// 투표는 다음과 같은 규칙에 따라 집계됩니다:
//   - 총 투표율이 정족수(quorum)에 도달해야 합니다.
//   - 거부권(veto) 투표가 거부권 임계값을 초과하면 제안은 거부됩니다.
//   - 기권(abstain) 투표를 제외한 찬성(yes) 투표가 임계값을 초과해야 합니다.
// 투표 결과에 따라 제안 상태가 'Passed' 또는 'Rejected'로 업데이트됩니다.
func (a *GovAdapter) tallyVotes(proposal *GovProposal) bool {
	// 투표 수 집계
	totalVotes := big.NewInt(0)
	yesVotes := big.NewInt(0)
	noVotes := big.NewInt(0)
	noWithVetoVotes := big.NewInt(0)
	abstainVotes := big.NewInt(0)

	// 각 투표자의 스테이킹 양에 따라 투표 가중치 계산
	for _, vote := range proposal.Votes {
		// 투표자의 스테이킹 양 가져오기 (실제로는 스테이킹 어댑터에서 가져와야 함)
		// 여기서는 간단히 모든 투표자가 동일한 가중치를 가진다고 가정
		weight := big.NewInt(1)
		totalVotes.Add(totalVotes, weight)

		switch vote.Option {
		case GovOptionYes:
			yesVotes.Add(yesVotes, weight)
		case GovOptionNo:
			noVotes.Add(noVotes, weight)
		case GovOptionNoWithVeto:
			noWithVetoVotes.Add(noWithVetoVotes, weight)
		case GovOptionAbstain:
			abstainVotes.Add(abstainVotes, weight)
		}
	}

	// 총 스테이킹 양 가져오기 (실제로는 스테이킹 어댑터에서 가져와야 함)
	// 여기서는 간단히 총 투표 수의 3배라고 가정
	totalStaked := new(big.Int).Mul(totalVotes, big.NewInt(3))

	// 쿼럼 확인
	if totalStaked.Sign() > 0 {
		quorumRatio := new(big.Float).Quo(
			new(big.Float).SetInt(totalVotes),
			new(big.Float).SetInt(totalStaked),
		)
		quorumThreshold := big.NewFloat(a.quorumFloat)

		if quorumRatio.Cmp(quorumThreshold) < 0 {
			a.logger.Info("Proposal rejected due to insufficient quorum", "id", proposal.ID, "quorum", quorumRatio, "threshold", quorumThreshold)
			return false
		}
	}

	// 거부권 확인
	if totalVotes.Sign() > 0 {
		vetoRatio := new(big.Float).Quo(
			new(big.Float).SetInt(noWithVetoVotes),
			new(big.Float).SetInt(totalVotes),
		)
		vetoThreshold := big.NewFloat(a.vetoThresholdFloat)

		if vetoRatio.Cmp(vetoThreshold) >= 0 {
			a.logger.Info("Proposal rejected due to veto", "id", proposal.ID, "veto_ratio", vetoRatio, "threshold", vetoThreshold)
			return false
		}
	}

	// 통과 임계값 확인
	if new(big.Int).Add(yesVotes, abstainVotes).Sign() > 0 {
		// 먼저 noVotes와 noWithVetoVotes를 더함
		noTotal := new(big.Int).Add(noVotes, noWithVetoVotes)
		// 그 다음 yesVotes와 noTotal을 더함
		voteTotal := new(big.Int).Add(yesVotes, noTotal)

		yesRatio := new(big.Float).Quo(
			new(big.Float).SetInt(yesVotes),
			new(big.Float).SetInt(voteTotal),
		)
		threshold := big.NewFloat(a.thresholdFloat)

		if yesRatio.Cmp(threshold) >= 0 {
			a.logger.Info("Proposal passed", "id", proposal.ID, "yes_ratio", yesRatio, "threshold", threshold)
			return true
		}
	}

	a.logger.Info("Proposal rejected", "id", proposal.ID)
	return false
}

// executeParameterChangeProposal은 매개변수 변경 제안을 실행합니다.
func (a *GovAdapter) executeParameterChangeProposal(proposal *GovProposal) error {
	// 매개변수 변경 로직 구현
	// 실제 구현에서는 각 매개변수에 대한 유효성 검사 및 적용 로직이 필요
	for key, value := range proposal.Params {
		a.logger.Info("Parameter changed", "key", key, "value", value)
	}
	return nil
}

// executeSoftwareUpgradeProposal은 소프트웨어 업그레이드 제안을 실행합니다.
func (a *GovAdapter) executeSoftwareUpgradeProposal(proposal *GovProposal, blockHeight uint64) error {
	if proposal.UpgradeInfo == nil {
		return fmt.Errorf("upgrade info is nil")
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
func (a *GovAdapter) executeCommunityPoolSpendProposal(state *state.StateDB, proposal *GovProposal) error {
	if proposal.CommunityPoolSpendInfo == nil {
		return fmt.Errorf("community pool spend info is nil")
	}

	// 커뮤니티 풀에서 토큰 전송
	// 실제 구현에서는 커뮤니티 풀 계정에서 수령인 계정으로 토큰 전송
	communityPoolAddress := common.HexToAddress("0x0000000000000000000000000000000000000100") // 예시 주소

	// 커뮤니티 풀 잔액 확인
	balance := state.GetBalance(communityPoolAddress)
	if balance.ToBig().Cmp(proposal.CommunityPoolSpendInfo.Amount) < 0 {
		return fmt.Errorf("insufficient community pool balance: %s < %s", balance.String(), proposal.CommunityPoolSpendInfo.Amount.String())
	}

	// 토큰 전송
	amountUint256, _ := uint256.FromBig(proposal.CommunityPoolSpendInfo.Amount)
	state.SubBalance(communityPoolAddress, amountUint256, tracing.BalanceChangeUnspecified)
	state.AddBalance(proposal.CommunityPoolSpendInfo.Recipient, amountUint256, tracing.BalanceChangeUnspecified)

	a.logger.Info("Community pool spend executed", "recipient", proposal.CommunityPoolSpendInfo.Recipient.Hex(), "amount", proposal.CommunityPoolSpendInfo.Amount.String())
	return nil
}

// GetProposal은 제안 정보를 반환합니다.
func (a *GovAdapter) GetProposal(proposalID uint64) (*GovProposal, error) {
	proposal, exists := a.proposals[proposalID]
	if !exists {
		return nil, fmt.Errorf("proposal %d not found", proposalID)
	}
	return proposal, nil
}

// GetProposals는 모든 제안 정보를 반환합니다.
func (a *GovAdapter) GetProposals() []*GovProposal {
	proposals := make([]*GovProposal, 0, len(a.proposals))
	for _, proposal := range a.proposals {
		proposals = append(proposals, proposal)
	}
	return proposals
}

// GetProposalsByStatus는 특정 상태의 제안 정보를 반환합니다.
func (a *GovAdapter) GetProposalsByStatus(status GovProposalStatus) []*GovProposal {
	proposals := make([]*GovProposal, 0)
	for _, proposal := range a.proposals {
		if proposal.Status == status {
			proposals = append(proposals, proposal)
		}
	}
	return proposals
}

// GetStateDB는 주어진 루트 해시에 대한 상태 DB를 반환합니다.
func (ga *GovAdapter) GetStateDB(root common.Hash) (*state.StateDB, error) {
	// 블록체인 상태에 접근하기 위해 StateDB 인스턴스를 생성합니다.
	// 참고: 실제 구현에서는 state.New 함수의 정확한 시그니처에 맞게 수정해야 합니다.
	// 여기서는 임시로 nil을 반환합니다.
	return nil, fmt.Errorf("GetStateDB not implemented")
}
