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
	"errors"
	"math/big"
	"strconv"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/common/hexutil"
	"github.com/zenanetwork/go-zenanet/consensus"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/rpc"
)

// API는 Eirene 합의 엔진의 RPC API를 제공합니다.
type API struct {
	chain  consensus.ChainHeaderReader
	eirene *Eirene
}

// GetValidators는 지정된 블록 번호에서 활성 검증자 목록을 반환합니다.
func (api *API) GetValidators(number *rpc.BlockNumber) ([]common.Address, error) {
	// 블록 번호 해석
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}
	// 헤더가 없으면 오류 반환
	if header == nil {
		return nil, errUnknownBlock
	}

	// 현재는 더미 데이터 반환
	return []common.Address{}, nil
}

// GetSignersAtHash는 지정된 블록 해시에서 활성 서명자 목록을 반환합니다.
func (api *API) GetSignersAtHash(hash common.Hash) ([]common.Address, error) {
	header := api.chain.GetHeaderByHash(hash)
	if header == nil {
		return nil, errUnknownBlock
	}

	// 현재는 더미 데이터 반환
	return []common.Address{}, nil
}

// Status는 로컬 서명자의 서명 상태를 반환합니다.
func (api *API) Status() map[string]interface{} {
	api.eirene.lock.RLock()
	defer api.eirene.lock.RUnlock()

	// 현재는 기본 상태 정보만 반환
	return map[string]interface{}{
		"signerAddress": api.eirene.signer,
	}
}

// Validators는 현재 검증자 집합을 반환합니다.
func (api *API) Validators(number *rpc.BlockNumber) ([]common.Address, error) {
	// 블록 번호 해석
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}
	if header == nil {
		return nil, errUnknownBlock
	}

	// 스냅샷 가져오기
	snap, err := api.eirene.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return nil, err
	}

	// 검증자 정보 수집
	validators := make([]common.Address, 0, len(snap.Validators))
	for validator := range snap.Validators {
		validators = append(validators, validator)
	}

	return validators, nil
}

// GetValidatorInfo는 특정 검증자의 정보를 반환합니다.
func (api *API) GetValidatorInfo(validator common.Address, number *rpc.BlockNumber) (map[string]interface{}, error) {
	// 블록 번호 해석
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}
	if header == nil {
		return nil, errUnknownBlock
	}

	// 스냅샷 가져오기
	snap, err := api.eirene.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return nil, err
	}

	// 검증자 정보 수집
	info := make(map[string]interface{})

	// 기본 정보
	info["isValidator"] = false
	if _, ok := snap.Validators[validator]; ok {
		info["isValidator"] = true
		info["weight"] = snap.Validators[validator]
	}

	// 스테이킹 정보
	if stake, ok := snap.Stakes[validator]; ok {
		info["stake"] = stake.String()
	} else {
		info["stake"] = "0"
	}

	// 성능 지표
	if stats, ok := snap.Performance[validator]; ok {
		info["performance"] = map[string]interface{}{
			"blocksProposed": stats.BlocksProposed,
			"blocksMissed":   stats.BlocksMissed,
			"uptime":         stats.Uptime,
			"lastActive":     stats.LastActive,
		}
	}

	// 슬래싱 정보
	if points, ok := snap.SlashingPoints[validator]; ok {
		info["slashingPoints"] = points
	} else {
		info["slashingPoints"] = 0
	}

	// 위임 정보
	if delegations, ok := snap.Delegations[validator]; ok {
		delegationInfo := make([]map[string]interface{}, 0, len(delegations))
		for _, delegation := range delegations {
			delegationInfo = append(delegationInfo, map[string]interface{}{
				"delegator": delegation.Delegator,
				"amount":    delegation.Amount.String(),
				"since":     delegation.Since,
			})
		}
		info["delegations"] = delegationInfo

		// 총 위임 금액 계산
		totalDelegated := new(big.Int)
		for _, delegation := range delegations {
			totalDelegated = new(big.Int).Add(totalDelegated, delegation.Amount)
		}
		info["totalDelegated"] = totalDelegated.String()
	} else {
		info["delegations"] = []map[string]interface{}{}
		info["totalDelegated"] = "0"
	}

	return info, nil
}

// GetDelegationInfo는 특정 주소의 위임 정보를 반환합니다.
func (api *API) GetDelegationInfo(delegator common.Address, number *rpc.BlockNumber) (map[string]interface{}, error) {
	// 블록 번호 해석
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}
	if header == nil {
		return nil, errUnknownBlock
	}

	// 스냅샷 가져오기
	snap, err := api.eirene.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return nil, err
	}

	// 위임 정보 수집
	info := make(map[string]interface{})
	delegations := make([]map[string]interface{}, 0)
	totalDelegated := new(big.Int)

	// 모든 검증자에 대한 위임 검색
	for validator, validatorDelegations := range snap.Delegations {
		for _, delegation := range validatorDelegations {
			if delegation.Delegator == delegator {
				delegations = append(delegations, map[string]interface{}{
					"validator": validator,
					"amount":    delegation.Amount.String(),
					"since":     delegation.Since,
				})
				totalDelegated = new(big.Int).Add(totalDelegated, delegation.Amount)
			}
		}
	}

	info["delegations"] = delegations
	info["totalDelegated"] = totalDelegated.String()
	info["delegationsCount"] = len(delegations)

	return info, nil
}

// Delegate는 토큰을 검증자에게 위임합니다.
func (api *API) Delegate(validator common.Address, amount *hexutil.Big) (bool, error) {
	// 현재 헤더 가져오기
	header := api.chain.CurrentHeader()
	if header == nil {
		return false, errUnknownBlock
	}

	// 스냅샷 가져오기
	snap, err := api.eirene.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return false, err
	}

	// 검증자 확인
	if _, ok := snap.Validators[validator]; !ok {
		return false, errors.New("not a validator")
	}

	// 위임 처리
	bigAmount := (*big.Int)(amount)
	if bigAmount.Sign() <= 0 {
		return false, errors.New("delegation amount must be positive")
	}

	// 위임자 주소 가져오기 (서명자 주소 사용)
	delegator := api.eirene.signer
	if (delegator == common.Address{}) {
		return false, errors.New("no signer available")
	}

	// 위임 추가
	snap.addDelegation(validator, delegator, bigAmount, header.Number.Uint64())

	// 스냅샷 저장
	if err := snap.store(api.eirene.db); err != nil {
		return false, err
	}

	return true, nil
}

// Undelegate는 검증자로부터 위임을 철회합니다.
func (api *API) Undelegate(validator common.Address, amount *hexutil.Big) (bool, error) {
	// 현재 헤더 가져오기
	header := api.chain.CurrentHeader()
	if header == nil {
		return false, errUnknownBlock
	}

	// 스냅샷 가져오기
	snap, err := api.eirene.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return false, err
	}

	// 위임자 주소 가져오기 (서명자 주소 사용)
	delegator := api.eirene.signer
	if (delegator == common.Address{}) {
		return false, errors.New("no signer available")
	}

	// 위임 제거
	bigAmount := (*big.Int)(amount)
	if bigAmount.Sign() <= 0 {
		return false, errors.New("undelegation amount must be positive")
	}

	// 위임 제거 시도
	if err := snap.removeDelegation(validator, delegator, bigAmount); err != nil {
		return false, err
	}

	// 스냅샷 저장
	if err := snap.store(api.eirene.db); err != nil {
		return false, err
	}

	return true, nil
}

// GetValidatorStats는 모든 검증자의 성능 통계를 반환합니다.
func (api *API) GetValidatorStats(number *rpc.BlockNumber) (map[string]interface{}, error) {
	// 블록 번호 해석
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}
	if header == nil {
		return nil, errUnknownBlock
	}

	// 스냅샷 가져오기
	snap, err := api.eirene.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return nil, err
	}

	// 검증자 통계 수집
	stats := make(map[string]interface{})
	validators := snap.validators()

	// 검증자 수
	stats["count"] = len(validators)

	// 총 스테이킹 양
	totalStake := new(big.Int)
	for _, validator := range validators {
		if stake, ok := snap.Stakes[validator]; ok {
			totalStake = new(big.Int).Add(totalStake, stake)
		}
	}
	stats["totalStake"] = totalStake.String()

	// 검증자별 통계
	validatorStats := make([]map[string]interface{}, 0, len(validators))
	for _, validator := range validators {
		vstat := make(map[string]interface{})
		vstat["address"] = validator
		vstat["weight"] = snap.Validators[validator]

		if stake, ok := snap.Stakes[validator]; ok {
			vstat["stake"] = stake.String()
		} else {
			vstat["stake"] = "0"
		}

		if perf, ok := snap.Performance[validator]; ok {
			vstat["performance"] = map[string]interface{}{
				"blocksProposed": perf.BlocksProposed,
				"blocksMissed":   perf.BlocksMissed,
				"uptime":         perf.Uptime,
				"lastActive":     perf.LastActive,
			}
		}

		if points, ok := snap.SlashingPoints[validator]; ok {
			vstat["slashingPoints"] = points
		} else {
			vstat["slashingPoints"] = 0
		}

		// 위임 수 및 총 위임 금액
		if delegations, ok := snap.Delegations[validator]; ok {
			vstat["delegationsCount"] = len(delegations)

			totalDelegated := new(big.Int)
			for _, delegation := range delegations {
				totalDelegated = new(big.Int).Add(totalDelegated, delegation.Amount)
			}
			vstat["totalDelegated"] = totalDelegated.String()
		} else {
			vstat["delegationsCount"] = 0
			vstat["totalDelegated"] = "0"
		}

		validatorStats = append(validatorStats, vstat)
	}
	stats["validators"] = validatorStats

	return stats, nil
}

// GovernanceAPI는 Eirene 거버넌스 시스템의 공개 API를 제공합니다.
type GovernanceAPI struct {
	chain  consensus.ChainHeaderReader
	eirene *Eirene
}

// NewGovernanceAPI는 새로운 거버넌스 API 인스턴스를 생성합니다.
func NewGovernanceAPI(chain consensus.ChainHeaderReader, eirene *Eirene) *GovernanceAPI {
	return &GovernanceAPI{chain: chain, eirene: eirene}
}

// GetProposal은 제안 정보를 반환합니다.
func (api *GovernanceAPI) GetProposal(proposalID uint64) (*Proposal, error) {
	return api.eirene.GetProposal(proposalID)
}

// GetProposals는 모든 제안 목록을 반환합니다.
func (api *GovernanceAPI) GetProposals() []*Proposal {
	return api.eirene.GetProposals()
}

// GetVotes는 제안에 대한 투표 목록을 반환합니다.
func (api *GovernanceAPI) GetVotes(proposalID uint64) ([]ProposalVote, error) {
	return api.eirene.GetVotes(proposalID)
}

// SubmitProposal은 새로운 거버넌스 제안을 제출합니다.
func (api *GovernanceAPI) SubmitProposal(
	proposer common.Address,
	title string,
	description string,
	proposalType uint8,
	parameters map[string]string,
	upgrade *UpgradeInfo,
	funding *FundingInfo,
	deposit *big.Int,
) (uint64, error) {
	// 현재 블록 번호 가져오기
	currentBlock := api.chain.CurrentHeader().Number.Uint64()

	return api.eirene.SubmitProposal(
		proposer,
		title,
		description,
		proposalType,
		parameters,
		upgrade,
		funding,
		deposit,
		currentBlock,
	)
}

// Vote는 거버넌스 제안에 투표합니다.
func (api *GovernanceAPI) Vote(
	proposalID uint64,
	voter common.Address,
	option uint8,
	weight *big.Int,
) error {
	// 현재 블록 번호 가져오기
	currentBlock := api.chain.CurrentHeader().Number.Uint64()

	return api.eirene.Vote(
		proposalID,
		voter,
		option,
		weight,
		currentBlock,
	)
}

// GetGovernanceParams는 현재 거버넌스 매개변수를 반환합니다.
func (api *GovernanceAPI) GetGovernanceParams() map[string]interface{} {
	return api.eirene.governance.getGovernanceParams()
}

// SlashingAPI는 Eirene 슬래싱 시스템의 공개 API를 제공합니다.
type SlashingAPI struct {
	chain  consensus.ChainHeaderReader
	eirene *Eirene
}

// NewSlashingAPI는 새로운 슬래싱 API 인스턴스를 생성합니다.
func NewSlashingAPI(chain consensus.ChainHeaderReader, eirene *Eirene) *SlashingAPI {
	return &SlashingAPI{chain: chain, eirene: eirene}
}

// GetValidatorSigningInfo는 검증자의 서명 정보를 반환합니다.
func (api *SlashingAPI) GetValidatorSigningInfo(validator common.Address) (*ValidatorSigningInfo, error) {
	info, exists := api.eirene.slashingState.ValidatorSigningInfo[validator]
	if !exists {
		return nil, errors.New("validator signing info not found")
	}
	return info, nil
}

// GetEvidences는 검증자의 슬래싱 증거 목록을 반환합니다.
func (api *SlashingAPI) GetEvidences(validator common.Address) []SlashingEvidence {
	return api.eirene.slashingState.getEvidences(validator)
}

// ReportDoubleSign은 이중 서명을 신고합니다.
func (api *SlashingAPI) ReportDoubleSign(reporter common.Address, evidence DoubleSignEvidence) error {
	return api.eirene.reportDoubleSign(reporter, evidence)
}

// Unjail은 검증자의 감금을 해제합니다.
func (api *SlashingAPI) Unjail(validator common.Address) error {
	return api.eirene.unjailValidator(validator)
}

// GetSlashingParams는 슬래싱 매개변수를 반환합니다.
func (api *SlashingAPI) GetSlashingParams() map[string]interface{} {
	return map[string]interface{}{
		"doubleSignSlashRatio":  api.eirene.slashingState.DoubleSignSlashRatio,
		"downtimeSlashRatio":    api.eirene.slashingState.DowntimeSlashRatio,
		"misbehaviorSlashRatio": api.eirene.slashingState.MisbehaviorSlashRatio,
		"doubleSignJailPeriod":  api.eirene.slashingState.DoubleSignJailPeriod,
		"downtimeJailPeriod":    api.eirene.slashingState.DowntimeJailPeriod,
		"misbehaviorJailPeriod": api.eirene.slashingState.MisbehaviorJailPeriod,
		"downtimeBlocksWindow":  api.eirene.slashingState.DowntimeBlocksWindow,
		"downtimeThreshold":     api.eirene.slashingState.DowntimeThreshold,
	}
}

// ValidatorAPI는 Eirene 검증자 시스템의 공개 API를 제공합니다.
type ValidatorAPI struct {
	chain  consensus.ChainHeaderReader
	eirene *Eirene
}

// NewValidatorAPI는 새로운 검증자 API 인스턴스를 생성합니다.
func NewValidatorAPI(chain consensus.ChainHeaderReader, eirene *Eirene) *ValidatorAPI {
	return &ValidatorAPI{chain: chain, eirene: eirene}
}

// GetValidators는 현재 활성 검증자 목록을 반환합니다.
func (api *ValidatorAPI) GetValidators() []*Validator {
	return api.eirene.validatorSet.getActiveValidators()
}

// GetValidator는 특정 검증자의 정보를 반환합니다.
func (api *ValidatorAPI) GetValidator(validator common.Address) (*Validator, error) {
	val := api.eirene.validatorSet.getValidatorByAddress(validator)
	if val == nil {
		return nil, errors.New("validator not found")
	}
	return val, nil
}

// GetDelegations는 특정 검증자에게 위임된 목록을 반환합니다.
func (api *ValidatorAPI) GetDelegations(validator common.Address) (map[common.Address]*ValidatorDelegation, error) {
	val := api.eirene.validatorSet.getValidatorByAddress(validator)
	if val == nil {
		return nil, errors.New("validator not found")
	}
	return val.Delegations, nil
}

// GetDelegation은 특정 위임자의 위임 정보를 반환합니다.
func (api *ValidatorAPI) GetDelegation(validator common.Address, delegator common.Address) (*ValidatorDelegation, error) {
	val := api.eirene.validatorSet.getValidatorByAddress(validator)
	if val == nil {
		return nil, errors.New("validator not found")
	}

	delegation, exists := val.Delegations[delegator]
	if !exists {
		return nil, errors.New("delegation not found")
	}

	return delegation, nil
}

// Delegate는 검증자에게 토큰을 위임합니다.
func (api *ValidatorAPI) Delegate(validator common.Address, delegator common.Address, amount *hexutil.Big) error {
	// 현재 블록 번호 가져오기
	currentBlock := api.chain.CurrentHeader().Number.Uint64()

	// 위임 추가
	return api.eirene.validatorSet.addDelegation(validator, delegator, (*big.Int)(amount), currentBlock)
}

// Undelegate는 검증자로부터 위임을 철회합니다.
func (api *ValidatorAPI) Undelegate(validator common.Address, delegator common.Address, amount *hexutil.Big) error {
	// 현재 블록 번호 가져오기
	currentBlock := api.chain.CurrentHeader().Number.Uint64()

	// 위임 제거
	return api.eirene.validatorSet.removeDelegation(validator, delegator, (*big.Int)(amount), currentBlock)
}

// GetValidatorStats는 검증자 통계 정보를 반환합니다.
func (api *ValidatorAPI) GetValidatorStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// 총 검증자 수
	stats["totalValidators"] = api.eirene.validatorSet.getValidatorCount()

	// 활성 검증자 수
	stats["activeValidators"] = api.eirene.validatorSet.getActiveValidatorCount()

	// 총 스테이킹 양
	stats["totalStake"] = api.eirene.validatorSet.getTotalStake()

	// 감금된 검증자 수
	jailedValidators := api.eirene.validatorSet.getValidatorsByStatus(ValidatorStatusJailed)
	stats["jailedValidators"] = len(jailedValidators)

	return stats
}

// RewardAPI는 Eirene 보상 시스템의 공개 API를 제공합니다.
type RewardAPI struct {
	chain  consensus.ChainHeaderReader
	eirene *Eirene
}

// NewRewardAPI는 새로운 보상 API 인스턴스를 생성합니다.
func NewRewardAPI(chain consensus.ChainHeaderReader, eirene *Eirene) *RewardAPI {
	return &RewardAPI{chain: chain, eirene: eirene}
}

// GetAccumulatedRewards는 주소의 누적 보상을 반환합니다.
func (api *RewardAPI) GetAccumulatedRewards(addr common.Address) *hexutil.Big {
	return (*hexutil.Big)(api.eirene.getAccumulatedRewards(addr))
}

// ClaimRewards는 누적된 보상을 청구합니다.
func (api *RewardAPI) ClaimRewards(claimer common.Address) (*hexutil.Big, error) {
	reward, err := api.eirene.claimRewards(claimer)
	if err != nil {
		return nil, err
	}
	return (*hexutil.Big)(reward), nil
}

// GetCommunityFund는 커뮤니티 기금 잔액을 반환합니다.
func (api *RewardAPI) GetCommunityFund() *hexutil.Big {
	return (*hexutil.Big)(api.eirene.getCommunityFund())
}

// WithdrawFromCommunityFund는 커뮤니티 기금에서 자금을 인출합니다.
func (api *RewardAPI) WithdrawFromCommunityFund(recipient common.Address, amount *hexutil.Big) error {
	return api.eirene.withdrawFromCommunityFund(recipient, (*big.Int)(amount))
}

// GetRewardStats는 보상 통계 정보를 반환합니다.
func (api *RewardAPI) GetRewardStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// 현재 블록 보상
	stats["currentBlockReward"] = (*hexutil.Big)(api.eirene.rewardState.CurrentBlockReward)

	// 마지막 보상 감소 블록
	stats["lastReductionBlock"] = api.eirene.rewardState.LastReductionBlock

	// 총 분배된 보상
	stats["totalDistributed"] = (*hexutil.Big)(api.eirene.rewardState.TotalDistributed)

	// 커뮤니티 기금 잔액
	stats["communityFund"] = (*hexutil.Big)(api.eirene.rewardState.CommunityFund)

	return stats
}

// IBCAPI는 Eirene IBC 시스템의 공개 API를 제공합니다.
type IBCAPI struct {
	chain  consensus.ChainHeaderReader
	eirene *Eirene
}

// NewIBCAPI는 새로운 IBC API 인스턴스를 생성합니다.
func NewIBCAPI(chain consensus.ChainHeaderReader, eirene *Eirene) *IBCAPI {
	return &IBCAPI{chain: chain, eirene: eirene}
}

// CreateClient는 새로운 IBC 클라이언트를 생성합니다.
func (api *IBCAPI) CreateClient(id string, clientType string, consensusState []byte, trustingPeriod uint64) error {
	_, err := api.eirene.ibcState.createClient(id, clientType, consensusState, trustingPeriod)
	if err != nil {
		return err
	}

	// IBC 상태 저장
	if err := api.eirene.ibcState.store(api.eirene.db); err != nil {
		return err
	}

	return nil
}

// UpdateClient는 IBC 클라이언트를 업데이트합니다.
func (api *IBCAPI) UpdateClient(id string, height uint64, consensusState []byte) error {
	err := api.eirene.ibcState.updateClient(id, height, consensusState)
	if err != nil {
		return err
	}

	// IBC 상태 저장
	if err := api.eirene.ibcState.store(api.eirene.db); err != nil {
		return err
	}

	return nil
}

// CreateConnection은 새로운 IBC 연결을 생성합니다.
func (api *IBCAPI) CreateConnection(id string, clientID string, counterpartyClientID string, counterpartyConnectionID string, version string) error {
	_, err := api.eirene.ibcState.createConnection(id, clientID, counterpartyClientID, counterpartyConnectionID, version)
	if err != nil {
		return err
	}

	// IBC 상태 저장
	if err := api.eirene.ibcState.store(api.eirene.db); err != nil {
		return err
	}

	return nil
}

// OpenConnection은 IBC 연결을 엽니다.
func (api *IBCAPI) OpenConnection(id string) error {
	err := api.eirene.ibcState.openConnection(id)
	if err != nil {
		return err
	}

	// IBC 상태 저장
	if err := api.eirene.ibcState.store(api.eirene.db); err != nil {
		return err
	}

	return nil
}

// CreateChannel은 새로운 IBC 채널을 생성합니다.
func (api *IBCAPI) CreateChannel(portID string, channelID string, connectionID string, counterpartyPortID string, counterpartyChannelID string, version string) error {
	_, err := api.eirene.ibcState.createChannel(portID, channelID, connectionID, counterpartyPortID, counterpartyChannelID, version)
	if err != nil {
		return err
	}

	// IBC 상태 저장
	if err := api.eirene.ibcState.store(api.eirene.db); err != nil {
		return err
	}

	return nil
}

// OpenChannel은 IBC 채널을 엽니다.
func (api *IBCAPI) OpenChannel(portID string, channelID string) error {
	err := api.eirene.ibcState.openChannel(portID, channelID)
	if err != nil {
		return err
	}

	// IBC 상태 저장
	if err := api.eirene.ibcState.store(api.eirene.db); err != nil {
		return err
	}

	return nil
}

// CloseChannel은 IBC 채널을 닫습니다.
func (api *IBCAPI) CloseChannel(portID string, channelID string) error {
	err := api.eirene.ibcState.closeChannel(portID, channelID)
	if err != nil {
		return err
	}

	// IBC 상태 저장
	if err := api.eirene.ibcState.store(api.eirene.db); err != nil {
		return err
	}

	return nil
}

// TransferToken은 IBC를 통해 토큰을 전송합니다.
func (api *IBCAPI) TransferToken(sourcePort string, sourceChannel string, token common.Address, amount *hexutil.Big, sender common.Address, receiver string) error {
	// 현재 블록 번호 가져오기
	currentBlock := api.chain.CurrentHeader().Number.Uint64()

	// 타임아웃 계산
	timeoutHeight := currentBlock + IBCDefaultTimeoutPeriod
	timeoutTimestamp := uint64(time.Now().Unix()) + IBCDefaultTimeoutPeriod*15 // 15초 블록 기준

	// 토큰 전송
	_, err := api.eirene.transferToken(
		sourcePort,
		sourceChannel,
		token,
		(*big.Int)(amount),
		sender,
		receiver,
		timeoutHeight,
		timeoutTimestamp,
	)

	return err
}

// GetClients는 IBC 클라이언트 목록을 반환합니다.
func (api *IBCAPI) GetClients() map[string]interface{} {
	result := make(map[string]interface{})

	for id, client := range api.eirene.ibcState.Clients {
		clientInfo := map[string]interface{}{
			"id":             id,
			"type":           client.Type,
			"state":          client.GetState(),
			"latestHeight":   client.GetLatestHeight(),
			"trustingPeriod": client.TrustingPeriod,
		}

		result[id] = clientInfo
	}

	return result
}

// GetConnections는 IBC 연결 목록을 반환합니다.
func (api *IBCAPI) GetConnections() map[string]interface{} {
	result := make(map[string]interface{})

	for id, connection := range api.eirene.ibcState.Connections {
		connectionInfo := map[string]interface{}{
			"id":                       id,
			"clientId":                 connection.ClientID,
			"counterpartyClientId":     connection.CounterpartyClientID,
			"counterpartyConnectionId": connection.CounterpartyConnectionID,
			"state":                    connection.State,
			"versions":                 connection.GetVersions(),
		}

		result[id] = connectionInfo
	}

	return result
}

// GetChannels는 IBC 채널 목록을 반환합니다.
func (api *IBCAPI) GetChannels() map[string]interface{} {
	result := make(map[string]interface{})

	for key, channel := range api.eirene.ibcState.Channels {
		channelInfo := map[string]interface{}{
			"portId":                channel.PortID,
			"channelId":             channel.ChannelID,
			"counterpartyPortId":    channel.CounterpartyPortID,
			"counterpartyChannelId": channel.CounterpartyChannelID,
			"state":                 channel.State,
			"version":               channel.Version,
			"connectionId":          channel.ConnectionID,
			"nextSequenceSend":      channel.GetNextSequenceSend(),
			"nextSequenceRecv":      channel.GetNextSequenceRecv(),
			"nextSequenceAck":       channel.GetNextSequenceAck(),
		}

		result[key] = channelInfo
	}

	return result
}

// GetPackets는 IBC 패킷 목록을 반환합니다.
func (api *IBCAPI) GetPackets() map[string]interface{} {
	result := make(map[string]interface{})

	for sequence, packet := range api.eirene.ibcState.Packets {
		packetInfo := map[string]interface{}{
			"sequence":         packet.Sequence,
			"sourcePort":       packet.SourcePort,
			"sourceChannel":    packet.SourceChannel,
			"destPort":         packet.GetDestPort(),
			"destChannel":      packet.GetDestChannel(),
			"timeoutHeight":    packet.TimeoutHeight,
			"timeoutTimestamp": packet.TimeoutTimestamp,
		}

		result[strconv.FormatUint(sequence, 10)] = packetInfo
	}

	return result
}

// GetIBCStats는 IBC 통계 정보를 반환합니다.
func (api *IBCAPI) GetIBCStats() map[string]interface{} {
	return map[string]interface{}{
		"totalPacketsSent":         api.eirene.ibcState.TotalPacketsSent,
		"totalPacketsReceived":     api.eirene.ibcState.TotalPacketsReceived,
		"totalPacketsAcknowledged": api.eirene.ibcState.TotalPacketsAcknowledged,
		"totalPacketsTimedOut":     api.eirene.ibcState.TotalPacketsTimedOut,
		"totalClients":             len(api.eirene.ibcState.Clients),
		"totalConnections":         len(api.eirene.ibcState.Connections),
		"totalChannels":            len(api.eirene.ibcState.Channels),
	}
}
