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
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/metrics"
)

// 적응형 워커 풀 관련 상수
const (
	// 워커 풀 관련
	defaultInitialWorkers = 4
	minWorkers            = 2
	maxWorkersMultiplier  = 2  // CPU 코어 수의 배수
	
	// 작업 큐 관련
	defaultQueueSize      = 1000
	minQueueSize          = 100
	
	// 조정 관련
	defaultAdjustInterval = 5 * time.Second
	minIdleWorkerRatio    = 0.1  // 최소 유휴 워커 비율 (10%)
	maxIdleWorkerRatio    = 0.3  // 최대 유휴 워커 비율 (30%)
	scaleUpThreshold      = 0.8  // 큐 사용률 임계값 (80%)
	scaleDownThreshold    = 0.2  // 큐 사용률 임계값 (20%)
	scaleUpFactor         = 1.5  // 확장 계수
	scaleDownFactor       = 0.8  // 축소 계수
	
	// 메트릭스 관련
	workerPoolMetricsInterval = 10 * time.Second
)

// Task는 워커 풀에서 실행할 작업을 나타냅니다.
type Task func() error

// AdaptiveWorkerPool은 부하에 따라 크기를 자동으로 조정하는 워커 풀입니다.
type AdaptiveWorkerPool struct {
	// 설정
	name            string
	initialWorkers  int
	maxWorkers      int
	queueSize       int
	adjustInterval  time.Duration
	
	// 상태
	numWorkers      int32
	activeWorkers   int32
	isRunning       bool
	
	// 작업 큐
	taskQueue       chan Task
	
	// 통계
	tasksProcessed  int64
	tasksQueued     int64
	taskErrors      int64
	queueHighWater  int64
	
	// 메트릭스
	workersGauge    *metrics.Gauge
	activeGauge     *metrics.Gauge
	queueSizeGauge  *metrics.Gauge
	processedGauge  *metrics.Counter
	errorRateGauge  *metrics.GaugeFloat64
	
	// 동기화
	mu              sync.RWMutex
	adjustTicker    *time.Ticker
	stopCh          chan struct{}
	workerStopCh    chan struct{}
}

// NewAdaptiveWorkerPool은 새로운 적응형 워커 풀을 생성합니다.
func NewAdaptiveWorkerPool(name string, initialWorkers, queueSize int) *AdaptiveWorkerPool {
	if initialWorkers <= 0 {
		initialWorkers = defaultInitialWorkers
	}
	
	if initialWorkers < minWorkers {
		initialWorkers = minWorkers
	}
	
	maxWorkers := runtime.NumCPU() * maxWorkersMultiplier
	if initialWorkers > maxWorkers {
		initialWorkers = maxWorkers
	}
	
	if queueSize <= 0 {
		queueSize = defaultQueueSize
	}
	
	if queueSize < minQueueSize {
		queueSize = minQueueSize
	}
	
	pool := &AdaptiveWorkerPool{
		name:           name,
		initialWorkers: initialWorkers,
		maxWorkers:     maxWorkers,
		queueSize:      queueSize,
		adjustInterval: defaultAdjustInterval,
		taskQueue:      make(chan Task, queueSize),
		workersGauge:   metrics.NewGauge(),
		activeGauge:    metrics.NewGauge(),
		queueSizeGauge: metrics.NewGauge(),
		processedGauge: metrics.NewCounter(),
		errorRateGauge: metrics.NewGaugeFloat64(),
		stopCh:         make(chan struct{}),
		workerStopCh:   make(chan struct{}),
	}
	
	// 메트릭스 등록
	metrics.Register(fmt.Sprintf("worker_pool.%s.workers", name), pool.workersGauge)
	metrics.Register(fmt.Sprintf("worker_pool.%s.active", name), pool.activeGauge)
	metrics.Register(fmt.Sprintf("worker_pool.%s.queue_size", name), pool.queueSizeGauge)
	metrics.Register(fmt.Sprintf("worker_pool.%s.processed", name), pool.processedGauge)
	metrics.Register(fmt.Sprintf("worker_pool.%s.error_rate", name), pool.errorRateGauge)
	
	return pool
}

// Start는 워커 풀을 시작합니다.
func (p *AdaptiveWorkerPool) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.isRunning {
		return fmt.Errorf("worker pool is already running")
	}
	
	p.isRunning = true
	
	// 초기 워커 시작
	for i := 0; i < p.initialWorkers; i++ {
		p.startWorker()
	}
	
	// 조정 타이머 시작
	p.adjustTicker = time.NewTicker(p.adjustInterval)
	go p.adjustWorkerCount()
	
	// 메트릭스 수집 시작
	go p.collectMetrics()
	
	log.Info("Adaptive worker pool started", "name", p.name, "workers", p.initialWorkers, "queueSize", p.queueSize)
	return nil
}

// Stop은 워커 풀을 중지합니다.
func (p *AdaptiveWorkerPool) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if !p.isRunning {
		return fmt.Errorf("worker pool is not running")
	}
	
	// 조정 타이머 중지
	if p.adjustTicker != nil {
		p.adjustTicker.Stop()
	}
	
	// 중지 신호 전송
	close(p.stopCh)
	close(p.workerStopCh)
	
	// 메트릭스 등록 해제
	metrics.Unregister(fmt.Sprintf("worker_pool.%s.workers", p.name))
	metrics.Unregister(fmt.Sprintf("worker_pool.%s.active", p.name))
	metrics.Unregister(fmt.Sprintf("worker_pool.%s.queue_size", p.name))
	metrics.Unregister(fmt.Sprintf("worker_pool.%s.processed", p.name))
	metrics.Unregister(fmt.Sprintf("worker_pool.%s.error_rate", p.name))
	
	p.isRunning = false
	log.Info("Adaptive worker pool stopped", "name", p.name, "tasksProcessed", p.tasksProcessed)
	return nil
}

// Submit은 작업을 워커 풀에 제출합니다.
func (p *AdaptiveWorkerPool) Submit(task Task) error {
	if task == nil {
		return fmt.Errorf("cannot submit nil task")
	}
	
	select {
	case p.taskQueue <- task:
		atomic.AddInt64(&p.tasksQueued, 1)
		queueLen := len(p.taskQueue)
		if queueLen > int(atomic.LoadInt64(&p.queueHighWater)) {
			atomic.StoreInt64(&p.queueHighWater, int64(queueLen))
		}
		return nil
	case <-p.stopCh:
		return fmt.Errorf("worker pool is stopped")
	default:
		return fmt.Errorf("task queue is full")
	}
}

// SubmitWait는 작업을 제출하고 완료될 때까지 기다립니다.
func (p *AdaptiveWorkerPool) SubmitWait(task Task) error {
	resultCh := make(chan error, 1)
	
	wrappedTask := func() error {
		err := task()
		resultCh <- err
		return err
	}
	
	if err := p.Submit(wrappedTask); err != nil {
		return err
	}
	
	select {
	case err := <-resultCh:
		return err
	case <-p.stopCh:
		return fmt.Errorf("worker pool was stopped while waiting for task")
	}
}

// QueueSize는 현재 큐에 있는 작업 수를 반환합니다.
func (p *AdaptiveWorkerPool) QueueSize() int {
	return len(p.taskQueue)
}

// WorkerCount는 현재 워커 수를 반환합니다.
func (p *AdaptiveWorkerPool) WorkerCount() int {
	return int(atomic.LoadInt32(&p.numWorkers))
}

// ActiveWorkerCount는 현재 활성 워커 수를 반환합니다.
func (p *AdaptiveWorkerPool) ActiveWorkerCount() int {
	return int(atomic.LoadInt32(&p.activeWorkers))
}

// GetStats는 워커 풀 통계를 반환합니다.
func (p *AdaptiveWorkerPool) GetStats() map[string]interface{} {
	workers := atomic.LoadInt32(&p.numWorkers)
	active := atomic.LoadInt32(&p.activeWorkers)
	processed := atomic.LoadInt64(&p.tasksProcessed)
	errors := atomic.LoadInt64(&p.taskErrors)
	
	errorRate := 0.0
	if processed > 0 {
		errorRate = float64(errors) / float64(processed)
	}
	
	return map[string]interface{}{
		"name":           p.name,
		"workers":        workers,
		"active_workers": active,
		"idle_workers":   workers - active,
		"queue_size":     len(p.taskQueue),
		"queue_capacity": p.queueSize,
		"queue_usage":    float64(len(p.taskQueue)) / float64(p.queueSize),
		"tasks_processed": processed,
		"tasks_errors":   errors,
		"error_rate":     errorRate,
		"queue_high_water": atomic.LoadInt64(&p.queueHighWater),
	}
}

// 내부 메서드

// startWorker는 새 워커를 시작합니다.
func (p *AdaptiveWorkerPool) startWorker() {
	atomic.AddInt32(&p.numWorkers, 1)
	
	go func() {
		for {
			select {
			case task, ok := <-p.taskQueue:
				if !ok {
					return
				}
				
				atomic.AddInt32(&p.activeWorkers, 1)
				err := task()
				atomic.AddInt32(&p.activeWorkers, -1)
				
				atomic.AddInt64(&p.tasksProcessed, 1)
				if err != nil {
					atomic.AddInt64(&p.taskErrors, 1)
					log.Debug("Task error in worker pool", "name", p.name, "err", err)
				}
				
			case <-p.workerStopCh:
				return
			}
		}
	}()
}

// stopWorker는 워커를 중지합니다.
func (p *AdaptiveWorkerPool) stopWorker() {
	atomic.AddInt32(&p.numWorkers, -1)
}

// adjustWorkerCount는 워커 수를 조정합니다.
func (p *AdaptiveWorkerPool) adjustWorkerCount() {
	for {
		select {
		case <-p.stopCh:
			return
		case <-p.adjustTicker.C:
			p.adjustBasedOnLoad()
		}
	}
}

// adjustBasedOnLoad는 부하에 따라 워커 수를 조정합니다.
func (p *AdaptiveWorkerPool) adjustBasedOnLoad() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if !p.isRunning {
		return
	}
	
	currentWorkers := atomic.LoadInt32(&p.numWorkers)
	activeWorkers := atomic.LoadInt32(&p.activeWorkers)
	queueLen := len(p.taskQueue)
	queueUsage := float64(queueLen) / float64(p.queueSize)
	idleWorkers := currentWorkers - activeWorkers
	idleRatio := float64(idleWorkers) / float64(currentWorkers)
	
	// 로그 기록
	log.Debug("Worker pool stats", 
		"name", p.name,
		"workers", currentWorkers, 
		"active", activeWorkers,
		"idle", idleWorkers,
		"idleRatio", idleRatio,
		"queueLen", queueLen,
		"queueUsage", queueUsage)
	
	// 조정 로직
	if queueUsage > scaleUpThreshold && idleRatio < minIdleWorkerRatio {
		// 큐가 거의 가득 차고 유휴 워커가 적으면 확장
		newWorkers := int32(math.Ceil(float64(currentWorkers) * scaleUpFactor))
		if newWorkers > int32(p.maxWorkers) {
			newWorkers = int32(p.maxWorkers)
		}
		
		if newWorkers > currentWorkers {
			workersToAdd := newWorkers - currentWorkers
			log.Info("Scaling up worker pool", "name", p.name, "from", currentWorkers, "to", newWorkers, "adding", workersToAdd)
			
			for i := int32(0); i < workersToAdd; i++ {
				p.startWorker()
			}
		}
	} else if queueUsage < scaleDownThreshold && idleRatio > maxIdleWorkerRatio {
		// 큐가 거의 비어 있고 유휴 워커가 많으면 축소
		newWorkers := int32(math.Floor(float64(currentWorkers) * scaleDownFactor))
		if newWorkers < minWorkers {
			newWorkers = minWorkers
		}
		
		if newWorkers < currentWorkers {
			workersToRemove := currentWorkers - newWorkers
			log.Info("Scaling down worker pool", "name", p.name, "from", currentWorkers, "to", newWorkers, "removing", workersToRemove)
			
			for i := int32(0); i < workersToRemove; i++ {
				p.stopWorker()
			}
		}
	}
}

// collectMetrics는 주기적으로 메트릭스를 수집합니다.
func (p *AdaptiveWorkerPool) collectMetrics() {
	ticker := time.NewTicker(workerPoolMetricsInterval)
	defer ticker.Stop()
	
	var lastProcessed int64 = 0
	
	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			workers := atomic.LoadInt32(&p.numWorkers)
			active := atomic.LoadInt32(&p.activeWorkers)
			queueLen := len(p.taskQueue)
			processed := atomic.LoadInt64(&p.tasksProcessed)
			errors := atomic.LoadInt64(&p.taskErrors)
			
			p.workersGauge.Update(int64(workers))
			p.activeGauge.Update(int64(active))
			p.queueSizeGauge.Update(int64(queueLen))
			
			// 마지막 측정 이후 처리된 작업 수 계산
			delta := processed - lastProcessed
			if delta > 0 {
				p.processedGauge.Inc(delta)
				lastProcessed = processed
			}
			
			errorRate := 0.0
			if processed > 0 {
				errorRate = float64(errors) / float64(processed)
			}
			p.errorRateGauge.Update(errorRate)
		}
	}
} 