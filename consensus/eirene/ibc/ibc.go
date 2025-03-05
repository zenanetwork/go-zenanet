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

package ibc

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/log"
)

// 상수 정의
const (
	// 채널 상태
	ChannelStateInit     = 0
	ChannelStateOpen     = 1
	ChannelStateClosed   = 2

	// 연결 상태
	ConnectionStateInit  = 0
	ConnectionStateOpen  = 1
	ConnectionStateClosed = 2

	// 클라이언트 상태
	ClientStateActive    = 0
	ClientStateExpired   = 1
	ClientStateFrozen    = 2

	// 기본 타임아웃
	DefaultTimeoutHeight = 1000 // 기본 타임아웃 높이 (블록 수)
	DefaultTimeoutTime   = 3600 // 기본 타임아웃 시간 (초)
)

// IBCPacket은 IBC 패킷을 나타냅니다
type IBCPacket struct {
	Sequence         uint64 `json:"sequence"`         // 패킷 시퀀스 번호
	SourcePort       string `json:"sourcePort"`       // 소스 포트
	SourceChannel    string `json:"sourceChannel"`    // 소스 채널
	DestPort         string `json:"destPort"`         // 목적지 포트
	DestChannel      string `json:"destChannel"`      // 목적지 채널
	Data             []byte `json:"data"`             // 패킷 데이터
	TimeoutHeight    uint64 `json:"timeoutHeight"`    // 타임아웃 높이
	TimeoutTimestamp uint64 `json:"timeoutTimestamp"` // 타임아웃 타임스탬프
}

// IBCTransferData는 토큰 전송 데이터를 나타냅니다
type IBCTransferData struct {
	Token    common.Address `json:"token"`    // 토큰 주소
	Amount   *big.Int       `json:"amount"`   // 전송 금액
	Sender   common.Address `json:"sender"`   // 송신자 주소
	Receiver string         `json:"receiver"` // 수신자 주소 (다른 체인의 주소 형식)
	Memo     string         `json:"memo"`     // 메모
}

// IBCAcknowledgementData는 패킷 확인 응답 데이터를 나타냅니다
type IBCAcknowledgementData struct {
	OriginalSequence uint64 `json:"originalSequence"` // 원본 패킷 시퀀스 번호
	Success          bool   `json:"success"`          // 성공 여부
	Error            string `json:"error"`            // 오류 메시지 (실패 시)
	Result           []byte `json:"result"`           // 결과 데이터 (성공 시)
}

// IBCChannel은 IBC 채널을 나타냅니다
type IBCChannel struct {
	PortID                string `json:"portId"`                // 포트 ID
	ChannelID             string `json:"channelId"`             // 채널 ID
	CounterpartyPortID    string `json:"counterpartyPortId"`    // 상대방 포트 ID
	CounterpartyChannelID string `json:"counterpartyChannelId"` // 상대방 채널 ID
	State                 uint8  `json:"state"`                 // 채널 상태
	Version               string `json:"version"`               // 채널 버전
	ConnectionID          string `json:"connectionId"`          // 연결 ID
	NextSequenceSend      uint64 `json:"nextSequenceSend"`      // 다음 전송 시퀀스 번호
	NextSequenceRecv      uint64 `json:"nextSequenceRecv"`      // 다음 수신 시퀀스 번호
	NextSequenceAck       uint64 `json:"nextSequenceAck"`       // 다음 확인 시퀀스 번호
}

// IBCConnection은 IBC 연결을 나타냅니다
type IBCConnection struct {
	ID                       string   `json:"id"`                       // 연결 ID
	ClientID                 string   `json:"clientId"`                 // 클라이언트 ID
	CounterpartyClientID     string   `json:"counterpartyClientId"`     // 상대방 클라이언트 ID
	CounterpartyConnectionID string   `json:"counterpartyConnectionId"` // 상대방 연결 ID
	State                    uint8    `json:"state"`                    // 연결 상태
	Versions                 []string `json:"versions"`                 // 연결 버전
}

// IBCClient는 IBC 클라이언트를 나타냅니다
type IBCClient struct {
	ID             string `json:"id"`             // 클라이언트 ID
	Type           string `json:"type"`           // 클라이언트 유형 (tendermint, solo-machine 등)
	State          uint8  `json:"state"`          // 클라이언트 상태
	LatestHeight   uint64 `json:"latestHeight"`   // 최신 높이
	TrustingPeriod uint64 `json:"trustingPeriod"` // 신뢰 기간 (블록 수)
	ConsensusState []byte `json:"consensusState"` // 합의 상태
}

// IBCState는 IBC 상태를 나타냅니다
type IBCState struct {
	Clients          map[string]*IBCClient              `json:"clients"`          // 클라이언트 맵
	Connections      map[string]*IBCConnection          `json:"connections"`      // 연결 맵
	Channels         map[string]*IBCChannel             `json:"channels"`         // 채널 맵
	Packets          map[uint64]*IBCPacket              `json:"packets"`          // 패킷 맵
	Acknowledgements map[uint64]*IBCAcknowledgementData `json:"acknowledgements"` // 확인 응답 맵

	// 통계
	TotalPacketsSent         uint64 `json:"totalPacketsSent"`         // 총 전송 패킷 수
	TotalPacketsReceived     uint64 `json:"totalPacketsReceived"`     // 총 수신 패킷 수
	TotalPacketsAcknowledged uint64 `json:"totalPacketsAcknowledged"` // 총 확인 패킷 수
	TotalPacketsTimedOut     uint64 `json:"totalPacketsTimedOut"`     // 총 타임아웃 패킷 수

	lock sync.RWMutex // 동시성 제어를 위한 잠금
}

// newIBCState는 새로운 IBC 상태를 생성합니다
func newIBCState() *IBCState {
	return &IBCState{
		Clients:          make(map[string]*IBCClient),
		Connections:      make(map[string]*IBCConnection),
		Channels:         make(map[string]*IBCChannel),
		Packets:          make(map[uint64]*IBCPacket),
		Acknowledgements: make(map[uint64]*IBCAcknowledgementData),
	}
}

// loadIBCState는 데이터베이스에서 IBC 상태를 로드합니다
func loadIBCState(db ethdb.Database) (*IBCState, error) {
	data, err := db.Get([]byte("ibc-state"))
	if err != nil {
		// 상태가 없으면 새로 생성
		if err == errors.New("not found") {
			return newIBCState(), nil
		}
		return nil, err
	}

	var state IBCState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// store는 IBC 상태를 데이터베이스에 저장합니다
func (is *IBCState) store(db ethdb.Database) error {
	is.lock.RLock()
	defer is.lock.RUnlock()

	data, err := json.Marshal(is)
	if err != nil {
		return err
	}

	return db.Put([]byte("ibc-state"), data)
}

// createClient는 새로운 IBC 클라이언트를 생성합니다
func (is *IBCState) createClient(id string, clientType string, consensusState []byte, trustingPeriod uint64) (*IBCClient, error) {
	is.lock.Lock()
	defer is.lock.Unlock()

	// 클라이언트 ID 중복 확인
	if _, exists := is.Clients[id]; exists {
		return nil, errors.New("client already exists")
	}

	// 클라이언트 유형 확인
	if clientType != "tendermint" && clientType != "solo-machine" {
		return nil, errors.New("unsupported client type")
	}

	// 클라이언트 생성
	client := &IBCClient{
		ID:             id,
		Type:           clientType,
		State:          ClientStateActive,
		LatestHeight:   0,
		TrustingPeriod: trustingPeriod,
		ConsensusState: consensusState,
	}

	is.Clients[id] = client
	log.Info("IBC client created", "id", id, "type", clientType)
	return client, nil
}

// updateClient는 IBC 클라이언트를 업데이트합니다
func (is *IBCState) updateClient(id string, height uint64, consensusState []byte) error {
	is.lock.Lock()
	defer is.lock.Unlock()

	// 클라이언트 존재 확인
	client, exists := is.Clients[id]
	if !exists {
		return errors.New("client not found")
	}

	// 클라이언트 상태 확인
	if client.State != ClientStateActive {
		return errors.New("client is not active")
	}

	// 높이 확인
	if height <= client.LatestHeight {
		return errors.New("height must be greater than latest height")
	}

	// 클라이언트 업데이트
	client.LatestHeight = height
	client.ConsensusState = consensusState

	log.Info("IBC client updated", "id", id, "height", height)
	return nil
}

// createConnection은 새로운 IBC 연결을 생성합니다
func (is *IBCState) createConnection(id string, clientID string, counterpartyClientID string, counterpartyConnectionID string, version string) (*IBCConnection, error) {
	is.lock.Lock()
	defer is.lock.Unlock()

	// 연결 ID 중복 확인
	if _, exists := is.Connections[id]; exists {
		return nil, errors.New("connection already exists")
	}

	// 클라이언트 존재 확인
	client, exists := is.Clients[clientID]
	if !exists {
		return nil, errors.New("client not found")
	}

	// 클라이언트 상태 확인
	if client.State != ClientStateActive {
		return nil, errors.New("client is not active")
	}

	// 연결 생성
	connection := &IBCConnection{
		ID:                       id,
		ClientID:                 clientID,
		CounterpartyClientID:     counterpartyClientID,
		CounterpartyConnectionID: counterpartyConnectionID,
		State:                    ConnectionStateInit,
		Versions:                 []string{version},
	}

	is.Connections[id] = connection
	log.Info("IBC connection created", "id", id, "clientID", clientID)
	return connection, nil
}

// openConnection은 IBC 연결을 열기 상태로 변경합니다
func (is *IBCState) openConnection(id string) error {
	is.lock.Lock()
	defer is.lock.Unlock()

	// 연결 존재 확인
	connection, exists := is.Connections[id]
	if !exists {
		return errors.New("connection not found")
	}

	// 연결 상태 확인
	if connection.State != ConnectionStateInit {
		return errors.New("connection is not in init state")
	}

	// 연결 상태 변경
	connection.State = ConnectionStateOpen
	log.Info("IBC connection opened", "id", id)
	return nil
}

// createChannel은 새로운 IBC 채널을 생성합니다
func (is *IBCState) createChannel(portID string, channelID string, connectionID string, counterpartyPortID string, counterpartyChannelID string, version string) (*IBCChannel, error) {
	is.lock.Lock()
	defer is.lock.Unlock()

	// 채널 ID 중복 확인
	channelKey := portID + "/" + channelID
	if _, exists := is.Channels[channelKey]; exists {
		return nil, errors.New("channel already exists")
	}

	// 연결 존재 확인
	connection, exists := is.Connections[connectionID]
	if !exists {
		return nil, errors.New("connection not found")
	}

	// 연결 상태 확인
	if connection.State != ConnectionStateOpen {
		return nil, errors.New("connection is not open")
	}

	// 채널 생성
	channel := &IBCChannel{
		PortID:                portID,
		ChannelID:             channelID,
		CounterpartyPortID:    counterpartyPortID,
		CounterpartyChannelID: counterpartyChannelID,
		State:                 ChannelStateInit,
		Version:               version,
		ConnectionID:          connectionID,
		NextSequenceSend:      1,
		NextSequenceRecv:      1,
		NextSequenceAck:       1,
	}

	is.Channels[channelKey] = channel
	log.Info("IBC channel created", "port", portID, "channel", channelID, "connection", connectionID)
	return channel, nil
}

// openChannel은 IBC 채널을 열기 상태로 변경합니다
func (is *IBCState) openChannel(portID string, channelID string) error {
	is.lock.Lock()
	defer is.lock.Unlock()

	// 채널 존재 확인
	channelKey := portID + "/" + channelID
	channel, exists := is.Channels[channelKey]
	if !exists {
		return errors.New("channel not found")
	}

	// 채널 상태 확인
	if channel.State != ChannelStateInit {
		return errors.New("channel is not in init state")
	}

	// 채널 상태 변경
	channel.State = ChannelStateOpen
	log.Info("IBC channel opened", "port", portID, "channel", channelID)
	return nil
}

// closeChannel은 IBC 채널을 닫기 상태로 변경합니다
func (is *IBCState) closeChannel(portID string, channelID string) error {
	is.lock.Lock()
	defer is.lock.Unlock()

	// 채널 존재 확인
	channelKey := portID + "/" + channelID
	channel, exists := is.Channels[channelKey]
	if !exists {
		return errors.New("channel not found")
	}

	// 채널 상태 확인
	if channel.State != ChannelStateOpen {
		return errors.New("channel is not open")
	}

	// 채널 상태 변경
	channel.State = ChannelStateClosed
	log.Info("IBC channel closed", "port", portID, "channel", channelID)
	return nil
}

// sendPacket은 IBC 패킷을 전송합니다
func (is *IBCState) sendPacket(sourcePort string, sourceChannel string, destPort string, destChannel string, data []byte, timeoutHeight uint64, timeoutTimestamp uint64) (*IBCPacket, error) {
	is.lock.Lock()
	defer is.lock.Unlock()

	// 채널 존재 확인
	channelKey := sourcePort + "/" + sourceChannel
	channel, exists := is.Channels[channelKey]
	if !exists {
		return nil, errors.New("channel not found")
	}

	// 채널 상태 확인
	if channel.State != ChannelStateOpen {
		return nil, errors.New("channel is not open")
	}

	// 목적지 확인
	if channel.CounterpartyPortID != destPort || channel.CounterpartyChannelID != destChannel {
		return nil, errors.New("invalid destination")
	}

	// 패킷 생성
	sequence := channel.NextSequenceSend
	packet := &IBCPacket{
		Sequence:         sequence,
		SourcePort:       sourcePort,
		SourceChannel:    sourceChannel,
		DestPort:         destPort,
		DestChannel:      destChannel,
		Data:             data,
		TimeoutHeight:    timeoutHeight,
		TimeoutTimestamp: timeoutTimestamp,
	}

	// 패킷 저장
	is.Packets[sequence] = packet

	// 시퀀스 증가
	channel.NextSequenceSend++

	// 통계 업데이트
	is.TotalPacketsSent++

	log.Info("IBC packet sent", "sequence", sequence, "sourcePort", sourcePort, "sourceChannel", sourceChannel)
	return packet, nil
}

// receivePacket은 IBC 패킷을 수신합니다
func (is *IBCState) receivePacket(packet *IBCPacket) error {
	is.lock.Lock()
	defer is.lock.Unlock()

	// 채널 존재 확인
	channelKey := packet.DestPort + "/" + packet.DestChannel
	channel, exists := is.Channels[channelKey]
	if !exists {
		return errors.New("channel not found")
	}

	// 채널 상태 확인
	if channel.State != ChannelStateOpen {
		return errors.New("channel is not open")
	}

	// 소스 확인
	if channel.CounterpartyPortID != packet.SourcePort || channel.CounterpartyChannelID != packet.SourceChannel {
		return errors.New("invalid source")
	}

	// 시퀀스 확인
	if packet.Sequence != channel.NextSequenceRecv {
		return fmt.Errorf("invalid sequence: expected %d, got %d", channel.NextSequenceRecv, packet.Sequence)
	}

	// 시퀀스 증가
	channel.NextSequenceRecv++

	// 통계 업데이트
	is.TotalPacketsReceived++

	log.Info("IBC packet received", "sequence", packet.Sequence, "destPort", packet.DestPort, "destChannel", packet.DestChannel)
	return nil
}

// acknowledgePacket은 IBC 패킷을 확인 응답합니다
func (is *IBCState) acknowledgePacket(sequence uint64, success bool, result []byte, errorMsg string) error {
	is.lock.Lock()
	defer is.lock.Unlock()

	// 패킷 존재 확인
	packet, exists := is.Packets[sequence]
	if !exists {
		return errors.New("packet not found")
	}

	// 채널 존재 확인
	channelKey := packet.SourcePort + "/" + packet.SourceChannel
	channel, exists := is.Channels[channelKey]
	if !exists {
		return errors.New("channel not found")
	}

	// 채널 상태 확인
	if channel.State != ChannelStateOpen {
		return errors.New("channel is not open")
	}

	// 확인 응답 생성
	ack := &IBCAcknowledgementData{
		OriginalSequence: sequence,
		Success:          success,
		Error:            errorMsg,
		Result:           result,
	}

	// 확인 응답 저장
	is.Acknowledgements[sequence] = ack

	// 패킷 삭제
	delete(is.Packets, sequence)

	// 시퀀스 증가
	channel.NextSequenceAck++

	// 통계 업데이트
	is.TotalPacketsAcknowledged++

	log.Info("IBC packet acknowledged", "sequence", sequence, "success", success)
	return nil
}

// timeoutPacket은 IBC 패킷을 타임아웃 처리합니다
func (is *IBCState) timeoutPacket(sequence uint64) error {
	is.lock.Lock()
	defer is.lock.Unlock()

	// 패킷 존재 확인
	packet, exists := is.Packets[sequence]
	if !exists {
		return errors.New("packet not found")
	}

	// 채널 존재 확인
	channelKey := packet.SourcePort + "/" + packet.SourceChannel
	channel, exists := is.Channels[channelKey]
	if !exists {
		return errors.New("channel not found")
	}

	// 채널 상태 확인
	if channel.State != ChannelStateOpen {
		return errors.New("channel is not open")
	}

	// 패킷 삭제
	delete(is.Packets, sequence)

	// 통계 업데이트
	is.TotalPacketsTimedOut++

	log.Info("IBC packet timed out", "sequence", sequence)
	return nil
}

// TransferToken은 IBC를 통해 토큰을 전송합니다
func TransferToken(ibcState *IBCState, sourcePort string, sourceChannel string, token common.Address, amount *big.Int, sender common.Address, receiver string, timeoutHeight uint64, timeoutTimestamp uint64) (*IBCPacket, error) {
	// 전송 데이터 생성
	transferData := IBCTransferData{
		Token:    token,
		Amount:   amount,
		Sender:   sender,
		Receiver: receiver,
		Memo:     "",
	}

	// 데이터 인코딩
	data, err := json.Marshal(transferData)
	if err != nil {
		return nil, err
	}

	// 채널 존재 확인
	channelKey := sourcePort + "/" + sourceChannel
	channel, exists := ibcState.Channels[channelKey]
	if !exists {
		return nil, errors.New("channel not found")
	}

	// 패킷 전송
	packet, err := ibcState.sendPacket(
		sourcePort,
		sourceChannel,
		channel.CounterpartyPortID,
		channel.CounterpartyChannelID,
		data,
		timeoutHeight,
		timeoutTimestamp,
	)
	if err != nil {
		return nil, err
	}

	log.Info("IBC token transfer initiated", 
		"token", token.Hex(), 
		"amount", amount.String(), 
		"sender", sender.Hex(), 
		"receiver", receiver)
	return packet, nil
}

// ProcessIBCPackets는 IBC 패킷을 처리합니다
func ProcessIBCPackets(ibcState *IBCState, currentBlock uint64, currentTime uint64) {
	ibcState.lock.Lock()
	defer ibcState.lock.Unlock()

	// 타임아웃된 패킷 처리
	for sequence, packet := range ibcState.Packets {
		if (packet.TimeoutHeight > 0 && currentBlock >= packet.TimeoutHeight) ||
			(packet.TimeoutTimestamp > 0 && currentTime >= packet.TimeoutTimestamp) {
			// 패킷 타임아웃 처리
			delete(ibcState.Packets, sequence)
			ibcState.TotalPacketsTimedOut++
			log.Info("IBC packet timed out during processing", "sequence", sequence)
		}
	}
}

// GetState는 클라이언트 상태를 반환합니다
func (c *IBCClient) GetState() uint8 {
	return c.State
}

// GetLatestHeight는 클라이언트의 최신 높이를 반환합니다
func (c *IBCClient) GetLatestHeight() uint64 {
	return c.LatestHeight
}

// GetVersions는 연결 버전을 반환합니다
func (c *IBCConnection) GetVersions() []string {
	return c.Versions
}

// GetNextSequenceSend는 채널의 다음 전송 시퀀스 번호를 반환합니다
func (c *IBCChannel) GetNextSequenceSend() uint64 {
	return c.NextSequenceSend
}

// GetNextSequenceRecv는 채널의 다음 수신 시퀀스 번호를 반환합니다
func (c *IBCChannel) GetNextSequenceRecv() uint64 {
	return c.NextSequenceRecv
}

// GetNextSequenceAck는 채널의 다음 확인 시퀀스 번호를 반환합니다
func (c *IBCChannel) GetNextSequenceAck() uint64 {
	return c.NextSequenceAck
}

// GetDestPort는 패킷의 목적지 포트를 반환합니다
func (p *IBCPacket) GetDestPort() string {
	return p.DestPort
}

// GetDestChannel는 패킷의 목적지 채널을 반환합니다
func (p *IBCPacket) GetDestChannel() string {
	return p.DestChannel
}
