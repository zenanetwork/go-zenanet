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

package validator

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/crypto"
	"github.com/zenanetwork/go-zenanet/log"
)

// 테스트용 검증자 생성 함수
func createTestValidator(address string, power int64) *utils.BasicValidator {
	return &utils.BasicValidator{
		Address:     common.HexToAddress(address),
		VotingPower: big.NewInt(power),
		Status:      utils.ValidatorStatusBonded,
	}
}

// MockValidatorSet은 테스트를 위한 ValidatorSet 모의 구현체입니다.
type MockValidatorSet struct {
	validators map[common.Address]*utils.BasicValidator
	snapshots  map[int64][]utils.ValidatorInterface
}

// NewValidatorSet은 새로운 MockValidatorSet 인스턴스를 생성합니다.
func NewValidatorSet() *MockValidatorSet {
	return &MockValidatorSet{
		validators: make(map[common.Address]*utils.BasicValidator),
		snapshots:  make(map[int64][]utils.ValidatorInterface),
	}
}

// AddValidator는 검증자를 집합에 추가합니다.
func (m *MockValidatorSet) AddValidator(validator *utils.BasicValidator) {
	m.validators[validator.Address] = validator
}

// RemoveValidator는 검증자를 집합에서 제거합니다.
func (m *MockValidatorSet) RemoveValidator(address common.Address) {
	delete(m.validators, address)
}

// UpdateValidatorPower는 검증자의 투표력을 업데이트합니다.
func (m *MockValidatorSet) UpdateValidatorPower(address common.Address, power *big.Int) {
	if validator, ok := m.validators[address]; ok {
		validator.VotingPower = power
	}
}

// GetValidatorCount는 검증자 수를 반환합니다.
func (m *MockValidatorSet) GetValidatorCount() int {
	return len(m.validators)
}

// GetActiveValidatorCount는 활성 검증자 수를 반환합니다.
func (m *MockValidatorSet) GetActiveValidatorCount() int {
	count := 0
	for _, v := range m.validators {
		if v.Status == utils.ValidatorStatusBonded && v.VotingPower.Cmp(big.NewInt(0)) > 0 {
			count++
		}
	}
	return count
}

// GetTotalStake는 모든 검증자의 총 투표력을 반환합니다.
func (m *MockValidatorSet) GetTotalStake() *big.Int {
	total := big.NewInt(0)
	for _, v := range m.validators {
		if v.Status == utils.ValidatorStatusBonded {
			total = new(big.Int).Add(total, v.VotingPower)
		}
	}
	return total
}

// GetValidatorByAddress는 주소로 검증자를 찾아 반환합니다.
func (m *MockValidatorSet) GetValidatorByAddress(address common.Address) utils.ValidatorInterface {
	if validator, ok := m.validators[address]; ok {
		return validator
	}
	return nil
}

// Contains는 주소가 검증자 집합에 포함되어 있는지 확인합니다.
func (m *MockValidatorSet) Contains(address common.Address) bool {
	_, ok := m.validators[address]
	return ok
}

// GetActiveValidators는 활성 검증자 목록을 반환합니다.
func (m *MockValidatorSet) GetActiveValidators() []utils.ValidatorInterface {
	var activeValidators []utils.ValidatorInterface
	for _, v := range m.validators {
		if v.Status == utils.ValidatorStatusBonded && v.VotingPower.Cmp(big.NewInt(0)) > 0 {
			activeValidators = append(activeValidators, v)
		}
	}
	
	// 투표력 기준 내림차순 정렬
	for i := 0; i < len(activeValidators)-1; i++ {
		for j := i + 1; j < len(activeValidators); j++ {
			if activeValidators[i].GetVotingPower().Cmp(activeValidators[j].GetVotingPower()) < 0 {
				activeValidators[i], activeValidators[j] = activeValidators[j], activeValidators[i]
			}
		}
	}
	
	return activeValidators
}

// SaveSnapshot은 현재 검증자 집합의 스냅샷을 저장합니다.
func (m *MockValidatorSet) SaveSnapshot(height int64) {
	var validators []utils.ValidatorInterface
	for _, v := range m.validators {
		validators = append(validators, v)
	}
	m.snapshots[height] = validators
}

// GetValidatorsAtHeight는 특정 높이의 검증자 집합을 반환합니다.
func (m *MockValidatorSet) GetValidatorsAtHeight(height int64) ([]utils.ValidatorInterface, error) {
	if validators, ok := m.snapshots[height]; ok {
		return validators, nil
	}
	return nil, fmt.Errorf("no snapshot at height %d", height)
}

// TestValidatorSetInitialization은 검증자 집합 초기화를 테스트합니다.
func TestValidatorSetInitialization(t *testing.T) {
	// 로거 생성
	logger := log.New("module", "validator_manager_test")
	
	// 테스트 검증자 생성
	validators := []*utils.BasicValidator{
		createTestValidator("0x1111111111111111111111111111111111111111", 100),
		createTestValidator("0x2222222222222222222222222222222222222222", 200),
		createTestValidator("0x3333333333333333333333333333333333333333", 300),
	}
	
	// ValidatorSet 인스턴스 생성
	validatorSet := NewValidatorSet()
	
	// 검증자 추가
	for _, v := range validators {
		validatorSet.AddValidator(v)
	}
	
	logger.Debug("Validator set initialization test", "validatorCount", validatorSet.GetValidatorCount())
	
	// 결과 검증
	assert.Equal(t, 3, validatorSet.GetValidatorCount(), "Validator count should be 3")
	assert.Equal(t, 3, validatorSet.GetActiveValidatorCount(), "Active validator count should be 3")
	
	// 총 투표력 검증
	expectedTotalPower := big.NewInt(600) // 100 + 200 + 300
	assert.Equal(t, 0, expectedTotalPower.Cmp(validatorSet.GetTotalStake()), "Total voting power should be 600")
	
	// 개별 검증자 검증
	validator1 := validatorSet.GetValidatorByAddress(common.HexToAddress("0x1111111111111111111111111111111111111111"))
	assert.NotNil(t, validator1, "Validator 1 should exist")
	assert.Equal(t, big.NewInt(100), validator1.GetVotingPower(), "Validator 1 should have 100 voting power")
	
	validator2 := validatorSet.GetValidatorByAddress(common.HexToAddress("0x2222222222222222222222222222222222222222"))
	assert.NotNil(t, validator2, "Validator 2 should exist")
	assert.Equal(t, big.NewInt(200), validator2.GetVotingPower(), "Validator 2 should have 200 voting power")
	
	validator3 := validatorSet.GetValidatorByAddress(common.HexToAddress("0x3333333333333333333333333333333333333333"))
	assert.NotNil(t, validator3, "Validator 3 should exist")
	assert.Equal(t, big.NewInt(300), validator3.GetVotingPower(), "Validator 3 should have 300 voting power")
}

// TestValidatorAddition은 검증자 추가를 테스트합니다.
func TestValidatorAddition(t *testing.T) {
	// 로거 생성
	logger := log.New("module", "validator_manager_test")
	
	// ValidatorSet 인스턴스 생성
	validatorSet := NewValidatorSet()
	
	// 초기 검증자 추가
	validator1 := createTestValidator("0x1111111111111111111111111111111111111111", 100)
	validatorSet.AddValidator(validator1)
	
	// 초기 상태 확인
	assert.Equal(t, 1, validatorSet.GetValidatorCount(), "Initial validator count should be 1")
	
	// 새 검증자 추가
	validator2 := createTestValidator("0x2222222222222222222222222222222222222222", 200)
	validatorSet.AddValidator(validator2)
	
	// 결과 검증
	assert.Equal(t, 2, validatorSet.GetValidatorCount(), "Validator count should be 2 after addition")
	assert.Equal(t, 2, validatorSet.GetActiveValidatorCount(), "Active validator count should be 2")
	
	// 검증자 존재 확인
	assert.True(t, validatorSet.Contains(common.HexToAddress("0x1111111111111111111111111111111111111111")), "Validator 1 should exist")
	assert.True(t, validatorSet.Contains(common.HexToAddress("0x2222222222222222222222222222222222222222")), "Validator 2 should exist")
	
	// 총 투표력 확인
	expectedTotalPower := big.NewInt(300) // 100 + 200
	assert.Equal(t, 0, expectedTotalPower.Cmp(validatorSet.GetTotalStake()), "Total voting power should be 300")
	
	logger.Debug("Validator addition test completed", "validatorCount", validatorSet.GetValidatorCount())
}

// TestValidatorRemoval은 검증자 제거를 테스트합니다.
func TestValidatorRemoval(t *testing.T) {
	// 로거 생성
	logger := log.New("module", "validator_manager_test")
	
	// ValidatorSet 인스턴스 생성
	validatorSet := NewValidatorSet()
	
	// 검증자 추가
	validator1 := createTestValidator("0x1111111111111111111111111111111111111111", 100)
	validator2 := createTestValidator("0x2222222222222222222222222222222222222222", 200)
	validator3 := createTestValidator("0x3333333333333333333333333333333333333333", 300)
	
	validatorSet.AddValidator(validator1)
	validatorSet.AddValidator(validator2)
	validatorSet.AddValidator(validator3)
	
	// 초기 상태 확인
	assert.Equal(t, 3, validatorSet.GetValidatorCount(), "Initial validator count should be 3")
	
	// 검증자 제거
	validatorSet.RemoveValidator(common.HexToAddress("0x2222222222222222222222222222222222222222"))
	
	// 결과 검증
	assert.Equal(t, 2, validatorSet.GetValidatorCount(), "Validator count should be 2 after removal")
	assert.Equal(t, 2, validatorSet.GetActiveValidatorCount(), "Active validator count should be 2")
	
	// 검증자 존재 확인
	assert.True(t, validatorSet.Contains(common.HexToAddress("0x1111111111111111111111111111111111111111")), "Validator 1 should exist")
	assert.False(t, validatorSet.Contains(common.HexToAddress("0x2222222222222222222222222222222222222222")), "Validator 2 should not exist")
	assert.True(t, validatorSet.Contains(common.HexToAddress("0x3333333333333333333333333333333333333333")), "Validator 3 should exist")
	
	// 총 투표력 확인
	expectedTotalPower := big.NewInt(400) // 100 + 300
	assert.Equal(t, 0, expectedTotalPower.Cmp(validatorSet.GetTotalStake()), "Total voting power should be 400")
	
	logger.Debug("Validator removal test completed", "validatorCount", validatorSet.GetValidatorCount())
}

// TestValidatorPowerUpdate는 검증자 투표력 업데이트를 테스트합니다.
func TestValidatorPowerUpdate(t *testing.T) {
	// 로거 생성
	logger := log.New("module", "validator_manager_test")
	
	// ValidatorSet 인스턴스 생성
	validatorSet := NewValidatorSet()
	
	// 검증자 추가
	validator1 := createTestValidator("0x1111111111111111111111111111111111111111", 100)
	validator2 := createTestValidator("0x2222222222222222222222222222222222222222", 200)
	
	validatorSet.AddValidator(validator1)
	validatorSet.AddValidator(validator2)
	
	// 초기 상태 확인
	assert.Equal(t, 2, validatorSet.GetValidatorCount(), "Initial validator count should be 2")
	
	// 검증자 투표력 업데이트
	validatorSet.UpdateValidatorPower(common.HexToAddress("0x1111111111111111111111111111111111111111"), big.NewInt(150))
	
	// 결과 검증
	validator1Updated := validatorSet.GetValidatorByAddress(common.HexToAddress("0x1111111111111111111111111111111111111111"))
	assert.NotNil(t, validator1Updated, "Updated validator should exist")
	assert.Equal(t, big.NewInt(150), validator1Updated.GetVotingPower(), "Validator 1 should have 150 voting power")
	
	// 총 투표력 확인
	expectedTotalPower := big.NewInt(350) // 150 + 200
	assert.Equal(t, 0, expectedTotalPower.Cmp(validatorSet.GetTotalStake()), "Total voting power should be 350")
	
	logger.Debug("Validator power update test completed", "totalPower", validatorSet.GetTotalStake())
}

// TestValidatorSetSnapshot은 검증자 집합 스냅샷을 테스트합니다.
func TestValidatorSetSnapshot(t *testing.T) {
	// 로거 생성
	logger := log.New("module", "validator_manager_test")
	
	// ValidatorSet 인스턴스 생성
	validatorSet := NewValidatorSet()
	
	// 검증자 추가
	validator1 := createTestValidator("0x1111111111111111111111111111111111111111", 100)
	validator2 := createTestValidator("0x2222222222222222222222222222222222222222", 200)
	
	validatorSet.AddValidator(validator1)
	validatorSet.AddValidator(validator2)
	
	// 스냅샷 저장
	height1 := int64(100)
	validatorSet.SaveSnapshot(height1)
	
	// 검증자 추가 및 제거
	validator3 := createTestValidator("0x3333333333333333333333333333333333333333", 300)
	validatorSet.AddValidator(validator3)
	validatorSet.RemoveValidator(common.HexToAddress("0x1111111111111111111111111111111111111111"))
	
	// 새 스냅샷 저장
	height2 := int64(200)
	validatorSet.SaveSnapshot(height2)
	
	// 스냅샷 검증
	validators1, err := validatorSet.GetValidatorsAtHeight(height1)
	assert.NoError(t, err, "Should retrieve validators at height 100")
	assert.Equal(t, 2, len(validators1), "Should have 2 validators at height 100")
	
	validators2, err := validatorSet.GetValidatorsAtHeight(height2)
	assert.NoError(t, err, "Should retrieve validators at height 200")
	assert.Equal(t, 2, len(validators2), "Should have 2 validators at height 200")
	
	// 존재하지 않는 높이 검증
	_, err = validatorSet.GetValidatorsAtHeight(int64(150))
	assert.Error(t, err, "Should return error for non-existent height")
	
	logger.Debug("Validator set snapshot test completed")
}

// TestValidatorRanking은 검증자 순위를 테스트합니다.
func TestValidatorRanking(t *testing.T) {
	// 로거 생성
	logger := log.New("module", "validator_manager_test")
	
	// ValidatorSet 인스턴스 생성
	validatorSet := NewValidatorSet()
	
	// 검증자 추가 (투표력 순서대로 추가하지 않음)
	validator1 := createTestValidator("0x1111111111111111111111111111111111111111", 100)
	validator2 := createTestValidator("0x2222222222222222222222222222222222222222", 300)
	validator3 := createTestValidator("0x3333333333333333333333333333333333333333", 200)
	
	validatorSet.AddValidator(validator1)
	validatorSet.AddValidator(validator2)
	validatorSet.AddValidator(validator3)
	
	// 활성 검증자 가져오기 (투표력 순서대로 정렬되어야 함)
	activeValidators := validatorSet.GetActiveValidators()
	
	// 결과 검증
	assert.Equal(t, 3, len(activeValidators), "Should have 3 active validators")
	
	// 투표력 순서대로 정렬되었는지 확인
	assert.Equal(t, big.NewInt(300), activeValidators[0].GetVotingPower(), "First validator should have 300 voting power")
	assert.Equal(t, big.NewInt(200), activeValidators[1].GetVotingPower(), "Second validator should have 200 voting power")
	assert.Equal(t, big.NewInt(100), activeValidators[2].GetVotingPower(), "Third validator should have 100 voting power")
	
	logger.Debug("Validator ranking test completed")
}

// TestValidatorStatusChange는 검증자 상태 변경을 테스트합니다.
func TestValidatorStatusChange(t *testing.T) {
	// 로거 생성
	logger := log.New("module", "validator_manager_test")
	
	// ValidatorSet 인스턴스 생성
	validatorSet := NewValidatorSet()
	
	// 검증자 추가
	validator1 := createTestValidator("0x1111111111111111111111111111111111111111", 100)
	validator2 := createTestValidator("0x2222222222222222222222222222222222222222", 200)
	validator3 := createTestValidator("0x3333333333333333333333333333333333333333", 300)
	
	validatorSet.AddValidator(validator1)
	validatorSet.AddValidator(validator2)
	validatorSet.AddValidator(validator3)
	
	// 초기 상태 확인
	assert.Equal(t, 3, validatorSet.GetActiveValidatorCount(), "Initial active validator count should be 3")
	
	// 검증자 상태 변경
	validator2.Status = utils.ValidatorStatusUnbonding
	
	// 결과 검증
	assert.Equal(t, 2, validatorSet.GetActiveValidatorCount(), "Active validator count should be 2 after status change")
	
	// 총 투표력 확인 (비활성 검증자는 제외)
	expectedTotalPower := big.NewInt(400) // 100 + 300
	assert.Equal(t, 0, expectedTotalPower.Cmp(validatorSet.GetTotalStake()), "Total voting power should be 400")
	
	// 활성 검증자 목록 확인
	activeValidators := validatorSet.GetActiveValidators()
	assert.Equal(t, 2, len(activeValidators), "Should have 2 active validators")
	
	logger.Debug("Validator status change test completed", "activeCount", validatorSet.GetActiveValidatorCount())
}

// TestValidatorSignature는 검증자 서명을 테스트합니다.
func TestValidatorSignature(t *testing.T) {
	// 로거 생성
	logger := log.New("module", "validator_manager_test")
	
	// 개인키 생성
	privateKey, err := crypto.GenerateKey()
	assert.NoError(t, err, "Should generate private key")
	
	// 주소 계산
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	
	// 검증자 생성
	validator := &utils.BasicValidator{
		Address:     address,
		VotingPower: big.NewInt(100),
		Status:      utils.ValidatorStatusBonded,
	}
	
	// 메시지 해시 생성
	message := []byte("test message")
	messageHash := crypto.Keccak256(message)
	
	// 서명 생성
	signature, err := crypto.Sign(messageHash, privateKey)
	assert.NoError(t, err, "Should sign message")
	
	// 서명에서 주소 복구
	recoveredPubKey, err := crypto.Ecrecover(messageHash, signature)
	assert.NoError(t, err, "Should recover public key")
	
	pubKey, err := crypto.UnmarshalPubkey(recoveredPubKey)
	assert.NoError(t, err, "Should unmarshal public key")
	
	recoveredAddress := crypto.PubkeyToAddress(*pubKey)
	
	// 결과 검증
	assert.Equal(t, address, recoveredAddress, "Recovered address should match validator address")
	assert.Equal(t, validator.Address, recoveredAddress, "Recovered address should match validator address")
	
	logger.Debug("Validator signature test completed", "address", address.Hex())
} 