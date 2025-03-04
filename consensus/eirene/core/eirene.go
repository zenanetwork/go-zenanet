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
	"sync"

	lru "github.com/hashicorp/golang-lru"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/consensus"
	"github.com/zenanetwork/go-zenanet/consensus/eirene/utils"
	"github.com/zenanetwork/go-zenanet/core"
	"github.com/zenanetwork/go-zenanet/core/state"
	"github.com/zenanetwork/go-zenanet/core/types"
	"github.com/zenanetwork/go-zenanet/core/vm"
	"github.com/zenanetwork/go-zenanet/crypto"
	"github.com/zenanetwork/go-zenanet/ethdb"
	"github.com/zenanetwork/go-zenanet/event"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/params"
	"github.com/zenanetwork/go-zenanet/rlp"
	"github.com/zenanetwork/go-zenanet/rpc"
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

	// 엑스트라 데이터 필드의 vanity 부분 크기
	extraVanity = 32
	// 엑스트라 데이터 필드의 서명 부분 크기
	extraSeal = 65
	// 검증자 선택 알고리즘에서 사용하는 난이도 조정 매개변수
	diffInTurn = 2
	diffNoTurn = 1

	// 기본 블록 생성 주기 (초)
	defaultPeriod = 15
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

	// 항상 Keccak256(RLP([])) 값으로, PoW 외부에서는 uncle이 의미가 없음
	uncleHash = types.CalcUncleHash(nil)
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

	// ErrInvalidValidatorSet은 검증자 집합이 유효하지 않을 때 반환됩니다
	ErrInvalidValidatorSet = errors.New("invalid validator set")
	// ErrInvalidCheckpointSignature는 체크포인트 서명이 유효하지 않을 때 반환됩니다
	ErrInvalidCheckpointSignature = errors.New("invalid checkpoint signature")
	// ErrInvalidExtraDataFormat은 엑스트라 데이터 형식이 유효하지 않을 때 반환됩니다
	ErrInvalidExtraDataFormat = errors.New("invalid extra data format")
	// ErrUnauthorizedValidator는 승인되지 않은 검증자가 블록을 생성하려고 할 때 반환됩니다
	ErrUnauthorizedValidator = errors.New("unauthorized validator")
)

// Eirene는 Proof-of-Stake 합의 엔진을 구현합니다.
// 이 구조체는 블록 생성, 검증, 검증자 관리, 거버넌스, 슬래싱, 보상 분배 등의
// 기능을 제공합니다. Eirene는 Cosmos SDK와 Tendermint의 합의 메커니즘을 기반으로 하며,
// go-zenanet의 합의 엔진 인터페이스를 구현합니다.
type Eirene struct {
	config *params.EireneConfig // 합의 엔진 설정
	db     ethdb.Database       // 스냅샷 및 검증자 정보를 저장하는 데이터베이스

	recents    *recentBlocks // 최근 서명된 블록의 캐시
	signatures *lru.ARCCache // 최근 블록 서명의 캐시

	proposals map[common.Address]bool // 현재 우리가 추진하고 있는 제안 목록

	signer common.Address // 서명자 주소
	signFn SignerFn       // 서명 함수
	lock   sync.RWMutex   // 뮤텍스

	// 거버넌스 상태
	governance utils.GovernanceInterface

	// 검증자 관리
	validatorSet utils.ValidatorSetInterface

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

	// 블록 생성 및 검증
	currentBlock func() *types.Block      // 현재 블록을 가져오는 함수
	stateAt      func(common.Hash) (*state.StateDB, error) // 특정 해시에서 상태를 가져오는 함수

	// 이벤트 피드
	eventMux      *event.TypeMux // 이벤트 멀티플렉서
	eventFeed     *event.Feed    // 이벤트 피드
	chainHeadCh   chan core.ChainHeadEvent
	chainHeadSub  event.Subscription
}

// SignerFn은 주어진 해시에 서명하고 결과를 반환하는 함수 유형입니다.
// 이 함수는 블록 서명 및 검증에 사용됩니다.
//
// 매개변수:
//   - signer: 서명자 주소
//   - hash: 서명할 해시
//
// 반환값:
//   - []byte: 서명 결과
//   - error: 오류 발생 시 반환
type SignerFn func(signer common.Address, hash []byte) ([]byte, error)

// New는 새로운 Eirene 합의 엔진을 생성합니다.
//
// 매개변수:
//   - config: Eirene 합의 엔진 설정
//   - db: 스냅샷 및 검증자 정보를 저장하는 데이터베이스
//
// 반환값:
//   - *Eirene: 새로운 Eirene 합의 엔진 인스턴스
func New(config *params.EireneConfig, db ethdb.Database) *Eirene {
	// 기본 설정 적용
	conf := *config
	if conf.Period == 0 {
		conf.Period = defaultPeriod
	}
	if conf.Epoch == 0 {
		conf.Epoch = uint64(defaultEpochLength)
	}
	// Allocate the snapshot caches and create the engine
	recents := newRecentBlocks()
	signatures, _ := lru.NewARC(inmemorySignatures)

	eirene := &Eirene{
		config:     &conf,
		db:         db,
		recents:    recents,
		signatures: signatures,
		proposals:  make(map[common.Address]bool),
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
func (e *Eirene) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header, seal bool) error {
	return e.verifyHeader(chain, header, nil)
}

// VerifyHeaders는 VerifyHeader와 유사하지만 헤더 배치를 동시에 확인합니다.
func (e *Eirene) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
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

// Finalize는 합의 엔진에 의해 모든 상태 전환이 실행된 후 블록 헤더를 준비합니다.
func (e *Eirene) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state vm.StateDB, txs []*types.Transaction, uncles []*types.Header) {
	// 보상 분배
	e.distributeBlockReward(header, e.rewardState)
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
		// e.validatorSet.processEpochTransition(currentBlock)

		// 검증자 집합 저장
		// if err := e.validatorSet.store(e.db); err != nil {
		// 	log.Error("검증자 집합 저장 실패", "err", err)
		// }
	}

	// 블록 서명자 목록 가져오기
	signers := make([]common.Address, 0)
	if snap, err := e.snapshot(chain, currentBlock-1, header.ParentHash, nil); err == nil {
		signers = snap.validators()
	}

	// 보상 분배
	e.distributeBlockReward(header, e.rewardState)

	// 서명 정보 업데이트
	e.updateSigningInfo(e.slashingState, header, signers)

	// 슬래싱 처리
	e.processSlashing(e.validatorSet, e.slashingState, currentBlock)

	// 보상 상태 저장
	if err := e.rewardState.store(e.db); err != nil {
		log.Error("보상 상태 저장 실패", "err", err)
	}

	// IBC 패킷 처리
	currentTime := header.Time
	e.processIBCPackets(currentBlock, uint64(currentTime))

	// 거버넌스 제안 처리
	e.ProcessProposals(currentBlock)

	// 거버넌스 상태를 데이터베이스에 저장
	if err := e.saveGovernanceState(); err != nil {
		log.Error("거버넌스 상태 저장 실패", "err", err)
	}

	// 검증자 집합 저장
	// if err := e.validatorSet.store(e.db); err != nil {
	// 	log.Error("검증자 집합 저장 실패", "err", err)
	// }

	// 새 블록 생성 및 반환
	return types.NewBlock(header, nil, nil, nil), nil
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

// snapshot은 지정된 블록 번호와 해시에 대한 스냅샷을 검색합니다.
func (e *Eirene) snapshot(chain consensus.ChainHeaderReader, number uint64, hash common.Hash, parents []*types.Header) (*Snapshot, error) {
	// 스냅샷 검색
	var snap *Snapshot

	// 캐시에서 스냅샷 검색
	e.lock.RLock()
	// if s, ok := e.recents.get(hash); ok {
	// 	snap = s.(*Snapshot)
	// }
	e.lock.RUnlock()

	if snap == nil {
		// 데이터베이스에서 스냅샷 검색
		if s, err := loadSnapshot(e.config, e.db, hash); err == nil {
			log.Trace("Loaded voting snapshot from disk", "number", number, "hash", hash)
			snap = s
		}
	}

	if snap == nil {
		// 스냅샷이 없으면 부모 블록에서 생성
		if number == 0 {
			// 제네시스 블록인 경우
			genesis := chain.GetHeaderByNumber(0)
			if genesis == nil {
				return nil, errors.New("genesis block not found")
			}

			// 초기 검증자 목록 생성
			validators := make(map[common.Address]uint64)
			// 실제 구현에서는 제네시스 블록에서 초기 검증자 목록을 가져와야 합니다.
			// 여기서는 임시로 빈 목록을 사용합니다.

			// 초기 스냅샷 생성
			snap = newSnapshot(e.config, 0, genesis.Hash(), validators)
			if err := snap.store(e.db); err != nil {
				return nil, err
			}
			log.Trace("Stored genesis voting snapshot to disk")
		} else {
			// 부모 블록에서 스냅샷 생성
			var err error
			if snap, err = e.snapshot(chain, number-1, parents[0].Hash(), parents[1:]); err != nil {
				return nil, err
			}

			// 헤더 적용
			header := chain.GetHeader(hash, number)
			if header == nil {
				return nil, errors.New("header not found")
			}

			// 헤더 적용
			snap, err = snap.apply(header)
			if err != nil {
				return nil, err
			}

			// 스냅샷 캐싱 및 저장
			e.lock.Lock()
			// e.recents.add(hash, snap)
			e.lock.Unlock()

			// 에포크 경계에서 스냅샷 저장
			if number%checkpointInterval == 0 {
				if err = snap.store(e.db); err != nil {
					log.Trace("Failed to store voting snapshot to disk", "err", err)
				} else {
					log.Trace("Stored voting snapshot to disk", "number", number, "hash", hash)
				}
			}
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
	proposalType string,
	parameters map[string]string,
	upgrade *utils.UpgradeInfo,
	funding *utils.FundingInfo,
	deposit *big.Int,
) (uint64, error) {
	e.lock.Lock()
	defer e.lock.Unlock()

	// 제안자가 검증자인지 확인
	if !e.validatorSet.Contains(proposer) {
		return 0, errors.New("proposer is not a validator")
	}

	// 현재 상태 가져오기
	currentBlock := e.currentBlock()
	if currentBlock == nil {
		return 0, errors.New("current block is nil")
	}

	// 상태 가져오기
	state, err := e.stateAt(currentBlock.Hash())
	if err != nil {
		return 0, err
	}

	// 제안 내용 생성
	var content utils.ProposalContentInterface
	switch proposalType {
	case utils.ProposalTypeParameter:
		// 매개변수 변경 제안
		content = &ParameterChangeProposal{
			Parameters: parameters,
		}
	case utils.ProposalTypeUpgrade:
		// 업그레이드 제안
		if upgrade == nil {
			return 0, errors.New("upgrade info is required for upgrade proposal")
		}
		content = &UpgradeProposal{
			UpgradeInfo: *upgrade,
		}
	case utils.ProposalTypeFunding:
		// 자금 지원 제안
		if funding == nil {
			return 0, errors.New("funding info is required for funding proposal")
		}
		// 자금이 충분한지 확인
		balance := state.GetBalance(funding.Recipient)
		if balance.ToBig().Cmp(funding.Amount) < 0 {
			return 0, errors.New("insufficient funds")
		}
		content = &FundingProposal{
			FundingInfo: *funding,
		}
	case utils.ProposalTypeText:
		// 텍스트 제안
		content = &TextProposal{}
	default:
		return 0, errors.New("invalid proposal type")
	}

	// 제안 제출
	return e.governance.SubmitProposal(proposer, title, description, proposalType, content, deposit, state)
}

// Vote는 거버넌스 제안에 투표합니다.
func (e *Eirene) Vote(
	proposalID uint64,
	voter common.Address,
	option string,
) error {
	e.lock.Lock()
	defer e.lock.Unlock()

	// 투표자가 검증자인지 확인
	if !e.validatorSet.Contains(voter) {
		return errors.New("voter is not a validator")
	}

	return e.governance.Vote(proposalID, voter, option)
}

// ProcessProposals는 현재 블록에서 제안을 처리합니다.
func (e *Eirene) ProcessProposals(currentBlock uint64) error {
	e.lock.Lock()
	defer e.lock.Unlock()

	// 현재 상태 가져오기
	current := e.currentBlock()
	if current == nil {
		return errors.New("current block is nil")
	}

	// 상태 가져오기
	state, err := e.stateAt(current.Hash())
	if err != nil {
		return err
	}

	// 모든 제안 가져오기
	proposals := e.governance.GetProposals()
	
	// 각 제안 처리
	for _, proposal := range proposals {
		// 투표 기간이 끝난 제안만 처리
		if proposal.GetVotingEndBlock() <= currentBlock && proposal.GetStatus() == utils.ProposalStatusVotingPeriod {
			// 제안 실행
			err := e.governance.ExecuteProposal(proposal.GetID(), state)
			if err != nil {
				log.Error("Failed to execute proposal", "id", proposal.GetID(), "error", err)
			}
		}
	}
	
	return nil
}

// ExecuteProposal은 통과된 제안을 실행합니다.
func (e *Eirene) ExecuteProposal(proposalID uint64) error {
	e.lock.Lock()
	defer e.lock.Unlock()

	// 현재 상태 가져오기
	current := e.currentBlock()
	if current == nil {
		return errors.New("current block is nil")
	}

	// 상태 가져오기
	state, err := e.stateAt(current.Hash())
	if err != nil {
		return err
	}

	// 제안 존재 여부 확인
	_, err = e.governance.GetProposal(proposalID)
	if err != nil {
		return err
	}

	// 제안 실행
	return e.governance.ExecuteProposal(proposalID, state)
}

// GetProposal은 특정 제안을 반환합니다.
func (e *Eirene) GetProposal(proposalID uint64) (utils.ProposalInterface, error) {
	e.lock.RLock()
	defer e.lock.RUnlock()

	return e.governance.GetProposal(proposalID)
}

// GetProposals는 모든 제안 목록을 반환합니다.
func (e *Eirene) GetProposals() []utils.ProposalInterface {
	e.lock.RLock()
	defer e.lock.RUnlock()

	return e.governance.GetProposals()
}

// GetVotes는 제안에 대한 투표 목록을 반환합니다.
func (e *Eirene) GetVotes(proposalID uint64) ([]ProposalVote, error) {
	e.lock.RLock()
	defer e.lock.RUnlock()

	// 제안 가져오기
	_, err := e.governance.GetProposal(proposalID)
	if err != nil {
		return nil, err
	}
	
	// 투표 정보 반환 로직 구현 필요
	// 현재는 빈 배열 반환
	return []ProposalVote{}, nil
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
func (e *Eirene) processSlashing(validatorSet utils.ValidatorSetInterface, slashingState *SlashingState, blockNumber uint64) {
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
func (e *Eirene) GetValidatorSet() utils.ValidatorSetInterface {
	return e.validatorSet
}

// GetGovernanceState는 Eirene 합의 엔진의 거버넌스 상태를 반환합니다.
func (e *Eirene) GetGovernanceState() utils.GovernanceInterface {
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

// GetSigner는 서명자 주소를 반환합니다.
// 주로 테스트 목적으로 사용됩니다.
func (e *Eirene) GetSigner() common.Address {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.signer
}

// GetSignerFn은 서명 함수를 반환합니다.
// 주로 테스트 목적으로 사용됩니다.
func (e *Eirene) GetSignerFn() SignerFn {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.signFn
}

// SetSigner는 서명자 주소를 설정합니다.
func (e *Eirene) SetSigner(addr common.Address) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.signer = addr
}

// SetSignerFn은 서명 함수를 설정합니다.
func (e *Eirene) SetSignerFn(fn SignerFn) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.signFn = fn
}

// recentBlocks는 최근 서명된 블록의 캐시를 관리합니다
type recentBlocks struct {
	items map[uint64]common.Address
	lock  sync.RWMutex
}

// newRecentBlocks는 새로운 recentBlocks 인스턴스를 생성합니다
func newRecentBlocks() *recentBlocks {
	return &recentBlocks{
		items: make(map[uint64]common.Address),
	}
}

// add는 블록 번호와 서명자를 캐시에 추가합니다
func (r *recentBlocks) add(blockNumber uint64, signer common.Address) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.items[blockNumber] = signer
}

// get은 블록 번호에 해당하는 서명자를 반환합니다
func (r *recentBlocks) get(blockNumber uint64) (common.Address, bool) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	signer, ok := r.items[blockNumber]
	return signer, ok
}

// saveGovernanceState는 거버넌스 상태를 데이터베이스에 저장합니다.
func (e *Eirene) saveGovernanceState() error {
	// 거버넌스 상태 저장 로직 구현
	// 현재는 단순히 성공 반환
	// 실제 구현에서는 거버넌스 상태를 직렬화하여 데이터베이스에 저장해야 합니다.
	return nil
}
