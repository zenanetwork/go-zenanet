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
	"sync"
	"sync/atomic"
	"time"

	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/core"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/event"
	"github.com/zenanetwork/go-zenanet/p2p"
	"github.com/zenanetwork/go-zenanet/p2p/enode"
)

// Handler는 Eirene 프로토콜의 P2P 메시지 핸들러입니다.
type Handler struct {
	eirene      *core.Eirene       // Eirene 합의 엔진
	networkID   uint64             // 네트워크 ID
	
	peerSet     *PeerSet           // 활성 피어 집합
	maxPeers    int                // 최대 피어 수
	
	txPool      core.TxPool        // 트랜잭션 풀
	blockchain  core.BlockChain    // 블록체인
	
	txsCh       chan core.NewTxsEvent         // 새 트랜잭션 이벤트 채널
	txsSub      event.Subscription            // 트랜잭션 이벤트 구독
	
	minedBlockSub event.Subscription          // 채굴된 블록 구독
	minedBlockCh  chan core.NewMinedBlockEvent // 채굴된 블록 이벤트 채널
	
	// 브로드캐스트 관련
	txsyncCh    chan *txsync      // 트랜잭션 동기화 채널
	queuedTxs   chan []*types.Transaction // 대기 중인 트랜잭션 채널
	queuedBlocks chan *types.Block        // 대기 중인 블록 채널
	
	wg          sync.WaitGroup    // 대기 그룹
	quit        chan struct{}     // 종료 채널
	
	started     int32             // 시작 여부 (atomic)
}

// NewHandler는 새로운 Eirene 프로토콜 핸들러를 생성합니다.
func NewHandler(eirene *core.Eirene, networkID uint64, txpool core.TxPool, blockchain core.BlockChain) *Handler {
	handler := &Handler{
		eirene:      eirene,
		networkID:   networkID,
		peerSet:     NewPeerSet(),
		maxPeers:    maxPeers,
		txPool:      txpool,
		blockchain:  blockchain,
		txsyncCh:    make(chan *txsync),
		queuedTxs:   make(chan []*types.Transaction, maxQueuedTxs),
		queuedBlocks: make(chan *types.Block, maxQueuedBlocks),
		quit:        make(chan struct{}),
	}
	
	return handler
}

// Start는 핸들러를 시작합니다.
func (h *Handler) Start() {
	if atomic.AddInt32(&h.started, 1) != 1 {
		return
	}
	
	h.wg.Add(1)
	go h.loop()
	
	// 트랜잭션 이벤트 구독
	h.txsCh = make(chan core.NewTxsEvent, maxQueuedTxs)
	h.txsSub = h.txPool.SubscribeNewTxsEvent(h.txsCh)
	
	// 채굴된 블록 이벤트 구독
	h.minedBlockCh = make(chan core.NewMinedBlockEvent, maxQueuedBlocks)
	h.minedBlockSub = new(event.Feed).Subscribe(h.minedBlockCh)
	
	// 브로드캐스트 고루틴 시작
	for i := 0; i < txConcurrency; i++ {
		h.wg.Add(1)
		go h.txBroadcastLoop()
	}
	
	for i := 0; i < blockConcurrency; i++ {
		h.wg.Add(1)
		go h.blockBroadcastLoop()
	}
	
	// 트랜잭션 동기화 고루틴 시작
	h.wg.Add(1)
	go h.txsyncLoop()
	
	logger.Info("Eirene P2P 핸들러 시작됨", "networkID", h.networkID)
}

// Stop은 핸들러를 중지합니다.
func (h *Handler) Stop() {
	if atomic.LoadInt32(&h.started) == 0 {
		return
	}
	
	// 구독 해제
	h.txsSub.Unsubscribe()
	h.minedBlockSub.Unsubscribe()
	
	// 종료 채널 닫기
	close(h.quit)
	
	// 피어 집합 닫기
	h.peerSet.Close()
	
	// 대기 그룹 대기
	h.wg.Wait()
	
	logger.Info("Eirene P2P 핸들러 중지됨")
}

// loop는 핸들러의 메인 루프입니다.
func (h *Handler) loop() {
	defer h.wg.Done()
	
	for {
		select {
		case ev := <-h.txsCh:
			// 새 트랜잭션 이벤트 처리
			h.BroadcastTxs(ev.Txs)
			
		case ev := <-h.minedBlockCh:
			// 채굴된 블록 이벤트 처리
			h.BroadcastBlock(ev.Block, true)
			
		case <-h.quit:
			return
		}
	}
}

// txBroadcastLoop는 트랜잭션 브로드캐스트 루프입니다.
func (h *Handler) txBroadcastLoop() {
	defer h.wg.Done()
	
	for {
		select {
		case txs := <-h.queuedTxs:
			// 트랜잭션 브로드캐스트
			peers := h.peerSet.PeersWithoutTx(txs[0].Hash())
			for _, peer := range peers {
				peer.SendNewTx(txs)
			}
			
		case <-h.quit:
			return
		}
	}
}

// blockBroadcastLoop는 블록 브로드캐스트 루프입니다.
func (h *Handler) blockBroadcastLoop() {
	defer h.wg.Done()
	
	for {
		select {
		case block := <-h.queuedBlocks:
			// 블록 브로드캐스트
			hash := block.Hash()
			peers := h.peerSet.PeersWithoutBlock(hash)
			
			// 총 난이도 계산
			td := h.blockchain.GetTd(hash, block.NumberU64())
			if td == nil {
				logger.Error("블록의 총 난이도를 가져올 수 없음", "hash", hash)
				continue
			}
			
			// 피어에게 블록 전송
			for _, peer := range peers {
				peer.SendNewBlock(block, td)
			}
			
		case <-h.quit:
			return
		}
	}
}

// txsync는 트랜잭션 동기화 요청을 나타냅니다.
type txsync struct {
	p    *Peer
	txs  []*types.Transaction
}

// txsyncLoop는 트랜잭션 동기화 루프입니다.
func (h *Handler) txsyncLoop() {
	defer h.wg.Done()
	
	var (
		pending = make(map[enode.ID]*txsync)
		sending = false
		done    = make(chan struct{})
	)
	
	// 트랜잭션 전송 함수
	send := func(s *txsync) {
		if s.p.version < ProtocolVersion {
			return
		}
		
		if len(s.txs) == 0 {
			return
		}
		
		// 트랜잭션 전송
		if err := s.p.SendNewTx(s.txs); err != nil {
			logger.Error("트랜잭션 전송 실패", "peer", s.p.ID(), "err", err)
		}
	}
	
	for {
		select {
		case s := <-h.txsyncCh:
			// 새 트랜잭션 동기화 요청
			pending[s.p.ID()] = s
			if !sending {
				send(s)
				sending = true
				go func() {
					time.Sleep(50 * time.Millisecond)
					done <- struct{}{}
				}()
			}
			
		case <-done:
			// 전송 완료
			sending = false
			if len(pending) > 0 {
				for _, s := range pending {
					send(s)
					delete(pending, s.p.ID())
					break
				}
				sending = true
				go func() {
					time.Sleep(50 * time.Millisecond)
					done <- struct{}{}
				}()
			}
			
		case <-h.quit:
			return
		}
	}
}

// BroadcastTxs는 트랜잭션을 브로드캐스트합니다.
func (h *Handler) BroadcastTxs(txs []*types.Transaction) {
	if len(txs) == 0 {
		return
	}
	
	// 트랜잭션 큐에 추가
	select {
	case h.queuedTxs <- txs:
		// 성공
	default:
		logger.Debug("트랜잭션 브로드캐스트 큐가 가득 참")
	}
}

// BroadcastBlock은 블록을 브로드캐스트합니다.
func (h *Handler) BroadcastBlock(block *types.Block, propagate bool) {
	hash := block.Hash()
	peers := h.peerSet.PeersWithoutBlock(hash)
	
	// 블록 큐에 추가
	if propagate {
		select {
		case h.queuedBlocks <- block:
			// 성공
		default:
			logger.Debug("블록 브로드캐스트 큐가 가득 참")
		}
	}
	
	// 최고 난이도 피어에게 즉시 전송
	if len(peers) > 0 {
		bestPeer := peers[0]
		for i := 1; i < len(peers); i++ {
			if peers[i].td.Cmp(bestPeer.td) > 0 {
				bestPeer = peers[i]
			}
		}
		
		// 총 난이도 계산
		td := h.blockchain.GetTd(hash, block.NumberU64())
		if td == nil {
			logger.Error("블록의 총 난이도를 가져올 수 없음", "hash", hash)
			return
		}
		
		// 최고 난이도 피어에게 블록 전송
		bestPeer.SendNewBlock(block, td)
	}
}

// Protocol은 Eirene 프로토콜 정보를 반환합니다.
func (h *Handler) Protocol() p2p.Protocol {
	return p2p.Protocol{
		Name:    ProtocolName,
		Version: ProtocolVersion,
		Length:  16, // 메시지 코드 수
		Run:     h.handle,
	}
}

// handle은 피어 연결을 처리합니다.
func (h *Handler) handle(peer *p2p.Peer, rw p2p.MsgReadWriter) error {
	// 피어 생성
	p := NewPeer(ProtocolVersion, peer, rw)
	
	// 핸드셰이크 수행
	currentBlock := h.blockchain.CurrentBlock()
	td := h.blockchain.GetTd(currentBlock.Hash(), currentBlock.NumberU64())
	if td == nil {
		return errors.New("현재 블록의 총 난이도를 가져올 수 없음")
	}
	
	if err := p.Handshake(currentBlock.Hash(), td); err != nil {
		return err
	}
	
	// 피어 등록
	if err := h.peerSet.Register(p); err != nil {
		return err
	}
	defer h.peerSet.Unregister(p.ID().String())
	
	// 트랜잭션 동기화 요청
	go h.syncTransactions(p)
	
	// 메시지 처리 루프
	for {
		if err := h.handleMsg(p); err != nil {
			return err
		}
	}
}

// handleMsg는 피어로부터 수신한 메시지를 처리합니다.
func (h *Handler) handleMsg(p *Peer) error {
	// 메시지 읽기
	msg, err := p.rw.ReadMsg()
	if err != nil {
		return err
	}
	if msg.Size > MaxMessageSize {
		return fmt.Errorf("메시지가 너무 큼: %v > %v", msg.Size, MaxMessageSize)
	}
	defer msg.Discard()
	
	// 메시지 코드에 따라 처리
	switch msg.Code {
	case StatusMsg:
		// 이미 핸드셰이크를 수행했으므로 무시
		return errors.New("중복된 상태 메시지")
		
	case NewBlockMsg:
		// 새 블록 메시지 처리
		var request struct {
			Block *types.Block
			TD    *big.Int
		}
		if err := msg.Decode(&request); err != nil {
			return fmt.Errorf("새 블록 메시지 디코딩 실패: %v", err)
		}
		
		// 블록 해시 추가
		p.knownBlocks.Add(request.Block.Hash())
		
		// 블록 처리
		n, err := h.blockchain.InsertChain([]*types.Block{request.Block})
		if err != nil {
			logger.Error("블록 삽입 실패", "err", err)
		} else if n == 0 {
			logger.Debug("블록 이미 알고 있음", "hash", request.Block.Hash())
		}
		
		// 블록 브로드캐스트
		h.BroadcastBlock(request.Block, true)
		
	case NewTxMsg:
		// 새 트랜잭션 메시지 처리
		var txs []*types.Transaction
		if err := msg.Decode(&txs); err != nil {
			return fmt.Errorf("새 트랜잭션 메시지 디코딩 실패: %v", err)
		}
		
		// 트랜잭션 해시 추가
		for _, tx := range txs {
			p.knownTxs.Add(tx.Hash())
		}
		
		// 트랜잭션 풀에 추가
		h.txPool.AddRemotes(txs)
		
	case ValidatorSetMsg:
		// 검증자 집합 메시지 처리
		var validators []common.Address
		if err := msg.Decode(&validators); err != nil {
			return fmt.Errorf("검증자 집합 메시지 디코딩 실패: %v", err)
		}
		
		// 검증자 집합 업데이트
		// TODO: 검증자 집합 업데이트 로직 구현
		
	case VoteMsg:
		// 투표 메시지 처리
		var vote []byte
		if err := msg.Decode(&vote); err != nil {
			return fmt.Errorf("투표 메시지 디코딩 실패: %v", err)
		}
		
		// 투표 처리
		// TODO: 투표 처리 로직 구현
		
	case ProposalMsg:
		// 제안 메시지 처리
		var proposal []byte
		if err := msg.Decode(&proposal); err != nil {
			return fmt.Errorf("제안 메시지 디코딩 실패: %v", err)
		}
		
		// 제안 처리
		// TODO: 제안 처리 로직 구현
		
	case EvidenceMsg:
		// 증거 메시지 처리
		var evidence []byte
		if err := msg.Decode(&evidence); err != nil {
			return fmt.Errorf("증거 메시지 디코딩 실패: %v", err)
		}
		
		// 증거 처리
		// TODO: 증거 처리 로직 구현
		
	case PingMsg:
		// 핑 메시지 처리
		return p2p.Send(p.rw, PongMsg, nil)
		
	case PongMsg:
		// 퐁 메시지 처리 (무시)
		
	default:
		return fmt.Errorf("알 수 없는 메시지 코드: %v", msg.Code)
	}
	
	return nil
}

// syncTransactions는 피어와 트랜잭션을 동기화합니다.
func (h *Handler) syncTransactions(p *Peer) {
	// 트랜잭션 풀에서 트랜잭션 가져오기
	txs, err := h.txPool.Pending(false)
	if err != nil || len(txs) == 0 {
		return
	}
	
	// 트랜잭션 목록 생성
	var txList []*types.Transaction
	for _, batch := range txs {
		txList = append(txList, batch...)
	}
	
	// 트랜잭션 동기화 요청
	select {
	case h.txsyncCh <- &txsync{p: p, txs: txList}:
		// 성공
	case <-h.quit:
		return
	}
} 