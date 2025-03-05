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

// Package staking implements the staking module for the Eirene consensus algorithm.
package staking

import (
	"errors"
	"math/big"
	"sort"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/log"
)

// 검증자 성능 가중치 상수
const (
	// 검증자 선택에 사용되는 가중치 (1000 단위)
	stakingWeightRatio     = 700 // 스테이킹 양 가중치 (70%)
	performanceWeightRatio = 300 // 성능 가중치 (30%)

	// 성능 지표 가중치 (1000 단위)
	blocksMissedWeight   = 400 // 놓친 블록 가중치 (40%)
	blocksSignedWeight   = 300 // 서명한 블록 가중치 (30%)
	uptimeWeight         = 200 // 업타임 가중치 (20%)
	governanceVoteWeight = 100 // 거버넌스 참여 가중치 (10%)

	// 최대 검증자 수
	maxValidators = 100

	// 최소 스테이킹 금액
	MinStake = 1000 * 1e18 // 1000 토큰

	// 위임자당 최소 위임 금액
	MinDelegation = 10 * 1e18 // 10 토큰

	// 슬래싱 비율
	DoubleSignSlashingRate  = 0.1  // 이중 서명 시 10% 슬래싱
	DowntimeSlashingRate    = 0.01 // 다운타임 시 1% 슬래싱
	MisbehaviorSlashingRate = 0.05 // 기타 악의적 행동 시 5% 슬래싱

	// 감금 기간
	JailPeriod = 86400 // 1일 (초 단위)
)

// ValidatorStatus는 검증자의 상태를 나타냅니다.
type ValidatorStatus int

const (
	ValidatorStatusUnbonded  ValidatorStatus = iota // 언본딩 상태 (활성화되지 않음)
	ValidatorStatusBonded                           // 본딩 상태 (활성화됨)
	ValidatorStatusUnbonding                        // 언본딩 중 (활성화 해제 중)
	ValidatorStatusJailed                           // 감금 상태
)

// String은 ValidatorStatus를 문자열로 변환합니다.
func (s ValidatorStatus) String() string {
	switch s {
	case ValidatorStatusUnbonded:
		return "Unbonded"
	case ValidatorStatusBonded:
		return "Bonded"
	case ValidatorStatusUnbonding:
		return "Unbonding"
	case ValidatorStatusJailed:
		return "Jailed"
	default:
		return "Unknown"
	}
}

// Validator는 검증자 정보를 담고 있습니다.
type Validator struct {
	Address     common.Address      // 검증자 주소
	PubKey      []byte              // 검증자 공개 키
	VotingPower *big.Int            // 투표력 (스테이킹된 토큰 양)
	Commission  *big.Int            // 커미션 비율 (0-100%)
	Description ValidatorDescription // 검증자 설명
	Status      ValidatorStatus     // 검증자 상태
	Delegations []*ValidatorDelegation // 위임 정보

	// 성능 지표
	BlocksProposed  uint64 `json:"blocksProposed"`  // 제안한 블록 수
	BlocksSigned    uint64 `json:"blocksSigned"`    // 서명한 블록 수
	BlocksMissed    uint64 `json:"blocksMissed"`    // 놓친 블록 수
	Uptime          uint64 `json:"uptime"`          // 업타임 (%)
	GovernanceVotes uint64 `json:"governanceVotes"` // 참여한 거버넌스 투표 수

	// 위임 정보
	SelfStake   *big.Int `json:"selfStake"`   // 자체 스테이킹 양
	AccumulatedRewards *big.Int `json:"accumulatedRewards"` // 누적 보상
	LastRewardBlock    uint64   `json:"lastRewardBlock"`    // 마지막 보상 블록

	// 슬래싱 정보
	JailedUntil      uint64 `json:"jailedUntil"`      // 감금 해제 블록
	SlashingCount    uint64 `json:"slashingCount"`    // 슬래싱 횟수
	LastSlashedBlock uint64 `json:"lastSlashedBlock"` // 마지막 슬래싱 블록
}

// NewValidator는 새로운 Validator 인스턴스를 생성합니다.
func NewValidator(address common.Address, pubKey []byte, votingPower *big.Int, commission *big.Int, description ValidatorDescription) *Validator {
	return &Validator{
		Address:     address,
		PubKey:      pubKey,
		VotingPower: votingPower,
		Commission:  commission,
		Description: description,
		Status:      ValidatorStatusUnbonded,
		Delegations: []*ValidatorDelegation{},
		SelfStake:   big.NewInt(0),
		AccumulatedRewards: big.NewInt(0),
	}
}

// AddDelegation은 검증자에게 위임을 추가합니다.
func (v *Validator) AddDelegation(delegation *ValidatorDelegation) {
	v.Delegations = append(v.Delegations, delegation)
	v.VotingPower = new(big.Int).Add(v.VotingPower, delegation.Amount)
}

// RemoveDelegation은 검증자에게서 위임을 제거합니다.
func (v *Validator) RemoveDelegation(delegator common.Address) *ValidatorDelegation {
	for i, d := range v.Delegations {
		if d.Delegator == delegator {
			v.VotingPower = new(big.Int).Sub(v.VotingPower, d.Amount)
			v.Delegations = append(v.Delegations[:i], v.Delegations[i+1:]...)
			return d
		}
	}
	return nil
}

// GetDelegation은 특정 위임자의 위임 정보를 반환합니다.
func (v *Validator) GetDelegation(delegator common.Address) *ValidatorDelegation {
	for _, d := range v.Delegations {
		if d.Delegator == delegator {
			return d
		}
	}
	return nil
}

// UpdateDelegation은 특정 위임자의 위임 정보를 업데이트합니다.
func (v *Validator) UpdateDelegation(delegator common.Address, amount *big.Int) {
	for _, d := range v.Delegations {
		if d.Delegator == delegator {
			v.VotingPower = new(big.Int).Sub(v.VotingPower, d.Amount)
			d.Amount = amount
			v.VotingPower = new(big.Int).Add(v.VotingPower, amount)
			return
		}
	}
}

// ToCosmosValidator는 Validator를 Cosmos SDK의 Validator 형식으로 변환합니다.
func (v *Validator) ToCosmosValidator() interface{} {
	// Cosmos SDK의 Validator 형식으로 변환하는 로직 구현
	// 실제 구현에서는 Cosmos SDK의 타입을 임포트하여 변환해야 함
	return nil
}

// FromCosmosValidator는 Cosmos SDK의 Validator를 Validator로 변환합니다.
func FromCosmosValidator(cosmosValidator interface{}) *Validator {
	// Cosmos SDK의 Validator를 Validator로 변환하는 로직 구현
	// 실제 구현에서는 Cosmos SDK의 타입을 임포트하여 변환해야 함
	return nil
}

// ValidatorSet은 검증자 집합을 관리합니다.
type ValidatorSet struct {
	Validators []*Validator // 검증자 목록
	BlockHeight uint64      // 마지막 업데이트 블록 높이
}

// NewValidatorSet은 새로운 ValidatorSet 인스턴스를 생성합니다.
func NewValidatorSet() *ValidatorSet {
	return &ValidatorSet{
		Validators: []*Validator{},
		BlockHeight: 0,
	}
}

// AddValidator는 검증자 집합에 검증자를 추가합니다.
func (vs *ValidatorSet) AddValidator(validator *Validator) {
	vs.Validators = append(vs.Validators, validator)
}

// RemoveValidator는 검증자 집합에서 검증자를 제거합니다.
func (vs *ValidatorSet) RemoveValidator(address common.Address) *Validator {
	for i, v := range vs.Validators {
		if v.Address == address {
			vs.Validators = append(vs.Validators[:i], vs.Validators[i+1:]...)
			return v
		}
	}
	return nil
}

// GetValidator는 주소로 검증자를 조회합니다.
func (vs *ValidatorSet) GetValidator(address common.Address) *Validator {
	for _, v := range vs.Validators {
		if v.Address == address {
			return v
		}
	}
	return nil
}

// GetValidators는 모든 검증자를 반환합니다.
func (vs *ValidatorSet) GetValidators() []*Validator {
	return vs.Validators
}

// GetActiveValidators는 활성화된 검증자만 반환합니다.
func (vs *ValidatorSet) getActiveValidators() []*Validator {
	var activeValidators []*Validator
	for _, v := range vs.Validators {
		if v.Status == ValidatorStatusBonded {
			activeValidators = append(activeValidators, v)
		}
	}
	return activeValidators
}

// GetTotalVotingPower는 모든 검증자의 총 투표력을 반환합니다.
func (vs *ValidatorSet) GetTotalVotingPower() *big.Int {
	total := big.NewInt(0)
	for _, v := range vs.Validators {
		total = new(big.Int).Add(total, v.VotingPower)
	}
	return total
}

// GetTotalActiveVotingPower는 활성화된 검증자의 총 투표력을 반환합니다.
func (vs *ValidatorSet) GetTotalActiveVotingPower() *big.Int {
	total := big.NewInt(0)
	for _, v := range vs.Validators {
		if v.Status == ValidatorStatusBonded {
			total = new(big.Int).Add(total, v.VotingPower)
		}
	}
	return total
}

// calculateValidatorScore는 검증자의 점수를 계산합니다.
func calculateValidatorScore(validator *Validator, totalBlocks uint64) *big.Int {
	// 스테이킹 양 점수 (70%)
	stakingScore := new(big.Int).Mul(validator.VotingPower, big.NewInt(stakingWeightRatio))
	stakingScore = new(big.Int).Div(stakingScore, big.NewInt(1000))

	// 성능 점수 계산 (30%)
	performanceScore := big.NewInt(0)

	// 1. 놓친 블록 점수 (40%)
	missedRatio := float64(validator.BlocksMissed) / float64(totalBlocks)
	missedScore := 1.0 - missedRatio
	if missedScore < 0 {
		missedScore = 0
	}
	missedScoreInt := new(big.Int).SetUint64(uint64(missedScore * 1000))
	missedScoreInt = new(big.Int).Mul(missedScoreInt, big.NewInt(blocksMissedWeight))
	missedScoreInt = new(big.Int).Div(missedScoreInt, big.NewInt(1000))

	// 2. 서명한 블록 점수 (30%)
	signedRatio := float64(validator.BlocksSigned) / float64(totalBlocks)
	signedScoreInt := new(big.Int).SetUint64(uint64(signedRatio * 1000))
	signedScoreInt = new(big.Int).Mul(signedScoreInt, big.NewInt(blocksSignedWeight))
	signedScoreInt = new(big.Int).Div(signedScoreInt, big.NewInt(1000))

	// 3. 업타임 점수 (20%)
	uptimeScoreInt := new(big.Int).SetUint64(validator.Uptime)
	uptimeScoreInt = new(big.Int).Mul(uptimeScoreInt, big.NewInt(uptimeWeight))
	uptimeScoreInt = new(big.Int).Div(uptimeScoreInt, big.NewInt(1000))

	// 4. 거버넌스 참여 점수 (10%)
	// 최대 10개의 투표를 고려
	govVotes := validator.GovernanceVotes
	if govVotes > 10 {
		govVotes = 10
	}
	govScoreInt := new(big.Int).SetUint64(govVotes * 100) // 0-1000 범위로 변환
	govScoreInt = new(big.Int).Mul(govScoreInt, big.NewInt(governanceVoteWeight))
	govScoreInt = new(big.Int).Div(govScoreInt, big.NewInt(1000))

	// 성능 점수 합산
	performanceScore = new(big.Int).Add(performanceScore, missedScoreInt)
	performanceScore = new(big.Int).Add(performanceScore, signedScoreInt)
	performanceScore = new(big.Int).Add(performanceScore, uptimeScoreInt)
	performanceScore = new(big.Int).Add(performanceScore, govScoreInt)

	// 성능 가중치 적용
	performanceScore = new(big.Int).Mul(performanceScore, big.NewInt(performanceWeightRatio))
	performanceScore = new(big.Int).Div(performanceScore, big.NewInt(1000))

	// 최종 점수 계산
	totalScore := new(big.Int).Add(stakingScore, performanceScore)

	return totalScore
}

// selectValidators는 다음 에포크의 검증자를 선택합니다.
func (vs *ValidatorSet) selectValidators(totalBlocks uint64) []*Validator {
	// 모든 활성 검증자를 슬라이스로 변환
	validators := vs.getActiveValidators()

	// 각 검증자의 점수 계산
	type validatorScore struct {
		validator *Validator
		score     *big.Int
	}

	scores := make([]validatorScore, len(validators))
	for i, validator := range validators {
		scores[i] = validatorScore{
			validator: validator,
			score:     calculateValidatorScore(validator, totalBlocks),
		}
	}

	// 점수에 따라 정렬
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score.Cmp(scores[j].score) > 0
	})

	// 상위 maxValidators개 선택
	result := make([]*Validator, 0, maxValidators)
	for i := 0; i < len(scores) && i < maxValidators; i++ {
		result = append(result, scores[i].validator)
	}

	return result
}

// updateValidatorPerformance는 검증자의 성능 지표를 업데이트합니다.
func (vs *ValidatorSet) updateValidatorPerformance(header *types.Header, proposer common.Address, signers []common.Address) {
	// 블록 높이 업데이트
	vs.BlockHeight = header.Number.Uint64()

	// 모든 활성 검증자에 대해 놓친 블록 수 증가
	activeValidators := vs.getActiveValidators()
	for _, validator := range activeValidators {
		validator.BlocksMissed++
	}

	// 제안자의 제안한 블록 수 증가
	proposerValidator := vs.GetValidator(proposer)
	if proposerValidator != nil {
		proposerValidator.BlocksProposed++
		proposerValidator.BlocksMissed-- // 제안자는 블록을 놓치지 않음
	}

	// 서명자들의 서명한 블록 수 증가
	for _, signer := range signers {
		signerValidator := vs.GetValidator(signer)
		if signerValidator != nil {
			signerValidator.BlocksSigned++
			signerValidator.BlocksMissed-- // 서명자는 블록을 놓치지 않음
		}
	}

	// 업타임 계산 (최근 100개 블록 기준)
	if vs.BlockHeight%100 == 0 {
		for _, validator := range vs.Validators {
			if validator.Status == ValidatorStatusBonded {
				missedRatio := float64(validator.BlocksMissed) / 100.0
				uptime := uint64((1.0 - missedRatio) * 100)
				validator.Uptime = uptime

				// 블록 카운터 리셋
				validator.BlocksMissed = 0
				validator.BlocksSigned = 0
				validator.BlocksProposed = 0
			}
		}
	}
}

// jailValidator는 검증자를 감금합니다.
func (vs *ValidatorSet) jailValidator(address common.Address, jailUntil uint64) {
	validator := vs.GetValidator(address)
	if validator == nil {
		return
	}

	validator.Status = ValidatorStatusJailed
	validator.JailedUntil = jailUntil
	validator.SlashingCount++
	validator.LastSlashedBlock = vs.BlockHeight

	log.Info("Validator jailed", "address", address, "until", jailUntil)
}

// unjailValidator는 검증자의 감금을 해제합니다.
func (vs *ValidatorSet) unjailValidator(address common.Address) {
	validator := vs.GetValidator(address)
	if validator == nil {
		return
	}

	if validator.Status != ValidatorStatusJailed {
		return
	}

	if vs.BlockHeight < validator.JailedUntil {
		return
	}

	validator.Status = ValidatorStatusBonded
	log.Info("Validator unjailed", "address", address)
}

// slashValidator는 검증자를 슬래싱합니다.
func (vs *ValidatorSet) slashValidator(address common.Address, slashRatio uint64) {
	validator := vs.GetValidator(address)
	if validator == nil {
		return
	}

	// 투표 파워 감소
	slashAmount := new(big.Int).Mul(validator.VotingPower, big.NewInt(int64(slashRatio)))
	slashAmount = new(big.Int).Div(slashAmount, big.NewInt(100))

	validator.VotingPower = new(big.Int).Sub(validator.VotingPower, slashAmount)

	// 자체 스테이킹 및 위임 금액 감소
	selfSlashAmount := new(big.Int).Mul(validator.SelfStake, big.NewInt(int64(slashRatio)))
	selfSlashAmount = new(big.Int).Div(selfSlashAmount, big.NewInt(100))
	validator.SelfStake = new(big.Int).Sub(validator.SelfStake, selfSlashAmount)

	for _, delegation := range validator.Delegations {
		delegationSlashAmount := new(big.Int).Mul(delegation.Amount, big.NewInt(int64(slashRatio)))
		delegationSlashAmount = new(big.Int).Div(delegationSlashAmount, big.NewInt(100))
		delegation.Amount = new(big.Int).Sub(delegation.Amount, delegationSlashAmount)
	}

	log.Info("Validator slashed", "address", address, "ratio", slashRatio, "amount", slashAmount)
}

// GetAddress는 검증자의 주소를 반환합니다.
func (v *Validator) GetAddress() common.Address {
	return v.Address
}

// GetVotingPower는 검증자의 투표 파워를 반환합니다.
func (v *Validator) GetVotingPower() *big.Int {
	return v.VotingPower
}

// GetStatus는 검증자의 상태를 반환합니다.
func (v *Validator) GetStatus() uint8 {
	return uint8(v.Status)
}

// IsActive는 검증자가 활성 상태인지 여부를 반환합니다.
func (v *Validator) IsActive() bool {
	return v.Status == ValidatorStatusBonded
}

// GetValidatorCount는 검증자 집합의 총 검증자 수를 반환합니다.
func (vs *ValidatorSet) GetValidatorCount() int {
	return len(vs.Validators)
}

// GetActiveValidatorCount는 검증자 집합의 활성 검증자 수를 반환합니다.
func (vs *ValidatorSet) GetActiveValidatorCount() int {
	count := 0
	for _, v := range vs.Validators {
		if v.Status == ValidatorStatusBonded {
			count++
		}
	}
	return count
}

// GetTotalStake는 검증자 집합의 총 스테이킹 양을 반환합니다.
func (vs *ValidatorSet) GetTotalStake() *big.Int {
	return vs.GetTotalVotingPower()
}

// GetValidatorByAddress는 주소로 검증자를 조회합니다.
func (vs *ValidatorSet) GetValidatorByAddress(address common.Address) utils.ValidatorInterface {
	for _, v := range vs.Validators {
		if v.Address == address {
			return v
		}
	}
	return nil
}

// GetActiveValidators는 활성 상태인 검증자 목록을 반환합니다.
func (vs *ValidatorSet) GetActiveValidators() []utils.ValidatorInterface {
	var activeValidators []utils.ValidatorInterface
	for _, v := range vs.Validators {
		if v.Status == ValidatorStatusBonded {
			activeValidators = append(activeValidators, v)
		}
	}
	return activeValidators
}

// Contains는 주어진 주소의 검증자가 검증자 집합에 포함되어 있는지 확인합니다.
func (vs *ValidatorSet) Contains(address common.Address) bool {
	for _, v := range vs.Validators {
		if v.Address == address {
			return true
		}
	}
	return false
}

// processEpochTransition은 에포크 전환 시 검증자 집합을 업데이트합니다.
func (vs *ValidatorSet) processEpochTransition(totalBlocks uint64) {
	// 감금 해제 처리
	for _, validator := range vs.Validators {
		if validator.Status == ValidatorStatusJailed && vs.BlockHeight >= validator.JailedUntil {
			validator.Status = ValidatorStatusBonded
		}
	}

	// 다음 에포크의 검증자 선택
	selectedValidators := vs.selectValidators(totalBlocks)

	log.Info("Epoch transition: selected validators", "count", len(selectedValidators))
}

// 오류 정의
var (
	ErrValidatorNotFound      = errors.New("validator not found")
	ErrValidatorNotActive     = errors.New("validator not active")
	ErrDelegationNotFound     = errors.New("delegation not found")
	ErrInsufficientDelegation = errors.New("insufficient delegation amount")
)
