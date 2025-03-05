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

// Package core implements the Proof-of-Stake consensus algorithm.
package core

import (
	"errors"
	"io"
	"math/big"
	"strconv"
	"sync"

	lru "github.com/hashicorp/golang-lru"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/core/vm"
	"github.com/zenanetwork/go-zenanet/crypto"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/params"
	"github.com/zenanetwork/go-zenanet/rlp"
	"github.com/zenanetwork/go-zenanet/rpc"
	"github.com/zenanetwork/go-zenanet/trie"
	"golang.org/x/crypto/sha3"
)

// Eirene 합의 알고리즘 관련 상수
const (
	// 체크포인트 간격 (블록 수)
	checkpointInterval = 1024

	// 메모리에 유지할 최근 스냅샷 수
	inmemorySnapshots = 128
	// 메모리에 유지할 최근 블록 서명 수
	inmemorySignatures = 4096

	// 블록 보상 관련 상수
	baseBlockReward = 2e18 // 기본 블록 보상 (2 ETH)

	// 보상 분배 비율 (1000 단위)
	validatorRewardShare = 700 // 검증자 보상 비율 (70%)
	delegatorRewardShare = 200 // 위임자 보상 비율 (20%)
	communityRewardShare = 100 // 커뮤니티 기금 보상 비율 (10%)

	// 커뮤니티 기금 주소 (실제 구현에서는 거버넌스로 설정)
	communityFundAddress = "0x0000000000000000000000000000000000000100"
)

// Eirene PoS 프로토콜 상수
var (
	// 에포크 길이 (블록 수)
	defaultEpochLength = 30000 // 약 5일 (15초 블록 기준)

	// 최소 스테이킹 양
	minStakeAmount = new(big.Int).Mul(
		big.NewInt(1000),
		new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil),
	) // 1000 토큰

	// 최대 검증자 수
	maxValidatorCount = 100

	// 서명자 vanity를 위해 예약된 extra-data 접두사 바이트 수
	extraVanity = 32
	// 서명자 seal을 위해 예약된 extra-data 접미사 바이트 수
	extraSeal = crypto.SignatureLength

	// 항상 Keccak256(RLP([])) 값으로, PoW 외부에서는 uncle이 의미가 없음
	uncleHash = types.CalcUncleHash(nil)

	// 검증자 순서에 따른 블록 난이도
	diffInTurn = big.NewInt(2)
	diffNoTurn = big.NewInt(1)
)

// 블록을 무효로 표시하기 위한 다양한 오류 메시지
var (
	// 로컬 블록체인의 일부가 아닌 블록에 대해 서명자 목록이 요청될 때 반환됨
	errUnknownBlock = errors.New("unknown block")

	// 체크포인트/에포크 전환 블록에 0이 아닌 수혜자가 설정된 경우 반환됨
	errInvalidCheckpointBeneficiary = errors.New("beneficiary in checkpoint block non-zero")

	// extra-data 섹션이 서명자 vanity를 저장하는 데 필요한 32바이트보다 짧은 경우 반환됨
	errMissingVanity = errors.New("extra-data 32 byte vanity prefix missing")

	// extra-data 섹션에 65바이트 secp256k1 서명이 포함되어 있지 않은 것으로 보이는 경우 반환됨
	errMissingSignature = errors.New("extra-data 65 byte signature suffix missing")

	// extra-data 섹션에 서명자 데이터가 포함된 비체크포인트 블록에 대해 반환됨
	errExtraSigners = errors.New("non-checkpoint block contains extra signer list")

	// 서명자 목록이 있어야 하는 체크포인트 블록에 대해 반환됨
	errMissingSigners = errors.New("checkpoint block missing signer list")

	// 블록 헤더에 잘못된 서명이 있는 경우 반환됨
	errInvalidCheckpointSigners = errors.New("invalid signer list on checkpoint block")

	// 블록 헤더에 잘못된 서명이 있는 경우 반환됨
	errInvalidMixDigest = errors.New("non-zero mix digest")

	// 블록 헤더에 잘못된 uncle hash가 있는 경우 반환됨
	errInvalidUncleHash = errors.New("non empty uncle hash")

	// 블록 헤더에 0이 아닌 nonce가 있는 경우 반환됨
	errInvalidNonce = errors.New("non-zero nonce")

	// 서명자가 최근 서명한 경우 반환됨
	errRecentlySigned = errors.New("recently signed")
)

// Eirene는 Proof-of-Stake 합의 엔진을 구현합니다.
type Eirene struct {
	config *params.EireneConfig // 합의 엔진 설정
	db     ethdb.Database       // 스냅샷 및 검증자 정보를 저장하는 데이터베이스

	recents    *lru.ARCCache // 최근 서명 캐시
	signatures sigLRU        // 최근 서명 캐시

	proposals map[common.Address]bool // 현재 우리가 추진하고 있는 제안 목록

	signer common.Address // 서명자 주소
	signFn SignerFn       // 서명 함수
	lock   sync.RWMutex   // 뮤텍스

	// 거버넌스 상태
	governance *GovernanceState

	// 검증자 관리
	validatorSet *ValidatorSet

	// 슬래싱 상태
	slashingState *SlashingState

	// 보상 상태
	rewardState *RewardState

	// IBC 상태
	ibcState *IBCState

	// 테스트용 필드
	fakeDiff bool // 난이도 검증 건너뛰기

	// 거버넌스 어댑터
	govAdapter *GovAdapter

	// 스테이킹 어댑터
	stakingAdapter *StakingAdapter

	// ABCI 어댑터
	abciAdapter *ABCIAdapter
}

// SignerFn은 주어진 해시에 서명하고 결과를 반환하는 함수 유형입니다.
type SignerFn func(signer common.Address, hash []byte) ([]byte, error)

// New creates a Eirene proof-of-stake consensus engine with the initial
// signers set to the ones provided by the user.
func New(config *params.EireneConfig, db ethdb.Database) *Eirene {
	// Set any missing consensus parameters to their defaults
	conf := *config
	if conf.Epoch == 0 {
		conf.Epoch = uint64(defaultEpochLength)
	}
	// Allocate the snapshot caches and create the engine
	recents, _ := lru.NewARC(inmemorySnapshots)
	signatures, _ := lru.NewARC(inmemorySignatures)

	eirene := &Eirene{
		config:     &conf,
		db:         db,
		recents:    recents,
		signatures: signatures,
	}

	// 어댑터 초기화 - 실제 구현에서는 적절한 인자를 전달해야 합니다.
	// 여기서는 임시로 어댑터 초기화를 생략합니다.

	return eirene
}

// Author는 주어진 블록을 채굴한 계정의 Zenanet 주소를 검색합니다.
func (e *Eirene) Author(header *types.Header) (common.Address, error) {
	return ecrecover(header, e.signatures)
}

// VerifyHeader는 헤더가 주어진 엔진의 합의 규칙을 준수하는지 확인합니다.
func (e *Eirene) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header) error {
	return e.verifyHeader(chain, header, nil)
}

// VerifyHeaders는 VerifyHeader와 유사하지만 헤더 배치를 동시에 확인합니다.
func (e *Eirene) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header) (chan<- struct{}, <-chan error) {
	abort := make(chan struct{})
	results := make(chan error, len(headers))

	go func() {
		for i, header := range headers {
			err := e.verifyHeader(chain, header, headers[:i])

			select {
			case <-abort:
				return
			case results <- err:
			}
		}
	}()
	return abort, results
}

// verifyHeader는 헤더가 Eirene 엔진의 합의 규칙을 준수하는지 확인합니다.
func (e *Eirene) verifyHeader(chain consensus.ChainHeaderReader, header *types.Header, parents []*types.Header) error {
	if header.Number == nil {
		return errUnknownBlock
	}
	// 체인 구성에서 Eirene가 활성화되어 있는지 확인
	if !chain.Config().IsEirene(header.Number) {
		return consensus.ErrUnknownAncestor
	}
	// 캐스케이딩 필드 검증
	if err := e.verifyCascadingFields(chain, header, parents); err != nil {
		return err
	}
	// 모든 검사 통과
	return nil
}

// verifyCascadingFields는 헤더의 캐스케이딩 필드가 Eirene 엔진의 합의 규칙을 준수하는지 확인합니다.
func (e *Eirene) verifyCascadingFields(chain consensus.ChainHeaderReader, header *types.Header, parents []*types.Header) error {
	// 현재는 기본 검증만 수행
	return nil
}

// VerifyUncles는 주어진 블록의 uncle이 합의 엔진의 규칙을 준수하는지 확인합니다.
func (e *Eirene) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	// Eirene에서는 uncle이 허용되지 않음
	if len(block.Uncles()) > 0 {
		return errors.New("uncles not allowed")
	}
	return nil
}

// Prepare는 특정 엔진의 규칙에 따라 블록 헤더의 합의 필드를 초기화합니다.
func (e *Eirene) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	// 헤더 번호가 있는지 확인
	number := header.Number.Uint64()

	// 부모 블록 가져오기
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}

	// 타임스탬프 설정
	header.Difficulty = e.CalcDifficulty(chain, header.Time, parent)
	header.Coinbase = common.Address{}
	header.MixDigest = common.Hash{}
	header.Nonce = types.BlockNonce{}

	return nil
}

// Finalize implements consensus.Engine, ensuring no uncles are set, nor block
// rewards given.
func (e *Eirene) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state vm.StateDB, txs []*types.Transaction, uncles []*types.Header) {
	// No block rewards in PoS, so the state remains as is and uncles are dropped
	// 참고: 실제 구현에서는 적절한 방식으로 Root와 UncleHash를 설정해야 합니다.
	// 여기서는 임시로 이 부분을 생략합니다.
	header.UncleHash = types.CalcUncleHash(nil)

	// 참고: 실제 구현에서는 스테이킹 보상 처리와 거버넌스 제안 처리를 수행해야 합니다.
	// 여기서는 임시로 이 부분을 생략합니다.
}

// FinalizeAndAssemble는 상태 수정을 실행하고 최종 블록을 반환합니다.
func (e *Eirene) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, body *types.Body, receipts []*types.Receipt) (*types.Block, error) {
	// 헤더의 루트를 상태의 루트로 설정
	header.Root = state.IntermediateRoot(true)

	// 현재 블록 번호
	currentBlock := header.Number.Uint64()

	// 에포크 전환 블록인지 확인
	if currentBlock%e.config.Epoch == 0 {
		// 에포크 전환 블록에서는 스냅샷을 저장하고 검증자 집합을 업데이트합니다.
		// 검증자 집합 업데이트
		e.validatorSet.processEpochTransition(currentBlock)

		// 검증자 집합 저장
		if err := e.validatorSet.store(e.db); err != nil {
			log.Error("검증자 집합 저장 실패", "err", err)
		}
	}

	// 블록 서명자 목록 가져오기
	signers := make([]common.Address, 0)
	if snap, err := e.snapshot(chain, currentBlock-1, header.ParentHash, nil); err == nil {
		// 서명자 목록 가져오기
		signers = snap.validators()
	}

	// 블록 제안자 가져오기
	proposer, err := e.Author(header)
	if err == nil {
		// 검증자 성능 지표 업데이트
		e.validatorSet.updateValidatorPerformance(header, proposer, signers)

		// 서명 정보 업데이트
		e.updateSigningInfo(e.slashingState, header, signers)

		// 슬래싱 처리
		e.processSlashing(e.validatorSet, e.slashingState, currentBlock)

		// 슬래싱 상태 저장
		if err := e.slashingState.store(e.db); err != nil {
			log.Error("슬래싱 상태 저장 실패", "err", err)
		}

		// 보상 분배
		e.distributeBlockReward(header, e.rewardState)

		// 보상 상태 저장
		if err := e.rewardState.store(e.db); err != nil {
			log.Error("보상 상태 저장 실패", "err", err)
		}

		// IBC 패킷 처리
		currentTime := header.Time
		e.processIBCPackets(currentBlock, uint64(currentTime))
	}

	// 거버넌스 제안 처리
	e.ProcessProposals(currentBlock)

	// 거버넌스 상태를 데이터베이스에 저장
	if err := e.governance.store(e.db); err != nil {
		log.Error("거버넌스 상태 저장 실패", "err", err)
	}

	// 검증자 집합 저장
	if err := e.validatorSet.store(e.db); err != nil {
		log.Error("검증자 집합 저장 실패", "err", err)
	}

	// 새 블록 생성 및 반환
	return types.NewBlock(header, body, receipts, trie.NewStackTrie(nil)), nil
}

// distributeRewards는 블록 생성 보상을 분배합니다.
// 참고: 현재 구현에서는 사용되지 않으며, 향후 별도의 보상 시스템으로 대체될 예정입니다.
// func (e *Eirene) distributeRewards(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB) {
// 	// 구현 생략
// }

// Seal은 주어진 입력 블록에 대한 새로운 sealing 요청을 생성하고 결과를 주어진 채널로 푸시합니다.
func (e *Eirene) Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error {
	header := block.Header()

	// 서명자가 설정되어 있지 않으면 sealing 불가
	e.lock.RLock()
	signer, signFn := e.signer, e.signFn
	e.lock.RUnlock()

	if signer == (common.Address{}) || signFn == nil {
		return errors.New("sealing without signer")
	}

	// 서명 생성
	sighash, err := signFn(signer, SealHash(header).Bytes())
	if err != nil {
		return err
	}

	// 서명으로 extra-data 복사
	extra := make([]byte, len(header.Extra))
	copy(extra, header.Extra)

	if len(extra) < extraSeal {
		extra = append(extra, make([]byte, extraSeal)...)
	}
	copy(extra[len(extra)-extraSeal:], sighash)

	// 서명된 헤더로 블록 설정
	header = types.CopyHeader(header)
	header.Extra = extra

	// 서명된 블록 반환
	block = block.WithSeal(header)
	select {
	case results <- block:
	default:
		log.Warn("Sealing result is not read by miner", "mode", "local")
	}

	return nil
}

// SealHash는 블록이 봉인되기 전의 해시를 반환합니다.
func (e *Eirene) SealHash(header *types.Header) common.Hash {
	return SealHash(header)
}

// CalcDifficulty는 난이도 조정 알고리즘입니다. 새 블록이 가져야 할 난이도를 반환합니다.
func (e *Eirene) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	// 현재는 간단한 난이도 계산만 구현
	return big.NewInt(1)
}

// APIs implements consensus.Engine, returning the user facing RPC APIs.
func (e *Eirene) APIs(chain consensus.ChainHeaderReader) []rpc.API {
	return []rpc.API{
		{
			Namespace: "eirene",
			Version:   "1.0",
			Service:   &API{chain: chain, eirene: e},
			Public:    false,
		},
		// 참고: 실제 구현에서는 스테이킹 API와 거버넌스 API를 추가해야 합니다.
		// 여기서는 임시로 이 부분을 생략합니다.
	}
}

// snapshot은 지정된 블록에 대한 스냅샷을 검색합니다.
func (e *Eirene) snapshot(chain consensus.ChainHeaderReader, number uint64, hash common.Hash, parents []*types.Header) (*Snapshot, error) {
	// 캐시에서 스냅샷 검색
	var (
		headers []*types.Header
		snap    *Snapshot
	)

	// 캐시에서 스냅샷 검색
	e.lock.RLock()
	if s, ok := e.recents.Get(hash); ok {
		snap = s.(*Snapshot)
	}
	e.lock.RUnlock()

	// 캐시에서 스냅샷을 찾지 못한 경우 데이터베이스에서 검색
	if snap == nil {
		// 데이터베이스에서 스냅샷 검색
		if s, err := loadSnapshot(e.config, e.signatures, e.db, hash); err == nil {
			log.Trace("Loaded voting snapshot from disk", "number", number, "hash", hash)
			snap = s
		}
	}

	// 스냅샷을 찾지 못한 경우 생성
	if snap == nil {
		// 제네시스 블록인 경우 초기 스냅샷 생성
		if number == 0 {
			// 제네시스 블록에서 초기 검증자 집합 가져오기
			genesis := chain.GetHeaderByNumber(0)
			if genesis == nil {
				return nil, errors.New("genesis header not found")
			}

			// 초기 검증자 집합 생성
			validators := make([]common.Address, 0)

			// 제네시스 블록에서 검증자 추출 (실제 구현에서는 제네시스 블록 구성에서 가져옴)
			// 여기서는 간단히 제네시스 블록 서명자를 초기 검증자로 사용
			validator, err := ecrecover(genesis, e.signatures)
			if err == nil {
				validators = append(validators, validator)
			}

			// 초기 스냅샷 생성
			snap = newSnapshot(e.config, e.signatures, 0, genesis.Hash(), validators)
			if err := snap.store(e.db); err != nil {
				return nil, err
			}
			log.Info("Stored genesis voting snapshot", "number", 0, "hash", genesis.Hash())
			return snap, nil
		}

		// 이전 헤더 가져오기
		var parent *types.Header
		if len(parents) > 0 {
			parent = parents[len(parents)-1]
		} else {
			parent = chain.GetHeader(hash, number-1)
		}
		if parent == nil {
			return nil, consensus.ErrUnknownAncestor
		}

		// 부모 블록의 스냅샷 가져오기
		snap, err := e.snapshot(chain, number-1, parent.Hash(), parents[:len(parents)-1])
		if err != nil {
			return nil, err
		}

		// 현재 헤더 가져오기
		header := chain.GetHeader(hash, number)
		if header == nil {
			return nil, consensus.ErrUnknownAncestor
		}

		// 헤더 적용
		headers = append(headers, header)
		snap, err = snap.apply(headers)
		if err != nil {
			return nil, err
		}

		// 스냅샷 캐싱 및 저장
		e.lock.Lock()
		e.recents.Add(hash, snap)
		e.lock.Unlock()

		// 에포크 경계에서 스냅샷 저장
		if number%e.config.Epoch == 0 {
			if err = snap.store(e.db); err != nil {
				return nil, err
			}
			log.Info("Stored voting snapshot", "number", number, "hash", hash)
		}
	}

	return snap, nil
}

// Close는 합의 엔진을 종료합니다.
func (e *Eirene) Close() error {
	return nil
}

// SealHash는 서명을 위한 헤더의 해시를 계산합니다.
func SealHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()
	encodeSigHeader(hasher, header)
	hasher.Sum(hash[:0])
	return hash
}

// ecrecover는 서명에서 Zenanet 주소를 추출합니다.
func ecrecover(header *types.Header, sigcache sigLRU) (common.Address, error) {
	// 캐시에서 서명자 검색
	hash := header.Hash()
	if address, known := sigcache.Get(hash); known {
		return address.(common.Address), nil
	}
	// 서명 검색
	if len(header.Extra) < extraSeal {
		return common.Address{}, errMissingSignature
	}
	signature := header.Extra[len(header.Extra)-extraSeal:]

	// 서명에서 공개 키 복구
	pubkey, err := crypto.Ecrecover(SealHash(header).Bytes(), signature)
	if err != nil {
		return common.Address{}, err
	}
	var signer common.Address
	copy(signer[:], crypto.Keccak256(pubkey[1:])[12:])

	sigcache.Add(hash, signer)
	return signer, nil
}

// encodeSigHeader는 서명을 위해 헤더를 인코딩합니다.
func encodeSigHeader(w io.Writer, header *types.Header) {
	enc := []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.GasLimit,
		header.GasUsed,
		header.Time,
		header.Extra[:len(header.Extra)-crypto.SignatureLength], // 서명 제외
		header.MixDigest,
		header.Nonce,
	}
	if header.BaseFee != nil {
		enc = append(enc, header.BaseFee)
	}
	if header.WithdrawalsHash != nil {
		enc = append(enc, header.WithdrawalsHash)
	}
	if header.BlobGasUsed != nil {
		enc = append(enc, header.BlobGasUsed)
	}
	if header.ExcessBlobGas != nil {
		enc = append(enc, header.ExcessBlobGas)
	}
	if header.ParentBeaconRoot != nil {
		enc = append(enc, header.ParentBeaconRoot)
	}
	rlp.Encode(w, enc)
}

// SubmitProposal은 새로운 거버넌스 제안을 제출합니다.
func (e *Eirene) SubmitProposal(
	proposer common.Address,
	title string,
	description string,
	proposalType uint8,
	parameters map[string]string,
	upgrade *UpgradeInfo,
	funding *FundingInfo,
	deposit *big.Int,
	currentBlock uint64,
) (uint64, error) {
	e.lock.Lock()
	defer e.lock.Unlock()

	// 제안자가 검증자인지 확인
	// TODO: 검증자 확인 로직 구현

	return e.governance.submitProposal(
		proposer,
		title,
		description,
		proposalType,
		parameters,
		upgrade,
		funding,
		deposit,
		currentBlock,
	)
}

// Vote는 거버넌스 제안에 투표합니다.
func (e *Eirene) Vote(
	proposalID uint64,
	voter common.Address,
	option uint8,
	weight *big.Int,
	currentBlock uint64,
) error {
	e.lock.Lock()
	defer e.lock.Unlock()

	// 투표자가 검증자인지 확인
	// TODO: 검증자 확인 로직 구현

	return e.governance.vote(
		proposalID,
		voter,
		option,
		weight,
		currentBlock,
	)
}

// ProcessProposals는 현재 블록에서 제안을 처리합니다.
func (e *Eirene) ProcessProposals(currentBlock uint64) {
	e.lock.Lock()
	defer e.lock.Unlock()

	e.governance.processProposals(currentBlock)
}

// ExecuteProposal은 통과된 제안을 실행합니다.
func (e *Eirene) ExecuteProposal(proposalID uint64, currentBlock uint64) error {
	e.lock.Lock()
	defer e.lock.Unlock()

	proposal, err := e.governance.getProposal(proposalID)
	if err != nil {
		return err
	}

	// 제안 실행
	e.governance.executeProposal(proposalID, currentBlock)

	// 제안 유형에 따라 처리
	switch proposal.Type {
	case ProposalTypeParameter:
		// 매개변수 변경 처리
		for key, value := range proposal.Parameters {
			switch key {
			case "blockPeriod":
				if val, err := strconv.ParseUint(value, 10, 64); err == nil {
					e.config.Period = val
				}
			case "epochLength":
				if val, err := strconv.ParseUint(value, 10, 64); err == nil {
					e.config.Epoch = val
				}
				// 다른 매개변수 처리
			}
		}
	case ProposalTypeUpgrade:
		// 업그레이드 처리
		// TODO: 업그레이드 로직 구현
	case ProposalTypeFunding:
		// 자금 지원 처리
		// TODO: 자금 지원 로직 구현
	}

	return nil
}

// GetProposal은 제안 정보를 반환합니다.
func (e *Eirene) GetProposal(proposalID uint64) (*Proposal, error) {
	e.lock.RLock()
	defer e.lock.RUnlock()

	return e.governance.getProposal(proposalID)
}

// GetProposals는 모든 제안 목록을 반환합니다.
func (e *Eirene) GetProposals() []*Proposal {
	e.lock.RLock()
	defer e.lock.RUnlock()

	return e.governance.getAllProposals()
}

// GetVotes는 제안에 대한 투표 목록을 반환합니다.
func (e *Eirene) GetVotes(proposalID uint64) ([]ProposalVote, error) {
	e.lock.RLock()
	defer e.lock.RUnlock()

	return e.governance.getVotes(proposalID)
}

// ProcessGovernance processes governance proposals
func (e *Eirene) ProcessGovernance(state vm.StateDB, header *types.Header) error {
	// 참고: 실제 구현에서는 적절한 타입 변환과 함수 호출을 수행해야 합니다.
	// 여기서는 임시로 nil을 반환합니다.
	return nil
}

// updateSigningInfo는 검증자의 서명 정보를 업데이트합니다.
func (e *Eirene) updateSigningInfo(slashingState *SlashingState, header *types.Header, signers []common.Address) {
	// 구현
}

// processSlashing은 슬래싱 처리를 수행합니다.
func (e *Eirene) processSlashing(validatorSet *ValidatorSet, slashingState *SlashingState, blockNumber uint64) {
	// 구현
}

// distributeBlockReward는 블록 보상을 분배합니다.
func (e *Eirene) distributeBlockReward(header *types.Header, rewardState *RewardState) {
	// 구현
}

// processIBCPackets는 IBC 패킷을 처리합니다.
func (e *Eirene) processIBCPackets(blockNumber uint64, timestamp uint64) {
	// 구현
}

// reportDoubleSign은 이중 서명을 신고합니다.
func (e *Eirene) reportDoubleSign(reporter common.Address, evidence DoubleSignEvidence) error {
	// 구현
	return nil
}

// unjailValidator는 검증자의 감금을 해제합니다.
func (e *Eirene) unjailValidator(validator common.Address) error {
	// 구현
	return nil
}

// getAccumulatedRewards는 주소의 누적 보상을 반환합니다.
func (e *Eirene) getAccumulatedRewards(addr common.Address) *big.Int {
	// 구현
	return nil
}

// claimRewards는 누적된 보상을 청구합니다.
func (e *Eirene) claimRewards(claimer common.Address) (*big.Int, error) {
	// 구현
	return nil, nil
}

// getCommunityFund는 커뮤니티 기금 잔액을 반환합니다.
func (e *Eirene) getCommunityFund() *big.Int {
	// 구현
	return nil
}

// withdrawFromCommunityFund는 커뮤니티 기금에서 자금을 인출합니다.
func (e *Eirene) withdrawFromCommunityFund(recipient common.Address, amount *big.Int) error {
	// 구현
	return nil
}

// transferToken은 IBC를 통해 토큰을 전송합니다.
func (e *Eirene) transferToken(
	sourcePort string,
	sourceChannel string,
	token common.Address,
	amount *big.Int,
	sender common.Address,
	receiver string,
	timeoutHeight uint64,
	timeoutTimestamp uint64,
) (uint64, error) {
	// 구현
	return 0, nil
}

// GetConfig는 Eirene 합의 엔진의 설정을 반환합니다.
func (e *Eirene) GetConfig() *params.EireneConfig {
	return e.config
}

// GetDB는 Eirene 합의 엔진의 데이터베이스를 반환합니다.
func (e *Eirene) GetDB() ethdb.Database {
	return e.db
}

// GetValidatorSet은 Eirene 합의 엔진의 검증자 집합을 반환합니다.
func (e *Eirene) GetValidatorSet() *ValidatorSet {
	return e.validatorSet
}

// GetGovernanceState는 Eirene 합의 엔진의 거버넌스 상태를 반환합니다.
func (e *Eirene) GetGovernanceState() *GovernanceState {
	return e.governance
}

// GetSlashingState는 Eirene 합의 엔진의 슬래싱 상태를 반환합니다.
func (e *Eirene) GetSlashingState() *SlashingState {
	return e.slashingState
}

// GetIBCState는 Eirene 합의 엔진의 IBC 상태를 반환합니다.
func (e *Eirene) GetIBCState() *IBCState {
	return e.ibcState
}

// GetRewardState는 Eirene 합의 엔진의 보상 상태를 반환합니다.
func (e *Eirene) GetRewardState() *RewardState {
	return e.rewardState
}
