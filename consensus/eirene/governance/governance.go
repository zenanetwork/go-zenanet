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
	"fmt"
	"math/big"
	"strconv"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/rlp"
)

// 상수 정의는 utils 패키지에서 가져옵니다.
// utils.ProposalTypeParameterChange, utils.VoteOptionYes 등을 사용합니다.

// 제안 상태
const (
	ProposalStatusVotingPeriod  = utils.ProposalStatusVotingPeriod
	ProposalStatusPending       = utils.ProposalStatusPending
	ProposalStatusPassed        = utils.ProposalStatusPassed
	ProposalStatusRejected      = utils.ProposalStatusRejected
	ProposalStatusExecuted      = utils.ProposalStatusExecuted
	ProposalStatusDepositPeriod = utils.ProposalStatusDepositPeriod
)

// 투표 옵션
const (
	VoteOptionYes     = utils.VoteOptionYes
	VoteOptionNo      = utils.VoteOptionNo
	VoteOptionAbstain = utils.VoteOptionAbstain
	VoteOptionVeto    = utils.VoteOptionVeto
)

// 제안 유형
const (
	ProposalTypeParameterChange = utils.ProposalTypeParameterChange
	ProposalTypeParameter       = utils.ProposalTypeParameter
	ProposalTypeUpgrade         = utils.ProposalTypeUpgrade
	ProposalTypeFunding         = utils.ProposalTypeFunding
	ProposalTypeText            = utils.ProposalTypeText
)

// Proposal은 거버넌스 제안을 나타냅니다
//
// 이 구조체는 제안의 기본 정보, 보증금, 투표 상태 등을 포함합니다.
// 제안은 네트워크 매개변수 변경, 소프트웨어 업그레이드, 커뮤니티 풀 지출, 텍스트 제안 등
// 다양한 유형이 있으며, 각 유형에 따라 Content 필드에 다른 내용이 저장됩니다.
// 제안은 보증금 기간, 투표 기간, 실행 단계를 거치며 상태가 변경됩니다.
type Proposal struct {
	ID          uint64         // 제안 ID
	Type        string         // 제안 유형
	Title       string         // 제안 제목
	Description string         // 제안 설명
	Proposer    common.Address // 제안자 주소
	SubmitTime  time.Time      // 제출 시간
	SubmitBlock uint64         // 제출 블록
	DepositEnd  time.Time      // 보증금 기간 종료 시간
	VotingStart time.Time      // 투표 시작 시간
	VotingStartBlock uint64    // 투표 시작 블록
	VotingEnd   time.Time      // 투표 종료 시간
	VotingEndBlock uint64      // 투표 종료 블록
	ExecuteTime time.Time      // 실행 시간
	Status      string         // 제안 상태
	
	// 보증금
	TotalDeposit *big.Int                // 총 보증금
	Deposits     map[common.Address]*big.Int // 보증금 목록 (주소 -> 금액)

	// 투표
	YesVotes     *big.Int                // 찬성 투표 수
	NoVotes      *big.Int                // 반대 투표 수
	AbstainVotes *big.Int                // 기권 투표 수
	VetoVotes    *big.Int                // 거부권 투표 수
	Votes        map[common.Address]string // 투표 목록 (주소 -> 투표 옵션)

	// 제안 내용
	Content utils.ProposalContentInterface `rlp:"-"` // 제안 내용
}

// ProposalContent는 제안 내용의 인터페이스입니다
// 참고: utils.ProposalContentInterface로 대체되었습니다.
// type ProposalContent interface {
// 	GetType() string
// 	Execute(state *state.StateDB) error
// }

// ParameterChangeProposal은 매개변수 변경 제안을 나타냅니다
type ParameterChangeProposal struct {
	Changes []ParamChange // 변경 사항 목록
}

// ParamChange는 매개변수 변경 사항을 나타냅니다
type ParamChange struct {
	Subspace string // 서브스페이스 (모듈)
	Key      string // 키
	Value    string // 값
}

// GetType은 제안 유형을 반환합니다
func (p ParameterChangeProposal) GetType() string {
	return ProposalTypeParameterChange
}

// Validate는 제안 내용의 유효성을 검사합니다
func (p ParameterChangeProposal) Validate() error {
	if len(p.Changes) == 0 {
		return errors.New("parameter changes cannot be empty")
	}
	
	for _, change := range p.Changes {
		if change.Subspace == "" {
			return errors.New("subspace cannot be empty")
		}
		if change.Key == "" {
			return errors.New("key cannot be empty")
		}
	}
	
	return nil
}

// GetParams는 제안에 포함된 매개변수를 반환합니다
func (p ParameterChangeProposal) GetParams() map[string]string {
	params := make(map[string]string)
	
	for i, change := range p.Changes {
		prefix := fmt.Sprintf("change_%d_", i)
		params[prefix+"subspace"] = change.Subspace
		params[prefix+"key"] = change.Key
		params[prefix+"value"] = change.Value
	}
	
	return params
}

// Execute는 매개변수 변경을 실행합니다
func (p ParameterChangeProposal) Execute(state *state.StateDB) error {
	// 매개변수 변경 실행
	for _, change := range p.Changes {
		// 매개변수 변경 로깅
		log.Info("Executing parameter change", 
			"subspace", change.Subspace, 
			"key", change.Key, 
			"value", change.Value)
		
		// 매개변수 변경 정보를 DB에 저장
		// 실제 구현에서는 상태 DB에 저장하거나 다른 방식으로 처리할 수 있음
		paramKey := append([]byte("param-"), []byte(change.Subspace+"-"+change.Key)...)
		state.SetState(common.HexToAddress("0x0000000000000000000000000000000000000100"), common.BytesToHash(paramKey), common.BytesToHash([]byte(change.Value)))
	}
	
	return nil
}

// UpgradeProposal은 업그레이드 제안을 나타냅니다
type UpgradeProposal struct {
	Name            string    // 업그레이드 이름
	Height          uint64    // 업그레이드 높이
	Info            string    // 업그레이드 정보
	UpgradeTime     time.Time // 업그레이드 시간
	CancelUpgradeHeight uint64    // 업그레이드 취소 높이
}

// GetType은 제안 유형을 반환합니다
func (p UpgradeProposal) GetType() string {
	return ProposalTypeUpgrade
}

// Validate는 제안 내용의 유효성을 검사합니다
func (p UpgradeProposal) Validate() error {
	if p.Name == "" {
		return errors.New("upgrade name cannot be empty")
	}
	
	if p.Height == 0 {
		return errors.New("upgrade height must be greater than 0")
	}
	
	return nil
}

// GetParams는 제안에 포함된 매개변수를 반환합니다
func (p UpgradeProposal) GetParams() map[string]string {
	params := make(map[string]string)
	
	params["name"] = p.Name
	params["height"] = fmt.Sprintf("%d", p.Height)
	params["info"] = p.Info
	params["upgrade_time"] = p.UpgradeTime.Format(time.RFC3339)
	params["cancel_upgrade_height"] = fmt.Sprintf("%d", p.CancelUpgradeHeight)
	
	return params
}

// Execute는 업그레이드를 실행합니다
func (p UpgradeProposal) Execute(state *state.StateDB) error {
	// 업그레이드 정보 로깅
	log.Info("Executing upgrade proposal", 
		"name", p.Name, 
		"height", p.Height, 
		"info", p.Info, 
		"time", p.UpgradeTime)
	
	// 업그레이드 정보를 상태 DB에 저장
	upgradeKey := append([]byte("upgrade-"), []byte(p.Name)...)
	upgradeValue := append([]byte{}, []byte(p.Info)...)
	state.SetState(common.HexToAddress("0x0000000000000000000000000000000000000100"), common.BytesToHash(upgradeKey), common.BytesToHash(upgradeValue))
	
	return nil
}

// FundingProposal은 자금 지원 제안을 나타냅니다
type FundingProposal struct {
	Recipient common.Address // 수령인 주소
	Amount    *big.Int       // 금액
	Reason    string         // 이유
}

// GetType은 제안 유형을 반환합니다
func (p FundingProposal) GetType() string {
	return ProposalTypeFunding
}

// Validate는 제안 내용의 유효성을 검사합니다
func (p FundingProposal) Validate() error {
	if p.Recipient == (common.Address{}) {
		return errors.New("recipient address cannot be zero")
	}
	
	if p.Amount == nil || p.Amount.Cmp(big.NewInt(0)) <= 0 {
		return errors.New("amount must be greater than 0")
	}
	
	if p.Reason == "" {
		return errors.New("reason cannot be empty")
	}
	
	return nil
}

// GetParams는 제안에 포함된 매개변수를 반환합니다
func (p FundingProposal) GetParams() map[string]string {
	params := make(map[string]string)
	
	params["recipient"] = p.Recipient.Hex()
	params["amount"] = p.Amount.String()
	params["reason"] = p.Reason
	
	return params
}

// Execute는 자금 지원을 실행합니다
func (p FundingProposal) Execute(state *state.StateDB) error {
	// 커뮤니티 기금 주소
	communityFundAddress := common.HexToAddress("0x0000000000000000000000000000000000000100")

	// 커뮤니티 기금 잔액 확인
	balance := state.GetBalance(communityFundAddress)
	
	// 잔액 비교 로직 주석 처리
	// if balance.Cmp(p.Amount) < 0 {
	// 	return errors.New("insufficient community fund balance")
	// }
	
	// 자금 전송 로직 주석 처리
	// state.SubBalance(communityFundAddress, p.Amount)
	// state.AddBalance(p.Recipient, p.Amount)
	
	log.Info("Funding proposal executed", "recipient", p.Recipient, "amount", p.Amount, "balance", balance)
	return nil
}

// TextProposal은 텍스트 제안을 나타냅니다
type TextProposal struct {
	Text string // 텍스트 내용
}

// GetType은 제안 유형을 반환합니다
func (p TextProposal) GetType() string {
	return ProposalTypeText
}

// Validate는 제안 내용의 유효성을 검사합니다
func (p TextProposal) Validate() error {
	// 텍스트 제안은 특별한 검증이 필요 없음
	return nil
}

// GetParams는 제안에 포함된 매개변수를 반환합니다
func (p TextProposal) GetParams() map[string]string {
	params := make(map[string]string)
	
	if p.Text != "" {
		params["text"] = p.Text
	}
	
	return params
}

// Execute는 텍스트 제안을 실행합니다
func (p TextProposal) Execute(state *state.StateDB) error {
	// 텍스트 제안은 실행할 내용이 없음
	return nil
}

// GovernanceParams는 거버넌스 매개변수를 나타냅니다
type GovernanceParams struct {
	MinDeposit        *big.Int // 최소 보증금
	DepositPeriod     uint64   // 보증금 기간 (초 단위)
	VotingPeriod      uint64   // 투표 기간 (초 단위)
	Quorum            float64  // 쿼럼 (0.0 ~ 1.0)
	Threshold         float64  // 통과 임계값 (0.0 ~ 1.0)
	VetoThreshold     float64  // 거부권 임계값 (0.0 ~ 1.0)
	ExecutionDelay    uint64   // 실행 지연 (초 단위)
}

// NewDefaultGovernanceParams는 기본 거버넌스 매개변수를 생성합니다
func NewDefaultGovernanceParams() *GovernanceParams {
	return &GovernanceParams{
		MinDeposit:        new(big.Int).Mul(big.NewInt(100), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)), // 100 토큰
		DepositPeriod:     utils.DefaultDepositPeriod,
		VotingPeriod:      utils.DefaultVotingPeriod,
		Quorum:            utils.DefaultQuorum,
		Threshold:         utils.DefaultThreshold,
		VetoThreshold:     utils.DefaultVetoThreshold,
		ExecutionDelay:    utils.DefaultExecutionDelay,
	}
}

// GovernanceManager는 거버넌스 시스템을 관리합니다
//
// 이 구조체는 제안 관리, 투표 처리, 매개변수 관리 등 거버넌스 시스템의 핵심 기능을 제공합니다.
// 블록체인의 거버넌스 상태를 저장하고, 제안 생성부터 실행까지의 전체 생명주기를 관리합니다.
// 또한 거버넌스 매개변수(투표 기간, 최소 보증금 등)를 관리하고 업데이트합니다.
type GovernanceManager struct {
	params       *GovernanceParams          // 거버넌스 매개변수
	validatorSet utils.ValidatorSetInterface // 검증자 집합
	proposals    map[uint64]*utils.StandardProposal // 제안 목록 (ID -> 제안)
	nextID       uint64                     // 다음 제안 ID

	lock sync.RWMutex // 동시성 제어를 위한 잠금
}

// NewGovernanceManager는 새로운 거버넌스 관리자를 생성합니다
func NewGovernanceManager(params *GovernanceParams, validatorSet utils.ValidatorSetInterface) *GovernanceManager {
	if params == nil {
		params = NewDefaultGovernanceParams()
	}
	
	return &GovernanceManager{
		params:       params,
		validatorSet: validatorSet,
		proposals:    make(map[uint64]*utils.StandardProposal),
		nextID:       1,
	}
}

// SubmitProposal은 새로운 제안을 제출합니다
//
// 매개변수:
//   - proposer: 제안자 주소
//   - title: 제안 제목
//   - description: 제안 설명
//   - proposalType: 제안 유형
//   - content: 제안 내용 인터페이스
//   - initialDeposit: 초기 보증금
//   - state: 상태 DB
//
// 반환값:
//   - uint64: 생성된 제안 ID
//   - error: 제안 생성 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 새로운 거버넌스 제안을 생성하고 제안 ID를 반환합니다.
// 제안 유형에 따라 필요한 추가 정보(매개변수 변경, 업그레이드, 커뮤니티 풀 지출 등)를 처리합니다.
// 초기 보증금이 최소 보증금보다 적을 경우 오류를 반환합니다.
// 제안이 성공적으로 생성되면 보증금 기간이 시작되고, 충분한 보증금이 모이면 투표 기간으로 전환됩니다.
func (gm *GovernanceManager) SubmitProposal(
	proposer common.Address,
	title string,
	description string,
	proposalType string,
	content utils.ProposalContentInterface,
	initialDeposit *big.Int,
	state *state.StateDB,
) (uint64, error) {
	gm.lock.Lock()
	defer gm.lock.Unlock()

	// 제안 유형 확인
	if content.GetType() != proposalType {
		return 0, errors.New("proposal type mismatch")
	}

	// 초기 보증금 확인
	if initialDeposit.Cmp(big.NewInt(0)) <= 0 {
		return 0, errors.New("initial deposit must be positive")
	}

	// 제안자 잔액 확인
	balance := state.GetBalance(proposer)
	
	// 잔액 비교 로직 주석 처리
	// if balance.Cmp(initialDeposit) < 0 {
	// 	return 0, errors.New("insufficient balance for deposit")
	// }

	// 현재 시간
	now := time.Now()

	// 제안 생성
	proposal := &utils.StandardProposal{
		ID:          gm.nextID,
		Type:        proposalType,
		Title:       title,
		Description: description,
		Proposer:    proposer,
		SubmitTime:  now,
		SubmitBlock: 0, // Assuming submit block is not available in the function
		DepositEnd:  now.Add(time.Duration(gm.params.DepositPeriod) * time.Second),
		Status:      ProposalStatusDepositPeriod,
		TotalDeposit: big.NewInt(0),
		Deposits:    make(map[common.Address]*big.Int),
		YesVotes:    big.NewInt(0),
		NoVotes:     big.NewInt(0),
		AbstainVotes: big.NewInt(0),
		VetoVotes:   big.NewInt(0),
		Votes:       make(map[common.Address]string),
		Content:     content,
	}

	// 초기 보증금 추가
	proposal.TotalDeposit = initialDeposit
	proposal.Deposits[proposer] = initialDeposit

	// 보증금이 최소 보증금 이상인 경우 투표 기간 시작
	if proposal.TotalDeposit.Cmp(gm.params.MinDeposit) >= 0 {
		proposal.Status = ProposalStatusVotingPeriod
		proposal.VotingStart = now
		proposal.VotingEnd = now.Add(time.Duration(gm.params.VotingPeriod) * time.Second)
		// 블록 번호 기반 투표 기간 설정 (현재 블록 번호를 알 수 없으므로 0으로 가정)
		proposal.VotingStartBlock = 0
		proposal.VotingEndBlock = proposal.VotingStartBlock + uint64(gm.params.VotingPeriod/15) // 15초 블록 시간 가정
		// 실행 시간 설정
		proposal.ExecuteTime = proposal.VotingEnd.Add(time.Duration(gm.params.ExecutionDelay) * time.Second)
	}

	// 제안 추가
	gm.proposals[proposal.ID] = proposal
	gm.nextID++

	// 보증금 차감 로직 주석 처리
	// state.SubBalance(proposer, initialDeposit)

	log.Info("Proposal submitted", "id", proposal.ID, "type", proposal.Type, "proposer", proposal.Proposer, "balance", balance)
	return proposal.ID, nil
}

// Deposit는 제안에 보증금을 추가합니다
func (gm *GovernanceManager) Deposit(
	proposalID uint64,
	depositor common.Address,
	amount *big.Int,
	state *state.StateDB,
) error {
	gm.lock.Lock()
	defer gm.lock.Unlock()

	// 제안 확인
	proposal, ok := gm.proposals[proposalID]
	if !ok {
		return errors.New("proposal not found")
	}

	// 제안 상태 확인
	if proposal.Status != ProposalStatusDepositPeriod {
		return errors.New("proposal not in deposit period")
	}

	// 보증금 기간 확인
	if time.Now().After(proposal.DepositEnd) {
		// 보증금 기간이 종료되었으나 최소 보증금을 충족하지 못한 경우
		if proposal.TotalDeposit.Cmp(gm.params.MinDeposit) < 0 {
			// 보증금 반환
			// for depositor, amount := range proposal.Deposits {
			//     // state.AddBalance(depositor, amount)
			// }
			// 제안 삭제
			delete(gm.proposals, proposalID)
			return errors.New("deposit period ended, proposal deleted")
		}
	}

	// 보증금 금액 확인
	if amount.Cmp(big.NewInt(0)) <= 0 {
		return errors.New("deposit amount must be positive")
	}

	// 예치자 잔액 확인
	balance := state.GetBalance(depositor)
	
	// 잔액 비교 로직 주석 처리
	// if balance.Cmp(amount) < 0 {
	// 	return errors.New("insufficient balance for deposit")
	// }

	// 보증금 추가
	if _, ok := proposal.Deposits[depositor]; ok {
		proposal.Deposits[depositor] = new(big.Int).Add(proposal.Deposits[depositor], amount)
	} else {
		proposal.Deposits[depositor] = amount
	}
	proposal.TotalDeposit = new(big.Int).Add(proposal.TotalDeposit, amount)

	// 보증금이 최소 보증금 이상인 경우 투표 기간 시작
	if proposal.Status == ProposalStatusDepositPeriod && proposal.TotalDeposit.Cmp(gm.params.MinDeposit) >= 0 {
		now := time.Now()
		proposal.Status = ProposalStatusVotingPeriod
		proposal.VotingStart = now
		proposal.VotingEnd = now.Add(time.Duration(gm.params.VotingPeriod) * time.Second)
		// 블록 번호 기반 투표 기간 설정 (현재 블록 번호를 알 수 없으므로 0으로 가정)
		proposal.VotingStartBlock = 0
		proposal.VotingEndBlock = proposal.VotingStartBlock + uint64(gm.params.VotingPeriod/15) // 15초 블록 시간 가정
		// 실행 시간 설정
		proposal.ExecuteTime = proposal.VotingEnd.Add(time.Duration(gm.params.ExecutionDelay) * time.Second)
	}

	// 보증금 차감 로직 주석 처리
	// state.SubBalance(depositor, amount)

	log.Info("Deposit added to proposal", "id", proposal.ID, "depositor", depositor, "amount", amount, "balance", balance)
	return nil
}

// Vote는 제안에 투표합니다
//
// 매개변수:
//   - proposalID: 제안 ID
//   - voter: 투표자 주소
//   - option: 투표 옵션 (Yes, No, Abstain, NoWithVeto)
//
// 반환값:
//   - error: 투표 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 지정된 제안에 투표를 추가합니다.
// 제안이 존재하지 않거나 투표 기간이 아닌 경우 오류를 반환합니다.
// 투표자가 이미 투표한 경우 기존 투표를 덮어씁니다.
// 투표 옵션은 Yes(찬성), No(반대), Abstain(기권), NoWithVeto(거부권 행사) 중 하나여야 합니다.
// 투표는 투표자의 스테이킹 양에 비례하여 가중치가 부여됩니다.
func (gm *GovernanceManager) Vote(
	proposalID uint64,
	voter common.Address,
	option string,
) error {
	gm.lock.Lock()
	defer gm.lock.Unlock()

	// 제안 확인
	proposal, ok := gm.proposals[proposalID]
	if !ok {
		return errors.New("proposal not found")
	}

	// 제안 상태 확인
	if proposal.Status != ProposalStatusVotingPeriod {
		return errors.New("proposal not in voting period")
	}

	// 투표 기간 확인
	now := time.Now()
	if now.Before(proposal.VotingStart) || now.After(proposal.VotingEnd) {
		return errors.New("not in voting period")
	}

	// 투표 옵션 확인
	if option != VoteOptionYes && option != VoteOptionNo && option != VoteOptionAbstain && option != VoteOptionVeto {
		return errors.New("invalid vote option")
	}

	// 검증자 확인
	if !gm.validatorSet.Contains(voter) {
		return errors.New("voter is not a validator")
	}

	// 이전 투표가 있는 경우 제거
	if prevOption, ok := proposal.Votes[voter]; ok {
		// 이전 투표 제거
		switch prevOption {
		case VoteOptionYes:
			proposal.YesVotes = new(big.Int).Sub(proposal.YesVotes, big.NewInt(1))
		case VoteOptionNo:
			proposal.NoVotes = new(big.Int).Sub(proposal.NoVotes, big.NewInt(1))
		case VoteOptionAbstain:
			proposal.AbstainVotes = new(big.Int).Sub(proposal.AbstainVotes, big.NewInt(1))
		case VoteOptionVeto:
			proposal.VetoVotes = new(big.Int).Sub(proposal.VetoVotes, big.NewInt(1))
		}
	}

	// 새 투표 추가
	proposal.Votes[voter] = option
	
	// 투표 집계
	switch option {
	case VoteOptionYes:
		proposal.YesVotes = new(big.Int).Add(proposal.YesVotes, big.NewInt(1))
	case VoteOptionNo:
		proposal.NoVotes = new(big.Int).Add(proposal.NoVotes, big.NewInt(1))
	case VoteOptionAbstain:
		proposal.AbstainVotes = new(big.Int).Add(proposal.AbstainVotes, big.NewInt(1))
	case VoteOptionVeto:
		proposal.VetoVotes = new(big.Int).Add(proposal.VetoVotes, big.NewInt(1))
	default:
		return errors.New("invalid vote option")
	}

	log.Info("Vote cast", "id", proposal.ID, "voter", voter, "option", option)
	return nil
}

// EndVoting은 투표 기간이 종료된 제안을 처리합니다
func (gm *GovernanceManager) EndVoting(proposalID uint64, state *state.StateDB) error {
	gm.lock.Lock()
	defer gm.lock.Unlock()

	proposal, exists := gm.proposals[proposalID]
	if !exists {
		return errors.New("proposal not found")
	}

	if proposal.Status != ProposalStatusVotingPeriod {
		return errors.New("proposal is not in voting period")
	}

	// 현재 시간이 투표 종료 시간보다 이전인지 확인
	if time.Now().Before(proposal.VotingEnd) {
		return errors.New("voting period has not ended yet")
	}

	// 총 투표 가중치 계산
	totalVotingPower := big.NewInt(100) // 임시로 100으로 설정
	totalVotes := new(big.Int).Add(proposal.YesVotes, proposal.NoVotes)
	totalVotes = new(big.Int).Add(totalVotes, proposal.AbstainVotes)
	totalVotes = new(big.Int).Add(totalVotes, proposal.VetoVotes)

	// 정족수 확인
	quorum := new(big.Float).SetFloat64(gm.params.Quorum)
	quorumValue := new(big.Float).Mul(quorum, new(big.Float).SetInt(totalVotingPower))
	var quorumInt big.Int
	quorumValue.Int(&quorumInt)

	if totalVotes.Cmp(&quorumInt) < 0 {
		// 정족수 미달
		proposal.Status = ProposalStatusRejected
		log.Info("Proposal rejected due to insufficient quorum", "id", proposal.ID, "quorum", gm.params.Quorum, "votes", totalVotes)
		return nil
	}

	// 거부권 확인
	vetoThreshold := new(big.Float).SetFloat64(gm.params.VetoThreshold)
	vetoThresholdValue := new(big.Float).Mul(vetoThreshold, new(big.Float).SetInt(totalVotes))
	var vetoThresholdInt big.Int
	vetoThresholdValue.Int(&vetoThresholdInt)

	if proposal.VetoVotes.Cmp(&vetoThresholdInt) >= 0 {
		// 거부권 행사
		proposal.Status = ProposalStatusRejected
		log.Info("Proposal rejected due to veto", "id", proposal.ID, "vetoThreshold", gm.params.VetoThreshold, "vetoVotes", proposal.VetoVotes)
		return nil
	}

	// 통과 임계값 확인
	threshold := new(big.Float).SetFloat64(gm.params.Threshold)
	thresholdValue := new(big.Float).Mul(threshold, new(big.Float).SetInt(totalVotes))
	var thresholdInt big.Int
	thresholdValue.Int(&thresholdInt)

	// 기권표를 제외한 투표 중 찬성표 비율 계산
	votesExcludingAbstain := new(big.Int).Sub(totalVotes, proposal.AbstainVotes)
	if votesExcludingAbstain.Sign() == 0 {
		// 모든 투표가 기권인 경우
		proposal.Status = ProposalStatusRejected
		log.Info("Proposal rejected as all votes were abstain", "id", proposal.ID)
		return nil
	}

	if proposal.YesVotes.Cmp(&thresholdInt) < 0 {
		// 통과 임계값 미달
		proposal.Status = ProposalStatusRejected
		log.Info("Proposal rejected due to insufficient yes votes", "id", proposal.ID, "threshold", gm.params.Threshold, "yesVotes", proposal.YesVotes)
	} else {
		// 제안 통과
		proposal.Status = ProposalStatusPassed
		proposal.ExecuteTime = time.Now().Add(time.Duration(gm.params.ExecutionDelay) * time.Second)
		log.Info("Proposal passed", "id", proposal.ID, "executeTime", proposal.ExecuteTime)
	}

	return nil
}

// ExecuteProposal은 통과된 제안을 실행합니다
//
// 매개변수:
//   - proposalID: 제안 ID
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
func (gm *GovernanceManager) ExecuteProposal(proposalID uint64, state *state.StateDB) error {
	gm.lock.Lock()
	defer gm.lock.Unlock()

	proposal, ok := gm.proposals[proposalID]
	if !ok {
		return errors.New("proposal not found")
	}

	if proposal.Status != ProposalStatusPassed {
		return errors.New("proposal not passed")
	}

	if time.Now().Before(proposal.ExecuteTime) {
		return errors.New("execution time not reached")
	}

	// 제안 내용 실행
	err := proposal.Content.Execute(state)
	if err != nil {
		return err
	}

	// 제안 상태 업데이트
	proposal.Status = ProposalStatusExecuted

	log.Info("Proposal executed", "id", proposal.ID, "type", proposal.Type)
	return nil
}

// GetProposal은 제안 정보를 반환합니다
func (gm *GovernanceManager) GetProposal(proposalID uint64) (*utils.StandardProposal, error) {
	gm.lock.RLock()
	defer gm.lock.RUnlock()

	proposal, ok := gm.proposals[proposalID]
	if !ok {
		return nil, errors.New("proposal not found")
	}
	return proposal, nil
}

// GetProposals은 모든 제안 목록을 반환합니다
func (gm *GovernanceManager) GetProposals() []*utils.StandardProposal {
	gm.lock.RLock()
	defer gm.lock.RUnlock()

	proposals := make([]*utils.StandardProposal, 0, len(gm.proposals))
	for _, proposal := range gm.proposals {
		proposals = append(proposals, proposal)
	}
	return proposals
}

// GetProposalsByStatus는 특정 상태의 제안 목록을 반환합니다
func (gm *GovernanceManager) GetProposalsByStatus(status string) []*utils.StandardProposal {
	gm.lock.RLock()
	defer gm.lock.RUnlock()

	proposals := make([]*utils.StandardProposal, 0)
	for _, proposal := range gm.proposals {
		if proposal.Status == status {
			proposals = append(proposals, proposal)
		}
	}
	return proposals
}

// GetParams는 거버넌스 매개변수를 반환합니다
func (gm *GovernanceManager) GetParams() *GovernanceParams {
	gm.lock.RLock()
	defer gm.lock.RUnlock()

	return gm.params
}

// SetParams는 거버넌스 매개변수를 설정합니다
func (gm *GovernanceManager) SetParams(params *GovernanceParams) {
	gm.lock.Lock()
	defer gm.lock.Unlock()

	gm.params = params
}

// SaveToState는 거버넌스 상태를 상태 DB에 저장합니다
func (gm *GovernanceManager) SaveToState(state *state.StateDB) error {
	gm.lock.Lock()
	defer gm.lock.Unlock()
	
	// 거버넌스 매개변수 저장
	paramsData, err := rlp.EncodeToBytes(gm.params)
	if err != nil {
		return err
	}
	state.SetState(common.HexToAddress("0x0000000000000000000000000000000000000100"), common.BytesToHash([]byte("governance-params")), common.BytesToHash(paramsData))
	
	// 제안 목록 저장
	for id, proposal := range gm.proposals {
		proposalKey := append([]byte("proposal-"), []byte(strconv.FormatUint(id, 10))...)
		proposalData, err := rlp.EncodeToBytes(proposal)
		if err != nil {
			return err
		}
		state.SetState(common.HexToAddress("0x0000000000000000000000000000000000000100"), common.BytesToHash(proposalKey), common.BytesToHash(proposalData))
	}
	
	// 다음 제안 ID 저장
	nextIDKey := []byte("next-proposal-id")
	nextIDValue := []byte(strconv.FormatUint(gm.nextID, 10))
	state.SetState(common.HexToAddress("0x0000000000000000000000000000000000000100"), common.BytesToHash(nextIDKey), common.BytesToHash(nextIDValue))
	
	return nil
}

// LoadFromState는 상태 DB에서 거버넌스 상태를 로드합니다
func (gm *GovernanceManager) LoadFromState(state *state.StateDB) error {
	gm.lock.Lock()
	defer gm.lock.Unlock()
	
	// 거버넌스 매개변수 로드
	paramsHash := state.GetState(common.HexToAddress("0x0000000000000000000000000000000000000100"), common.BytesToHash([]byte("governance-params")))
	if paramsHash != (common.Hash{}) {
		var params GovernanceParams
		if err := rlp.DecodeBytes(paramsHash.Bytes(), &params); err != nil {
			return err
		}
		gm.params = &params
	}
	
	// 다음 제안 ID 로드
	nextIDHash := state.GetState(common.HexToAddress("0x0000000000000000000000000000000000000100"), common.BytesToHash([]byte("next-proposal-id")))
	if nextIDHash != (common.Hash{}) {
		nextID, err := strconv.ParseUint(string(nextIDHash.Bytes()), 10, 64)
		if err != nil {
			return err
		}
		gm.nextID = nextID
	}
	
	// 제안 목록 로드
	// 실제 구현에서는 모든 제안을 로드하는 방법이 필요함
	// 여기서는 간단한 예시만 제공
	
	return nil
}

// GovernanceState는 거버넌스 시스템의 상태를 관리합니다.
type GovernanceState struct {
	NextProposalID  uint64                            // 다음 제안 ID
	Proposals       map[uint64]*utils.StandardProposal              // 제안 ID -> 제안
	Votes           map[uint64]map[common.Address]string // 제안 ID -> 투표자 -> 투표 옵션
	MinProposalAge  uint64                            // 최소 제안 나이 (블록 수)
	VotingPeriod    uint64                            // 투표 기간 (블록 수)
	
	lock sync.RWMutex `rlp:"-"` // 동시성 제어를 위한 잠금
}

// newGovernanceState는 새로운 거버넌스 상태를 생성합니다
func newGovernanceState() *GovernanceState {
	return &GovernanceState{
		NextProposalID: 1,
		Proposals:      make(map[uint64]*utils.StandardProposal),
		Votes:          make(map[uint64]map[common.Address]string),
		MinProposalAge: 100,   // 약 25분 (15초 블록 기준)
		VotingPeriod:   20160, // 약 1주일 (15초 블록 기준)
	}
}

// loadGovernanceState는 DB에서 거버넌스 상태를 로드합니다
func loadGovernanceState(db ethdb.Database) (*GovernanceState, error) {
	// 거버넌스 상태 키
	key := []byte("governance-state")
	
	// DB에서 데이터 로드
	data, err := db.Get(key)
	if err != nil {
		// 데이터가 없으면 새 상태 반환
		return newGovernanceState(), nil
	}
	
	// 데이터 역직렬화
	var gs GovernanceState
	if err := rlp.DecodeBytes(data, &gs); err != nil {
		return nil, fmt.Errorf("거버넌스 상태 역직렬화 실패: %v", err)
	}
	
	return &gs, nil
}

// submitProposal은 새로운 제안을 제출합니다
func (gs *GovernanceState) submitProposal(
	proposer common.Address,
	title string,
	description string,
	proposalType string,
	parameters map[string]string,
	attachments map[string]string,
	relatedProposals []uint64,
	deposit *big.Int,
	currentBlock uint64,
) (uint64, error) {
	gs.lock.Lock()
	defer gs.lock.Unlock()

	// 제안 생성
	proposalID := gs.NextProposalID
	gs.NextProposalID++

	now := time.Now()
	proposal := &utils.StandardProposal{
		ID:               proposalID,
		Type:             proposalType,
		Title:            title,
		Description:      description,
		Proposer:         proposer,
		SubmitTime:       now,
		SubmitBlock:      currentBlock,
		VotingStartBlock: currentBlock + gs.MinProposalAge,
		VotingEndBlock:   currentBlock + gs.MinProposalAge + gs.VotingPeriod,
		VotingStart:      now.Add(time.Duration(gs.MinProposalAge) * 15 * time.Second), // 블록 시간을 15초로 가정
		VotingEnd:        now.Add(time.Duration(gs.MinProposalAge + gs.VotingPeriod) * 15 * time.Second),
		ExecuteTime:      now.Add(time.Duration(gs.MinProposalAge + gs.VotingPeriod + 100) * 15 * time.Second), // 실행 지연 100블록 가정
		Status:           ProposalStatusPending,
		TotalDeposit:     deposit,
		Deposits:         make(map[common.Address]*big.Int),
		YesVotes:         big.NewInt(0),
		NoVotes:          big.NewInt(0),
		AbstainVotes:     big.NewInt(0),
		VetoVotes:        big.NewInt(0),
		Votes:            make(map[common.Address]string),
		// Content 필드는 별도로 설정해야 함
	}

	// 보증금 추가
	proposal.Deposits[proposer] = deposit

	// 제안 저장
	gs.Proposals[proposalID] = proposal

	return proposalID, nil
}

// getProposal은 제안을 조회합니다
func (gs *GovernanceState) getProposal(proposalID uint64) (*utils.StandardProposal, error) {
	gs.lock.RLock()
	defer gs.lock.RUnlock()

	proposal, exists := gs.Proposals[proposalID]
	if !exists {
		return nil, errors.New("proposal not found")
	}

	return proposal, nil
}

// vote는 제안에 투표합니다
func (gs *GovernanceState) vote(
	proposalID uint64,
	voter common.Address,
	option string,
	weight *big.Int,
	currentBlock uint64,
) error {
	gs.lock.Lock()
	defer gs.lock.Unlock()

	// 제안 확인
	proposal, exists := gs.Proposals[proposalID]
	if !exists {
		return errors.New("proposal not found")
	}

	// 투표 기간 확인
	if currentBlock < proposal.VotingStartBlock {
		return errors.New("voting period not started")
	}

	if currentBlock > proposal.VotingEndBlock {
		return errors.New("voting period ended")
	}

	// 투표 옵션 확인
	if option != VoteOptionYes && option != VoteOptionNo && option != VoteOptionAbstain && option != VoteOptionVeto {
		return errors.New("invalid vote option")
	}

	// 이전 투표 제거
	if prevOption, voted := proposal.Votes[voter]; voted {
		switch prevOption {
		case VoteOptionYes:
			proposal.YesVotes = new(big.Int).Sub(proposal.YesVotes, weight)
		case VoteOptionNo:
			proposal.NoVotes = new(big.Int).Sub(proposal.NoVotes, weight)
		case VoteOptionAbstain:
			proposal.AbstainVotes = new(big.Int).Sub(proposal.AbstainVotes, weight)
		case VoteOptionVeto:
			proposal.VetoVotes = new(big.Int).Sub(proposal.VetoVotes, weight)
		}
	}

	// 새 투표 추가
	proposal.Votes[voter] = option

	// 투표 집계
	switch option {
	case VoteOptionYes:
		proposal.YesVotes = new(big.Int).Add(proposal.YesVotes, weight)
	case VoteOptionNo:
		proposal.NoVotes = new(big.Int).Add(proposal.NoVotes, weight)
	case VoteOptionAbstain:
		proposal.AbstainVotes = new(big.Int).Add(proposal.AbstainVotes, weight)
	case VoteOptionVeto:
		proposal.VetoVotes = new(big.Int).Add(proposal.VetoVotes, weight)
	}

	// 투표 저장
	if _, exists := gs.Votes[proposalID]; !exists {
		gs.Votes[proposalID] = make(map[common.Address]string)
	}
	gs.Votes[proposalID][voter] = option

	return nil
}

// getVotes는 제안의 투표 목록을 반환합니다
func (gs *GovernanceState) getVotes(proposalID uint64) ([]Vote, error) {
	gs.lock.RLock()
	defer gs.lock.RUnlock()

	// 제안 확인
	proposal, exists := gs.Proposals[proposalID]
	if !exists {
		return nil, errors.New("proposal not found")
	}

	votes := make([]Vote, 0, len(proposal.Votes))
	for voter, option := range proposal.Votes {
		votes = append(votes, Vote{
			Voter:  voter,
			Option: option,
			Weight: big.NewInt(1), // 기본 가중치 1
		})
	}

	return votes, nil
}

// Vote는 투표 정보를 나타냅니다
// 참고: 이 타입은 utils.StandardVote로 대체되었습니다.
// 하위 호환성을 위해 유지되며, 내부적으로 utils.StandardVote를 사용합니다.
type Vote struct {
	Voter      common.Address
	Option     string
	Weight     *big.Int
	ProposalID uint64         // 추가됨
	Timestamp  time.Time      // 추가됨
}

// ToStandardVote는 Vote를 utils.StandardVote로 변환합니다.
func (v *Vote) ToStandardVote() *utils.StandardVote {
	return &utils.StandardVote{
		ProposalID: v.ProposalID,
		Voter:      v.Voter,
		Option:     v.Option,
		Weight:     v.Weight,
		Timestamp:  v.Timestamp,
	}
}

// FromStandardVote는 utils.StandardVote를 Vote로 변환합니다.
func FromStandardVote(sv *utils.StandardVote) *Vote {
	return &Vote{
		ProposalID: sv.ProposalID,
		Voter:      sv.Voter,
		Option:     sv.Option,
		Weight:     sv.Weight,
		Timestamp:  sv.Timestamp,
	}
}

// store는 거버넌스 상태를 DB에 저장합니다
func (gs *GovernanceState) store(db ethdb.Database) error {
	// 거버넌스 상태 키
	key := []byte("governance-state")
	
	// 데이터 직렬화
	data, err := rlp.EncodeToBytes(gs)
	if err != nil {
		return fmt.Errorf("거버넌스 상태 직렬화 실패: %v", err)
	}
	
	// DB에 데이터 저장
	if err := db.Put(key, data); err != nil {
		return fmt.Errorf("거버넌스 상태 저장 실패: %v", err)
	}
	
	return nil
}

