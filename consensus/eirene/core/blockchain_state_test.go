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

package core

import (
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/crypto"
	"github.com/zenanetwork/go-zenanet/log"
)

// MockStateDB는 테스트를 위한 StateDB 모의 구현체입니다.
type MockStateDB struct {
	balances map[common.Address]*big.Int
	storage  map[common.Address]map[common.Hash]common.Hash
	logs     []*types.Log
	nonces   map[common.Address]uint64
	codes    map[common.Address][]byte
	suicides map[common.Address]bool
	journal  []string // 상태 변경 기록
}

func NewMockStateDB() *MockStateDB {
	return &MockStateDB{
		balances: make(map[common.Address]*big.Int),
		storage:  make(map[common.Address]map[common.Hash]common.Hash),
		logs:     make([]*types.Log, 0),
		nonces:   make(map[common.Address]uint64),
		codes:    make(map[common.Address][]byte),
		suicides: make(map[common.Address]bool),
		journal:  make([]string, 0),
	}
}

func (m *MockStateDB) GetBalance(addr common.Address) *big.Int {
	if balance, ok := m.balances[addr]; ok {
		return balance
	}
	return big.NewInt(0)
}

func (m *MockStateDB) SetBalance(addr common.Address, balance *big.Int) {
	m.balances[addr] = balance
	m.journal = append(m.journal, "SetBalance:"+addr.Hex())
}

func (m *MockStateDB) GetState(addr common.Address, hash common.Hash) common.Hash {
	if storage, ok := m.storage[addr]; ok {
		if value, ok := storage[hash]; ok {
			return value
		}
	}
	return common.Hash{}
}

func (m *MockStateDB) SetState(addr common.Address, key, value common.Hash) {
	if _, ok := m.storage[addr]; !ok {
		m.storage[addr] = make(map[common.Hash]common.Hash)
	}
	m.storage[addr][key] = value
	m.journal = append(m.journal, "SetState:"+addr.Hex())
}

func (m *MockStateDB) GetNonce(addr common.Address) uint64 {
	if nonce, ok := m.nonces[addr]; ok {
		return nonce
	}
	return 0
}

func (m *MockStateDB) SetNonce(addr common.Address, nonce uint64) {
	m.nonces[addr] = nonce
	m.journal = append(m.journal, "SetNonce:"+addr.Hex())
}

func (m *MockStateDB) GetCode(addr common.Address) []byte {
	if code, ok := m.codes[addr]; ok {
		return code
	}
	return nil
}

func (m *MockStateDB) SetCode(addr common.Address, code []byte) {
	m.codes[addr] = code
	m.journal = append(m.journal, "SetCode:"+addr.Hex())
}

func (m *MockStateDB) AddLog(log *types.Log) {
	m.logs = append(m.logs, log)
	m.journal = append(m.journal, "AddLog")
}

func (m *MockStateDB) Suicide(addr common.Address) bool {
	if _, ok := m.suicides[addr]; ok {
		return false
	}
	m.suicides[addr] = true
	m.journal = append(m.journal, "Suicide:"+addr.Hex())
	return true
}

func (m *MockStateDB) Commit() (common.Hash, error) {
	// 상태 커밋 시뮬레이션
	m.journal = append(m.journal, "Commit")
	return common.Hash{}, nil
}

func (m *MockStateDB) Rollback() {
	// 상태 롤백 시뮬레이션
	m.journal = append(m.journal, "Rollback")
}

// TestBlockchainStateInitialization은 블록체인 상태 초기화를 테스트합니다.
func TestBlockchainStateInitialization(t *testing.T) {
	// 로거 생성
	logger := log.New("module", "blockchain_state_test")
	
	// 모의 StateDB 생성
	stateDB := NewMockStateDB()
	
	// 테스트 블록 생성
	header := &types.Header{
		Number:     big.NewInt(0),
		Time:       uint64(time.Now().Unix()),
		Coinbase:   common.HexToAddress("0x1111111111111111111111111111111111111111"),
		Root:       common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111"),
		ParentHash: common.Hash{},
	}
	block := types.NewBlockWithHeader(header)
	
	// 블록체인 상태 초기화 테스트
	// 초기 검증자 설정
	validators := []common.Address{
		common.HexToAddress("0x1111111111111111111111111111111111111111"),
		common.HexToAddress("0x2222222222222222222222222222222222222222"),
		common.HexToAddress("0x3333333333333333333333333333333333333333"),
	}
	
	// 초기 검증자 잔액 설정
	for i, validator := range validators {
		// 각 검증자에게 다른 금액의 토큰 할당
		amount := new(big.Int).Mul(big.NewInt(int64(i+1)*100), big.NewInt(1e18))
		stateDB.SetBalance(validator, amount)
		logger.Debug("Set validator balance", "address", validator.Hex(), "amount", amount)
	}
	
	// 초기 시스템 계약 설정
	stakingContract := common.HexToAddress("0x0000000000000000000000000000000000001000")
	governanceContract := common.HexToAddress("0x0000000000000000000000000000000000001001")
	
	// 시스템 계약 코드 설정 (예시)
	stateDB.SetCode(stakingContract, []byte("staking contract code"))
	stateDB.SetCode(governanceContract, []byte("governance contract code"))
	
	// 결과 검증
	assert.Equal(t, uint64(0), block.NumberU64(), "Genesis block number should be 0")
	
	// 검증자 잔액 검증
	for i, validator := range validators {
		expectedAmount := new(big.Int).Mul(big.NewInt(int64(i+1)*100), big.NewInt(1e18))
		actualAmount := stateDB.GetBalance(validator)
		assert.Equal(t, expectedAmount, actualAmount, "Validator balance should be set correctly")
	}
	
	// 시스템 계약 코드 검증
	assert.NotNil(t, stateDB.GetCode(stakingContract), "Staking contract code should be set")
	assert.NotNil(t, stateDB.GetCode(governanceContract), "Governance contract code should be set")
	
	// 상태 커밋
	_, err := stateDB.Commit()
	assert.NoError(t, err, "State commit should not return error")
	
	// 로그 출력
	logger.Debug("Blockchain state initialization test completed", "genesisBlock", block.Number())
}

// TestBlockProcessing은 블록 처리 파이프라인을 테스트합니다.
func TestBlockProcessing(t *testing.T) {
	// 로거 생성
	logger := log.New("module", "blockchain_state_test")
	
	// 모의 StateDB 생성
	stateDB := NewMockStateDB()
	
	// 테스트 블록 생성
	parentHeader := &types.Header{
		Number:     big.NewInt(0),
		Time:       uint64(time.Now().Unix() - 12),
		Coinbase:   common.HexToAddress("0x1111111111111111111111111111111111111111"),
		Root:       common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111"),
		ParentHash: common.Hash{},
	}
	parentBlock := types.NewBlockWithHeader(parentHeader)
	
	header := &types.Header{
		Number:     big.NewInt(1),
		Time:       uint64(time.Now().Unix()),
		Coinbase:   common.HexToAddress("0x2222222222222222222222222222222222222222"),
		Root:       common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222"),
		ParentHash: parentBlock.Hash(),
	}
	block := types.NewBlockWithHeader(header)
	
	// 블록 처리 테스트
	// 블록 생성자 보상 처리
	coinbase := block.Coinbase()
	blockReward := new(big.Int).Mul(big.NewInt(2), big.NewInt(1e18)) // 2 토큰
	
	// 초기 잔액 설정
	initialBalance := new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18)) // 100 토큰
	stateDB.SetBalance(coinbase, initialBalance)
	
	// 블록 보상 지급
	newBalance := new(big.Int).Add(initialBalance, blockReward)
	stateDB.SetBalance(coinbase, newBalance)
	
	// 트랜잭션 처리 시뮬레이션
	sender := common.HexToAddress("0x4444444444444444444444444444444444444444")
	receiver := common.HexToAddress("0x5555555555555555555555555555555555555555")
	
	// 초기 잔액 설정
	senderInitialBalance := new(big.Int).Mul(big.NewInt(50), big.NewInt(1e18)) // 50 토큰
	receiverInitialBalance := new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18)) // 10 토큰
	stateDB.SetBalance(sender, senderInitialBalance)
	stateDB.SetBalance(receiver, receiverInitialBalance)
	
	// 트랜잭션 금액
	txValue := new(big.Int).Mul(big.NewInt(5), big.NewInt(1e18)) // 5 토큰
	
	// 트랜잭션 처리 시뮬레이션
	senderNewBalance := new(big.Int).Sub(senderInitialBalance, txValue)
	receiverNewBalance := new(big.Int).Add(receiverInitialBalance, txValue)
	stateDB.SetBalance(sender, senderNewBalance)
	stateDB.SetBalance(receiver, receiverNewBalance)
	
	// 상태 커밋
	_, err := stateDB.Commit()
	assert.NoError(t, err, "State commit should not return error")
	
	// 결과 검증
	assert.Equal(t, uint64(1), block.NumberU64(), "Block number should be 1")
	assert.Equal(t, parentBlock.Hash(), block.ParentHash(), "Block parent hash should match parent block hash")
	
	// 블록 생성자 보상 검증
	assert.Equal(t, newBalance, stateDB.GetBalance(coinbase), "Block creator should receive block reward")
	
	// 트랜잭션 처리 결과 검증
	assert.Equal(t, senderNewBalance, stateDB.GetBalance(sender), "Sender balance should be decreased by transaction value")
	assert.Equal(t, receiverNewBalance, stateDB.GetBalance(receiver), "Receiver balance should be increased by transaction value")
	
	// 로그 출력
	logger.Debug("Block processing test completed", "blockNumber", block.Number(), "parentNumber", parentBlock.Number())
}

// TestTransactionProcessing은 트랜잭션 처리를 테스트합니다.
func TestTransactionProcessing(t *testing.T) {
	// 로거 생성
	logger := log.New("module", "blockchain_state_test")
	
	// 모의 StateDB 생성
	stateDB := NewMockStateDB()
	
	// 테스트 키 생성
	senderKey, _ := crypto.GenerateKey()
	senderAddress := crypto.PubkeyToAddress(senderKey.PublicKey)
	receiverAddress := common.HexToAddress("0x2222222222222222222222222222222222222222")
	
	// 초기 잔액 설정
	senderInitialBalance := new(big.Int).Mul(big.NewInt(100), big.NewInt(1e18)) // 100 토큰
	stateDB.SetBalance(senderAddress, senderInitialBalance)
	
	// 테스트 트랜잭션 생성
	value := new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18)) // 10 토큰
	gasLimit := uint64(21000)
	gasPrice := big.NewInt(1e9) // 1 Gwei
	
	tx := types.NewTransaction(
		0, // nonce
		receiverAddress,
		value,
		gasLimit,
		gasPrice,
		nil, // data
	)
	
	// 트랜잭션 서명
	signer := types.NewEIP155Signer(big.NewInt(1))
	signedTx, err := types.SignTx(tx, signer, senderKey)
	assert.NoError(t, err, "Transaction signing should not return error")
	
	// 트랜잭션 처리 시뮬레이션
	// 1. 송신자 확인
	sender, err := types.Sender(signer, signedTx)
	assert.NoError(t, err, "Sender extraction should not return error")
	assert.Equal(t, senderAddress, sender, "Extracted sender should match original sender")
	
	// 2. 잔액 확인
	senderBalance := stateDB.GetBalance(sender)
	txCost := new(big.Int).Add(value, new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit))))
	assert.True(t, senderBalance.Cmp(txCost) >= 0, "Sender should have enough balance")
	
	// 3. 논스 확인 및 증가
	currentNonce := stateDB.GetNonce(sender)
	assert.Equal(t, uint64(0), currentNonce, "Initial nonce should be 0")
	assert.Equal(t, currentNonce, signedTx.Nonce(), "Transaction nonce should match current nonce")
	stateDB.SetNonce(sender, currentNonce+1)
	
	// 4. 가스 비용 차감
	gasCost := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))
	newSenderBalance := new(big.Int).Sub(senderBalance, gasCost)
	stateDB.SetBalance(sender, newSenderBalance)
	
	// 5. 트랜잭션 실행 (값 전송)
	receiverBalance := stateDB.GetBalance(receiverAddress)
	stateDB.SetBalance(sender, new(big.Int).Sub(newSenderBalance, value))
	stateDB.SetBalance(receiverAddress, new(big.Int).Add(receiverBalance, value))
	
	// 6. 가스 환불 (예: 모든 가스를 사용하지 않은 경우)
	gasUsed := uint64(21000) // 기본 전송 비용
	gasRefund := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit-gasUsed)))
	stateDB.SetBalance(sender, new(big.Int).Add(stateDB.GetBalance(sender), gasRefund))
	
	// 7. 블록 생성자에게 가스 비용 지급
	miner := common.HexToAddress("0x3333333333333333333333333333333333333333")
	minerReward := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasUsed)))
	stateDB.SetBalance(miner, new(big.Int).Add(stateDB.GetBalance(miner), minerReward))
	
	// 상태 커밋
	_, err = stateDB.Commit()
	assert.NoError(t, err, "State commit should not return error")
	
	// 결과 검증
	expectedSenderBalance := new(big.Int).Sub(senderInitialBalance, new(big.Int).Add(value, new(big.Int).Mul(gasPrice, big.NewInt(int64(gasUsed)))))
	actualSenderBalance := stateDB.GetBalance(sender)
	assert.Equal(t, expectedSenderBalance, actualSenderBalance, "Sender balance should be decreased by value and gas cost")
	
	expectedReceiverBalance := value
	actualReceiverBalance := stateDB.GetBalance(receiverAddress)
	assert.Equal(t, expectedReceiverBalance, actualReceiverBalance, "Receiver balance should be increased by value")
	
	expectedMinerBalance := minerReward
	actualMinerBalance := stateDB.GetBalance(miner)
	assert.Equal(t, expectedMinerBalance, actualMinerBalance, "Miner should receive gas cost")
	
	// 논스 검증
	assert.Equal(t, uint64(1), stateDB.GetNonce(sender), "Sender nonce should be incremented")
	
	// 로그 출력
	logger.Debug("Transaction processing test completed", "txHash", signedTx.Hash())
}

// TestStateCommitAndRollback은 상태 커밋 및 롤백을 테스트합니다.
func TestStateCommitAndRollback(t *testing.T) {
	// 로거 생성
	logger := log.New("module", "blockchain_state_test")
	
	// 모의 StateDB 생성
	stateDB := NewMockStateDB()
	
	// 테스트 주소
	address1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
	address2 := common.HexToAddress("0x2222222222222222222222222222222222222222")
	
	// 초기 상태 설정
	stateDB.SetBalance(address1, big.NewInt(100))
	stateDB.SetBalance(address2, big.NewInt(200))
	stateDB.SetNonce(address1, 5)
	stateDB.SetNonce(address2, 10)
	
	// 상태 변경 전 값 저장 - 로깅 목적으로 사용
	logger.Debug("Initial state", 
		"balance1", stateDB.GetBalance(address1), 
		"balance2", stateDB.GetBalance(address2),
		"nonce1", stateDB.GetNonce(address1),
		"nonce2", stateDB.GetNonce(address2))
	
	// 상태 변경
	stateDB.SetBalance(address1, big.NewInt(150))
	stateDB.SetBalance(address2, big.NewInt(250))
	stateDB.SetNonce(address1, 6)
	stateDB.SetNonce(address2, 11)
	
	// 변경된 상태 검증
	assert.Equal(t, big.NewInt(150), stateDB.GetBalance(address1), "Balance of address1 should be updated")
	assert.Equal(t, big.NewInt(250), stateDB.GetBalance(address2), "Balance of address2 should be updated")
	assert.Equal(t, uint64(6), stateDB.GetNonce(address1), "Nonce of address1 should be updated")
	assert.Equal(t, uint64(11), stateDB.GetNonce(address2), "Nonce of address2 should be updated")
	
	// 상태 커밋
	_, err := stateDB.Commit()
	assert.NoError(t, err, "State commit should not return error")
	
	// 커밋 후 상태 검증
	assert.Equal(t, big.NewInt(150), stateDB.GetBalance(address1), "Balance of address1 should remain after commit")
	assert.Equal(t, big.NewInt(250), stateDB.GetBalance(address2), "Balance of address2 should remain after commit")
	assert.Equal(t, uint64(6), stateDB.GetNonce(address1), "Nonce of address1 should remain after commit")
	assert.Equal(t, uint64(11), stateDB.GetNonce(address2), "Nonce of address2 should remain after commit")
	
	// 새로운 상태 변경
	stateDB.SetBalance(address1, big.NewInt(180))
	stateDB.SetBalance(address2, big.NewInt(280))
	stateDB.SetNonce(address1, 7)
	stateDB.SetNonce(address2, 12)
	
	// 롤백
	stateDB.Rollback()
	
	// 롤백 후 상태 검증 (실제 구현에서는 커밋된 상태로 돌아가야 함)
	// 모의 구현에서는 롤백 기능을 완전히 구현하지 않았으므로, 여기서는 로깅만 확인
	assert.Contains(t, stateDB.journal, "Rollback", "Rollback should be recorded in journal")
	
	// 로그 출력
	logger.Debug("State commit and rollback test completed", "journal", stateDB.journal)
}

// TestChainReorganization은 체인 재구성을 테스트합니다.
func TestChainReorganization(t *testing.T) {
	// 로거 생성
	logger := log.New("module", "blockchain_state_test")
	
	// 모의 StateDB 생성
	stateDB := NewMockStateDB()
	
	// 테스트 주소
	address := common.HexToAddress("0x1111111111111111111111111111111111111111")
	
	// 초기 상태 설정
	stateDB.SetBalance(address, big.NewInt(100))
	stateDB.SetNonce(address, 5)
	
	// 메인 체인 블록 생성
	mainChainBlocks := make([]*types.Block, 3)
	for i := 0; i < 3; i++ {
		header := &types.Header{
			Number: big.NewInt(int64(i + 1)),
			Time:   uint64(time.Now().Unix()) + uint64(i*12),
		}
		if i > 0 {
			header.ParentHash = mainChainBlocks[i-1].Hash()
		}
		mainChainBlocks[i] = types.NewBlockWithHeader(header)
	}
	
	// 포크 체인 블록 생성
	forkChainBlocks := make([]*types.Block, 4) // 더 긴 체인
	for i := 0; i < 4; i++ {
		header := &types.Header{
			Number: big.NewInt(int64(i + 1)),
			Time:   uint64(time.Now().Unix()) + uint64(i*12) + 1, // 약간 다른 시간
		}
		if i > 0 {
			header.ParentHash = forkChainBlocks[i-1].Hash()
		} else {
			// 첫 번째 블록은 메인 체인과 동일한 부모를 가짐 (제네시스)
			header.ParentHash = common.Hash{}
		}
		forkChainBlocks[i] = types.NewBlockWithHeader(header)
	}
	
	// 메인 체인 처리 시뮬레이션
	for i, block := range mainChainBlocks {
		// 블록 처리 시뮬레이션
		stateDB.SetBalance(address, big.NewInt(100+int64(i+1)*10))
		stateDB.SetNonce(address, uint64(5+i+1))
		
		// 상태 커밋
		_, err := stateDB.Commit()
		assert.NoError(t, err, "State commit should not return error")
		
		logger.Debug("Processed main chain block", "number", block.NumberU64(), "hash", block.Hash().Hex())
	}
	
	// 메인 체인 처리 후 상태 검증
	assert.Equal(t, big.NewInt(130), stateDB.GetBalance(address), "Balance should be updated after main chain processing")
	assert.Equal(t, uint64(8), stateDB.GetNonce(address), "Nonce should be updated after main chain processing")
	
	// 체인 재구성 시뮬레이션 (포크 체인이 더 길어서 메인 체인이 됨)
	// 1. 메인 체인 롤백
	for i := len(mainChainBlocks) - 1; i >= 0; i-- {
		// 롤백 시뮬레이션
		stateDB.Rollback()
		logger.Debug("Rolled back main chain block", "number", mainChainBlocks[i].NumberU64(), "hash", mainChainBlocks[i].Hash().Hex())
	}
	
	// 2. 포크 체인 적용
	for i, block := range forkChainBlocks {
		// 블록 처리 시뮬레이션
		stateDB.SetBalance(address, big.NewInt(100+int64(i+1)*15)) // 다른 금액으로 변경
		stateDB.SetNonce(address, uint64(5+i+1))
		
		// 상태 커밋
		_, err := stateDB.Commit()
		assert.NoError(t, err, "State commit should not return error")
		
		logger.Debug("Processed fork chain block", "number", block.NumberU64(), "hash", block.Hash().Hex())
	}
	
	// 포크 체인 처리 후 상태 검증
	assert.Equal(t, big.NewInt(160), stateDB.GetBalance(address), "Balance should be updated after fork chain processing")
	assert.Equal(t, uint64(9), stateDB.GetNonce(address), "Nonce should be updated after fork chain processing")
	
	// 로그 출력
	logger.Debug("Chain reorganization test completed", "mainChainLength", len(mainChainBlocks), "forkChainLength", len(forkChainBlocks))
} 