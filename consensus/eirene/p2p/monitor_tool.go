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
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/log"
)

// MonitoringTool은 P2P 네트워크 모니터링을 위한 도구를 제공합니다.
type MonitoringTool struct {
	monitor      *NetworkMonitor
	httpServer   *http.Server
	apiEndpoints map[string]http.HandlerFunc
	
	// 설정
	config       *MonitoringToolConfig
	
	// 인증
	authTokens   map[string]bool
	
	// 알림 관리
	alertManager *AlertManager
	
	// 동시성 제어
	lock         sync.RWMutex
	
	// 로깅
	logger       log.Logger
}

// MonitoringToolConfig는 모니터링 도구 설정을 정의합니다.
type MonitoringToolConfig struct {
	// HTTP 서버 설정
	HTTPAddr           string        `json:"http_addr"`
	HTTPPort           int           `json:"http_port"`
	EnableHTTPS        bool          `json:"enable_https"`
	CertFile           string        `json:"cert_file"`
	KeyFile            string        `json:"key_file"`
	
	// 인증 설정
	EnableAuth         bool          `json:"enable_auth"`
	AuthTokens         []string      `json:"auth_tokens"`
	
	// 알림 설정
	EnableAlerts       bool          `json:"enable_alerts"`
	AlertEndpoints     []AlertEndpoint `json:"alert_endpoints"`
	
	// 데이터 보관 설정
	DataRetentionDays  int           `json:"data_retention_days"`
	
	// 대시보드 설정
	DashboardRefreshInterval time.Duration `json:"dashboard_refresh_interval"`
}

// AlertEndpoint는 알림을 보낼 엔드포인트를 정의합니다.
type AlertEndpoint struct {
	Type     string            `json:"type"`      // "webhook", "email", "slack" 등
	URL      string            `json:"url"`       // 웹훅 URL
	Headers  map[string]string `json:"headers"`   // HTTP 헤더
	Settings map[string]string `json:"settings"`  // 추가 설정
}

// AlertManager는 알림 관리를 담당합니다.
type AlertManager struct {
	config      *MonitoringToolConfig
	alertHistory []Alert
	lock        sync.RWMutex
	logger      log.Logger
}

// Alert은 알림 정보를 나타냅니다.
type Alert struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`      // "info", "warning", "error", "critical"
	Source    string    `json:"source"`     // 알림 소스
	Message   string    `json:"message"`    // 알림 메시지
	Details   string    `json:"details"`    // 상세 정보
	Resolved  bool      `json:"resolved"`   // 해결 여부
	ResolvedAt *time.Time `json:"resolved_at"` // 해결 시간
}

// NewMonitoringTool은 새로운 모니터링 도구를 생성합니다.
func NewMonitoringTool(monitor *NetworkMonitor, config *MonitoringToolConfig) *MonitoringTool {
	tool := &MonitoringTool{
		monitor:      monitor,
		apiEndpoints: make(map[string]http.HandlerFunc),
		config:       config,
		authTokens:   make(map[string]bool),
		logger:       log.New("module", "p2p-monitor-tool"),
	}
	
	// 인증 토큰 설정
	if config.EnableAuth {
		for _, token := range config.AuthTokens {
			tool.authTokens[token] = true
		}
	}
	
	// 알림 관리자 초기화
	tool.alertManager = &AlertManager{
		config: config,
		logger: log.New("module", "p2p-alert-manager"),
	}
	
	// API 엔드포인트 등록
	tool.registerAPIEndpoints()
	
	return tool
}

// Start는 모니터링 도구를 시작합니다.
func (mt *MonitoringTool) Start() error {
	mt.logger.Info("Starting P2P network monitoring tool")
	
	// HTTP 서버 설정
	addr := fmt.Sprintf("%s:%d", mt.config.HTTPAddr, mt.config.HTTPPort)
	mux := http.NewServeMux()
	
	// API 엔드포인트 등록
	for path, handler := range mt.apiEndpoints {
		if mt.config.EnableAuth {
			mux.HandleFunc(path, mt.authMiddleware(handler))
		} else {
			mux.HandleFunc(path, handler)
		}
	}
	
	// 정적 파일 서빙 (대시보드 UI)
	mux.Handle("/dashboard/", http.StripPrefix("/dashboard/", http.FileServer(http.Dir("./dashboard"))))
	
	// 기본 페이지 리다이렉트
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/dashboard/", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})
	
	mt.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	
	// HTTPS 사용 여부에 따라 서버 시작
	if mt.config.EnableHTTPS {
		go func() {
			mt.logger.Info("Starting HTTPS server", "addr", addr)
			if err := mt.httpServer.ListenAndServeTLS(mt.config.CertFile, mt.config.KeyFile); err != nil && err != http.ErrServerClosed {
				mt.logger.Error("HTTPS server error", "err", err)
			}
		}()
	} else {
		go func() {
			mt.logger.Info("Starting HTTP server", "addr", addr)
			if err := mt.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				mt.logger.Error("HTTP server error", "err", err)
			}
		}()
	}
	
	return nil
}

// Stop은 모니터링 도구를 중지합니다.
func (mt *MonitoringTool) Stop() error {
	mt.logger.Info("Stopping P2P network monitoring tool")
	
	if mt.httpServer != nil {
		return mt.httpServer.Close()
	}
	
	return nil
}

// registerAPIEndpoints는 API 엔드포인트를 등록합니다.
func (mt *MonitoringTool) registerAPIEndpoints() {
	// 현재 네트워크 상태 조회
	mt.apiEndpoints["/api/v1/network/status"] = mt.handleNetworkStatus
	
	// 네트워크 통계 조회
	mt.apiEndpoints["/api/v1/network/stats"] = mt.handleNetworkStats
	
	// 피어 정보 조회
	mt.apiEndpoints["/api/v1/peers"] = mt.handlePeers
	
	// 경고 정보 조회
	mt.apiEndpoints["/api/v1/warnings"] = mt.handleWarnings
	
	// 알림 관리
	mt.apiEndpoints["/api/v1/alerts"] = mt.handleAlerts
	
	// 네트워크 보고서 생성
	mt.apiEndpoints["/api/v1/reports/network"] = mt.handleNetworkReport
	
	// 대시보드 데이터 조회
	mt.apiEndpoints["/api/v1/dashboard/data"] = mt.handleDashboardData
}

// authMiddleware는 인증 미들웨어를 제공합니다.
func (mt *MonitoringTool) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 인증 토큰 확인
		token := r.Header.Get("Authorization")
		if token == "" {
			// 쿼리 파라미터에서 토큰 확인
			token = r.URL.Query().Get("token")
		} else {
			// Bearer 토큰 형식 처리
			parts := strings.Split(token, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}
		
		// 토큰 유효성 검사
		mt.lock.RLock()
		valid := mt.authTokens[token]
		mt.lock.RUnlock()
		
		if !valid {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Unauthorized",
			})
			return
		}
		
		next(w, r)
	}
}

// handleNetworkStatus는 현재 네트워크 상태를 반환합니다.
func (mt *MonitoringTool) handleNetworkStatus(w http.ResponseWriter, r *http.Request) {
	stats := mt.monitor.GetCurrentStats()
	warnings := mt.monitor.GetWarnings()
	
	// 응답 데이터 구성
	response := map[string]interface{}{
		"timestamp":     time.Now(),
		"peer_count":    stats.PeerCount,
		"warnings":      warnings,
		"partition_risk": stats.NetworkPartitionRisk,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleNetworkStats는 네트워크 통계를 반환합니다.
func (mt *MonitoringTool) handleNetworkStats(w http.ResponseWriter, r *http.Request) {
	// 쿼리 파라미터에서 기간 설정
	period := r.URL.Query().Get("period")
	limit := 24 // 기본값
	
	if period != "" {
		switch period {
		case "day":
			limit = 24
		case "week":
			limit = 168
		case "hour":
			limit = 6
		default:
			// 사용자 지정 기간
			if l, err := strconv.Atoi(period); err == nil && l > 0 {
				limit = l
			}
		}
	}
	
	// 통계 데이터 조회
	stats := mt.monitor.GetHistoricalStats(limit)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handlePeers는 피어 정보를 반환합니다.
func (mt *MonitoringTool) handlePeers(w http.ResponseWriter, r *http.Request) {
	stats := mt.monitor.GetCurrentStats()
	
	// 정렬 옵션
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "id"
	}
	
	// 피어 정보 정렬
	peers := stats.ConnectedPeers
	sort.Slice(peers, func(i, j int) bool {
		switch sortBy {
		case "latency":
			return peers[i].Latency < peers[j].Latency
		case "connected":
			return peers[i].ConnectedTime.Before(peers[j].ConnectedTime)
		default:
			return peers[i].ID < peers[j].ID
		}
	})
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(peers)
}

// handleWarnings는 경고 정보를 반환합니다.
func (mt *MonitoringTool) handleWarnings(w http.ResponseWriter, r *http.Request) {
	warnings := mt.monitor.GetWarnings()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(warnings)
}

// handleAlerts는 알림 정보를 관리합니다.
func (mt *MonitoringTool) handleAlerts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 알림 목록 조회
		mt.alertManager.lock.RLock()
		alerts := mt.alertManager.alertHistory
		mt.alertManager.lock.RUnlock()
		
		// 필터링 옵션
		resolved := r.URL.Query().Get("resolved")
		if resolved == "true" {
			filteredAlerts := make([]Alert, 0)
			for _, alert := range alerts {
				if alert.Resolved {
					filteredAlerts = append(filteredAlerts, alert)
				}
			}
			alerts = filteredAlerts
		} else if resolved == "false" {
			filteredAlerts := make([]Alert, 0)
			for _, alert := range alerts {
				if !alert.Resolved {
					filteredAlerts = append(filteredAlerts, alert)
				}
			}
			alerts = filteredAlerts
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(alerts)
		
	case http.MethodPost:
		// 알림 해결 처리
		var request struct {
			ID       string `json:"id"`
			Resolved bool   `json:"resolved"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request format",
			})
			return
		}
		
		// 알림 상태 업데이트
		mt.alertManager.lock.Lock()
		for i, alert := range mt.alertManager.alertHistory {
			if alert.ID == request.ID {
				mt.alertManager.alertHistory[i].Resolved = request.Resolved
				if request.Resolved {
					now := time.Now()
					mt.alertManager.alertHistory[i].ResolvedAt = &now
				} else {
					mt.alertManager.alertHistory[i].ResolvedAt = nil
				}
				break
			}
		}
		mt.alertManager.lock.Unlock()
		
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "success",
		})
		
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleNetworkReport는 네트워크 보고서를 생성합니다.
func (mt *MonitoringTool) handleNetworkReport(w http.ResponseWriter, r *http.Request) {
	report := mt.monitor.GenerateNetworkReport()
	
	// 요청 형식에 따라 응답 형식 결정
	format := r.URL.Query().Get("format")
	if format == "json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"report": report,
		})
	} else {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(report))
	}
}

// handleDashboardData는 대시보드 데이터를 반환합니다.
func (mt *MonitoringTool) handleDashboardData(w http.ResponseWriter, r *http.Request) {
	currentStats := mt.monitor.GetCurrentStats()
	historicalStats := mt.monitor.GetHistoricalStats(24) // 최근 24시간 데이터
	warnings := mt.monitor.GetWarnings()
	
	// 알림 데이터
	mt.alertManager.lock.RLock()
	activeAlerts := 0
	for _, alert := range mt.alertManager.alertHistory {
		if !alert.Resolved {
			activeAlerts++
		}
	}
	mt.alertManager.lock.RUnlock()
	
	// 대시보드 데이터 구성
	dashboardData := map[string]interface{}{
		"current_stats":    currentStats,
		"historical_stats": historicalStats,
		"warnings":         warnings,
		"active_alerts":    activeAlerts,
		"refresh_interval": mt.config.DashboardRefreshInterval,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dashboardData)
}

// SendAlert는 새로운 알림을 생성하고 전송합니다.
func (mt *MonitoringTool) SendAlert(level, source, message, details string) {
	if !mt.config.EnableAlerts {
		return
	}
	
	alert := Alert{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Level:     level,
		Source:    source,
		Message:   message,
		Details:   details,
		Resolved:  false,
	}
	
	// 알림 기록
	mt.alertManager.lock.Lock()
	mt.alertManager.alertHistory = append(mt.alertManager.alertHistory, alert)
	mt.alertManager.lock.Unlock()
	
	// 알림 전송
	go mt.sendAlertToEndpoints(alert)
}

// sendAlertToEndpoints는 알림을 모든 엔드포인트로 전송합니다.
func (mt *MonitoringTool) sendAlertToEndpoints(alert Alert) {
	for _, endpoint := range mt.config.AlertEndpoints {
		switch endpoint.Type {
		case "webhook":
			mt.sendWebhookAlert(endpoint, alert)
		case "email":
			// 이메일 알림 구현
		case "slack":
			mt.sendSlackAlert(endpoint, alert)
		}
	}
}

// sendWebhookAlert는 웹훅으로 알림을 전송합니다.
func (mt *MonitoringTool) sendWebhookAlert(endpoint AlertEndpoint, alert Alert) {
	payload, err := json.Marshal(alert)
	if err != nil {
		mt.logger.Error("Failed to marshal alert", "err", err)
		return
	}
	
	req, err := http.NewRequest("POST", endpoint.URL, strings.NewReader(string(payload)))
	if err != nil {
		mt.logger.Error("Failed to create webhook request", "err", err)
		return
	}
	
	// 헤더 설정
	req.Header.Set("Content-Type", "application/json")
	for key, value := range endpoint.Headers {
		req.Header.Set(key, value)
	}
	
	// 요청 전송
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		mt.logger.Error("Failed to send webhook alert", "err", err)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		mt.logger.Error("Webhook alert failed", "status", resp.StatusCode)
	}
}

// sendSlackAlert는 Slack으로 알림을 전송합니다.
func (mt *MonitoringTool) sendSlackAlert(endpoint AlertEndpoint, alert Alert) {
	// Slack 메시지 형식 구성
	message := map[string]interface{}{
		"text": fmt.Sprintf("[%s] %s: %s", alert.Level, alert.Source, alert.Message),
		"attachments": []map[string]interface{}{
			{
				"color":      getAlertColor(alert.Level),
				"title":      "Alert Details",
				"text":       alert.Details,
				"fields": []map[string]interface{}{
					{
						"title": "Time",
						"value": alert.Timestamp.Format(time.RFC3339),
						"short": true,
					},
					{
						"title": "ID",
						"value": alert.ID,
						"short": true,
					},
				},
			},
		},
	}
	
	payload, err := json.Marshal(message)
	if err != nil {
		mt.logger.Error("Failed to marshal Slack message", "err", err)
		return
	}
	
	req, err := http.NewRequest("POST", endpoint.URL, strings.NewReader(string(payload)))
	if err != nil {
		mt.logger.Error("Failed to create Slack request", "err", err)
		return
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	// 요청 전송
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		mt.logger.Error("Failed to send Slack alert", "err", err)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		mt.logger.Error("Slack alert failed", "status", resp.StatusCode)
	}
}

// getAlertColor는 알림 레벨에 따른 색상을 반환합니다.
func getAlertColor(level string) string {
	switch level {
	case "info":
		return "#2196F3" // 파란색
	case "warning":
		return "#FF9800" // 주황색
	case "error":
		return "#F44336" // 빨간색
	case "critical":
		return "#9C27B0" // 보라색
	default:
		return "#9E9E9E" // 회색
	}
} 