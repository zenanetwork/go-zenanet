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
	"bytes"
	"errors"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/core"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/rlp"
)

// SlashingAdapter는 슬래싱 관리를 위한 어댑터입니다.
type SlashingAdapter struct {
	eirene        *core.Eirene // Eirene 합의 엔진 인스턴스
	logger        log.Logger
	slashingState *SlashingState
	lock          sync.RWMutex
}

// NewSlashingAdapter는 새로운 SlashingAdapter 인스턴스를 생성합니다.
func NewSlashingAdapter(eirene *core.Eirene) *SlashingAdapter {
	return &SlashingAdapter{
		eirene:        eirene,
		logger:        log.New("module", "slashing"),
		slashingState: newSlashingState(),
	}
}

// 슬래싱 유형 상수
const (
	SlashingTypeDoubleSign  = "double_sign"
	SlashingTypeDowntime    = "downtime"
	SlashingTypeMisbehavior = "misbehavior"
)

// 슬래싱 매개변수 상수
const (
	// 슬래싱 비율 (%)
	DoubleSignSlashRatio  = 5 // 이중 서명 슬래싱 비율 (5%)
	DowntimeSlashRatio    = 1 // 다운타임 슬래싱 비율 (1%)
	MisbehaviorSlashRatio = 3 // 기타 악의적 행동 슬래싱 비율 (3%)

	// 감금 기간 (블록 수)
	DoubleSignJailPeriod  = 20000 // 이중 서명 감금 기간 (약 3일, 15초 블록 기준)
	DowntimeJailPeriod    = 10000 // 다운타임 감금 기간 (약 1.5일, 15초 블록 기준)
	MisbehaviorJailPeriod = 15000 // 기타 악의적 행동 감금 기간 (약 2.5일, 15초 블록 기준)

	// 다운타임 감지 매개변수
	DowntimeBlocksWindow = 100 // 다운타임 감지 윈도우 (블록 수)
	DowntimeThreshold    = 50  // 다운타임 임계값 (%)
)

// SlashingEvidence는 슬래싱 증거를 나타냅니다.
type SlashingEvidence struct {
	Type      string         // 슬래싱 유형
	Validator common.Address // 검증자 주소
	Height    uint64         // 증거가 발생한 블록 높이
	Time      time.Time      // 증거가 발생한 시간
	Data      []byte         // 추가 데이터
}

// DoubleSignEvidence는 이중 서명 증거를 나타냅니다.
type DoubleSignEvidence struct {
	Height     uint64         // 블록 높이
	Validator  common.Address // 검증자 주소
	VoteA      []byte         // 첫 번째 투표
	VoteB      []byte         // 두 번째 투표
	Timestamp  time.Time      // 증거가 발생한 시간
}

// DowntimeEvidence는 다운타임 증거를 나타냅니다.
type DowntimeEvidence struct {
	Validator     common.Address // 검증자 주소
	MissedBlocks  []uint64       // 놓친 블록 목록
	StartHeight   uint64         // 시작 블록 높이
	EndHeight     uint64         // 종료 블록 높이
	MissedPercent float64        // 놓친 블록 비율
}

// MisbehaviorEvidence는 기타 악의적 행동 증거를 나타냅니다.
type MisbehaviorEvidence struct {
	Validator common.Address // 검증자 주소
	Height    uint64         // 블록 높이
	Type      string         // 악의적 행동 유형
	Data      []byte         // 추가 데이터
}

// SlashingState는 슬래싱 상태를 나타냅니다.
type SlashingState struct {
	// 슬래싱 증거 목록
	Evidences map[common.Address][]SlashingEvidence `json:"evidences"` // 검증자별 슬래싱 증거 목록

	// 다운타임 추적
	ValidatorSigningInfo map[common.Address]*ValidatorSigningInfo `json:"validatorSigningInfo"` // 검증자별 서명 정보

	// 슬래싱 매개변수
	DoubleSignSlashRatio  uint64 `json:"doubleSignSlashRatio"`  // 이중 서명 슬래싱 비율 (%)
	DowntimeSlashRatio    uint64 `json:"downtimeSlashRatio"`    // 다운타임 슬래싱 비율 (%)
	MisbehaviorSlashRatio uint64 `json:"misbehaviorSlashRatio"` // 기타 악의적 행동 슬래싱 비율 (%)

	DoubleSignJailPeriod  uint64 `json:"doubleSignJailPeriod"`  // 이중 서명 감금 기간 (블록 수)
	DowntimeJailPeriod    uint64 `json:"downtimeJailPeriod"`    // 다운타임 감금 기간 (블록 수)
	MisbehaviorJailPeriod uint64 `json:"misbehaviorJailPeriod"` // 기타 악의적 행동 감금 기간 (블록 수)

	DowntimeBlocksWindow uint64 `json:"downtimeBlocksWindow"` // 다운타임 감지 윈도우 (블록 수)
	DowntimeThreshold    uint64 `json:"downtimeThreshold"`    // 다운타임 임계값 (%)
}

// ValidatorSigningInfo는 검증자의 서명 정보를 나타냅니다.
type ValidatorSigningInfo struct {
	Address             common.Address `json:"address"`             // 검증자 주소
	StartHeight         uint64         `json:"startHeight"`         // 시작 블록 높이
	IndexOffset         uint64         `json:"indexOffset"`         // 현재 윈도우 내 오프셋
	MissedBlocksCounter uint64         `json:"missedBlocksCounter"` // 놓친 블록 수
	JailedUntil         uint64         `json:"jailedUntil"`         // 감금 해제 블록 높이
}

// newSlashingState는 새로운 슬래싱 상태를 생성합니다.
func newSlashingState() *SlashingState {
	return &SlashingState{
		Evidences:             make(map[common.Address][]SlashingEvidence),
		ValidatorSigningInfo:  make(map[common.Address]*ValidatorSigningInfo),
		DoubleSignSlashRatio:  DoubleSignSlashRatio,
		DowntimeSlashRatio:    DowntimeSlashRatio,
		MisbehaviorSlashRatio: MisbehaviorSlashRatio,
		DoubleSignJailPeriod:  DoubleSignJailPeriod,
		DowntimeJailPeriod:    DowntimeJailPeriod,
		MisbehaviorJailPeriod: MisbehaviorJailPeriod,
		DowntimeBlocksWindow:  DowntimeBlocksWindow,
		DowntimeThreshold:     DowntimeThreshold,
	}
}

// loadSlashingState는 데이터베이스에서 슬래싱 상태를 로드합니다.
func loadSlashingState(db ethdb.Database) (*SlashingState, error) {
	data, err := db.Get([]byte("eirene-slashing"))
	if err != nil {
		// 데이터가 없으면 새로운 상태 생성
		return newSlashingState(), nil
	}

	var state SlashingState
	if err := rlp.DecodeBytes(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// store는 슬래싱 상태를 데이터베이스에 저장합니다.
func (ss *SlashingState) store(db ethdb.Database) error {
	data, err := rlp.EncodeToBytes(ss)
	if err != nil {
		return err
	}

	return db.Put([]byte("eirene-slashing"), data)
}

// addEvidence는 슬래싱 증거를 추가합니다.
func (ss *SlashingState) addEvidence(evidence SlashingEvidence) {
	if _, exists := ss.Evidences[evidence.Validator]; !exists {
		ss.Evidences[evidence.Validator] = make([]SlashingEvidence, 0)
	}
	ss.Evidences[evidence.Validator] = append(ss.Evidences[evidence.Validator], evidence)
}

// getEvidences는 검증자의 슬래싱 증거 목록을 반환합니다.
func (ss *SlashingState) getEvidences(validator common.Address) []SlashingEvidence {
	if evidences, exists := ss.Evidences[validator]; exists {
		return evidences
	}
	return []SlashingEvidence{}
}

// initializeSigningInfo는 검증자의 서명 정보를 초기화합니다.
func (ss *SlashingState) initializeSigningInfo(validator common.Address, height uint64) {
	ss.ValidatorSigningInfo[validator] = &ValidatorSigningInfo{
		Address:             validator,
		StartHeight:         height,
		IndexOffset:         0,
		MissedBlocksCounter: 0,
		JailedUntil:         0,
	}
}

// handleValidatorSignature는 검증자의 서명 여부를 처리합니다.
func (ss *SlashingState) handleValidatorSignature(validator common.Address, height uint64, signed bool) {
	// 서명 정보가 없으면 초기화
	if _, exists := ss.ValidatorSigningInfo[validator]; !exists {
		ss.initializeSigningInfo(validator, height)
	}

	info := ss.ValidatorSigningInfo[validator]

	// 윈도우 내 인덱스 계산
	index := height % ss.DowntimeBlocksWindow

	// 서명하지 않은 경우 카운터 증가
	if !signed {
		info.MissedBlocksCounter++
	}

	// 윈도우가 완료되면 다운타임 검사
	if index == 0 && height > ss.DowntimeBlocksWindow {
		// 놓친 블록 비율 계산
		missedRatio := float64(info.MissedBlocksCounter) / float64(ss.DowntimeBlocksWindow)
		missedPercent := uint64(missedRatio * 100)

		// 임계값을 초과하면 다운타임 증거 생성
		if missedPercent >= ss.DowntimeThreshold {
			evidence := SlashingEvidence{
				Type:      SlashingTypeDowntime,
				Validator: validator,
				Height:    height,
				Time:      time.Now(),
				Data:      nil, // 다운타임은 별도의 증거 데이터가 필요 없음
			}

			ss.addEvidence(evidence)

			// 카운터 리셋
			info.MissedBlocksCounter = 0
		}
	}
}

// handleDoubleSign은 이중 서명을 처리합니다.
func (ss *SlashingState) handleDoubleSign(validator common.Address, height uint64, evidence []byte) {
	// 이중 서명 증거 생성
	slashingEvidence := SlashingEvidence{
		Type:      SlashingTypeDoubleSign,
		Validator: validator,
		Height:    height,
		Time:      time.Now(),
		Data:      evidence,
	}

	ss.addEvidence(slashingEvidence)
}

// getSlashRatio는 슬래싱 유형에 따른 슬래싱 비율을 반환합니다.
func (ss *SlashingState) getSlashRatio(slashType string) uint64 {
	switch slashType {
	case SlashingTypeDoubleSign:
		return ss.DoubleSignSlashRatio
	case SlashingTypeDowntime:
		return ss.DowntimeSlashRatio
	case SlashingTypeMisbehavior:
		return ss.MisbehaviorSlashRatio
	default:
		return 0
	}
}

// getJailPeriod는 슬래싱 유형에 따른 감금 기간을 반환합니다.
func (ss *SlashingState) getJailPeriod(slashType string) uint64 {
	switch slashType {
	case SlashingTypeDoubleSign:
		return ss.DoubleSignJailPeriod
	case SlashingTypeDowntime:
		return ss.DowntimeJailPeriod
	case SlashingTypeMisbehavior:
		return ss.MisbehaviorJailPeriod
	default:
		return 0
	}
}

// ProcessSlashing은 슬래싱 처리를 수행합니다.
func (a *SlashingAdapter) ProcessSlashing(validatorSet *ValidatorSet, currentBlock uint64) {
	// 모든 검증자에 대해 처리
	for addr, validator := range validatorSet.Validators {
		// 이미 감금된 검증자는 건너뜀
		if validator.Status == ValidatorStatusJailed {
			continue
		}

		// 검증자의 슬래싱 증거 확인
		evidences := a.slashingState.getEvidences(addr)

		// 처리되지 않은 증거가 있는지 확인
		for _, evidence := range evidences {
			// 이미 처리된 증거는 건너뜀
			if evidence.Height >= validator.LastSlashedBlock {
				// 슬래싱 비율 및 감금 기간 계산
				slashRatio := a.slashingState.getSlashRatio(evidence.Type)
				jailPeriod := a.slashingState.getJailPeriod(evidence.Type)

				// 슬래싱 실행
				validatorSet.slashValidator(addr, slashRatio)

				// 감금 실행
				jailUntil := currentBlock + jailPeriod
				validatorSet.jailValidator(addr, jailUntil)

				// 서명 정보 업데이트
				if info, exists := a.slashingState.ValidatorSigningInfo[addr]; exists {
					info.JailedUntil = jailUntil
				}

				log.Info("Validator slashed and jailed",
					"address", addr,
					"type", evidence.Type,
					"ratio", slashRatio,
					"jailUntil", jailUntil)

				// 마지막 슬래싱 블록 업데이트
				validator.LastSlashedBlock = currentBlock

				// 하나의 증거만 처리하고 종료 (여러 증거가 있어도 한 번에 하나만 처리)
				break
			}
		}
	}
}

// UpdateSigningInfo는 서명 정보를 업데이트합니다.
func (a *SlashingAdapter) UpdateSigningInfo(header *types.Header, signers []common.Address) {
	height := header.Number.Uint64()

	// 활성 검증자 목록 가져오기
	// 실제 구현에서는 a.eirene.GetValidatorSet().GetActiveValidators() 형태로 호출
	// 여기서는 간단히 구현
	activeValidators := []*Validator{}

	// 모든 활성 검증자에 대해 서명 여부 확인
	for _, validator := range activeValidators {
		signed := false
		for _, signer := range signers {
			if signer == validator.Address {
				signed = true
				break
			}
		}

		// 서명 정보 업데이트
		a.slashingState.handleValidatorSignature(validator.Address, height, signed)
	}
}

// VerifyDoubleSign은 이중 서명 증거를 검증합니다.
func (a *SlashingAdapter) VerifyDoubleSign(evidence DoubleSignEvidence) bool {
	// 두 서명이 동일한 블록 높이와 라운드에 대한 것인지 확인
	// 두 서명이 다른지 확인
	// 두 서명이 모두 유효한지 확인
	// 실제 구현에서는 더 복잡한 검증 로직이 필요함

	return evidence.VoteA != nil &&
		evidence.VoteB != nil &&
		len(evidence.VoteA) > 0 &&
		len(evidence.VoteB) > 0 &&
		!bytes.Equal(evidence.VoteA, evidence.VoteB)
}

// ReportDoubleSign은 이중 서명을 신고합니다.
func (a *SlashingAdapter) ReportDoubleSign(reporter common.Address, evidence DoubleSignEvidence) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	// 증거 검증
	if !a.VerifyDoubleSign(evidence) {
		return errors.New("invalid double sign evidence")
	}

	// 슬래싱 증거 생성
	slashingEvidence := SlashingEvidence{
		Type:      SlashingTypeDoubleSign,
		Validator: evidence.Validator,
		Height:    evidence.Height,
		Time:      time.Now(),
		Data:      append(evidence.VoteA, evidence.VoteB...),
	}

	// 증거 추가
	a.slashingState.addEvidence(slashingEvidence)

	// 데이터베이스에 저장
	// 실제 구현에서는 a.eirene.GetDB() 형태로 호출
	// 여기서는 간단히 구현
	// if err := a.slashingState.store(a.eirene.GetDB()); err != nil {
	// 	return err
	// }

	return nil
}

// UnjailValidator는 감금된 검증자를 해제합니다.
func (a *SlashingAdapter) UnjailValidator(validator common.Address) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	// 검증자 확인
	// 실제 구현에서는 a.eirene.GetValidatorSet().GetValidatorByAddress(validator) 형태로 호출
	// 여기서는 간단히 구현
	val := &Validator{
		Address:     validator,
		Status:      ValidatorStatusJailed,
		JailedUntil: 100, // 임의의 값
	}

	if val == nil {
		return errors.New("validator not found")
	}

	// 검증자가 감금 상태인지 확인
	if val.Status != ValidatorStatusJailed {
		return errors.New("validator is not jailed")
	}

	// 감금 기간 확인
	// 실제 구현에서는 a.eirene.GetValidatorSet().BlockHeight 형태로 호출
	// 여기서는 간단히 구현
	currentBlock := uint64(0)
	if currentBlock < val.JailedUntil {
		return errors.New("validator jail period not over")
	}

	// 감금 해제
	// 실제 구현에서는 a.eirene.GetValidatorSet().UnjailValidator(validator) 형태로 호출
	// 여기서는 간단히 구현
	// val.Status = ValidatorStatusActive
	// val.JailedUntil = 0

	// 서명 정보 업데이트
	if info, exists := a.slashingState.ValidatorSigningInfo[validator]; exists {
		info.JailedUntil = 0
	}

	// 데이터베이스에 저장
	// 실제 구현에서는 a.eirene.GetDB() 형태로 호출
	// 여기서는 간단히 구현
	// if err := a.slashingState.store(a.eirene.GetDB()); err != nil {
	// 	return err
	// }

	return nil
}

// GetValidatorSigningInfo는 검증자의 서명 정보를 반환합니다.
func (a *SlashingAdapter) GetValidatorSigningInfo(validator common.Address) (*ValidatorSigningInfo, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	info, exists := a.slashingState.ValidatorSigningInfo[validator]
	if !exists {
		return nil, errors.New("validator signing info not found")
	}
	return info, nil
}

// GetEvidences는 검증자에 대한 증거 목록을 반환합니다.
func (a *SlashingAdapter) GetEvidences(validator common.Address) []SlashingEvidence {
	a.lock.RLock()
	defer a.lock.RUnlock()

	return a.slashingState.getEvidences(validator)
}

// GetSlashingParams는 슬래싱 매개변수를 반환합니다.
func (a *SlashingAdapter) GetSlashingParams() map[string]interface{} {
	a.lock.RLock()
	defer a.lock.RUnlock()

	return map[string]interface{}{
		"doubleSignSlashRatio":  a.slashingState.DoubleSignSlashRatio,
		"downtimeSlashRatio":    a.slashingState.DowntimeSlashRatio,
		"misbehaviorSlashRatio": a.slashingState.MisbehaviorSlashRatio,
		"doubleSignJailPeriod":  a.slashingState.DoubleSignJailPeriod,
		"downtimeJailPeriod":    a.slashingState.DowntimeJailPeriod,
		"misbehaviorJailPeriod": a.slashingState.MisbehaviorJailPeriod,
		"downtimeBlocksWindow":  a.slashingState.DowntimeBlocksWindow,
		"downtimeThreshold":     a.slashingState.DowntimeThreshold,
	}
}
