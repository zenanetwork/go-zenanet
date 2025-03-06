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
	"context"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/p2p/discover"
	"github.com/zenanetwork/go-zenanet/p2p/enode"
)

// 피어 검색 관련 상수
const (
	// 검색 간격
	discoveryInterval = 30 * time.Second
	
	// 최대 검색 시도 횟수
	maxDiscoveryAttempts = 5
	
	// 검색 타임아웃
	discoveryTimeout = 10 * time.Second
	
	// 최소 피어 수
	minPeers = 10
	
	// 최대 저장 노드 수
	maxStoredNodes = 200
	
	// 노드 유효 기간
	nodeValidityPeriod = 24 * time.Hour
	
	// 지역 기반 검색 관련
	localPeerPreference = 0.3 // 30%의 피어는 같은 지역에서 선택
	
	// 연결 재시도 관련
	initialRetryDelay = 5 * time.Second
	maxRetryDelay = 5 * time.Minute
	retryBackoffFactor = 2.0
	
	// 피어 품질 평가 관련
	latencyWeight = 0.3
	uptimeWeight = 0.3
	bandwidthWeight = 0.2
	reliabilityWeight = 0.2
)

// PeerDiscovery는 피어 검색 메커니즘을 구현합니다.
type PeerDiscovery struct {
	localNode  *enode.LocalNode   // 로컬 노드
	table      *discover.UDPv4    // 디스커버리 테이블
	bootnodes  []*enode.Node      // 부트노드 목록
	
	knownNodes map[enode.ID]*discoveredNode // 알려진 노드 맵
	
	peerSet    *PeerSet           // 피어 집합
	
	quit       chan struct{}      // 종료 채널
	wg         sync.WaitGroup     // 대기 그룹
	
	lock       sync.RWMutex       // 동시성 제어를 위한 락
	
	logger     log.Logger         // 로거
	
	// 지역 기반 검색 관련
	localRegion string            // 로컬 노드의 지역
	regionCache map[string]string // IP -> 지역 캐시
	
	// 피어 품질 평가 관련
	peerQuality map[enode.ID]*peerQualityMetrics // 피어 품질 메트릭
}

// discoveredNode는 발견된 노드 정보를 저장합니다.
type discoveredNode struct {
	node      *enode.Node         // 노드 정보
	lastSeen  time.Time           // 마지막으로 본 시간
	attempts  int                 // 연결 시도 횟수
	validated bool                // 유효성 검증 여부
	score     int                 // 노드 점수 (높을수록 좋음)
	region    string              // 노드의 지역
	nextRetry time.Time           // 다음 연결 시도 시간
}

// peerQualityMetrics는 피어 품질 평가 메트릭을 저장합니다.
type peerQualityMetrics struct {
	latency     time.Duration // 평균 지연 시간
	uptime      time.Duration // 업타임
	bandwidth   float64       // 대역폭 (bytes/s)
	reliability float64       // 신뢰성 (0-1)
	lastUpdated time.Time     // 마지막 업데이트 시간
}

// NewPeerDiscovery는 새로운 피어 검색 인스턴스를 생성합니다.
func NewPeerDiscovery(localNode *enode.LocalNode, table *discover.UDPv4, bootnodes []*enode.Node, peerSet *PeerSet) *PeerDiscovery {
	pd := &PeerDiscovery{
		localNode:   localNode,
		table:       table,
		bootnodes:   bootnodes,
		knownNodes:  make(map[enode.ID]*discoveredNode),
		peerSet:     peerSet,
		quit:        make(chan struct{}),
		logger:      log.New("module", "p2p/discovery"),
		regionCache: make(map[string]string),
		peerQuality: make(map[enode.ID]*peerQualityMetrics),
	}
	
	// 로컬 노드의 지역 결정
	if ip := localNode.Node().IP(); ip != nil {
		pd.localRegion = pd.getRegionForIP(ip)
	}
	
	return pd
}

// Start는 피어 검색을 시작합니다.
func (pd *PeerDiscovery) Start() {
	pd.logger.Info("Starting peer discovery", "bootnodes", len(pd.bootnodes))
	
	// 부트노드 추가
	for _, node := range pd.bootnodes {
		pd.addNode(node, true)
	}
	
	pd.wg.Add(1)
	go pd.loop()
}

// Stop은 피어 검색을 중지합니다.
func (pd *PeerDiscovery) Stop() {
	pd.logger.Info("Stopping peer discovery")
	close(pd.quit)
	pd.wg.Wait()
}

// loop는 주기적으로 피어를 검색합니다.
func (pd *PeerDiscovery) loop() {
	defer pd.wg.Done()
	
	ticker := time.NewTicker(discoveryInterval)
	defer ticker.Stop()
	
	// 초기 검색 즉시 실행
	pd.discoverPeers()
	
	for {
		select {
		case <-ticker.C:
			pd.discoverPeers()
		case <-pd.quit:
			return
		}
	}
}

// discoverPeers는 새로운 피어를 검색합니다.
func (pd *PeerDiscovery) discoverPeers() {
	pd.lock.RLock()
	peerCount := pd.peerSet.Len()
	pd.lock.RUnlock()
	
	// 충분한 피어가 있는 경우 검색 빈도 감소
	if peerCount >= minPeers*2 {
		// 이미 충분한 피어가 있으므로 로깅만 하고 반환
		pd.logger.Debug("Sufficient peers connected", "count", peerCount, "min", minPeers)
		return
	}
	
	// 필요한 피어 수 계산
	neededPeers := minPeers - peerCount
	if neededPeers <= 0 {
		neededPeers = 1 // 최소 1개는 검색
	}
	
	// 컨텍스트 생성
	ctx, cancel := context.WithTimeout(context.Background(), discoveryTimeout)
	defer cancel()
	
	// 랜덤 노드 검색
	randomNodes := pd.findRandomNodes(ctx, neededPeers*2) // 필요한 수의 2배를 검색
	
	// 검색된 노드 추가
	for _, node := range randomNodes {
		pd.addNode(node, false)
	}
	
	// 지역 기반 노드 검색 추가
	if pd.localRegion != "" {
		localNodes := pd.findNodesInRegion(ctx, pd.localRegion, neededPeers)
		for _, node := range localNodes {
			pd.addNode(node, false)
		}
	}
	
	// 최적의 노드에 연결 시도
	pd.connectToBestNodes(neededPeers)
	
	// 오래된 노드 정리
	pd.cleanupNodes()
}

// findRandomNodes는 랜덤 노드를 검색합니다.
func (pd *PeerDiscovery) findRandomNodes(ctx context.Context, count int) []*enode.Node {
	var nodes []*enode.Node
	
	// 디스커버리 테이블에서 랜덤 노드 검색
	if pd.table != nil {
		iterator := pd.table.RandomNodes()
		for i := 0; i < count && iterator.Next(); i++ {
			nodes = append(nodes, iterator.Node())
		}
		iterator.Close()
	}
	
	pd.logger.Debug("Found random nodes", "count", len(nodes))
	return nodes
}

// findNodesInRegion은 특정 지역의 노드를 검색합니다.
func (pd *PeerDiscovery) findNodesInRegion(ctx context.Context, region string, count int) []*enode.Node {
	var nodesInRegion []*enode.Node
	
	// 이미 알고 있는 노드 중에서 해당 지역의 노드 찾기
	pd.lock.RLock()
	for _, node := range pd.knownNodes {
		if node.region == region {
			nodesInRegion = append(nodesInRegion, node.node)
			if len(nodesInRegion) >= count {
				break
			}
		}
	}
	pd.lock.RUnlock()
	
	pd.logger.Debug("Found nodes in region", "region", region, "count", len(nodesInRegion))
	return nodesInRegion
}

// addNode는 새로운 노드를 추가합니다.
func (pd *PeerDiscovery) addNode(node *enode.Node, isBootnode bool) {
	pd.lock.Lock()
	defer pd.lock.Unlock()
	
	id := node.ID()
	
	// 이미 알고 있는 노드인 경우 업데이트
	if existing, ok := pd.knownNodes[id]; ok {
		existing.lastSeen = time.Now()
		if isBootnode {
			existing.score += 10 // 부트노드는 높은 점수 부여
			existing.validated = true
		}
		return
	}
	
	// 새 노드 추가
	score := 0
	if isBootnode {
		score = 10 // 부트노드는 높은 점수 부여
	}
	
	// 노드의 지역 결정
	region := ""
	if ip := node.IP(); ip != nil {
		region = pd.getRegionForIP(ip)
		
		// 같은 지역의 노드에 추가 점수 부여
		if region == pd.localRegion {
			score += 5
		}
	}
	
	pd.knownNodes[id] = &discoveredNode{
		node:      node,
		lastSeen:  time.Now(),
		attempts:  0,
		validated: isBootnode,
		score:     score,
		region:    region,
		nextRetry: time.Now(),
	}
	
	// 노드 수 제한
	if len(pd.knownNodes) > maxStoredNodes {
		pd.cleanupNodes()
	}
}

// cleanupNodes는 오래된 노드를 정리합니다.
func (pd *PeerDiscovery) cleanupNodes() {
	now := time.Now()
	
	// 오래된 노드 또는 점수가 낮은 노드 제거
	var nodesToRemove []enode.ID
	for id, node := range pd.knownNodes {
		// 유효 기간이 지난 노드
		if now.Sub(node.lastSeen) > nodeValidityPeriod {
			nodesToRemove = append(nodesToRemove, id)
			continue
		}
		
		// 연결 시도 횟수가 많고 점수가 낮은 노드
		if node.attempts > maxDiscoveryAttempts && node.score < 0 {
			nodesToRemove = append(nodesToRemove, id)
			continue
		}
	}
	
	// 노드 제거
	for _, id := range nodesToRemove {
		delete(pd.knownNodes, id)
	}
	
	// 노드 수가 여전히 많으면 점수가 낮은 노드부터 제거
	if len(pd.knownNodes) > maxStoredNodes {
		var nodes []*discoveredNode
		for _, node := range pd.knownNodes {
			nodes = append(nodes, node)
		}
		
		// 점수에 따라 정렬
		sortNodesByScore(nodes)
		
		// 제한 초과 노드 제거
		for i := maxStoredNodes; i < len(nodes); i++ {
			delete(pd.knownNodes, nodes[i].node.ID())
		}
	}
	
	pd.logger.Debug("Cleaned up nodes", "removed", len(nodesToRemove), "remaining", len(pd.knownNodes))
}

// connectToBestNodes는 최적의 노드에 연결을 시도합니다.
func (pd *PeerDiscovery) connectToBestNodes(count int) {
	pd.lock.Lock()
	defer pd.lock.Unlock()
	
	now := time.Now()
	
	// 연결 가능한 노드 필터링
	var connectableNodes []*discoveredNode
	for _, node := range pd.knownNodes {
		// 이미 연결된 노드는 제외
		if pd.peerSet.Peer(node.node.ID().String()) != nil {
			continue
		}
		
		// 다음 재시도 시간이 지나지 않은 노드는 제외
		if now.Before(node.nextRetry) {
			continue
		}
		
		connectableNodes = append(connectableNodes, node)
	}
	
	// 점수에 따라 정렬
	sortNodesByScore(connectableNodes)
	
	// 지역 기반 선택을 위한 노드 분류
	var localNodes, remoteNodes []*discoveredNode
	for _, node := range connectableNodes {
		if node.region == pd.localRegion {
			localNodes = append(localNodes, node)
		} else {
			remoteNodes = append(remoteNodes, node)
		}
	}
	
	// 지역 기반 선택 비율 계산
	localCount := int(float64(count) * localPeerPreference)
	if localCount < 1 {
		localCount = 1
	}
	remoteCount := count - localCount
	
	// 최종 선택 노드
	var selectedNodes []*discoveredNode
	
	// 로컬 노드 선택
	for i := 0; i < localCount && i < len(localNodes); i++ {
		selectedNodes = append(selectedNodes, localNodes[i])
	}
	
	// 원격 노드 선택
	for i := 0; i < remoteCount && i < len(remoteNodes); i++ {
		selectedNodes = append(selectedNodes, remoteNodes[i])
	}
	
	// 부족한 경우 나머지 노드로 채움
	if len(selectedNodes) < count {
		remaining := connectableNodes
		for _, node := range selectedNodes {
			for i, n := range remaining {
				if n.node.ID() == node.node.ID() {
					remaining = append(remaining[:i], remaining[i+1:]...)
					break
				}
			}
		}
		
		for i := 0; i < count-len(selectedNodes) && i < len(remaining); i++ {
			selectedNodes = append(selectedNodes, remaining[i])
		}
	}
	
	// 선택된 노드에 연결 시도
	for _, node := range selectedNodes {
		// 연결 시도 횟수 증가
		node.attempts++
		
		// 다음 재시도 시간 계산 (지수 백오프)
		backoffFactor := 1 << uint(node.attempts-1)
		retryDelay := initialRetryDelay * time.Duration(float64(backoffFactor) * retryBackoffFactor)
		if retryDelay > maxRetryDelay {
			retryDelay = maxRetryDelay
		}
		node.nextRetry = now.Add(retryDelay)
		
		// 비동기적으로 연결 시도
		go pd.tryConnect(node.node)
	}
	
	pd.logger.Debug("Connecting to best nodes", "count", len(selectedNodes), "local", len(localNodes), "remote", len(remoteNodes))
}

// tryConnect는 노드에 연결을 시도합니다.
func (pd *PeerDiscovery) tryConnect(node *enode.Node) {
	id := node.ID()
	
	pd.logger.Debug("Trying to connect to node", "id", id.String()[:8], "addr", node.IP().String())
	
	// 실제 연결 로직은 PeerSet에 위임
	// 여기서는 간단히 로깅만 수행
	
	// 연결 성공 시 노드 점수 증가
	pd.lock.Lock()
	if n, ok := pd.knownNodes[id]; ok {
		n.validated = true
		n.score += 2
	}
	pd.lock.Unlock()
}

// sortNodesByScore는 노드를 점수에 따라 정렬합니다.
func sortNodesByScore(nodes []*discoveredNode) {
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].score > nodes[j].score // 높은 점수가 앞으로
	})
}

// UpdateNodeScore는 노드의 점수를 업데이트합니다.
func (pd *PeerDiscovery) UpdateNodeScore(id enode.ID, delta int) {
	pd.lock.Lock()
	defer pd.lock.Unlock()
	
	if node, ok := pd.knownNodes[id]; ok {
		node.score += delta
		
		// 점수 범위 제한
		if node.score < -100 {
			node.score = -100
		} else if node.score > 100 {
			node.score = 100
		}
		
		pd.logger.Debug("Updated node score", "id", id.String()[:8], "delta", delta, "new_score", node.score)
	}
}

// UpdatePeerQuality는 피어 품질 메트릭을 업데이트합니다.
func (pd *PeerDiscovery) UpdatePeerQuality(id enode.ID, latency time.Duration, bandwidth float64, reliability float64) {
	pd.lock.Lock()
	defer pd.lock.Unlock()
	
	metrics, ok := pd.peerQuality[id]
	if !ok {
		metrics = &peerQualityMetrics{
			uptime:      0,
			lastUpdated: time.Now(),
		}
		pd.peerQuality[id] = metrics
	}
	
	// 메트릭 업데이트
	metrics.latency = latency
	metrics.bandwidth = bandwidth
	metrics.reliability = reliability
	
	// 업타임 업데이트
	now := time.Now()
	if !metrics.lastUpdated.IsZero() {
		metrics.uptime += now.Sub(metrics.lastUpdated)
	}
	metrics.lastUpdated = now
	
	// 품질 점수 계산 및 노드 점수 업데이트
	qualityScore := pd.calculateQualityScore(metrics)
	scoreAdjustment := int(qualityScore*10) - 5 // -5 ~ +5 범위로 조정
	
	if node, ok := pd.knownNodes[id]; ok {
		node.score += scoreAdjustment
	}
}

// calculateQualityScore는 피어 품질 점수를 계산합니다 (0-1 범위).
func (pd *PeerDiscovery) calculateQualityScore(metrics *peerQualityMetrics) float64 {
	// 지연 시간 점수 (낮을수록 좋음)
	latencyScore := 0.0
	if metrics.latency > 0 {
		// 1초 이상은 0점, 50ms 이하는 1점
		latencyScore = 1.0 - float64(metrics.latency) / float64(time.Second)
		if latencyScore < 0 {
			latencyScore = 0
		} else if latencyScore > 1 {
			latencyScore = 1
		}
	}
	
	// 업타임 점수 (높을수록 좋음)
	uptimeScore := 0.0
	if metrics.uptime > 0 {
		// 1시간 이상은 1점
		uptimeScore = float64(metrics.uptime) / float64(time.Hour)
		if uptimeScore > 1 {
			uptimeScore = 1
		}
	}
	
	// 대역폭 점수 (높을수록 좋음)
	bandwidthScore := 0.0
	if metrics.bandwidth > 0 {
		// 1MB/s 이상은 1점
		bandwidthScore = metrics.bandwidth / (1024 * 1024)
		if bandwidthScore > 1 {
			bandwidthScore = 1
		}
	}
	
	// 신뢰성 점수 (이미 0-1 범위)
	reliabilityScore := metrics.reliability
	
	// 가중 평균 계산
	return latencyScore*latencyWeight + uptimeScore*uptimeWeight + 
	       bandwidthScore*bandwidthWeight + reliabilityScore*reliabilityWeight
}

// getRegionForIP는 IP 주소의 지역을 반환합니다.
func (pd *PeerDiscovery) getRegionForIP(ip net.IP) string {
	ipStr := ip.String()
	
	// 캐시에서 확인
	pd.lock.RLock()
	if region, ok := pd.regionCache[ipStr]; ok {
		pd.lock.RUnlock()
		return region
	}
	pd.lock.RUnlock()
	
	// 간단한 지역 결정 로직 (실제로는 GeoIP 데이터베이스 사용)
	region := "unknown"
	
	// 사설 IP 범위 확인
	if ip.IsPrivate() || ip.IsLoopback() {
		region = "local"
	} else {
		// 첫 바이트로 대략적인 지역 추정 (실제 구현에서는 GeoIP 사용)
		firstByte := ip[0]
		if firstByte < 50 {
			region = "north_america"
		} else if firstByte < 100 {
			region = "europe"
		} else if firstByte < 150 {
			region = "asia"
		} else if firstByte < 200 {
			region = "south_america"
		} else {
			region = "oceania"
		}
	}
	
	// 캐시에 저장
	pd.lock.Lock()
	pd.regionCache[ipStr] = region
	pd.lock.Unlock()
	
	return region
}

// GetKnownNodesCount는 알려진 노드 수를 반환합니다.
func (pd *PeerDiscovery) GetKnownNodesCount() int {
	pd.lock.RLock()
	defer pd.lock.RUnlock()
	return len(pd.knownNodes)
}

// GetValidatedNodesCount는 유효성이 검증된 노드 수를 반환합니다.
func (pd *PeerDiscovery) GetValidatedNodesCount() int {
	pd.lock.RLock()
	defer pd.lock.RUnlock()
	
	count := 0
	for _, node := range pd.knownNodes {
		if node.validated {
			count++
		}
	}
	
	return count
}

// GetNodesByRegion은 지역별 노드 수를 반환합니다.
func (pd *PeerDiscovery) GetNodesByRegion() map[string]int {
	pd.lock.RLock()
	defer pd.lock.RUnlock()
	
	regions := make(map[string]int)
	for _, node := range pd.knownNodes {
		regions[node.region]++
	}
	
	return regions
}

// GetNodeInfo는 노드 정보를 반환합니다.
func (pd *PeerDiscovery) GetNodeInfo(id enode.ID) map[string]interface{} {
	pd.lock.RLock()
	defer pd.lock.RUnlock()
	
	node, ok := pd.knownNodes[id]
	if !ok {
		return nil
	}
	
	quality, hasQuality := pd.peerQuality[id]
	
	info := map[string]interface{}{
		"id":        id.String(),
		"ip":        node.node.IP().String(),
		"lastSeen":  node.lastSeen,
		"attempts":  node.attempts,
		"validated": node.validated,
		"score":     node.score,
		"region":    node.region,
	}
	
	if hasQuality {
		info["latency"] = quality.latency.String()
		info["uptime"] = quality.uptime.String()
		info["bandwidth"] = quality.bandwidth
		info["reliability"] = quality.reliability
		info["qualityScore"] = pd.calculateQualityScore(quality)
	}
	
	return info
} 