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
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAdaptiveWorkerPool_Basic(t *testing.T) {
	// 워커 풀 생성
	pool := NewAdaptiveWorkerPool("test-pool", 4, 100)
	
	// 시작
	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer pool.Stop()
	
	// 기본 기능 테스트
	var counter int32
	
	// 작업 제출
	for i := 0; i < 10; i++ {
		err := pool.Submit(func() error {
			atomic.AddInt32(&counter, 1)
			return nil
		})
		
		if err != nil {
			t.Fatalf("Failed to submit task: %v", err)
		}
	}
	
	// 작업 완료 대기
	time.Sleep(100 * time.Millisecond)
	
	// 카운터 확인
	if atomic.LoadInt32(&counter) != 10 {
		t.Errorf("Expected counter to be 10, got %d", counter)
	}
	
	// 통계 확인
	stats := pool.GetStats()
	if stats["tasks_processed"].(int64) != 10 {
		t.Errorf("Expected 10 tasks processed, got %v", stats["tasks_processed"])
	}
}

func TestAdaptiveWorkerPool_SubmitWait(t *testing.T) {
	// 워커 풀 생성
	pool := NewAdaptiveWorkerPool("test-pool", 4, 100)
	
	// 시작
	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer pool.Stop()
	
	// SubmitWait 테스트
	result := 0
	
	err := pool.SubmitWait(func() error {
		// 작업 수행
		result = 42
		return nil
	})
	
	if err != nil {
		t.Fatalf("SubmitWait failed: %v", err)
	}
	
	// 결과 확인
	if result != 42 {
		t.Errorf("Expected result to be 42, got %d", result)
	}
}

func TestAdaptiveWorkerPool_ErrorHandling(t *testing.T) {
	// 워커 풀 생성
	pool := NewAdaptiveWorkerPool("test-pool", 4, 100)
	
	// 시작
	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer pool.Stop()
	
	// 오류 반환 작업 제출
	expectedErr := errors.New("test error")
	
	err := pool.SubmitWait(func() error {
		return expectedErr
	})
	
	// 오류 확인
	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
	
	// 통계 확인
	time.Sleep(10 * time.Millisecond) // 통계 업데이트 대기
	stats := pool.GetStats()
	if stats["tasks_errors"].(int64) != 1 {
		t.Errorf("Expected 1 task error, got %v", stats["tasks_errors"])
	}
}

func TestAdaptiveWorkerPool_QueueFull(t *testing.T) {
	// 작은 큐 크기로 워커 풀 생성
	pool := NewAdaptiveWorkerPool("test-pool", 1, 5)
	
	// 시작
	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer pool.Stop()
	
	// 큐를 가득 채우는 작업 제출
	var wg sync.WaitGroup
	wg.Add(1)
	
	// 첫 번째 작업은 완료 신호를 받을 때까지 대기
	err := pool.Submit(func() error {
		wg.Wait()
		return nil
	})
	
	if err != nil {
		t.Fatalf("Failed to submit first task: %v", err)
	}
	
	// 큐를 가득 채우는 작업 제출
	for i := 0; i < 5; i++ {
		err := pool.Submit(func() error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})
		
		if err != nil {
			t.Fatalf("Failed to submit task %d: %v", i, err)
		}
	}
	
	// 큐가 가득 찼을 때 작업 제출
	err = pool.Submit(func() error {
		return nil
	})
	
	// 큐가 가득 찼는지 확인
	if err == nil {
		t.Errorf("Expected error when submitting to full queue, but got none")
	}
	
	// 첫 번째 작업 완료 신호
	wg.Done()
}

func TestAdaptiveWorkerPool_WorkerScaling(t *testing.T) {
	// 워커 풀 생성 (조정 간격을 짧게 설정)
	pool := NewAdaptiveWorkerPool("test-pool", 2, 1000)
	pool.adjustInterval = 100 * time.Millisecond
	
	// 시작
	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer pool.Stop()
	
	// 초기 워커 수 확인
	initialWorkers := pool.WorkerCount()
	if initialWorkers != 2 {
		t.Errorf("Expected initial worker count to be 2, got %d", initialWorkers)
	}
	
	// 많은 작업 제출하여 워커 수 증가 유도
	for i := 0; i < 100; i++ {
		err := pool.Submit(func() error {
			time.Sleep(50 * time.Millisecond)
			return nil
		})
		
		if err != nil {
			t.Fatalf("Failed to submit task: %v", err)
		}
	}
	
	// 워커 수 조정 대기
	time.Sleep(300 * time.Millisecond)
	
	// 워커 수 증가 확인
	scaledUpWorkers := pool.WorkerCount()
	if scaledUpWorkers <= initialWorkers {
		t.Errorf("Expected worker count to increase from %d, but got %d", initialWorkers, scaledUpWorkers)
	}
	
	// 모든 작업 완료 대기
	time.Sleep(1 * time.Second)
	
	// 워커 수 감소 대기
	time.Sleep(500 * time.Millisecond)
	
	// 워커 수 감소 확인 (항상 성공하지 않을 수 있음)
	scaledDownWorkers := pool.WorkerCount()
	if scaledDownWorkers >= scaledUpWorkers {
		t.Logf("Worker count did not decrease as expected: %d -> %d", scaledUpWorkers, scaledDownWorkers)
	}
}

func TestAdaptiveWorkerPool_Concurrency(t *testing.T) {
	// 워커 풀 생성
	pool := NewAdaptiveWorkerPool("test-pool", 4, 1000)
	
	// 시작
	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer pool.Stop()
	
	// 동시 작업 제출 테스트
	const numGoroutines = 10
	const numTasks = 100
	
	var counter int32
	var wg sync.WaitGroup
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			for j := 0; j < numTasks; j++ {
				err := pool.Submit(func() error {
					atomic.AddInt32(&counter, 1)
					return nil
				})
				
				if err != nil {
					t.Errorf("Failed to submit task: %v", err)
					return
				}
			}
		}(i)
	}
	
	// 모든 고루틴 완료 대기
	wg.Wait()
	
	// 모든 작업 완료 대기
	time.Sleep(500 * time.Millisecond)
	
	// 카운터 확인
	expectedCount := int32(numGoroutines * numTasks)
	if atomic.LoadInt32(&counter) != expectedCount {
		t.Errorf("Expected counter to be %d, got %d", expectedCount, counter)
	}
	
	// 통계 확인
	stats := pool.GetStats()
	if stats["tasks_processed"].(int64) != int64(expectedCount) {
		t.Errorf("Expected %d tasks processed, got %v", expectedCount, stats["tasks_processed"])
	}
}

func TestAdaptiveWorkerPool_StopWithPendingTasks(t *testing.T) {
	// 워커 풀 생성
	pool := NewAdaptiveWorkerPool("test-pool", 1, 100)
	
	// 시작
	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	
	// 많은 작업 제출
	for i := 0; i < 50; i++ {
		err := pool.Submit(func() error {
			time.Sleep(10 * time.Millisecond)
			return nil
		})
		
		if err != nil {
			t.Fatalf("Failed to submit task: %v", err)
		}
	}
	
	// 즉시 중지
	pool.Stop()
	
	// 중지 후 작업 제출 시도
	err := pool.Submit(func() error {
		return nil
	})
	
	// 중지된 풀에 작업 제출 시 오류 확인
	if err == nil {
		t.Errorf("Expected error when submitting to stopped pool, but got none")
	}
}

func TestAdaptiveWorkerPool_ActiveWorkerCount(t *testing.T) {
	// 워커 풀 생성
	pool := NewAdaptiveWorkerPool("test-pool", 4, 100)
	
	// 시작
	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer pool.Stop()
	
	// 초기 활성 워커 수 확인
	initialActive := pool.ActiveWorkerCount()
	if initialActive != 0 {
		t.Errorf("Expected initial active worker count to be 0, got %d", initialActive)
	}
	
	// 동시에 실행되는 작업 제출
	var wg sync.WaitGroup
	wg.Add(3)
	
	for i := 0; i < 3; i++ {
		err := pool.Submit(func() error {
			defer wg.Done()
			time.Sleep(100 * time.Millisecond)
			return nil
		})
		
		if err != nil {
			t.Fatalf("Failed to submit task: %v", err)
		}
	}
	
	// 활성 워커 수 확인
	time.Sleep(50 * time.Millisecond)
	activeWorkers := pool.ActiveWorkerCount()
	if activeWorkers != 3 {
		t.Errorf("Expected active worker count to be 3, got %d", activeWorkers)
	}
	
	// 작업 완료 대기
	wg.Wait()
	time.Sleep(50 * time.Millisecond)
	
	// 작업 완료 후 활성 워커 수 확인
	finalActive := pool.ActiveWorkerCount()
	if finalActive != 0 {
		t.Errorf("Expected final active worker count to be 0, got %d", finalActive)
	}
}

func TestAdaptiveWorkerPool_HighLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping high load test in short mode")
	}
	
	// 워커 풀 생성
	pool := NewAdaptiveWorkerPool("test-pool", 4, 10000)
	
	// 시작
	if err := pool.Start(); err != nil {
		t.Fatalf("Failed to start worker pool: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer pool.Stop()
	
	// 높은 부하 테스트
	const numTasks = 10000
	
	var counter int32
	var errors int32
	
	// 많은 작업 제출
	for i := 0; i < numTasks; i++ {
		taskID := i
		err := pool.Submit(func() error {
			// 간헐적 오류 시뮬레이션
			if taskID%100 == 0 {
				atomic.AddInt32(&errors, 1)
				return fmt.Errorf("simulated error for task %d", taskID)
			}
			
			atomic.AddInt32(&counter, 1)
			return nil
		})
		
		if err != nil {
			t.Fatalf("Failed to submit task: %v", err)
		}
	}
	
	// 모든 작업 완료 대기
	time.Sleep(2 * time.Second)
	
	// 결과 확인
	successCount := atomic.LoadInt32(&counter)
	errorCount := atomic.LoadInt32(&errors)
	
	if successCount+errorCount != numTasks {
		t.Errorf("Expected %d tasks to complete, got %d successes and %d errors", numTasks, successCount, errorCount)
	}
	
	// 통계 확인
	stats := pool.GetStats()
	if stats["tasks_processed"].(int64) != int64(numTasks) {
		t.Errorf("Expected %d tasks processed, got %v", numTasks, stats["tasks_processed"])
	}
	if stats["tasks_errors"].(int64) != int64(errorCount) {
		t.Errorf("Expected %d task errors, got %v", errorCount, stats["tasks_errors"])
	}
} 