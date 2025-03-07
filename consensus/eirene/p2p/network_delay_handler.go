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
	"math"
	"sort"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/log"
)

// NetworkDelayConfig는 네트워크 지연 핸들러의 설정을 정의합니다.
type NetworkDelayConfig struct {
	// 기본 설정
	MaxLatencyThreshold    time.Duration // 최대 허용 지연 시간
	LatencyHistorySize     int           // 지연 시간 히스토리 크기
	AdaptiveTimeoutEnabled bool          // 적응형 타임아웃 활성화 여부
	
	// 적응형 타임아웃 설정
	MinTimeout             time.Duration // 최소 타임아웃
	MaxTimeout             time.Duration // 최대 타임아웃
	TimeoutMultiplier      float64       // 타임아웃 승수
	
	// 지연 시간 예측 설정
	PredictionEnabled      bool          // 지연 시간 예측 활성화 여부
	PredictionWindow       int           // 예측 윈도우 크기
	
	// 우선순위 설정
	PrioritizationEnabled  bool          // 우선순위 지정 활성화 여부
	HighPriorityLatencyFactor float64    // 높은 우선순위 지연 시간 계수
	
	// 네트워크 파티션 감지 설정
	PartitionDetectionEnabled bool        // 네트워크 파티션 감지 활성화 여부
	PartitionThreshold     float64       // 파티션 임계값
}

// PeerLatencyInfo는 피어의 지연 시간 정보를 저장합니다.
type PeerLatencyInfo struct {
	Address        common.Address  // 피어 주소
	CurrentLatency time.Duration   // 현재 지연 시간
	AvgLatency     time.Duration   // 평균 지연 시간
	MinLatency     time.Duration   // 최소 지연 시간
	MaxLatency     time.Duration   // 최대 지연 시간
	LatencyHistory []time.Duration // 지연 시간 히스토리
	LastUpdated    time.Time       // 마지막 업데이트 시간
	Timeout        time.Duration   // 현재 타임아웃
	Priority       int             // 우선순위 (1: 높음, 2: 중간, 3: 낮음)
	Reliability    float64         // 신뢰도 (0.0 ~ 1.0)
}

// NetworkDelayHandler는 네트워크 지연을 처리하고 대응하는 구조체입니다.
type NetworkDelayHandler struct {
	config         NetworkDelayConfig // 설정
	peerLatencies  map[common.Address]*PeerLatencyInfo // 피어별 지연 시간 정보
	
	// 네트워크 상태
	avgNetworkLatency time.Duration // 전체 네트워크 평균 지연 시간
	networkJitter     time.Duration // 네트워크 지터
	networkPartitionRisk float64    // 네트워크 파티션 위험도
	
	// 통계
	latencyDistribution map[string]int // 지연 시간 분포
	
	// 동시성 제어
	lock sync.RWMutex
	
	// 로깅
	logger log.Logger
}

// NewNetworkDelayHandler는 새로운 NetworkDelayHandler 인스턴스를 생성합니다.
func NewNetworkDelayHandler(config NetworkDelayConfig) *NetworkDelayHandler {
	if config.LatencyHistorySize <= 0 {
		config.LatencyHistorySize = 100
	}
	if config.MaxLatencyThreshold <= 0 {
		config.MaxLatencyThreshold = 5 * time.Second
	}
	if config.MinTimeout <= 0 {
		config.MinTimeout = 1 * time.Second
	}
	if config.MaxTimeout <= 0 {
		config.MaxTimeout = 30 * time.Second
	}
	if config.TimeoutMultiplier <= 0 {
		config.TimeoutMultiplier = 2.0
	}
	if config.PredictionWindow <= 0 {
		config.PredictionWindow = 10
	}
	if config.HighPriorityLatencyFactor <= 0 {
		config.HighPriorityLatencyFactor = 0.8
	}
	if config.PartitionThreshold <= 0 {
		config.PartitionThreshold = 0.5
	}
	
	return &NetworkDelayHandler{
		config:        config,
		peerLatencies: make(map[common.Address]*PeerLatencyInfo),
		latencyDistribution: make(map[string]int),
		logger:        log.New("module", "network_delay_handler"),
	}
}

// UpdatePeerLatency는 피어의 지연 시간을 업데이트합니다.
func (h *NetworkDelayHandler) UpdatePeerLatency(peer common.Address, latency time.Duration) {
	h.lock.Lock()
	defer h.lock.Unlock()
	
	info, exists := h.peerLatencies[peer]
	if !exists {
		// 새 피어 정보 생성
		info = &PeerLatencyInfo{
			Address:        peer,
			CurrentLatency: latency,
			AvgLatency:     latency,
			MinLatency:     latency,
			MaxLatency:     latency,
			LatencyHistory: make([]time.Duration, 0, h.config.LatencyHistorySize),
			LastUpdated:    time.Now(),
			Timeout:        h.config.MinTimeout,
			Priority:       2, // 기본 우선순위: 중간
		}
		h.peerLatencies[peer] = info
	} else {
		// 기존 피어 정보 업데이트
		info.CurrentLatency = latency
		info.LastUpdated = time.Now()
		
		// 최소/최대 지연 시간 업데이트
		if latency < info.MinLatency {
			info.MinLatency = latency
		}
		if latency > info.MaxLatency {
			info.MaxLatency = latency
		}
		
		// 지연 시간 히스토리 업데이트
		info.LatencyHistory = append(info.LatencyHistory, latency)
		if len(info.LatencyHistory) > h.config.LatencyHistorySize {
			// 가장 오래된 항목 제거
			info.LatencyHistory = info.LatencyHistory[1:]
		}
		
		// 평균 지연 시간 계산
		var sum time.Duration
		for _, l := range info.LatencyHistory {
			sum += l
		}
		info.AvgLatency = sum / time.Duration(len(info.LatencyHistory))
		
		// 타임아웃 업데이트 (적응형 타임아웃이 활성화된 경우)
		if h.config.AdaptiveTimeoutEnabled {
			h.updatePeerTimeout(info)
		}
	}
	
	// 우선순위 업데이트
	if h.config.PrioritizationEnabled {
		h.updatePeerPriority(info)
	}
	
	// 전체 네트워크 통계 업데이트
	h.updateNetworkStats()
}

// UpdatePeerLatencyByID는 피어 ID를 사용하여 피어의 지연 시간을 업데이트합니다.
func (h *NetworkDelayHandler) UpdatePeerLatencyByID(peerID string, latency time.Duration) {
	// 피어 ID를 해시하여 주소로 변환
	addr := common.HexToAddress(peerID)
	h.UpdatePeerLatency(addr, latency)
}

// GetPeerTimeout은 피어의 현재 타임아웃 값을 반환합니다.
func (h *NetworkDelayHandler) GetPeerTimeout(peer common.Address) time.Duration {
	h.lock.RLock()
	defer h.lock.RUnlock()
	
	info, exists := h.peerLatencies[peer]
	if !exists {
		return h.config.MinTimeout
	}
	
	return info.Timeout
}

// GetPeerLatency는 피어의 지연 시간 정보를 반환합니다.
func (h *NetworkDelayHandler) GetPeerLatency(peer common.Address) *PeerLatencyInfo {
	h.lock.RLock()
	defer h.lock.RUnlock()
	
	info, exists := h.peerLatencies[peer]
	if !exists {
		return nil
	}
	
	// 복사본 반환
	result := *info
	result.LatencyHistory = make([]time.Duration, len(info.LatencyHistory))
	copy(result.LatencyHistory, info.LatencyHistory)
	
	return &result
}

// PredictLatency는 피어의 미래 지연 시간을 예측합니다.
func (h *NetworkDelayHandler) PredictLatency(peer common.Address) time.Duration {
	if !h.config.PredictionEnabled {
		return 0
	}
	
	h.lock.RLock()
	defer h.lock.RUnlock()
	
	info, exists := h.peerLatencies[peer]
	if !exists || len(info.LatencyHistory) < h.config.PredictionWindow {
		return 0
	}
	
	// 선형 회귀를 사용한 간단한 예측
	n := len(info.LatencyHistory)
	window := h.config.PredictionWindow
	if window > n {
		window = n
	}
	
	// 최근 window 개의 샘플 사용
	samples := info.LatencyHistory[n-window:]
	
	// 선형 회귀 계수 계산
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0
	
	for i, latency := range samples {
		x := float64(i)
		y := float64(latency)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	
	n64 := float64(window)
	slope := (n64*sumXY - sumX*sumY) / (n64*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / n64
	
	// 다음 값 예측
	nextX := float64(window)
	predictedY := slope*nextX + intercept
	
	if predictedY < 0 {
		predictedY = 0
	}
	
	return time.Duration(predictedY)
}

// GetNetworkStats는 네트워크 지연 통계를 반환합니다.
func (h *NetworkDelayHandler) GetNetworkStats() map[string]interface{} {
	h.lock.RLock()
	defer h.lock.RUnlock()
	
	return map[string]interface{}{
		"avg_network_latency":    h.avgNetworkLatency,
		"network_jitter":         h.networkJitter,
		"network_partition_risk": h.networkPartitionRisk,
		"latency_distribution":   h.latencyDistribution,
		"peer_count":             len(h.peerLatencies),
	}
}

// GetHighLatencyPeers는 지연 시간이 높은 피어 목록을 반환합니다.
func (h *NetworkDelayHandler) GetHighLatencyPeers() []common.Address {
	h.lock.RLock()
	defer h.lock.RUnlock()
	
	var highLatencyPeers []common.Address
	
	for addr, info := range h.peerLatencies {
		if info.AvgLatency > h.config.MaxLatencyThreshold {
			highLatencyPeers = append(highLatencyPeers, addr)
		}
	}
	
	return highLatencyPeers
}

// GetPrioritizedPeers는 우선순위가 지정된 피어 목록을 반환합니다.
func (h *NetworkDelayHandler) GetPrioritizedPeers() map[int][]common.Address {
	h.lock.RLock()
	defer h.lock.RUnlock()
	
	result := make(map[int][]common.Address)
	
	for addr, info := range h.peerLatencies {
		priority := info.Priority
		if _, exists := result[priority]; !exists {
			result[priority] = make([]common.Address, 0)
		}
		result[priority] = append(result[priority], addr)
	}
	
	return result
}

// IsNetworkPartitioned는 네트워크가 파티션되었는지 여부를 반환합니다.
func (h *NetworkDelayHandler) IsNetworkPartitioned() bool {
	h.lock.RLock()
	defer h.lock.RUnlock()
	
	return h.networkPartitionRisk > h.config.PartitionThreshold
}

// 내부 함수: 적응형 타임아웃 업데이트
func (h *NetworkDelayHandler) updatePeerTimeout(info *PeerLatencyInfo) {
	// 평균 지연 시간 기반 타임아웃 계산
	newTimeout := time.Duration(float64(info.AvgLatency) * h.config.TimeoutMultiplier)
	
	// 지터 고려
	if len(info.LatencyHistory) > 1 {
		var variance float64
		for _, latency := range info.LatencyHistory {
			diff := float64(latency - info.AvgLatency)
			variance += diff * diff
		}
		variance /= float64(len(info.LatencyHistory))
		stdDev := time.Duration(math.Sqrt(variance))
		
		// 표준 편차를 타임아웃에 추가
		newTimeout += stdDev
	}
	
	// 범위 제한
	if newTimeout < h.config.MinTimeout {
		newTimeout = h.config.MinTimeout
	}
	if newTimeout > h.config.MaxTimeout {
		newTimeout = h.config.MaxTimeout
	}
	
	// 우선순위 고려
	if h.config.PrioritizationEnabled && info.Priority == 1 {
		// 높은 우선순위 피어는 더 짧은 타임아웃 사용
		newTimeout = time.Duration(float64(newTimeout) * h.config.HighPriorityLatencyFactor)
	}
	
	info.Timeout = newTimeout
}

// 내부 함수: 피어 우선순위 업데이트
func (h *NetworkDelayHandler) updatePeerPriority(info *PeerLatencyInfo) {
	// 지연 시간 기반 우선순위 계산
	// 낮은 지연 시간 = 높은 우선순위
	
	// 신뢰도 계산 (지연 시간 변동성 기반)
	if len(info.LatencyHistory) > 1 {
		var variance float64
		for _, latency := range info.LatencyHistory {
			diff := float64(latency - info.AvgLatency)
			variance += diff * diff
		}
		variance /= float64(len(info.LatencyHistory))
		stdDev := math.Sqrt(variance)
		
		// 변동성이 낮을수록 신뢰도가 높음
		avgLatencyFloat := float64(info.AvgLatency)
		if avgLatencyFloat > 0 {
			variationCoeff := stdDev / avgLatencyFloat
			info.Reliability = 1.0 / (1.0 + variationCoeff)
		}
	}
	
	// 우선순위 결정
	if info.AvgLatency < h.avgNetworkLatency/2 && info.Reliability > 0.8 {
		info.Priority = 1 // 높음
	} else if info.AvgLatency > h.avgNetworkLatency*2 || info.Reliability < 0.3 {
		info.Priority = 3 // 낮음
	} else {
		info.Priority = 2 // 중간
	}
}

// 내부 함수: 네트워크 통계 업데이트
func (h *NetworkDelayHandler) updateNetworkStats() {
	if len(h.peerLatencies) == 0 {
		return
	}
	
	// 평균 지연 시간 계산
	var sum time.Duration
	var latencies []time.Duration
	
	for _, info := range h.peerLatencies {
		sum += info.AvgLatency
		latencies = append(latencies, info.AvgLatency)
	}
	
	h.avgNetworkLatency = sum / time.Duration(len(h.peerLatencies))
	
	// 지연 시간 분포 계산
	h.latencyDistribution = make(map[string]int)
	for _, latency := range latencies {
		var category string
		switch {
		case latency < 100*time.Millisecond:
			category = "< 100ms"
		case latency < 200*time.Millisecond:
			category = "100-200ms"
		case latency < 500*time.Millisecond:
			category = "200-500ms"
		case latency < 1*time.Second:
			category = "500ms-1s"
		default:
			category = "> 1s"
		}
		h.latencyDistribution[category]++
	}
	
	// 네트워크 지터 계산
	if len(latencies) > 1 {
		sort.Slice(latencies, func(i, j int) bool {
			return latencies[i] < latencies[j]
		})
		
		// 사분위수 범위를 지터로 사용
		q1Idx := len(latencies) / 4
		q3Idx := len(latencies) * 3 / 4
		
		q1 := latencies[q1Idx]
		q3 := latencies[q3Idx]
		
		h.networkJitter = q3 - q1
	}
	
	// 네트워크 파티션 위험도 계산
	if h.config.PartitionDetectionEnabled {
		h.detectNetworkPartition(latencies)
	}
}

// 내부 함수: 네트워크 파티션 감지
func (h *NetworkDelayHandler) detectNetworkPartition(latencies []time.Duration) {
	if len(latencies) < 3 {
		h.networkPartitionRisk = 0
		return
	}
	
	// 지연 시간 분포 분석
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})
	
	// 중앙값
	median := latencies[len(latencies)/2]
	
	// 지연 시간 클러스터 감지
	clusters := make(map[time.Duration]int)
	for _, latency := range latencies {
		// 가장 가까운 클러스터 찾기
		var closestCluster time.Duration
		minDiff := time.Hour
		
		for cluster := range clusters {
			diff := absDuration(latency - cluster)
			if diff < minDiff {
				minDiff = diff
				closestCluster = cluster
			}
		}
		
		// 클러스터 임계값 (중앙값의 20%)
		threshold := time.Duration(float64(median) * 0.2)
		
		if minDiff <= threshold {
			// 기존 클러스터에 추가
			clusters[closestCluster]++
		} else {
			// 새 클러스터 생성
			clusters[latency] = 1
		}
	}
	
	// 클러스터 수가 많을수록 파티션 위험도 증가
	clusterCount := len(clusters)
	
	// 클러스터 간 격차 분석
	var clusterCenters []time.Duration
	for cluster := range clusters {
		clusterCenters = append(clusterCenters, cluster)
	}
	
	sort.Slice(clusterCenters, func(i, j int) bool {
		return clusterCenters[i] < clusterCenters[j]
	})
	
	var maxGap time.Duration
	for i := 1; i < len(clusterCenters); i++ {
		gap := clusterCenters[i] - clusterCenters[i-1]
		if gap > maxGap {
			maxGap = gap
		}
	}
	
	// 최대 격차가 중앙값의 2배 이상이면 파티션 가능성 높음
	gapFactor := float64(maxGap) / float64(median)
	
	// 파티션 위험도 계산 (클러스터 수와 격차 기반)
	h.networkPartitionRisk = math.Min(1.0, float64(clusterCount-1)*0.2 + math.Max(0, gapFactor-1.0)*0.5)
	
	if h.networkPartitionRisk > h.config.PartitionThreshold {
		h.logger.Warn("Network partition risk detected", 
			"risk", h.networkPartitionRisk, 
			"clusters", clusterCount,
			"max_gap", maxGap,
			"median_latency", median)
	}
}

// 내부 함수: 지속 시간의 절대값 계산
func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
} 