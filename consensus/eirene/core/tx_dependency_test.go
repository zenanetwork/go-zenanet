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
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/crypto"
)

// 테스트용 계정 생성
func generateTestAccount() (*ecdsa.PrivateKey, common.Address) {
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)
	return key, addr
}

// 테스트용 트랜잭션 생성
func createTestTransaction(key *ecdsa.PrivateKey, nonce uint64, to common.Address, value *big.Int, data []byte) *types.Transaction {
	tx := types.NewTransaction(nonce, to, value, 21000, big.NewInt(1), data)
	signer := types.NewEIP155Signer(big.NewInt(1))
	signedTx, _ := types.SignTx(tx, signer, key)
	return signedTx
}

func TestTxDependencyAnalyzer_Basic(t *testing.T) {
	// 의존성 분석기 생성
	analyzer := NewTxDependencyAnalyzer()
	
	// 시작
	if err := analyzer.Start(); err != nil {
		t.Fatalf("Failed to start transaction dependency analyzer: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer analyzer.Stop()
	
	// 테스트 계정 생성
	key1, addr1 := generateTestAccount()
	_, addr2 := generateTestAccount()
	
	// 테스트 트랜잭션 생성
	tx1 := createTestTransaction(key1, 0, addr2, big.NewInt(1000), nil)
	tx2 := createTestTransaction(key1, 1, addr2, big.NewInt(2000), nil)
	
	// 트랜잭션 목록 생성
	txs := types.Transactions{tx1, tx2}
	
	// 의존성 분석 (상태 DB 없이)
	graph, err := analyzer.AnalyzeDependencies(txs, nil)
	if err != nil {
		t.Fatalf("Failed to analyze dependencies: %v", err)
	}
	
	// 그래프 검증
	if len(graph.Transactions) != 2 {
		t.Errorf("Expected 2 transactions in graph, got %d", len(graph.Transactions))
	}
	
	// 의존성 검증 (tx2는 tx1에 의존해야 함)
	tx1Hash := tx1.Hash()
	tx2Hash := tx2.Hash()
	
	deps, ok := graph.Dependencies[tx2Hash]
	if !ok || len(deps) != 1 || deps[0] != tx1Hash {
		t.Errorf("Expected tx2 to depend on tx1, got dependencies: %v", deps)
	}
	
	// 의존성 유형 검증
	depDetails, ok := graph.DependencyDetails[tx2Hash]
	if !ok {
		t.Fatalf("Expected dependency details for tx2")
	}
	
	depDetail, ok := depDetails[tx1Hash]
	if !ok {
		t.Fatalf("Expected dependency detail for tx2 -> tx1")
	}
	
	if depDetail.Type != NonceDependent {
		t.Errorf("Expected NonceDependent dependency type, got %v", depDetail.Type)
	}
	
	if depDetail.From != addr1 || depDetail.To != addr1 {
		t.Errorf("Expected dependency from/to addresses to be %v, got from=%v, to=%v", addr1, depDetail.From, depDetail.To)
	}
}

func TestTxDependencyAnalyzer_ExecutionOrder(t *testing.T) {
	// 의존성 분석기 생성
	analyzer := NewTxDependencyAnalyzer()
	
	// 시작
	if err := analyzer.Start(); err != nil {
		t.Fatalf("Failed to start transaction dependency analyzer: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer analyzer.Stop()
	
	// 테스트 계정 생성
	key1, _ := generateTestAccount()
	_, addr2 := generateTestAccount()
	
	// 테스트 트랜잭션 생성 (논스 순서대로)
	tx1 := createTestTransaction(key1, 0, addr2, big.NewInt(1000), nil)
	tx2 := createTestTransaction(key1, 1, addr2, big.NewInt(2000), nil)
	tx3 := createTestTransaction(key1, 2, addr2, big.NewInt(3000), nil)
	
	// 트랜잭션 목록 생성 (순서 섞기)
	txs := types.Transactions{tx2, tx1, tx3}
	
	// 의존성 분석
	graph, err := analyzer.AnalyzeDependencies(txs, nil)
	if err != nil {
		t.Fatalf("Failed to analyze dependencies: %v", err)
	}
	
	// 실행 순서 검증
	tx1Hash := tx1.Hash()
	tx2Hash := tx2.Hash()
	tx3Hash := tx3.Hash()
	
	if graph.ExecutionOrder[tx1Hash] != 0 {
		t.Errorf("Expected tx1 execution order to be 0, got %d", graph.ExecutionOrder[tx1Hash])
	}
	
	if graph.ExecutionOrder[tx2Hash] != 1 {
		t.Errorf("Expected tx2 execution order to be 1, got %d", graph.ExecutionOrder[tx2Hash])
	}
	
	if graph.ExecutionOrder[tx3Hash] != 2 {
		t.Errorf("Expected tx3 execution order to be 2, got %d", graph.ExecutionOrder[tx3Hash])
	}
}

func TestTxDependencyAnalyzer_OptimalBatches(t *testing.T) {
	// 의존성 분석기 생성
	analyzer := NewTxDependencyAnalyzer()
	
	// 시작
	if err := analyzer.Start(); err != nil {
		t.Fatalf("Failed to start transaction dependency analyzer: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer analyzer.Stop()
	
	// 테스트 계정 생성
	key1, _ := generateTestAccount()
	key2, addr2 := generateTestAccount()
	key3, addr3 := generateTestAccount()
	
	// 테스트 트랜잭션 생성
	// 계정1의 트랜잭션 (서로 의존)
	tx1 := createTestTransaction(key1, 0, addr2, big.NewInt(1000), nil)
	tx2 := createTestTransaction(key1, 1, addr2, big.NewInt(2000), nil)
	
	// 계정2의 트랜잭션 (서로 의존)
	tx3 := createTestTransaction(key2, 0, addr3, big.NewInt(1000), nil)
	tx4 := createTestTransaction(key2, 1, addr3, big.NewInt(2000), nil)
	
	// 계정3의 트랜잭션 (독립적)
	tx5 := createTestTransaction(key3, 0, addr2, big.NewInt(1000), nil)
	
	// 트랜잭션 목록 생성 (순서 섞기)
	txs := types.Transactions{tx2, tx4, tx1, tx5, tx3}
	
	// 의존성 분석
	graph, err := analyzer.AnalyzeDependencies(txs, nil)
	if err != nil {
		t.Fatalf("Failed to analyze dependencies: %v", err)
	}
	
	// 최적 배치 생성
	batches := analyzer.GetOptimalBatches(graph)
	
	// 배치 검증
	if len(batches) < 2 {
		t.Errorf("Expected at least 2 batches, got %d", len(batches))
	}
}

func TestTxDependencyAnalyzer_ContractCallDependent(t *testing.T) {
	// 의존성 분석기 생성
	analyzer := NewTxDependencyAnalyzer()
	
	// 시작
	if err := analyzer.Start(); err != nil {
		t.Fatalf("Failed to start transaction dependency analyzer: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer analyzer.Stop()
	
	// 테스트 계정 생성
	key1, _ := generateTestAccount()
	key2, _ := generateTestAccount()
	_, contractAddr := generateTestAccount() // 가상의 컨트랙트 주소
	
	// 컨트랙트 호출 데이터
	callData := []byte{0x12, 0x34, 0x56, 0x78}
	
	// 테스트 트랜잭션 생성 (같은 컨트랙트 호출)
	tx1 := createTestTransaction(key1, 0, contractAddr, big.NewInt(0), callData)
	tx2 := createTestTransaction(key2, 0, contractAddr, big.NewInt(0), callData)
	
	// 트랜잭션 목록 생성
	txs := types.Transactions{tx1, tx2}
	
	// 의존성 분석
	graph, err := analyzer.AnalyzeDependencies(txs, nil)
	if err != nil {
		t.Fatalf("Failed to analyze dependencies: %v", err)
	}
	
	// 그래프 검증
	if len(graph.Transactions) != 2 {
		t.Errorf("Expected 2 transactions in graph, got %d", len(graph.Transactions))
	}
}

func TestTxDependencyAnalyzer_GetDependencyStats(t *testing.T) {
	// 의존성 분석기 생성
	analyzer := NewTxDependencyAnalyzer()
	
	// 시작
	if err := analyzer.Start(); err != nil {
		t.Fatalf("Failed to start transaction dependency analyzer: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer analyzer.Stop()
	
	// 테스트 계정 생성
	key1, _ := generateTestAccount()
	_, addr2 := generateTestAccount()
	
	// 테스트 트랜잭션 생성
	tx1 := createTestTransaction(key1, 0, addr2, big.NewInt(1000), nil)
	tx2 := createTestTransaction(key1, 1, addr2, big.NewInt(2000), nil)
	tx3 := createTestTransaction(key1, 2, addr2, big.NewInt(3000), nil)
	
	// 트랜잭션 목록 생성
	txs := types.Transactions{tx1, tx2, tx3}
	
	// 의존성 분석
	graph, err := analyzer.AnalyzeDependencies(txs, nil)
	if err != nil {
		t.Fatalf("Failed to analyze dependencies: %v", err)
	}
	
	// 의존성 통계 가져오기
	stats := analyzer.GetDependencyStats(graph)
	
	// 통계 검증
	if stats["tx_count"].(int) != 3 {
		t.Errorf("Expected tx_count to be 3, got %v", stats["tx_count"])
	}
	
	if stats["dependent_tx_count"].(int) != 2 {
		t.Errorf("Expected dependent_tx_count to be 2, got %v", stats["dependent_tx_count"])
	}
	
	if stats["nonce_dependencies"].(int) != 2 {
		t.Errorf("Expected nonce_dependencies to be 2, got %v", stats["nonce_dependencies"])
	}
	
	if stats["max_depth"].(int) != 2 {
		t.Errorf("Expected max_depth to be 2, got %v", stats["max_depth"])
	}
}

func TestTxDependencyAnalyzer_EmptyTransactions(t *testing.T) {
	// 의존성 분석기 생성
	analyzer := NewTxDependencyAnalyzer()
	
	// 시작
	if err := analyzer.Start(); err != nil {
		t.Fatalf("Failed to start transaction dependency analyzer: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer analyzer.Stop()
	
	// 빈 트랜잭션 목록
	txs := types.Transactions{}
	
	// 의존성 분석
	graph, err := analyzer.AnalyzeDependencies(txs, nil)
	if err != nil {
		t.Fatalf("Failed to analyze dependencies: %v", err)
	}
	
	// 그래프 검증
	if len(graph.Transactions) != 0 {
		t.Errorf("Expected 0 transactions in graph, got %d", len(graph.Transactions))
	}
	
	if len(graph.Dependencies) != 0 {
		t.Errorf("Expected 0 dependencies in graph, got %d", len(graph.Dependencies))
	}
	
	if len(graph.ExecutionOrder) != 0 {
		t.Errorf("Expected 0 execution orders in graph, got %d", len(graph.ExecutionOrder))
	}
}

func BenchmarkTxDependencyAnalyzer(b *testing.B) {
	// 의존성 분석기 생성
	analyzer := NewTxDependencyAnalyzer()
	
	// 시작
	if err := analyzer.Start(); err != nil {
		b.Fatalf("Failed to start transaction dependency analyzer: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer analyzer.Stop()
	
	// 테스트 계정 생성
	key1, _ := generateTestAccount()
	_, addr2 := generateTestAccount()
	
	// 테스트 트랜잭션 생성
	var txs types.Transactions
	for i := 0; i < 100; i++ {
		tx := createTestTransaction(key1, uint64(i), addr2, big.NewInt(int64(i*1000)), nil)
		txs = append(txs, tx)
	}
	
	// 벤치마크 실행
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := analyzer.AnalyzeDependencies(txs, nil)
		if err != nil {
			b.Fatalf("Failed to analyze dependencies: %v", err)
		}
	}
} 