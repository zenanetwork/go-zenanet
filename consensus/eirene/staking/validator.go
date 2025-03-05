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

package staking

import (
	"errors"
	"math/big"
	"sort"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/rlp"
)

// 검증자 상태 상수
const (
	ValidatorStatusActive    = 0 // 활성 상태
	ValidatorStatusJailed    = 1 // 감금 상태
	ValidatorStatusUnbonding = 2 // 언본딩 상태
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
)

// ValidatorDelegation은 검증자에게 위임된 정보를 나타냅니다.
type ValidatorDelegation struct {
	Delegator          common.Address `json:"delegator"`          // 위임자 주소
	Amount             *big.Int       `json:"amount"`             // 위임 양
	AccumulatedRewards *big.Int       `json:"accumulatedRewards"` // 누적 보상
	StartBlock         uint64         `json:"startBlock"`         // 위임 시작 블록
	EndBlock           uint64         `json:"endBlock"`           // 위임 종료 블록 (언본딩 중인 경우)
}

// Validator는 검증자 정보를 나타냅니다.
type Validator struct {
	Address     common.Address `json:"address"`     // 검증자 주소
	PubKey      []byte         `json:"pubKey"`      // 검증자 공개키
	VotingPower *big.Int       `json:"votingPower"` // 투표 파워 (스테이킹 양)
	Status      uint8          `json:"status"`      // 검증자 상태

	// 성능 지표
	BlocksProposed  uint64 `json:"blocksProposed"`  // 제안한 블록 수
	BlocksSigned    uint64 `json:"blocksSigned"`    // 서명한 블록 수
	BlocksMissed    uint64 `json:"blocksMissed"`    // 놓친 블록 수
	Uptime          uint64 `json:"uptime"`          // 업타임 (%)
	GovernanceVotes uint64 `json:"governanceVotes"` // 참여한 거버넌스 투표 수

	// 위임 정보
	SelfStake   *big.Int                                `json:"selfStake"`   // 자체 스테이킹 양
	Delegations map[common.Address]*ValidatorDelegation `json:"delegations"` // 위임 정보

	// 보상 정보
	AccumulatedRewards *big.Int `json:"accumulatedRewards"` // 누적 보상
	Commission         uint64   `json:"commission"`         // 수수료 (%)
	LastRewardBlock    uint64   `json:"lastRewardBlock"`    // 마지막 보상 블록

	// 슬래싱 정보
	JailedUntil      uint64 `json:"jailedUntil"`      // 감금 해제 블록
	SlashingCount    uint64 `json:"slashingCount"`    // 슬래싱 횟수
	LastSlashedBlock uint64 `json:"lastSlashedBlock"` // 마지막 슬래싱 블록
}

// ValidatorSet은 검증자 집합을 나타냅니다.
type ValidatorSet struct {
	Validators  map[common.Address]*Validator `json:"validators"`  // 검증자 맵
	TotalStake  *big.Int                      `json:"totalStake"`  // 총 스테이킹 양
	BlockHeight uint64                        `json:"blockHeight"` // 마지막 업데이트 블록 높이
}

// newValidatorSet은 새로운 검증자 집합을 생성합니다.
func newValidatorSet() *ValidatorSet {
	return &ValidatorSet{
		Validators:  make(map[common.Address]*Validator),
		TotalStake:  new(big.Int),
		BlockHeight: 0,
	}
}

// loadValidatorSet은 데이터베이스에서 검증자 집합을 로드합니다.
func loadValidatorSet(db ethdb.Database) (*ValidatorSet, error) {
	data, err := db.Get([]byte("eirene-validators"))
	if err != nil {
		// 데이터가 없으면 새로운 검증자 집합 생성
		return newValidatorSet(), nil
	}

	var validatorSet ValidatorSet
	if err := rlp.DecodeBytes(data, &validatorSet); err != nil {
		return nil, err
	}

	return &validatorSet, nil
}

// store는 검증자 집합을 데이터베이스에 저장합니다.
func (vs *ValidatorSet) store(db ethdb.Database) error {
	data, err := rlp.EncodeToBytes(vs)
	if err != nil {
		return err
	}

	return db.Put([]byte("eirene-validators"), data)
}

// addValidator는 새로운 검증자를 추가합니다.
func (vs *ValidatorSet) addValidator(validator *Validator) {
	vs.Validators[validator.Address] = validator
	vs.TotalStake = new(big.Int).Add(vs.TotalStake, validator.VotingPower)
}

// removeValidator는 검증자를 제거합니다.
func (vs *ValidatorSet) removeValidator(address common.Address) {
	validator, exists := vs.Validators[address]
	if !exists {
		return
	}

	vs.TotalStake = new(big.Int).Sub(vs.TotalStake, validator.VotingPower)
	delete(vs.Validators, address)
}

// updateValidator는 검증자 정보를 업데이트합니다.
func (vs *ValidatorSet) updateValidator(validator *Validator) {
	oldValidator, exists := vs.Validators[validator.Address]
	if exists {
		vs.TotalStake = new(big.Int).Sub(vs.TotalStake, oldValidator.VotingPower)
	}

	vs.Validators[validator.Address] = validator
	vs.TotalStake = new(big.Int).Add(vs.TotalStake, validator.VotingPower)
}

// getActiveValidators는 활성 검증자 목록을 반환합니다.
func (vs *ValidatorSet) getActiveValidators() []*Validator {
	activeValidators := make([]*Validator, 0)

	for _, validator := range vs.Validators {
		if validator.Status == ValidatorStatusActive {
			activeValidators = append(activeValidators, validator)
		}
	}

	return activeValidators
}

// getTopValidators는 상위 N개의 검증자를 반환합니다.
func (vs *ValidatorSet) getTopValidators(n int) []*Validator {
	// 모든 검증자를 슬라이스로 변환
	validators := make([]*Validator, 0, len(vs.Validators))
	for _, validator := range vs.Validators {
		if validator.Status == ValidatorStatusActive {
			validators = append(validators, validator)
		}
	}

	// 검증자를 투표 파워에 따라 정렬
	sort.Slice(validators, func(i, j int) bool {
		return validators[i].VotingPower.Cmp(validators[j].VotingPower) > 0
	})

	// 상위 N개 반환 (또는 전체 검증자 수가 N보다 작으면 전체 반환)
	if len(validators) <= n {
		return validators
	}
	return validators[:n]
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
	if proposer, exists := vs.Validators[proposer]; exists {
		proposer.BlocksProposed++
		proposer.BlocksMissed-- // 제안자는 블록을 놓치지 않음
	}

	// 서명자들의 서명한 블록 수 증가
	for _, signer := range signers {
		if validator, exists := vs.Validators[signer]; exists {
			validator.BlocksSigned++
			validator.BlocksMissed-- // 서명자는 블록을 놓치지 않음
		}
	}

	// 업타임 계산 (최근 100개 블록 기준)
	if vs.BlockHeight%100 == 0 {
		for _, validator := range vs.Validators {
			if validator.Status == ValidatorStatusActive {
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
	validator, exists := vs.Validators[address]
	if !exists {
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
	validator, exists := vs.Validators[address]
	if !exists {
		return
	}

	if validator.Status != ValidatorStatusJailed {
		return
	}

	if vs.BlockHeight < validator.JailedUntil {
		return
	}

	validator.Status = ValidatorStatusActive
	log.Info("Validator unjailed", "address", address)
}

// slashValidator는 검증자를 슬래싱합니다.
func (vs *ValidatorSet) slashValidator(address common.Address, slashRatio uint64) {
	validator, exists := vs.Validators[address]
	if !exists {
		return
	}

	// 투표 파워 감소
	slashAmount := new(big.Int).Mul(validator.VotingPower, big.NewInt(int64(slashRatio)))
	slashAmount = new(big.Int).Div(slashAmount, big.NewInt(100))

	vs.TotalStake = new(big.Int).Sub(vs.TotalStake, slashAmount)
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

// getValidatorByAddress는 주소로 검증자를 조회합니다.
func (vs *ValidatorSet) getValidatorByAddress(address common.Address) *Validator {
	return vs.Validators[address]
}

// getValidatorCount는 검증자 수를 반환합니다.
func (vs *ValidatorSet) getValidatorCount() int {
	return len(vs.Validators)
}

// getActiveValidatorCount는 활성 검증자 수를 반환합니다.
func (vs *ValidatorSet) getActiveValidatorCount() int {
	count := 0
	for _, validator := range vs.Validators {
		if validator.Status == ValidatorStatusActive {
			count++
		}
	}
	return count
}

// getTotalStake는 총 스테이킹 양을 반환합니다.
func (vs *ValidatorSet) getTotalStake() *big.Int {
	return new(big.Int).Set(vs.TotalStake)
}

// getValidatorsByStatus는 상태별 검증자 목록을 반환합니다.
func (vs *ValidatorSet) getValidatorsByStatus(status uint8) []*Validator {
	validators := make([]*Validator, 0)
	for _, validator := range vs.Validators {
		if validator.Status == status {
			validators = append(validators, validator)
		}
	}
	return validators
}

// processEpochTransition은 에포크 전환 시 검증자 집합을 업데이트합니다.
func (vs *ValidatorSet) processEpochTransition(totalBlocks uint64) {
	// 감금 해제 처리
	for _, validator := range vs.Validators {
		if validator.Status == ValidatorStatusJailed && vs.BlockHeight >= validator.JailedUntil {
			validator.Status = ValidatorStatusActive
		}
	}

	// 다음 에포크의 검증자 선택
	selectedValidators := vs.selectValidators(totalBlocks)

	log.Info("Epoch transition: selected validators", "count", len(selectedValidators))
}

// addDelegation은 위임을 추가합니다.
func (vs *ValidatorSet) addDelegation(validatorAddr common.Address, delegatorAddr common.Address, amount *big.Int, startBlock uint64) error {
	validator, exists := vs.Validators[validatorAddr]
	if !exists {
		return ErrValidatorNotFound
	}

	if validator.Status != ValidatorStatusActive {
		return ErrValidatorNotActive
	}

	// 기존 위임이 있는지 확인
	delegation, exists := validator.Delegations[delegatorAddr]
	if exists {
		// 기존 위임에 추가
		delegation.Amount = new(big.Int).Add(delegation.Amount, amount)
	} else {
		// 새 위임 생성
		delegation = &ValidatorDelegation{
			Delegator:          delegatorAddr,
			Amount:             new(big.Int).Set(amount),
			AccumulatedRewards: new(big.Int),
			StartBlock:         startBlock,
			EndBlock:           0,
		}
		validator.Delegations[delegatorAddr] = delegation
	}

	// 검증자의 투표 파워 증가
	validator.VotingPower = new(big.Int).Add(validator.VotingPower, amount)

	// 총 스테이킹 양 증가
	vs.TotalStake = new(big.Int).Add(vs.TotalStake, amount)

	return nil
}

// removeDelegation은 위임을 제거합니다.
func (vs *ValidatorSet) removeDelegation(validatorAddr common.Address, delegatorAddr common.Address, amount *big.Int, endBlock uint64) error {
	validator, exists := vs.Validators[validatorAddr]
	if !exists {
		return ErrValidatorNotFound
	}

	delegation, exists := validator.Delegations[delegatorAddr]
	if !exists {
		return ErrDelegationNotFound
	}

	// 위임 금액이 충분한지 확인
	if delegation.Amount.Cmp(amount) < 0 {
		return ErrInsufficientDelegation
	}

	// 위임 금액 감소
	delegation.Amount = new(big.Int).Sub(delegation.Amount, amount)
	delegation.EndBlock = endBlock

	// 위임 금액이 0이면 위임 제거
	if delegation.Amount.Cmp(big.NewInt(0)) == 0 {
		delete(validator.Delegations, delegatorAddr)
	}

	// 검증자의 투표 파워 감소
	validator.VotingPower = new(big.Int).Sub(validator.VotingPower, amount)

	// 총 스테이킹 양 감소
	vs.TotalStake = new(big.Int).Sub(vs.TotalStake, amount)

	return nil
}

// 오류 정의
var (
	ErrValidatorNotFound      = errors.New("validator not found")
	ErrValidatorNotActive     = errors.New("validator not active")
	ErrDelegationNotFound     = errors.New("delegation not found")
	ErrInsufficientDelegation = errors.New("insufficient delegation amount")
)
