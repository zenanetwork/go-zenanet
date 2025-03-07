// Copyright 2023 The go-zenanet Authors
// This file is part of go-zenanet.
//
// go-zenanet is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-zenanet is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-zenanet. If not, see <http://www.gnu.org/licenses/>.

// Package utils contains internal helper functions for go-zenanet commands.
package utils

import (
	"github.com/zenanetwork/go-zenanet/eth/ethconfig"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/node"
	"github.com/zenanetwork/go-zenanet/params"
)

// RegisterEireneService는 Eirene 합의 알고리즘 서비스를 노드에 등록합니다.
func RegisterEireneService(stack *node.Node, ethConfig *ethconfig.Config, eireneConfig interface{}, isMainnet bool) {
	// 현재는 로그만 출력하는 간단한 구현
	if isMainnet {
		log.Info("Eirene 메인넷 서비스 등록")
	} else {
		log.Info("Eirene 테스트넷 서비스 등록")
	}

	// Eirene 설정 로그 출력
	if cfg, ok := eireneConfig.(*params.EireneConfig); ok {
		log.Info("Eirene 합의 알고리즘 설정",
			"period", cfg.Period,
			"epoch", cfg.Epoch,
			"slashingThreshold", cfg.SlashingThreshold,
			"slashingRate", cfg.SlashingRate,
			"missedBlockPenalty", cfg.MissedBlockPenalty)
	} else if cfg, ok := eireneConfig.(struct {
		Period             uint64
		Epoch              uint64
		SlashingThreshold  uint64
		SlashingRate       uint64
		MissedBlockPenalty uint64
	}); ok {
		log.Info("Eirene 합의 알고리즘 설정",
			"period", cfg.Period,
			"epoch", cfg.Epoch,
			"slashingThreshold", cfg.SlashingThreshold,
			"slashingRate", cfg.SlashingRate,
			"missedBlockPenalty", cfg.MissedBlockPenalty)
	} else {
		log.Warn("알 수 없는 Eirene 설정 형식")
	}

	// TODO: 실제 Eirene 서비스 등록 구현
	// 예: stack.Register(func(ctx *node.ServiceContext) (node.Service, error) {
	//     return eirene.New(ctx, ethConfig, eireneConfig)
	// })
} 