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
	"fmt"
	"sync"
	"testing"
)

func TestLRUCache_Basic(t *testing.T) {
	// 캐시 생성
	cache := NewLRUCache(100, 0)
	
	// 시작
	cache.Start()
	
	// 중지 (테스트 종료 시)
	defer cache.Stop()
	
	// 기본 기능 테스트
	testKey := "test-key"
	testValue := "test-value"
	
	// 항목 추가
	if err := cache.Add(testKey, testValue, 1); err != nil {
		t.Fatalf("Failed to add item to cache: %v", err)
	}
	
	// 항목 조회
	value, ok := cache.Get(testKey)
	if !ok {
		t.Fatalf("Failed to get item from cache")
	}
	
	// 값 비교
	if value != testValue {
		t.Fatalf("Retrieved value does not match added value. Got %v, want %v", value, testValue)
	}
	
	// 통계 확인
	stats := cache.GetStats()
	if stats["hits"].(uint64) != 1 {
		t.Errorf("Expected 1 hit, got %v", stats["hits"])
	}
	if stats["misses"].(uint64) != 0 {
		t.Errorf("Expected 0 misses, got %v", stats["misses"])
	}
}

func TestLRUCache_Eviction(t *testing.T) {
	// 작은 용량의 캐시 생성
	capacity := 10
	cache := NewLRUCache(capacity, 0)
	
	// 시작
	cache.Start()
	
	// 중지 (테스트 종료 시)
	defer cache.Stop()
	
	// 제거 콜백 설정
	evicted := make(map[string]interface{})
	evictMutex := sync.Mutex{}
	
	cache.SetEvictCallback(func(key string, value interface{}) {
		evictMutex.Lock()
		defer evictMutex.Unlock()
		evicted[key] = value
	})
	
	// 용량을 초과하는 항목 추가
	for i := 0; i < capacity*2; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		if err := cache.Add(key, value, 1); err != nil {
			t.Fatalf("Failed to add item to cache: %v", err)
		}
	}
	
	// 캐시 크기 확인
	if cache.Len() > capacity {
		t.Errorf("Cache size %d exceeds capacity %d", cache.Len(), capacity)
	}
	
	// 제거된 항목 확인
	evictMutex.Lock()
	if len(evicted) == 0 {
		t.Errorf("No items were evicted")
	}
	evictMutex.Unlock()
	
	// 가장 오래된 항목이 제거되었는지 확인
	for i := 0; i < capacity; i++ {
		key := fmt.Sprintf("key-%d", i)
		_, ok := cache.Get(key)
		if ok {
			t.Errorf("Expected key %s to be evicted, but it still exists", key)
		}
	}
	
	// 최근 항목이 남아있는지 확인
	for i := capacity; i < capacity*2; i++ {
		key := fmt.Sprintf("key-%d", i)
		_, ok := cache.Get(key)
		if !ok {
			t.Errorf("Expected key %s to exist, but it was evicted", key)
		}
	}
}

func TestLRUCache_Update(t *testing.T) {
	// 캐시 생성
	cache := NewLRUCache(100, 0)
	
	// 시작
	cache.Start()
	
	// 중지 (테스트 종료 시)
	defer cache.Stop()
	
	// 항목 추가
	testKey := "test-key"
	testValue1 := "test-value-1"
	testValue2 := "test-value-2"
	
	// 첫 번째 값 추가
	if err := cache.Add(testKey, testValue1, 1); err != nil {
		t.Fatalf("Failed to add item to cache: %v", err)
	}
	
	// 값 확인
	value, ok := cache.Get(testKey)
	if !ok || value != testValue1 {
		t.Fatalf("Retrieved value does not match added value. Got %v, want %v", value, testValue1)
	}
	
	// 두 번째 값으로 업데이트
	if err := cache.Add(testKey, testValue2, 1); err != nil {
		t.Fatalf("Failed to update item in cache: %v", err)
	}
	
	// 업데이트된 값 확인
	value, ok = cache.Get(testKey)
	if !ok || value != testValue2 {
		t.Fatalf("Retrieved value does not match updated value. Got %v, want %v", value, testValue2)
	}
	
	// 통계 확인
	stats := cache.GetStats()
	if stats["hits"].(uint64) != 2 {
		t.Errorf("Expected 2 hits, got %v", stats["hits"])
	}
}

func TestLRUCache_Remove(t *testing.T) {
	// 캐시 생성
	cache := NewLRUCache(100, 0)
	
	// 시작
	cache.Start()
	
	// 중지 (테스트 종료 시)
	defer cache.Stop()
	
	// 항목 추가
	testKey := "test-key"
	testValue := "test-value"
	
	if err := cache.Add(testKey, testValue, 1); err != nil {
		t.Fatalf("Failed to add item to cache: %v", err)
	}
	
	// 항목 제거
	removed := cache.Remove(testKey)
	if !removed {
		t.Fatalf("Failed to remove item from cache")
	}
	
	// 제거된 항목 조회
	_, ok := cache.Get(testKey)
	if ok {
		t.Fatalf("Item was not removed from cache")
	}
	
	// 통계 확인
	stats := cache.GetStats()
	if stats["misses"].(uint64) != 1 {
		t.Errorf("Expected 1 miss, got %v", stats["misses"])
	}
}

func TestLRUCache_Clear(t *testing.T) {
	// 캐시 생성
	cache := NewLRUCache(100, 0)
	
	// 시작
	cache.Start()
	
	// 중지 (테스트 종료 시)
	defer cache.Stop()
	
	// 여러 항목 추가
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		if err := cache.Add(key, value, 1); err != nil {
			t.Fatalf("Failed to add item to cache: %v", err)
		}
	}
	
	// 캐시 크기 확인
	if cache.Len() != 10 {
		t.Errorf("Expected cache size 10, got %d", cache.Len())
	}
	
	// 캐시 초기화
	cache.Clear()
	
	// 초기화 후 크기 확인
	if cache.Len() != 0 {
		t.Errorf("Expected empty cache after clear, got size %d", cache.Len())
	}
	
	// 초기화 후 항목 조회
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key-%d", i)
		_, ok := cache.Get(key)
		if ok {
			t.Errorf("Item %s still exists after clear", key)
		}
	}
}

func TestLRUCache_Concurrency(t *testing.T) {
	// 캐시 생성
	cache := NewLRUCache(1000, 0)
	
	// 시작
	cache.Start()
	
	// 중지 (테스트 종료 시)
	defer cache.Stop()
	
	// 동시 접근 테스트
	const numGoroutines = 10
	const numOperations = 100
	
	done := make(chan bool, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				value := fmt.Sprintf("value-%d-%d", id, j)
				
				// 항목 추가
				if err := cache.Add(key, value, 1); err != nil {
					t.Errorf("Failed to add item to cache: %v", err)
					done <- false
					return
				}
				
				// 항목 조회
				retrievedValue, ok := cache.Get(key)
				if !ok {
					t.Errorf("Failed to get item from cache")
					done <- false
					return
				}
				
				// 값 비교
				if retrievedValue != value {
					t.Errorf("Retrieved value does not match added value. Got %v, want %v", retrievedValue, value)
					done <- false
					return
				}
				
				// 일부 항목 제거
				if j%2 == 0 {
					if !cache.Remove(key) {
						t.Errorf("Failed to remove item from cache")
						done <- false
						return
					}
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
	stats := cache.GetStats()
	if stats["hits"].(uint64) < uint64(numGoroutines*numOperations) {
		t.Errorf("Expected at least %d hits, got %v", numGoroutines*numOperations, stats["hits"])
	}
}

func TestLRUCache_MaxItemSize(t *testing.T) {
	// 최대 항목 크기 제한이 있는 캐시 생성
	cache := NewLRUCache(100, 10)
	
	// 시작
	cache.Start()
	
	// 중지 (테스트 종료 시)
	defer cache.Stop()
	
	// 허용된 크기의 항목 추가
	if err := cache.Add("small-key", "small-value", 5); err != nil {
		t.Fatalf("Failed to add small item to cache: %v", err)
	}
	
	// 크기 제한을 초과하는 항목 추가 시도
	err := cache.Add("large-key", "large-value", 15)
	if err == nil {
		t.Fatalf("Expected error when adding item exceeding max size, but got none")
	}
	
	// 크기 제한을 초과하는 항목이 추가되지 않았는지 확인
	_, ok := cache.Get("large-key")
	if ok {
		t.Fatalf("Large item was added to cache despite size limit")
	}
}

func TestLRUCache_Keys(t *testing.T) {
	// 캐시 생성
	cache := NewLRUCache(100, 0)
	
	// 시작
	cache.Start()
	
	// 중지 (테스트 종료 시)
	defer cache.Stop()
	
	// 여러 항목 추가
	expectedKeys := make(map[string]bool)
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		if err := cache.Add(key, value, 1); err != nil {
			t.Fatalf("Failed to add item to cache: %v", err)
		}
		expectedKeys[key] = true
	}
	
	// 키 목록 가져오기
	keys := cache.Keys()
	
	// 키 수 확인
	if len(keys) != 10 {
		t.Errorf("Expected 10 keys, got %d", len(keys))
	}
	
	// 모든 키가 포함되어 있는지 확인
	for _, key := range keys {
		if !expectedKeys[key] {
			t.Errorf("Unexpected key in result: %s", key)
		}
		delete(expectedKeys, key)
	}
	
	// 누락된 키가 없는지 확인
	if len(expectedKeys) > 0 {
		t.Errorf("Some keys are missing from result: %v", expectedKeys)
	}
}

func TestLRUCache_LRUBehavior(t *testing.T) {
	// 작은 용량의 캐시 생성
	capacity := 3
	cache := NewLRUCache(capacity, 0)
	
	// 시작
	cache.Start()
	
	// 중지 (테스트 종료 시)
	defer cache.Stop()
	
	// 항목 추가
	for i := 0; i < capacity; i++ {
		key := fmt.Sprintf("key-%d", i)
		value := fmt.Sprintf("value-%d", i)
		if err := cache.Add(key, value, 1); err != nil {
			t.Fatalf("Failed to add item to cache: %v", err)
		}
	}
	
	// 첫 번째 항목 접근하여 LRU 순서 변경
	_, ok := cache.Get("key-0")
	if !ok {
		t.Fatalf("Failed to get item from cache")
	}
	
	// 용량을 초과하는 항목 추가
	if err := cache.Add("key-3", "value-3", 1); err != nil {
		t.Fatalf("Failed to add item to cache: %v", err)
	}
	
	// LRU 항목(key-1)이 제거되었는지 확인
	_, ok = cache.Get("key-1")
	if ok {
		t.Errorf("Expected LRU item to be evicted, but it still exists")
	}
	
	// 다른 항목들은 남아있는지 확인
	for _, key := range []string{"key-0", "key-2", "key-3"} {
		_, ok := cache.Get(key)
		if !ok {
			t.Errorf("Expected key %s to exist, but it was evicted", key)
		}
	}
} 