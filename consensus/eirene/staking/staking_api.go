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

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/params"
)

// StakingParams는 스테이킹 관련 매개변수를 정의합니다.
type StakingParams struct {
	MinStake            *big.Int // 최소 스테이킹 금액
	MinDelegation       *big.Int // 최소 위임 금액
	UnbondingTime       uint64   // 언본딩 기간 (초)
	MaxValidators       uint64   // 최대 검증자 수
	MaxEntries          uint64   // 최대 언본딩 항목 수
	HistoricalEntries   uint64   // 보관할 이력 항목 수
	BondDenom           string   // 본딩 토큰 단위
	PowerReduction      *big.Int // 파워 감소 계수
	MaxCommissionRate   *big.Int // 최대 커미션 비율
	MaxCommissionChange *big.Int // 최대 커미션 변경 비율
}

// API는 스테이킹 시스템의 RPC API를 구현합니다
type API struct {
	chain         consensus.ChainHeaderReader
	stakingManager *StakingManager
	stateAt       func(common.Hash) (*state.StateDB, error)
	currentBlock  func() *types.Block
	logger        log.Logger
}

// NewAPI는 새로운 스테이킹 API를 생성합니다
func NewAPI(chain consensus.ChainHeaderReader, stakingManager *StakingManager, stateAt func(common.Hash) (*state.StateDB, error), currentBlock func() *types.Block) *API {
	return &API{
		chain:         chain,
		stakingManager: stakingManager,
		stateAt:       stateAt,
		currentBlock:  currentBlock,
		logger:        log.New("module", "staking_api"),
	}
}

// ValidatorResponse는 검증자 정보 응답을 나타냅니다
type ValidatorResponse struct {
	Address     common.Address      `json:"address"`
	PubKey      string              `json:"pubKey"`
	VotingPower string              `json:"votingPower"`
	Commission  string              `json:"commission"`
	Description ValidatorDescription `json:"description"`
	Status      string              `json:"status"`
	SelfStake   string              `json:"selfStake"`
	Delegations []DelegationResponse `json:"delegations"`
}

// DelegationResponse는 위임 정보 응답을 나타냅니다
type DelegationResponse struct {
	DelegatorAddress common.Address `json:"delegatorAddress"`
	ValidatorAddress common.Address `json:"validatorAddress"`
	Shares           string         `json:"shares"`
}

// GetValidator는 지정된 주소의 검증자 정보를 반환합니다
func (api *API) GetValidator(address common.Address) (*ValidatorResponse, error) {
	// 현재 블록 헤더 가져오기
	header := api.chain.CurrentHeader()
	if header == nil {
		return nil, errors.New("current header not found")
	}

	// 스테이킹 매니저에서 검증자 가져오기
	validator, err := api.stakingManager.GetValidator(address)
	if err != nil {
		return nil, err
	}

	// 검증자 응답 생성
	delegations := make([]DelegationResponse, 0, len(validator.Delegations))
	for _, delegation := range validator.Delegations {
		delegations = append(delegations, DelegationResponse{
			DelegatorAddress: delegation.Delegator,
			ValidatorAddress: validator.Address,
			Shares:           delegation.Amount.String(),
		})
	}

	return &ValidatorResponse{
		Address:     validator.Address,
		PubKey:      common.Bytes2Hex(validator.PubKey),
		VotingPower: validator.VotingPower.String(),
		Commission:  validator.Commission.String(),
		Description: validator.Description,
		Status:      validator.Status.String(),
		SelfStake:   validator.SelfStake.String(),
		Delegations: delegations,
	}, nil
}

// GetValidators는 모든 검증자 정보를 반환합니다
func (api *API) GetValidators() ([]ValidatorResponse, error) {
	// 현재 블록 헤더 가져오기
	header := api.chain.CurrentHeader()
	if header == nil {
		return nil, errors.New("current header not found")
	}

	// 스테이킹 매니저에서 검증자 목록 가져오기
	validators := api.stakingManager.GetValidators()

	// 검증자 응답 목록 생성
	validatorResponses := make([]ValidatorResponse, 0, len(validators))
	for _, validator := range validators {
		delegations := make([]DelegationResponse, 0, len(validator.Delegations))
		for _, delegation := range validator.Delegations {
			delegations = append(delegations, DelegationResponse{
				DelegatorAddress: delegation.Delegator,
				ValidatorAddress: validator.Address,
				Shares:           delegation.Amount.String(),
			})
		}

		validatorResponses = append(validatorResponses, ValidatorResponse{
			Address:     validator.Address,
			PubKey:      common.Bytes2Hex(validator.PubKey),
			VotingPower: validator.VotingPower.String(),
			Commission:  validator.Commission.String(),
			Description: validator.Description,
			Status:      validator.Status.String(),
			SelfStake:   validator.SelfStake.String(),
			Delegations: delegations,
		})
	}

	return validatorResponses, nil
}

// GetStakingManager는 스테이킹 매니저를 반환합니다
func (api *API) GetStakingManager() *StakingManager {
	return api.stakingManager
}

// StakingManager는 스테이킹 관련 기능을 관리합니다
type StakingManager struct {
	adapter StakingAdapterInterface
	params  StakingParams
	logger  log.Logger
}

// NewStakingManager는 새로운 스테이킹 매니저를 생성합니다
func NewStakingManager(adapter StakingAdapterInterface, config *params.EireneConfig) *StakingManager {
	// 기본 스테이킹 매개변수 설정
	minStake := new(big.Int).Mul(big.NewInt(1000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	minDelegation := new(big.Int).Mul(big.NewInt(10), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	
	stakingParams := StakingParams{
		MinStake:            minStake,
		MinDelegation:       minDelegation,
		UnbondingTime:       86400 * 21, // 21일
		MaxValidators:       100,
		MaxEntries:          7,
		HistoricalEntries:   10000,
		BondDenom:           "zena",
		PowerReduction:      big.NewInt(1e6),
		MaxCommissionRate:   big.NewInt(100),
		MaxCommissionChange: big.NewInt(5),
	}

	return &StakingManager{
		adapter: adapter,
		params:  stakingParams,
		logger:  log.New("module", "staking_manager"),
	}
}

// GetValidator는 지정된 주소의 검증자를 반환합니다
func (m *StakingManager) GetValidator(address common.Address) (*Validator, error) {
	return m.adapter.GetValidator(address)
}

// GetValidators는 모든 검증자를 반환합니다
func (m *StakingManager) GetValidators() []*Validator {
	return m.adapter.GetValidators()
}

// GetParams는 스테이킹 매개변수를 반환합니다
func (m *StakingManager) GetParams() StakingParams {
	return m.params
}

// SetParams는 스테이킹 매개변수를 설정합니다
func (m *StakingManager) SetParams(params StakingParams) {
	m.params = params
} 