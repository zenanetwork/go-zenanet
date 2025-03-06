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

package integration

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/governance"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core/rawdb"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/log"
)

// MockNetworkNode는 네트워크 파티션 테스트를 위한 노드를 나타냅니다.
type MockNetworkNode struct {
	ID            int
	ValidatorAddr common.Address
	GovAPI        *governance.API
	StateDB       *state.StateDB
	DB            ethdb.Database
	IsPartitioned bool
	Peers         []*MockNetworkNode
}

// NewMockNetworkNode는 새로운 모의 네트워크 노드를 생성합니다.
func NewMockNetworkNode(id int, t *testing.T) *MockNetworkNode {
	// 로거 설정
	log.New("module", fmt.Sprintf("node_%d", id))
	
	// 메모리 데이터베이스 생성
	db := rawdb.NewMemoryDatabase()
	
	// 상태 DB 생성
	stateDB, _ := state.New(common.Hash{}, state.NewDatabaseForTesting())
	
	// 검증자 주소 생성
	validatorAddr := common.BytesToAddress([]byte(fmt.Sprintf("validator%d", id)))
	
	// 검증자 집합 생성
	validatorSet := NewMockValidatorSet()
	
	// 자신을 검증자로 추가
	validatorSet.AddValidator(&MockValidator{
		address:    validatorAddr,
		votingPower: big.NewInt(100),
		status:     utils.ValidatorStatusBonded,
	})
	
	// 거버넌스 매개변수 생성
	govParams := governance.NewDefaultGovernanceParams()
	
	// 거버넌스 매니저 생성
	govManager := governance.NewGovernanceManager(govParams, validatorSet)
	
	// 현재 블록 헤더 생성
	header := &types.Header{
		Number:     big.NewInt(100),
		Time:       uint64(time.Now().Unix()),
		Difficulty: big.NewInt(1),
		GasLimit:   8000000,
	}
	
	// 체인 리더 생성
	chainReader := &MockChainReader{
		currentHeader: header,
		currentBlock:  types.NewBlockWithHeader(header),
	}
	
	// 거버넌스 API 생성
	govAPI := governance.NewAPI(
		chainReader,
		govManager,
		func(hash common.Hash) (*state.StateDB, error) { return stateDB, nil },
		func() *types.Block { return chainReader.currentBlock },
	)
	
	return &MockNetworkNode{
		ID:            id,
		ValidatorAddr: validatorAddr,
		GovAPI:        govAPI,
		StateDB:       stateDB,
		DB:            db,
		IsPartitioned: false,
		Peers:         make([]*MockNetworkNode, 0),
	}
}

// AddPeer는 노드에 피어를 추가합니다.
func (n *MockNetworkNode) AddPeer(peer *MockNetworkNode) {
	// 이미 피어인지 확인
	for _, p := range n.Peers {
		if p.ID == peer.ID {
			return
		}
	}
	
	// 피어 추가
	n.Peers = append(n.Peers, peer)
	
	// 양방향 연결 (상대방도 나를 피어로 추가)
	peer.AddPeer(n)
}

// RemovePeer는 노드에서 피어를 제거합니다.
func (n *MockNetworkNode) RemovePeer(peerID int) {
	for i, p := range n.Peers {
		if p.ID == peerID {
			// 피어 제거
			n.Peers = append(n.Peers[:i], n.Peers[i+1:]...)
			return
		}
	}
}

// BroadcastProposal은 제안을 모든 피어에게 브로드캐스트합니다.
func (n *MockNetworkNode) BroadcastProposal(proposalID uint64) {
	// 파티션된 상태라면 브로드캐스트하지 않음
	if n.IsPartitioned {
		return
	}
	
	// 제안 조회
	proposal, err := n.GovAPI.GetProposal(proposalID)
	if err != nil {
		return
	}
	
	// 모든 피어에게 제안 전파
	for _, peer := range n.Peers {
		// 피어가 파티션된 상태라면 전파하지 않음
		if peer.IsPartitioned {
			continue
		}
		
		// 제안 내용 생성
		content := &TestProposalContent{
			ProposalType: proposal.Type,
			Params:       make(map[string]string),
		}
		
		// 제안 제출
		submitArgs := governance.SubmitProposalArgs{
			Type:           proposal.Type,
			Title:          proposal.Title,
			Description:    proposal.Description,
			Proposer:       proposal.Proposer,
			InitialDeposit: proposal.TotalDeposit,
			Content:        content,
		}
		
		// 피어에게 제안 전파
		peer.GovAPI.SubmitProposal(submitArgs)
	}
}

// BroadcastVote는 투표를 모든 피어에게 브로드캐스트합니다.
func (n *MockNetworkNode) BroadcastVote(proposalID uint64, option string) {
	// 파티션된 상태라면 브로드캐스트하지 않음
	if n.IsPartitioned {
		return
	}
	
	// 모든 피어에게 투표 전파
	for _, peer := range n.Peers {
		// 피어가 파티션된 상태라면 전파하지 않음
		if peer.IsPartitioned {
			continue
		}
		
		// 투표 제출
		voteArgs := governance.VoteArgs{
			ProposalID: proposalID,
			Voter:      n.ValidatorAddr,
			Option:     option,
		}
		
		// 피어에게 투표 전파
		peer.GovAPI.Vote(voteArgs)
	}
}

// SyncProposalState는 노드 간 제안 상태 동기화를 위한 메서드입니다.
func (n *MockNetworkNode) SyncProposalState(proposalID uint64) {
	// 현재 노드의 제안 상태 가져오기
	proposal, err := n.GovAPI.GetProposal(proposalID)
	if err != nil {
		return
	}
	
	// 피어 노드들에게 제안 상태 전파
	for _, peer := range n.Peers {
		if !peer.IsPartitioned {
			// 피어 노드의 제안 상태 업데이트
			peerProposal, err := peer.GovAPI.GetProposal(proposalID)
			if err != nil {
				continue
			}
			
			// 상태가 다른 경우에만 업데이트
			if peerProposal.Status != proposal.Status {
				// 피어 노드의 거버넌스 매니저 가져오기
				govManager := peer.GovAPI.GetGovernanceManager()
				
				// 제안 상태 업데이트
				prop, _ := govManager.GetProposal(proposalID)
				if prop != nil {
					prop.Status = proposal.Status
				}
			}
		}
	}
}

// setupNetworkPartitionTest는 네트워크 파티션 테스트를 위한 환경을 설정합니다.
func setupNetworkPartitionTest(t *testing.T, nodeCount int) []*MockNetworkNode {
	// 노드 생성
	nodes := make([]*MockNetworkNode, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = NewMockNetworkNode(i, t)
	}
	
	// 노드 간 연결 (완전 연결 그래프)
	for i := 0; i < nodeCount; i++ {
		for j := i + 1; j < nodeCount; j++ {
			nodes[i].AddPeer(nodes[j])
		}
	}
	
	return nodes
}

// createNetworkPartition은 네트워크를 두 파티션으로 분할합니다.
func createNetworkPartition(nodes []*MockNetworkNode, partitionRatio float64) {
	nodeCount := len(nodes)
	partitionSize := int(float64(nodeCount) * partitionRatio)
	
	// 첫 번째 파티션의 노드들을 파티션 상태로 설정
	for i := 0; i < partitionSize; i++ {
		nodes[i].IsPartitioned = true
	}
	
	// 파티션 간 연결 제거
	for i := 0; i < partitionSize; i++ {
		for j := partitionSize; j < nodeCount; j++ {
			nodes[i].RemovePeer(nodes[j].ID)
			nodes[j].RemovePeer(nodes[i].ID)
		}
	}
}

// healNetworkPartition은 네트워크 파티션을 복구합니다.
func healNetworkPartition(nodes []*MockNetworkNode) {
	// 모든 노드의 파티션 상태 해제
	for _, node := range nodes {
		node.IsPartitioned = false
	}
	
	// 모든 노드 간 연결 복구
	for i := 0; i < len(nodes); i++ {
		for j := i + 1; j < len(nodes); j++ {
			nodes[i].AddPeer(nodes[j])
		}
	}
}

// TestNetworkPartitionRecovery는 네트워크 파티션 발생 후 복구 과정을 테스트합니다.
// 이 테스트는 네트워크가 분할된 상황에서 각 파티션이 독립적으로 동작하고,
// 파티션이 복구된 후 상태가 올바르게 동기화되는지 검증합니다.
func TestNetworkPartitionRecovery(t *testing.T) {
	// 노드 수
	nodeCount := 10
	
	// 테스트 환경 설정
	nodes := setupNetworkPartitionTest(t, nodeCount)
	
	// 1. 초기 상태: 모든 노드가 연결된 상태
	t.Log("1. 초기 상태: 모든 노드가 연결된 상태")
	
	// 첫 번째 노드에서 제안 생성
	proposer := nodes[0]
	
	// 제안 내용 생성
	content := &TestProposalContent{
		ProposalType: utils.ProposalTypeText,
		Params:       make(map[string]string),
	}
	
	// 제안 제출
	submitArgs := governance.SubmitProposalArgs{
		Type:           utils.ProposalTypeText,
		Title:          "네트워크 파티션 테스트 제안",
		Description:    "이 제안은 네트워크 파티션 테스트를 위한 제안입니다.",
		Proposer:       proposer.ValidatorAddr,
		InitialDeposit: "100",
		Content:        content,
	}
	
	proposalID, err := proposer.GovAPI.SubmitProposal(submitArgs)
	assert.NoError(t, err, "제안 제출 중 오류 발생")
	
	// 제안 브로드캐스트
	proposer.BroadcastProposal(proposalID)
	
	// 모든 노드에서 제안이 존재하는지 확인
	for i, node := range nodes {
		proposal, err := node.GovAPI.GetProposal(proposalID)
		assert.NoError(t, err, "노드 %d에서 제안 조회 중 오류 발생", i)
		assert.NotNil(t, proposal, "노드 %d에서 제안이 존재하지 않음", i)
		assert.Equal(t, "네트워크 파티션 테스트 제안", proposal.Title, "노드 %d의 제안 제목이 일치하지 않음", i)
	}
	
	// 2. 네트워크 파티션 발생
	t.Log("2. 네트워크 파티션 발생")
	
	// 네트워크를 60:40 비율로 분할
	createNetworkPartition(nodes, 0.6)
	
	// 파티션 A (0~5번 노드)에서 제안에 대한 투표
	for i := 0; i < 6; i++ {
		voteArgs := governance.VoteArgs{
			ProposalID: proposalID,
			Voter:      nodes[i].ValidatorAddr,
			Option:     utils.VoteOptionYes,
		}
		
		_, err := nodes[i].GovAPI.Vote(voteArgs)
		assert.NoError(t, err, "노드 %d에서 투표 중 오류 발생", i)
		
		// 투표 브로드캐스트 (파티션 내에서만 전파됨)
		nodes[i].BroadcastVote(proposalID, utils.VoteOptionYes)
	}
	
	// 파티션 B (6~9번 노드)에서 제안에 대한 투표
	for i := 6; i < nodeCount; i++ {
		voteArgs := governance.VoteArgs{
			ProposalID: proposalID,
			Voter:      nodes[i].ValidatorAddr,
			Option:     utils.VoteOptionNo,
		}
		
		_, err := nodes[i].GovAPI.Vote(voteArgs)
		assert.NoError(t, err, "노드 %d에서 투표 중 오류 발생", i)
		
		// 투표 브로드캐스트 (파티션 내에서만 전파됨)
		nodes[i].BroadcastVote(proposalID, utils.VoteOptionNo)
	}
	
	// 파티션 A의 투표 상태 확인
	partitionANode := nodes[0]
	proposalA, _ := partitionANode.GovAPI.GetProposal(proposalID)
	
	// 파티션 B의 투표 상태 확인
	partitionBNode := nodes[6]
	proposalB, _ := partitionBNode.GovAPI.GetProposal(proposalID)
	
	// 각 파티션의 투표 상태 출력
	t.Logf("파티션 A의 투표 상태: 찬성=%s, 반대=%s", proposalA.YesVotes, proposalA.NoVotes)
	t.Logf("파티션 B의 투표 상태: 찬성=%s, 반대=%s", proposalB.YesVotes, proposalB.NoVotes)
	
	// 3. 네트워크 파티션 복구
	t.Log("3. 네트워크 파티션 복구")
	
	// 파티션 복구
	healNetworkPartition(nodes)
	
	// 파티션 A의 노드에서 투표 상태 동기화
	for i := 0; i < 6; i++ {
		for j := 6; j < nodeCount; j++ {
			// 파티션 B의 노드들의 투표를 파티션 A의 노드들에게 전파
			voteArgs := governance.VoteArgs{
				ProposalID: proposalID,
				Voter:      nodes[j].ValidatorAddr,
				Option:     utils.VoteOptionNo,
			}
			
			nodes[i].GovAPI.Vote(voteArgs)
		}
	}
	
	// 파티션 B의 노드에서 투표 상태 동기화
	for i := 6; i < nodeCount; i++ {
		for j := 0; j < 6; j++ {
			// 파티션 A의 노드들의 투표를 파티션 B의 노드들에게 전파
			voteArgs := governance.VoteArgs{
				ProposalID: proposalID,
				Voter:      nodes[j].ValidatorAddr,
				Option:     utils.VoteOptionYes,
			}
			
			nodes[i].GovAPI.Vote(voteArgs)
		}
	}
	
	// 4. 동기화 후 상태 확인
	t.Log("4. 동기화 후 상태 확인")
	
	// 모든 노드의 투표 상태가 동일한지 확인
	for i := 1; i < nodeCount; i++ {
		proposal0, _ := nodes[0].GovAPI.GetProposal(proposalID)
		proposalI, _ := nodes[i].GovAPI.GetProposal(proposalID)
		
		assert.Equal(t, proposal0.YesVotes, proposalI.YesVotes, "노드 0과 노드 %d의 찬성 투표 수가 일치하지 않음", i)
		assert.Equal(t, proposal0.NoVotes, proposalI.NoVotes, "노드 0과 노드 %d의 반대 투표 수가 일치하지 않음", i)
	}
	
	// 최종 투표 상태 출력
	finalProposal, _ := nodes[0].GovAPI.GetProposal(proposalID)
	t.Logf("최종 투표 상태: 찬성=%s, 반대=%s", finalProposal.YesVotes, finalProposal.NoVotes)
	
	// 예상 결과: 모든 노드가 모든 투표를 받아야 함
	// 찬성 투표: 6개 (파티션 A의 노드들)
	// 반대 투표: 4개 (파티션 B의 노드들)
	assert.Equal(t, "6", finalProposal.YesVotes, "찬성 투표 수가 예상과 다름")
	assert.Equal(t, "4", finalProposal.NoVotes, "반대 투표 수가 예상과 다름")
}

// TestNetworkPartitionConsensus는 네트워크 파티션 상황에서의 합의 과정을 테스트합니다.
// 이 테스트는 네트워크가 분할된 상황에서 각 파티션이 독립적으로 합의를 진행하고,
// 파티션이 복구된 후 합의가 올바르게 이루어지는지 검증합니다.
func TestNetworkPartitionConsensus(t *testing.T) {
	// 테스트 시간이 길어 일반 테스트에서는 건너뛰기
	if testing.Short() {
		t.Skip("네트워크 파티션 합의 테스트는 -short 플래그가 없을 때만 실행됩니다")
	}
	
	// 노드 수
	nodeCount := 10
	
	// 테스트 노드 생성
	nodes := make([]*MockNetworkNode, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = NewMockNetworkNode(i, t)
	}
	
	// 노드 간 연결 설정
	for i := 0; i < nodeCount; i++ {
		for j := 0; j < nodeCount; j++ {
			if i != j {
				nodes[i].Peers = append(nodes[i].Peers, nodes[j])
			}
		}
	}
	
	t.Log("네트워크 파티션 합의 테스트 시작")
	
	// 이 테스트는 실제 구현에 따라 달라질 수 있으므로 기본 구조만 제공
	// 실제 테스트 구현은 합의 알고리즘의 세부 사항에 따라 작성해야 함
} 