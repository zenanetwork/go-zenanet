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
	"errors"
	"math/big"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/rlp"
)

// IBC 관련 상수
const (
	// 패킷 유형
	IBCPacketTypeTransfer        = 0 // 자산 전송
	IBCPacketTypeAcknowledgement = 1 // 확인 응답
	IBCPacketTypeTimeout         = 2 // 타임아웃

	// 채널 상태
	IBCChannelStateInit   = 0 // 초기화
	IBCChannelStateOpen   = 1 // 열림
	IBCChannelStateClosed = 2 // 닫힘

	// 클라이언트 상태
	IBCClientStateActive  = 0 // 활성
	IBCClientStateExpired = 1 // 만료
	IBCClientStateFrozen  = 2 // 동결

	// 타임아웃 기간 (블록 수)
	IBCDefaultTimeoutPeriod = 10000 // 약 1.7일 (15초 블록 기준)
)

// IBCPacket은 IBC 패킷을 나타냅니다.
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

// IBCTransferData는 자산 전송 데이터를 나타냅니다.
type IBCTransferData struct {
	Token    common.Address `json:"token"`    // 토큰 주소
	Amount   *big.Int       `json:"amount"`   // 전송 금액
	Sender   common.Address `json:"sender"`   // 송신자 주소
	Receiver string         `json:"receiver"` // 수신자 주소 (다른 체인의 주소 형식)
	Memo     string         `json:"memo"`     // 메모
}

// IBCAcknowledgementData는 확인 응답 데이터를 나타냅니다.
type IBCAcknowledgementData struct {
	OriginalSequence uint64 `json:"originalSequence"` // 원본 패킷 시퀀스 번호
	Success          bool   `json:"success"`          // 성공 여부
	Error            string `json:"error"`            // 오류 메시지 (실패 시)
	Result           []byte `json:"result"`           // 결과 데이터 (성공 시)
}

// IBCChannel은 IBC 채널을 나타냅니다.
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

// IBCConnection은 IBC 연결을 나타냅니다.
type IBCConnection struct {
	ID                       string   `json:"id"`                       // 연결 ID
	ClientID                 string   `json:"clientId"`                 // 클라이언트 ID
	CounterpartyClientID     string   `json:"counterpartyClientId"`     // 상대방 클라이언트 ID
	CounterpartyConnectionID string   `json:"counterpartyConnectionId"` // 상대방 연결 ID
	State                    uint8    `json:"state"`                    // 연결 상태
	Versions                 []string `json:"versions"`                 // 연결 버전
}

// IBCClient는 IBC 클라이언트를 나타냅니다.
type IBCClient struct {
	ID             string `json:"id"`             // 클라이언트 ID
	Type           string `json:"type"`           // 클라이언트 유형 (tendermint, solo-machine 등)
	State          uint8  `json:"state"`          // 클라이언트 상태
	LatestHeight   uint64 `json:"latestHeight"`   // 최신 높이
	TrustingPeriod uint64 `json:"trustingPeriod"` // 신뢰 기간 (블록 수)
	ConsensusState []byte `json:"consensusState"` // 합의 상태
}

// IBCState는 IBC 상태를 나타냅니다.
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
}

// newIBCState는 새로운 IBC 상태를 생성합니다.
func newIBCState() *IBCState {
	return &IBCState{
		Clients:          make(map[string]*IBCClient),
		Connections:      make(map[string]*IBCConnection),
		Channels:         make(map[string]*IBCChannel),
		Packets:          make(map[uint64]*IBCPacket),
		Acknowledgements: make(map[uint64]*IBCAcknowledgementData),
	}
}

// loadIBCState는 데이터베이스에서 IBC 상태를 로드합니다.
func loadIBCState(db ethdb.Database) (*IBCState, error) {
	data, err := db.Get([]byte("eirene-ibc"))
	if err != nil {
		// 데이터가 없으면 새로운 상태 생성
		return newIBCState(), nil
	}

	var state IBCState
	if err := rlp.DecodeBytes(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// store는 IBC 상태를 데이터베이스에 저장합니다.
func (is *IBCState) store(db ethdb.Database) error {
	data, err := rlp.EncodeToBytes(is)
	if err != nil {
		return err
	}

	return db.Put([]byte("eirene-ibc"), data)
}

// createClient는 새로운 IBC 클라이언트를 생성합니다.
func (is *IBCState) createClient(id string, clientType string, consensusState []byte, trustingPeriod uint64) (*IBCClient, error) {
	// 클라이언트 ID 중복 확인
	if _, exists := is.Clients[id]; exists {
		return nil, errors.New("client already exists")
	}

	// 클라이언트 생성
	client := &IBCClient{
		ID:             id,
		Type:           clientType,
		State:          IBCClientStateActive,
		LatestHeight:   0,
		TrustingPeriod: trustingPeriod,
		ConsensusState: consensusState,
	}

	// 클라이언트 저장
	is.Clients[id] = client

	log.Info("IBC client created", "id", id, "type", clientType)

	return client, nil
}

// updateClient는 IBC 클라이언트를 업데이트합니다.
func (is *IBCState) updateClient(id string, height uint64, consensusState []byte) error {
	// 클라이언트 확인
	client, exists := is.Clients[id]
	if !exists {
		return errors.New("client not found")
	}

	// 클라이언트 상태 확인
	if client.State != IBCClientStateActive {
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

// createConnection은 새로운 IBC 연결을 생성합니다.
func (is *IBCState) createConnection(id string, clientID string, counterpartyClientID string, counterpartyConnectionID string, version string) (*IBCConnection, error) {
	// 연결 ID 중복 확인
	if _, exists := is.Connections[id]; exists {
		return nil, errors.New("connection already exists")
	}

	// 클라이언트 확인
	client, exists := is.Clients[clientID]
	if !exists {
		return nil, errors.New("client not found")
	}

	// 클라이언트 상태 확인
	if client.State != IBCClientStateActive {
		return nil, errors.New("client is not active")
	}

	// 연결 생성
	connection := &IBCConnection{
		ID:                       id,
		ClientID:                 clientID,
		CounterpartyClientID:     counterpartyClientID,
		CounterpartyConnectionID: counterpartyConnectionID,
		State:                    IBCChannelStateInit,
		Versions:                 []string{version},
	}

	// 연결 저장
	is.Connections[id] = connection

	log.Info("IBC connection created", "id", id, "clientId", clientID)

	return connection, nil
}

// openConnection은 IBC 연결을 엽니다.
func (is *IBCState) openConnection(id string) error {
	// 연결 확인
	connection, exists := is.Connections[id]
	if !exists {
		return errors.New("connection not found")
	}

	// 연결 상태 확인
	if connection.State != IBCChannelStateInit {
		return errors.New("connection is not in init state")
	}

	// 연결 열기
	connection.State = IBCChannelStateOpen

	log.Info("IBC connection opened", "id", id)

	return nil
}

// createChannel은 새로운 IBC 채널을 생성합니다.
func (is *IBCState) createChannel(portID string, channelID string, connectionID string, counterpartyPortID string, counterpartyChannelID string, version string) (*IBCChannel, error) {
	// 채널 ID 중복 확인
	channelKey := portID + "/" + channelID
	if _, exists := is.Channels[channelKey]; exists {
		return nil, errors.New("channel already exists")
	}

	// 연결 확인
	connection, exists := is.Connections[connectionID]
	if !exists {
		return nil, errors.New("connection not found")
	}

	// 연결 상태 확인
	if connection.State != IBCChannelStateOpen {
		return nil, errors.New("connection is not open")
	}

	// 채널 생성
	channel := &IBCChannel{
		PortID:                portID,
		ChannelID:             channelID,
		CounterpartyPortID:    counterpartyPortID,
		CounterpartyChannelID: counterpartyChannelID,
		State:                 IBCChannelStateInit,
		Version:               version,
		ConnectionID:          connectionID,
		NextSequenceSend:      1,
		NextSequenceRecv:      1,
		NextSequenceAck:       1,
	}

	// 채널 저장
	is.Channels[channelKey] = channel

	log.Info("IBC channel created", "portId", portID, "channelId", channelID)

	return channel, nil
}

// openChannel은 IBC 채널을 엽니다.
func (is *IBCState) openChannel(portID string, channelID string) error {
	// 채널 확인
	channelKey := portID + "/" + channelID
	channel, exists := is.Channels[channelKey]
	if !exists {
		return errors.New("channel not found")
	}

	// 채널 상태 확인
	if channel.State != IBCChannelStateInit {
		return errors.New("channel is not in init state")
	}

	// 채널 열기
	channel.State = IBCChannelStateOpen

	log.Info("IBC channel opened", "portId", portID, "channelId", channelID)

	return nil
}

// closeChannel은 IBC 채널을 닫습니다.
func (is *IBCState) closeChannel(portID string, channelID string) error {
	// 채널 확인
	channelKey := portID + "/" + channelID
	channel, exists := is.Channels[channelKey]
	if !exists {
		return errors.New("channel not found")
	}

	// 채널 상태 확인
	if channel.State != IBCChannelStateOpen {
		return errors.New("channel is not open")
	}

	// 채널 닫기
	channel.State = IBCChannelStateClosed

	log.Info("IBC channel closed", "portId", portID, "channelId", channelID)

	return nil
}

// sendPacket은 IBC 패킷을 전송합니다.
func (is *IBCState) sendPacket(sourcePort string, sourceChannel string, destPort string, destChannel string, data []byte, timeoutHeight uint64, timeoutTimestamp uint64) (*IBCPacket, error) {
	// 채널 확인
	channelKey := sourcePort + "/" + sourceChannel
	channel, exists := is.Channels[channelKey]
	if !exists {
		return nil, errors.New("channel not found")
	}

	// 채널 상태 확인
	if channel.State != IBCChannelStateOpen {
		return nil, errors.New("channel is not open")
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

// receivePacket은 IBC 패킷을 수신합니다.
func (is *IBCState) receivePacket(packet *IBCPacket) error {
	// 채널 확인
	channelKey := packet.DestPort + "/" + packet.DestChannel
	channel, exists := is.Channels[channelKey]
	if !exists {
		return errors.New("channel not found")
	}

	// 채널 상태 확인
	if channel.State != IBCChannelStateOpen {
		return errors.New("channel is not open")
	}

	// 시퀀스 확인
	if packet.Sequence != channel.NextSequenceRecv {
		return errors.New("invalid sequence")
	}

	// 타임아웃 확인
	currentTime := uint64(time.Now().Unix())
	if packet.TimeoutTimestamp > 0 && currentTime >= packet.TimeoutTimestamp {
		return errors.New("packet timed out")
	}

	// 시퀀스 증가
	channel.NextSequenceRecv++

	// 통계 업데이트
	is.TotalPacketsReceived++

	log.Info("IBC packet received", "sequence", packet.Sequence, "destPort", packet.DestPort, "destChannel", packet.DestChannel)

	return nil
}

// acknowledgePacket은 IBC 패킷을 확인합니다.
func (is *IBCState) acknowledgePacket(sequence uint64, success bool, result []byte, errorMsg string) error {
	// 패킷 확인
	packet, exists := is.Packets[sequence]
	if !exists {
		return errors.New("packet not found")
	}

	// 채널 확인
	channelKey := packet.SourcePort + "/" + packet.SourceChannel
	channel, exists := is.Channels[channelKey]
	if !exists {
		return errors.New("channel not found")
	}

	// 채널 상태 확인
	if channel.State != IBCChannelStateOpen {
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

	// 시퀀스 증가
	channel.NextSequenceAck++

	// 통계 업데이트
	is.TotalPacketsAcknowledged++

	log.Info("IBC packet acknowledged", "sequence", sequence, "success", success)

	return nil
}

// timeoutPacket은 IBC 패킷을 타임아웃 처리합니다.
func (is *IBCState) timeoutPacket(sequence uint64) error {
	// 패킷 확인
	packet, exists := is.Packets[sequence]
	if !exists {
		return errors.New("packet not found")
	}

	// 채널 확인
	channelKey := packet.SourcePort + "/" + packet.SourceChannel
	channel, exists := is.Channels[channelKey]
	if !exists {
		return errors.New("channel not found")
	}

	// 채널 상태 확인
	if channel.State != IBCChannelStateOpen {
		return errors.New("channel is not open")
	}

	// 타임아웃 확인
	currentTime := uint64(time.Now().Unix())
	currentHeight := uint64(0) // 실제 구현에서는 현재 블록 높이를 가져와야 함

	if (packet.TimeoutHeight > 0 && currentHeight >= packet.TimeoutHeight) ||
		(packet.TimeoutTimestamp > 0 && currentTime >= packet.TimeoutTimestamp) {
		// 패킷 삭제
		delete(is.Packets, sequence)

		// 통계 업데이트
		is.TotalPacketsTimedOut++

		log.Info("IBC packet timed out", "sequence", sequence)

		return nil
	}

	return errors.New("packet has not timed out")
}

// TransferToken은 IBC를 통해 토큰을 전송합니다.
func TransferToken(ibcState *IBCState, sourcePort string, sourceChannel string, token common.Address, amount *big.Int, sender common.Address, receiver string, timeoutHeight uint64, timeoutTimestamp uint64) (*IBCPacket, error) {
	// 전송 데이터 생성
	transferData := &IBCTransferData{
		Token:    token,
		Amount:   amount,
		Sender:   sender,
		Receiver: receiver,
		Memo:     "",
	}

	// 데이터 인코딩
	data, err := rlp.EncodeToBytes(transferData)
	if err != nil {
		return nil, err
	}

	// 채널 확인
	channel, exists := ibcState.Channels[sourcePort+"/"+sourceChannel]
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

	// IBC 상태 저장
	if err := ibcState.store(nil); err != nil {
		log.Error("IBC 상태 저장 실패", "err", err)
	}

	return packet, nil
}

// ProcessIBCPackets는 IBC 패킷을 처리합니다.
func ProcessIBCPackets(ibcState *IBCState, currentBlock uint64, currentTime uint64) {
	// 타임아웃된 패킷 처리
	for sequence, packet := range ibcState.Packets {
		if (packet.TimeoutHeight > 0 && currentBlock >= packet.TimeoutHeight) ||
			(packet.TimeoutTimestamp > 0 && currentTime >= packet.TimeoutTimestamp) {
			// 패킷 타임아웃 처리
			if err := ibcState.timeoutPacket(sequence); err != nil {
				log.Error("패킷 타임아웃 처리 실패", "err", err)
			}
		}
	}

	// IBC 상태 저장
	if err := ibcState.store(nil); err != nil {
		log.Error("IBC 상태 저장 실패", "err", err)
	}
}

// IBCClient에 대한 getter 메서드 추가
func (c *IBCClient) GetState() uint8 {
	return c.State
}

func (c *IBCClient) GetLatestHeight() uint64 {
	return c.LatestHeight
}

// IBCConnection에 대한 getter 메서드 추가
func (c *IBCConnection) GetVersions() []string {
	return c.Versions
}

// IBCChannel에 대한 getter 메서드 추가
func (c *IBCChannel) GetNextSequenceSend() uint64 {
	return c.NextSequenceSend
}

func (c *IBCChannel) GetNextSequenceRecv() uint64 {
	return c.NextSequenceRecv
}

func (c *IBCChannel) GetNextSequenceAck() uint64 {
	return c.NextSequenceAck
}

// IBCPacket에 대한 getter 메서드 추가
func (p *IBCPacket) GetDestPort() string {
	return p.DestPort
}

func (p *IBCPacket) GetDestChannel() string {
	return p.DestChannel
}
