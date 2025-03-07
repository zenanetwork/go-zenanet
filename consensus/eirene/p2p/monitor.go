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
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/log"
)

// 모니터링 관련 상수
const (
	// 모니터링 간격
	monitoringInterval = 10 * time.Second
	
	// 통계 저장 간격
	statsStorageInterval = 1 * time.Hour
	
	// 통계 보관 기간
	statsRetentionPeriod = 7 * 24 * time.Hour
	
	// 최대 저장 통계 수
	maxStoredStats = 168 // 7일 * 24시간 (1시간 간격)
	
	// 경고 임계값
	peerCountWarningThreshold = 5     // 최소 피어 수
	blockPropagationWarningThreshold = 5 * time.Second // 블록 전파 시간 임계값
	networkPartitionWarningThreshold = 0.3 // 네트워크 파티션 임계값 (30%)
	
	// 통계 파일 이름
	statsFileName = "network_stats.json"
)

// NetworkMonitor는 P2P 네트워크 모니터링 기능을 제공합니다.
type NetworkMonitor struct {
	peerSet         *PeerSet           // 피어 집합
	discovery       *PeerDiscovery     // 피어 검색
	propagator      *BlockPropagator   // 블록 전파기
	securityManager *SecurityManager   // 보안 관리자
	delayHandler    *NetworkDelayHandler // 네트워크 지연 핸들러
	
	// 네트워크 통계
	stats           *NetworkStats      // 현재 네트워크 통계
	historicalStats []*NetworkStats    // 과거 네트워크 통계
	
	// 경고 상태
	warnings        map[string]bool    // 현재 경고 상태
	
	// 데이터 저장 경로
	dataDir         string             // 데이터 디렉토리
	
	quit            chan struct{}      // 종료 채널
	wg              sync.WaitGroup     // 대기 그룹
	
	lock            sync.RWMutex       // 동시성 제어를 위한 락
	
	logger          log.Logger         // 로거
}

// NetworkStats는 네트워크 통계를 나타냅니다.
type NetworkStats struct {
	Timestamp             time.Time              `json:"timestamp"`
	
	// 피어 관련 통계
	PeerCount             int                    `json:"peer_count"`
	ConnectedPeers        []MonitorPeerInfo      `json:"connected_peers"`
	InboundPeers          int                    `json:"inbound_peers"`
	OutboundPeers         int                    `json:"outbound_peers"`
	
	// 지역 분포
	GeographicDistribution map[string]int        `json:"geographic_distribution"`
	
	// 블록 전파 통계
	AvgBlockPropagationTime time.Duration        `json:"avg_block_propagation_time"`
	MaxBlockPropagationTime time.Duration        `json:"max_block_propagation_time"`
	BlockPropagationSuccess float64              `json:"block_propagation_success"`
	
	// 트랜잭션 전파 통계
	AvgTxPropagationTime   time.Duration        `json:"avg_tx_propagation_time"`
	TxPropagationSuccess   float64              `json:"tx_propagation_success"`
	
	// 네트워크 지연 통계
	AvgNetworkLatency      time.Duration        `json:"avg_network_latency"`
	MaxNetworkLatency      time.Duration        `json:"max_network_latency"`
	
	// 대역폭 사용량
	InboundBandwidth       uint64               `json:"inbound_bandwidth"`
	OutboundBandwidth      uint64               `json:"outbound_bandwidth"`
	
	// 보안 통계
	SecurityStats          map[string]interface{} `json:"security_stats"`
	
	// 네트워크 파티션 감지
	NetworkPartitionRisk   float64              `json:"network_partition_risk"`
}

// MonitorPeerInfo는 모니터링을 위한 피어 정보를 나타냅니다.
type MonitorPeerInfo struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Address       string    `json:"address"`
	Direction     string    `json:"direction"`
	ConnectedTime time.Time `json:"connected_time"`
	Latency       time.Duration `json:"latency"`
	Version       uint      `json:"version"`
	Head          string    `json:"head"`
	TD            *big.Int  `json:"td"`
}

// NewNetworkMonitor는 새로운 네트워크 모니터를 생성합니다.
func NewNetworkMonitor(
	peerSet *PeerSet,
	discovery *PeerDiscovery,
	propagator *BlockPropagator,
	securityManager *SecurityManager,
	dataDir string,
) *NetworkMonitor {
	// 네트워크 지연 핸들러 설정
	delayConfig := NetworkDelayConfig{
		MaxLatencyThreshold:    3 * time.Second,
		LatencyHistorySize:     100,
		AdaptiveTimeoutEnabled: true,
		MinTimeout:             1 * time.Second,
		MaxTimeout:             30 * time.Second,
		TimeoutMultiplier:      2.0,
		PredictionEnabled:      true,
		PredictionWindow:       10,
		PrioritizationEnabled:  true,
		HighPriorityLatencyFactor: 0.8,
		PartitionDetectionEnabled: true,
		PartitionThreshold:     0.5,
	}

	nm := &NetworkMonitor{
		peerSet:         peerSet,
		discovery:       discovery,
		propagator:      propagator,
		securityManager: securityManager,
		delayHandler:    NewNetworkDelayHandler(delayConfig),
		stats:           &NetworkStats{},
		historicalStats: make([]*NetworkStats, 0),
		warnings:        make(map[string]bool),
		dataDir:         dataDir,
		quit:            make(chan struct{}),
		logger:          log.New("module", "network_monitor"),
	}

	return nm
}

// Start는 네트워크 모니터를 시작합니다.
func (nm *NetworkMonitor) Start() {
	nm.logger.Info("Starting network monitor")
	
	// 저장된 통계 로드
	nm.loadStats()
	
	nm.wg.Add(2)
	go nm.monitoringLoop()
	go nm.storageLoop()
}

// Stop은 네트워크 모니터를 중지합니다.
func (nm *NetworkMonitor) Stop() {
	nm.logger.Info("Stopping network monitor")
	
	// 통계 저장
	nm.saveStats()
	
	close(nm.quit)
	nm.wg.Wait()
}

// monitoringLoop는 주기적으로 네트워크 상태를 모니터링합니다.
func (nm *NetworkMonitor) monitoringLoop() {
	defer nm.wg.Done()
	
	ticker := time.NewTicker(monitoringInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			nm.collectStats()
			nm.detectIssues()
		case <-nm.quit:
			return
		}
	}
}

// storageLoop는 주기적으로 통계를 저장합니다.
func (nm *NetworkMonitor) storageLoop() {
	defer nm.wg.Done()
	
	ticker := time.NewTicker(statsStorageInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			nm.saveStats()
		case <-nm.quit:
			return
		}
	}
}

// collectStats는 네트워크 통계를 수집합니다.
func (nm *NetworkMonitor) collectStats() {
	nm.lock.Lock()
	defer nm.lock.Unlock()
	
	// 새 통계 객체 생성
	stats := &NetworkStats{
		Timestamp:             time.Now(),
		GeographicDistribution: make(map[string]int),
		SecurityStats:          make(map[string]interface{}),
	}
	
	// 피어 정보 수집
	peers := nm.peerSet.AllPeers()
	stats.PeerCount = len(peers)
	stats.ConnectedPeers = make([]MonitorPeerInfo, 0, len(peers))
	
	inbound := 0
	outbound := 0
	
	for _, p := range peers {
		// 피어 정보 생성
		info := MonitorPeerInfo{
			ID:            p.ID().String(),
			Name:          p.Name(),
			Address:       p.RemoteAddr().String(),
			ConnectedTime: p.ConnectedTime(),
			Latency:       p.Latency(),
			Version:       p.Version(),
		}
		
		// 방향 설정
		if p.IsInbound() {
			info.Direction = "inbound"
			inbound++
		} else {
			info.Direction = "outbound"
			outbound++
		}
		
		// 헤드 블록 정보 설정 (가능한 경우)
		head := p.Head()
		if head != (common.Hash{}) { // 빈 해시가 아닌 경우
			info.Head = head.Hex()
			info.TD = p.TD()
		}
		
		stats.ConnectedPeers = append(stats.ConnectedPeers, info)
		
		// 지역 분포 업데이트
		region := getRegionFromIP(p.RemoteAddr().String())
		stats.GeographicDistribution[region]++
		
		// 네트워크 지연 핸들러 업데이트
		nm.delayHandler.UpdatePeerLatencyByID(p.ID().String(), p.Latency())
	}
	
	stats.InboundPeers = inbound
	stats.OutboundPeers = outbound
	
	// 블록 전파 통계 수집
	if nm.propagator != nil {
		propStats := nm.propagator.GetPropagationStats()
		if avgTime, ok := propStats["avg_propagation_time"].(time.Duration); ok {
			stats.AvgBlockPropagationTime = avgTime
		}
		if maxTime, ok := propStats["max_propagation_time"].(time.Duration); ok {
			stats.MaxBlockPropagationTime = maxTime
		}
		if successRate, ok := propStats["success_rate"].(float64); ok {
			stats.BlockPropagationSuccess = successRate
		}
		if avgTxTime, ok := propStats["avg_tx_propagation_time"].(time.Duration); ok {
			stats.AvgTxPropagationTime = avgTxTime
		}
		if txSuccessRate, ok := propStats["tx_success_rate"].(float64); ok {
			stats.TxPropagationSuccess = txSuccessRate
		}
	}
	
	// 네트워크 지연 통계 수집
	delayStats := nm.delayHandler.GetNetworkStats()
	stats.AvgNetworkLatency = delayStats["avg_network_latency"].(time.Duration)
	stats.MaxNetworkLatency = delayStats["network_jitter"].(time.Duration)
	stats.NetworkPartitionRisk = delayStats["network_partition_risk"].(float64)
	
	// 대역폭 사용량 수집 (예시)
	stats.InboundBandwidth = 0  // 실제 구현에서는 실제 대역폭 측정
	stats.OutboundBandwidth = 0 // 실제 구현에서는 실제 대역폭 측정
	
	// 보안 통계 수집
	if nm.securityManager != nil {
		stats.SecurityStats = nm.securityManager.GetSecurityStats()
	}
	
	// 현재 통계 업데이트
	nm.stats = stats
	
	// 과거 통계에 추가
	nm.historicalStats = append(nm.historicalStats, stats)
	if len(nm.historicalStats) > 1000 { // 최대 1000개 유지
		nm.historicalStats = nm.historicalStats[1:]
	}
	
	// 이슈 감지
	nm.detectIssues()
}

// getRegionFromIP는 IP 주소에서 지역 정보를 추출합니다.
func getRegionFromIP(ipAddr string) string {
	// 실제 구현에서는 GeoIP 데이터베이스를 사용하여 지역 정보를 추출
	// 여기서는 간단히 "unknown" 반환
	return "unknown"
}

// calculatePartitionRisk는 네트워크 파티션 위험을 계산합니다.
func (nm *NetworkMonitor) calculatePartitionRisk() float64 {
	// 실제 구현에서는 더 복잡한 알고리즘을 사용할 수 있음
	// 여기서는 간단한 휴리스틱 사용
	
	// 피어 수가 적으면 파티션 위험이 높음
	peerCount := nm.peerSet.Len()
	if peerCount < 3 {
		return 0.9 // 90% 위험
	} else if peerCount < 5 {
		return 0.7 // 70% 위험
	} else if peerCount < 10 {
		return 0.5 // 50% 위험
	} else if peerCount < 20 {
		return 0.3 // 30% 위험
	} else {
		return 0.1 // 10% 위험
	}
}

// detectIssues는 네트워크 이슈를 감지합니다.
func (nm *NetworkMonitor) detectIssues() {
	// 경고 초기화
	nm.warnings = make(map[string]bool)
	
	// 피어 수 확인
	if nm.stats.PeerCount < 3 {
		nm.warnings["low_peer_count"] = true
		nm.logger.Warn("Low peer count detected", "count", nm.stats.PeerCount)
	}
	
	// 인바운드/아웃바운드 비율 확인
	if nm.stats.OutboundPeers > 0 && float64(nm.stats.InboundPeers)/float64(nm.stats.OutboundPeers) < 0.2 {
		nm.warnings["inbound_outbound_imbalance"] = true
		nm.logger.Warn("Inbound/outbound peer imbalance detected", 
			"inbound", nm.stats.InboundPeers, 
			"outbound", nm.stats.OutboundPeers)
	}
	
	// 블록 전파 성공률 확인
	if nm.stats.BlockPropagationSuccess < 0.9 {
		nm.warnings["low_block_propagation"] = true
		nm.logger.Warn("Low block propagation success rate", 
			"rate", nm.stats.BlockPropagationSuccess)
	}
	
	// 네트워크 지연 확인
	if nm.stats.AvgNetworkLatency > 2*time.Second {
		nm.warnings["high_network_latency"] = true
		nm.logger.Warn("High network latency detected", 
			"latency", nm.stats.AvgNetworkLatency)
	}
	
	// 네트워크 파티션 확인
	if nm.delayHandler.IsNetworkPartitioned() {
		nm.warnings["network_partition"] = true
		nm.logger.Warn("Network partition risk detected", 
			"risk", nm.stats.NetworkPartitionRisk)
	}
	
	// 지연 시간이 높은 피어 확인
	highLatencyPeers := nm.delayHandler.GetHighLatencyPeers()
	if len(highLatencyPeers) > 0 {
		nm.warnings["high_latency_peers"] = true
		nm.logger.Warn("High latency peers detected", 
			"count", len(highLatencyPeers))
	}
	
	// 보안 이슈 확인
	if nm.securityManager != nil {
		securityIssues := nm.securityManager.GetSecurityStats()
		for issue, value := range securityIssues {
			if active, ok := value.(bool); ok && active {
				nm.warnings[issue] = true
				nm.logger.Warn("Security issue detected", "issue", issue)
			}
		}
	}
}

// saveStats는 통계를 파일에 저장합니다.
func (nm *NetworkMonitor) saveStats() {
	nm.lock.RLock()
	defer nm.lock.RUnlock()
	
	// 데이터 디렉토리가 없으면 생성
	if err := os.MkdirAll(nm.dataDir, 0755); err != nil {
		nm.logger.Error("Failed to create data directory", "dir", nm.dataDir, "error", err)
		return
	}
	
	// 통계 파일 경로
	filePath := filepath.Join(nm.dataDir, statsFileName)
	
	// 통계 직렬화
	data, err := json.MarshalIndent(nm.historicalStats, "", "  ")
	if err != nil {
		nm.logger.Error("Failed to marshal network stats", "error", err)
		return
	}
	
	// 파일에 저장
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		nm.logger.Error("Failed to save network stats", "file", filePath, "error", err)
		return
	}
	
	nm.logger.Debug("Network stats saved", "file", filePath, "count", len(nm.historicalStats))
}

// loadStats는 파일에서 통계를 로드합니다.
func (nm *NetworkMonitor) loadStats() {
	nm.lock.Lock()
	defer nm.lock.Unlock()
	
	// 통계 파일 경로
	filePath := filepath.Join(nm.dataDir, statsFileName)
	
	// 파일이 없으면 무시
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return
	}
	
	// 파일 읽기
	data, err := os.ReadFile(filePath)
	if err != nil {
		nm.logger.Error("Failed to read network stats", "file", filePath, "error", err)
		return
	}
	
	// 통계 역직렬화
	var stats []*NetworkStats
	if err := json.Unmarshal(data, &stats); err != nil {
		nm.logger.Error("Failed to unmarshal network stats", "error", err)
		return
	}
	
	// 오래된 통계 필터링
	now := time.Now()
	var validStats []*NetworkStats
	for _, stat := range stats {
		if now.Sub(stat.Timestamp) <= statsRetentionPeriod {
			validStats = append(validStats, stat)
		}
	}
	
	// 최대 저장 통계 수 제한
	if len(validStats) > maxStoredStats {
		validStats = validStats[len(validStats)-maxStoredStats:]
	}
	
	nm.historicalStats = validStats
	
	nm.logger.Debug("Network stats loaded", "file", filePath, "count", len(nm.historicalStats))
}

// GetCurrentStats는 현재 네트워크 통계를 반환합니다.
func (nm *NetworkMonitor) GetCurrentStats() *NetworkStats {
	nm.lock.RLock()
	defer nm.lock.RUnlock()
	
	return nm.stats
}

// GetHistoricalStats는 과거 네트워크 통계를 반환합니다.
func (nm *NetworkMonitor) GetHistoricalStats(limit int) []*NetworkStats {
	nm.lock.RLock()
	defer nm.lock.RUnlock()
	
	if limit <= 0 || limit > len(nm.historicalStats) {
		limit = len(nm.historicalStats)
	}
	
	result := make([]*NetworkStats, limit)
	copy(result, nm.historicalStats[len(nm.historicalStats)-limit:])
	
	return result
}

// GetWarnings는 현재 경고 상태를 반환합니다.
func (nm *NetworkMonitor) GetWarnings() map[string]bool {
	nm.lock.RLock()
	defer nm.lock.RUnlock()
	
	// 경고 복사
	warnings := make(map[string]bool)
	for k, v := range nm.warnings {
		warnings[k] = v
	}
	
	return warnings
}

// GenerateNetworkReport는 네트워크 보고서를 생성합니다.
func (nm *NetworkMonitor) GenerateNetworkReport() string {
	nm.lock.RLock()
	defer nm.lock.RUnlock()
	
	stats := nm.stats
	
	// 보고서 생성
	report := "=== 네트워크 상태 보고서 ===\n"
	report += fmt.Sprintf("시간: %s\n\n", stats.Timestamp.Format(time.RFC3339))
	
	// 피어 정보
	report += fmt.Sprintf("피어 수: %d (인바운드: %d, 아웃바운드: %d)\n", 
		stats.PeerCount, stats.InboundPeers, stats.OutboundPeers)
	
	// 블록 전파 정보
	report += fmt.Sprintf("평균 블록 전파 시간: %s\n", stats.AvgBlockPropagationTime)
	report += fmt.Sprintf("블록 전파 성공률: %.2f%%\n", stats.BlockPropagationSuccess*100)
	
	// 네트워크 파티션 위험
	report += fmt.Sprintf("네트워크 파티션 위험: %.2f%%\n", stats.NetworkPartitionRisk*100)
	
	// 경고 정보
	activeWarnings := 0
	for _, active := range nm.warnings {
		if active {
			activeWarnings++
		}
	}
	
	if activeWarnings > 0 {
		report += "\n=== 경고 ===\n"
		for warningType, active := range nm.warnings {
			if active {
				report += fmt.Sprintf("- %s\n", warningType)
			}
		}
	}
	
	return report
}

// GetPeerTimeout은 피어의 타임아웃 값을 반환합니다.
func (nm *NetworkMonitor) GetPeerTimeout(peer common.Address) time.Duration {
	return nm.delayHandler.GetPeerTimeout(peer)
}

// GetPrioritizedPeers는 우선순위가 지정된 피어 목록을 반환합니다.
func (nm *NetworkMonitor) GetPrioritizedPeers() map[int][]common.Address {
	return nm.delayHandler.GetPrioritizedPeers()
}

// PredictPeerLatency는 피어의 미래 지연 시간을 예측합니다.
func (nm *NetworkMonitor) PredictPeerLatency(peer common.Address) time.Duration {
	return nm.delayHandler.PredictLatency(peer)
} 