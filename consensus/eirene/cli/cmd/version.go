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

package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// 버전 정보
var (
	Version   = "1.0.0"
	Commit    = "abcdef"
	BuildDate = "2023-01-01"
)

// VersionCmd는 버전 정보 명령어를 생성합니다.
func VersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "버전 정보 표시",
		Long:  `Eirene CLI 도구의 버전 정보를 표시합니다.`,
		Run: func(cmd *cobra.Command, args []string) {
			// 버전 정보 표시
			fmt.Println("Eirene CLI 버전 정보:")
			fmt.Printf("  버전: %s\n", Version)
			fmt.Printf("  커밋: %s\n", Commit)
			fmt.Printf("  빌드 날짜: %s\n", BuildDate)
			fmt.Printf("  Go 버전: %s\n", runtime.Version())
			fmt.Printf("  OS/아키텍처: %s/%s\n", runtime.GOOS, runtime.GOARCH)

			// 상세 정보 표시 여부
			detailed, _ := cmd.Flags().GetBool("detailed")
			if detailed {
				fmt.Println("\n상세 정보:")
				fmt.Printf("  GOMAXPROCS: %d\n", runtime.GOMAXPROCS(0))
				fmt.Printf("  NumCPU: %d\n", runtime.NumCPU())
				fmt.Printf("  NumGoroutine: %d\n", runtime.NumGoroutine())
				
				// 의존성 정보 표시
				fmt.Println("\n주요 의존성:")
				fmt.Println("  github.com/cosmos/cosmos-sdk: v0.46.0")
				fmt.Println("  github.com/tendermint/tendermint: v0.34.24")
				fmt.Println("  github.com/spf13/cobra: v1.6.1")
				fmt.Println("  github.com/spf13/viper: v1.15.0")
			}
		},
	}

	// 플래그 추가
	cmd.Flags().Bool("detailed", false, "상세 버전 정보 표시")

	return cmd
} 