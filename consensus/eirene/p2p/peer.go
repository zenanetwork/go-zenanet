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
	"errors"
	"fmt"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/p2p"
	"github.com/zenanetwork/go-zenanet/p2p/enode"
)

// Peer는 Eirene 프로토콜 피어를 나타냅니다.
type Peer struct {
	id     string        // 피어 ID
	peer   *p2p.Peer     // 기본 p2p 피어
	rw     p2p.MsgReadWriter // 메시지 읽기/쓰기 인터페이스
	version uint          // 프로토콜 버전
	
	head   common.Hash   // 현재 헤드 해시
	td     *big.Int      // 총 난이도
	
	knownTxs    *KnownCache // 알려진 트랜잭션 캐시
	knownBlocks *KnownCache // 알려진 블록 캐시
	
	lock  sync.RWMutex   // 동시성 제어를 위한 락
}

// NewPeer는 새로운 Eirene 프로토콜 피어를 생성합니다.
func NewPeer(version uint, p *p2p.Peer, rw p2p.MsgReadWriter) *Peer {
	return &Peer{
		id:          p.ID().String(),
		peer:        p,
		rw:          rw,
		version:     version,
		td:          big.NewInt(0),
		knownTxs:    NewKnownCache(maxKnownTxs),
		knownBlocks: NewKnownCache(maxKnownBlocks),
	}
}

// ID는 피어의 ID를 반환합니다.
func (p *Peer) ID() enode.ID {
	return p.peer.ID()
}

// Name은 피어의 이름을 반환합니다.
func (p *Peer) Name() string {
	return p.peer.Name()
}

// Version은 피어의 프로토콜 버전을 반환합니다.
func (p *Peer) Version() uint {
	return p.version
}

// Head는 피어의 현재 헤드 해시를 반환합니다.
func (p *Peer) Head() common.Hash {
	p.lock.RLock()
	defer p.lock.RUnlock()
	
	return p.head
}

// TD는 피어의 총 난이도를 반환합니다.
func (p *Peer) TD() *big.Int {
	p.lock.RLock()
	defer p.lock.RUnlock()
	
	return new(big.Int).Set(p.td)
}

// SetHead는 피어의 현재 헤드 해시를 설정합니다.
func (p *Peer) SetHead(hash common.Hash, td *big.Int) {
	p.lock.Lock()
	defer p.lock.Unlock()
	
	p.head = hash
	p.td.Set(td)
}

// KnownTransaction은 피어가 트랜잭션을 알고 있는지 확인합니다.
func (p *Peer) KnownTransaction(hash common.Hash) bool {
	return p.knownTxs.Contains(hash)
}

// KnownBlock은 피어가 블록을 알고 있는지 확인합니다.
func (p *Peer) KnownBlock(hash common.Hash) bool {
	return p.knownBlocks.Contains(hash)
}

// MarkTransaction은 피어가 트랜잭션을 알고 있음을 표시합니다.
func (p *Peer) MarkTransaction(hash common.Hash) {
	p.knownTxs.Add(hash)
}

// MarkBlock은 피어가 블록을 알고 있음을 표시합니다.
func (p *Peer) MarkBlock(hash common.Hash) {
	p.knownBlocks.Add(hash)
}

// Handshake는 피어와 핸드셰이크를 수행합니다.
func (p *Peer) Handshake(head common.Hash, td *big.Int) error {
	// 상태 메시지 전송
	err := p2p.Send(p.rw, StatusMsg, []interface{}{
		ProtocolVersion,
		head,
		td,
	})
	if err != nil {
		return err
	}
	
	// 상태 메시지 수신
	msg, err := p.rw.ReadMsg()
	if err != nil {
		return err
	}
	if msg.Code != StatusMsg {
		return fmt.Errorf("첫 메시지가 상태 메시지가 아님: %v", msg.Code)
	}
	if msg.Size > MaxMessageSize {
		return fmt.Errorf("메시지가 너무 큼: %v > %v", msg.Size, MaxMessageSize)
	}
	
	// 상태 메시지 디코딩
	var status []interface{}
	if err := msg.Decode(&status); err != nil {
		return fmt.Errorf("상태 메시지 디코딩 실패: %v", err)
	}
	if len(status) < 3 {
		return errors.New("상태 메시지가 너무 짧음")
	}
	
	// 프로토콜 버전 확인
	version, ok := status[0].(uint64)
	if !ok {
		return errors.New("첫 번째 상태 항목이 프로토콜 버전이 아님")
	}
	if version != ProtocolVersion {
		return fmt.Errorf("프로토콜 버전 불일치: %v != %v", version, ProtocolVersion)
	}
	
	// 헤드 해시 확인
	headHash, ok := status[1].(common.Hash)
	if !ok {
		return errors.New("두 번째 상태 항목이 헤드 해시가 아님")
	}
	
	// 총 난이도 확인
	peerTd, ok := status[2].(*big.Int)
	if !ok {
		return errors.New("세 번째 상태 항목이 총 난이도가 아님")
	}
	
	// 피어 상태 설정
	p.SetHead(headHash, peerTd)
	
	return nil
}

// SendNewBlock은 피어에게 새 블록을 전송합니다.
func (p *Peer) SendNewBlock(block *types.Block, td *big.Int) error {
	p.knownBlocks.Add(block.Hash())
	return p2p.Send(p.rw, NewBlockMsg, []interface{}{block, td})
}

// SendNewTx는 피어에게 새 트랜잭션을 전송합니다.
func (p *Peer) SendNewTx(txs []*types.Transaction) error {
	for _, tx := range txs {
		p.knownTxs.Add(tx.Hash())
	}
	return p2p.Send(p.rw, NewTxMsg, txs)
}

// SendValidatorSet은 피어에게 검증자 집합을 전송합니다.
func (p *Peer) SendValidatorSet(validators []common.Address) error {
	return p2p.Send(p.rw, ValidatorSetMsg, validators)
}

// SendVote는 피어에게 투표를 전송합니다.
func (p *Peer) SendVote(vote []byte) error {
	return p2p.Send(p.rw, VoteMsg, vote)
}

// SendProposal은 피어에게 제안을 전송합니다.
func (p *Peer) SendProposal(proposal []byte) error {
	return p2p.Send(p.rw, ProposalMsg, proposal)
}

// SendEvidence는 피어에게 증거를 전송합니다.
func (p *Peer) SendEvidence(evidence []byte) error {
	return p2p.Send(p.rw, EvidenceMsg, evidence)
}

// PeerInfo는 피어 정보를 나타냅니다.
type PeerInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Version   uint      `json:"version"`
	Head      string    `json:"head"`
	TD        *big.Int  `json:"td"`
}

// Info는 피어 정보를 반환합니다.
func (p *Peer) Info() *PeerInfo {
	p.lock.RLock()
	defer p.lock.RUnlock()
	
	return &PeerInfo{
		ID:        p.id,
		Name:      p.peer.Name(),
		Version:   p.version,
		Head:      p.head.Hex(),
		TD:        new(big.Int).Set(p.td),
	}
}

// PeerSet는 활성 피어 집합을 나타냅니다.
type PeerSet struct {
	peers  map[string]*Peer
	lock   sync.RWMutex
}

// NewPeerSet는 새로운 피어 집합을 생성합니다.
func NewPeerSet() *PeerSet {
	return &PeerSet{
		peers: make(map[string]*Peer),
	}
}

// Register는 피어를 등록합니다.
func (ps *PeerSet) Register(p *Peer) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	
	if _, ok := ps.peers[p.id]; ok {
		return errors.New("피어가 이미 등록됨")
	}
	
	ps.peers[p.id] = p
	return nil
}

// Unregister는 피어 등록을 해제합니다.
func (ps *PeerSet) Unregister(id string) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	
	if _, ok := ps.peers[id]; !ok {
		return errors.New("피어가 등록되지 않음")
	}
	
	delete(ps.peers, id)
	return nil
}

// Peer는 ID로 피어를 검색합니다.
func (ps *PeerSet) Peer(id string) *Peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	
	return ps.peers[id]
}

// Len은 피어 수를 반환합니다.
func (ps *PeerSet) Len() int {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	
	return len(ps.peers)
}

// PeersWithoutTx는 트랜잭션을 알지 못하는 피어 목록을 반환합니다.
func (ps *PeerSet) PeersWithoutTx(hash common.Hash) []*Peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	
	var peers []*Peer
	for _, p := range ps.peers {
		if !p.KnownTransaction(hash) {
			peers = append(peers, p)
		}
	}
	
	return peers
}

// PeersWithoutBlock은 블록을 알지 못하는 피어 목록을 반환합니다.
func (ps *PeerSet) PeersWithoutBlock(hash common.Hash) []*Peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	
	var peers []*Peer
	for _, p := range ps.peers {
		if !p.KnownBlock(hash) {
			peers = append(peers, p)
		}
	}
	
	return peers
}

// BestPeer는 가장 높은 총 난이도를 가진 피어를 반환합니다.
func (ps *PeerSet) BestPeer() *Peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	
	var bestPeer *Peer
	var bestTd *big.Int
	
	for _, p := range ps.peers {
		if bestPeer == nil || p.td.Cmp(bestTd) > 0 {
			bestPeer = p
			bestTd = p.td
		}
	}
	
	return bestPeer
}

// AllPeers는 모든 피어 목록을 반환합니다.
func (ps *PeerSet) AllPeers() []*Peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	
	var peers []*Peer
	for _, p := range ps.peers {
		peers = append(peers, p)
	}
	
	return peers
}

// Close는 피어 집합을 닫습니다.
func (ps *PeerSet) Close() {
	ps.lock.Lock()
	defer ps.lock.Unlock()
	
	for _, p := range ps.peers {
		p.peer.Disconnect(p2p.DiscQuitting)
	}
	ps.peers = make(map[string]*Peer)
}

// RemoteAddr는 피어의 원격 주소를 반환합니다.
func (p *Peer) RemoteAddr() net.Addr {
	return p.peer.RemoteAddr()
}

// ConnectedTime은 피어의 연결 시간을 반환합니다.
func (p *Peer) ConnectedTime() time.Time {
	// 현재는 단순히 현재 시간을 반환합니다.
	// 실제 구현에서는 피어가 연결된 시간을 저장하고 반환해야 합니다.
	return time.Now()
}

// Latency는 피어의 지연 시간을 반환합니다.
func (p *Peer) Latency() time.Duration {
	// 현재는 단순히 기본값을 반환합니다.
	// 실제 구현에서는 피어의 실제 지연 시간을 측정하고 반환해야 합니다.
	return 100 * time.Millisecond
}

// IsInbound는 피어가 인바운드 연결인지 여부를 반환합니다.
func (p *Peer) IsInbound() bool {
	// 현재는 단순히 false를 반환합니다.
	// 실제 구현에서는 피어의 연결 방향을 확인하고 반환해야 합니다.
	return false
} 