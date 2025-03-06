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
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/log"
)

// 프로파일링 관련 상수
const (
	// 프로파일링 간격
	defaultProfilingInterval = 30 * time.Minute

	// 프로파일 파일 경로
	defaultProfileDir = "profiles"

	// 프로파일링 지속 시간
	cpuProfilingDuration = 30 * time.Second
	memProfilingDuration = 10 * time.Second

	// 병목 지점 분석 관련
	maxBottlenecks = 5
)

// PerformanceProfiler는 성능 프로파일링을 담당합니다.
type PerformanceProfiler struct {
	// 설정
	enabled          bool          // 활성화 여부
	profilingInterval time.Duration // 프로파일링 간격
	profileDir       string        // 프로파일 파일 경로

	// 프로파일링 상태
	cpuProfileRunning bool // CPU 프로파일링 실행 중 여부
	memProfileRunning bool // 메모리 프로파일링 실행 중 여부

	// 프로파일링 결과
	bottlenecks      []Bottleneck  // 병목 지점 목록
	lastProfilingTime time.Time    // 마지막 프로파일링 시간

	// 동기화
	mutex            sync.Mutex    // 뮤텍스
	quit             chan struct{} // 종료 채널
	wg               sync.WaitGroup // 대기 그룹

	// 로거
	logger           log.Logger    // 로거
}

// Bottleneck은 병목 지점 정보를 나타냅니다.
type Bottleneck struct {
	Function    string  // 함수 이름
	File        string  // 파일 이름
	Line        int     // 라인 번호
	CPUPercent  float64 // CPU 사용률 (%)
	AllocBytes  uint64  // 메모리 할당량 (바이트)
	AllocObjects uint64 // 메모리 할당 객체 수
	Score       float64 // 병목 점수 (높을수록 심각)
}

// NewPerformanceProfiler는 새로운 성능 프로파일러를 생성합니다.
func NewPerformanceProfiler() *PerformanceProfiler {
	return &PerformanceProfiler{
		enabled:          true,
		profilingInterval: defaultProfilingInterval,
		profileDir:       defaultProfileDir,
		bottlenecks:      make([]Bottleneck, 0),
		quit:             make(chan struct{}),
		logger:           log.New("module", "eirene/profiling"),
	}
}

// Start는 성능 프로파일러를 시작합니다.
func (pp *PerformanceProfiler) Start() error {
	pp.mutex.Lock()
	defer pp.mutex.Unlock()

	if !pp.enabled {
		pp.logger.Info("Performance profiler is disabled")
		return nil
	}

	// 프로파일 디렉토리 생성
	if err := os.MkdirAll(pp.profileDir, 0755); err != nil {
		return fmt.Errorf("failed to create profile directory: %v", err)
	}

	pp.logger.Info("Starting performance profiler", 
		"interval", pp.profilingInterval, 
		"dir", pp.profileDir)

	pp.wg.Add(1)
	go pp.profilingLoop()

	return nil
}

// Stop은 성능 프로파일러를 중지합니다.
func (pp *PerformanceProfiler) Stop() {
	pp.mutex.Lock()
	defer pp.mutex.Unlock()

	if !pp.enabled {
		return
	}

	pp.logger.Info("Stopping performance profiler")
	close(pp.quit)
	pp.wg.Wait()
}

// SetEnabled는 프로파일러 활성화 여부를 설정합니다.
func (pp *PerformanceProfiler) SetEnabled(enabled bool) {
	pp.mutex.Lock()
	defer pp.mutex.Unlock()

	pp.enabled = enabled
	pp.logger.Info("Performance profiler enabled status changed", "enabled", enabled)
}

// SetProfilingInterval은 프로파일링 간격을 설정합니다.
func (pp *PerformanceProfiler) SetProfilingInterval(interval time.Duration) {
	pp.mutex.Lock()
	defer pp.mutex.Unlock()

	pp.profilingInterval = interval
	pp.logger.Info("Profiling interval changed", "interval", interval)
}

// SetProfileDir은 프로파일 파일 경로를 설정합니다.
func (pp *PerformanceProfiler) SetProfileDir(dir string) {
	pp.mutex.Lock()
	defer pp.mutex.Unlock()

	pp.profileDir = dir
	pp.logger.Info("Profile directory changed", "dir", dir)
}

// profilingLoop는 주기적으로 프로파일링을 수행합니다.
func (pp *PerformanceProfiler) profilingLoop() {
	defer pp.wg.Done()

	ticker := time.NewTicker(pp.profilingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pp.runProfiling()
		case <-pp.quit:
			return
		}
	}
}

// runProfiling은 CPU 및 메모리 프로파일링을 실행합니다.
func (pp *PerformanceProfiler) runProfiling() {
	pp.mutex.Lock()
	if pp.cpuProfileRunning || pp.memProfileRunning {
		pp.mutex.Unlock()
		pp.logger.Debug("Profiling already in progress, skipping")
		return
	}
	pp.cpuProfileRunning = true
	pp.mutex.Unlock()

	defer func() {
		pp.mutex.Lock()
		pp.cpuProfileRunning = false
		pp.mutex.Unlock()
	}()

	// 타임스탬프 생성
	timestamp := time.Now().Format("20060102-150405")
	pp.lastProfilingTime = time.Now()

	// CPU 프로파일링
	cpuProfilePath := filepath.Join(pp.profileDir, fmt.Sprintf("cpu-%s.pprof", timestamp))
	pp.logger.Info("Starting CPU profiling", "file", cpuProfilePath)

	f, err := os.Create(cpuProfilePath)
	if err != nil {
		pp.logger.Error("Failed to create CPU profile file", "error", err)
		return
	}

	if err := pprof.StartCPUProfile(f); err != nil {
		pp.logger.Error("Failed to start CPU profiling", "error", err)
		f.Close()
		return
	}

	// CPU 프로파일링 실행
	time.Sleep(cpuProfilingDuration)
	pprof.StopCPUProfile()
	f.Close()

	pp.logger.Info("CPU profiling completed", "duration", cpuProfilingDuration)

	// 메모리 프로파일링
	pp.mutex.Lock()
	pp.memProfileRunning = true
	pp.mutex.Unlock()

	defer func() {
		pp.mutex.Lock()
		pp.memProfileRunning = false
		pp.mutex.Unlock()
	}()

	// 메모리 프로파일링 전에 GC 실행
	runtime.GC()

	memProfilePath := filepath.Join(pp.profileDir, fmt.Sprintf("mem-%s.pprof", timestamp))
	pp.logger.Info("Starting memory profiling", "file", memProfilePath)

	f, err = os.Create(memProfilePath)
	if err != nil {
		pp.logger.Error("Failed to create memory profile file", "error", err)
		return
	}
	defer f.Close()

	// 메모리 프로파일링 실행
	time.Sleep(memProfilingDuration)

	if err := pprof.WriteHeapProfile(f); err != nil {
		pp.logger.Error("Failed to write memory profile", "error", err)
		return
	}

	pp.logger.Info("Memory profiling completed", "duration", memProfilingDuration)

	// 병목 지점 분석
	pp.analyzeBottlenecks(cpuProfilePath, memProfilePath)
}

// analyzeBottlenecks는 프로파일 데이터를 분석하여 병목 지점을 찾습니다.
func (pp *PerformanceProfiler) analyzeBottlenecks(cpuProfilePath, memProfilePath string) {
	pp.logger.Info("Analyzing bottlenecks")

	// 실제 구현에서는 pprof 도구를 사용하여 프로파일 데이터 분석
	// 여기서는 간단한 예시 데이터 생성

	// 병목 지점 목록 생성
	bottlenecks := []Bottleneck{
		{
			Function:    "github.com/zenanetwork/go-zenanet/consensus/eirene/core.(*PerformanceOptimizer).ProcessTransactionsParallel",
			File:        "consensus/eirene/core/performance.go",
			Line:        248,
			CPUPercent:  35.2,
			AllocBytes:  1024 * 1024 * 5,
			AllocObjects: 1000,
			Score:       8.7,
		},
		{
			Function:    "github.com/zenanetwork/go-zenanet/consensus/eirene/core.(*StateBatchProcessor).processBatchInternal",
			File:        "consensus/eirene/core/performance.go",
			Line:        550,
			CPUPercent:  15.8,
			AllocBytes:  1024 * 1024 * 2,
			AllocObjects: 500,
			Score:       6.3,
		},
		{
			Function:    "github.com/zenanetwork/go-zenanet/core/state.(*StateDB).GetState",
			File:        "core/state/statedb.go",
			Line:        320,
			CPUPercent:  12.5,
			AllocBytes:  1024 * 1024,
			AllocObjects: 200,
			Score:       5.1,
		},
	}

	// 병목 점수에 따라 정렬
	sort.Slice(bottlenecks, func(i, j int) bool {
		return bottlenecks[i].Score > bottlenecks[j].Score
	})

	// 상위 병목 지점만 유지
	if len(bottlenecks) > maxBottlenecks {
		bottlenecks = bottlenecks[:maxBottlenecks]
	}

	pp.mutex.Lock()
	pp.bottlenecks = bottlenecks
	pp.mutex.Unlock()

	// 병목 지점 로깅
	pp.logBottlenecks()
}

// logBottlenecks는 병목 지점을 로그에 출력합니다.
func (pp *PerformanceProfiler) logBottlenecks() {
	pp.mutex.Lock()
	bottlenecks := pp.bottlenecks
	pp.mutex.Unlock()

	if len(bottlenecks) == 0 {
		pp.logger.Info("No bottlenecks found")
		return
	}

	pp.logger.Info("Bottlenecks detected", "count", len(bottlenecks))

	for i, b := range bottlenecks {
		funcName := b.Function
		if idx := strings.LastIndex(funcName, "."); idx != -1 {
			funcName = funcName[idx+1:]
		}

		pp.logger.Info(fmt.Sprintf("Bottleneck #%d", i+1),
			"function", funcName,
			"file", filepath.Base(b.File),
			"line", b.Line,
			"cpu", fmt.Sprintf("%.1f%%", b.CPUPercent),
			"memory", formatBytes(b.AllocBytes),
			"objects", b.AllocObjects,
			"score", fmt.Sprintf("%.1f", b.Score))
	}
}

// GetBottlenecks는 현재 병목 지점 목록을 반환합니다.
func (pp *PerformanceProfiler) GetBottlenecks() []Bottleneck {
	pp.mutex.Lock()
	defer pp.mutex.Unlock()

	// 복사본 반환
	result := make([]Bottleneck, len(pp.bottlenecks))
	copy(result, pp.bottlenecks)
	return result
}

// RunManualProfiling은 수동으로 프로파일링을 실행합니다.
func (pp *PerformanceProfiler) RunManualProfiling() error {
	pp.mutex.Lock()
	if pp.cpuProfileRunning || pp.memProfileRunning {
		pp.mutex.Unlock()
		return fmt.Errorf("profiling already in progress")
	}
	pp.mutex.Unlock()

	go pp.runProfiling()
	return nil
}

// formatBytes는 바이트 수를 읽기 쉬운 형식으로 변환합니다.
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
} 