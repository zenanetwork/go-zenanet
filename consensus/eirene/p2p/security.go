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
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/p2p/enode"
)

// 보안 관련 상수
const (
	// 평판 점수 범위
	minReputationScore     = -100
	maxReputationScore     = 100
	initialReputationScore = 0

	// 평판 점수 변화량
	goodBehaviorDelta    = 1
	minorOffenseDelta    = -5
	majorOffenseDelta    = -20
	criticalOffenseDelta = -50

	// 평판 점수 임계값
	banThreshold       = -50
	probationThreshold = -30

	// 평판 회복 간격
	reputationRecoveryInterval = 1 * time.Hour
	reputationRecoveryAmount   = 1

	// 속도 제한 관련
	messageRateLimitWindow = 1 * time.Minute
	maxMessagesPerWindow   = 1000
	maxBlocksPerWindow     = 100
	maxTxsPerWindow        = 5000

	// 새로 추가된 속도 제한 관련 상수
	maxHeadersPerWindow  = 500
	maxBodiesPerWindow   = 100
	maxNodeDataPerWindow = 200
	maxReceiptsPerWindow = 200

	// 차단 관련
	defaultBanDuration    = 24 * time.Hour
	maxBanDuration        = 30 * 24 * time.Hour
	banDurationMultiplier = 2 // 반복 위반 시 차단 기간 증가 배수

	// 의심스러운 행동 임계값
	suspiciousBehaviorThreshold = 5

	// 위반 유형
	violationTypeInvalidMessage    = 1
	violationTypeInvalidBlock      = 2
	violationTypeInvalidTx         = 3
	violationTypeRateLimit         = 4
	violationTypeProtocolViolation = 5
	violationTypeDuplicateMessage  = 6
	violationTypeInvalidNodeData   = 7
	violationTypeInvalidReceipts   = 8
	violationTypeInvalidHeaders    = 9
	violationTypeInvalidBodies     = 10

	// 새로 추가된 위반 유형
	violationTypeDDoSAttempt    = 11
	violationTypeEclipseAttempt = 12
	violationTypeSybilAttempt   = 13
	violationTypeSpamming       = 14
	violationTypeMaliciousData  = 15
)

// 메시지 타입 상수 (이미 다른 곳에서 정의되어 있을 수 있음)
const (
	// 메시지 타입 식별자 (예시)
	msgTypeBlock = iota + 100
	msgTypeTx
	msgTypeHeader
	msgTypeBody
	msgTypeNodeData
	msgTypeReceipt
)

// SecurityManager는 P2P 네트워크 보안을 관리합니다.
type SecurityManager struct {
	peerSet *PeerSet // 피어 집합

	// 평판 시스템
	reputations map[enode.ID]*PeerReputation // 피어 평판 맵

	// 차단 목록
	blacklist map[enode.ID]*BanInfo // 차단된 피어 맵

	// 속도 제한
	rateLimits map[enode.ID]*RateLimiter // 피어별 속도 제한

	// 의심스러운 행동 추적
	suspiciousBehavior map[enode.ID]map[int]int // 피어별 의심스러운 행동 횟수

	// 새로 추가된 필드
	trustedPeers  map[enode.ID]bool        // 신뢰할 수 있는 피어 목록
	ipReputations map[string]*IPReputation // IP 기반 평판 (IP 주소 -> 평판)

	quit chan struct{}  // 종료 채널
	wg   sync.WaitGroup // 대기 그룹

	lock sync.RWMutex // 동시성 제어를 위한 락

	logger log.Logger // 로거
}

// PeerReputation은 피어의 평판 정보를 나타냅니다.
type PeerReputation struct {
	ID          enode.ID    // 피어 ID
	Score       int         // 평판 점수
	LastUpdate  time.Time   // 마지막 업데이트 시간
	Violations  map[int]int // 위반 유형별 횟수
	OnProbation bool        // 관찰 중 여부

	// 새로 추가된 필드
	FirstSeen    time.Time // 처음 본 시간
	LastSeen     time.Time // 마지막으로 본 시간
	GoodActions  int       // 좋은 행동 횟수
	TotalActions int       // 총 행동 횟수
}

// BanInfo는 차단된 피어의 정보를 나타냅니다.
type BanInfo struct {
	ID         enode.ID  // 피어 ID
	Reason     string    // 차단 이유
	BanTime    time.Time // 차단 시간
	ExpiryTime time.Time // 만료 시간
	Permanent  bool      // 영구 차단 여부

	// 새로 추가된 필드
	BanCount int    // 차단 횟수
	IP       string // IP 주소
}

// RateLimiter는 피어의 메시지 속도를 제한합니다.
type RateLimiter struct {
	MessageCount int       // 메시지 수
	BlockCount   int       // 블록 수
	TxCount      int       // 트랜잭션 수
	WindowStart  time.Time // 윈도우 시작 시간

	// 새로 추가된 필드
	HeaderCount   int // 헤더 수
	BodiesCount   int // 바디 수
	NodeDataCount int // 노드 데이터 수
	ReceiptsCount int // 영수증 수
}

// IPReputation은 IP 주소 기반 평판 정보를 나타냅니다.
type IPReputation struct {
	IP         string     // IP 주소
	Score      int        // 평판 점수
	LastUpdate time.Time  // 마지막 업데이트 시간
	BanCount   int        // 차단 횟수
	PeerIDs    []enode.ID // 이 IP에서 연결한 피어 ID 목록
}

// NewSecurityManager는 새로운 보안 관리자를 생성합니다.
func NewSecurityManager(peerSet *PeerSet) *SecurityManager {
	return &SecurityManager{
		peerSet:            peerSet,
		reputations:        make(map[enode.ID]*PeerReputation),
		blacklist:          make(map[enode.ID]*BanInfo),
		rateLimits:         make(map[enode.ID]*RateLimiter),
		suspiciousBehavior: make(map[enode.ID]map[int]int),
		trustedPeers:       make(map[enode.ID]bool),
		ipReputations:      make(map[string]*IPReputation),
		quit:               make(chan struct{}),
		logger:             log.New("module", "p2p/security"),
	}
}

// Start는 보안 관리자를 시작합니다.
func (sm *SecurityManager) Start() {
	sm.logger.Info("Starting security manager")

	sm.wg.Add(1)
	go sm.maintenanceLoop()
}

// Stop은 보안 관리자를 중지합니다.
func (sm *SecurityManager) Stop() {
	sm.logger.Info("Stopping security manager")
	close(sm.quit)
	sm.wg.Wait()
}

// maintenanceLoop는 주기적으로 보안 관련 유지 보수를 수행합니다.
func (sm *SecurityManager) maintenanceLoop() {
	defer sm.wg.Done()

	// 평판 회복 타이머
	recoveryTicker := time.NewTicker(reputationRecoveryInterval)
	defer recoveryTicker.Stop()

	// 블랙리스트 정리 타이머
	cleanupTicker := time.NewTicker(1 * time.Hour)
	defer cleanupTicker.Stop()

	// 공격 탐지 타이머
	attackDetectionTicker := time.NewTicker(5 * time.Minute)
	defer attackDetectionTicker.Stop()

	for {
		select {
		case <-sm.quit:
			return
		case <-recoveryTicker.C:
			sm.recoverReputations()
			sm.RecoverIPReputations()
		case <-cleanupTicker.C:
			sm.cleanupBlacklist()
		case <-attackDetectionTicker.C:
			sm.DetectEclipseAttack()
		}
	}
}

// recoverReputations는 시간이 지남에 따라 피어의 평판을 회복시킵니다.
func (sm *SecurityManager) recoverReputations() {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	now := time.Now()

	for id, rep := range sm.reputations {
		// 이미 최대 점수인 경우 건너뛰기
		if rep.Score >= maxReputationScore {
			continue
		}

		// 마지막 업데이트 이후 충분한 시간이 지났는지 확인
		if now.Sub(rep.LastUpdate) >= reputationRecoveryInterval {
			// 점수 회복
			rep.Score += reputationRecoveryAmount
			if rep.Score > maxReputationScore {
				rep.Score = maxReputationScore
			}

			// 관찰 상태 업데이트
			if rep.OnProbation && rep.Score > probationThreshold {
				rep.OnProbation = false
				sm.logger.Debug("Peer removed from probation", "id", id.String(), "score", rep.Score)
			}

			rep.LastUpdate = now
		}
	}
}

// cleanupBlacklist는 만료된 차단 정보를 정리합니다.
func (sm *SecurityManager) cleanupBlacklist() {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	now := time.Now()

	for id, info := range sm.blacklist {
		// 영구 차단은 건너뛰기
		if info.Permanent {
			continue
		}

		// 만료된 차단 정보 제거
		if now.After(info.ExpiryTime) {
			delete(sm.blacklist, id)
			sm.logger.Debug("Ban expired", "id", id.String(), "reason", info.Reason)
		}
	}
}

// RegisterPeer는 새로운 피어를 등록합니다.
func (sm *SecurityManager) RegisterPeer(id enode.ID) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	// 이미 등록된 피어인 경우 무시
	if _, ok := sm.reputations[id]; ok {
		return
	}

	// 새 평판 정보 생성
	sm.reputations[id] = &PeerReputation{
		ID:           id,
		Score:        initialReputationScore,
		LastUpdate:   time.Now(),
		Violations:   make(map[int]int),
		OnProbation:  false,
		FirstSeen:    time.Now(),
		LastSeen:     time.Now(),
		GoodActions:  0,
		TotalActions: 0,
	}

	// 속도 제한 정보 생성
	sm.rateLimits[id] = &RateLimiter{
		WindowStart:   time.Now(),
		HeaderCount:   0,
		BodiesCount:   0,
		NodeDataCount: 0,
		ReceiptsCount: 0,
	}

	// 의심스러운 행동 추적 정보 생성
	sm.suspiciousBehavior[id] = make(map[int]int)

	sm.logger.Debug("Peer registered", "id", id.String())
}

// UnregisterPeer는 피어 등록을 해제합니다.
func (sm *SecurityManager) UnregisterPeer(id enode.ID) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	// 평판 정보 유지 (나중에 재연결 시 사용)

	// 속도 제한 정보 제거
	delete(sm.rateLimits, id)

	sm.logger.Debug("Peer unregistered", "id", id.String())
}

// IsBanned는 피어가 차단되었는지 확인합니다.
func (sm *SecurityManager) IsBanned(id enode.ID) bool {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	info, ok := sm.blacklist[id]
	if !ok {
		return false
	}

	// 영구 차단인 경우
	if info.Permanent {
		return true
	}

	// 만료 여부 확인
	return time.Now().Before(info.ExpiryTime)
}

// BanPeer는 피어를 차단합니다.
func (sm *SecurityManager) BanPeer(id enode.ID, reason string, duration time.Duration, permanent bool) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	now := time.Now()

	// 차단 정보 생성
	info := &BanInfo{
		ID:         id,
		Reason:     reason,
		BanTime:    now,
		ExpiryTime: now.Add(duration),
		Permanent:  permanent,
		BanCount:   1,
		IP:         "",
	}

	sm.blacklist[id] = info

	// 연결 해제
	if peer := sm.peerSet.Peer(id.String()); peer != nil {
		sm.peerSet.Unregister(id.String())
	}

	sm.logger.Info("Peer banned",
		"id", id.String(),
		"reason", reason,
		"duration", duration,
		"permanent", permanent)
}

// UpdateReputation은 피어의 평판을 업데이트합니다.
func (sm *SecurityManager) UpdateReputation(id enode.ID, delta int, violationType int, reason string) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	rep, ok := sm.reputations[id]
	if !ok {
		// 등록되지 않은 피어는 무시
		return
	}

	// 위반 횟수 업데이트
	if violationType >= 0 {
		rep.Violations[violationType]++
	}

	// 점수 업데이트
	rep.Score += delta
	rep.LastUpdate = time.Now()

	// 점수 범위 제한
	if rep.Score < minReputationScore {
		rep.Score = minReputationScore
	} else if rep.Score > maxReputationScore {
		rep.Score = maxReputationScore
	}

	// 관찰 상태 업데이트
	if rep.Score <= probationThreshold && !rep.OnProbation {
		rep.OnProbation = true
		sm.logger.Debug("Peer placed on probation", "id", id.String(), "score", rep.Score, "reason", reason)
	}

	// 차단 임계값 확인
	if rep.Score <= banThreshold {
		// 차단 기간 결정
		var duration time.Duration
		var permanent bool

		if rep.Score <= minReputationScore {
			duration = maxBanDuration
			permanent = true
		} else if rep.Score <= banThreshold-20 {
			duration = maxBanDuration
			permanent = false
		} else {
			duration = defaultBanDuration
			permanent = false
		}

		// 피어 차단
		sm.BanPeer(id, reason, duration, permanent)
	}

	sm.logger.Debug("Peer reputation updated",
		"id", id.String(),
		"delta", delta,
		"new_score", rep.Score,
		"violation", violationType,
		"reason", reason)
}

// ReportGoodBehavior는 피어의 좋은 행동을 보고합니다.
func (sm *SecurityManager) ReportGoodBehavior(id enode.ID) {
	sm.UpdateReputation(id, goodBehaviorDelta, -1, "good behavior")
}

// ReportViolation은 피어의 위반 행동을 보고합니다.
func (sm *SecurityManager) ReportViolation(id enode.ID, violationType int, reason string) {
	var delta int

	switch violationType {
	case violationTypeInvalidMessage:
		delta = minorOffenseDelta
	case violationTypeInvalidBlock:
		delta = majorOffenseDelta
	case violationTypeInvalidTx:
		delta = minorOffenseDelta
	case violationTypeRateLimit:
		delta = minorOffenseDelta
	case violationTypeProtocolViolation:
		delta = majorOffenseDelta
	case violationTypeDuplicateMessage:
		delta = minorOffenseDelta
	case violationTypeInvalidNodeData:
		delta = minorOffenseDelta
	case violationTypeInvalidReceipts:
		delta = minorOffenseDelta
	case violationTypeInvalidHeaders:
		delta = minorOffenseDelta
	case violationTypeInvalidBodies:
		delta = minorOffenseDelta
	case violationTypeDDoSAttempt:
		delta = majorOffenseDelta
	case violationTypeEclipseAttempt:
		delta = majorOffenseDelta
	case violationTypeSybilAttempt:
		delta = majorOffenseDelta
	case violationTypeSpamming:
		delta = majorOffenseDelta
	case violationTypeMaliciousData:
		delta = criticalOffenseDelta
	default:
		delta = minorOffenseDelta
	}

	sm.UpdateReputation(id, delta, violationType, reason)

	// 의심스러운 행동 추적
	sm.lock.Lock()
	defer sm.lock.Unlock()

	behaviors, ok := sm.suspiciousBehavior[id]
	if ok {
		behaviors[violationType]++

		// 의심스러운 행동 임계값 확인
		if behaviors[violationType] >= suspiciousBehaviorThreshold {
			// 추가 페널티 부여
			sm.UpdateReputation(id, majorOffenseDelta, violationType, "repeated violation")

			// 카운터 리셋
			behaviors[violationType] = 0
		}
	}
}

// CheckRateLimit은 피어의 메시지 속도를 확인합니다.
func (sm *SecurityManager) CheckRateLimit(id enode.ID, msgType uint64) bool {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	// 신뢰할 수 있는 피어는 속도 제한 없음
	if sm.trustedPeers[id] {
		return true
	}

	// 속도 제한 정보 가져오기
	limiter, exists := sm.rateLimits[id]
	if !exists {
		return true
	}

	// 윈도우 시간 확인
	now := time.Now()
	if now.Sub(limiter.WindowStart) > messageRateLimitWindow {
		// 윈도우 초기화
		limiter.MessageCount = 0
		limiter.BlockCount = 0
		limiter.TxCount = 0
		limiter.HeaderCount = 0
		limiter.BodiesCount = 0
		limiter.NodeDataCount = 0
		limiter.ReceiptsCount = 0
		limiter.WindowStart = now
	}

	// 메시지 타입별 처리
	switch msgType {
	case msgTypeBlock:
		limiter.BlockCount++
		limiter.MessageCount++

		if limiter.BlockCount > maxBlocksPerWindow {
			sm.ReportViolation(id, violationTypeRateLimit, "block rate limit exceeded")
			return false
		}

	case msgTypeTx:
		limiter.TxCount++
		limiter.MessageCount++

		if limiter.TxCount > maxTxsPerWindow {
			sm.ReportViolation(id, violationTypeRateLimit, "transaction rate limit exceeded")
			return false
		}

	case msgTypeHeader:
		limiter.HeaderCount++
		limiter.MessageCount++

		if limiter.HeaderCount > maxHeadersPerWindow {
			sm.ReportViolation(id, violationTypeRateLimit, "header rate limit exceeded")
			return false
		}

	case msgTypeBody:
		limiter.BodiesCount++
		limiter.MessageCount++

		if limiter.BodiesCount > maxBodiesPerWindow {
			sm.ReportViolation(id, violationTypeRateLimit, "body rate limit exceeded")
			return false
		}

	case msgTypeNodeData:
		limiter.NodeDataCount++
		limiter.MessageCount++

		if limiter.NodeDataCount > maxNodeDataPerWindow {
			sm.ReportViolation(id, violationTypeRateLimit, "node data rate limit exceeded")
			return false
		}

	case msgTypeReceipt:
		limiter.ReceiptsCount++
		limiter.MessageCount++

		if limiter.ReceiptsCount > maxReceiptsPerWindow {
			sm.ReportViolation(id, violationTypeRateLimit, "receipt rate limit exceeded")
			return false
		}

	default:
		limiter.MessageCount++

		if limiter.MessageCount > maxMessagesPerWindow {
			sm.ReportViolation(id, violationTypeRateLimit, "message rate limit exceeded")
			return false
		}
	}

	return true
}

// GetReputationInfo는 피어의 평판 정보를 반환합니다.
func (sm *SecurityManager) GetReputationInfo(id enode.ID) map[string]interface{} {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	rep, ok := sm.reputations[id]
	if !ok {
		return nil
	}

	// 위반 정보 변환
	violations := make(map[string]int)
	for vType, count := range rep.Violations {
		var typeName string
		switch vType {
		case violationTypeInvalidMessage:
			typeName = "invalid_message"
		case violationTypeInvalidBlock:
			typeName = "invalid_block"
		case violationTypeInvalidTx:
			typeName = "invalid_tx"
		case violationTypeRateLimit:
			typeName = "rate_limit"
		case violationTypeProtocolViolation:
			typeName = "protocol_violation"
		case violationTypeDuplicateMessage:
			typeName = "duplicate_message"
		case violationTypeInvalidNodeData:
			typeName = "invalid_node_data"
		case violationTypeInvalidReceipts:
			typeName = "invalid_receipts"
		case violationTypeInvalidHeaders:
			typeName = "invalid_headers"
		case violationTypeInvalidBodies:
			typeName = "invalid_bodies"
		case violationTypeDDoSAttempt:
			typeName = "ddos_attempt"
		case violationTypeEclipseAttempt:
			typeName = "eclipse_attempt"
		case violationTypeSybilAttempt:
			typeName = "sybil_attempt"
		case violationTypeSpamming:
			typeName = "spamming"
		case violationTypeMaliciousData:
			typeName = "malicious_data"
		default:
			typeName = "unknown"
		}
		violations[typeName] = count
	}

	return map[string]interface{}{
		"id":           id.String(),
		"score":        rep.Score,
		"last_update":  rep.LastUpdate,
		"on_probation": rep.OnProbation,
		"violations":   violations,
	}
}

// GetBanInfo는 피어의 차단 정보를 반환합니다.
func (sm *SecurityManager) GetBanInfo(id enode.ID) map[string]interface{} {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	info, ok := sm.blacklist[id]
	if !ok {
		return nil
	}

	return map[string]interface{}{
		"id":          id.String(),
		"reason":      info.Reason,
		"ban_time":    info.BanTime,
		"expiry_time": info.ExpiryTime,
		"permanent":   info.Permanent,
		"ban_count":   info.BanCount,
		"ip":          info.IP,
	}
}

// GetSecurityStats는 보안 통계를 반환합니다.
func (sm *SecurityManager) GetSecurityStats() map[string]interface{} {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	// 평판 점수 분포 계산
	var goodPeers, neutralPeers, badPeers, probationPeers int
	for _, rep := range sm.reputations {
		if rep.Score > 50 {
			goodPeers++
		} else if rep.Score >= 0 {
			neutralPeers++
		} else {
			badPeers++
		}

		if rep.OnProbation {
			probationPeers++
		}
	}

	// 차단된 피어 수 계산
	var temporaryBans, permanentBans int
	for _, info := range sm.blacklist {
		if info.Permanent {
			permanentBans++
		} else {
			temporaryBans++
		}
	}

	// 위반 유형별 횟수 계산
	violationCounts := make(map[string]int)
	for _, rep := range sm.reputations {
		for vType, count := range rep.Violations {
			var typeName string
			switch vType {
			case violationTypeInvalidMessage:
				typeName = "invalid_message"
			case violationTypeInvalidBlock:
				typeName = "invalid_block"
			case violationTypeInvalidTx:
				typeName = "invalid_tx"
			case violationTypeRateLimit:
				typeName = "rate_limit"
			case violationTypeProtocolViolation:
				typeName = "protocol_violation"
			case violationTypeDuplicateMessage:
				typeName = "duplicate_message"
			case violationTypeInvalidNodeData:
				typeName = "invalid_node_data"
			case violationTypeInvalidReceipts:
				typeName = "invalid_receipts"
			case violationTypeInvalidHeaders:
				typeName = "invalid_headers"
			case violationTypeInvalidBodies:
				typeName = "invalid_bodies"
			case violationTypeDDoSAttempt:
				typeName = "ddos_attempt"
			case violationTypeEclipseAttempt:
				typeName = "eclipse_attempt"
			case violationTypeSybilAttempt:
				typeName = "sybil_attempt"
			case violationTypeSpamming:
				typeName = "spamming"
			case violationTypeMaliciousData:
				typeName = "malicious_data"
			default:
				typeName = "unknown"
			}
			violationCounts[typeName] += count
		}
	}

	return map[string]interface{}{
		"total_peers":      len(sm.reputations),
		"good_peers":       goodPeers,
		"neutral_peers":    neutralPeers,
		"bad_peers":        badPeers,
		"probation_peers":  probationPeers,
		"temporary_bans":   temporaryBans,
		"permanent_bans":   permanentBans,
		"violation_counts": violationCounts,
	}
}

// AddTrustedPeer는 신뢰할 수 있는 피어를 추가합니다.
func (sm *SecurityManager) AddTrustedPeer(id enode.ID) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	sm.trustedPeers[id] = true
	sm.logger.Info("Added trusted peer", "id", id.String())
}

// RemoveTrustedPeer는 신뢰할 수 있는 피어를 제거합니다.
func (sm *SecurityManager) RemoveTrustedPeer(id enode.ID) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	delete(sm.trustedPeers, id)
	sm.logger.Info("Removed trusted peer", "id", id.String())
}

// IsTrustedPeer는 피어가 신뢰할 수 있는지 확인합니다.
func (sm *SecurityManager) IsTrustedPeer(id enode.ID) bool {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	return sm.trustedPeers[id]
}

// UpdateIPReputation은 IP 주소의 평판을 업데이트합니다.
func (sm *SecurityManager) UpdateIPReputation(ip string, id enode.ID, delta int) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	// IP 평판 정보 가져오기
	rep, exists := sm.ipReputations[ip]
	if !exists {
		// 새 IP 평판 정보 생성
		rep = &IPReputation{
			IP:         ip,
			Score:      initialReputationScore,
			LastUpdate: time.Now(),
			BanCount:   0,
			PeerIDs:    []enode.ID{id},
		}
		sm.ipReputations[ip] = rep
	} else {
		// 피어 ID 추가 (중복 방지)
		found := false
		for _, existingID := range rep.PeerIDs {
			if existingID == id {
				found = true
				break
			}
		}
		if !found {
			rep.PeerIDs = append(rep.PeerIDs, id)
		}
	}

	// 평판 점수 업데이트
	rep.Score += delta
	if rep.Score > maxReputationScore {
		rep.Score = maxReputationScore
	} else if rep.Score < minReputationScore {
		rep.Score = minReputationScore
	}

	rep.LastUpdate = time.Now()

	// IP 평판이 너무 낮으면 해당 IP의 모든 피어 차단
	if rep.Score <= banThreshold {
		rep.BanCount++
		for _, peerID := range rep.PeerIDs {
			// 이미 차단된 피어는 건너뛰기
			if sm.IsBanned(peerID) {
				continue
			}

			// 피어 차단
			duration := defaultBanDuration * time.Duration(rep.BanCount)
			if duration > maxBanDuration {
				duration = maxBanDuration
			}

			sm.BanPeer(peerID, "IP reputation too low", duration, false)
		}
	}
}

// DetectSybilAttack은 시빌 공격을 탐지합니다.
func (sm *SecurityManager) DetectSybilAttack(ip string, id enode.ID) bool {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	// IP 평판 정보 가져오기
	rep, exists := sm.ipReputations[ip]
	if !exists {
		return false
	}

	// 같은 IP에서 너무 많은 피어가 연결되면 시빌 공격으로 간주
	if len(rep.PeerIDs) > 10 {
		sm.logger.Warn("Possible Sybil attack detected", "ip", ip, "peer_count", len(rep.PeerIDs))
		return true
	}

	return false
}

// DetectEclipseAttack은 이클립스 공격을 탐지합니다.
func (sm *SecurityManager) DetectEclipseAttack() {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	// 모든 피어 가져오기
	peers := sm.peerSet.AllPeers()

	// IP 주소별 피어 수 계산
	ipCounts := make(map[string]int)
	// 실제 구현에서는 각 피어의 IP 주소를 가져와야 함
	// 여기서는 임의의 IP 사용
	for i := 0; i < len(peers); i++ {
		ip := "0.0.0.0"
		ipCounts[ip]++
	}

	// 전체 피어 수
	totalPeers := len(peers)
	if totalPeers == 0 {
		return
	}

	// 특정 IP 또는 서브넷에서 너무 많은 피어가 연결되면 이클립스 공격으로 간주
	for ip, count := range ipCounts {
		percentage := float64(count) / float64(totalPeers)
		if percentage > 0.25 { // 25% 이상이면 의심
			sm.logger.Warn("Possible Eclipse attack detected", "ip", ip, "percentage", percentage*100)

			// 해당 IP의 평판 감소
			sm.UpdateIPReputation(ip, enode.ID{}, criticalOffenseDelta)
		}
	}
}

// RecoverIPReputations는 IP 평판을 회복합니다.
func (sm *SecurityManager) RecoverIPReputations() {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	now := time.Now()

	for ip, rep := range sm.ipReputations {
		// 마지막 업데이트 이후 일정 시간이 지났고, 평판이 최대가 아니면 회복
		if now.Sub(rep.LastUpdate) >= reputationRecoveryInterval && rep.Score < maxReputationScore {
			rep.Score += reputationRecoveryAmount
			if rep.Score > maxReputationScore {
				rep.Score = maxReputationScore
			}
			rep.LastUpdate = now

			sm.logger.Debug("Recovered IP reputation", "ip", ip, "score", rep.Score)
		}
	}
}
