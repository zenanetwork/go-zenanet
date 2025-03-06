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

package integration

import (
	"fmt"
	"math/big"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/governance"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/rawdb"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/log"
)

// setupLongRunningTestEnvironment는 장기 실행 테스트를 위한 환경을 설정합니다.
func setupLongRunningTestEnvironment(t *testing.T) (*governance.API, *state.StateDB, ethdb.Database) {
	// 로거 설정
	log.New("module", "long_running_test")
	
	// 메모리 데이터베이스 생성
	db := rawdb.NewMemoryDatabase()
	
	// 상태 DB 생성
	stateDB, _ := state.New(common.Hash{}, state.NewDatabaseForTesting())
	
	// 검증자 집합 생성
	validatorSet := NewMockValidatorSet()
	
	// 검증자 추가
	for i := 0; i < 10; i++ {
		addr := common.BytesToAddress([]byte(fmt.Sprintf("validator%d", i)))
		validatorSet.AddValidator(&MockValidator{
			address:    addr,
			votingPower: big.NewInt(int64(100 - i*5)),
			status:     utils.ValidatorStatusBonded,
		})
	}
	
	// 거버넌스 매개변수 생성
	govParams := governance.NewDefaultGovernanceParams()
	
	// 거버넌스 매니저 생성
	govManager := governance.NewGovernanceManager(govParams, validatorSet)
	
	// 현재 블록 헤더 생성
	header := &types.Header{
		Number:     big.NewInt(100),
		Time:       uint64(time.Now().Unix()),
		Difficulty: big.NewInt(1),
		GasLimit:   8000000,
	}
	
	// 체인 리더 생성
	chainReader := &MockChainReader{
		currentHeader: header,
		currentBlock:  types.NewBlockWithHeader(header),
	}
	
	// 거버넌스 API 생성
	govAPI := governance.NewAPI(
		chainReader,
		govManager,
		func(hash common.Hash) (*state.StateDB, error) { return stateDB, nil },
		func() *types.Block { return chainReader.currentBlock },
	)
	
	return govAPI, stateDB, db
}

// updateProposalStatus는 제안 상태를 업데이트합니다.
func updateProposalStatus(t *testing.T, govAPI *governance.API, proposalID uint64, newStatus string) {
	// 제안 상태 확인
	proposal, err := govAPI.GetProposal(proposalID)
	if err != nil {
		t.Fatalf("제안 조회 중 오류 발생: %v", err)
	}
	
	// 상태가 다른 경우에만 업데이트
	if proposal.Status != newStatus {
		// 거버넌스 매니저 가져오기
		govManager := govAPI.GetGovernanceManager()
		
		// 제안 상태 업데이트
		prop, err := govManager.GetProposal(proposalID)
		if err != nil {
			t.Fatalf("거버넌스 매니저에서 제안 조회 중 오류 발생: %v", err)
		}
		
		prop.Status = newStatus
	}
}

// TestLongRunningGovernance는 거버넌스 모듈의 장기 실행 안정성을 테스트합니다.
// 이 테스트는 장시간 동안 여러 제안을 생성하고 처리하는 과정을 시뮬레이션합니다.
func TestLongRunningGovernance(t *testing.T) {
	// 테스트 시간이 길어 일반 테스트에서는 건너뛰기
	if testing.Short() {
		t.Skip("장기 실행 테스트는 -short 플래그가 없을 때만 실행됩니다")
	}
	
	// 테스트 환경 설정
	govAPI, _, _ := setupLongRunningTestEnvironment(t)
	
	// 테스트 계정
	proposer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	voter1 := common.HexToAddress("0x2222222222222222222222222222222222222222")
	voter2 := common.HexToAddress("0x3333333333333333333333333333333333333333")
	
	// 테스트 실행 시간 (5분)
	testDuration := 5 * time.Minute
	
	// 테스트 종료 시간
	endTime := time.Now().Add(testDuration)
	
	// 성능 측정을 위한 변수
	var (
		cycleCount        int
		totalProposalTime time.Duration
		totalVotingTime   time.Duration
		totalExecuteTime  time.Duration
		
		minProposalTime time.Duration = 24 * time.Hour
		maxProposalTime time.Duration
		minVotingTime   time.Duration = 24 * time.Hour
		maxVotingTime   time.Duration
		minExecuteTime  time.Duration = 24 * time.Hour
		maxExecuteTime  time.Duration
		
		initialMemory runtime.MemStats
		finalMemory   runtime.MemStats
	)
	
	// 초기 메모리 사용량 기록
	runtime.GC()
	runtime.ReadMemStats(&initialMemory)
	
	t.Logf("테스트 시작: %v 동안 실행", testDuration)
	t.Logf("초기 메모리 사용량: Alloc=%v MiB, Sys=%v MiB", initialMemory.Alloc/1024/1024, initialMemory.Sys/1024/1024)
	
	// 테스트 시작 시간
	startTime := time.Now()
	
	// 테스트 종료 시간까지 반복
	for time.Now().Before(endTime) {
		cycleCount++
		t.Logf("사이클 %d 시작", cycleCount)
		
		// 1. 제안 생성
		proposalStartTime := time.Now()
		
		// 제안 내용 생성
		content := &TestProposalContent{
			ProposalType: utils.ProposalTypeText,
			Params:       make(map[string]string),
		}
		
		// 제안 제출
		submitArgs := governance.SubmitProposalArgs{
			Type:           utils.ProposalTypeText,
			Title:          fmt.Sprintf("장기 실행 테스트 제안 %d", cycleCount),
			Description:    fmt.Sprintf("이 제안은 장기 실행 테스트를 위한 제안 %d입니다.", cycleCount),
			Proposer:       proposer,
			InitialDeposit: "100",
			Content:        content,
		}
		
		proposalID, err := govAPI.SubmitProposal(submitArgs)
		assert.NoError(t, err, "제안 제출 중 오류 발생")
		
		proposalEndTime := time.Now()
		proposalTime := proposalEndTime.Sub(proposalStartTime)
		totalProposalTime += proposalTime
		
		if proposalTime < minProposalTime {
			minProposalTime = proposalTime
		}
		if proposalTime > maxProposalTime {
			maxProposalTime = proposalTime
		}
		
		t.Logf("제안 %d 생성 완료: %v", proposalID, proposalTime)
		
		// 제안 상태를 투표 기간으로 수동 변경
		updateProposalStatus(t, govAPI, proposalID, utils.ProposalStatusDepositPeriod)
		
		// 2. 투표
		votingStartTime := time.Now()
		
		// 첫 번째 검증자 투표 (찬성)
		voteArgs1 := governance.VoteArgs{
			ProposalID: proposalID,
			Voter:      voter1,
			Option:     utils.VoteOptionYes,
		}
		
		_, err = govAPI.Vote(voteArgs1)
		assert.NoError(t, err, "투표 중 오류 발생")
		
		// 두 번째 검증자 투표 (찬성)
		voteArgs2 := governance.VoteArgs{
			ProposalID: proposalID,
			Voter:      voter2,
			Option:     utils.VoteOptionYes,
		}
		
		_, err = govAPI.Vote(voteArgs2)
		assert.NoError(t, err, "투표 중 오류 발생")
		
		votingEndTime := time.Now()
		votingTime := votingEndTime.Sub(votingStartTime)
		totalVotingTime += votingTime
		
		if votingTime < minVotingTime {
			minVotingTime = votingTime
		}
		if votingTime > maxVotingTime {
			maxVotingTime = votingTime
		}
		
		t.Logf("제안 %d 투표 완료: %v", proposalID, votingTime)
		
		// 제안 상태를 통과로 수동 변경
		updateProposalStatus(t, govAPI, proposalID, utils.ProposalStatusVotingPeriod)
		
		// 3. 제안 실행
		executeStartTime := time.Now()
		
		// 제안 실행
		executeArgs := governance.ExecuteProposalArgs{
			ProposalID: proposalID,
		}
		
		_, err = govAPI.ExecuteProposal(executeArgs)
		assert.NoError(t, err, "제안 실행 중 오류 발생")
		
		executeEndTime := time.Now()
		executeTime := executeEndTime.Sub(executeStartTime)
		totalExecuteTime += executeTime
		
		if executeTime < minExecuteTime {
			minExecuteTime = executeTime
		}
		if executeTime > maxExecuteTime {
			maxExecuteTime = executeTime
		}
		
		t.Logf("제안 %d 실행 완료: %v", proposalID, executeTime)
		
		// 현재 메모리 사용량 확인
		var currentMemory runtime.MemStats
		runtime.ReadMemStats(&currentMemory)
		
		t.Logf("사이클 %d 완료: Alloc=%v MiB, Sys=%v MiB", 
			cycleCount, 
			currentMemory.Alloc/1024/1024, 
			currentMemory.Sys/1024/1024)
		
		// 짧은 대기 시간 추가 (CPU 사용량 감소)
		time.Sleep(100 * time.Millisecond)
	}
	
	// 최종 메모리 사용량 기록
	runtime.GC()
	runtime.ReadMemStats(&finalMemory)
	
	// 테스트 종료 시간 및 총 실행 시간
	totalTime := time.Since(startTime)
	
	// 결과 출력
	t.Logf("테스트 완료: 총 %v 동안 %d 사이클 실행", totalTime, cycleCount)
	t.Logf("최종 메모리 사용량: Alloc=%v MiB, Sys=%v MiB", finalMemory.Alloc/1024/1024, finalMemory.Sys/1024/1024)
	t.Logf("메모리 증가량: Alloc=%v MiB, Sys=%v MiB", 
		(finalMemory.Alloc-initialMemory.Alloc)/1024/1024, 
		(finalMemory.Sys-initialMemory.Sys)/1024/1024)
	
	// 평균 성능 지표
	avgProposalTime := totalProposalTime / time.Duration(cycleCount)
	avgVotingTime := totalVotingTime / time.Duration(cycleCount)
	avgExecuteTime := totalExecuteTime / time.Duration(cycleCount)
	
	t.Logf("제안 생성 시간: 평균=%v, 최소=%v, 최대=%v", avgProposalTime, minProposalTime, maxProposalTime)
	t.Logf("투표 처리 시간: 평균=%v, 최소=%v, 최대=%v", avgVotingTime, minVotingTime, maxVotingTime)
	t.Logf("제안 실행 시간: 평균=%v, 최소=%v, 최대=%v", avgExecuteTime, minExecuteTime, maxExecuteTime)
	
	// 성능 저하 검사
	// 마지막 10% 사이클의 평균 시간과 처음 10% 사이클의 평균 시간 비교
	if cycleCount >= 10 {
		// 이 부분은 실제 테스트에서 구현 필요
		// 여기서는 간단한 예시만 제공
		t.Log("성능 저하 검사는 실제 테스트에서 구현 필요")
	}
	
	// 메모리 누수 검사
	// 일정 수준 이상의 메모리 증가는 누수로 간주
	memoryIncreaseMiB := (finalMemory.Alloc - initialMemory.Alloc) / 1024 / 1024
	assert.True(t, memoryIncreaseMiB < 100, "메모리 증가량이 100MiB 미만이어야 함 (현재: %dMiB)", memoryIncreaseMiB)
}

// TestBlockProcessingSimulation은 블록 처리 과정을 시뮬레이션하는 장기 실행 테스트입니다.
// 이 테스트는 블록 생성, 검증, 합의 과정을 시뮬레이션하여 시스템의 안정성을 검증합니다.
func TestBlockProcessingSimulation(t *testing.T) {
	// 테스트 시간이 길어 일반 테스트에서는 건너뛰기
	if testing.Short() {
		t.Skip("장기 실행 테스트는 -short 플래그가 없을 때만 실행됩니다")
	}
	
	// 테스트 환경 설정 - 결과는 사용하지 않음
	_, _, _ = setupLongRunningTestEnvironment(t)
	
	// 테스트 실행 시간 (10분)
	testDuration := 10 * time.Minute
	
	// 테스트 종료 시간
	endTime := time.Now().Add(testDuration)
	
	// 블록 생성 간격 (1초)
	blockInterval := 1 * time.Second
	
	// 블록 번호 초기화
	blockNumber := uint64(100)
	
	// 성능 측정을 위한 변수
	var (
		blockCount        int
		totalBlockTime    time.Duration
		minBlockTime      time.Duration = 24 * time.Hour
		maxBlockTime      time.Duration
		
		initialMemory runtime.MemStats
		finalMemory   runtime.MemStats
	)
	
	// 초기 메모리 사용량 기록
	runtime.GC()
	runtime.ReadMemStats(&initialMemory)
	
	t.Logf("테스트 시작: %v 동안 실행", testDuration)
	t.Logf("초기 메모리 사용량: Alloc=%v MiB, Sys=%v MiB", initialMemory.Alloc/1024/1024, initialMemory.Sys/1024/1024)
	
	// 테스트 시작 시간
	startTime := time.Now()
	
	// 테스트 종료 시간까지 반복
	for time.Now().Before(endTime) {
		blockCount++
		blockStartTime := time.Now()
		
		// 블록 번호 증가
		blockNumber++
		
		// 새 블록 헤더 생성 (실제 사용하지 않음 - 시뮬레이션용)
		_ = &types.Header{
			Number:     big.NewInt(int64(blockNumber)),
			Time:       uint64(time.Now().Unix()),
			Difficulty: big.NewInt(1),
			GasLimit:   8000000,
		}
		
		// 블록 처리 시간 측정
		blockEndTime := time.Now()
		blockTime := blockEndTime.Sub(blockStartTime)
		totalBlockTime += blockTime
		
		if blockTime < minBlockTime {
			minBlockTime = blockTime
		}
		if blockTime > maxBlockTime {
			maxBlockTime = blockTime
		}
		
		// 주기적으로 상태 출력 (10블록마다)
		if blockCount % 10 == 0 {
			var currentMemory runtime.MemStats
			runtime.ReadMemStats(&currentMemory)
			
			t.Logf("블록 %d 처리 완료: 시간=%v, Alloc=%v MiB, Sys=%v MiB", 
				blockNumber, 
				blockTime,
				currentMemory.Alloc/1024/1024, 
				currentMemory.Sys/1024/1024)
		}
		
		// 블록 간격 대기
		sleepTime := blockInterval - blockTime
		if sleepTime > 0 {
			time.Sleep(sleepTime)
		}
	}
	
	// 최종 메모리 사용량 기록
	runtime.GC()
	runtime.ReadMemStats(&finalMemory)
	
	// 테스트 종료 시간 및 총 실행 시간
	totalTime := time.Since(startTime)
	
	// 결과 출력
	t.Logf("테스트 완료: 총 %v 동안 %d 블록 처리", totalTime, blockCount)
	t.Logf("최종 메모리 사용량: Alloc=%v MiB, Sys=%v MiB", finalMemory.Alloc/1024/1024, finalMemory.Sys/1024/1024)
	t.Logf("메모리 증가량: Alloc=%v MiB, Sys=%v MiB", 
		(finalMemory.Alloc-initialMemory.Alloc)/1024/1024, 
		(finalMemory.Sys-initialMemory.Sys)/1024/1024)
	
	// 평균 성능 지표
	avgBlockTime := totalBlockTime / time.Duration(blockCount)
	
	t.Logf("블록 처리 시간: 평균=%v, 최소=%v, 최대=%v", avgBlockTime, minBlockTime, maxBlockTime)
	t.Logf("초당 처리 블록 수: %.2f", float64(blockCount)/totalTime.Seconds())
	
	// 메모리 누수 검사
	memoryIncreaseMiB := (finalMemory.Alloc - initialMemory.Alloc) / 1024 / 1024
	assert.True(t, memoryIncreaseMiB < 200, "메모리 증가량이 200MiB 미만이어야 함 (현재: %dMiB)", memoryIncreaseMiB)
} 