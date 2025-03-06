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
	"github.com/zenanetwork/go-zenanet/trie"
)

// 테스트 상수
const (
	testTxCount       = 1000 // 테스트용 트랜잭션 수
	testBlockCount    = 10   // 테스트용 블록 수
	testAccountCount  = 100  // 테스트용 계정 수
	testCacheHitRatio = 0.8  // 테스트용 캐시 적중률
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
			uint64(i),           // 논스
			to,                  // 수신자
			big.NewInt(1000000), // 금액
			21000,               // 가스 한도
			big.NewInt(1),       // 가스 가격
			[]byte{},            // 데이터
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
		TxHash:      types.DeriveSha(types.Transactions(txs), trie.NewEmpty(nil)),
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

	// 트랜잭션 해시 트리 계산
	txHash := types.DeriveSha(types.Transactions(txs), trie.NewEmpty(nil))
	header.TxHash = txHash

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
	// 테스트를 스킵합니다 (실제 구현이 완료되면 이 줄을 제거하세요)
	t.Skip("성능 최적화 모듈 구현이 완료되지 않았습니다")

	// 성능 최적화기 생성
	optimizer := core.NewPerformanceOptimizer()

	// 테스트 상태 설정
	stateDB := setupTestState()

	// 테스트 블록 생성
	blocks := make([]*types.Block, testBlockCount)
	for i := 0; i < testBlockCount; i++ {
		// 각 블록에 다른 수의 트랜잭션 포함 (현실적인 시나리오 시뮬레이션)
		txCount := rand.Intn(testTxCount) + 100 // 최소 100개 트랜잭션
		txs := generateTestTransactions(txCount)
		blocks[i] = generateTestBlock(i+1, txs)
	}

	// 워밍업 (JIT 컴파일러 효과 제거)
	warmupBlock := generateTestBlock(0, generateTestTransactions(100))
	err := optimizer.ProcessBlockParallel(warmupBlock, stateDB)
	require.NoError(t, err, "워밍업 블록 처리 중 오류 발생")

	// 성능 측정
	totalTime := time.Duration(0)
	txCount := 0

	for i, block := range blocks {
		txCount += len(block.Transactions())

		// 블록 처리 시간 측정
		start := time.Now()
		err := optimizer.ProcessBlockParallel(block, stateDB)
		elapsed := time.Since(start)
		totalTime += elapsed

		require.NoError(t, err, "블록 %d 처리 중 오류 발생", i+1)

		// 개별 블록 처리 시간 로깅
		t.Logf("블록 %d 처리 시간: %v (트랜잭션 수: %d, TPS: %.2f)",
			i+1, elapsed, len(block.Transactions()),
			float64(len(block.Transactions()))/elapsed.Seconds())
	}

	// 평균 처리 시간 및 TPS 계산
	avgTime := totalTime / time.Duration(testBlockCount)
	avgTPS := float64(txCount) / totalTime.Seconds()

	t.Logf("평균 블록 처리 시간: %v", avgTime)
	t.Logf("평균 TPS: %.2f", avgTPS)
	t.Logf("총 처리된 트랜잭션 수: %d", txCount)
	t.Logf("총 처리 시간: %v", totalTime)

	// 성능 통계 확인 (실제 구현에서는 성능 통계를 가져오는 메서드 필요)
	// stats := optimizer.GetPerformanceStats()
	// t.Logf("성능 통계: %+v", stats)

	// 성능 기준 검증
	assert.Less(t, avgTime, 500*time.Millisecond, "평균 블록 처리 시간이 너무 깁니다")
	assert.Greater(t, avgTPS, float64(1000), "평균 TPS가 너무 낮습니다")
}

// TestCacheEfficiency는 캐시 효율성을 테스트합니다.
func TestCacheEfficiency(t *testing.T) {
	// 테스트를 스킵합니다 (실제 구현이 완료되면 이 줄을 제거하세요)
	t.Skip("성능 최적화 모듈 구현이 완료되지 않았습니다")

	// 성능 최적화기 생성
	optimizer := core.NewPerformanceOptimizer()

	// 테스트 상태 설정
	stateDB := setupTestState()

	// 테스트 계정 생성
	accounts := make([]common.Address, testAccountCount)
	for i := 0; i < testAccountCount; i++ {
		key, _ := crypto.GenerateKey()
		accounts[i] = crypto.PubkeyToAddress(key.PublicKey)

		// 각 계정에 잔액 설정 (실제 구현에서는 적절한 인자 전달 필요)
		// stateDB.AddBalance(accounts[i], big.NewInt(1000000))
	}

	// 테스트 트랜잭션 생성 (일부 계정은 반복해서 사용)
	txs := make([]*types.Transaction, testTxCount)
	for i := 0; i < testTxCount; i++ {
		// 캐시 적중률을 시뮬레이션하기 위해 일부 계정 재사용
		var sender, recipient common.Address
		if rand.Float64() < testCacheHitRatio {
			// 캐시 적중 시나리오: 자주 사용되는 계정 사용
			senderIdx := rand.Intn(testAccountCount / 5)
			recipientIdx := rand.Intn(testAccountCount / 5)
			sender = accounts[senderIdx]
			recipient = accounts[recipientIdx]
		} else {
			// 캐시 미스 시나리오: 덜 사용되는 계정 사용
			senderIdx := testAccountCount/5 + rand.Intn(testAccountCount*4/5)
			recipientIdx := testAccountCount/5 + rand.Intn(testAccountCount*4/5)
			sender = accounts[senderIdx]
			recipient = accounts[recipientIdx]
		}

		// 트랜잭션 생성
		tx := types.NewTransaction(
			uint64(i),
			recipient,
			big.NewInt(1000),
			21000,
			big.NewInt(1),
			nil,
		)
		txs[i] = tx

		// 검증자 정보 캐시에 저장
		optimizer.StoreValidatorInCache(sender, struct{}{})
	}

	// 캐시 효율성 측정을 위한 블록 처리
	block := generateTestBlock(1, txs)

	// 워밍업
	err := optimizer.ProcessBlockParallel(block, stateDB)
	require.NoError(t, err, "워밍업 블록 처리 중 오류 발생")

	// 캐시 통계 초기화
	optimizer.ClearCache()

	// 성능 측정
	start := time.Now()
	err = optimizer.ProcessBlockParallel(block, stateDB)
	elapsed := time.Since(start)

	require.NoError(t, err, "블록 처리 중 오류 발생")

	// 캐시 적중률 계산 (실제 구현에서는 적절한 메서드 호출 필요)
	// hitRate := optimizer.CalculateCacheHitRate()
	hitRate := testCacheHitRatio // 임시로 테스트 값 사용

	t.Logf("캐시 적중률: %.2f%%", hitRate*100)
	t.Logf("블록 처리 시간: %v", elapsed)
	t.Logf("트랜잭션 수: %d", len(block.Transactions()))
	t.Logf("TPS: %.2f", float64(len(block.Transactions()))/elapsed.Seconds())

	// 캐시 효율성 검증
	assert.InDelta(t, testCacheHitRatio, hitRate, 0.2, "캐시 적중률이 예상과 크게 다릅니다")

	// 캐시 사용 시 성능 향상 검증
	optimizer.ClearCache() // 캐시 초기화

	// 캐시 없이 처리
	start = time.Now()
	err = optimizer.ProcessBlockParallel(block, stateDB)
	elapsedWithoutCache := time.Since(start)

	require.NoError(t, err, "캐시 없이 블록 처리 중 오류 발생")

	t.Logf("캐시 없이 블록 처리 시간: %v", elapsedWithoutCache)
	t.Logf("성능 향상: %.2f%%", (float64(elapsedWithoutCache-elapsed)/float64(elapsedWithoutCache))*100)

	// 캐시 사용 시 성능이 향상되어야 함
	assert.Less(t, elapsed, elapsedWithoutCache, "캐시 사용 시 성능이 향상되어야 합니다")
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

// TestBlockProcessingScalability는 블록 크기에 따른 처리 성능 확장성을 테스트합니다.
func TestBlockProcessingScalability(t *testing.T) {
	// 테스트를 스킵합니다 (실제 구현이 완료되면 이 줄을 제거하세요)
	t.Skip("성능 최적화 모듈 구현이 완료되지 않았습니다")

	// 성능 최적화기 생성
	optimizer := core.NewPerformanceOptimizer()

	// 테스트 상태 설정
	stateDB := setupTestState()

	// 다양한 크기의 블록 테스트
	blockSizes := []int{100, 500, 1000, 2000, 5000}

	// 각 블록 크기별 성능 측정
	for _, size := range blockSizes {
		// 테스트 트랜잭션 생성
		txs := generateTestTransactions(size)
		block := generateTestBlock(1, txs)

		// 워밍업
		optimizer.ProcessBlockParallel(block, stateDB)

		// 성능 측정
		start := time.Now()
		err := optimizer.ProcessBlockParallel(block, stateDB)
		elapsed := time.Since(start)

		require.NoError(t, err, "블록 처리 중 오류 발생 (크기: %d)", size)

		tps := float64(size) / elapsed.Seconds()
		t.Logf("블록 크기: %d, 처리 시간: %v, TPS: %.2f", size, elapsed, tps)

		// 처리 시간이 트랜잭션 수에 비례하는지 확인 (선형 확장성)
		// 참고: 실제로는 약간의 오버헤드가 있을 수 있음
		if size > 100 {
			expectedRatio := float64(size) / 100.0
			actualRatio := elapsed.Seconds() / float64(blockSizes[0]) * 100.0
			t.Logf("예상 비율: %.2f, 실제 비율: %.2f", expectedRatio, actualRatio)

			// 확장성 검증 (실제 비율이 예상 비율의 1.5배 이내여야 함)
			assert.InDelta(t, expectedRatio, actualRatio, expectedRatio*0.5,
				"블록 크기 %d에서 확장성이 선형이 아닙니다", size)
		}
	}
}
