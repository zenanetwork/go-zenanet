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
type GovVote struct {
	ProposalID uint64         // 제안 ID
	Voter      common.Address // 투표자 주소
	Option     GovVoteOption  // 투표 옵션
	Timestamp  time.Time      // 투표 시간
}

// GovVoteOption은 투표 옵션을 나타내는 열거형입니다.
type GovVoteOption uint8

const (
	GovOptionYes GovVoteOption = iota
	GovOptionNo
	GovOptionNoWithVeto
	GovOptionAbstain
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
	return &GovAdapter{
		eirene:             eirene,
		logger:             log.New("module", "eirene/gov"),
		db:                 db,
		minDeposit:         new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18)), // 100 토큰
		maxDepositPeriod:   14 * 24 * time.Hour,                                 // 14일
		votingPeriod:       14 * 24 * time.Hour,                                 // 14일
		quorumFloat:        0.334,                                               // 33.4%
		thresholdFloat:     0.5,                                                 // 50%
		vetoThresholdFloat: 0.334,                                               // 33.4%
		proposals:          make(map[uint64]*GovProposal),
		nextProposalID:     1,
	}
}

// SubmitProposal은 새로운 제안을 제출합니다.
func (a *GovAdapter) SubmitProposal(state *state.StateDB, proposer common.Address, title, description string, proposalType GovProposalType, initialDeposit *big.Int, params map[string]string, upgradeInfo *GovUpgradeInfo, communityPoolSpendInfo *GovCommunityPoolSpendInfo) (uint64, error) {
	// 초기 보증금 확인
	if initialDeposit.Cmp(big.NewInt(0)) <= 0 {
		return 0, fmt.Errorf("initial deposit must be positive")
	}

	// 계정 잔액 확인
	balance := state.GetBalance(proposer)
	if balance.ToBig().Cmp(initialDeposit) < 0 {
		return 0, fmt.Errorf("insufficient balance for proposal deposit: %s < %s", balance.String(), initialDeposit.String())
	}

	// 제안 생성
	now := time.Now()
	proposal := &GovProposal{
		ID:                     a.nextProposalID,
		Title:                  title,
		Description:            description,
		ProposalType:           proposalType,
		ProposerAddress:        proposer,
		Status:                 GovProposalStatusDepositPeriod,
		SubmitTime:             now,
		DepositEndTime:         now.Add(a.maxDepositPeriod),
		TotalDeposit:           initialDeposit,
		Params:                 params,
		UpgradeInfo:            upgradeInfo,
		CommunityPoolSpendInfo: communityPoolSpendInfo,
	}

	// 초기 보증금 예치
	deposit := &GovDeposit{
		ProposalID: proposal.ID,
		Depositor:  proposer,
		Amount:     initialDeposit,
		Timestamp:  now,
	}
	proposal.Deposits = append(proposal.Deposits, deposit)

	// 제안 저장
	a.proposals[proposal.ID] = proposal
	a.nextProposalID++

	// 보증금이 최소 보증금 이상이면 투표 기간으로 전환
	if proposal.TotalDeposit.Cmp(a.minDeposit) >= 0 {
		a.activateVotingPeriod(proposal)
	}

	// 토큰 예치 (잔액에서 차감)
	initialDepositUint256, _ := uint256.FromBig(initialDeposit)
	state.SubBalance(proposer, initialDepositUint256, tracing.BalanceChangeUnspecified)

	a.logger.Info("New proposal submitted", "id", proposal.ID, "title", proposal.Title, "proposer", proposer.Hex(), "deposit", initialDeposit.String())

	return proposal.ID, nil
}

// Deposit은 제안에 보증금을 예치합니다.
func (a *GovAdapter) Deposit(state *state.StateDB, depositor common.Address, proposalID uint64, amount *big.Int) error {
	// 제안 확인
	proposal, exists := a.proposals[proposalID]
	if !exists {
		return fmt.Errorf("proposal %d not found", proposalID)
	}

	// 제안 상태 확인
	if proposal.Status != GovProposalStatusDepositPeriod {
		return fmt.Errorf("proposal %d is not in deposit period", proposalID)
	}

	// 계정 잔액 확인
	balance := state.GetBalance(depositor)
	if balance.ToBig().Cmp(amount) < 0 {
		return fmt.Errorf("insufficient balance for deposit: %s < %s", balance.String(), amount.String())
	}

	// 보증금 예치
	deposit := &GovDeposit{
		ProposalID: proposalID,
		Depositor:  depositor,
		Amount:     amount,
		Timestamp:  time.Now(),
	}
	proposal.Deposits = append(proposal.Deposits, deposit)
	proposal.TotalDeposit = new(big.Int).Add(proposal.TotalDeposit, amount)

	// 보증금이 최소 보증금 이상이면 투표 기간으로 전환
	if proposal.TotalDeposit.Cmp(a.minDeposit) >= 0 && proposal.Status == GovProposalStatusDepositPeriod {
		a.activateVotingPeriod(proposal)
	}

	// 토큰 예치 (잔액에서 차감)
	amountUint256, _ := uint256.FromBig(amount)
	state.SubBalance(depositor, amountUint256, tracing.BalanceChangeUnspecified)

	a.logger.Info("Deposit added to proposal", "id", proposalID, "depositor", depositor.Hex(), "amount", amount.String(), "total", proposal.TotalDeposit.String())

	return nil
}

// Vote는 제안에 투표합니다.
func (a *GovAdapter) Vote(voter common.Address, proposalID uint64, option GovVoteOption) error {
	// 제안 확인
	proposal, exists := a.proposals[proposalID]
	if !exists {
		return fmt.Errorf("proposal %d not found", proposalID)
	}

	// 제안 상태 확인
	if proposal.Status != GovProposalStatusVotingPeriod {
		return fmt.Errorf("proposal %d is not in voting period", proposalID)
	}

	// 이미 투표했는지 확인
	for _, v := range proposal.Votes {
		if v.Voter == voter {
			return fmt.Errorf("voter %s has already voted on proposal %d", voter.Hex(), proposalID)
		}
	}

	// 투표 생성
	vote := &GovVote{
		ProposalID: proposalID,
		Voter:      voter,
		Option:     option,
		Timestamp:  time.Now(),
	}
	proposal.Votes = append(proposal.Votes, vote)

	a.logger.Info("Vote cast", "id", proposalID, "voter", voter.Hex(), "option", option)

	return nil
}

// ProcessProposals는 모든 활성 제안을 처리합니다.
func (a *GovAdapter) ProcessProposals(state *state.StateDB, blockTime time.Time, blockHeight uint64) error {
	for _, proposal := range a.proposals {
		// 보증금 기간 종료 확인
		if proposal.Status == GovProposalStatusDepositPeriod && blockTime.After(proposal.DepositEndTime) {
			if proposal.TotalDeposit.Cmp(a.minDeposit) < 0 {
				// 최소 보증금을 모으지 못한 경우 제안 실패
				proposal.Status = GovProposalStatusFailed
				a.refundDeposits(state, proposal)
				a.logger.Info("Proposal failed due to insufficient deposit", "id", proposal.ID)
			} else {
				// 투표 기간으로 전환
				a.activateVotingPeriod(proposal)
			}
		}

		// 투표 기간 종료 확인
		if proposal.Status == GovProposalStatusVotingPeriod && blockTime.After(proposal.VotingEndTime) {
			// 투표 결과 계산
			result := a.tallyVotes(proposal)
			if result {
				proposal.Status = GovProposalStatusPassed
				proposal.ExecutionTime = blockTime.Add(24 * time.Hour) // 24시간 후 실행
				a.logger.Info("Proposal passed", "id", proposal.ID)
			} else {
				proposal.Status = GovProposalStatusRejected
				a.refundDeposits(state, proposal)
				a.logger.Info("Proposal rejected", "id", proposal.ID)
			}
		}

		// 통과된 제안 실행
		if proposal.Status == GovProposalStatusPassed && blockTime.After(proposal.ExecutionTime) {
			if err := a.executeProposal(state, proposal, blockHeight); err != nil {
				a.logger.Error("Failed to execute proposal", "id", proposal.ID, "error", err)
				proposal.Status = GovProposalStatusFailed
				a.refundDeposits(state, proposal)
			} else {
				proposal.Status = GovProposalStatusExecuted
				a.logger.Info("Proposal executed", "id", proposal.ID)
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

// tallyVotes는 제안에 대한 투표를 집계하고 통과 여부를 반환합니다.
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

// executeProposal은 통과된 제안을 실행합니다.
func (a *GovAdapter) executeProposal(state *state.StateDB, proposal *GovProposal, blockHeight uint64) error {
	switch proposal.ProposalType {
	case GovProposalTypeParameterChange:
		return a.executeParameterChangeProposal(proposal)
	case GovProposalTypeSoftwareUpgrade:
		return a.executeSoftwareUpgradeProposal(proposal, blockHeight)
	case GovProposalTypeCommunityPoolSpend:
		return a.executeCommunityPoolSpendProposal(state, proposal)
	case GovProposalTypeText:
		// 텍스트 제안은 실행할 내용이 없음
		return nil
	default:
		return fmt.Errorf("unknown proposal type: %d", proposal.ProposalType)
	}
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

// refundDeposits는 제안의 보증금을 환불합니다.
func (a *GovAdapter) refundDeposits(state *state.StateDB, proposal *GovProposal) {
	for _, deposit := range proposal.Deposits {
		amountUint256, _ := uint256.FromBig(deposit.Amount)
		state.AddBalance(deposit.Depositor, amountUint256, tracing.BalanceChangeUnspecified)
		a.logger.Info("Deposit refunded", "id", proposal.ID, "depositor", deposit.Depositor.Hex(), "amount", deposit.Amount.String())
	}
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
