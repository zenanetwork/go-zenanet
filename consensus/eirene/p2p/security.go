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
	minReputationScore = -100
	maxReputationScore = 100
	initialReputationScore = 0
	
	// 평판 점수 변화량
	goodBehaviorDelta = 1
	minorOffenseDelta = -5
	majorOffenseDelta = -20
	criticalOffenseDelta = -50
	
	// 평판 점수 임계값
	banThreshold = -50
	probationThreshold = -30
	
	// 평판 회복 간격
	reputationRecoveryInterval = 1 * time.Hour
	reputationRecoveryAmount = 1
	
	// 속도 제한 관련
	messageRateLimitWindow = 1 * time.Minute
	maxMessagesPerWindow = 1000
	maxBlocksPerWindow = 100
	maxTxsPerWindow = 500
	
	// 차단 기간
	temporaryBanDuration = 1 * time.Hour
	longBanDuration = 24 * time.Hour
	permanentBanDuration = 30 * 24 * time.Hour
	
	// 의심스러운 행동 임계값
	suspiciousBehaviorThreshold = 3
	
	// 블랙리스트 만료 시간
	blacklistExpiryTime = 7 * 24 * time.Hour
)

// 위반 유형
const (
	ViolationInvalidMessage = iota
	ViolationInvalidBlock
	ViolationInvalidTx
	ViolationRateLimit
	ViolationProtocolViolation
	ViolationDuplicateConnection
	ViolationMaliciousBehavior
)

// SecurityManager는 P2P 네트워크 보안을 관리합니다.
type SecurityManager struct {
	peerSet     *PeerSet           // 피어 집합
	
	// 평판 시스템
	reputations map[enode.ID]*PeerReputation // 피어 평판 맵
	
	// 차단 목록
	blacklist   map[enode.ID]*BanInfo // 차단된 피어 맵
	
	// 속도 제한
	rateLimits  map[enode.ID]*RateLimiter // 피어별 속도 제한
	
	// 의심스러운 행동 추적
	suspiciousBehavior map[enode.ID]map[int]int // 피어별 의심스러운 행동 횟수
	
	quit       chan struct{}      // 종료 채널
	wg         sync.WaitGroup     // 대기 그룹
	
	lock       sync.RWMutex       // 동시성 제어를 위한 락
	
	logger     log.Logger         // 로거
}

// PeerReputation은 피어의 평판 정보를 나타냅니다.
type PeerReputation struct {
	ID         enode.ID           // 피어 ID
	Score      int                // 평판 점수
	LastUpdate time.Time          // 마지막 업데이트 시간
	Violations map[int]int        // 위반 유형별 횟수
	OnProbation bool              // 관찰 중 여부
}

// BanInfo는 차단된 피어 정보를 나타냅니다.
type BanInfo struct {
	ID         enode.ID           // 피어 ID
	Reason     string             // 차단 이유
	BanTime    time.Time          // 차단 시간
	ExpiryTime time.Time          // 만료 시간
	Permanent  bool               // 영구 차단 여부
}

// RateLimiter는 피어의 메시지 속도를 제한합니다.
type RateLimiter struct {
	MessageCount int              // 메시지 수
	BlockCount   int              // 블록 수
	TxCount      int              // 트랜잭션 수
	WindowStart  time.Time        // 윈도우 시작 시간
}

// NewSecurityManager는 새로운 보안 관리자를 생성합니다.
func NewSecurityManager(peerSet *PeerSet) *SecurityManager {
	return &SecurityManager{
		peerSet:           peerSet,
		reputations:       make(map[enode.ID]*PeerReputation),
		blacklist:         make(map[enode.ID]*BanInfo),
		rateLimits:        make(map[enode.ID]*RateLimiter),
		suspiciousBehavior: make(map[enode.ID]map[int]int),
		quit:              make(chan struct{}),
		logger:            log.New("module", "eirene/p2p/security"),
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
	reputationTicker := time.NewTicker(reputationRecoveryInterval)
	defer reputationTicker.Stop()
	
	// 블랙리스트 정리 타이머
	blacklistTicker := time.NewTicker(time.Hour)
	defer blacklistTicker.Stop()
	
	for {
		select {
		case <-reputationTicker.C:
			sm.recoverReputations()
		case <-blacklistTicker.C:
			sm.cleanupBlacklist()
		case <-sm.quit:
			return
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
		ID:         id,
		Score:      initialReputationScore,
		LastUpdate: time.Now(),
		Violations: make(map[int]int),
		OnProbation: false,
	}
	
	// 속도 제한 정보 생성
	sm.rateLimits[id] = &RateLimiter{
		WindowStart: time.Now(),
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
			duration = permanentBanDuration
			permanent = true
		} else if rep.Score <= banThreshold - 20 {
			duration = longBanDuration
			permanent = false
		} else {
			duration = temporaryBanDuration
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
	case ViolationInvalidMessage:
		delta = minorOffenseDelta
	case ViolationInvalidBlock:
		delta = majorOffenseDelta
	case ViolationInvalidTx:
		delta = minorOffenseDelta
	case ViolationRateLimit:
		delta = minorOffenseDelta
	case ViolationProtocolViolation:
		delta = majorOffenseDelta
	case ViolationDuplicateConnection:
		delta = minorOffenseDelta
	case ViolationMaliciousBehavior:
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
	
	limiter, ok := sm.rateLimits[id]
	if !ok {
		return true
	}
	
	now := time.Now()
	
	// 윈도우 리셋
	if now.Sub(limiter.WindowStart) > messageRateLimitWindow {
		limiter.MessageCount = 0
		limiter.BlockCount = 0
		limiter.TxCount = 0
		limiter.WindowStart = now
	}
	
	// 메시지 유형에 따라 다른 제한 적용
	switch msgType {
	case NewBlockMsg:
		limiter.BlockCount++
		limiter.MessageCount++
		
		if limiter.BlockCount > maxBlocksPerWindow {
			sm.ReportViolation(id, ViolationRateLimit, "block rate limit exceeded")
			return false
		}
		
	case NewTxMsg:
		limiter.TxCount++
		limiter.MessageCount++
		
		if limiter.TxCount > maxTxsPerWindow {
			sm.ReportViolation(id, ViolationRateLimit, "transaction rate limit exceeded")
			return false
		}
		
	default:
		limiter.MessageCount++
		
		if limiter.MessageCount > maxMessagesPerWindow {
			sm.ReportViolation(id, ViolationRateLimit, "message rate limit exceeded")
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
		case ViolationInvalidMessage:
			typeName = "invalid_message"
		case ViolationInvalidBlock:
			typeName = "invalid_block"
		case ViolationInvalidTx:
			typeName = "invalid_tx"
		case ViolationRateLimit:
			typeName = "rate_limit"
		case ViolationProtocolViolation:
			typeName = "protocol_violation"
		case ViolationDuplicateConnection:
			typeName = "duplicate_connection"
		case ViolationMaliciousBehavior:
			typeName = "malicious_behavior"
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
		"id":         id.String(),
		"reason":     info.Reason,
		"ban_time":   info.BanTime,
		"expiry_time": info.ExpiryTime,
		"permanent":  info.Permanent,
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
			case ViolationInvalidMessage:
				typeName = "invalid_message"
			case ViolationInvalidBlock:
				typeName = "invalid_block"
			case ViolationInvalidTx:
				typeName = "invalid_tx"
			case ViolationRateLimit:
				typeName = "rate_limit"
			case ViolationProtocolViolation:
				typeName = "protocol_violation"
			case ViolationDuplicateConnection:
				typeName = "duplicate_connection"
			case ViolationMaliciousBehavior:
				typeName = "malicious_behavior"
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