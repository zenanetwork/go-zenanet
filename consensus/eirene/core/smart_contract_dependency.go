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
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/metrics"
	"github.com/zenanetwork/go-zenanet/params"
)

// 스마트 컨트랙트 의존성 분석 관련 상수
const (
	// 캐싱 관련
	defaultContractDependencyCacheSize = 2048
	
	// 분석 관련
	maxContractDependencyDepth = 5
	
	// 메트릭스 관련
	contractDependencyMetricsInterval = 1 * time.Minute
	
	// 함수 시그니처 관련
	functionSigLength = 4 // 함수 시그니처 길이 (바이트)
)

// ContractDependencyType은 스마트 컨트랙트 의존성 유형을 나타냅니다.
type ContractDependencyType int

const (
	// DirectCall은 직접 호출 의존성을 나타냅니다.
	DirectCall ContractDependencyType = iota
	
	// StateRead는 상태 읽기 의존성을 나타냅니다.
	StateRead
	
	// StateWrite는 상태 쓰기 의존성을 나타냅니다.
	StateWrite
	
	// DelegateCall은 위임 호출 의존성을 나타냅니다.
	DelegateCall
	
	// EventEmit은 이벤트 발생 의존성을 나타냅니다.
	EventEmit
)

// String은 ContractDependencyType의 문자열 표현을 반환합니다.
func (cdt ContractDependencyType) String() string {
	switch cdt {
	case DirectCall:
		return "DirectCall"
	case StateRead:
		return "StateRead"
	case StateWrite:
		return "StateWrite"
	case DelegateCall:
		return "DelegateCall"
	case EventEmit:
		return "EventEmit"
	default:
		return "Unknown"
	}
}

// ContractDependency는 스마트 컨트랙트 간의 의존성을 나타냅니다.
type ContractDependency struct {
	FromContract common.Address       // 호출 컨트랙트
	ToContract   common.Address       // 대상 컨트랙트
	FunctionSig  [4]byte              // 함수 시그니처
	Type         ContractDependencyType // 의존성 유형
	AccessedKeys []common.Hash        // 접근한 상태 키
}

// ContractDependencyGraph는 스마트 컨트랙트 의존성 그래프를 나타냅니다.
type ContractDependencyGraph struct {
	// 컨트랙트 주소 -> 의존성 목록
	Dependencies map[common.Address][]ContractDependency
	
	// 함수 시그니처 -> 의존성 목록
	FunctionDependencies map[[4]byte][]ContractDependency
	
	// 상태 키 -> 접근하는 컨트랙트 목록
	StateAccessors map[common.Hash][]common.Address
	
	// 락
	lock sync.RWMutex
}

// NewContractDependencyGraph는 새로운 스마트 컨트랙트 의존성 그래프를 생성합니다.
func NewContractDependencyGraph() *ContractDependencyGraph {
	return &ContractDependencyGraph{
		Dependencies:        make(map[common.Address][]ContractDependency),
		FunctionDependencies: make(map[[4]byte][]ContractDependency),
		StateAccessors:      make(map[common.Hash][]common.Address),
	}
}

// AddDependency는 의존성을 그래프에 추가합니다.
func (cdg *ContractDependencyGraph) AddDependency(dep ContractDependency) {
	cdg.lock.Lock()
	defer cdg.lock.Unlock()
	
	// 컨트랙트 의존성 추가
	cdg.Dependencies[dep.FromContract] = append(cdg.Dependencies[dep.FromContract], dep)
	
	// 함수 의존성 추가
	cdg.FunctionDependencies[dep.FunctionSig] = append(cdg.FunctionDependencies[dep.FunctionSig], dep)
	
	// 상태 접근자 추가
	for _, key := range dep.AccessedKeys {
		// 중복 방지
		exists := false
		for _, addr := range cdg.StateAccessors[key] {
			if addr == dep.FromContract {
				exists = true
				break
			}
		}
		
		if !exists {
			cdg.StateAccessors[key] = append(cdg.StateAccessors[key], dep.FromContract)
		}
	}
}

// GetDependencies는 특정 컨트랙트의 의존성을 반환합니다.
func (cdg *ContractDependencyGraph) GetDependencies(contract common.Address) []ContractDependency {
	cdg.lock.RLock()
	defer cdg.lock.RUnlock()
	
	return cdg.Dependencies[contract]
}

// GetFunctionDependencies는 특정 함수 시그니처의 의존성을 반환합니다.
func (cdg *ContractDependencyGraph) GetFunctionDependencies(funcSig [4]byte) []ContractDependency {
	cdg.lock.RLock()
	defer cdg.lock.RUnlock()
	
	return cdg.FunctionDependencies[funcSig]
}

// GetStateAccessors는 특정 상태 키에 접근하는 컨트랙트 목록을 반환합니다.
func (cdg *ContractDependencyGraph) GetStateAccessors(key common.Hash) []common.Address {
	cdg.lock.RLock()
	defer cdg.lock.RUnlock()
	
	return cdg.StateAccessors[key]
}

// SmartContractDependencyAnalyzer는 스마트 컨트랙트 호출 간 의존성을 분석합니다.
type SmartContractDependencyAnalyzer struct {
	// 의존성 그래프
	graph *ContractDependencyGraph
	
	// 캐시
	dependencyCache *LRUCache
	
	// 상태 DB
	stateDB *state.StateDB
	
	// 체인 설정
	chainConfig *params.ChainConfig
	
	// 메트릭스
	metrics struct {
		analysisTime        *metrics.Gauge
		dependencyCount     *metrics.Gauge
		cacheHitRate        *metrics.Gauge
		complexityScore     *metrics.Gauge
		parallelizableRatio *metrics.Gauge
	}
	
	// 로깅
	logger log.Logger
	
	// 락
	lock sync.RWMutex
}

// NewSmartContractDependencyAnalyzer는 새로운 스마트 컨트랙트 의존성 분석기를 생성합니다.
func NewSmartContractDependencyAnalyzer(chainConfig *params.ChainConfig) *SmartContractDependencyAnalyzer {
	analyzer := &SmartContractDependencyAnalyzer{
		graph:           NewContractDependencyGraph(),
		dependencyCache: NewLRUCache(defaultContractDependencyCacheSize, 0), // 두 번째 인자는 TTL (0은 무제한)
		chainConfig:     chainConfig,
		logger:          log.New("module", "contract-dependency"),
	}
	
	// 메트릭스 초기화
	analyzer.metrics.analysisTime = metrics.NewGauge()
	metrics.Register("eirene/contract_dependency/analysis_time", analyzer.metrics.analysisTime)
	
	analyzer.metrics.dependencyCount = metrics.NewGauge()
	metrics.Register("eirene/contract_dependency/dependency_count", analyzer.metrics.dependencyCount)
	
	analyzer.metrics.cacheHitRate = metrics.NewGauge()
	metrics.Register("eirene/contract_dependency/cache_hit_rate", analyzer.metrics.cacheHitRate)
	
	analyzer.metrics.complexityScore = metrics.NewGauge()
	metrics.Register("eirene/contract_dependency/complexity_score", analyzer.metrics.complexityScore)
	
	analyzer.metrics.parallelizableRatio = metrics.NewGauge()
	metrics.Register("eirene/contract_dependency/parallelizable_ratio", analyzer.metrics.parallelizableRatio)
	
	return analyzer
}

// SetStateDB는 상태 DB를 설정합니다.
func (scda *SmartContractDependencyAnalyzer) SetStateDB(stateDB *state.StateDB) {
	scda.lock.Lock()
	defer scda.lock.Unlock()
	
	scda.stateDB = stateDB
}

// AnalyzeTransaction은 트랜잭션의 스마트 컨트랙트 의존성을 분석합니다.
func (scda *SmartContractDependencyAnalyzer) AnalyzeTransaction(tx *types.Transaction) ([]ContractDependency, error) {
	scda.lock.RLock()
	defer scda.lock.RUnlock()
	
	// 상태 DB가 설정되지 않은 경우
	if scda.stateDB == nil {
		return nil, fmt.Errorf("state DB not set")
	}
	
	// 컨트랙트 호출이 아닌 경우
	if tx.To() == nil || len(tx.Data()) < functionSigLength {
		return nil, nil
	}
	
	// 캐시 확인
	cacheKey := tx.Hash().String() // Hash를 문자열로 변환
	if cached, ok := scda.dependencyCache.Get(cacheKey); ok {
		if deps, ok := cached.([]ContractDependency); ok {
			return deps, nil
		}
	}
	
	// 함수 시그니처 추출
	var funcSig [4]byte
	copy(funcSig[:], tx.Data()[:functionSigLength])
	
	// 발신자 주소 추출
	sender, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		return nil, err
	}
	
	// 의존성 분석 시작
	startTime := time.Now()
	
	// 실제 구현에서는 EVM 실행 결과를 분석하여 의존성을 추출해야 함
	// 여기서는 간단한 예시로 구현
	
	// 의존성 목록
	dependencies := []ContractDependency{}
	
	// 직접 호출 의존성 추가
	directCallDep := ContractDependency{
		FromContract: sender,
		ToContract:   *tx.To(),
		FunctionSig:  funcSig,
		Type:         DirectCall,
		AccessedKeys: []common.Hash{},
	}
	dependencies = append(dependencies, directCallDep)
	
	// 캐시에 저장 (크기는 1로 가정)
	scda.dependencyCache.Add(cacheKey, dependencies, 1)
	
	// 메트릭스 업데이트
	analysisTime := time.Since(startTime)
	scda.metrics.analysisTime.Update(analysisTime.Nanoseconds())
	scda.metrics.dependencyCount.Update(int64(len(dependencies)))
	
	return dependencies, nil
}

// AnalyzeTransactions은 여러 트랜잭션의 스마트 컨트랙트 의존성을 분석합니다.
func (scda *SmartContractDependencyAnalyzer) AnalyzeTransactions(txs types.Transactions) (*ContractDependencyGraph, error) {
	graph := NewContractDependencyGraph()
	
	// 각 트랜잭션에 대해 의존성 분석
	for _, tx := range txs {
		deps, err := scda.AnalyzeTransaction(tx)
		if err != nil {
			scda.logger.Warn("Failed to analyze transaction", "hash", tx.Hash(), "error", err)
			continue
		}
		
		// 의존성이 없는 경우
		if deps == nil || len(deps) == 0 {
			continue
		}
		
		// 의존성 그래프에 추가
		for _, dep := range deps {
			graph.AddDependency(dep)
		}
	}
	
	// 메트릭스 업데이트
	stats := scda.dependencyCache.GetStats()
	hitRatio, ok := stats["hit_ratio"].(float64)
	if ok {
		scda.metrics.cacheHitRate.Update(int64(hitRatio * 100))
	}
	
	// 복잡도 점수 계산
	complexityScore := scda.calculateComplexityScore(graph)
	scda.metrics.complexityScore.Update(int64(complexityScore))
	
	// 병렬화 가능 비율 계산
	parallelizableRatio := scda.calculateParallelizableRatio(graph)
	scda.metrics.parallelizableRatio.Update(int64(parallelizableRatio * 100))
	
	return graph, nil
}

// calculateComplexityScore는 의존성 그래프의 복잡도 점수를 계산합니다.
func (scda *SmartContractDependencyAnalyzer) calculateComplexityScore(graph *ContractDependencyGraph) float64 {
	// 의존성 수
	depCount := 0
	for _, deps := range graph.Dependencies {
		depCount += len(deps)
	}
	
	// 상태 접근 수
	stateAccessCount := len(graph.StateAccessors)
	
	// 복잡도 점수 계산
	// 의존성 수와 상태 접근 수를 가중치를 두어 합산
	return float64(depCount)*0.7 + float64(stateAccessCount)*0.3
}

// calculateParallelizableRatio는 병렬화 가능한 트랜잭션의 비율을 계산합니다.
func (scda *SmartContractDependencyAnalyzer) calculateParallelizableRatio(graph *ContractDependencyGraph) float64 {
	// 의존성이 없는 컨트랙트 수
	independentCount := 0
	totalCount := len(graph.Dependencies)
	
	if totalCount == 0 {
		return 1.0 // 의존성이 없으면 모두 병렬화 가능
	}
	
	for _, deps := range graph.Dependencies {
		if len(deps) == 0 {
			independentCount++
		}
	}
	
	return float64(independentCount) / float64(totalCount)
}

// GetDependencyGraph는 현재 의존성 그래프를 반환합니다.
func (scda *SmartContractDependencyAnalyzer) GetDependencyGraph() *ContractDependencyGraph {
	scda.lock.RLock()
	defer scda.lock.RUnlock()
	
	return scda.graph
}

// ExtractFunctionSignature는 데이터에서 함수 시그니처를 추출합니다.
func ExtractFunctionSignature(data []byte) ([4]byte, error) {
	if len(data) < functionSigLength {
		return [4]byte{}, fmt.Errorf("data too short for function signature")
	}
	
	var sig [4]byte
	copy(sig[:], data[:functionSigLength])
	return sig, nil
}

// GetFunctionSignatureString은 함수 시그니처의 문자열 표현을 반환합니다.
func GetFunctionSignatureString(sig [4]byte) string {
	return "0x" + hex.EncodeToString(sig[:])
}

// GetFunctionSignatureFromString은 문자열에서 함수 시그니처를 추출합니다.
func GetFunctionSignatureFromString(sigStr string) ([4]byte, error) {
	var sig [4]byte
	
	// "0x" 접두사 제거
	if len(sigStr) >= 2 && sigStr[:2] == "0x" {
		sigStr = sigStr[2:]
	}
	
	// 16진수 문자열 디코딩
	sigBytes, err := hex.DecodeString(sigStr)
	if err != nil {
		return sig, err
	}
	
	if len(sigBytes) < functionSigLength {
		return sig, fmt.Errorf("signature too short")
	}
	
	copy(sig[:], sigBytes[:functionSigLength])
	return sig, nil
} 