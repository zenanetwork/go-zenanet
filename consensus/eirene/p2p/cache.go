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

package p2p

import (
	"container/list"
	"sync"

	"github.com/zenanetwork/go-zenanet/common"
)

// KnownCache는 알려진 항목(트랜잭션, 블록 등)을 추적하는 캐시입니다.
// LRU(Least Recently Used) 알고리즘을 사용하여 캐시 크기를 제한합니다.
type KnownCache struct {
	capacity int                      // 최대 캐시 크기
	items    map[common.Hash]*list.Element // 해시 -> 리스트 요소 매핑
	queue    *list.List               // LRU 큐
	lock     sync.RWMutex             // 동시성 제어를 위한 락
}

// NewKnownCache는 새로운 알려진 항목 캐시를 생성합니다.
func NewKnownCache(capacity int) *KnownCache {
	return &KnownCache{
		capacity: capacity,
		items:    make(map[common.Hash]*list.Element),
		queue:    list.New(),
	}
}

// Add는 해시를 캐시에 추가합니다.
func (kc *KnownCache) Add(hash common.Hash) {
	kc.lock.Lock()
	defer kc.lock.Unlock()

	// 이미 캐시에 있는 경우, 큐의 앞으로 이동
	if element, exists := kc.items[hash]; exists {
		kc.queue.MoveToFront(element)
		return
	}

	// 캐시가 가득 찬 경우, 가장 오래된 항목 제거
	if kc.queue.Len() >= kc.capacity {
		oldest := kc.queue.Back()
		if oldest != nil {
			kc.queue.Remove(oldest)
			delete(kc.items, oldest.Value.(common.Hash))
		}
	}

	// 새 항목 추가
	element := kc.queue.PushFront(hash)
	kc.items[hash] = element
}

// Contains는 해시가 캐시에 있는지 확인합니다.
func (kc *KnownCache) Contains(hash common.Hash) bool {
	kc.lock.RLock()
	defer kc.lock.RUnlock()

	_, exists := kc.items[hash]
	return exists
}

// Len은 캐시에 있는 항목 수를 반환합니다.
func (kc *KnownCache) Len() int {
	kc.lock.RLock()
	defer kc.lock.RUnlock()

	return kc.queue.Len()
}

// Clear는 캐시를 비웁니다.
func (kc *KnownCache) Clear() {
	kc.lock.Lock()
	defer kc.lock.Unlock()

	kc.items = make(map[common.Hash]*list.Element)
	kc.queue = list.New()
}

// Remove는 해시를 캐시에서 제거합니다.
func (kc *KnownCache) Remove(hash common.Hash) {
	kc.lock.Lock()
	defer kc.lock.Unlock()

	if element, exists := kc.items[hash]; exists {
		kc.queue.Remove(element)
		delete(kc.items, hash)
	}
}

// Items는 캐시에 있는 모든 해시를 반환합니다.
func (kc *KnownCache) Items() []common.Hash {
	kc.lock.RLock()
	defer kc.lock.RUnlock()

	items := make([]common.Hash, 0, kc.queue.Len())
	for e := kc.queue.Front(); e != nil; e = e.Next() {
		items = append(items, e.Value.(common.Hash))
	}
	return items
}

// Capacity는 캐시의 최대 크기를 반환합니다.
func (kc *KnownCache) Capacity() int {
	return kc.capacity
}

// Resize는 캐시의 최대 크기를 변경합니다.
func (kc *KnownCache) Resize(capacity int) {
	kc.lock.Lock()
	defer kc.lock.Unlock()

	// 새 크기가 현재 크기보다 작은 경우, 초과 항목 제거
	for kc.queue.Len() > capacity {
		oldest := kc.queue.Back()
		if oldest != nil {
			kc.queue.Remove(oldest)
			delete(kc.items, oldest.Value.(common.Hash))
		}
	}

	kc.capacity = capacity
} 