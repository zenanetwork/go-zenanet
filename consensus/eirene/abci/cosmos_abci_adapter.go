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

// Package abci implements the ABCI adapter for the Eirene consensus algorithm.
package abci

import (
	"fmt"
	"math/big"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	abci "github.com/tendermint/tendermint/abci/types"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/core"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/cosmos"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/staking"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	ethstate "github.com/zenanetwork/go-zenanet/core/state"
	ethtypes "github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/log"
)

// CosmosABCIAdapter는 Cosmos SDK와 Tendermint의 ABCI 인터페이스를 연결하는 어댑터입니다.
type CosmosABCIAdapter struct {
	eirene         *core.Eirene                  // Eirene 합의 엔진 인스턴스
	storeAdapter   *cosmos.StateDBAdapter        // 상태 DB 어댑터
	stakingAdapter *staking.CosmosStakingAdapter // 스테이킹 어댑터
	logger         log.Logger
	validatorSet   utils.ValidatorSetInterface // 검증자 집합
}

// NewCosmosABCIAdapter는 새로운 CosmosABCIAdapter 인스턴스를 생성합니다.
func NewCosmosABCIAdapter(
	eirene *core.Eirene,
	storeAdapter *cosmos.StateDBAdapter,
	stakingAdapter *staking.CosmosStakingAdapter,
	validatorSet utils.ValidatorSetInterface,
) *CosmosABCIAdapter {
	return &CosmosABCIAdapter{
		eirene:         eirene,
		storeAdapter:   storeAdapter,
		stakingAdapter: stakingAdapter,
		logger:         log.New("module", "cosmos_abci"),
		validatorSet:   validatorSet,
	}
}

// InitChain은 체인 초기화 시 호출되는 ABCI 메서드입니다.
func (a *CosmosABCIAdapter) InitChain(genesisState *ethstate.StateDB) error {
	a.logger.Debug("Initializing chain with Cosmos SDK")

	// 초기 검증자 집합 생성
	validatorUpdates := a.createInitialValidatorUpdates(genesisState)
	a.logger.Info("Initial validator set created", "count", len(validatorUpdates))

	// 각 모듈의 InitGenesis 호출
	ctx := sdk.Context{} // 임시로 빈 Context 생성
	if err := a.stakingAdapter.InitGenesis(ctx, genesisState); err != nil {
		return fmt.Errorf("failed to initialize genesis state in staking module: %w", err)
	}

	a.logger.Debug("Chain initialized with Cosmos SDK")
	return nil
}

// BeginBlock은 블록 처리 시작 시 호출되는 ABCI 메서드입니다.
func (a *CosmosABCIAdapter) BeginBlock(chain consensus.ChainHeaderReader, block *ethtypes.Block, state *ethstate.StateDB) error {
	a.logger.Debug("Begin processing block with Cosmos SDK", "height", block.Number(), "hash", block.Hash())

	// 블록 헤더 변환
	header := block.Header()
	tmHeader := tmproto.Header{
		ChainID: fmt.Sprintf("%d", chain.Config().ChainID.Uint64()),
		Height:  header.Number.Int64(),
		Time:    time.Unix(int64(header.Time), 0),
		LastBlockId: tmproto.BlockID{
			Hash: block.ParentHash().Bytes(),
		},
		AppHash:         header.Root.Bytes(),
		ProposerAddress: header.Coinbase.Bytes(),
	}

	// 이전 블록의 커밋 정보 가져오기
	lastHeight := header.Number.Int64() - 1
	if lastHeight < 0 {
		lastHeight = 0
	}

	lastValidators := a.validatorSet.GetActiveValidators()

	// 커밋 정보 생성
	voteInfos := make([]abci.VoteInfo, 0, len(lastValidators))
	for _, val := range lastValidators {
		voteInfos = append(voteInfos, abci.VoteInfo{
			Validator: abci.Validator{
				Address: val.GetAddress().Bytes(),
				Power:   val.GetVotingPower().Int64(),
			},
			SignedLastBlock: true, // 실제 구현에서는 서명 여부를 확인
		})
	}

	// 악의적인 검증자 증거 수집
	evidences := a.collectEvidences(block)

	// BeginBlock 요청 생성
	req := abci.RequestBeginBlock{
		Hash:   block.Hash().Bytes(),
		Header: tmHeader,
		LastCommitInfo: abci.LastCommitInfo{
			Round: 0,
			Votes: voteInfos,
		},
		ByzantineValidators: evidences,
	}

	// 애플리케이션 컨텍스트 생성
	// 참고: sdk.NewContext 함수 시그니처가 변경되었습니다.
	// 실제 구현에서는 적절한 MultiStore와 Logger를 제공해야 합니다.
	ctx := sdk.Context{} // 임시로 빈 Context 생성

	// 각 모듈의 BeginBlock 호출
	if err := a.stakingAdapter.BeginBlockWithABCI(ctx, req, state); err != nil {
		a.logger.Error("Failed to begin block in staking module", "err", err)
		return err
	}

	a.logger.Debug("Block processing started with Cosmos SDK", "height", block.Number())
	return nil
}

// DeliverTx는 트랜잭션 처리 시 호출되는 ABCI 메서드입니다.
func (a *CosmosABCIAdapter) DeliverTx(tx *ethtypes.Transaction, state *ethstate.StateDB) error {
	a.logger.Debug("Processing transaction with Cosmos SDK", "txHash", tx.Hash())

	// 트랜잭션 바이트 변환
	txBytes, err := tx.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to marshal transaction: %w", err)
	}

	// 로그에 트랜잭션 크기 기록
	a.logger.Debug("Transaction marshalled", "size", len(txBytes))

	// DeliverTx 요청 생성
	// 실제 구현에서는 아래와 같은 요청을 사용할 수 있습니다:
	// abci.RequestDeliverTx{
	//   Tx: txBytes,
	// }

	// 애플리케이션 컨텍스트 가져오기
	ctxInterface := a.storeAdapter.GetContext()
	
	// 타입 변환 수행
	ctx, ok := ctxInterface.(sdk.Context)
	if !ok {
		return fmt.Errorf("failed to convert context to sdk.Context")
	}

	// 트랜잭션 처리
	// 실제 구현에서는 트랜잭션 타입에 따라 적절한 핸들러 호출
	if tx.To() == nil {
		// 컨트랙트 생성 트랜잭션
		a.logger.Debug("Contract creation transaction", "hash", tx.Hash())
	} else {
		// 일반 트랜잭션 또는 컨트랙트 호출
		a.logger.Debug("Regular transaction", "to", tx.To(), "hash", tx.Hash())

		// 스테이킹 관련 트랜잭션 처리
		if a.isStakingTransaction(tx) {
			if err := a.processStakingTransaction(ctx, tx, state); err != nil {
				a.logger.Error("Failed to process staking transaction", "txHash", tx.Hash(), "err", err)
				return err
			}
		}
	}

	a.logger.Debug("Transaction processed with Cosmos SDK", "txHash", tx.Hash())
	return nil
}

// EndBlock은 블록 처리 종료 시 호출되는 ABCI 메서드입니다.
func (a *CosmosABCIAdapter) EndBlock(req interface{}, state *ethstate.StateDB) ([]abci.ValidatorUpdate, error) {
	a.logger.Debug("End block")

	// 실제 구현에서는 req를 사용하여 블록 높이 등의 정보를 얻어야 합니다.
	// 현재는 req를 사용하지 않으므로 로그만 남깁니다.
	a.logger.Debug("EndBlock request", "req", req)

	// 스테이킹 모듈의 EndBlock 호출
	// 반환값은 []interface{} 타입이지만 현재는 사용하지 않습니다.
	ctx := sdk.Context{} // 임시로 빈 Context 생성
	_, err := a.stakingAdapter.EndBlockWithABCI(ctx, req, state)
	if err != nil {
		a.logger.Error("Failed to end block in staking module", "err", err)
		return nil, err
	}

	// 현재 구현에서는 검증자 업데이트가 없으므로 빈 슬라이스 반환
	// 실제 구현에서는 검증자 업데이트 로직을 추가해야 합니다.
	validatorUpdates := []abci.ValidatorUpdate{}
	a.logger.Debug("Validator updates", "count", len(validatorUpdates))

	return validatorUpdates, nil
}

// Commit은 블록 처리 완료 후 상태를 커밋하는 ABCI 메서드입니다.
func (a *CosmosABCIAdapter) Commit(state *ethstate.StateDB) (common.Hash, error) {
	a.logger.Debug("Committing state with Cosmos SDK")

	// 상태 DB 커밋
	// state.Commit 함수의 시그니처가 변경되었습니다.
	// 실제 구현에서는 적절한 인자를 제공해야 합니다.
	// 현재 버전에서는 (uint64, bool, bool) 인자가 필요합니다.
	root, err := state.Commit(0, true, true)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to commit state: %w", err)
	}

	a.logger.Debug("Block committed with Cosmos SDK", "root", root)
	return root, nil
}

// collectEvidences는 블록에서 악의적인 검증자 증거를 수집합니다.
func (a *CosmosABCIAdapter) collectEvidences(block *ethtypes.Block) []abci.Evidence {
	// 실제 구현에서는 블록에서 증거를 수집
	// 테스트 구현에서는 빈 배열 반환
	return []abci.Evidence{}
}

// 스테이킹 트랜잭션 여부 확인
func (a *CosmosABCIAdapter) isStakingTransaction(tx *ethtypes.Transaction) bool {
	// 실제 구현에서는 트랜잭션 데이터를 분석하여 스테이킹 관련 트랜잭션인지 확인
	// 현재는 간단한 구현으로 대체
	if tx.To() == nil {
		return false
	}

	// 스테이킹 컨트랙트 주소 확인
	// 실제 구현에서는 정확한 스테이킹 컨트랙트 주소 사용
	stakingContractAddress := common.HexToAddress("0x0000000000000000000000000000000000001000")
	return *tx.To() == stakingContractAddress
}

// 스테이킹 트랜잭션 처리
func (a *CosmosABCIAdapter) processStakingTransaction(ctx sdk.Context, tx *ethtypes.Transaction, state *ethstate.StateDB) error {
	// 트랜잭션 데이터 분석
	data := tx.Data()
	if len(data) < 4 {
		return fmt.Errorf("invalid staking transaction data")
	}

	// 함수 시그니처 추출 (첫 4바이트)
	methodID := data[:4]

	// 함수 시그니처에 따라 적절한 처리
	switch {
	case isCreateValidatorMethod(methodID):
		return a.processCreateValidator(ctx, tx, data[4:], state)
	case isEditValidatorMethod(methodID):
		return a.processEditValidator(ctx, tx, data[4:], state)
	case isDelegateMethod(methodID):
		return a.processDelegate(ctx, tx, data[4:], state)
	case isUndelegateMethod(methodID):
		return a.processUndelegate(ctx, tx, data[4:], state)
	case isRedelegateMethod(methodID):
		return a.processRedelegate(ctx, tx, data[4:], state)
	default:
		return fmt.Errorf("unknown staking method")
	}
}

// 검증자 생성 메서드 확인
func isCreateValidatorMethod(methodID []byte) bool {
	// 실제 구현에서는 정확한 함수 시그니처 사용
	return false
}

// 검증자 수정 메서드 확인
func isEditValidatorMethod(methodID []byte) bool {
	// 실제 구현에서는 정확한 함수 시그니처 사용
	return false
}

// 위임 메서드 확인
func isDelegateMethod(methodID []byte) bool {
	// 실제 구현에서는 정확한 함수 시그니처 사용
	return false
}

// 위임 해제 메서드 확인
func isUndelegateMethod(methodID []byte) bool {
	// 실제 구현에서는 정확한 함수 시그니처 사용
	return false
}

// 재위임 메서드 확인
func isRedelegateMethod(methodID []byte) bool {
	// 실제 구현에서는 정확한 함수 시그니처 사용
	return false
}

// 검증자 생성 처리
func (a *CosmosABCIAdapter) processCreateValidator(ctx sdk.Context, tx *ethtypes.Transaction, data []byte, state *ethstate.StateDB) error {
	// 실제 구현에서는 데이터 디코딩 및 검증자 생성 로직 구현
	return nil
}

// 검증자 수정 처리
func (a *CosmosABCIAdapter) processEditValidator(ctx sdk.Context, tx *ethtypes.Transaction, data []byte, state *ethstate.StateDB) error {
	// 실제 구현에서는 데이터 디코딩 및 검증자 수정 로직 구현
	return nil
}

// 위임 처리
func (a *CosmosABCIAdapter) processDelegate(ctx sdk.Context, tx *ethtypes.Transaction, data []byte, state *ethstate.StateDB) error {
	// 실제 구현에서는 데이터 디코딩 및 위임 로직 구현
	return nil
}

// 위임 해제 처리
func (a *CosmosABCIAdapter) processUndelegate(ctx sdk.Context, tx *ethtypes.Transaction, data []byte, state *ethstate.StateDB) error {
	// 실제 구현에서는 데이터 디코딩 및 위임 해제 로직 구현
	return nil
}

// 재위임 처리
func (a *CosmosABCIAdapter) processRedelegate(ctx sdk.Context, tx *ethtypes.Transaction, data []byte, state *ethstate.StateDB) error {
	// 실제 구현에서는 데이터 디코딩 및 재위임 로직 구현
	return nil
}

// 검증자 업데이트
func (a *CosmosABCIAdapter) updateValidators(updates []abci.ValidatorUpdate, state *ethstate.StateDB) error {
	a.logger.Debug("Updating validators", "count", len(updates))

	for _, update := range updates {
		// 검증자 주소 추출
		valAddr := common.BytesToAddress(update.PubKey.GetEd25519())
		power := update.Power

		if update.Power > 0 {
			// 검증자 추가 또는 업데이트
			validator, err := a.stakingAdapter.GetValidator(valAddr)
			if err != nil {
				a.logger.Error("Failed to get validator", "address", valAddr, "err", err)
				continue
			}

			if validator == nil {
				// 새 검증자 생성
				a.logger.Info("Creating new validator", "address", valAddr, "power", power)
				// 새 검증자 생성 로직
				err := a.stakingAdapter.CreateValidator(valAddr, update.PubKey.GetEd25519(), big.NewInt(power))
				if err != nil {
					a.logger.Error("Failed to create validator", "address", valAddr, "err", err)
				}
				continue
			}

			// 검증자 업데이트
			a.logger.Info("Updating validator", "address", valAddr, "power", power)
			// 검증자의 투표 파워를 설정
			validator.VotingPower = big.NewInt(power)
			
			// 검증자 정보 저장
			// stakingAdapter를 통해 검증자 정보 업데이트
			err = a.stakingAdapter.EditValidator(valAddr, "", nil) // 설명과 커미션은 변경하지 않음
			if err != nil {
				a.logger.Error("Failed to update validator", "address", valAddr, "err", err)
			}
			
			// 상태 저장
			if err := a.stakingAdapter.SaveState(state); err != nil {
				a.logger.Error("Failed to save state after updating validator", "address", valAddr, "err", err)
			}
		} else {
			// 검증자 제거
			a.logger.Info("Removing validator", "address", valAddr)
			
			// stakingAdapter를 통해 검증자 제거
			err := a.stakingAdapter.Unstake(state, valAddr)
			if err != nil {
				a.logger.Error("Failed to remove validator", "address", valAddr, "err", err)
			}
			
			// 상태 저장
			if err := a.stakingAdapter.SaveState(state); err != nil {
				a.logger.Error("Failed to save state after removing validator", "address", valAddr, "err", err)
			}
		}
	}

	return nil
}

// 초기 검증자 집합 생성
func (a *CosmosABCIAdapter) createInitialValidatorUpdates(state *ethstate.StateDB) []abci.ValidatorUpdate {
	// 상태 DB에서 검증자 정보 가져오기
	validators := a.stakingAdapter.GetValidators()

	validatorUpdates := make([]abci.ValidatorUpdate, 0, len(validators))
	for _, val := range validators {
		power := val.GetVotingPower()
		// PubKey 필드를 직접 설정하지 않고 구조체 리터럴을 사용합니다.
		update := abci.ValidatorUpdate{}
		update.Power = power.Int64()
		// 실제 구현에서는 PubKey 필드를 적절히 설정해야 합니다.
		validatorUpdates = append(validatorUpdates, update)
	}

	return validatorUpdates
} 