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
	"time"

	abci "github.com/tendermint/tendermint/abci/types"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/zenanetwork/go-zenanet/consensus"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/core"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/params"
)

// Validator 구조체 정의
type Validator struct {
	Address []byte
	Power   int64
}

// ABCIAdapter는 Tendermint의 ABCI 인터페이스와 Eirene 합의 엔진을 연결하는 어댑터입니다.
type ABCIAdapter struct {
	eirene *core.Eirene     // Eirene 합의 엔진 인스턴스
	app    abci.Application // Tendermint ABCI 애플리케이션 인스턴스
	logger log.Logger
	validatorSet utils.ValidatorSetInterface // 검증자 집합
}

// NewABCIAdapter는 새로운 ABCIAdapter 인스턴스를 생성합니다.
func NewABCIAdapter(eirene *core.Eirene, app abci.Application, validatorSet utils.ValidatorSetInterface) *ABCIAdapter {
	return &ABCIAdapter{
		eirene: eirene,
		app:    app,
		logger: log.New("module", "abci"),
		validatorSet: validatorSet,
	}
}

// ConvertHeader는 go-zenanet의 Header를 Tendermint의 Header로 변환합니다.
func (a *ABCIAdapter) ConvertHeader(header *types.Header) *tmtypes.Header {
	// 시간 변환 (Unix 타임스탬프를 Time으로 변환)
	timeVal := time.Unix(int64(header.Time), 0)

	// 해시 변환
	hash := header.Hash()

	// Tendermint Header 생성
	tmHeader := &tmtypes.Header{
		Version: tmversion.Consensus{
			Block: 10, // 버전 정보 설정
		},
		ChainID: fmt.Sprintf("%d", header.Number.Uint64()), // 체인 ID 설정
		Height:  header.Number.Int64(),                     // 블록 높이 설정
		Time:    timeVal,                                   // 시간 설정
		LastBlockID: tmtypes.BlockID{
			Hash: hash.Bytes(), // 이전 블록 해시 설정
		},
		AppHash:         header.Root.Bytes(),     // 애플리케이션 상태 해시 설정
		ProposerAddress: header.Coinbase.Bytes(), // 제안자 주소 설정
	}

	return tmHeader
}

// ConvertBlock은 go-zenanet의 Block을 Tendermint의 Block으로 변환합니다.
func (a *ABCIAdapter) ConvertBlock(block *types.Block) *tmtypes.Block {
	// Header 변환
	header := a.ConvertHeader(block.Header())

	// 트랜잭션 변환
	txs := make([]tmtypes.Tx, len(block.Transactions()))
	for i, tx := range block.Transactions() {
		txBytes, err := tx.MarshalJSON()
		if err != nil {
			a.logger.Error("Failed to marshal transaction", "err", err)
			continue
		}
		txs[i] = txBytes
	}

	// 증거 데이터 (현재는 비어있음)
	evidence := tmtypes.EvidenceData{}

	// Tendermint Block 생성
	return &tmtypes.Block{
		Header:     *header,
		Data:       tmtypes.Data{Txs: txs},
		Evidence:   evidence,
		LastCommit: &tmtypes.Commit{}, // 마지막 커밋 정보 (실제 구현에서는 서명 정보를 포함)
	}
}

// ConvertTx는 go-zenanet의 Transaction을 Tendermint의 Tx로 변환합니다.
func (a *ABCIAdapter) ConvertTx(tx *types.Transaction) tmtypes.Tx {
	txBytes, err := tx.MarshalJSON()
	if err != nil {
		a.logger.Error("Failed to marshal transaction", "err", err)
		return nil
	}
	return txBytes
}

// ProcessBlock은 go-zenanet의 Block을 Tendermint ABCI 애플리케이션에서 처리합니다.
func (a *ABCIAdapter) ProcessBlock(chain consensus.ChainHeaderReader, block *types.Block, state *state.StateDB) error {
	// 블록 변환
	tmBlock := a.ConvertBlock(block)

	// BeginBlock 호출
	beginBlockReq := abci.RequestBeginBlock{
		Hash: block.Hash().Bytes(),
		Header: tmproto.Header{
			Version: tmversion.Consensus{
				Block: 10,
			},
			ChainID: fmt.Sprintf("%d", block.Number().Uint64()),
			Height:  block.Number().Int64(),
			Time:    time.Unix(int64(block.Time()), 0),
			LastBlockId: tmproto.BlockID{
				Hash: block.ParentHash().Bytes(),
			},
			AppHash:         block.Root().Bytes(),
			ProposerAddress: block.Coinbase().Bytes(),
		},
		LastCommitInfo: abci.LastCommitInfo{
			Round: 0,
			Votes: []abci.VoteInfo{}, // 실제 구현에서는 검증자 투표 정보를 포함
		},
		ByzantineValidators: []abci.Evidence{}, // 실제 구현에서는 악의적인 검증자 증거를 포함
	}
	beginBlockRes := a.app.BeginBlock(beginBlockReq)
	a.logger.Debug("BeginBlock response", "res", beginBlockRes)

	// DeliverTx 호출 (각 트랜잭션 처리)
	for _, tx := range tmBlock.Data.Txs {
		deliverTxReq := abci.RequestDeliverTx{
			Tx: tx,
		}
		deliverTxRes := a.app.DeliverTx(deliverTxReq)
		if deliverTxRes.Code != 0 {
			a.logger.Error("DeliverTx failed", "code", deliverTxRes.Code, "log", deliverTxRes.Log)
			// 실제 구현에서는 트랜잭션 실패 처리 로직 추가
		}
	}

	// EndBlock 호출
	endBlockReq := abci.RequestEndBlock{
		Height: block.Number().Int64(),
	}
	endBlockRes := a.app.EndBlock(endBlockReq)
	a.logger.Debug("EndBlock response", "res", endBlockRes)

	// Commit 호출
	commitRes := a.app.Commit()
	a.logger.Debug("Commit response", "res", commitRes)

	return nil
}

// InitChain은 체인 초기화 시 Tendermint ABCI 애플리케이션의 InitChain을 호출합니다.
func (a *ABCIAdapter) InitChain(chainConfig *params.ChainConfig, genesisBlock *types.Block) error {
	// 검증자 집합 생성
	validators := []abci.ValidatorUpdate{}
	// 실제 구현에서는 초기 검증자 집합을 설정

	// InitChain 호출
	initChainReq := abci.RequestInitChain{
		Time:    time.Unix(int64(genesisBlock.Time()), 0),
		ChainId: fmt.Sprintf("%d", chainConfig.ChainID.Uint64()),
		ConsensusParams: &abci.ConsensusParams{
			Block: &abci.BlockParams{
				MaxBytes: 1048576, // 1MB
				MaxGas:   -1,      // 무제한
			},
			Evidence: &tmproto.EvidenceParams{
				MaxAgeNumBlocks: 100000,
				MaxAgeDuration:  172800000000000, // 48시간
			},
			Validator: &tmproto.ValidatorParams{
				PubKeyTypes: []string{"ed25519"},
			},
		},
		Validators:    validators,
		AppStateBytes: []byte{}, // 실제 구현에서는 초기 애플리케이션 상태를 설정
	}
	initChainRes := a.app.InitChain(initChainReq)
	a.logger.Debug("InitChain response", "res", initChainRes)

	return nil
}

// Query는 Tendermint ABCI 애플리케이션의 Query를 호출합니다.
func (a *ABCIAdapter) Query(path string, data []byte) ([]byte, error) {
	queryReq := abci.RequestQuery{
		Path:   path,
		Data:   data,
		Height: 0, // 최신 상태 조회
		Prove:  false,
	}
	queryRes := a.app.Query(queryReq)
	if queryRes.Code != 0 {
		return nil, fmt.Errorf("query failed: %s", queryRes.Log)
	}
	return queryRes.Value, nil
}

// CheckTx는 Tendermint ABCI 애플리케이션의 CheckTx를 호출합니다.
func (a *ABCIAdapter) CheckTx(tx *types.Transaction) error {
	txBytes := a.ConvertTx(tx)
	checkTxReq := abci.RequestCheckTx{
		Tx:   txBytes,
		Type: abci.CheckTxType_New,
	}
	checkTxRes := a.app.CheckTx(checkTxReq)
	if checkTxRes.Code != 0 {
		return fmt.Errorf("check tx failed: %s", checkTxRes.Log)
	}
	return nil
}

// GetValidators는 현재 검증자 집합을 반환합니다.
func (a *ABCIAdapter) GetValidators() ([]*Validator, error) {
	// Query를 통해 검증자 집합 조회
	validatorsBytes, err := a.Query("validators", nil)
	if err != nil {
		return nil, err
	}

	// 실제 구현에서는 바이트 배열을 Validator 구조체로 변환
	validators := []*Validator{}

	// 여기서는 간단한 예시로 빈 검증자 목록을 반환합니다.
	// 실제 구현에서는 validatorsBytes를 파싱하여 검증자 목록을 생성해야 합니다.
	a.logger.Debug("Retrieved validators data", "bytes_length", len(validatorsBytes))

	return validators, nil
}

// UpdateValidators는 검증자 집합을 업데이트합니다.
func (a *ABCIAdapter) UpdateValidators(validators []*Validator) error {
	// 실제 구현에서는 검증자 집합 업데이트 로직 구현
	return nil
}
