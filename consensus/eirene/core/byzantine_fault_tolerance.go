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
	"math/big"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/log"
)

// ByzantineFaultToleranceConfig는 비잔틴 내결함성 설정을 정의합니다.
type ByzantineFaultToleranceConfig struct {
	// 기본 설정
	MaxFaultyValidators     int           // 최대 허용 비잔틴 검증자 수
	ConsensusThreshold      float64       // 합의 임계값 (0.0 ~ 1.0)
	VoteCollectionTimeout   time.Duration // 투표 수집 타임아웃
	
	// 증거 수집 설정
	EvidenceExpiryBlocks    uint64        // 증거 만료 블록 수
	MaxEvidencePerBlock     int           // 블록당 최대 증거 수
	
	// 슬래싱 설정
	DoubleSignSlashAmount   *big.Int      // 이중 서명 슬래싱 금액
	DowntimeSlashAmount     *big.Int      // 다운타임 슬래싱 금액
	
	// 투표 설정
	PrecommitWaitTime       time.Duration // 사전 커밋 대기 시간
	PrecommitResendInterval time.Duration // 사전 커밋 재전송 간격
	
	// 블록 검증 설정
	BlockVerificationTimeout time.Duration // 블록 검증 타임아웃
	BlockVerificationRetries int           // 블록 검증 재시도 횟수
}

// ByzantineEvidence는 비잔틴 행동에 대한 증거를 나타냅니다.
type ByzantineEvidence struct {
	Type          string         // 증거 유형 (DoubleSign, Downtime 등)
	Validator     common.Address // 검증자 주소
	BlockNumber   uint64         // 블록 번호
	BlockHash     common.Hash    // 블록 해시
	Timestamp     time.Time      // 타임스탬프
	Evidence      []byte         // 증거 데이터
	ReporterAddr  common.Address // 신고자 주소
	Verified      bool           // 검증 여부
	SlashExecuted bool           // 슬래싱 실행 여부
}

// ByzantineFaultTolerance는 비잔틴 내결함성을 관리하는 구조체입니다.
type ByzantineFaultTolerance struct {
	config        ByzantineFaultToleranceConfig // 설정
	
	// 증거 관리
	evidenceStore map[common.Hash]*ByzantineEvidence // 증거 저장소 (증거 해시 -> 증거)
	
	// 투표 관리
	votes         map[common.Hash]map[common.Address]bool // 투표 저장소 (블록 해시 -> 검증자 -> 투표)
	voteCount     map[common.Hash]int                     // 투표 수 (블록 해시 -> 투표 수)
	
	// 검증자 관리
	validators    map[common.Address]bool                 // 활성 검증자 목록
	faultyNodes   map[common.Address]int                  // 비잔틴 노드 목록 (주소 -> 위반 횟수)
	
	// 동시성 제어
	lock          sync.RWMutex
	
	// 로깅
	logger        log.Logger
}

// NewByzantineFaultTolerance는 새로운 ByzantineFaultTolerance 인스턴스를 생성합니다.
func NewByzantineFaultTolerance(config ByzantineFaultToleranceConfig) *ByzantineFaultTolerance {
	// 기본값 설정
	if config.MaxFaultyValidators <= 0 {
		config.MaxFaultyValidators = 1
	}
	if config.ConsensusThreshold <= 0 {
		config.ConsensusThreshold = 0.67 // 2/3
	}
	if config.VoteCollectionTimeout <= 0 {
		config.VoteCollectionTimeout = 10 * time.Second
	}
	if config.EvidenceExpiryBlocks <= 0 {
		config.EvidenceExpiryBlocks = 100
	}
	if config.MaxEvidencePerBlock <= 0 {
		config.MaxEvidencePerBlock = 50
	}
	if config.DoubleSignSlashAmount == nil {
		config.DoubleSignSlashAmount = new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18)) // 100 토큰
	}
	if config.DowntimeSlashAmount == nil {
		config.DowntimeSlashAmount = new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18)) // 10 토큰
	}
	if config.PrecommitWaitTime <= 0 {
		config.PrecommitWaitTime = 1 * time.Second
	}
	if config.PrecommitResendInterval <= 0 {
		config.PrecommitResendInterval = 500 * time.Millisecond
	}
	if config.BlockVerificationTimeout <= 0 {
		config.BlockVerificationTimeout = 5 * time.Second
	}
	if config.BlockVerificationRetries <= 0 {
		config.BlockVerificationRetries = 3
	}
	
	return &ByzantineFaultTolerance{
		config:        config,
		evidenceStore: make(map[common.Hash]*ByzantineEvidence),
		votes:         make(map[common.Hash]map[common.Address]bool),
		voteCount:     make(map[common.Hash]int),
		validators:    make(map[common.Address]bool),
		faultyNodes:   make(map[common.Address]int),
		logger:        log.New("module", "byzantine_fault_tolerance"),
	}
}

// AddValidator는 검증자를 추가합니다.
func (bft *ByzantineFaultTolerance) AddValidator(validator common.Address) {
	bft.lock.Lock()
	defer bft.lock.Unlock()
	
	bft.validators[validator] = true
}

// RemoveValidator는 검증자를 제거합니다.
func (bft *ByzantineFaultTolerance) RemoveValidator(validator common.Address) {
	bft.lock.Lock()
	defer bft.lock.Unlock()
	
	delete(bft.validators, validator)
}

// SubmitVote는 블록에 대한 투표를 제출합니다.
func (bft *ByzantineFaultTolerance) SubmitVote(blockHash common.Hash, validator common.Address, vote bool) bool {
	bft.lock.Lock()
	defer bft.lock.Unlock()
	
	// 검증자 확인
	if !bft.validators[validator] {
		bft.logger.Warn("Vote from non-validator", "validator", validator)
		return false
	}
	
	// 투표 저장
	if _, exists := bft.votes[blockHash]; !exists {
		bft.votes[blockHash] = make(map[common.Address]bool)
		bft.voteCount[blockHash] = 0
	}
	
	// 이미 투표했는지 확인
	if _, voted := bft.votes[blockHash][validator]; voted {
		bft.logger.Warn("Duplicate vote", "validator", validator, "block", blockHash)
		return false
	}
	
	// 투표 저장
	bft.votes[blockHash][validator] = vote
	if vote {
		bft.voteCount[blockHash]++
	}
	
	return true
}

// HasConsensus는 블록에 대한 합의가 이루어졌는지 확인합니다.
func (bft *ByzantineFaultTolerance) HasConsensus(blockHash common.Hash) bool {
	bft.lock.RLock()
	defer bft.lock.RUnlock()
	
	// 투표 수 확인
	voteCount, exists := bft.voteCount[blockHash]
	if !exists {
		return false
	}
	
	// 합의 임계값 확인
	validatorCount := len(bft.validators)
	if validatorCount == 0 {
		return false
	}
	
	consensusThreshold := int(float64(validatorCount) * bft.config.ConsensusThreshold)
	return voteCount >= consensusThreshold
}

// SubmitEvidence는 비잔틴 행동에 대한 증거를 제출합니다.
func (bft *ByzantineFaultTolerance) SubmitEvidence(evidence *ByzantineEvidence) bool {
	bft.lock.Lock()
	defer bft.lock.Unlock()
	
	// 증거 해시 계산
	evidenceHash := common.BytesToHash(evidence.Evidence)
	
	// 이미 제출된 증거인지 확인
	if _, exists := bft.evidenceStore[evidenceHash]; exists {
		bft.logger.Debug("Duplicate evidence", "hash", evidenceHash)
		return false
	}
	
	// 증거 저장
	bft.evidenceStore[evidenceHash] = evidence
	
	// 비잔틴 노드 카운트 증가
	bft.faultyNodes[evidence.Validator]++
	
	bft.logger.Info("Evidence submitted", 
		"type", evidence.Type, 
		"validator", evidence.Validator, 
		"block", evidence.BlockNumber)
	
	return true
}

// VerifyEvidence는 증거를 검증합니다.
func (bft *ByzantineFaultTolerance) VerifyEvidence(evidenceHash common.Hash) bool {
	bft.lock.Lock()
	defer bft.lock.Unlock()
	
	// 증거 확인
	evidence, exists := bft.evidenceStore[evidenceHash]
	if !exists {
		return false
	}
	
	// 이미 검증된 증거인지 확인
	if evidence.Verified {
		return true
	}
	
	// 증거 유형에 따라 검증
	var verified bool
	switch evidence.Type {
	case "DoubleSign":
		verified = bft.verifyDoubleSignEvidence(evidence)
	case "Downtime":
		verified = bft.verifyDowntimeEvidence(evidence)
	default:
		bft.logger.Warn("Unknown evidence type", "type", evidence.Type)
		return false
	}
	
	// 검증 결과 저장
	evidence.Verified = verified
	
	return verified
}

// ExecuteSlashing은 슬래싱을 실행합니다.
func (bft *ByzantineFaultTolerance) ExecuteSlashing(evidenceHash common.Hash) bool {
	bft.lock.Lock()
	defer bft.lock.Unlock()
	
	// 증거 확인
	evidence, exists := bft.evidenceStore[evidenceHash]
	if !exists {
		return false
	}
	
	// 검증되지 않은 증거인지 확인
	if !evidence.Verified {
		bft.logger.Warn("Attempting to slash with unverified evidence", "hash", evidenceHash)
		return false
	}
	
	// 이미 슬래싱이 실행된 증거인지 확인
	if evidence.SlashExecuted {
		return true
	}
	
	// 슬래싱 금액 결정
	var slashAmount *big.Int
	switch evidence.Type {
	case "DoubleSign":
		slashAmount = bft.config.DoubleSignSlashAmount
	case "Downtime":
		slashAmount = bft.config.DowntimeSlashAmount
	default:
		bft.logger.Warn("Unknown evidence type for slashing", "type", evidence.Type)
		return false
	}
	
	// 슬래싱 실행 (실제 구현에서는 스테이킹 컨트랙트 호출)
	bft.logger.Info("Executing slashing", 
		"type", evidence.Type, 
		"validator", evidence.Validator, 
		"amount", slashAmount)
	
	// 슬래싱 실행 결과 저장
	evidence.SlashExecuted = true
	
	return true
}

// GetFaultyValidators는 비잔틴 검증자 목록을 반환합니다.
func (bft *ByzantineFaultTolerance) GetFaultyValidators() map[common.Address]int {
	bft.lock.RLock()
	defer bft.lock.RUnlock()
	
	// 복사본 반환
	result := make(map[common.Address]int)
	for addr, count := range bft.faultyNodes {
		result[addr] = count
	}
	
	return result
}

// GetEvidenceCount는 증거 수를 반환합니다.
func (bft *ByzantineFaultTolerance) GetEvidenceCount() int {
	bft.lock.RLock()
	defer bft.lock.RUnlock()
	
	return len(bft.evidenceStore)
}

// CleanupExpiredEvidence는 만료된 증거를 정리합니다.
func (bft *ByzantineFaultTolerance) CleanupExpiredEvidence(currentBlock uint64) int {
	bft.lock.Lock()
	defer bft.lock.Unlock()
	
	removed := 0
	for hash, evidence := range bft.evidenceStore {
		// 만료 확인
		if currentBlock > evidence.BlockNumber+bft.config.EvidenceExpiryBlocks {
			delete(bft.evidenceStore, hash)
			removed++
		}
	}
	
	if removed > 0 {
		bft.logger.Debug("Cleaned up expired evidence", "count", removed)
	}
	
	return removed
}

// VerifyBlock은 블록을 검증합니다.
func (bft *ByzantineFaultTolerance) VerifyBlock(chain consensus.ChainHeaderReader, block *types.Block) error {
	// 블록 헤더 검증
	if err := bft.verifyBlockHeader(chain, block.Header()); err != nil {
		return err
	}
	
	// 블록 바디 검증
	if err := bft.verifyBlockBody(chain, block); err != nil {
		return err
	}
	
	return nil
}

// 내부 함수: 이중 서명 증거 검증
func (bft *ByzantineFaultTolerance) verifyDoubleSignEvidence(evidence *ByzantineEvidence) bool {
	// 실제 구현에서는 서명 검증 로직 구현
	// 여기서는 간단히 true 반환
	return true
}

// 내부 함수: 다운타임 증거 검증
func (bft *ByzantineFaultTolerance) verifyDowntimeEvidence(evidence *ByzantineEvidence) bool {
	// 실제 구현에서는 다운타임 검증 로직 구현
	// 여기서는 간단히 true 반환
	return true
}

// 내부 함수: 블록 헤더 검증
func (bft *ByzantineFaultTolerance) verifyBlockHeader(chain consensus.ChainHeaderReader, header *types.Header) error {
	// 실제 구현에서는 블록 헤더 검증 로직 구현
	// 여기서는 간단히 nil 반환
	return nil
}

// 내부 함수: 블록 바디 검증
func (bft *ByzantineFaultTolerance) verifyBlockBody(chain consensus.ChainHeaderReader, block *types.Block) error {
	// 실제 구현에서는 블록 바디 검증 로직 구현
	// 여기서는 간단히 nil 반환
	return nil
} 