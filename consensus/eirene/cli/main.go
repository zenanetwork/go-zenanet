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

// Package main은 Eirene 합의 알고리즘을 위한 CLI 도구를 제공합니다.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/cli/cmd"
)

func main() {
	// 루트 명령어 생성
	rootCmd := &cobra.Command{
		Use:   "eirene",
		Short: "Eirene 합의 알고리즘을 위한 CLI 도구",
		Long: `Eirene CLI는 Zenanet 블록체인의 Eirene 합의 알고리즘을 관리하기 위한 명령줄 인터페이스입니다.
이 도구를 사용하여 노드 관리, 계정 관리, 트랜잭션 관리, 스테이킹 및 거버넌스 관련 작업을 수행할 수 있습니다.`,
		Run: func(cmd *cobra.Command, args []string) {
			// 인자가 없으면 도움말 표시
			if len(args) == 0 {
				cmd.Help()
				return
			}
		},
	}

	// 하위 명령어 추가
	rootCmd.AddCommand(cmd.NodeCmd())
	rootCmd.AddCommand(cmd.AccountCmd())
	rootCmd.AddCommand(cmd.TxCmd())
	rootCmd.AddCommand(cmd.StakingCmd())
	rootCmd.AddCommand(cmd.GovernanceCmd())
	rootCmd.AddCommand(cmd.NetworkCmd())
	rootCmd.AddCommand(cmd.VersionCmd())

	// 명령어 실행
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
} 