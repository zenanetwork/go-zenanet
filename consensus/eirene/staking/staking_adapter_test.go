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
	"math/big"
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/tracing"
	"github.com/zenanetwork/go-zenanet/crypto"
	"github.com/zenanetwork/go-zenanet/params"
)

// 테스트를 위한 헬퍼 함수들
func setupTestStakingAdapter(t *testing.T) (*StakingAdapter, *state.StateDB) {
	// 상태 DB 생성
	stateDB, err := state.New(common.Hash{}, state.NewDatabaseForTesting())
	require.NoError(t, err, "상태 DB 생성 실패")
	
	// 검증자 집합 생성
	validatorSet := NewValidatorSet()
	
	// 스테이킹 어댑터 생성
	config := &params.EireneConfig{
		SlashingThreshold: 100, // 최대 검증자 수로 사용
	}
	adapter := NewStakingAdapter(validatorSet, config)
	
	// 테스트 전에 기존 검증자 상태를 초기화하기 위한 모의 함수 추가
	adapter.validatorSet = &ValidatorSet{
		Validators: []*Validator{},
		BlockHeight: 0,
	}
	
	return adapter, stateDB
}

// 테스트용 키 생성 함수
func generateTestKey(t *testing.T) (common.Address, []byte) {
	privateKey, err := crypto.GenerateKey()
	require.NoError(t, err, "개인 키 생성 실패")
	
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	pubKey := crypto.FromECDSAPub(&privateKey.PublicKey)
	
	return address, pubKey
}

// TestStake는 스테이킹 기능을 테스트합니다.
func TestStake(t *testing.T) {
	adapter, stateDB := setupTestStakingAdapter(t)
	
	// 테스트용 키 생성
	address, pubKey := generateTestKey(t)
	
	// 계정에 충분한 잔액 부여
	amount := uint256.NewInt(2000000000000000000) // 2 ETH
	stateDB.AddBalance(address, amount, tracing.BalanceChangeUnspecified)
	
	// 검증자 설명 생성
	description := ValidatorDescription{
		Moniker:         "Test Validator",
		Identity:        "test-identity",
		Website:         "https://test.validator.com",
		SecurityContact: "security@test.validator.com",
		Details:         "Test validator for unit tests",
	}
	
	// 스테이킹 수행
	stakeAmount := big.NewInt(1500000000000000000) // 1.5 ETH
	commission := big.NewInt(5) // 5% 커미션
	
	err := adapter.Stake(stateDB, address, stakeAmount, pubKey, description, commission)
	require.NoError(t, err, "스테이킹 실패")
	
	// 검증자 확인
	validator, err := adapter.GetValidator(address)
	require.NoError(t, err, "검증자 조회 실패")
	
	assert.NotNil(t, validator, "검증자가 존재하지 않음")
	assert.Equal(t, address, validator.Address, "검증자 주소 불일치")
	assert.Equal(t, stakeAmount, validator.VotingPower, "스테이킹 양 불일치")
	assert.Equal(t, description.Moniker, validator.Description.Moniker, "검증자 이름 불일치")
	assert.Equal(t, commission, validator.Commission, "커미션 불일치")
}

// TestUnstake는 언스테이킹 기능을 테스트합니다.
func TestUnstake(t *testing.T) {
	adapter, stateDB := setupTestStakingAdapter(t)
	
	// 테스트용 키 생성
	address, pubKey := generateTestKey(t)
	
	// 계정에 충분한 잔액 부여
	amount := uint256.NewInt(2000000000000000000) // 2 ETH
	stateDB.AddBalance(address, amount, tracing.BalanceChangeUnspecified)
	
	// 검증자 설명 생성
	description := ValidatorDescription{
		Moniker:         "Test Validator",
		Identity:        "test-identity",
		Website:         "https://test.validator.com",
		SecurityContact: "security@test.validator.com",
		Details:         "Test validator for unit tests",
	}
	
	// 스테이킹 수행
	stakeAmount := big.NewInt(1500000000000000000) // 1.5 ETH
	commission := big.NewInt(5) // 5% 커미션
	
	err := adapter.Stake(stateDB, address, stakeAmount, pubKey, description, commission)
	require.NoError(t, err, "스테이킹 실패")
	
	// 언스테이킹 수행
	err = adapter.Unstake(stateDB, address)
	assert.NoError(t, err, "언스테이킹 실패")
	
	// 검증자 확인 (존재하지 않아야 함)
	validator, err := adapter.GetValidator(address)
	assert.Error(t, err, "검증자가 여전히 존재함")
	assert.Nil(t, validator, "검증자가 여전히 존재함")
	
	// 잔액 확인 (원래 잔액으로 복구되어야 함)
	balance := stateDB.GetBalance(address)
	expectedBalance := uint256.NewInt(2000000000000000000) // 2 ETH
	assert.Equal(t, expectedBalance.String(), balance.String(), "잔액 불일치")
}

// TestDelegate는 위임 기능을 테스트합니다.
func TestDelegate(t *testing.T) {
	adapter, stateDB := setupTestStakingAdapter(t)
	
	// 테스트용 키 생성
	validatorAddr, validatorPubKey := generateTestKey(t)
	delegatorAddr, _ := generateTestKey(t)
	
	// 계정에 충분한 잔액 부여
	validatorAmount := uint256.NewInt(2000000000000000000) // 2 ETH
	delegatorAmount := uint256.NewInt(2000000000000000000) // 2 ETH
	stateDB.AddBalance(validatorAddr, validatorAmount, tracing.BalanceChangeUnspecified)
	stateDB.AddBalance(delegatorAddr, delegatorAmount, tracing.BalanceChangeUnspecified)
	
	// 검증자 설명 생성
	description := ValidatorDescription{
		Moniker:         "Test Validator",
		Identity:        "test-identity",
		Website:         "https://test.validator.com",
		SecurityContact: "security@test.validator.com",
		Details:         "Test validator for unit tests",
	}
	
	// 스테이킹 수행
	stakeAmount := big.NewInt(1500000000000000000) // 1.5 ETH
	commission := big.NewInt(5) // 5% 커미션
	
	err := adapter.Stake(stateDB, validatorAddr, stakeAmount, validatorPubKey, description, commission)
	require.NoError(t, err, "스테이킹 실패")
	
	// 위임 수행
	delegationAmount := big.NewInt(1000000000000000000) // 1 ETH
	err = adapter.Delegate(stateDB, delegatorAddr, validatorAddr, delegationAmount)
	require.NoError(t, err, "위임 실패")
	
	// 위임 정보 확인
	delegation, err := adapter.GetDelegation(delegatorAddr, validatorAddr)
	require.NoError(t, err, "위임 정보 조회 실패")
	assert.NotNil(t, delegation, "위임 정보가 존재하지 않음")
	
	// 위임자 잔액 확인
	balance := stateDB.GetBalance(delegatorAddr)
	expectedBalance := uint256.NewInt(1000000000000000000) // 1 ETH
	assert.Equal(t, expectedBalance.String(), balance.String(), "위임자 잔액 불일치")
	
	// 검증자 정보 확인
	validator, err := adapter.GetValidator(validatorAddr)
	require.NoError(t, err, "검증자 조회 실패")
	expectedTokens := big.NewInt(2500000000000000000) // 2.5 ETH (1.5 + 1)
	assert.Equal(t, expectedTokens, validator.VotingPower, "검증자 토큰 불일치")
}

// TestUndelegate는 위임 철회 기능을 테스트합니다.
func TestUndelegate(t *testing.T) {
	adapter, stateDB := setupTestStakingAdapter(t)
	
	// 테스트용 키 생성
	validatorAddr, validatorPubKey := generateTestKey(t)
	delegatorAddr, _ := generateTestKey(t)
	
	// 계정에 충분한 잔액 부여
	validatorAmount := uint256.NewInt(2000000000000000000) // 2 ETH
	delegatorAmount := uint256.NewInt(2000000000000000000) // 2 ETH
	stateDB.AddBalance(validatorAddr, validatorAmount, tracing.BalanceChangeUnspecified)
	stateDB.AddBalance(delegatorAddr, delegatorAmount, tracing.BalanceChangeUnspecified)
	
	// 검증자 설명 생성
	description := ValidatorDescription{
		Moniker:         "Test Validator",
		Identity:        "test-identity",
		Website:         "https://test.validator.com",
		SecurityContact: "security@test.validator.com",
		Details:         "Test validator for unit tests",
	}
	
	// 스테이킹 수행
	stakeAmount := big.NewInt(1500000000000000000) // 1.5 ETH
	commission := big.NewInt(5) // 5% 커미션
	
	err := adapter.Stake(stateDB, validatorAddr, stakeAmount, validatorPubKey, description, commission)
	require.NoError(t, err, "스테이킹 실패")
	
	// 위임 수행
	delegationAmount := big.NewInt(1000000000000000000) // 1 ETH
	err = adapter.Delegate(stateDB, delegatorAddr, validatorAddr, delegationAmount)
	require.NoError(t, err, "위임 실패")
	
	// 위임 철회 수행
	undelegateAmount := big.NewInt(500000000000000000) // 0.5 ETH
	err = adapter.Undelegate(stateDB, delegatorAddr, validatorAddr, undelegateAmount)
	require.NoError(t, err, "위임 철회 실패")
	
	// 위임 정보 확인 - 변수 사용하지 않으므로 _ 처리
	_, err = adapter.GetDelegation(delegatorAddr, validatorAddr)
	require.NoError(t, err, "위임 정보 조회 실패")
	
	// 위임자 잔액 확인
	balance := stateDB.GetBalance(delegatorAddr)
	expectedBalance := uint256.NewInt(1500000000000000000) // 1.5 ETH
	assert.Equal(t, expectedBalance.String(), balance.String(), "위임자 잔액 불일치")
	
	// 검증자 정보 확인
	validator, err := adapter.GetValidator(validatorAddr)
	require.NoError(t, err, "검증자 조회 실패")
	expectedTokens := big.NewInt(2000000000000000000) // 2 ETH (1.5 + 0.5)
	assert.Equal(t, expectedTokens, validator.VotingPower, "검증자 토큰 불일치")
}

// TestRedelegate는 재위임 기능을 테스트합니다.
func TestRedelegate(t *testing.T) {
	adapter, stateDB := setupTestStakingAdapter(t)
	
	// 테스트용 키 생성
	validator1Addr, validator1PubKey := generateTestKey(t)
	validator2Addr, validator2PubKey := generateTestKey(t)
	delegatorAddr, _ := generateTestKey(t)
	
	// 계정에 충분한 잔액 부여
	validator1Amount := uint256.NewInt(2000000000000000000) // 2 ETH
	validator2Amount := uint256.NewInt(2000000000000000000) // 2 ETH
	delegatorAmount := uint256.NewInt(2000000000000000000) // 2 ETH
	stateDB.AddBalance(validator1Addr, validator1Amount, tracing.BalanceChangeUnspecified)
	stateDB.AddBalance(validator2Addr, validator2Amount, tracing.BalanceChangeUnspecified)
	stateDB.AddBalance(delegatorAddr, delegatorAmount, tracing.BalanceChangeUnspecified)
	
	// 검증자 설명 생성
	description1 := ValidatorDescription{
		Moniker:         "Test Validator 1",
		Identity:        "test-identity-1",
		Website:         "https://test1.validator.com",
		SecurityContact: "security@test1.validator.com",
		Details:         "Test validator 1 for unit tests",
	}
	
	description2 := ValidatorDescription{
		Moniker:         "Test Validator 2",
		Identity:        "test-identity-2",
		Website:         "https://test2.validator.com",
		SecurityContact: "security@test2.validator.com",
		Details:         "Test validator 2 for unit tests",
	}
	
	// 검증자1 스테이킹 수행
	stakeAmount1 := big.NewInt(1500000000000000000) // 1.5 ETH
	commission1 := big.NewInt(5) // 5% 커미션
	
	err := adapter.Stake(stateDB, validator1Addr, stakeAmount1, validator1PubKey, description1, commission1)
	require.NoError(t, err, "검증자1 스테이킹 실패")
	
	// 검증자2 스테이킹 수행
	stakeAmount2 := big.NewInt(1500000000000000000) // 1.5 ETH
	commission2 := big.NewInt(5) // 5% 커미션
	
	err = adapter.Stake(stateDB, validator2Addr, stakeAmount2, validator2PubKey, description2, commission2)
	require.NoError(t, err, "검증자2 스테이킹 실패")
	
	// 위임 수행 (검증자1에게)
	delegationAmount := big.NewInt(1000000000000000000) // 1 ETH
	err = adapter.Delegate(stateDB, delegatorAddr, validator1Addr, delegationAmount)
	require.NoError(t, err, "위임 실패")
	
	// 재위임 수행 (검증자1 -> 검증자2)
	redelegateAmount := big.NewInt(500000000000000000) // 0.5 ETH
	err = adapter.Redelegate(stateDB, delegatorAddr, validator1Addr, validator2Addr, redelegateAmount)
	require.NoError(t, err, "재위임 실패")
	
	// 위임 정보 확인 (검증자1) - 변수 사용하지 않으므로 _ 처리
	_, err = adapter.GetDelegation(delegatorAddr, validator1Addr)
	require.NoError(t, err, "위임 정보 조회 실패")
	
	// 위임 정보 확인 (검증자2) - 변수 사용하지 않으므로 _ 처리
	_, err = adapter.GetDelegation(delegatorAddr, validator2Addr)
	require.NoError(t, err, "위임 정보 조회 실패")
	
	// 검증자1 토큰 확인
	validator1, err := adapter.GetValidator(validator1Addr)
	assert.NoError(t, err, "검증자1 조회 실패")
	expectedTokens1 := big.NewInt(2000000000000000000) // 2 ETH (1.5 + 0.5)
	assert.Equal(t, expectedTokens1, validator1.VotingPower, "검증자1 토큰 불일치")
	
	// 검증자2 토큰 확인
	validator2, err := adapter.GetValidator(validator2Addr)
	assert.NoError(t, err, "검증자2 조회 실패")
	expectedTokens2 := big.NewInt(2000000000000000000) // 2 ETH (1.5 + 0.5)
	assert.Equal(t, expectedTokens2, validator2.VotingPower, "검증자2 토큰 불일치")
}

// TestGetValidators는 검증자 목록 조회 기능을 테스트합니다.
func TestGetValidators(t *testing.T) {
	adapter, stateDB := setupTestStakingAdapter(t)
	
	// 테스트용 키 생성
	validator1Addr, validator1PubKey := generateTestKey(t)
	validator2Addr, validator2PubKey := generateTestKey(t)
	validator3Addr, validator3PubKey := generateTestKey(t)
	
	// 계정에 충분한 잔액 부여
	validator1Amount := uint256.NewInt(2000000000000000000) // 2 ETH
	validator2Amount := uint256.NewInt(2000000000000000000) // 2 ETH
	validator3Amount := uint256.NewInt(2000000000000000000) // 2 ETH
	stateDB.AddBalance(validator1Addr, validator1Amount, tracing.BalanceChangeUnspecified)
	stateDB.AddBalance(validator2Addr, validator2Amount, tracing.BalanceChangeUnspecified)
	stateDB.AddBalance(validator3Addr, validator3Amount, tracing.BalanceChangeUnspecified)
	
	// 검증자 설명 생성
	description1 := ValidatorDescription{
		Moniker: "Test Validator 1",
	}
	description2 := ValidatorDescription{
		Moniker: "Test Validator 2",
	}
	description3 := ValidatorDescription{
		Moniker: "Test Validator 3",
	}
	
	// 검증자 스테이킹 수행
	amount1 := big.NewInt(1500000000000000000) // 1.5 ETH
	amount2 := big.NewInt(1700000000000000000) // 1.7 ETH
	amount3 := big.NewInt(1600000000000000000) // 1.6 ETH
	commission := big.NewInt(5) // 5% 커미션
	
	err := adapter.Stake(stateDB, validator1Addr, amount1, validator1PubKey, description1, commission)
	assert.NoError(t, err, "검증자1 스테이킹 실패")
	
	err = adapter.Stake(stateDB, validator2Addr, amount2, validator2PubKey, description2, commission)
	assert.NoError(t, err, "검증자2 스테이킹 실패")
	
	err = adapter.Stake(stateDB, validator3Addr, amount3, validator3PubKey, description3, commission)
	assert.NoError(t, err, "검증자3 스테이킹 실패")
	
	// 검증자 목록 조회
	validators := adapter.GetValidators()
	assert.Equal(t, 3, len(validators), "검증자 수 불일치")
	
	// 검증자 주소 확인
	validatorAddresses := make(map[common.Address]bool)
	for _, validator := range validators {
		validatorAddresses[validator.Address] = true
	}
	
	assert.True(t, validatorAddresses[validator1Addr], "검증자1이 목록에 없음")
	assert.True(t, validatorAddresses[validator2Addr], "검증자2가 목록에 없음")
	assert.True(t, validatorAddresses[validator3Addr], "검증자3이 목록에 없음")
} 