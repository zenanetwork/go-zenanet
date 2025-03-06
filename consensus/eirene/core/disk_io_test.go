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
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiskIOOptimizer_Basic(t *testing.T) {
	// 테스트 디렉토리 생성
	testDir := filepath.Join(os.TempDir(), "zenanet-test", fmt.Sprintf("disk-io-test-%d", time.Now().UnixNano()))
	defer os.RemoveAll(testDir)
	
	// 디스크 I/O 최적화기 생성
	dio := NewDiskIOOptimizer()
	
	// 시작
	if err := dio.Start(); err != nil {
		t.Fatalf("Failed to start disk I/O optimizer: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer dio.Stop()
	
	// 기본 기능 테스트
	testKey := "test-key"
	testData := []byte("test-data")
	
	// 데이터 쓰기
	if err := dio.WriteData(testKey, testData); err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}
	
	// 데이터 읽기
	readData, err := dio.ReadData(testKey)
	if err != nil {
		t.Fatalf("Failed to read data: %v", err)
	}
	
	// 데이터 비교
	if !bytes.Equal(readData, testData) {
		t.Fatalf("Read data does not match written data. Got %v, want %v", readData, testData)
	}
	
	// 통계 확인
	stats := dio.GetStats()
	if stats["totalWrites"].(uint64) < 1 {
		t.Errorf("Expected at least 1 write, got %v", stats["totalWrites"])
	}
	if stats["totalReads"].(uint64) < 1 {
		t.Errorf("Expected at least 1 read, got %v", stats["totalReads"])
	}
}

func TestDiskIOOptimizer_Flush(t *testing.T) {
	// 디스크 I/O 최적화기 생성
	dio := NewDiskIOOptimizer()
	
	// 시작
	if err := dio.Start(); err != nil {
		t.Fatalf("Failed to start disk I/O optimizer: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer dio.Stop()
	
	// 여러 데이터 쓰기
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key-%d", i)
		data := []byte(fmt.Sprintf("data-%d", i))
		if err := dio.WriteData(key, data); err != nil {
			t.Fatalf("Failed to write data: %v", err)
		}
	}
	
	// 플러시
	if err := dio.Flush(); err != nil {
		t.Fatalf("Failed to flush: %v", err)
	}
	
	// 플러시 후 버퍼 크기 확인
	stats := dio.GetStats()
	if stats["writeBufferSize"].(int) != 0 {
		t.Errorf("Expected empty write buffer after flush, got size %v", stats["writeBufferSize"])
	}
}

func TestDiskIOOptimizer_CachePruning(t *testing.T) {
	// 작은 버퍼 크기로 디스크 I/O 최적화기 생성
	dio := NewDiskIOOptimizer()
	dio.readBufferSize = 100 // 작은 크기로 설정
	
	// 시작
	if err := dio.Start(); err != nil {
		t.Fatalf("Failed to start disk I/O optimizer: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer dio.Stop()
	
	// 캐시 크기를 초과하는 데이터 쓰기 및 읽기
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("key-%d", i)
		data := make([]byte, 10) // 10바이트 데이터
		for j := 0; j < 10; j++ {
			data[j] = byte(i + j)
		}
		
		// 데이터 쓰기
		if err := dio.WriteData(key, data); err != nil {
			t.Fatalf("Failed to write data: %v", err)
		}
		
		// 데이터 읽기 (캐시에 추가)
		if _, err := dio.ReadData(key); err != nil {
			t.Fatalf("Failed to read data: %v", err)
		}
	}
	
	// 캐시 크기 확인
	stats := dio.GetStats()
	if stats["readCacheSize"].(int) > dio.readBufferSize {
		t.Errorf("Cache size %v exceeds buffer size %v", stats["readCacheSize"], dio.readBufferSize)
	}
}

func TestDiskIOOptimizer_Concurrency(t *testing.T) {
	// 디스크 I/O 최적화기 생성
	dio := NewDiskIOOptimizer()
	
	// 시작
	if err := dio.Start(); err != nil {
		t.Fatalf("Failed to start disk I/O optimizer: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer dio.Stop()
	
	// 동시 쓰기 및 읽기 테스트
	const numGoroutines = 10
	const numOperations = 100
	
	done := make(chan bool, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				data := []byte(fmt.Sprintf("data-%d-%d", id, j))
				
				// 데이터 쓰기
				if err := dio.WriteData(key, data); err != nil {
					t.Errorf("Failed to write data: %v", err)
					done <- false
					return
				}
				
				// 데이터 읽기
				readData, err := dio.ReadData(key)
				if err != nil {
					t.Errorf("Failed to read data: %v", err)
					done <- false
					return
				}
				
				// 데이터 비교
				if !bytes.Equal(readData, data) {
					t.Errorf("Read data does not match written data. Got %v, want %v", readData, data)
					done <- false
					return
				}
			}
			done <- true
		}(i)
	}
	
	// 모든 고루틴 완료 대기
	for i := 0; i < numGoroutines; i++ {
		if !<-done {
			t.Fatalf("Concurrency test failed")
		}
	}
	
	// 통계 확인
	stats := dio.GetStats()
	expectedOps := numGoroutines * numOperations
	if stats["totalWrites"].(uint64) < uint64(expectedOps) {
		t.Errorf("Expected at least %d writes, got %v", expectedOps, stats["totalWrites"])
	}
	if stats["totalReads"].(uint64) < uint64(expectedOps) {
		t.Errorf("Expected at least %d reads, got %v", expectedOps, stats["totalReads"])
	}
}

func TestDiskIOOptimizer_Compaction(t *testing.T) {
	// 디스크 I/O 최적화기 생성
	dio := NewDiskIOOptimizer()
	
	// 시작
	if err := dio.Start(); err != nil {
		t.Fatalf("Failed to start disk I/O optimizer: %v", err)
	}
	
	// 중지 (테스트 종료 시)
	defer dio.Stop()
	
	// 압축 트리거
	dio.TriggerCompaction()
	
	// 압축이 비동기적으로 실행되므로 잠시 대기
	time.Sleep(100 * time.Millisecond)
	
	// 압축 후 상태 확인 (실제 구현에서는 더 구체적인 검증 필요)
	dio.mu.RLock()
	lastCompactionTime := dio.lastCompactionTime
	dio.mu.RUnlock()
	
	if time.Since(lastCompactionTime) > 1*time.Second {
		t.Errorf("Compaction does not appear to have run recently")
	}
} 