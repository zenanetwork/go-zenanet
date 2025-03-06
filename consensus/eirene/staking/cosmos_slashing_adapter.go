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
	"fmt"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/core"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/cosmos"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/log"
)

// SlashingParams는 슬래싱 관련 매개변수를 정의합니다.
type SlashingParams struct {
	DoubleSignSlashRatio  float64 // 이중 서명 슬래싱 비율 (0-1)
	DowntimeSlashRatio    float64 // 다운타임 슬래싱 비율 (0-1)
	DoubleSignJailPeriod  uint64  // 이중 서명 감금 기간 (블록 수)
	DowntimeJailPeriod    uint64  // 다운타임 감금 기간 (블록 수)
	DowntimeBlocksWindow  uint64  // 다운타임 감지 윈도우 (블록 수)
	DowntimeThreshold     float64 // 다운타임 임계값 (0-1)
}

// DefaultSlashingParams는 기본 슬래싱 매개변수를 반환합니다.
func DefaultSlashingParams() SlashingParams {
	return SlashingParams{
		DoubleSignSlashRatio:  0.05,  // 5%
		DowntimeSlashRatio:    0.01,  // 1%
		DoubleSignJailPeriod:  20000, // 약 3일 (15초 블록 기준)
		DowntimeJailPeriod:    10000, // 약 1.5일 (15초 블록 기준)
		DowntimeBlocksWindow:  100,   // 100 블록
		DowntimeThreshold:     0.5,   // 50%
	}
}

// CosmosSlashingAdapter는 Cosmos SDK의 slashing 모듈과 연동하는 어댑터입니다.
type CosmosSlashingAdapter struct {
	eirene       *core.Eirene
	logger       log.Logger
	storeAdapter *cosmos.StateDBAdapter
	validatorSet *ValidatorSet
	params       SlashingParams
}

// NewCosmosSlashingAdapter는 새로운 CosmosSlashingAdapter 인스턴스를 생성합니다.
func NewCosmosSlashingAdapter(eirene *core.Eirene, storeAdapter *cosmos.StateDBAdapter, validatorSet *ValidatorSet) *CosmosSlashingAdapter {
	return &CosmosSlashingAdapter{
		eirene:       eirene,
		logger:       log.New("module", "cosmos_slashing"),
		storeAdapter: storeAdapter,
		validatorSet: validatorSet,
		params:       DefaultSlashingParams(),
	}
}

// ProcessDoubleSign은 이중 서명 증거를 처리합니다.
func (a *CosmosSlashingAdapter) ProcessDoubleSign(evidence DoubleSignEvidence) error {
	a.logger.Info("Processing double sign evidence", "validator", evidence.Validator.Hex(), "height", evidence.Height)

	// 검증자 확인
	validator := a.validatorSet.GetValidator(evidence.Validator)
	if validator == nil {
		return errors.New("validator not found")
	}

	// 이중 서명 증거 생성
	infraction := "double_sign"

	// 슬래싱 실행
	slashRatio := a.params.DoubleSignSlashRatio
	a.slashValidator(evidence.Validator, slashRatio, infraction, evidence.Height)

	// 감금 실행
	jailPeriod := a.params.DoubleSignJailPeriod
	a.jailValidator(evidence.Validator, jailPeriod)

	// 이벤트 로깅
	a.logger.Info("Validator slashed for double signing",
		"validator", evidence.Validator.Hex(),
		"height", evidence.Height,
		"slashRatio", slashRatio,
		"jailPeriod", jailPeriod)

	return nil
}

// ProcessDowntime은 다운타임 증거를 처리합니다.
func (a *CosmosSlashingAdapter) ProcessDowntime(validator common.Address, missedBlocks uint64, totalBlocks uint64) error {
	a.logger.Info("Processing downtime", "validator", validator.Hex(), "missedBlocks", missedBlocks, "totalBlocks", totalBlocks)

	// 검증자 확인
	validatorObj := a.validatorSet.GetValidator(validator)
	if validatorObj == nil {
		return errors.New("validator not found")
	}

	// 다운타임 임계값 확인
	missedRatio := float64(missedBlocks) / float64(totalBlocks)
	if missedRatio < a.params.DowntimeThreshold {
		return nil
	}

	// 슬래싱 실행
	slashRatio := a.params.DowntimeSlashRatio
	infraction := "downtime"
	a.slashValidator(validator, slashRatio, infraction, a.validatorSet.BlockHeight)

	// 감금 실행
	jailPeriod := a.params.DowntimeJailPeriod
	a.jailValidator(validator, jailPeriod)

	// 이벤트 로깅
	a.logger.Info("Validator slashed for downtime",
		"validator", validator.Hex(),
		"missedBlocks", missedBlocks,
		"totalBlocks", totalBlocks,
		"missedRatio", missedRatio,
		"slashRatio", slashRatio,
		"jailPeriod", jailPeriod)

	return nil
}

// UpdateSigningInfo는 검증자의 서명 정보를 업데이트합니다.
func (a *CosmosSlashingAdapter) UpdateSigningInfo(header *types.Header, signers []common.Address) error {
	height := header.Number.Uint64()
	a.logger.Debug("Updating signing info", "height", height, "signers", len(signers))

	// 활성 검증자 목록 가져오기
	activeValidators := a.validatorSet.GetActiveValidators()

	// 모든 활성 검증자에 대해 서명 여부 확인
	for _, v := range activeValidators {
		validator, ok := v.(*Validator)
		if !ok {
			continue
		}

		signed := false
		for _, signer := range signers {
			if signer == validator.Address {
				signed = true
				break
			}
		}

		// 서명 정보 업데이트
		if signed {
			// 서명한 경우 카운터 리셋
			validator.BlocksMissed = 0
			validator.BlocksSigned++
		} else {
			// 서명하지 않은 경우 카운터 증가
			validator.BlocksMissed++

			// 다운타임 임계값 확인
			if validator.BlocksMissed >= a.params.DowntimeBlocksWindow {
				// 다운타임 처리
				a.ProcessDowntime(validator.Address, validator.BlocksMissed, a.params.DowntimeBlocksWindow)
				
				// 카운터 리셋
				validator.BlocksMissed = 0
			}
		}
	}

	return nil
}

// VerifyDoubleSign은 이중 서명 증거를 검증합니다.
func (a *CosmosSlashingAdapter) VerifyDoubleSign(evidence DoubleSignEvidence) (bool, error) {
	a.logger.Debug("Verifying double sign evidence", "validator", evidence.Validator.Hex(), "height", evidence.Height)

	// 검증자 확인
	validator := a.validatorSet.GetValidator(evidence.Validator)
	if validator == nil {
		return false, errors.New("validator not found")
	}

	// 두 투표가 다른지 확인
	if len(evidence.VoteA) == 0 || len(evidence.VoteB) == 0 {
		return false, errors.New("invalid evidence: empty votes")
	}

	// 두 투표가 같은 높이와 라운드에 대한 것인지 확인
	// 실제 구현에서는 투표 내용을 파싱하여 확인해야 함
	// 여기서는 간단히 구현

	// 두 투표가 다른 내용인지 확인
	if string(evidence.VoteA) == string(evidence.VoteB) {
		return false, errors.New("invalid evidence: identical votes")
	}

	return true, nil
}

// ReportDoubleSign은 이중 서명을 보고합니다.
func (a *CosmosSlashingAdapter) ReportDoubleSign(reporter common.Address, evidence DoubleSignEvidence) error {
	a.logger.Info("Double sign reported", "reporter", reporter.Hex(), "validator", evidence.Validator.Hex(), "height", evidence.Height)

	// 증거 검증
	valid, err := a.VerifyDoubleSign(evidence)
	if err != nil {
		return err
	}
	if !valid {
		return errors.New("invalid double sign evidence")
	}

	// 이중 서명 처리
	return a.ProcessDoubleSign(evidence)
}

// UnjailValidator는 검증자의 감금을 해제합니다.
func (a *CosmosSlashingAdapter) UnjailValidator(validator common.Address) error {
	a.logger.Info("Unjailing validator", "validator", validator.Hex())

	// 검증자 확인
	validatorObj := a.validatorSet.GetValidator(validator)
	if validatorObj == nil {
		return errors.New("validator not found")
	}

	// 감금 상태 확인
	if validatorObj.Status != ValidatorStatusJailed {
		return errors.New("validator is not jailed")
	}

	// 감금 기간 확인
	if a.validatorSet.BlockHeight < validatorObj.JailedUntil {
		return fmt.Errorf("validator still jailed for %d blocks", validatorObj.JailedUntil-a.validatorSet.BlockHeight)
	}

	// 감금 해제
	validatorObj.Status = ValidatorStatusBonded
	a.logger.Info("Validator unjailed", "validator", validator.Hex())

	return nil
}

// slashValidator는 검증자를 슬래싱합니다.
func (a *CosmosSlashingAdapter) slashValidator(validator common.Address, slashRatio float64, infraction string, height uint64) {
	a.logger.Info("Slashing validator", "validator", validator.Hex(), "ratio", slashRatio, "infraction", infraction, "height", height)

	// 검증자 확인
	validatorObj := a.validatorSet.GetValidator(validator)
	if validatorObj == nil {
		return
	}

	// 슬래싱 실행
	slashRatioInt := uint64(slashRatio * 100)
	a.validatorSet.slashValidator(validator, slashRatioInt)

	// 슬래싱 정보 업데이트
	validatorObj.SlashingCount++
	validatorObj.LastSlashedBlock = height
}

// jailValidator는 검증자를 감금합니다.
func (a *CosmosSlashingAdapter) jailValidator(validator common.Address, jailPeriod uint64) {
	a.logger.Info("Jailing validator", "validator", validator.Hex(), "period", jailPeriod)

	// 검증자 확인
	validatorObj := a.validatorSet.GetValidator(validator)
	if validatorObj == nil {
		return
	}

	// 감금 실행
	jailUntil := a.validatorSet.BlockHeight + jailPeriod
	a.validatorSet.jailValidator(validator, jailUntil)
} 