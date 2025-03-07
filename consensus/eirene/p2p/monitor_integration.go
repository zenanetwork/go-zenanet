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
	"os"
	"path/filepath"
	"time"

	"github.com/zenanetwork/go-zenanet/eth"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/node"
)

// MonitoringService는 P2P 네트워크 모니터링 서비스를 제공합니다.
type MonitoringService struct {
	monitor *NetworkMonitor
	tool    *MonitoringTool
	config  *MonitoringServiceConfig
	logger  log.Logger
}

// MonitoringServiceConfig는 모니터링 서비스 설정을 정의합니다.
type MonitoringServiceConfig struct {
	// 기본 설정
	Enabled           bool   `json:"enabled"`
	DataDir           string `json:"data_dir"`
	
	// 도구 설정
	Tool              *MonitoringToolConfig `json:"tool"`
	
	// ethstats 설정
	EthstatsURL       string `json:"ethstats_url"`
}

// DefaultMonitoringServiceConfig는 기본 모니터링 서비스 설정을 반환합니다.
func DefaultMonitoringServiceConfig() *MonitoringServiceConfig {
	return &MonitoringServiceConfig{
		Enabled: true,
		DataDir: "p2p_monitor",
		Tool: &MonitoringToolConfig{
			HTTPAddr:                "localhost",
			HTTPPort:                8080,
			EnableHTTPS:             false,
			EnableAuth:              false,
			EnableAlerts:            true,
			DataRetentionDays:       7,
			DashboardRefreshInterval: 30 * time.Second,
		},
		EthstatsURL: "", // 기본적으로 ethstats 비활성화
	}
}

// NewMonitoringService는 새로운 모니터링 서비스를 생성합니다.
func NewMonitoringService(
	peerSet *PeerSet,
	discovery *PeerDiscovery,
	propagator *BlockPropagator,
	securityManager *SecurityManager,
	config *MonitoringServiceConfig,
	nodeConfig *node.Config,
) *MonitoringService {
	// 데이터 디렉토리 설정
	dataDir := filepath.Join(nodeConfig.DataDir, config.DataDir)
	
	// 네트워크 모니터 생성
	monitor := NewNetworkMonitor(
		peerSet,
		discovery,
		propagator,
		securityManager,
		dataDir,
	)
	
	// 모니터링 도구 생성
	tool := NewMonitoringTool(monitor, config.Tool)
	
	return &MonitoringService{
		monitor: monitor,
		tool:    tool,
		config:  config,
		logger:  log.New("module", "p2p-monitor-service"),
	}
}

// Start는 모니터링 서비스를 시작합니다.
func (ms *MonitoringService) Start() error {
	if !ms.config.Enabled {
		ms.logger.Info("P2P network monitoring service is disabled")
		return nil
	}
	
	ms.logger.Info("Starting P2P network monitoring service")
	
	// 네트워크 모니터 시작
	ms.monitor.Start()
	
	// 모니터링 도구 시작
	if err := ms.tool.Start(); err != nil {
		ms.logger.Error("Failed to start monitoring tool", "err", err)
		ms.monitor.Stop()
		return err
	}
	
	return nil
}

// Stop은 모니터링 서비스를 중지합니다.
func (ms *MonitoringService) Stop() error {
	if !ms.config.Enabled {
		return nil
	}
	
	ms.logger.Info("Stopping P2P network monitoring service")
	
	// 모니터링 도구 중지
	if err := ms.tool.Stop(); err != nil {
		ms.logger.Error("Failed to stop monitoring tool", "err", err)
	}
	
	// 네트워크 모니터 중지
	ms.monitor.Stop()
	
	return nil
}

// LoadConfig는 설정 파일에서 모니터링 서비스 설정을 로드합니다.
func LoadMonitoringServiceConfig(configPath string) (*MonitoringServiceConfig, error) {
	// 기본 설정
	config := DefaultMonitoringServiceConfig()
	
	// 설정 파일이 없으면 기본 설정 반환
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return config, nil
	}
	
	// 설정 파일 읽기
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}
	
	// 설정 파일 파싱
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}
	
	return config, nil
}

// SaveConfig는 모니터링 서비스 설정을 파일에 저장합니다.
func SaveMonitoringServiceConfig(config *MonitoringServiceConfig, configPath string) error {
	// 설정 디렉토리 생성
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}
	
	// 설정 직렬화
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}
	
	// 설정 파일 저장
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}
	
	return nil
}

// RegisterMonitoringService는 노드에 모니터링 서비스를 등록합니다.
func RegisterMonitoringService(
	stack *node.Node,
	peerSet *PeerSet,
	discovery *PeerDiscovery,
	propagator *BlockPropagator,
	securityManager *SecurityManager,
	configPath string,
) error {
	// 설정 로드
	config, err := LoadMonitoringServiceConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load monitoring service config: %v", err)
	}
	
	// 모니터링 서비스 생성
	service := NewMonitoringService(
		peerSet,
		discovery,
		propagator,
		securityManager,
		config,
		stack.Config(),
	)
	
	// 노드에 서비스 등록
	stack.RegisterLifecycle(service)
	
	return nil
}

// RegisterCombinedMonitoringServices는 P2P 모니터링 서비스와 ethstats 서비스를 함께 등록합니다.
// 이 함수는 cmd/utils/flags.go의 RegisterEthStatsService 함수와 유사하게 동작합니다.
func RegisterCombinedMonitoringServices(
	stack *node.Node,
	peerSet *PeerSet,
	discovery *PeerDiscovery,
	propagator *BlockPropagator,
	securityManager *SecurityManager,
	backend *eth.EthAPIBackend,
	configPath string,
) error {
	// 설정 로드
	config, err := LoadMonitoringServiceConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load monitoring service config: %v", err)
	}
	
	// P2P 모니터링 서비스 등록
	if config.Enabled {
		service := NewMonitoringService(
			peerSet,
			discovery,
			propagator,
			securityManager,
			config,
			stack.Config(),
		)
		
		stack.RegisterLifecycle(service)
	}
	
	// ethstats 서비스 등록 (cmd/utils/flags.go의 RegisterEthStatsService 함수 참조)
	if config.EthstatsURL != "" {
		// ethstats 서비스는 cmd/utils/flags.go에 있는 RegisterEthStatsService 함수를 사용하여 등록해야 합니다.
		// 여기서는 직접 등록하지 않고, 해당 함수를 호출하는 방식으로 구현합니다.
		log.Info("Ethstats URL is configured", "url", config.EthstatsURL)
		log.Info("Please use cmd/utils.RegisterEthStatsService to register ethstats service")
	}
	
	return nil
} 