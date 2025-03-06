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
	"sort"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/metrics"
)

// 트랜잭션 의존성 분석 관련 상수
const (
	// 캐싱 관련
	defaultDependencyCacheSize = 1024
	
	// 분석 관련
	maxDependencyDepth = 10
	
	// 메트릭스 관련
	txDependencyMetricsInterval = 1 * time.Minute
)

// TxDependencyType은 트랜잭션 의존성 유형을 나타냅니다.
type TxDependencyType int

const (
	// NonceDependent는 논스 기반 의존성을 나타냅니다.
	NonceDependent TxDependencyType = iota
	
	// StateDependent는 상태 기반 의존성을 나타냅니다.
	StateDependent
	
	// ContractCallDependent는 컨트랙트 호출 기반 의존성을 나타냅니다.
	ContractCallDependent
)

// TxDependency는 트랜잭션 간의 의존성을 나타냅니다.
type TxDependency struct {
	From      common.Address
	To        common.Address
	Type      TxDependencyType
	StateKeys []common.Hash
}

// TxDependencyGraph는 트랜잭션 의존성 그래프를 나타냅니다.
type TxDependencyGraph struct {
	// 트랜잭션 해시 -> 의존하는 트랜잭션 해시 목록
	Dependencies map[common.Hash][]common.Hash
	
	// 트랜잭션 해시 -> 의존성 유형 및 세부 정보
	DependencyDetails map[common.Hash]map[common.Hash]TxDependency
	
	// 트랜잭션 해시 -> 트랜잭션
	Transactions map[common.Hash]*types.Transaction
	
	// 트랜잭션 해시 -> 실행 순서
	ExecutionOrder map[common.Hash]int
}

// TxDependencyAnalyzer는 트랜잭션 의존성 분석을 담당합니다.
type TxDependencyAnalyzer struct {
	// 캐시
	dependencyCache *LRUCache
	
	// 통계
	analysisCount     uint64
	cacheHitCount     uint64
	cacheMissCount    uint64
	avgAnalysisTime   *metrics.GaugeFloat64
	dependencyRatio   *metrics.GaugeFloat64
	
	// 동기화
	mu                sync.RWMutex
	stopCh            chan struct{}
}

// NewTxDependencyAnalyzer는 새로운 트랜잭션 의존성 분석기를 생성합니다.
func NewTxDependencyAnalyzer() *TxDependencyAnalyzer {
	analyzer := &TxDependencyAnalyzer{
		dependencyCache:  NewLRUCache(defaultDependencyCacheSize, 0),
		avgAnalysisTime:  metrics.NewGaugeFloat64(),
		dependencyRatio:  metrics.NewGaugeFloat64(),
		stopCh:           make(chan struct{}),
	}
	
	// 메트릭스 등록
	metrics.Register("tx_dependency.avg_analysis_time", analyzer.avgAnalysisTime)
	metrics.Register("tx_dependency.dependency_ratio", analyzer.dependencyRatio)
	
	return analyzer
}

// Start는 트랜잭션 의존성 분석기를 시작합니다.
func (tda *TxDependencyAnalyzer) Start() error {
	tda.mu.Lock()
	defer tda.mu.Unlock()
	
	// 캐시 시작
	tda.dependencyCache.Start()
	
	// 메트릭스 수집 시작
	go tda.collectMetrics()
	
	log.Info("Transaction dependency analyzer started")
	return nil
}

// Stop은 트랜잭션 의존성 분석기를 중지합니다.
func (tda *TxDependencyAnalyzer) Stop() error {
	tda.mu.Lock()
	defer tda.mu.Unlock()
	
	// 중지 신호 전송
	close(tda.stopCh)
	
	// 캐시 중지
	tda.dependencyCache.Stop()
	
	// 메트릭스 등록 해제
	metrics.Unregister("tx_dependency.avg_analysis_time")
	metrics.Unregister("tx_dependency.dependency_ratio")
	
	log.Info("Transaction dependency analyzer stopped")
	return nil
}

// AnalyzeDependencies는 트랜잭션 목록의 의존성을 분석합니다.
func (tda *TxDependencyAnalyzer) AnalyzeDependencies(txs types.Transactions, state *state.StateDB) (*TxDependencyGraph, error) {
	startTime := time.Now()
	
	// 결과 그래프 초기화
	graph := &TxDependencyGraph{
		Dependencies:      make(map[common.Hash][]common.Hash),
		DependencyDetails: make(map[common.Hash]map[common.Hash]TxDependency),
		Transactions:      make(map[common.Hash]*types.Transaction),
		ExecutionOrder:    make(map[common.Hash]int),
	}
	
	// 트랜잭션이 없으면 빈 그래프 반환
	if len(txs) == 0 {
		return graph, nil
	}
	
	// 트랜잭션 맵 구성
	for _, tx := range txs {
		graph.Transactions[tx.Hash()] = tx
	}
	
	// 논스 기반 의존성 분석
	tda.analyzeNonceDependencies(txs, graph)
	
	// 상태 기반 의존성 분석
	if state != nil {
		tda.analyzeStateDependencies(txs, state, graph)
	}
	
	// 컨트랙트 호출 기반 의존성 분석
	tda.analyzeContractCallDependencies(txs, graph)
	
	// 실행 순서 결정
	tda.determineExecutionOrder(graph)
	
	// 통계 업데이트
	tda.mu.Lock()
	tda.analysisCount++
	analysisTime := time.Since(startTime)
	tda.mu.Unlock()
	
	log.Debug("Transaction dependency analysis completed", 
		"txCount", len(txs), 
		"dependencyCount", len(graph.Dependencies),
		"analysisTime", analysisTime)
	
	return graph, nil
}

// GetOptimalBatches는 의존성 그래프를 기반으로 최적의 병렬 실행 배치를 생성합니다.
func (tda *TxDependencyAnalyzer) GetOptimalBatches(graph *TxDependencyGraph) []types.Transactions {
	// 실행 순서별로 트랜잭션 그룹화
	orderGroups := make(map[int]types.Transactions)
	maxOrder := 0
	
	for txHash, order := range graph.ExecutionOrder {
		tx := graph.Transactions[txHash]
		orderGroups[order] = append(orderGroups[order], tx)
		if order > maxOrder {
			maxOrder = order
		}
	}
	
	// 순서대로 배치 생성
	batches := make([]types.Transactions, maxOrder+1)
	for order := 0; order <= maxOrder; order++ {
		if txs, ok := orderGroups[order]; ok {
			batches[order] = txs
		}
	}
	
	return batches
}

// GetDependencyStats는 의존성 통계를 반환합니다.
func (tda *TxDependencyAnalyzer) GetDependencyStats(graph *TxDependencyGraph) map[string]interface{} {
	txCount := len(graph.Transactions)
	dependentTxCount := len(graph.Dependencies)
	
	// 의존성 유형별 카운트
	nonceDepCount := 0
	stateDepCount := 0
	contractCallDepCount := 0
	
	for _, details := range graph.DependencyDetails {
		for _, dep := range details {
			switch dep.Type {
			case NonceDependent:
				nonceDepCount++
			case StateDependent:
				stateDepCount++
			case ContractCallDependent:
				contractCallDepCount++
			}
		}
	}
	
	// 최대 의존성 깊이 계산
	maxDepth := 0
	for _, order := range graph.ExecutionOrder {
		if order > maxDepth {
			maxDepth = order
		}
	}
	
	return map[string]interface{}{
		"tx_count":              txCount,
		"dependent_tx_count":    dependentTxCount,
		"dependency_ratio":      float64(dependentTxCount) / float64(txCount),
		"nonce_dependencies":    nonceDepCount,
		"state_dependencies":    stateDepCount,
		"contract_dependencies": contractCallDepCount,
		"max_depth":             maxDepth,
		"batch_count":           maxDepth + 1,
	}
}

// 내부 메서드

// analyzeNonceDependencies는 논스 기반 의존성을 분석합니다.
func (tda *TxDependencyAnalyzer) analyzeNonceDependencies(txs types.Transactions, graph *TxDependencyGraph) {
	// 발신자별 트랜잭션 그룹화
	senderTxs := make(map[common.Address]types.Transactions)
	
	for _, tx := range txs {
		sender, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
		if err != nil {
			continue
		}
		
		senderTxs[sender] = append(senderTxs[sender], tx)
	}
	
	// 각 발신자에 대해 논스 기반 의존성 분석
	for sender, senderTransactions := range senderTxs {
		// 논스 기준으로 정렬
		sort.Slice(senderTransactions, func(i, j int) bool {
			return senderTransactions[i].Nonce() < senderTransactions[j].Nonce()
		})
		
		// 의존성 설정 (각 트랜잭션은 이전 논스의 트랜잭션에 의존)
		for i := 1; i < len(senderTransactions); i++ {
			prevTx := senderTransactions[i-1]
			currentTx := senderTransactions[i]
			
			prevHash := prevTx.Hash()
			currentHash := currentTx.Hash()
			
			// 의존성 추가
			graph.Dependencies[currentHash] = append(graph.Dependencies[currentHash], prevHash)
			
			// 의존성 세부 정보 추가
			if _, ok := graph.DependencyDetails[currentHash]; !ok {
				graph.DependencyDetails[currentHash] = make(map[common.Hash]TxDependency)
			}
			
			graph.DependencyDetails[currentHash][prevHash] = TxDependency{
				From: sender,
				To:   sender,
				Type: NonceDependent,
			}
		}
	}
}

// analyzeStateDependencies는 상태 기반 의존성을 분석합니다.
func (tda *TxDependencyAnalyzer) analyzeStateDependencies(txs types.Transactions, state *state.StateDB, graph *TxDependencyGraph) {
	// 상태 접근 패턴 분석 (실제 구현에서는 더 복잡한 분석 필요)
	// 이 예제에서는 간단한 구현만 제공
	
	// 트랜잭션별 상태 키 접근 맵
	txStateAccess := make(map[common.Hash][]common.Hash)
	
	// 각 트랜잭션에 대해 상태 접근 패턴 분석
	for _, tx := range txs {
		txHash := tx.Hash()
		
		// 발신자 계정 상태 접근
		sender, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
		if err != nil {
			continue
		}
		
		// 발신자 잔액 키
		senderBalanceKey := common.BytesToHash(append(sender.Bytes(), []byte("balance")...))
		txStateAccess[txHash] = append(txStateAccess[txHash], senderBalanceKey)
		
		// 수신자가 있는 경우 수신자 계정 상태 접근
		if tx.To() != nil {
			// 수신자 잔액 키
			receiverBalanceKey := common.BytesToHash(append(tx.To().Bytes(), []byte("balance")...))
			txStateAccess[txHash] = append(txStateAccess[txHash], receiverBalanceKey)
		}
		
		// 컨트랙트 생성인 경우 코드 키 접근
		if tx.To() == nil {
			// 컨트랙트 코드 키 (예시)
			contractCodeKey := common.BytesToHash(append(sender.Bytes(), []byte("code")...))
			txStateAccess[txHash] = append(txStateAccess[txHash], contractCodeKey)
		}
	}
	
	// 상태 키 접근 기반 의존성 분석
	for i, tx1 := range txs {
		tx1Hash := tx1.Hash()
		
		for j, tx2 := range txs {
			// 같은 트랜잭션이거나 이미 처리된 쌍은 건너뜀
			if i >= j {
				continue
			}
			
			tx2Hash := tx2.Hash()
			
			// 상태 키 접근 충돌 확인
			for _, key1 := range txStateAccess[tx1Hash] {
				for _, key2 := range txStateAccess[tx2Hash] {
					if key1 == key2 {
						// 상태 키 충돌 발견, 의존성 추가
						// 트랜잭션 순서에 따라 의존성 방향 결정
						graph.Dependencies[tx2Hash] = append(graph.Dependencies[tx2Hash], tx1Hash)
						
						// 의존성 세부 정보 추가
						if _, ok := graph.DependencyDetails[tx2Hash]; !ok {
							graph.DependencyDetails[tx2Hash] = make(map[common.Hash]TxDependency)
						}
						
						sender1, _ := types.Sender(types.LatestSignerForChainID(tx1.ChainId()), tx1)
						sender2, _ := types.Sender(types.LatestSignerForChainID(tx2.ChainId()), tx2)
						
						graph.DependencyDetails[tx2Hash][tx1Hash] = TxDependency{
							From:      sender2,
							To:        sender1,
							Type:      StateDependent,
							StateKeys: []common.Hash{key1},
						}
						
						// 하나의 충돌만 찾으면 충분
						break
					}
				}
			}
		}
	}
}

// analyzeContractCallDependencies는 컨트랙트 호출 기반 의존성을 분석합니다.
func (tda *TxDependencyAnalyzer) analyzeContractCallDependencies(txs types.Transactions, graph *TxDependencyGraph) {
	// 컨트랙트 주소별 트랜잭션 맵
	contractTxs := make(map[common.Address][]common.Hash)
	
	// 각 트랜잭션에 대해 컨트랙트 호출 분석
	for _, tx := range txs {
		// 컨트랙트 호출인 경우
		if tx.To() != nil && len(tx.Data()) > 0 {
			contractAddr := *tx.To()
			contractTxs[contractAddr] = append(contractTxs[contractAddr], tx.Hash())
		}
	}
	
	// 같은 컨트랙트를 호출하는 트랜잭션 간의 의존성 분석
	for _, txHashes := range contractTxs {
		// 2개 이상의 트랜잭션이 같은 컨트랙트를 호출하는 경우
		if len(txHashes) < 2 {
			continue
		}
		
		// 트랜잭션 간의 의존성 설정 (순서대로)
		for i := 1; i < len(txHashes); i++ {
			prevTxHash := txHashes[i-1]
			currentTxHash := txHashes[i]
			
			prevTx := graph.Transactions[prevTxHash]
			currentTx := graph.Transactions[currentTxHash]
			
			// 의존성 추가
			graph.Dependencies[currentTxHash] = append(graph.Dependencies[currentTxHash], prevTxHash)
			
			// 의존성 세부 정보 추가
			if _, ok := graph.DependencyDetails[currentTxHash]; !ok {
				graph.DependencyDetails[currentTxHash] = make(map[common.Hash]TxDependency)
			}
			
			sender, _ := types.Sender(types.LatestSignerForChainID(currentTx.ChainId()), currentTx)
			
			graph.DependencyDetails[currentTxHash][prevTxHash] = TxDependency{
				From: sender,
				To:   *prevTx.To(),
				Type: ContractCallDependent,
			}
		}
	}
}

// determineExecutionOrder는 의존성 그래프를 기반으로 실행 순서를 결정합니다.
func (tda *TxDependencyAnalyzer) determineExecutionOrder(graph *TxDependencyGraph) {
	// 각 트랜잭션의 의존성 수 계산
	dependencyCounts := make(map[common.Hash]int)
	for txHash := range graph.Transactions {
		dependencyCounts[txHash] = len(graph.Dependencies[txHash])
	}
	
	// 의존성이 없는 트랜잭션 찾기
	var queue []common.Hash
	for txHash, count := range dependencyCounts {
		if count == 0 {
			queue = append(queue, txHash)
			graph.ExecutionOrder[txHash] = 0
		}
	}
	
	// 위상 정렬
	for len(queue) > 0 {
		// 큐에서 트랜잭션 꺼내기
		txHash := queue[0]
		queue = queue[1:]
		
		// 이 트랜잭션에 의존하는 트랜잭션 찾기
		for depTxHash, deps := range graph.Dependencies {
			for _, dep := range deps {
				if dep == txHash {
					// 의존성 카운트 감소
					dependencyCounts[depTxHash]--
					
					// 모든 의존성이 처리되었으면 큐에 추가
					if dependencyCounts[depTxHash] == 0 {
						queue = append(queue, depTxHash)
						
						// 실행 순서 설정 (의존하는 트랜잭션 중 가장 높은 순서 + 1)
						maxOrder := 0
						for _, depHash := range graph.Dependencies[depTxHash] {
							if order, ok := graph.ExecutionOrder[depHash]; ok && order > maxOrder {
								maxOrder = order
							}
						}
						graph.ExecutionOrder[depTxHash] = maxOrder + 1
					}
				}
			}
		}
	}
	
	// 순환 의존성이 있는 경우 처리 (실제 구현에서는 더 복잡한 처리 필요)
	for txHash := range graph.Transactions {
		if _, ok := graph.ExecutionOrder[txHash]; !ok {
			// 순환 의존성이 있는 트랜잭션에 임의의 높은 순서 할당
			graph.ExecutionOrder[txHash] = maxDependencyDepth
			log.Warn("Circular dependency detected", "txHash", txHash)
		}
	}
}

// collectMetrics는 주기적으로 메트릭스를 수집합니다.
func (tda *TxDependencyAnalyzer) collectMetrics() {
	ticker := time.NewTicker(txDependencyMetricsInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-tda.stopCh:
			return
		case <-ticker.C:
			tda.mu.RLock()
			totalAnalysis := tda.analysisCount
			tda.mu.RUnlock()
			
			// 캐시 통계
			cacheStats := tda.dependencyCache.GetStats()
			
			log.Debug("Transaction dependency analyzer metrics", 
				"analysisCount", totalAnalysis,
				"cacheHitRatio", cacheStats["hit_ratio"],
				"cacheSize", cacheStats["size"])
		}
	}
} 