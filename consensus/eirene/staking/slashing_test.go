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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zenanetwork/go-zenanet/common"
)

// TestNewSlashingState는 새로운 슬래싱 상태 생성 기능을 테스트합니다.
func TestNewSlashingState(t *testing.T) {
	// 새로운 슬래싱 상태 생성
	slashingState := newSlashingState()
	
	// 필드 확인
	assert.NotNil(t, slashingState.Evidences, "증거 맵이 nil이 아니어야 함")
	assert.NotNil(t, slashingState.ValidatorSigningInfo, "검증자 서명 정보 맵이 nil이 아니어야 함")
	
	// 기본값 확인
	assert.Equal(t, 0, len(slashingState.Evidences), "증거 맵이 비어 있어야 함")
	assert.Equal(t, 0, len(slashingState.ValidatorSigningInfo), "검증자 서명 정보 맵이 비어 있어야 함")
	
	// 슬래싱 매개변수 확인
	assert.Equal(t, uint64(DoubleSignSlashRatio), slashingState.DoubleSignSlashRatio, "이중 서명 슬래싱 비율이 일치해야 함")
	assert.Equal(t, uint64(DowntimeSlashRatio), slashingState.DowntimeSlashRatio, "다운타임 슬래싱 비율이 일치해야 함")
	assert.Equal(t, uint64(MisbehaviorSlashRatio), slashingState.MisbehaviorSlashRatio, "기타 악의적 행동 슬래싱 비율이 일치해야 함")
	
	assert.Equal(t, uint64(DoubleSignJailPeriod), slashingState.DoubleSignJailPeriod, "이중 서명 감금 기간이 일치해야 함")
	assert.Equal(t, uint64(DowntimeJailPeriod), slashingState.DowntimeJailPeriod, "다운타임 감금 기간이 일치해야 함")
	assert.Equal(t, uint64(MisbehaviorJailPeriod), slashingState.MisbehaviorJailPeriod, "기타 악의적 행동 감금 기간이 일치해야 함")
	
	assert.Equal(t, uint64(DowntimeBlocksWindow), slashingState.DowntimeBlocksWindow, "다운타임 감지 윈도우가 일치해야 함")
	assert.Equal(t, uint64(DowntimeThreshold), slashingState.DowntimeThreshold, "다운타임 임계값이 일치해야 함")
}

// TestInitializeSigningInfo는 검증자 서명 정보 초기화 기능을 테스트합니다.
func TestInitializeSigningInfo(t *testing.T) {
	// 슬래싱 상태 생성
	slashingState := newSlashingState()
	
	// 테스트 검증자 주소
	validatorAddr := common.BytesToAddress([]byte{1})
	
	// 초기 검증자 서명 정보 확인
	_, exists := slashingState.ValidatorSigningInfo[validatorAddr]
	assert.False(t, exists, "초기에는 검증자 서명 정보가 존재하지 않아야 함")
	
	// 검증자 서명 정보 초기화
	startHeight := uint64(1000)
	slashingState.initializeSigningInfo(validatorAddr, startHeight)
	
	// 검증자 서명 정보 확인
	signingInfo, exists := slashingState.ValidatorSigningInfo[validatorAddr]
	assert.True(t, exists, "검증자 서명 정보가 존재해야 함")
	assert.Equal(t, validatorAddr, signingInfo.Address, "검증자 주소가 일치해야 함")
	assert.Equal(t, startHeight, signingInfo.StartHeight, "시작 블록 높이가 일치해야 함")
	assert.Equal(t, uint64(0), signingInfo.IndexOffset, "인덱스 오프셋이 0이어야 함")
	assert.Equal(t, uint64(0), signingInfo.MissedBlocksCounter, "놓친 블록 수가 0이어야 함")
	assert.Equal(t, uint64(0), signingInfo.JailedUntil, "감금 해제 블록 높이가 0이어야 함")
}

// TestHandleValidatorSignature는 검증자 서명 처리 기능을 테스트합니다.
func TestHandleValidatorSignature(t *testing.T) {
	// 슬래싱 상태 생성
	slashingState := newSlashingState()
	
	// 테스트 검증자 주소
	validatorAddr := common.BytesToAddress([]byte{1})
	
	// 검증자 서명 정보 초기화
	startHeight := uint64(1000)
	slashingState.initializeSigningInfo(validatorAddr, startHeight)
	
	// 초기 놓친 블록 수 확인
	signingInfo := slashingState.ValidatorSigningInfo[validatorAddr]
	assert.Equal(t, uint64(0), signingInfo.MissedBlocksCounter, "초기 놓친 블록 수가 0이어야 함")
	
	// 서명 실패 처리
	currentHeight := startHeight + 1
	slashingState.handleValidatorSignature(validatorAddr, currentHeight, false)
	
	// 업데이트된 놓친 블록 수 확인
	signingInfo = slashingState.ValidatorSigningInfo[validatorAddr]
	assert.Equal(t, uint64(1), signingInfo.MissedBlocksCounter, "놓친 블록 수가 1이어야 함")
	
	// 서명 성공 처리
	currentHeight++
	slashingState.handleValidatorSignature(validatorAddr, currentHeight, true)
	
	// 업데이트된 놓친 블록 수 확인 (서명 성공 시 놓친 블록 수는 변경되지 않음)
	signingInfo = slashingState.ValidatorSigningInfo[validatorAddr]
	assert.Equal(t, uint64(1), signingInfo.MissedBlocksCounter, "놓친 블록 수가 1이어야 함")
}

// TestAddEvidence는 슬래싱 증거 추가 기능을 테스트합니다.
func TestAddEvidence(t *testing.T) {
	// 슬래싱 상태 생성
	slashingState := newSlashingState()
	
	// 테스트 검증자 주소
	validatorAddr := common.BytesToAddress([]byte{1})
	
	// 초기 슬래싱 증거 확인
	evidences := slashingState.getEvidences(validatorAddr)
	assert.Equal(t, 0, len(evidences), "초기에는 슬래싱 증거가 없어야 함")
	
	// 슬래싱 증거 생성
	evidence := SlashingEvidence{
		Type:      SlashingTypeDowntime,
		Validator: validatorAddr,
		Height:    1000,
		Data:      nil,
	}
	
	// 슬래싱 증거 추가
	slashingState.addEvidence(evidence)
	
	// 슬래싱 증거 확인
	evidences = slashingState.getEvidences(validatorAddr)
	assert.Equal(t, 1, len(evidences), "슬래싱 증거 수가 1이어야 함")
	assert.Equal(t, SlashingTypeDowntime, evidences[0].Type, "슬래싱 증거 유형이 일치해야 함")
	assert.Equal(t, validatorAddr, evidences[0].Validator, "슬래싱 증거 검증자가 일치해야 함")
	assert.Equal(t, uint64(1000), evidences[0].Height, "슬래싱 증거 블록 높이가 일치해야 함")
} 