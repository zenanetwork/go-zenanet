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
	"math/big"
	"testing"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/rawdb"
)

// 테스트용 상수
const (
	testClientID     = "07-tendermint-0"
	testConnectionID = "connection-0"
	testPortID       = "transfer"
	testChannelID    = "channel-0"
)

// TestIBCState는 IBC 상태 관리를 테스트합니다.
func TestIBCState(t *testing.T) {
	// 새로운 IBC 상태 생성
	ibcState := newIBCState()

	// 초기 상태 확인
	if len(ibcState.Clients) != 0 {
		t.Errorf("초기 Clients가 비어있지 않음: %d", len(ibcState.Clients))
	}

	if len(ibcState.Connections) != 0 {
		t.Errorf("초기 Connections가 비어있지 않음: %d", len(ibcState.Connections))
	}

	if len(ibcState.Channels) != 0 {
		t.Errorf("초기 Channels가 비어있지 않음: %d", len(ibcState.Channels))
	}

	if len(ibcState.Packets) != 0 {
		t.Errorf("초기 Packets가 비어있지 않음: %d", len(ibcState.Packets))
	}

	if ibcState.TotalPacketsSent != 0 {
		t.Errorf("초기 TotalPacketsSent가 0이 아님: %d", ibcState.TotalPacketsSent)
	}
}

// TestCreateClient는 IBC 클라이언트 생성을 테스트합니다.
func TestCreateClient(t *testing.T) {
	// 새로운 IBC 상태 생성
	ibcState := newIBCState()

	// 클라이언트 생성
	clientType := "tendermint"
	consensusState := []byte("test consensus state")
	trustingPeriod := uint64(100000)

	client, err := ibcState.createClient(testClientID, clientType, consensusState, trustingPeriod)
	if err != nil {
		t.Fatalf("클라이언트 생성 실패: %v", err)
	}

	// 클라이언트 확인
	if client.ID != testClientID {
		t.Errorf("클라이언트 ID가 일치하지 않음: %s != %s", client.ID, testClientID)
	}

	if client.Type != clientType {
		t.Errorf("클라이언트 유형이 일치하지 않음: %s != %s", client.Type, clientType)
	}

	if client.State != ClientStateActive {
		t.Errorf("클라이언트 상태가 활성 상태가 아님: %d", client.State)
	}

	if client.TrustingPeriod != trustingPeriod {
		t.Errorf("클라이언트 신뢰 기간이 일치하지 않음: %d != %d", client.TrustingPeriod, trustingPeriod)
	}

	// 중복 클라이언트 생성 시도
	_, err = ibcState.createClient(testClientID, clientType, consensusState, trustingPeriod)
	if err == nil {
		t.Errorf("중복 클라이언트 생성이 성공함")
	}
}

// TestUpdateClient는 IBC 클라이언트 업데이트를 테스트합니다.
func TestUpdateClient(t *testing.T) {
	// 새로운 IBC 상태 생성
	ibcState := newIBCState()

	// 클라이언트 생성
	clientType := "tendermint"
	consensusState := []byte("test consensus state")
	trustingPeriod := uint64(100000)

	_, err := ibcState.createClient(testClientID, clientType, consensusState, trustingPeriod)
	if err != nil {
		t.Fatalf("클라이언트 생성 실패: %v", err)
	}

	// 클라이언트 업데이트
	newHeight := uint64(100)
	newConsensusState := []byte("new consensus state")

	err = ibcState.updateClient(testClientID, newHeight, newConsensusState)
	if err != nil {
		t.Fatalf("클라이언트 업데이트 실패: %v", err)
	}

	// 업데이트된 클라이언트 확인
	client := ibcState.Clients[testClientID]

	if client.LatestHeight != newHeight {
		t.Errorf("클라이언트 높이가 일치하지 않음: %d != %d", client.LatestHeight, newHeight)
	}

	if string(client.ConsensusState) != string(newConsensusState) {
		t.Errorf("클라이언트 합의 상태가 일치하지 않음")
	}

	// 존재하지 않는 클라이언트 업데이트 시도
	err = ibcState.updateClient("non-existent", newHeight, newConsensusState)
	if err == nil {
		t.Errorf("존재하지 않는 클라이언트 업데이트가 성공함")
	}
}

// TestCreateConnection은 IBC 연결 생성을 테스트합니다.
func TestCreateConnection(t *testing.T) {
	// 새로운 IBC 상태 생성
	ibcState := newIBCState()

	// 클라이언트 생성
	clientType := "tendermint"
	consensusState := []byte("test consensus state")
	trustingPeriod := uint64(100000)

	_, err := ibcState.createClient(testClientID, clientType, consensusState, trustingPeriod)
	if err != nil {
		t.Fatalf("클라이언트 생성 실패: %v", err)
	}

	// 연결 생성
	counterpartyClientID := "07-tendermint-1"
	counterpartyConnectionID := "connection-1"
	version := "1.0"

	connection, err := ibcState.createConnection(
		testConnectionID,
		testClientID,
		counterpartyClientID,
		counterpartyConnectionID,
		version,
	)
	if err != nil {
		t.Fatalf("연결 생성 실패: %v", err)
	}

	// 연결 확인
	if connection.ID != testConnectionID {
		t.Errorf("연결 ID가 일치하지 않음: %s != %s", connection.ID, testConnectionID)
	}

	if connection.ClientID != testClientID {
		t.Errorf("연결 클라이언트 ID가 일치하지 않음: %s != %s", connection.ClientID, testClientID)
	}

	if connection.CounterpartyClientID != counterpartyClientID {
		t.Errorf("연결 상대방 클라이언트 ID가 일치하지 않음: %s != %s", connection.CounterpartyClientID, counterpartyClientID)
	}

	if connection.CounterpartyConnectionID != counterpartyConnectionID {
		t.Errorf("연결 상대방 연결 ID가 일치하지 않음: %s != %s", connection.CounterpartyConnectionID, counterpartyConnectionID)
	}

	if connection.State != ConnectionStateInit {
		t.Errorf("연결 상태가 초기화 상태가 아님: %d", connection.State)
	}

	if len(connection.Versions) != 1 || connection.Versions[0] != version {
		t.Errorf("연결 버전이 일치하지 않음: %v != [%s]", connection.Versions, version)
	}

	// 중복 연결 생성 시도
	_, err = ibcState.createConnection(
		testConnectionID,
		testClientID,
		counterpartyClientID,
		counterpartyConnectionID,
		version,
	)
	if err == nil {
		t.Errorf("중복 연결 생성이 성공함")
	}

	// 존재하지 않는 클라이언트로 연결 생성 시도
	_, err = ibcState.createConnection(
		"connection-2",
		"non-existent",
		counterpartyClientID,
		counterpartyConnectionID,
		version,
	)
	if err == nil {
		t.Errorf("존재하지 않는 클라이언트로 연결 생성이 성공함")
	}
}

// TestOpenConnection은 IBC 연결 열기를 테스트합니다.
func TestOpenConnection(t *testing.T) {
	// 새로운 IBC 상태 생성
	ibcState := newIBCState()

	// 클라이언트 생성
	clientType := "tendermint"
	consensusState := []byte("test consensus state")
	trustingPeriod := uint64(100000)

	_, err := ibcState.createClient(testClientID, clientType, consensusState, trustingPeriod)
	if err != nil {
		t.Fatalf("클라이언트 생성 실패: %v", err)
	}

	// 연결 생성
	counterpartyClientID := "07-tendermint-1"
	counterpartyConnectionID := "connection-1"
	version := "1.0"

	_, err = ibcState.createConnection(
		testConnectionID,
		testClientID,
		counterpartyClientID,
		counterpartyConnectionID,
		version,
	)
	if err != nil {
		t.Fatalf("연결 생성 실패: %v", err)
	}

	// 연결 열기
	err = ibcState.openConnection(testConnectionID)
	if err != nil {
		t.Fatalf("연결 열기 실패: %v", err)
	}

	// 열린 연결 확인
	connection := ibcState.Connections[testConnectionID]

	if connection.State != ConnectionStateOpen {
		t.Errorf("연결 상태가 열림 상태가 아님: %d", connection.State)
	}

	// 존재하지 않는 연결 열기 시도
	err = ibcState.openConnection("non-existent")
	if err == nil {
		t.Errorf("존재하지 않는 연결 열기가 성공함")
	}
}

// TestCreateChannel은 IBC 채널 생성을 테스트합니다.
func TestCreateChannel(t *testing.T) {
	// 새로운 IBC 상태 생성
	ibcState := newIBCState()

	// 클라이언트 생성
	clientType := "tendermint"
	consensusState := []byte("test consensus state")
	trustingPeriod := uint64(100000)

	_, err := ibcState.createClient(testClientID, clientType, consensusState, trustingPeriod)
	if err != nil {
		t.Fatalf("클라이언트 생성 실패: %v", err)
	}

	// 연결 생성
	counterpartyClientID := "07-tendermint-1"
	counterpartyConnectionID := "connection-1"
	version := "1.0"

	_, err = ibcState.createConnection(
		testConnectionID,
		testClientID,
		counterpartyClientID,
		counterpartyConnectionID,
		version,
	)
	if err != nil {
		t.Fatalf("연결 생성 실패: %v", err)
	}

	// 연결 열기
	err = ibcState.openConnection(testConnectionID)
	if err != nil {
		t.Fatalf("연결 열기 실패: %v", err)
	}

	// 채널 생성
	counterpartyPortID := "transfer"
	counterpartyChannelID := "channel-1"
	channelVersion := "ics20-1"

	channel, err := ibcState.createChannel(
		testPortID,
		testChannelID,
		testConnectionID,
		counterpartyPortID,
		counterpartyChannelID,
		channelVersion,
	)
	if err != nil {
		t.Fatalf("채널 생성 실패: %v", err)
	}

	// 채널 확인
	channelKey := testPortID + "/" + testChannelID

	// 채널 키 확인 (실제 구현에서는 이 키를 사용하여 채널을 조회할 수 있음)
	t.Logf("생성된 채널 키: %s", channelKey)

	if channel.PortID != testPortID {
		t.Errorf("채널 포트 ID가 일치하지 않음: %s != %s", channel.PortID, testPortID)
	}

	if channel.ChannelID != testChannelID {
		t.Errorf("채널 ID가 일치하지 않음: %s != %s", channel.ChannelID, testChannelID)
	}

	if channel.ConnectionID != testConnectionID {
		t.Errorf("채널 연결 ID가 일치하지 않음: %s != %s", channel.ConnectionID, testConnectionID)
	}

	if channel.CounterpartyPortID != counterpartyPortID {
		t.Errorf("채널 상대방 포트 ID가 일치하지 않음: %s != %s", channel.CounterpartyPortID, counterpartyPortID)
	}

	if channel.CounterpartyChannelID != counterpartyChannelID {
		t.Errorf("채널 상대방 채널 ID가 일치하지 않음: %s != %s", channel.CounterpartyChannelID, counterpartyChannelID)
	}

	if channel.State != ChannelStateInit {
		t.Errorf("채널 상태가 초기화 상태가 아님: %d", channel.State)
	}

	if channel.Version != channelVersion {
		t.Errorf("채널 버전이 일치하지 않음: %s != %s", channel.Version, channelVersion)
	}

	if channel.NextSequenceSend != 1 {
		t.Errorf("채널 다음 전송 시퀀스가 1이 아님: %d", channel.NextSequenceSend)
	}

	if channel.NextSequenceRecv != 1 {
		t.Errorf("채널 다음 수신 시퀀스가 1이 아님: %d", channel.NextSequenceRecv)
	}

	if channel.NextSequenceAck != 1 {
		t.Errorf("채널 다음 확인 시퀀스가 1이 아님: %d", channel.NextSequenceAck)
	}

	// 중복 채널 생성 시도
	_, err = ibcState.createChannel(
		testPortID,
		testChannelID,
		testConnectionID,
		counterpartyPortID,
		counterpartyChannelID,
		channelVersion,
	)
	if err == nil {
		t.Errorf("중복 채널 생성이 성공함")
	}

	// 존재하지 않는 연결로 채널 생성 시도
	_, err = ibcState.createChannel(
		testPortID,
		"channel-2",
		"non-existent",
		counterpartyPortID,
		counterpartyChannelID,
		channelVersion,
	)
	if err == nil {
		t.Errorf("존재하지 않는 연결로 채널 생성이 성공함")
	}
}

// TestSendPacket은 IBC 패킷 전송을 테스트합니다.
func TestSendPacket(t *testing.T) {
	// 새로운 IBC 상태 생성
	ibcState := newIBCState()

	// 클라이언트 생성
	clientType := "tendermint"
	consensusState := []byte("test consensus state")
	trustingPeriod := uint64(100000)

	_, err := ibcState.createClient(testClientID, clientType, consensusState, trustingPeriod)
	if err != nil {
		t.Fatalf("클라이언트 생성 실패: %v", err)
	}

	// 연결 생성
	counterpartyClientID := "07-tendermint-1"
	counterpartyConnectionID := "connection-1"
	version := "1.0"

	_, err = ibcState.createConnection(
		testConnectionID,
		testClientID,
		counterpartyClientID,
		counterpartyConnectionID,
		version,
	)
	if err != nil {
		t.Fatalf("연결 생성 실패: %v", err)
	}

	// 연결 열기
	err = ibcState.openConnection(testConnectionID)
	if err != nil {
		t.Fatalf("연결 열기 실패: %v", err)
	}

	// 채널 생성
	counterpartyPortID := "transfer"
	counterpartyChannelID := "channel-1"
	channelVersion := "ics20-1"

	_, err = ibcState.createChannel(
		testPortID,
		testChannelID,
		testConnectionID,
		counterpartyPortID,
		counterpartyChannelID,
		channelVersion,
	)
	if err != nil {
		t.Fatalf("채널 생성 실패: %v", err)
	}

	// 채널 열기
	err = ibcState.openChannel(testPortID, testChannelID)
	if err != nil {
		t.Fatalf("채널 열기 실패: %v", err)
	}

	// 패킷 전송
	data := []byte("test data")
	timeoutHeight := uint64(1000)
	timeoutTimestamp := uint64(time.Now().Unix() + 3600)

	packet, err := ibcState.sendPacket(
		testPortID,
		testChannelID,
		counterpartyPortID,
		counterpartyChannelID,
		data,
		timeoutHeight,
		timeoutTimestamp,
	)
	if err != nil {
		t.Fatalf("패킷 전송 실패: %v", err)
	}

	// 패킷 확인
	if packet.Sequence != 1 {
		t.Errorf("패킷 시퀀스가 1이 아님: %d", packet.Sequence)
	}

	if packet.SourcePort != testPortID {
		t.Errorf("패킷 소스 포트가 일치하지 않음: %s != %s", packet.SourcePort, testPortID)
	}

	if packet.SourceChannel != testChannelID {
		t.Errorf("패킷 소스 채널이 일치하지 않음: %s != %s", packet.SourceChannel, testChannelID)
	}

	if packet.DestPort != counterpartyPortID {
		t.Errorf("패킷 목적지 포트가 일치하지 않음: %s != %s", packet.DestPort, counterpartyPortID)
	}

	if packet.DestChannel != counterpartyChannelID {
		t.Errorf("패킷 목적지 채널이 일치하지 않음: %s != %s", packet.DestChannel, counterpartyChannelID)
	}

	if string(packet.Data) != string(data) {
		t.Errorf("패킷 데이터가 일치하지 않음")
	}

	if packet.TimeoutHeight != timeoutHeight {
		t.Errorf("패킷 타임아웃 높이가 일치하지 않음: %d != %d", packet.TimeoutHeight, timeoutHeight)
	}

	if packet.TimeoutTimestamp != timeoutTimestamp {
		t.Errorf("패킷 타임아웃 타임스탬프가 일치하지 않음: %d != %d", packet.TimeoutTimestamp, timeoutTimestamp)
	}

	// 채널 시퀀스 확인
	channelKey := testPortID + "/" + testChannelID
	channel := ibcState.Channels[channelKey]

	if channel.NextSequenceSend != 2 {
		t.Errorf("채널 다음 전송 시퀀스가 2가 아님: %d", channel.NextSequenceSend)
	}

	// 통계 확인
	if ibcState.TotalPacketsSent != 1 {
		t.Errorf("총 전송 패킷 수가 1이 아님: %d", ibcState.TotalPacketsSent)
	}
}

// TestIBCEngine는 IBC 엔진의 기능을 테스트합니다.
func TestIBCEngine(t *testing.T) {
	// 테스트 데이터베이스 생성
	db := rawdb.NewMemoryDatabase()

	// IBC 상태 생성
	ibcState := newIBCState()

	// 클라이언트 생성
	clientID := "07-tendermint-0"
	consensusState := []byte("consensus state")
	trustingPeriod := uint64(100000)
	_, err := ibcState.createClient(clientID, "tendermint", consensusState, trustingPeriod)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// 연결 생성
	connectionID := "connection-0"
	counterpartyClientID := "07-tendermint-1"
	counterpartyConnectionID := "connection-1"
	version := "1.0"
	_, err = ibcState.createConnection(connectionID, clientID, counterpartyClientID, counterpartyConnectionID, version)
	if err != nil {
		t.Fatalf("Failed to create connection: %v", err)
	}

	// 연결 열기
	err = ibcState.openConnection(connectionID)
	if err != nil {
		t.Fatalf("Failed to open connection: %v", err)
	}

	// 채널 생성
	sourcePort := "transfer"
	sourceChannel := "channel-0"
	counterpartyPort := "transfer"
	counterpartyChannel := "channel-1"
	channelVersion := "ics20-1"
	_, err = ibcState.createChannel(sourcePort, sourceChannel, connectionID, counterpartyPort, counterpartyChannel, channelVersion)
	if err != nil {
		t.Fatalf("Failed to create channel: %v", err)
	}

	// 채널 열기
	err = ibcState.openChannel(sourcePort, sourceChannel)
	if err != nil {
		t.Fatalf("Failed to open channel: %v", err)
	}

	// 토큰 전송
	token := common.HexToAddress("0x1234567890123456789012345678901234567890")
	amount := big.NewInt(1000000000000000000) // 1 ETH
	sender := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")
	receiver := "cosmos1abcdefabcdefabcdefabcdefabcdefabcdefabc"
	timeoutHeight := uint64(1000)
	timeoutTimestamp := uint64(time.Now().Unix() + 3600)

	_, err = TransferToken(ibcState, sourcePort, sourceChannel, token, amount, sender, receiver, timeoutHeight, timeoutTimestamp)
	if err != nil {
		t.Fatalf("Failed to transfer token: %v", err)
	}

	// 패킷 처리
	currentBlock := uint64(500)
	currentTime := uint64(time.Now().Unix())
	ProcessIBCPackets(ibcState, currentBlock, currentTime)

	// 상태 저장
	err = ibcState.store(db)
	if err != nil {
		t.Fatalf("Failed to store IBC state: %v", err)
	}

	// 상태 로드
	loadedState, err := loadIBCState(db)
	if err != nil {
		t.Fatalf("Failed to load IBC state: %v", err)
	}

	// 검증
	if len(loadedState.Clients) != 1 {
		t.Errorf("Expected 1 client, got %d", len(loadedState.Clients))
	}
	if len(loadedState.Connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(loadedState.Connections))
	}
	if len(loadedState.Channels) != 1 {
		t.Errorf("Expected 1 channel, got %d", len(loadedState.Channels))
	}
	if len(loadedState.Packets) != 1 {
		t.Errorf("Expected 1 packet, got %d", len(loadedState.Packets))
	}
}

// TestCreateDuplicateClient는 중복 클라이언트 생성을 테스트합니다.
func TestCreateDuplicateClient(t *testing.T) {
	// 새로운 IBC 상태 생성
	ibcState := newIBCState()

	// 클라이언트 생성
	clientType := "tendermint"
	consensusState := []byte("test consensus state")
	trustingPeriod := uint64(100000)

	// 첫 번째 클라이언트 생성
	_, err := ibcState.createClient(testClientID, clientType, consensusState, trustingPeriod)
	if err != nil {
		t.Fatalf("첫 번째 클라이언트 생성 실패: %v", err)
	}

	// 동일한 ID로 두 번째 클라이언트 생성 시도
	_, err = ibcState.createClient(testClientID, clientType, consensusState, trustingPeriod)
	
	// 오류가 발생해야 함
	if err == nil {
		t.Error("중복 클라이언트 생성이 성공함")
	}
}

// TestUpdateNonExistentClient는 존재하지 않는 클라이언트 업데이트를 테스트합니다.
func TestUpdateNonExistentClient(t *testing.T) {
	// 새로운 IBC 상태 생성
	ibcState := newIBCState()

	// 존재하지 않는 클라이언트 ID
	nonExistentClientID := "07-tendermint-999"
	
	// 존재하지 않는 클라이언트 업데이트 시도
	height := uint64(100)
	consensusState := []byte("updated consensus state")
	err := ibcState.updateClient(nonExistentClientID, height, consensusState)
	
	// 오류가 발생해야 함
	if err == nil {
		t.Error("존재하지 않는 클라이언트 업데이트가 성공함")
	}
}

// TestCreateConnectionWithNonExistentClient는 존재하지 않는 클라이언트로 연결 생성을 테스트합니다.
func TestCreateConnectionWithNonExistentClient(t *testing.T) {
	// 새로운 IBC 상태 생성
	ibcState := newIBCState()

	// 존재하지 않는 클라이언트 ID
	nonExistentClientID := "07-tendermint-999"
	
	// 존재하지 않는 클라이언트로 연결 생성 시도
	connectionID := "connection-0"
	counterpartyClientID := "07-tendermint-1"
	counterpartyConnectionID := "connection-1"
	version := "1.0"
	
	_, err := ibcState.createConnection(connectionID, nonExistentClientID, counterpartyClientID, counterpartyConnectionID, version)
	
	// 오류가 발생해야 함
	if err == nil {
		t.Error("존재하지 않는 클라이언트로 연결 생성이 성공함")
	}
}

// TestCreateDuplicateConnection는 중복 연결 생성을 테스트합니다.
func TestCreateDuplicateConnection(t *testing.T) {
	// 새로운 IBC 상태 생성
	ibcState := newIBCState()

	// 클라이언트 생성
	clientType := "tendermint"
	consensusState := []byte("test consensus state")
	trustingPeriod := uint64(100000)
	
	_, err := ibcState.createClient(testClientID, clientType, consensusState, trustingPeriod)
	if err != nil {
		t.Fatalf("클라이언트 생성 실패: %v", err)
	}

	// 연결 생성
	connectionID := "connection-0"
	counterpartyClientID := "07-tendermint-1"
	counterpartyConnectionID := "connection-1"
	version := "1.0"
	
	_, err = ibcState.createConnection(connectionID, testClientID, counterpartyClientID, counterpartyConnectionID, version)
	if err != nil {
		t.Fatalf("첫 번째 연결 생성 실패: %v", err)
	}

	// 동일한 ID로 두 번째 연결 생성 시도
	_, err = ibcState.createConnection(connectionID, testClientID, counterpartyClientID, counterpartyConnectionID, version)
	
	// 오류가 발생해야 함
	if err == nil {
		t.Error("중복 연결 생성이 성공함")
	}
}

// TestOpenNonExistentConnection는 존재하지 않는 연결 열기를 테스트합니다.
func TestOpenNonExistentConnection(t *testing.T) {
	// 새로운 IBC 상태 생성
	ibcState := newIBCState()

	// 존재하지 않는 연결 ID
	nonExistentConnectionID := "connection-999"
	
	// 존재하지 않는 연결 열기 시도
	err := ibcState.openConnection(nonExistentConnectionID)
	
	// 오류가 발생해야 함
	if err == nil {
		t.Error("존재하지 않는 연결 열기가 성공함")
	}
}

// TestCreateChannelWithNonExistentConnection는 존재하지 않는 연결로 채널 생성을 테스트합니다.
func TestCreateChannelWithNonExistentConnection(t *testing.T) {
	// 새로운 IBC 상태 생성
	ibcState := newIBCState()

	// 존재하지 않는 연결 ID
	nonExistentConnectionID := "connection-999"
	
	// 존재하지 않는 연결로 채널 생성 시도
	portID := "transfer"
	channelID := "channel-0"
	counterpartyPortID := "transfer"
	counterpartyChannelID := "channel-1"
	version := "ics20-1"
	
	_, err := ibcState.createChannel(portID, channelID, nonExistentConnectionID, counterpartyPortID, counterpartyChannelID, version)
	
	// 오류가 발생해야 함
	if err == nil {
		t.Error("존재하지 않는 연결로 채널 생성이 성공함")
	}
}

// TestCreateDuplicateChannel는 중복 채널 생성을 테스트합니다.
func TestCreateDuplicateChannel(t *testing.T) {
	// 새로운 IBC 상태 생성
	ibcState := newIBCState()

	// 클라이언트 생성
	clientType := "tendermint"
	consensusState := []byte("test consensus state")
	trustingPeriod := uint64(100000)
	
	_, err := ibcState.createClient(testClientID, clientType, consensusState, trustingPeriod)
	if err != nil {
		t.Fatalf("클라이언트 생성 실패: %v", err)
	}

	// 연결 생성
	connectionID := "connection-0"
	counterpartyClientID := "07-tendermint-1"
	counterpartyConnectionID := "connection-1"
	connVersion := "1.0"
	
	_, err = ibcState.createConnection(connectionID, testClientID, counterpartyClientID, counterpartyConnectionID, connVersion)
	if err != nil {
		t.Fatalf("연결 생성 실패: %v", err)
	}

	// 연결 열기
	err = ibcState.openConnection(connectionID)
	if err != nil {
		t.Fatalf("연결 열기 실패: %v", err)
	}

	// 채널 생성
	portID := "transfer"
	channelID := "channel-0"
	counterpartyPortID := "transfer"
	counterpartyChannelID := "channel-1"
	channelVersion := "ics20-1"
	
	_, err = ibcState.createChannel(portID, channelID, connectionID, counterpartyPortID, counterpartyChannelID, channelVersion)
	if err != nil {
		t.Fatalf("첫 번째 채널 생성 실패: %v", err)
	}

	// 동일한 포트 ID와 채널 ID로 두 번째 채널 생성 시도
	_, err = ibcState.createChannel(portID, channelID, connectionID, counterpartyPortID, counterpartyChannelID, channelVersion)
	
	// 오류가 발생해야 함
	if err == nil {
		t.Error("중복 채널 생성이 성공함")
	}
}

// TestOpenNonExistentChannel는 존재하지 않는 채널 열기를 테스트합니다.
func TestOpenNonExistentChannel(t *testing.T) {
	// 새로운 IBC 상태 생성
	ibcState := newIBCState()

	// 존재하지 않는 포트 ID와 채널 ID
	nonExistentPortID := "transfer"
	nonExistentChannelID := "channel-999"
	
	// 존재하지 않는 채널 열기 시도
	err := ibcState.openChannel(nonExistentPortID, nonExistentChannelID)
	
	// 오류가 발생해야 함
	if err == nil {
		t.Error("존재하지 않는 채널 열기가 성공함")
	}
}

// TestCloseNonExistentChannel는 존재하지 않는 채널 닫기를 테스트합니다.
func TestCloseNonExistentChannel(t *testing.T) {
	// 새로운 IBC 상태 생성
	ibcState := newIBCState()

	// 존재하지 않는 포트 ID와 채널 ID
	nonExistentPortID := "transfer"
	nonExistentChannelID := "channel-999"
	
	// 존재하지 않는 채널 닫기 시도
	err := ibcState.closeChannel(nonExistentPortID, nonExistentChannelID)
	
	// 오류가 발생해야 함
	if err == nil {
		t.Error("존재하지 않는 채널 닫기가 성공함")
	}
}

// TestSendPacketWithNonExistentChannel는 존재하지 않는 채널로 패킷 전송을 테스트합니다.
func TestSendPacketWithNonExistentChannel(t *testing.T) {
	// 새로운 IBC 상태 생성
	ibcState := newIBCState()

	// 존재하지 않는 포트 ID와 채널 ID
	nonExistentPortID := "transfer"
	nonExistentChannelID := "channel-999"
	
	// 존재하지 않는 채널로 패킷 전송 시도
	destPort := "transfer"
	destChannel := "channel-1"
	data := []byte("test data")
	timeoutHeight := uint64(1000)
	timeoutTimestamp := uint64(0)
	
	_, err := ibcState.sendPacket(nonExistentPortID, nonExistentChannelID, destPort, destChannel, data, timeoutHeight, timeoutTimestamp)
	
	// 오류가 발생해야 함
	if err == nil {
		t.Error("존재하지 않는 채널로 패킷 전송이 성공함")
	}
}

// TestSendPacketWithClosedChannel는 닫힌 채널로 패킷 전송을 테스트합니다.
func TestSendPacketWithClosedChannel(t *testing.T) {
	// 새로운 IBC 상태 생성
	ibcState := newIBCState()

	// 클라이언트 생성
	clientType := "tendermint"
	consensusState := []byte("test consensus state")
	trustingPeriod := uint64(100000)
	
	_, err := ibcState.createClient(testClientID, clientType, consensusState, trustingPeriod)
	if err != nil {
		t.Fatalf("클라이언트 생성 실패: %v", err)
	}

	// 연결 생성
	connectionID := "connection-0"
	counterpartyClientID := "07-tendermint-1"
	counterpartyConnectionID := "connection-1"
	connVersion := "1.0"
	
	_, err = ibcState.createConnection(connectionID, testClientID, counterpartyClientID, counterpartyConnectionID, connVersion)
	if err != nil {
		t.Fatalf("연결 생성 실패: %v", err)
	}

	// 연결 열기
	err = ibcState.openConnection(connectionID)
	if err != nil {
		t.Fatalf("연결 열기 실패: %v", err)
	}

	// 채널 생성
	portID := "transfer"
	channelID := "channel-0"
	counterpartyPortID := "transfer"
	counterpartyChannelID := "channel-1"
	channelVersion := "ics20-1"
	
	_, err = ibcState.createChannel(portID, channelID, connectionID, counterpartyPortID, counterpartyChannelID, channelVersion)
	if err != nil {
		t.Fatalf("채널 생성 실패: %v", err)
	}

	// 채널 열기
	err = ibcState.openChannel(portID, channelID)
	if err != nil {
		t.Fatalf("채널 열기 실패: %v", err)
	}

	// 채널 닫기
	err = ibcState.closeChannel(portID, channelID)
	if err != nil {
		t.Fatalf("채널 닫기 실패: %v", err)
	}

	// 닫힌 채널로 패킷 전송 시도
	destPort := "transfer"
	destChannel := "channel-1"
	data := []byte("test data")
	timeoutHeight := uint64(1000)
	timeoutTimestamp := uint64(0)
	
	_, err = ibcState.sendPacket(portID, channelID, destPort, destChannel, data, timeoutHeight, timeoutTimestamp)
	
	// 오류가 발생해야 함
	if err == nil {
		t.Error("닫힌 채널로 패킷 전송이 성공함")
	}
}
