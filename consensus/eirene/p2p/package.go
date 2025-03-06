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

// Package p2p는 Eirene 합의 알고리즘을 위한 P2P 네트워킹 기능을 제공합니다.
// 이 패키지는 go-zenanet의 P2P 모듈을 활용하여 Eirene 합의 알고리즘에 필요한
// 피어 검색, 연결 관리, 블록 및 트랜잭션 전파 등의 기능을 구현합니다.
package p2p

import (
	"time"

	"github.com/zenanetwork/go-zenanet/log"
)

// 로깅을 위한 기본 로거
var logger = log.New("module", "eirene/p2p")

// 프로토콜 상수
const (
	// 프로토콜 이름
	ProtocolName = "eirene"
	
	// 프로토콜 버전
	ProtocolVersion = 1
	
	// 최대 메시지 크기
	MaxMessageSize = 10 * 1024 * 1024 // 10MB
	
	// 핑 간격
	PingInterval = 15 * time.Second
)

// 메시지 코드
const (
	// 상태 메시지
	StatusMsg = 0x00
	
	// 블록 관련 메시지
	NewBlockMsg        = 0x01
	BlockRequestMsg    = 0x02
	BlockResponseMsg   = 0x03
	
	// 트랜잭션 관련 메시지
	NewTxMsg           = 0x04
	TxRequestMsg       = 0x05
	TxResponseMsg      = 0x06
	
	// 검증자 관련 메시지
	ValidatorSetMsg    = 0x07
	
	// 합의 관련 메시지
	VoteMsg            = 0x08
	ProposalMsg        = 0x09
	
	// 증거 관련 메시지
	EvidenceMsg        = 0x0A
	
	// 기타 메시지
	PingMsg            = 0x0B
	PongMsg            = 0x0C
)

// 알려진 항목 캐시 상수
const (
	// 최대 알려진 트랜잭션 수
	maxKnownTxs = 32768
	
	// 최대 알려진 블록 수
	maxKnownBlocks = 1024
	
	// 최대 대기 트랜잭션 수
	maxQueuedTxs = 4096
	
	// 최대 대기 블록 수
	maxQueuedBlocks = 1024
	
	// 최대 피어 수
	maxPeers = 50
	
	// 브로드캐스트 고루틴 수
	txConcurrency = 10
	blockConcurrency = 5
) 