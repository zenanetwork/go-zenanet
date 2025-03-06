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
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/metrics"
)

// 디스크 I/O 최적화 관련 상수
const (
	// 디스크 I/O 관련
	defaultWriteBufferSize  = 4 * 1024 * 1024  // 4MB
	defaultReadBufferSize   = 2 * 1024 * 1024  // 2MB
	defaultFlushInterval    = 5 * time.Second  // 5초
	defaultCompactionInterval = 1 * time.Hour  // 1시간
	
	// 파일 시스템 관련
	defaultFilePermission   = 0644
	defaultDirPermission    = 0755
	
	// 메트릭스 관련
	diskIOMetricsInterval   = 1 * time.Minute  // 1분
)

// DiskIOOptimizer는 디스크 I/O 최적화를 담당합니다.
type DiskIOOptimizer struct {
	// 설정
	writeBufferSize  int
	readBufferSize   int
	flushInterval    time.Duration
	compactionInterval time.Duration
	
	// 상태
	isRunning        bool
	lastFlushTime    time.Time
	lastCompactionTime time.Time
	
	// 통계
	totalReads       uint64
	totalWrites      uint64
	totalReadBytes   uint64
	totalWriteBytes  uint64
	readLatency      metrics.Histogram
	writeLatency     metrics.Histogram
	
	// 버퍼
	writeBuffer      map[string][]byte
	readCache        map[string][]byte
	
	// 동기화
	mu               sync.RWMutex
	flushCh          chan struct{}
	compactionCh     chan struct{}
	stopCh           chan struct{}
}

// NewDiskIOOptimizer는 새로운 DiskIOOptimizer 인스턴스를 생성합니다.
func NewDiskIOOptimizer() *DiskIOOptimizer {
	return &DiskIOOptimizer{
		writeBufferSize:    defaultWriteBufferSize,
		readBufferSize:     defaultReadBufferSize,
		flushInterval:      defaultFlushInterval,
		compactionInterval: defaultCompactionInterval,
		lastFlushTime:      time.Now(),
		lastCompactionTime: time.Now(),
		writeBuffer:        make(map[string][]byte),
		readCache:          make(map[string][]byte),
		readLatency:        metrics.NewHistogram(metrics.NewExpDecaySample(1028, 0.015)),
		writeLatency:       metrics.NewHistogram(metrics.NewExpDecaySample(1028, 0.015)),
		flushCh:            make(chan struct{}),
		compactionCh:       make(chan struct{}),
		stopCh:             make(chan struct{}),
	}
}

// Start는 디스크 I/O 최적화 프로세스를 시작합니다.
func (dio *DiskIOOptimizer) Start() error {
	dio.mu.Lock()
	defer dio.mu.Unlock()
	
	if dio.isRunning {
		return fmt.Errorf("disk I/O optimizer is already running")
	}
	
	dio.isRunning = true
	
	// 백그라운드 작업 시작
	go dio.backgroundWorker()
	go dio.metricsCollector()
	
	log.Info("Disk I/O optimizer started", "writeBufferSize", dio.writeBufferSize, "readBufferSize", dio.readBufferSize)
	return nil
}

// Stop은 디스크 I/O 최적화 프로세스를 중지합니다.
func (dio *DiskIOOptimizer) Stop() error {
	dio.mu.Lock()
	defer dio.mu.Unlock()
	
	if !dio.isRunning {
		return fmt.Errorf("disk I/O optimizer is not running")
	}
	
	// 중지 신호 전송
	close(dio.stopCh)
	
	// 마지막 플러시 수행
	dio.flush()
	
	dio.isRunning = false
	log.Info("Disk I/O optimizer stopped", "totalReads", dio.totalReads, "totalWrites", dio.totalWrites)
	return nil
}

// WriteData는 데이터를 버퍼에 쓰고 필요시 플러시합니다.
func (dio *DiskIOOptimizer) WriteData(key string, data []byte) error {
	start := time.Now()
	
	dio.mu.Lock()
	defer dio.mu.Unlock()
	
	// 버퍼에 데이터 추가
	dio.writeBuffer[key] = data
	dio.totalWrites++
	dio.totalWriteBytes += uint64(len(data))
	
	// 버퍼 크기 확인 및 필요시 플러시
	bufferSize := dio.getWriteBufferSize()
	if bufferSize >= dio.writeBufferSize {
		if err := dio.flushLocked(); err != nil {
			return err
		}
	}
	
	// 지연 시간 측정
	dio.writeLatency.Update(time.Since(start).Nanoseconds())
	
	return nil
}

// ReadData는 캐시 또는 디스크에서 데이터를 읽습니다.
func (dio *DiskIOOptimizer) ReadData(key string) ([]byte, error) {
	start := time.Now()
	
	dio.mu.RLock()
	// 먼저 쓰기 버퍼 확인
	if data, ok := dio.writeBuffer[key]; ok {
		dio.mu.RUnlock()
		dio.totalReads++
		dio.readLatency.Update(time.Since(start).Nanoseconds())
		return data, nil
	}
	
	// 읽기 캐시 확인
	if data, ok := dio.readCache[key]; ok {
		dio.mu.RUnlock()
		dio.totalReads++
		dio.readLatency.Update(time.Since(start).Nanoseconds())
		return data, nil
	}
	dio.mu.RUnlock()
	
	// 디스크에서 읽기
	data, err := dio.readFromDisk(key)
	if err != nil {
		return nil, err
	}
	
	// 캐시에 추가
	dio.mu.Lock()
	dio.readCache[key] = data
	dio.totalReads++
	dio.totalReadBytes += uint64(len(data))
	
	// 캐시 크기 관리
	if dio.getCacheSize() > dio.readBufferSize {
		dio.pruneCache()
	}
	dio.mu.Unlock()
	
	// 지연 시간 측정
	dio.readLatency.Update(time.Since(start).Nanoseconds())
	
	return data, nil
}

// Flush는 버퍼의 모든 데이터를 디스크에 기록합니다.
func (dio *DiskIOOptimizer) Flush() error {
	dio.mu.Lock()
	defer dio.mu.Unlock()
	
	return dio.flushLocked()
}

// TriggerCompaction은 데이터베이스 압축을 트리거합니다.
func (dio *DiskIOOptimizer) TriggerCompaction() {
	select {
	case dio.compactionCh <- struct{}{}:
	default:
		// 이미 압축이 예약된 경우 무시
	}
}

// GetStats는 디스크 I/O 통계를 반환합니다.
func (dio *DiskIOOptimizer) GetStats() map[string]interface{} {
	dio.mu.RLock()
	defer dio.mu.RUnlock()
	
	return map[string]interface{}{
		"totalReads":      dio.totalReads,
		"totalWrites":     dio.totalWrites,
		"totalReadBytes":  dio.totalReadBytes,
		"totalWriteBytes": dio.totalWriteBytes,
		"writeBufferSize": dio.getWriteBufferSize(),
		"readCacheSize":   dio.getCacheSize(),
		"avgReadLatency":  float64(dio.readLatency.Snapshot().Mean()) / float64(time.Millisecond),
		"avgWriteLatency": float64(dio.writeLatency.Snapshot().Mean()) / float64(time.Millisecond),
	}
}

// 내부 메서드

// backgroundWorker는 백그라운드 작업을 처리합니다.
func (dio *DiskIOOptimizer) backgroundWorker() {
	flushTicker := time.NewTicker(dio.flushInterval)
	compactionTicker := time.NewTicker(dio.compactionInterval)
	
	defer flushTicker.Stop()
	defer compactionTicker.Stop()
	
	for {
		select {
		case <-dio.stopCh:
			return
		case <-flushTicker.C:
			if err := dio.Flush(); err != nil {
				log.Error("Failed to flush write buffer", "err", err)
			}
		case <-compactionTicker.C:
			dio.runCompaction()
		case <-dio.flushCh:
			if err := dio.Flush(); err != nil {
				log.Error("Failed to flush write buffer", "err", err)
			}
		case <-dio.compactionCh:
			dio.runCompaction()
		}
	}
}

// metricsCollector는 메트릭스를 수집합니다.
func (dio *DiskIOOptimizer) metricsCollector() {
	ticker := time.NewTicker(diskIOMetricsInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-dio.stopCh:
			return
		case <-ticker.C:
			stats := dio.GetStats()
			log.Debug("Disk I/O metrics", 
				"reads", stats["totalReads"], 
				"writes", stats["totalWrites"],
				"readBytes", stats["totalReadBytes"],
				"writeBytes", stats["totalWriteBytes"],
				"readLatency", stats["avgReadLatency"],
				"writeLatency", stats["avgWriteLatency"])
		}
	}
}

// flushLocked는 버퍼의 모든 데이터를 디스크에 기록합니다 (잠금 상태에서 호출).
func (dio *DiskIOOptimizer) flushLocked() error {
	if len(dio.writeBuffer) == 0 {
		return nil
	}
	
	// 버퍼 복사
	bufferCopy := make(map[string][]byte, len(dio.writeBuffer))
	for k, v := range dio.writeBuffer {
		bufferCopy[k] = v
	}
	
	// 버퍼 초기화
	dio.writeBuffer = make(map[string][]byte)
	dio.lastFlushTime = time.Now()
	
	// 잠금 해제 후 디스크에 쓰기
	dio.mu.Unlock()
	err := dio.writeToDisk(bufferCopy)
	dio.mu.Lock()
	
	return err
}

// writeToDisk는 데이터를 디스크에 기록합니다.
func (dio *DiskIOOptimizer) writeToDisk(data map[string][]byte) error {
	// 실제 구현에서는 데이터베이스 또는 파일 시스템에 쓰기 작업 수행
	// 예시 구현:
	for key, value := range data {
		filePath := filepath.Join(os.TempDir(), "zenanet", key)
		
		// 디렉토리 생성
		if err := os.MkdirAll(filepath.Dir(filePath), defaultDirPermission); err != nil {
			return err
		}
		
		// 파일 쓰기
		if err := os.WriteFile(filePath, value, defaultFilePermission); err != nil {
			return err
		}
	}
	
	return nil
}

// readFromDisk는 디스크에서 데이터를 읽습니다.
func (dio *DiskIOOptimizer) readFromDisk(key string) ([]byte, error) {
	// 실제 구현에서는 데이터베이스 또는 파일 시스템에서 읽기 작업 수행
	// 예시 구현:
	filePath := filepath.Join(os.TempDir(), "zenanet", key)
	return os.ReadFile(filePath)
}

// runCompaction은 데이터베이스 압축을 실행합니다.
func (dio *DiskIOOptimizer) runCompaction() {
	dio.mu.Lock()
	dio.lastCompactionTime = time.Now()
	dio.mu.Unlock()
	
	log.Info("Running database compaction")
	
	// 실제 구현에서는 데이터베이스 압축 작업 수행
	// 예시 구현:
	// db.Compact(nil, nil)
}

// getWriteBufferSize는 쓰기 버퍼의 크기를 계산합니다.
func (dio *DiskIOOptimizer) getWriteBufferSize() int {
	size := 0
	for _, data := range dio.writeBuffer {
		size += len(data)
	}
	return size
}

// getCacheSize는 읽기 캐시의 크기를 계산합니다.
func (dio *DiskIOOptimizer) getCacheSize() int {
	size := 0
	for _, data := range dio.readCache {
		size += len(data)
	}
	return size
}

// pruneCache는 읽기 캐시를 정리합니다.
func (dio *DiskIOOptimizer) pruneCache() {
	// 간단한 구현: 캐시 절반 비우기
	newCache := make(map[string][]byte)
	count := 0
	maxCount := len(dio.readCache) / 2
	
	for k, v := range dio.readCache {
		if count < maxCount {
			newCache[k] = v
			count++
		} else {
			break
		}
	}
	
	dio.readCache = newCache
}

// flush는 버퍼의 모든 데이터를 디스크에 기록합니다.
func (dio *DiskIOOptimizer) flush() error {
	select {
	case dio.flushCh <- struct{}{}:
	default:
		// 이미 플러시가 예약된 경우 무시
	}
	return nil
} 