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
	"container/list"
	"fmt"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/metrics"
)

// LRU 캐시 관련 상수
const (
	defaultLRUCacheSize = 1024
	minLRUCacheSize     = 64
	maxLRUCacheSize     = 16384
	
	// 메트릭스 관련
	lruMetricsInterval  = 1 * time.Minute
)

// LRUCacheItem은 LRU 캐시의 항목을 나타냅니다.
type LRUCacheItem struct {
	Key       string
	Value     interface{}
	Size      int
	Timestamp time.Time
}

// LRUCache는 Least Recently Used 캐싱 알고리즘을 구현합니다.
type LRUCache struct {
	// 설정
	capacity    int
	maxItemSize int
	
	// 캐시 데이터 구조
	items     map[string]*list.Element
	evictList *list.List
	
	// 통계
	hits       uint64
	misses     uint64
	evictions  uint64
	totalSize  int
	hitRatio   *metrics.GaugeFloat64
	missRatio  *metrics.GaugeFloat64
	sizeGauge  *metrics.Gauge
	
	// 콜백 함수
	onEvict func(key string, value interface{})
	
	// 동기화
	mu       sync.RWMutex
	stopCh   chan struct{}
}

// NewLRUCache는 새로운 LRU 캐시 인스턴스를 생성합니다.
func NewLRUCache(capacity int, maxItemSize int) *LRUCache {
	if capacity <= 0 {
		capacity = defaultLRUCacheSize
	}
	
	if capacity < minLRUCacheSize {
		capacity = minLRUCacheSize
	}
	
	if capacity > maxLRUCacheSize {
		capacity = maxLRUCacheSize
	}
	
	if maxItemSize <= 0 {
		maxItemSize = capacity / 10
	}
	
	cache := &LRUCache{
		capacity:    capacity,
		maxItemSize: maxItemSize,
		items:       make(map[string]*list.Element),
		evictList:   list.New(),
		hitRatio:    metrics.NewGaugeFloat64(),
		missRatio:   metrics.NewGaugeFloat64(),
		sizeGauge:   metrics.NewGauge(),
		stopCh:      make(chan struct{}),
	}
	
	// 메트릭스 등록
	metrics.Register("lru_cache.hit_ratio", cache.hitRatio)
	metrics.Register("lru_cache.miss_ratio", cache.missRatio)
	metrics.Register("lru_cache.size", cache.sizeGauge)
	
	// 메트릭스 수집 시작
	go cache.collectMetrics()
	
	return cache
}

// Start는 LRU 캐시를 시작합니다.
func (c *LRUCache) Start() {
	log.Info("LRU cache started", "capacity", c.capacity, "maxItemSize", c.maxItemSize)
}

// Stop은 LRU 캐시를 중지합니다.
func (c *LRUCache) Stop() {
	close(c.stopCh)
	log.Info("LRU cache stopped", "hits", c.hits, "misses", c.misses, "evictions", c.evictions)
	
	// 메트릭스 등록 해제
	metrics.Unregister("lru_cache.hit_ratio")
	metrics.Unregister("lru_cache.miss_ratio")
	metrics.Unregister("lru_cache.size")
}

// SetEvictCallback은 항목이 제거될 때 호출될 콜백 함수를 설정합니다.
func (c *LRUCache) SetEvictCallback(onEvict func(key string, value interface{})) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onEvict = onEvict
}

// Get은 캐시에서 키에 해당하는 값을 가져옵니다.
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if ent, ok := c.items[key]; ok {
		// 캐시 히트: 항목을 리스트의 앞으로 이동
		c.evictList.MoveToFront(ent)
		item := ent.Value.(*LRUCacheItem)
		item.Timestamp = time.Now() // 접근 시간 업데이트
		c.hits++
		return item.Value, true
	}
	
	// 캐시 미스
	c.misses++
	return nil, false
}

// Add는 캐시에 새 항목을 추가합니다.
func (c *LRUCache) Add(key string, value interface{}, size int) error {
	if size > c.maxItemSize {
		return fmt.Errorf("item size %d exceeds maximum allowed size %d", size, c.maxItemSize)
	}
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// 이미 존재하는 항목인 경우 업데이트
	if ent, ok := c.items[key]; ok {
		c.evictList.MoveToFront(ent)
		item := ent.Value.(*LRUCacheItem)
		c.totalSize = c.totalSize - item.Size + size
		item.Value = value
		item.Size = size
		item.Timestamp = time.Now()
		return nil
	}
	
	// 새 항목 추가 전에 공간 확보
	for c.totalSize+size > c.capacity && c.evictList.Len() > 0 {
		c.removeOldest()
	}
	
	// 새 항목 추가
	item := &LRUCacheItem{
		Key:       key,
		Value:     value,
		Size:      size,
		Timestamp: time.Now(),
	}
	entry := c.evictList.PushFront(item)
	c.items[key] = entry
	c.totalSize += size
	
	return nil
}

// Remove는 캐시에서 키에 해당하는 항목을 제거합니다.
func (c *LRUCache) Remove(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if ent, ok := c.items[key]; ok {
		c.removeElement(ent)
		return true
	}
	return false
}

// Len은 캐시에 있는 항목의 수를 반환합니다.
func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.evictList.Len()
}

// Size는 캐시의 현재 크기를 반환합니다.
func (c *LRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.totalSize
}

// Clear는 캐시의 모든 항목을 제거합니다.
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// 모든 항목에 대해 콜백 호출
	if c.onEvict != nil {
		for _, v := range c.items {
			item := v.Value.(*LRUCacheItem)
			c.onEvict(item.Key, item.Value)
		}
	}
	
	// 캐시 초기화
	c.items = make(map[string]*list.Element)
	c.evictList.Init()
	c.totalSize = 0
}

// Keys는 캐시에 있는 모든 키의 목록을 반환합니다.
func (c *LRUCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	keys := make([]string, 0, len(c.items))
	for k := range c.items {
		keys = append(keys, k)
	}
	return keys
}

// GetStats는 캐시 통계를 반환합니다.
func (c *LRUCache) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	totalAccess := c.hits + c.misses
	hitRatio := 0.0
	if totalAccess > 0 {
		hitRatio = float64(c.hits) / float64(totalAccess)
	}
	
	return map[string]interface{}{
		"capacity":   c.capacity,
		"size":       c.totalSize,
		"items":      c.evictList.Len(),
		"hits":       c.hits,
		"misses":     c.misses,
		"evictions":  c.evictions,
		"hit_ratio":  hitRatio,
		"usage_ratio": float64(c.totalSize) / float64(c.capacity),
	}
}

// 내부 메서드

// removeOldest는 가장 오래된 항목을 제거합니다.
func (c *LRUCache) removeOldest() {
	ent := c.evictList.Back()
	if ent != nil {
		c.removeElement(ent)
	}
}

// removeElement는 지정된 요소를 제거합니다.
func (c *LRUCache) removeElement(e *list.Element) {
	c.evictList.Remove(e)
	item := e.Value.(*LRUCacheItem)
	delete(c.items, item.Key)
	c.totalSize -= item.Size
	c.evictions++
	
	// 콜백 호출
	if c.onEvict != nil {
		c.onEvict(item.Key, item.Value)
	}
}

// collectMetrics는 주기적으로 메트릭스를 수집합니다.
func (c *LRUCache) collectMetrics() {
	ticker := time.NewTicker(lruMetricsInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.mu.RLock()
			totalAccess := c.hits + c.misses
			hitRatio := 0.0
			missRatio := 0.0
			if totalAccess > 0 {
				hitRatio = float64(c.hits) / float64(totalAccess)
				missRatio = float64(c.misses) / float64(totalAccess)
			}
			
			c.hitRatio.Update(hitRatio)
			c.missRatio.Update(missRatio)
			c.sizeGauge.Update(int64(c.totalSize))
			
			log.Debug("LRU cache metrics", 
				"size", c.totalSize, 
				"items", c.evictList.Len(),
				"hit_ratio", hitRatio,
				"evictions", c.evictions)
			c.mu.RUnlock()
		}
	}
} 