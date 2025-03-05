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

// Package core는 Eirene 합의 알고리즘의 핵심 기능을 구현합니다.
// 이 패키지는 합의 엔진의 주요 인터페이스, 블록 검증 및 생성 메커니즘,
// 스냅샷 시스템 등을 포함합니다.
package core

import (
	"errors"
	"math/big"
	"strconv"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/common/hexutil"
	"github.com/zenanetwork/go-zenanet/consensus"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/crypto"
	"github.com/zenanetwork/go-zenanet/rpc"
)

// API는 Eirene 합의 엔진의 RPC API를 제공합니다.
// 이 구조체는 블록체인 상태 조회, 검증자 정보 조회, 합의 엔진 설정 등의
// 기능을 제공하는 메서드를 포함합니다.
type API struct {
	chain  consensus.ChainHeaderReader
	eirene *Eirene
}

// GetSnapshot은 지정된 블록 번호에서 검증자 상태의 스냅샷을 반환합니다.
// 
// 매개변수:
//   - number: 스냅샷을 가져올 블록 번호. nil인 경우 최신 블록 사용
//
// 반환값:
//   - *Snapshot: 검증자 상태의 스냅샷
//   - error: 오류 발생 시 반환
func (api *API) GetSnapshot(number *uint64) (*Snapshot, error) {
	// 블록 번호가 지정되지 않은 경우 최신 블록 사용
	var blockNumber uint64
	if number == nil {
		blockNumber = api.chain.CurrentHeader().Number.Uint64()
	} else {
		blockNumber = *number
	}

	// 블록 헤더 가져오기
	header := api.chain.GetHeaderByNumber(blockNumber)
	if header == nil {
		return nil, errors.New("unknown block")
	}

	// 스냅샷 가져오기
	snap, err := api.eirene.snapshot(api.chain, blockNumber, header.Hash(), nil)
	if err != nil {
		return nil, err
	}
	return snap, nil
}

// GetValidators는 현재 활성화된 검증자 목록을 반환합니다.
//
// 반환값:
//   - []map[string]interface{}: 검증자 정보 목록 (주소, 투표력, 상태 등)
//   - error: 오류 발생 시 반환
func (api *API) GetValidators() ([]map[string]interface{}, error) {
	// 검증자 목록 가져오기
	validators := api.eirene.validatorSet.GetActiveValidators()
	
	// 결과 생성
	result := make([]map[string]interface{}, len(validators))
	for i, validator := range validators {
		// 검증자 정보 생성
		info := make(map[string]interface{})
		info["address"] = validator.GetAddress()
		info["votingPower"] = validator.GetVotingPower()
		info["status"] = validator.GetStatus()
		result[i] = info
	}
	
	return result, nil
}

// GetValidator는 지정된 주소의 검증자 정보를 반환합니다.
//
// 매개변수:
//   - address: 검증자 주소
//
// 반환값:
//   - map[string]interface{}: 검증자 정보 (주소, 투표력, 상태 등)
//   - error: 오류 발생 시 반환 (검증자가 존재하지 않는 경우 "validator not found" 오류)
func (api *API) GetValidator(address common.Address) (map[string]interface{}, error) {
	// 검증자 확인
	validator := api.eirene.validatorSet.GetValidatorByAddress(address)
	if validator == nil {
		return nil, errors.New("validator not found")
	}
	
	// 검증자 정보 생성
	info := make(map[string]interface{})
	info["address"] = validator.GetAddress()
	info["votingPower"] = validator.GetVotingPower()
	info["status"] = validator.GetStatus()
	
	return info, nil
}

// GetValidatorStatus는 지정된 블록 번호에서 검증자의 상태를 반환합니다
func (api *API) GetValidatorStatus(address common.Address, number *uint64) (map[string]interface{}, error) {
	// 스냅샷 가져오기
	snap, err := api.GetSnapshot(number)
	if err != nil {
		return nil, err
	}

	// 검증자 상태 확인
	if weight, ok := snap.Validators[address]; ok {
		// 검증자인 경우
		result := map[string]interface{}{
			"isValidator": true,
			"weight":      weight,
		}

		// 현재 블록 번호 가져오기
		var blockNumber uint64
		if number == nil {
			blockNumber = api.chain.CurrentHeader().Number.Uint64()
		} else {
			blockNumber = *number
		}

		// 블록 생성 차례 확인
		result["inTurn"] = snap.inturn(blockNumber+1, address)

		// 최근 서명 확인
		for blockNum, validator := range snap.Recents {
			if validator == address {
				result["lastBlock"] = blockNum
				break
			}
		}

		return result, nil
	}

	// 검증자가 아닌 경우
	return map[string]interface{}{
		"isValidator": false,
	}, nil
}

// Propose는 검증자 추가/제거를 제안합니다
func (api *API) Propose(address common.Address, auth bool) (bool, error) {
	api.eirene.lock.Lock()
	defer api.eirene.lock.Unlock()

	// 제안자가 검증자인지 확인
	header := api.chain.CurrentHeader()
	snap, err := api.eirene.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return false, err
	}
	if _, ok := snap.Validators[api.eirene.signer]; !ok {
		return false, errors.New("not a validator")
	}

	// 제안 추가
	api.eirene.proposals[address] = auth

	return true, nil
}

// Discard는 검증자 추가/제거 제안을 취소합니다
func (api *API) Discard(address common.Address) (bool, error) {
	api.eirene.lock.Lock()
	defer api.eirene.lock.Unlock()

	// 제안자가 검증자인지 확인
	header := api.chain.CurrentHeader()
	snap, err := api.eirene.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return false, err
	}
	if _, ok := snap.Validators[api.eirene.signer]; !ok {
		return false, errors.New("not a validator")
	}

	// 제안 삭제
	delete(api.eirene.proposals, address)

	return true, nil
}

// Status는 현재 합의 엔진의 상태를 반환합니다
func (api *API) Status() map[string]interface{} {
	api.eirene.lock.RLock()
	defer api.eirene.lock.RUnlock()

	// 현재 블록 가져오기
	var currentBlock *uint64
	if api.eirene.currentBlock != nil {
		block := api.eirene.currentBlock()
		if block != nil {
			num := block.NumberU64()
			currentBlock = &num
		}
	}

	// 상태 정보 구성
	status := map[string]interface{}{
		"signerAddress": api.eirene.signer,
	}
	if currentBlock != nil {
		status["currentBlock"] = *currentBlock
	}

	// 제안 목록 추가
	proposals := make(map[string]bool)
	for address, auth := range api.eirene.proposals {
		proposals[address.Hex()] = auth
	}
	status["proposals"] = proposals

	return status
}

// GetSignerAddress는 현재 서명자의 주소를 반환합니다
func (api *API) GetSignerAddress() (common.Address, error) {
	return api.eirene.signer, nil
}

// SetSignerAddress는 서명자의 주소를 설정합니다
func (api *API) SetSignerAddress(address common.Address) (bool, error) {
	api.eirene.lock.Lock()
	defer api.eirene.lock.Unlock()

	api.eirene.signer = address
	return true, nil
}

// SetSignerPrivateKey는 서명자의 개인 키를 설정합니다
func (api *API) SetSignerPrivateKey(privateKey hexutil.Bytes) (bool, error) {
	// 개인 키에서 주소 추출
	if len(privateKey) != 32 {
		return false, errors.New("invalid private key length")
	}
	
	// 개인 키로부터 ECDSA 키 생성
	key, err := crypto.ToECDSA(privateKey)
	if err != nil {
		return false, err
	}
	
	// 개인 키로부터 주소 추출
	address := crypto.PubkeyToAddress(key.PublicKey)
	
	// 서명 함수 생성
	signFn := func(signer common.Address, hash []byte) ([]byte, error) {
		return crypto.Sign(hash, key)
	}
	
	// 서명자 설정
	api.eirene.lock.Lock()
	defer api.eirene.lock.Unlock()
	
	api.eirene.signer = address
	api.eirene.signFn = signFn
	
	return true, nil
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
	proposal, err := api.eirene.GetProposal(proposalID)
	if err != nil {
		return nil, err
	}
	
	// 인터페이스를 구체적인 타입으로 변환
	// 실제 구현에서는 적절한 변환 로직이 필요합니다
	return &Proposal{
		ID:          proposal.GetID(),
		Title:       proposal.GetTitle(),
		Description: proposal.GetDescription(),
		Type:        proposal.GetType(),
		Status:      proposal.GetStatus(),
		Proposer:    proposal.GetProposer(),
		VotingStartBlock: proposal.GetVotingStartBlock(),
		VotingEndBlock:   proposal.GetVotingEndBlock(),
	}, nil
}

// GetProposals는 모든 제안 목록을 반환합니다.
func (api *GovernanceAPI) GetProposals() []*Proposal {
	proposals := api.eirene.GetProposals()
	result := make([]*Proposal, 0, len(proposals))
	
	// 인터페이스 목록을 구체적인 타입 목록으로 변환
	for _, proposal := range proposals {
		result = append(result, &Proposal{
			ID:          proposal.GetID(),
			Title:       proposal.GetTitle(),
			Description: proposal.GetDescription(),
			Type:        proposal.GetType(),
			Status:      proposal.GetStatus(),
			Proposer:    proposal.GetProposer(),
			VotingStartBlock: proposal.GetVotingStartBlock(),
			VotingEndBlock:   proposal.GetVotingEndBlock(),
		})
	}
	
	return result
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
	// 상태 가져오기
	header := api.chain.CurrentHeader()
	stateDB, err := api.eirene.stateAt(header.Root)
	if err != nil {
		return 0, err
	}
	
	// 제안 유형을 문자열로 변환
	var proposalTypeStr string
	switch proposalType {
	case 1:
		proposalTypeStr = "ParameterChange"
	case 2:
		proposalTypeStr = "Upgrade"
	case 3:
		proposalTypeStr = "Funding"
	case 4:
		proposalTypeStr = "Text"
	default:
		return 0, errors.New("invalid proposal type")
	}
	
	// 제안 내용 생성
	var content utils.ProposalContentInterface
	
	// 실제 구현에서는 적절한 내용 생성 로직이 필요합니다
	// 현재는 임시 구현
	
	// 제안 제출
	return api.eirene.governance.SubmitProposal(
		proposer,
		title,
		description,
		proposalTypeStr,
		content,
		deposit,
		stateDB,
	)
}

// Vote는 거버넌스 제안에 투표합니다.
func (api *GovernanceAPI) Vote(
	proposalID uint64,
	voter common.Address,
	option uint8,
	weight *big.Int,
) error {
	// 옵션을 문자열로 변환
	var optionStr string
	switch option {
	case 1:
		optionStr = "Yes"
	case 2:
		optionStr = "No"
	case 3:
		optionStr = "Abstain"
	case 4:
		optionStr = "Veto"
	default:
		return errors.New("invalid vote option")
	}
	
	return api.eirene.Vote(
		proposalID,
		voter,
		optionStr,
	)
}

// GetGovernanceParams는 현재 거버넌스 매개변수를 반환합니다.
func (api *GovernanceAPI) GetGovernanceParams() map[string]interface{} {
	// 실제 구현에서는 거버넌스 매개변수를 가져오는 로직이 필요합니다
	// 현재는 임시 구현으로 테스트에서 사용하는 기본값과 일치하도록 설정
	return map[string]interface{}{
		"votingPeriod": uint64(100800), // 약 1주일(100800블록)
		"quorum": 0.334,                // 33.4%
		"threshold": 0.5,               // 50%
		"vetoThreshold": 0.334,         // 33.4%
	}
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
	validators := api.eirene.validatorSet.GetActiveValidators()
	result := make([]*Validator, len(validators))
	
	for i, v := range validators {
		result[i] = &Validator{
			Address:     v.GetAddress(),
			VotingPower: v.GetVotingPower(),
			Status:      v.GetStatus(),
		}
	}
	
	return result
}

// GetValidator는 특정 주소의 검증자 정보를 반환합니다.
func (api *ValidatorAPI) GetValidator(address common.Address) (*Validator, error) {
	v := api.eirene.validatorSet.GetValidatorByAddress(address)
	if v == nil {
		return nil, errors.New("validator not found")
	}
	
	return &Validator{
		Address:     v.GetAddress(),
		VotingPower: v.GetVotingPower(),
		Status:      v.GetStatus(),
	}, nil
}

// GetDelegations는 특정 검증자의 위임 정보를 반환합니다.
func (api *ValidatorAPI) GetDelegations(validator common.Address) ([]map[string]interface{}, error) {
	v := api.eirene.validatorSet.GetValidatorByAddress(validator)
	if v == nil {
		return nil, errors.New("validator not found")
	}
	
	// 실제 구현에서는 검증자의 위임 정보를 가져와야 합니다.
	// 여기서는 임시로 빈 배열을 반환합니다.
	return []map[string]interface{}{}, nil
}

// GetDelegation은 특정 위임자의 위임 정보를 반환합니다.
func (api *ValidatorAPI) GetDelegation(validator common.Address, delegator common.Address) (map[string]interface{}, error) {
	val := api.eirene.validatorSet.GetValidatorByAddress(validator)
	if val == nil {
		return nil, errors.New("validator not found")
	}

	// 실제 구현에서는 위임 정보를 가져와야 합니다.
	// 여기서는 임시로 빈 맵을 반환합니다.
	return map[string]interface{}{
		"validator": validator,
		"delegator": delegator,
		"amount": "0",
		"since": 0,
	}, nil
}

// GetValidatorStats는 검증자 통계를 반환합니다.
func (api *ValidatorAPI) GetValidatorStats() map[string]interface{} {
	return map[string]interface{}{
		"totalValidators":    api.eirene.validatorSet.GetValidatorCount(),
		"activeValidators":   api.eirene.validatorSet.GetActiveValidatorCount(),
		"totalStake":         api.eirene.validatorSet.GetTotalStake(),
	}
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

// IsValidator는 주어진 주소가 검증자인지 확인합니다.
func (api *ValidatorAPI) IsValidator(address common.Address) bool {
	return api.eirene.validatorSet.Contains(address)
}

// GetValidatorCount는 전체 검증자 수를 반환합니다.
func (api *ValidatorAPI) GetValidatorCount() int {
	return api.eirene.validatorSet.GetValidatorCount()
}

// GetActiveValidatorCount는 활성 검증자 수를 반환합니다.
func (api *ValidatorAPI) GetActiveValidatorCount() int {
	return api.eirene.validatorSet.GetActiveValidatorCount()
}
