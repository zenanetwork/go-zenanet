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
	"sync"
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

// setupStressTestEnvironment는 스트레스 테스트를 위한 환경을 설정합니다.
func setupStressTestEnvironment(t *testing.T) (*governance.API, *state.StateDB, ethdb.Database) {
	// 로거 설정
	log.New("module", "stress_test")
	
	// 메모리 데이터베이스 생성
	db := rawdb.NewMemoryDatabase()
	
	// 상태 DB 생성
	stateDB, _ := state.New(common.Hash{}, state.NewDatabaseForTesting())
	
	// 검증자 집합 생성
	validatorSet := NewMockValidatorSet()
	
	// 다수의 검증자 추가 (스트레스 테스트를 위해)
	for i := 0; i < 100; i++ {
		addr := common.BytesToAddress([]byte(fmt.Sprintf("validator%d", i)))
		validatorSet.AddValidator(&MockValidator{
			address:    addr,
			votingPower: big.NewInt(int64(100 - i)), // 다양한 투표력 부여
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

// TestConcurrentProposals는 다수의 제안을 동시에 처리하는 스트레스 테스트입니다.
// 이 테스트는 시스템이 동시에 여러 제안을 처리할 수 있는지 검증합니다.
func TestConcurrentProposals(t *testing.T) {
	// 테스트 환경 설정
	govAPI, _, _ := setupStressTestEnvironment(t)
	
	// 동시 처리할 제안 수
	numProposals := 50
	
	// 동기화를 위한 WaitGroup
	var wg sync.WaitGroup
	wg.Add(numProposals)
	
	// 결과 저장을 위한 맵 (동시성 제어를 위한 뮤텍스 포함)
	results := struct {
		sync.Mutex
		proposalIDs map[int]uint64
		errors      map[int]error
	}{
		proposalIDs: make(map[int]uint64),
		errors:      make(map[int]error),
	}
	
	// 시작 시간 기록
	startTime := time.Now()
	
	// 다수의 제안을 동시에 생성
	for i := 0; i < numProposals; i++ {
		go func(index int) {
			defer wg.Done()
			
			// 제안자 주소 생성 (각 고루틴마다 다른 주소 사용)
			proposer := common.BytesToAddress([]byte(fmt.Sprintf("proposer%d", index)))
			
			// 제안 내용 생성
			content := &TestProposalContent{
				ProposalType: utils.ProposalTypeText,
				Params:       make(map[string]string),
			}
			
			// 제안 제출
			submitArgs := governance.SubmitProposalArgs{
				Type:           utils.ProposalTypeText,
				Title:          fmt.Sprintf("스트레스 테스트 제안 %d", index),
				Description:    fmt.Sprintf("이 제안은 스트레스 테스트를 위한 제안 %d입니다.", index),
				Proposer:       proposer,
				InitialDeposit: "100",
				Content:        content,
			}
			
			// 제안 제출 및 결과 저장
			proposalID, err := govAPI.SubmitProposal(submitArgs)
			
			// 결과 저장 (뮤텍스로 보호)
			results.Lock()
			results.proposalIDs[index] = proposalID
			results.errors[index] = err
			results.Unlock()
		}(i)
	}
	
	// 모든 고루틴이 완료될 때까지 대기
	wg.Wait()
	
	// 종료 시간 기록 및 소요 시간 계산
	endTime := time.Now()
	duration := endTime.Sub(startTime)
	
	// 결과 분석
	successCount := 0
	errorCount := 0
	
	for i := 0; i < numProposals; i++ {
		if results.errors[i] == nil {
			successCount++
		} else {
			errorCount++
			t.Logf("제안 %d 생성 실패: %v", i, results.errors[i])
		}
	}
	
	// 결과 출력
	t.Logf("총 제안 수: %d", numProposals)
	t.Logf("성공한 제안 수: %d", successCount)
	t.Logf("실패한 제안 수: %d", errorCount)
	t.Logf("소요 시간: %v", duration)
	t.Logf("초당 처리 제안 수: %.2f", float64(successCount)/duration.Seconds())
	
	// 성공률 검증
	successRate := float64(successCount) / float64(numProposals)
	assert.True(t, successRate >= 0.9, "제안 생성 성공률이 90%% 이상이어야 함 (현재: %.2f%%)", successRate*100)
}

// TestHighVolumeVoting은 다수의 투표를 처리하는 스트레스 테스트입니다.
// 이 테스트는 시스템이 대량의 투표를 처리할 수 있는지 검증합니다.
func TestHighVolumeVoting(t *testing.T) {
	// 테스트 환경 설정
	govAPI, _, _ := setupStressTestEnvironment(t)
	
	// 테스트 제안 생성
	proposer := common.HexToAddress("0x1111111111111111111111111111111111111111")
	
	// 제안 내용 생성
	content := &TestProposalContent{
		ProposalType: utils.ProposalTypeText,
		Params:       make(map[string]string),
	}
	
	// 제안 제출
	submitArgs := governance.SubmitProposalArgs{
		Type:           utils.ProposalTypeText,
		Title:          "대량 투표 테스트 제안",
		Description:    "이 제안은 대량의 투표를 테스트하기 위한 제안입니다.",
		Proposer:       proposer,
		InitialDeposit: "100",
		Content:        content,
	}
	
	proposalID, err := govAPI.SubmitProposal(submitArgs)
	assert.NoError(t, err, "제안 제출 중 오류 발생")
	
	// 제안 상태를 투표 기간으로 수동 변경
	proposal, _ := govAPI.GetProposal(proposalID)
	if proposal.Status == utils.ProposalStatusDepositPeriod {
		// 이 부분은 실제 구현에 따라 다를 수 있음
		// 테스트 환경에서는 상태 변경을 수동으로 처리
		// 실제 환경에서는 블록 처리 과정에서 자동으로 처리됨
	}
	
	// 투표자 수
	numVoters := 100
	
	// 동기화를 위한 WaitGroup
	var wg sync.WaitGroup
	wg.Add(numVoters)
	
	// 결과 저장을 위한 맵 (동시성 제어를 위한 뮤텍스 포함)
	results := struct {
		sync.Mutex
		success map[int]bool
		errors  map[int]error
	}{
		success: make(map[int]bool),
		errors:  make(map[int]error),
	}
	
	// 시작 시간 기록
	startTime := time.Now()
	
	// 다수의 투표를 동시에 처리
	for i := 0; i < numVoters; i++ {
		go func(index int) {
			defer wg.Done()
			
			// 투표자 주소 생성 (각 고루틴마다 다른 주소 사용)
			voter := common.BytesToAddress([]byte(fmt.Sprintf("validator%d", index)))
			
			// 투표 옵션 결정 (인덱스에 따라 다른 옵션 선택)
			var option string
			switch index % 4 {
			case 0:
				option = utils.VoteOptionYes
			case 1:
				option = utils.VoteOptionNo
			case 2:
				option = utils.VoteOptionAbstain
			case 3:
				option = utils.VoteOptionVeto
			}
			
			// 투표 제출
			voteArgs := governance.VoteArgs{
				ProposalID: proposalID,
				Voter:      voter,
				Option:     option,
			}
			
			// 투표 제출 및 결과 저장
			success, err := govAPI.Vote(voteArgs)
			
			// 결과 저장 (뮤텍스로 보호)
			results.Lock()
			results.success[index] = success
			results.errors[index] = err
			results.Unlock()
		}(i)
	}
	
	// 모든 고루틴이 완료될 때까지 대기
	wg.Wait()
	
	// 종료 시간 기록 및 소요 시간 계산
	endTime := time.Now()
	duration := endTime.Sub(startTime)
	
	// 결과 분석
	successCount := 0
	errorCount := 0
	
	for i := 0; i < numVoters; i++ {
		if results.errors[i] == nil && results.success[i] {
			successCount++
		} else {
			errorCount++
			if results.errors[i] != nil {
				t.Logf("투표 %d 실패: %v", i, results.errors[i])
			}
		}
	}
	
	// 결과 출력
	t.Logf("총 투표 수: %d", numVoters)
	t.Logf("성공한 투표 수: %d", successCount)
	t.Logf("실패한 투표 수: %d", errorCount)
	t.Logf("소요 시간: %v", duration)
	t.Logf("초당 처리 투표 수: %.2f", float64(successCount)/duration.Seconds())
	
	// 성공률 검증
	successRate := float64(successCount) / float64(numVoters)
	assert.True(t, successRate >= 0.9, "투표 성공률이 90%% 이상이어야 함 (현재: %.2f%%)", successRate*100)
	
	// 제안 조회 및 투표 결과 확인
	proposal, err = govAPI.GetProposal(proposalID)
	assert.NoError(t, err, "제안 조회 중 오류 발생")
	
	// 투표 결과 출력
	t.Logf("찬성 투표 수: %s", proposal.YesVotes)
	t.Logf("반대 투표 수: %s", proposal.NoVotes)
	t.Logf("기권 투표 수: %s", proposal.AbstainVotes)
	t.Logf("거부권 투표 수: %s", proposal.VetoVotes)
}

// TestProposalProcessingPerformance는 제안 처리 성능을 측정하는 스트레스 테스트입니다.
// 이 테스트는 시스템이 다수의 제안을 효율적으로 처리할 수 있는지 검증합니다.
func TestProposalProcessingPerformance(t *testing.T) {
	// 테스트 환경 설정
	govAPI, _, _ := setupStressTestEnvironment(t)
	
	// 처리할 제안 수
	numProposals := 20
	
	// 제안 ID 저장을 위한 슬라이스
	proposalIDs := make([]uint64, numProposals)
	
	// 1. 다수의 제안 생성
	t.Log("1. 다수의 제안 생성")
	
	// 시작 시간 기록
	createStartTime := time.Now()
	
	for i := 0; i < numProposals; i++ {
		// 제안자 주소 생성
		proposer := common.BytesToAddress([]byte(fmt.Sprintf("proposer%d", i)))
		
		// 제안 내용 생성
		content := &TestProposalContent{
			ProposalType: utils.ProposalTypeText,
			Params:       make(map[string]string),
		}
		
		// 제안 제출
		submitArgs := governance.SubmitProposalArgs{
			Type:           utils.ProposalTypeText,
			Title:          fmt.Sprintf("성능 테스트 제안 %d", i),
			Description:    fmt.Sprintf("이 제안은 성능 테스트를 위한 제안 %d입니다.", i),
			Proposer:       proposer,
			InitialDeposit: "100",
			Content:        content,
		}
		
		// 제안 제출 및 ID 저장
		proposalID, err := govAPI.SubmitProposal(submitArgs)
		assert.NoError(t, err, "제안 %d 제출 중 오류 발생", i)
		proposalIDs[i] = proposalID
	}
	
	// 제안 생성 종료 시간 기록 및 소요 시간 계산
	createEndTime := time.Now()
	createDuration := createEndTime.Sub(createStartTime)
	t.Logf("제안 생성 소요 시간: %v (평균: %v/제안)", createDuration, createDuration/time.Duration(numProposals))
	
	// 2. 제안 조회 성능 측정
	t.Log("2. 제안 조회 성능 측정")
	
	// 시작 시간 기록
	queryStartTime := time.Now()
	
	for i := 0; i < numProposals; i++ {
		// 제안 조회
		proposal, err := govAPI.GetProposal(proposalIDs[i])
		assert.NoError(t, err, "제안 %d 조회 중 오류 발생", i)
		assert.NotNil(t, proposal, "제안 %d가 nil임", i)
	}
	
	// 제안 조회 종료 시간 기록 및 소요 시간 계산
	queryEndTime := time.Now()
	queryDuration := queryEndTime.Sub(queryStartTime)
	t.Logf("제안 조회 소요 시간: %v (평균: %v/제안)", queryDuration, queryDuration/time.Duration(numProposals))
	
	// 3. 제안 목록 조회 성능 측정
	t.Log("3. 제안 목록 조회 성능 측정")
	
	// 시작 시간 기록
	listStartTime := time.Now()
	
	// 제안 목록 조회
	proposals, err := govAPI.GetProposals()
	assert.NoError(t, err, "제안 목록 조회 중 오류 발생")
	assert.Equal(t, numProposals, len(proposals), "제안 목록 길이가 일치하지 않음")
	
	// 제안 목록 조회 종료 시간 기록 및 소요 시간 계산
	listEndTime := time.Now()
	listDuration := listEndTime.Sub(listStartTime)
	t.Logf("제안 목록 조회 소요 시간: %v", listDuration)
	
	// 4. 제안 상태별 조회 성능 측정
	t.Log("4. 제안 상태별 조회 성능 측정")
	
	// 시작 시간 기록
	statusStartTime := time.Now()
	
	// 제안 상태별 조회
	depositProposals, err := govAPI.GetProposalsByStatus(utils.ProposalStatusDepositPeriod)
	assert.NoError(t, err, "보증금 기간 제안 조회 중 오류 발생")
	
	// 제안 상태별 조회 종료 시간 기록 및 소요 시간 계산
	statusEndTime := time.Now()
	statusDuration := statusEndTime.Sub(statusStartTime)
	t.Logf("제안 상태별 조회 소요 시간: %v", statusDuration)
	t.Logf("보증금 기간 제안 수: %d", len(depositProposals))
	
	// 성능 지표 검증
	assert.True(t, createDuration/time.Duration(numProposals) < 100*time.Millisecond, "제안 생성 평균 시간이 100ms 미만이어야 함")
	assert.True(t, queryDuration/time.Duration(numProposals) < 10*time.Millisecond, "제안 조회 평균 시간이 10ms 미만이어야 함")
	assert.True(t, listDuration < 100*time.Millisecond, "제안 목록 조회 시간이 100ms 미만이어야 함")
	assert.True(t, statusDuration < 100*time.Millisecond, "제안 상태별 조회 시간이 100ms 미만이어야 함")
} 