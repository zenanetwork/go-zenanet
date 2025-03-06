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

package core_test

import (
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/core"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/crypto"
)

// 테스트 상수
const (
	testTxCount       = 1000  // 테스트용 트랜잭션 수
	testBlockCount    = 10    // 테스트용 블록 수
	testAccountCount  = 100   // 테스트용 계정 수
	testCacheHitRatio = 0.8   // 테스트용 캐시 적중률
)

// 테스트 헬퍼 함수
func setupTestState() *state.StateDB {
	// 테스트용 상태 DB 생성
	stateDB, _ := state.New(common.Hash{}, state.NewDatabaseForTesting())
	return stateDB
}

// 테스트용 트랜잭션 생성
func generateTestTransactions(count int) []*types.Transaction {
	txs := make([]*types.Transaction, count)
	
	// 테스트용 개인키 생성
	privateKey, _ := crypto.GenerateKey()
	
	for i := 0; i < count; i++ {
		// 랜덤 수신자 주소 생성
		to := common.BytesToAddress([]byte{byte(i % 256), byte((i / 256) % 256), byte((i / 65536) % 256)})
		
		// 트랜잭션 생성
		tx := types.NewTransaction(
			uint64(i),                // 논스
			to,                       // 수신자
			big.NewInt(1000000),      // 금액
			21000,                    // 가스 한도
			big.NewInt(1),            // 가스 가격
			[]byte{},                 // 데이터
		)
		
		// 트랜잭션 서명
		signedTx, _ := types.SignTx(tx, types.NewEIP155Signer(big.NewInt(1)), privateKey)
		txs[i] = signedTx
	}
	
	return txs
}

// 테스트용 블록 생성
func generateTestBlock(number int, txs []*types.Transaction) *types.Block {
	header := &types.Header{
		ParentHash:  common.BytesToHash([]byte{byte(number)}),
		UncleHash:   types.EmptyUncleHash,
		Coinbase:    common.Address{},
		Root:        common.Hash{},
		TxHash:      types.EmptyRootHash,
		ReceiptHash: types.EmptyRootHash,
		Bloom:       types.Bloom{},
		Difficulty:  big.NewInt(1),
		Number:      big.NewInt(int64(number)),
		GasLimit:    10000000,
		GasUsed:     0,
		Time:        uint64(time.Now().Unix()),
		Extra:       []byte{},
		MixDigest:   common.Hash{},
		Nonce:       types.BlockNonce{},
	}
	
	// 블록 바디 생성
	body := &types.Body{
		Transactions: txs,
		Uncles:       []*types.Header{},
	}
	
	block := types.NewBlock(header, body, nil, nil)
	return block
}

// TestTransactionProcessingPerformance는 트랜잭션 처리 성능을 테스트합니다.
func TestTransactionProcessingPerformance(t *testing.T) {
	// 테스트 설정
	stateDB := setupTestState()
	optimizer := core.NewPerformanceOptimizer()
	txs := generateTestTransactions(testTxCount)
	
	// 성능 최적화기 시작
	optimizer.Start()
	defer optimizer.Stop()
	
	// 트랜잭션 처리 시간 측정
	start := time.Now()
	err := optimizer.ProcessTransactionsParallel(txs, stateDB)
	duration := time.Since(start)
	
	// 검증
	require.NoError(t, err, "트랜잭션 처리 중 오류 발생")
	
	// 결과 출력
	t.Logf("트랜잭션 %d개 처리 시간: %v (평균: %v/tx)", 
		testTxCount, duration, duration/time.Duration(testTxCount))
	
	// 성능 기준 검증 (예: 트랜잭션당 1ms 이하)
	avgTxTime := duration / time.Duration(testTxCount)
	assert.Less(t, avgTxTime, time.Millisecond, 
		"트랜잭션 처리 성능이 기준보다 낮음 (트랜잭션당 1ms 이하 기대)")
}

// TestBlockProcessingPerformance는 블록 처리 성능을 테스트합니다.
func TestBlockProcessingPerformance(t *testing.T) {
	// 테스트 설정
	stateDB := setupTestState()
	optimizer := core.NewPerformanceOptimizer()
	
	// 성능 최적화기 시작
	optimizer.Start()
	defer optimizer.Stop()
	
	// 여러 블록 처리 시간 측정
	var totalDuration time.Duration
	txsPerBlock := testTxCount / testBlockCount
	
	for i := 0; i < testBlockCount; i++ {
		// 블록별 트랜잭션 생성
		txs := generateTestTransactions(txsPerBlock)
		block := generateTestBlock(i+1, txs)
		
		// 블록 처리 시간 측정
		start := time.Now()
		err := optimizer.ProcessBlockParallel(block, stateDB)
		blockDuration := time.Since(start)
		totalDuration += blockDuration
		
		// 검증
		require.NoError(t, err, "블록 처리 중 오류 발생")
		
		t.Logf("블록 #%d 처리 시간: %v (트랜잭션 %d개)", 
			i+1, blockDuration, txsPerBlock)
	}
	
	// 결과 출력
	avgBlockTime := totalDuration / time.Duration(testBlockCount)
	t.Logf("평균 블록 처리 시간: %v", avgBlockTime)
	
	// 성능 기준 검증 (예: 블록당 100ms 이하)
	assert.Less(t, avgBlockTime, 100*time.Millisecond, 
		"블록 처리 성능이 기준보다 낮음 (블록당 100ms 이하 기대)")
}

// TestCacheEfficiency는 캐시 효율성을 테스트합니다.
func TestCacheEfficiency(t *testing.T) {
	// 테스트 설정
	optimizer := core.NewPerformanceOptimizer()
	
	// 성능 최적화기 시작
	optimizer.Start()
	defer optimizer.Stop()
	
	// 캐시 테스트를 위한 데이터 준비
	addresses := make([]common.Address, testAccountCount)
	for i := 0; i < testAccountCount; i++ {
		addresses[i] = common.BytesToAddress([]byte{byte(i % 256), byte((i / 256) % 256)})
	}
	
	// 캐시 적중률 테스트
	hitCount := 0
	totalCount := 1000
	
	// 먼저 모든 주소를 캐시에 저장
	for _, addr := range addresses {
		optimizer.StoreValidatorInCache(addr, addr.Bytes())
	}
	
	// 캐시 조회 테스트
	for i := 0; i < totalCount; i++ {
		// 캐시 적중률에 따라 기존 주소 또는 새 주소 선택
		var addr common.Address
		if rand.Float64() < testCacheHitRatio {
			// 기존 주소 (캐시 적중)
			addr = addresses[rand.Intn(len(addresses))]
		} else {
			// 새 주소 (캐시 미스)
			addr = common.BytesToAddress([]byte{byte(rand.Intn(256)), byte(rand.Intn(256)), byte(rand.Intn(256))})
		}
		
		// 캐시에서 조회
		val, found := optimizer.GetValidatorFromCache(addr)
		if found {
			hitCount++
			assert.NotNil(t, val, "캐시에서 찾은 값이 nil임")
		}
	}
	
	// 결과 출력
	actualHitRatio := float64(hitCount) / float64(totalCount)
	t.Logf("캐시 적중률: %.2f%% (기대: %.2f%%)", actualHitRatio*100, testCacheHitRatio*100)
	
	// 캐시 적중률 검증 (기대 적중률의 ±10% 이내)
	minExpectedRatio := testCacheHitRatio * 0.9
	maxExpectedRatio := testCacheHitRatio * 1.1
	assert.GreaterOrEqual(t, actualHitRatio, minExpectedRatio, 
		"캐시 적중률이 기대치보다 낮음")
	assert.LessOrEqual(t, actualHitRatio, maxExpectedRatio, 
		"캐시 적중률이 기대치보다 높음")
}

// TestBatchProcessingPerformance는 배치 처리 성능을 테스트합니다.
func TestBatchProcessingPerformance(t *testing.T) {
	// 테스트 설정
	stateDB := setupTestState()
	
	// 다양한 배치 크기로 테스트
	batchSizes := []int{10, 50, 100, 200, 500}
	txs := generateTestTransactions(testTxCount)
	
	for _, batchSize := range batchSizes {
		// 배치 처리기 생성
		processor := core.NewStateBatchProcessor(batchSize)
		processor.Reset(stateDB)
		
		// 배치 처리 시간 측정
		start := time.Now()
		
		// 트랜잭션 추가
		for _, tx := range txs {
			processor.AddTransaction(tx)
		}
		
		// 남은 트랜잭션 처리
		err := processor.Flush()
		require.NoError(t, err, "배치 처리 중 오류 발생")
		
		duration := time.Since(start)
		
		// 결과 출력
		t.Logf("배치 크기 %d: 트랜잭션 %d개 처리 시간: %v (평균: %v/tx)", 
			batchSize, testTxCount, duration, duration/time.Duration(testTxCount))
	}
}

// TestParallelProcessingScalability는 병렬 처리 확장성을 테스트합니다.
func TestParallelProcessingScalability(t *testing.T) {
	// 테스트 설정
	stateDB := setupTestState()
	txs := generateTestTransactions(testTxCount)
	
	// 다양한 워커 수로 테스트
	workerCounts := []int{1, 2, 4, 8, 16}
	
	for _, workerCount := range workerCounts {
		// 성능 최적화기 생성 (워커 수 수동 설정)
		optimizer := core.NewPerformanceOptimizer()
		optimizer.SetWorkerCount(workerCount)
		
		// 성능 최적화기 시작
		optimizer.Start()
		
		// 트랜잭션 처리 시간 측정
		start := time.Now()
		err := optimizer.ProcessTransactionsParallel(txs, stateDB)
		duration := time.Since(start)
		
		// 성능 최적화기 중지
		optimizer.Stop()
		
		// 검증
		require.NoError(t, err, "트랜잭션 처리 중 오류 발생")
		
		// 결과 출력
		t.Logf("워커 수 %d: 트랜잭션 %d개 처리 시간: %v (평균: %v/tx)", 
			workerCount, testTxCount, duration, duration/time.Duration(testTxCount))
	}
} 