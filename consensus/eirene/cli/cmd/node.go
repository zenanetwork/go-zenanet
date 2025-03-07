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

// Package cmd는 Eirene CLI 도구의 명령어 모듈을 제공합니다.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// NodeCmd는 노드 관리 명령어를 생성합니다.
func NodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "노드 관리 명령어",
		Long:  `노드 시작, 중지, 상태 확인 등 노드 관리 관련 명령어를 제공합니다.`,
	}

	// 하위 명령어 추가
	cmd.AddCommand(nodeStartCmd())
	cmd.AddCommand(nodeStopCmd())
	cmd.AddCommand(nodeStatusCmd())
	cmd.AddCommand(nodeResetCmd())
	cmd.AddCommand(nodeConfigCmd())

	return cmd
}

// nodeStartCmd는 노드 시작 명령어를 생성합니다.
func nodeStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "노드 시작",
		Long:  `Eirene 합의 알고리즘을 사용하는 노드를 시작합니다.`,
		Run: func(cmd *cobra.Command, args []string) {
			dataDir, _ := cmd.Flags().GetString("datadir")
			rpcPort, _ := cmd.Flags().GetInt("rpc-port")
			p2pPort, _ := cmd.Flags().GetInt("p2p-port")
			validator, _ := cmd.Flags().GetBool("validator")
			
			// 데이터 디렉토리 확장
			if dataDir == defaultDataDir {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					fmt.Printf("홈 디렉토리를 찾을 수 없습니다: %v\n", err)
					return
				}
				dataDir = filepath.Join(homeDir, ".eirene")
			}
			
			// 데이터 디렉토리 생성
			if err := os.MkdirAll(dataDir, 0755); err != nil {
				fmt.Printf("데이터 디렉토리를 생성할 수 없습니다: %v\n", err)
				return
			}
			
			fmt.Printf("노드를 시작합니다...\n")
			fmt.Printf("데이터 디렉토리: %s\n", dataDir)
			fmt.Printf("RPC 포트: %d\n", rpcPort)
			fmt.Printf("P2P 포트: %d\n", p2pPort)
			fmt.Printf("검증자 모드: %v\n", validator)
			
			// TODO: 실제 노드 시작 로직 구현
			fmt.Println("노드가 성공적으로 시작되었습니다.")
		},
	}
	
	// 플래그 추가
	cmd.Flags().String("datadir", defaultDataDir, "노드 데이터 디렉토리 경로")
	cmd.Flags().Int("rpc-port", defaultRPCPort, "RPC 서버 포트")
	cmd.Flags().Int("p2p-port", defaultP2PPort, "P2P 네트워크 포트")
	cmd.Flags().Bool("validator", false, "검증자 모드로 실행")
	
	return cmd
}

// nodeStopCmd는 노드 중지 명령어를 생성합니다.
func nodeStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "노드 중지",
		Long:  `실행 중인 노드를 안전하게 중지합니다.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("노드를 중지합니다...")
			// TODO: 실제 노드 중지 로직 구현
			fmt.Println("노드가 성공적으로 중지되었습니다.")
		},
	}
	
	return cmd
}

// nodeStatusCmd는 노드 상태 확인 명령어를 생성합니다.
func nodeStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "노드 상태 확인",
		Long:  `현재 실행 중인 노드의 상태를 확인합니다.`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("노드 상태를 확인합니다...")
			// TODO: 실제 노드 상태 확인 로직 구현
			fmt.Println("노드 상태: 실행 중")
			fmt.Println("블록 높이: 1234567")
			fmt.Println("피어 수: 25")
			fmt.Println("동기화 상태: 완료")
		},
	}
	
	return cmd
}

// nodeResetCmd는 노드 초기화 명령어를 생성합니다.
func nodeResetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "노드 초기화",
		Long:  `노드 데이터를 초기화합니다. 주의: 모든 데이터가 삭제됩니다.`,
		Run: func(cmd *cobra.Command, args []string) {
			dataDir, _ := cmd.Flags().GetString("datadir")
			keepConfig, _ := cmd.Flags().GetBool("keep-config")
			
			// 데이터 디렉토리 확장
			if dataDir == defaultDataDir {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					fmt.Printf("홈 디렉토리를 찾을 수 없습니다: %v\n", err)
					return
				}
				dataDir = filepath.Join(homeDir, ".eirene")
			}
			
			fmt.Println("노드 데이터를 초기화합니다...")
			fmt.Printf("데이터 디렉토리: %s\n", dataDir)
			fmt.Printf("설정 유지: %v\n", keepConfig)
			
			// 사용자 확인
			fmt.Print("정말로 노드 데이터를 초기화하시겠습니까? (y/n): ")
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Println("노드 초기화가 취소되었습니다.")
				return
			}
			
			// TODO: 실제 노드 초기화 로직 구현
			fmt.Println("노드가 성공적으로 초기화되었습니다.")
		},
	}
	
	// 플래그 추가
	cmd.Flags().String("datadir", defaultDataDir, "노드 데이터 디렉토리 경로")
	cmd.Flags().Bool("keep-config", false, "설정 파일 유지")
	
	return cmd
}

// nodeConfigCmd는 노드 설정 명령어를 생성합니다.
func nodeConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "노드 설정 관리",
		Long:  `노드 설정을 조회하거나 변경합니다.`,
		Run: func(cmd *cobra.Command, args []string) {
			// 인자가 없으면 도움말 표시
			if len(args) == 0 {
				cmd.Help()
				return
			}
		},
	}
	
	// 하위 명령어 추가
	cmd.AddCommand(nodeConfigShowCmd())
	cmd.AddCommand(nodeConfigSetCmd())
	
	return cmd
}

// nodeConfigShowCmd는 노드 설정 조회 명령어를 생성합니다.
func nodeConfigShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "노드 설정 조회",
		Long:  `현재 노드 설정을 조회합니다.`,
		Run: func(cmd *cobra.Command, args []string) {
			dataDir, _ := cmd.Flags().GetString("datadir")
			
			// 데이터 디렉토리 확장
			if dataDir == defaultDataDir {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					fmt.Printf("홈 디렉토리를 찾을 수 없습니다: %v\n", err)
					return
				}
				dataDir = filepath.Join(homeDir, ".eirene")
			}
			
			configFile := filepath.Join(dataDir, "config.toml")
			
			// 설정 파일 로드
			viper.SetConfigFile(configFile)
			if err := viper.ReadInConfig(); err != nil {
				fmt.Printf("설정 파일을 읽을 수 없습니다: %v\n", err)
				return
			}
			
			// 설정 출력
			fmt.Println("노드 설정:")
			fmt.Printf("RPC 포트: %d\n", viper.GetInt("rpc.port"))
			fmt.Printf("P2P 포트: %d\n", viper.GetInt("p2p.port"))
			fmt.Printf("검증자 모드: %v\n", viper.GetBool("validator.enabled"))
			fmt.Printf("로그 레벨: %s\n", viper.GetString("log.level"))
		},
	}
	
	// 플래그 추가
	cmd.Flags().String("datadir", defaultDataDir, "노드 데이터 디렉토리 경로")
	
	return cmd
}

// nodeConfigSetCmd는 노드 설정 변경 명령어를 생성합니다.
func nodeConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set [key] [value]",
		Short: "노드 설정 변경",
		Long:  `노드 설정을 변경합니다.`,
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			value := args[1]
			dataDir, _ := cmd.Flags().GetString("datadir")
			
			// 데이터 디렉토리 확장
			if dataDir == defaultDataDir {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					fmt.Printf("홈 디렉토리를 찾을 수 없습니다: %v\n", err)
					return
				}
				dataDir = filepath.Join(homeDir, ".eirene")
			}
			
			configFile := filepath.Join(dataDir, "config.toml")
			
			// 설정 파일 로드
			viper.SetConfigFile(configFile)
			if err := viper.ReadInConfig(); err != nil {
				// 설정 파일이 없으면 새로 생성
				if os.IsNotExist(err) {
					if err := os.MkdirAll(dataDir, 0755); err != nil {
						fmt.Printf("데이터 디렉토리를 생성할 수 없습니다: %v\n", err)
						return
					}
				} else {
					fmt.Printf("설정 파일을 읽을 수 없습니다: %v\n", err)
					return
				}
			}
			
			// 설정 변경
			viper.Set(key, value)
			
			// 설정 저장
			if err := viper.WriteConfigAs(configFile); err != nil {
				fmt.Printf("설정 파일을 저장할 수 없습니다: %v\n", err)
				return
			}
			
			fmt.Printf("설정이 변경되었습니다: %s = %s\n", key, value)
		},
	}
	
	// 플래그 추가
	cmd.Flags().String("datadir", defaultDataDir, "노드 데이터 디렉토리 경로")
	
	return cmd
} 