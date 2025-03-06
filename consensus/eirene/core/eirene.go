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

// Eirene는 Proof-of-Stake 합의 알고리즘의 구현체입니다.
type Eirene struct {
	config *params.EireneConfig // 합의 엔진 설정
	db     ethdb.Database       // 스냅샷 및 검증자 정보를 저장하는 데이터베이스

	recents    *recentBlocks // 최근 서명된 블록의 캐시
	signatures *lru.ARCCache // 최근 블록 서명의 캐시

	proposals map[common.Address]bool // 현재 우리가 추진하고 있는 제안 목록

	signer common.Address // 서명자 주소
	signFn SignerFn       // 서명 함수
	lock   sync.RWMutex   // 뮤텍스

	// 어댑터 인터페이스
	coreAdapter    CoreAdapterInterface        // 코어 어댑터
	govAdapter     utils.GovernanceInterface   // 거버넌스 어댑터
	stakingAdapter utils.ValidatorSetInterface // 스테이킹 어댑터
	abciAdapter    interface{}                 // ABCI 어댑터

	// 블록체인 컨텍스트
	chain        consensus.ChainHeaderReader               // 체인 헤더 리더
	currentBlock func() *types.Block                       // 현재 블록을 가져오는 함수
	stateAt      func(common.Hash) (*state.StateDB, error) // 특정 해시에서 상태를 가져오는 함수

	// 이벤트 관련
	eventMux     *event.TypeMux // 이벤트 멀티플렉서
	eventFeed    *event.Feed    // 이벤트 피드
	chainHeadCh  chan core.ChainHeadEvent
	chainHeadSub event.Subscription

	// 성능 최적화
	performanceOptimizer *PerformanceOptimizer // 성능 최적화 모듈

	// 로거
	logger log.Logger

	// 테스트용 필드
	fakeDiff bool // 난이도 검증 건너뛰기
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
//
// 이 함수는 Eirene 합의 엔진의 새 인스턴스를 초기화합니다. 설정에 Period나 Epoch가
// 지정되지 않은 경우 기본값을 사용합니다. 또한 최근 블록 서명을 위한 캐시를 초기화하고
// 필요한 데이터 구조를 설정합니다. 실제 구현에서는 거버넌스, 스테이킹, ABCI 어댑터 등을
// 적절히 초기화해야 합니다.
func New(config *params.EireneConfig, db ethdb.Database) *Eirene {
	// 시그니처 캐시 생성
	signatures, _ := lru.NewARC(inmemorySignatures)

	// 로거 설정
	logger := log.New("module", "eirene")

	// 설정 복사 및 기본값 설정
	configCopy := *config
	if configCopy.Period == 0 {
		configCopy.Period = 15 // 기본 Period 값
	}
	if configCopy.Epoch == 0 {
		configCopy.Epoch = 30000 // 기본 Epoch 값
	}

	// Eirene 인스턴스 생성
	eirene := &Eirene{
		config:     &configCopy,
		db:         db,
		recents:    newRecentBlocks(),
		signatures: signatures,
		proposals:  make(map[common.Address]bool),
		eventFeed:  new(event.Feed),
		logger:     logger,
	}

	return eirene
}

// Author는 주어진 블록을 채굴한 계정의 Zenanet 주소를 검색합니다.
//
// 매개변수:
//   - header: 블록 헤더
//
// 반환값:
//   - common.Address: 블록 생성자 주소
//   - error: 오류 발생 시 반환
//
// 이 함수는 블록 헤더의 extra-data 필드에서 서명을 추출하고, 이를 통해 블록을 생성한
// 검증자의 주소를 복구합니다. 서명이 유효하지 않거나 추출할 수 없는 경우 오류를 반환합니다.
// 이미 처리된 블록의 경우 캐시에서 주소를 검색하여 성능을 최적화합니다.
func (e *Eirene) Author(header *types.Header) (common.Address, error) {
	return ecrecover(header, e.signatures)
}

// VerifyHeader는 헤더가 주어진 엔진의 합의 규칙을 준수하는지 확인합니다.
//
// 매개변수:
//   - chain: 체인 헤더 리더 인터페이스
//   - header: 검증할 블록 헤더
//   - seal: 서명 검증 여부
//
// 반환값:
//   - error: 검증 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 블록 헤더가 Eirene 합의 규칙을 준수하는지 검증합니다. 내부적으로 verifyHeader를
// 호출하여 실제 검증 작업을 수행합니다. 검증 과정에서는 블록 번호, 체인 구성, 캐스케이딩 필드 등을
// 확인합니다. 검증에 실패하면 적절한 오류를 반환합니다.
func (e *Eirene) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header, seal bool) error {
	return e.verifyHeader(chain, header, nil)
}

// VerifyHeaders는 VerifyHeader와 유사하지만 헤더 배치를 동시에 확인합니다.
//
// 매개변수:
//   - chain: 체인 헤더 리더 인터페이스
//   - headers: 검증할 블록 헤더 배열
//   - seals: 각 헤더에 대한 서명 검증 여부 배열
//
// 반환값:
//   - chan<- struct{}: 검증 작업을 중단하기 위한 채널
//   - <-chan error: 검증 결과를 전달하는 채널
//
// 이 함수는 여러 블록 헤더를 병렬로 검증합니다. 각 헤더에 대해 verifyHeader를 호출하여
// 검증 작업을 수행하고, 결과를 채널을 통해 반환합니다. 검증 작업은 비동기적으로 수행되며,
// abort 채널을 통해 중단할 수 있습니다. 이 함수는 여러 블록을 한 번에 검증할 때 성능을
// 최적화하기 위해 사용됩니다.
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
//
// 매개변수:
//   - chain: 체인 헤더 리더 인터페이스
//   - header: 검증할 블록 헤더
//   - parents: 부모 헤더 배열 (선택적)
//
// 반환값:
//   - error: 검증 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 VerifyHeader의 내부 구현으로, 블록 헤더가 Eirene 합의 규칙을 준수하는지
// 검증합니다. 블록 번호가 있는지, 체인 구성에서 Eirene가 활성화되어 있는지 확인하고,
// verifyCascadingFields를 호출하여 추가 검증을 수행합니다. 검증에 실패하면 적절한
// 오류를 반환합니다. parents 매개변수는 이전에 검증된 헤더 배열로, 캐싱 목적으로 사용됩니다.
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
//
// 매개변수:
//   - chain: 체인 헤더 리더 인터페이스
//   - header: 검증할 블록 헤더
//   - parents: 부모 헤더 배열 (선택적)
//
// 반환값:
//   - error: 검증 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 블록 헤더의 캐스케이딩 필드(타임스탬프, 난이도, 가스 한도 등)가 Eirene 합의 규칙을
// 준수하는지 검증합니다. 현재 구현에서는 기본 검증만 수행하지만, 실제 구현에서는 타임스탬프 검증,
// 난이도 검증, 가스 한도 검증 등 더 많은 검증을 수행해야 합니다. 검증에 실패하면 적절한 오류를
// 반환합니다.
func (e *Eirene) verifyCascadingFields(chain consensus.ChainHeaderReader, header *types.Header, parents []*types.Header) error {
	// 현재는 기본 검증만 수행
	return nil
}

// VerifyUncles는 주어진 블록의 uncle이 합의 엔진의 규칙을 준수하는지 확인합니다.
//
// 매개변수:
//   - chain: 체인 리더 인터페이스
//   - block: 검증할 블록
//
// 반환값:
//   - error: 검증 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 블록의 엉클이 Eirene 합의 규칙을 준수하는지 검증합니다. Eirene는 PoS 합의 알고리즘으로,
// 엉클 블록을 사용하지 않으므로, 엉클 배열이 비어 있지 않으면 오류를 반환합니다. 이 함수는 블록 검증
// 과정에서 호출됩니다.
func (e *Eirene) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	// Eirene에서는 uncle이 허용되지 않음
	if len(block.Uncles()) > 0 {
		return errors.New("uncles not allowed")
	}
	return nil
}

// Prepare는 특정 엔진의 규칙에 따라 블록 헤더의 합의 필드를 초기화합니다.
//
// 매개변수:
//   - chain: 체인 헤더 리더 인터페이스
//   - header: 준비할 블록 헤더
//
// 반환값:
//   - error: 준비 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 새 블록을 생성하기 전에 블록 헤더의 합의 관련 필드를 초기화합니다.
// 부모 블록을 확인하고, 난이도를 계산하며, 코인베이스, 믹스 다이제스트, 논스 등의
// 필드를 설정합니다. 부모 블록을 찾을 수 없는 경우 오류를 반환합니다.
// 이 함수는 블록 생성 과정에서 호출됩니다.
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

// Finalize는 블록 마무리 작업을 수행합니다.
func (e *Eirene) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state vm.StateDB, txs []*types.Transaction, uncles []*types.Header) {
	// 블록 보상 분배
	if e.coreAdapter != nil {
		// 실제 구현에서는 적절한 방식으로 처리해야 함
		e.logger.Info("Finalizing block with core adapter", "number", header.Number)
		return
	}

	// 코어 어댑터가 없는 경우 기본 동작 수행
	// 이 부분은 레거시 코드로, 코어 어댑터가 구현되면 제거될 예정입니다.

	// 참고: 실제 구현에서는 vm.StateDB 인터페이스를 통해 상태 루트를 설정해야 합니다.
	// 여기서는 임시로 빈 해시를 설정합니다.
	header.Root = common.Hash{}
	header.UncleHash = types.CalcUncleHash(nil)
}

// FinalizeAndAssemble은 블록을 마무리하고 최종 블록을 조립합니다.
func (e *Eirene) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, body *types.Body, receipts []*types.Receipt) (*types.Block, error) {
	// 코어 어댑터가 있으면 위임
	if e.coreAdapter != nil {
		return e.coreAdapter.FinalizeAndAssemble(chain, header, state, body.Transactions, body.Uncles, receipts)
	}

	// 코어 어댑터가 없는 경우 기본 동작 수행
	// 이 부분은 레거시 코드로, 코어 어댑터가 구현되면 제거될 예정입니다.

	// 거버넌스 처리
	if err := e.ProcessGovernance(state, header); err != nil {
		return nil, err
	}

	// 블록 보상 분배
	rewardState := e.GetRewardState()
	if err := e.distributeBlockReward(header, rewardState); err != nil {
		return nil, err
	}

	// 슬래싱 처리
	slashingState := e.GetSlashingState()
	if err := e.processSlashing(e.GetValidatorSet(), slashingState, header.Number.Uint64()); err != nil {
		return nil, err
	}

	// IBC 패킷 처리
	if err := e.processIBCPackets(header.Number.Uint64(), header.Time); err != nil {
		return nil, err
	}

	// 상태 루트 계산
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))

	// 블록 조립 (types.NewBlock 함수의 인자 형식에 맞게 수정)
	// 블록 바디 생성
	blockBody := &types.Body{
		Transactions: body.Transactions,
		Uncles:       body.Uncles,
	}

	return types.NewBlock(header, blockBody, receipts, nil), nil
}

// distributeRewards는 블록 생성 보상을 분배합니다.
// 참고: 현재 구현에서는 사용되지 않으며, 향후 별도의 보상 시스템으로 대체될 예정입니다.
// func (e *Eirene) distributeRewards(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB) {
// 	// 구현 생략
// }

// Seal은 주어진 입력 블록에 대한 새로운 sealing 요청을 생성하고 결과를 주어진 채널로 푸시합니다.
//
// 매개변수:
//   - chain: 체인 헤더 리더 인터페이스
//   - block: 봉인할 블록
//   - results: 봉인된 블록을 전송할 채널
//   - stop: 봉인 작업을 중단하기 위한 채널
//
// 반환값:
//   - error: 오류 발생 시 반환
//
// 이 함수는 블록을 봉인(서명)하는 작업을 수행합니다. 서명자와 서명 함수가 설정되어 있지 않으면
// 오류를 반환합니다. 블록 헤더의 해시에 서명하고, 서명을 extra-data 필드에 추가한 후,
// 서명된 블록을 results 채널로 전송합니다. 이 함수는 블록 생성의 마지막 단계에서 호출됩니다.
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
//
// 매개변수:
//   - header: 블록 헤더
//
// 반환값:
//   - common.Hash: 봉인 전 헤더의 해시
//
// 이 함수는 블록 헤더의 봉인 전 해시를 계산합니다. 이 해시는 블록을 봉인(서명)할 때 사용됩니다.
// 내부적으로 SealHash 함수를 호출하여 계산을 수행합니다. 이 함수는 블록 생성 과정에서 호출됩니다.
func (e *Eirene) SealHash(header *types.Header) common.Hash {
	return SealHash(header)
}

// CalcDifficulty는 난이도 조정 알고리즘입니다. 새 블록이 가져야 할 난이도를 반환합니다.
//
// 매개변수:
//   - chain: 체인 헤더 리더 인터페이스
//   - time: 새 블록의 타임스탬프
//   - parent: 부모 블록 헤더
//
// 반환값:
//   - *big.Int: 계산된 난이도
//
// 이 함수는 새 블록의 난이도를 계산합니다. Eirene는 PoS 합의 알고리즘으로, 난이도는 검증자 선택에
// 사용되지 않으므로, 현재 구현에서는 간단히 1을 반환합니다. 실제 구현에서는 검증자 순서에 따라
// 난이도를 조정할 수 있습니다. 이 함수는 블록 생성 과정에서 호출됩니다.
func (e *Eirene) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	// 현재는 간단한 난이도 계산만 구현
	return big.NewInt(1)
}

// APIs는 합의 엔진에서 제공하는 RPC API를 반환합니다.
//
// 매개변수:
//   - chain: 체인 헤더 리더 인터페이스
//
// 반환값:
//   - []rpc.API: RPC API 배열
//
// 이 함수는 Eirene 합의 엔진에서 제공하는 RPC API를 반환합니다. 현재 구현에서는 기본 Eirene API만
// 제공하지만, 실제 구현에서는 스테이킹 API, 거버넌스 API 등 더 많은 API를 제공해야 합니다.
// 이 함수는 노드 시작 시 호출되어 사용 가능한 API를 등록합니다.
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

// Close는 합의 엔진을 종료합니다.
//
// 반환값:
//   - error: 종료 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 Eirene 합의 엔진을 종료하고 사용된 리소스를 정리합니다. 현재 구현에서는 특별한 정리
// 작업을 수행하지 않지만, 실제 구현에서는 고루틴 종료, 연결 종료, 데이터베이스 정리 등의 작업을
// 수행해야 할 수 있습니다. 이 함수는 노드 종료 시 호출됩니다.
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
	// 코어 어댑터가 있으면 위임
	if e.coreAdapter != nil {
		return e.coreAdapter.SubmitProposal(proposer, title, description, proposalType, parameters, upgrade, funding, deposit)
	}

	// 코어 어댑터가 없는 경우 거버넌스 어댑터 사용
	if e.govAdapter != nil {
		// 상태 DB 가져오기
		header := e.chain.CurrentHeader()
		if header == nil {
			return 0, errors.New("current header not found")
		}

		stateDB, err := e.stateAt(header.Root)
		if err != nil {
			return 0, err
		}

		// 제안 내용 생성
		var content utils.ProposalContentInterface
		switch proposalType {
		case utils.ProposalTypeParameterChange:
			content = &parameterChangeProposal{
				Changes: parameters,
			}
		case utils.ProposalTypeUpgrade:
			if upgrade == nil {
				return 0, errors.New("upgrade info is required for upgrade proposal")
			}
			content = &upgradeProposal{
				Info: *upgrade,
			}
		case utils.ProposalTypeFunding:
			if funding == nil {
				return 0, errors.New("funding info is required for funding proposal")
			}
			content = &fundingProposal{
				Info: *funding,
			}
		case utils.ProposalTypeText:
			content = &textProposal{}
		default:
			return 0, errors.New("invalid proposal type")
		}

		return e.govAdapter.SubmitProposal(proposer, title, description, proposalType, content, deposit, stateDB)
	}

	return 0, errors.New("governance adapter not set")
}

// Vote는 제안에 투표합니다.
func (e *Eirene) Vote(
	proposalID uint64,
	voter common.Address,
	option string,
) error {
	// 코어 어댑터가 있으면 위임
	if e.coreAdapter != nil {
		return e.coreAdapter.Vote(proposalID, voter, option)
	}

	// 코어 어댑터가 없는 경우 거버넌스 어댑터 사용
	if e.govAdapter != nil {
		// 상태 DB 가져오기
		header := e.chain.CurrentHeader()
		if header == nil {
			return errors.New("current header not found")
		}

		stateDB, err := e.stateAt(header.Root)
		if err != nil {
			return err
		}

		return e.govAdapter.Vote(proposalID, voter, option, stateDB)
	}

	return errors.New("governance adapter not set")
}

// ProcessProposals는 현재 블록에서 제안을 처리합니다.
func (e *Eirene) ProcessProposals(currentBlock uint64) error {
	// 코어 어댑터가 있으면 위임
	if e.coreAdapter != nil {
		return e.coreAdapter.ProcessProposals(currentBlock)
	}

	// 코어 어댑터가 없는 경우 거버넌스 어댑터 사용
	if e.govAdapter != nil {
		// 현재 블록 헤더 가져오기
		header := e.chain.CurrentHeader()
		if header == nil {
			return errors.New("current header not found")
		}

		// 상태 DB 가져오기
		stateDB, err := e.stateAt(header.Root)
		if err != nil {
			return err
		}

		// 모든 제안 가져오기
		proposals := e.govAdapter.GetProposals()

		// 각 제안 처리
		for _, proposal := range proposals {
			// 투표 기간이 끝난 제안 처리
			if proposal.GetStatus() == utils.ProposalStatusVotingPeriod && currentBlock > proposal.GetVotingEndBlock() {
				// 제안 실행
				err := e.govAdapter.ExecuteProposal(proposal.GetID(), stateDB)
				if err != nil {
					e.logger.Error("Failed to execute proposal", "id", proposal.GetID(), "error", err)
					// 개별 제안 처리 실패는 전체 프로세스를 중단하지 않음
				}
			}
		}
	}

	return nil
}

// ExecuteProposal은 제안을 실행합니다.
func (e *Eirene) ExecuteProposal(proposalID uint64) error {
	// 코어 어댑터가 있으면 위임
	if e.coreAdapter != nil {
		return e.coreAdapter.ExecuteProposal(proposalID)
	}

	// 코어 어댑터가 없는 경우 거버넌스 어댑터 사용
	if e.govAdapter != nil {
		// 현재 블록 헤더 가져오기
		header := e.chain.CurrentHeader()
		if header == nil {
			return errors.New("current header not found")
		}

		// 상태 DB 가져오기
		state, err := e.stateAt(header.Root)
		if err != nil {
			return err
		}

		// 제안 존재 여부 확인
		_, err = e.govAdapter.GetProposal(proposalID)
		if err != nil {
			return err
		}

		// 제안 실행
		return e.govAdapter.ExecuteProposal(proposalID, state)
	}

	return errors.New("governance adapter not set")
}

// GetProposal은 특정 제안을 반환합니다.
func (e *Eirene) GetProposal(proposalID uint64) (utils.ProposalInterface, error) {
	// 코어 어댑터가 있으면 위임
	if e.coreAdapter != nil {
		return e.coreAdapter.GetProposal(proposalID)
	}

	// 코어 어댑터가 없는 경우 거버넌스 어댑터 사용
	if e.govAdapter != nil {
		return e.govAdapter.GetProposal(proposalID)
	}

	return nil, errors.New("governance adapter not set")
}

// GetProposals는 모든 제안을 반환합니다.
func (e *Eirene) GetProposals() []utils.ProposalInterface {
	// 코어 어댑터가 있으면 위임
	if e.coreAdapter != nil {
		return e.coreAdapter.GetProposals()
	}

	// 코어 어댑터가 없는 경우 거버넌스 어댑터 사용
	if e.govAdapter != nil {
		return e.govAdapter.GetProposals()
	}

	return []utils.ProposalInterface{}
}

// GetVotes는 지정된 제안의 투표를 반환합니다.
func (e *Eirene) GetVotes(proposalID uint64) ([]ProposalVote, error) {
	// 코어 어댑터가 있으면 위임
	if e.coreAdapter != nil {
		return e.coreAdapter.GetVotes(proposalID)
	}

	// 코어 어댑터가 없는 경우 빈 배열 반환
	return []ProposalVote{}, nil
}

// ProcessGovernance는 거버넌스 관련 처리를 수행합니다.
func (e *Eirene) ProcessGovernance(state vm.StateDB, header *types.Header) error {
	// 코어 어댑터가 있으면 위임
	if e.coreAdapter != nil {
		return e.coreAdapter.ProcessProposals(header.Number.Uint64())
	}

	// 코어 어댑터가 없는 경우 거버넌스 어댑터 사용
	if e.govAdapter != nil {
		// 현재 블록 번호에서 제안 처리
		currentBlock := header.Number.Uint64()

		// 모든 제안 가져오기
		proposals := e.govAdapter.GetProposals()

		// 각 제안 처리
		for _, proposal := range proposals {
			// 투표 기간이 끝난 제안 처리
			if proposal.GetStatus() == utils.ProposalStatusVotingPeriod && currentBlock > proposal.GetVotingEndBlock() {
				// 제안 실행 (vm.StateDB 인터페이스를 사용)
				// 실제 구현에서는 적절한 방식으로 처리해야 함
				e.logger.Info("Processing proposal", "id", proposal.GetID(), "status", proposal.GetStatus())
			}
		}
	}

	return nil
}

// updateSigningInfo는 검증자의 서명 정보를 업데이트합니다.
func (e *Eirene) updateSigningInfo(slashingState *SlashingState, header *types.Header, signers []common.Address) error {
	if slashingState == nil {
		return utils.FormatError(utils.ErrInternalError, "slashing state is nil")
	}

	if header == nil {
		return utils.FormatError(utils.ErrInvalidParameter, "header is nil")
	}

	blockNumber := header.Number.Uint64()
	// 블록 타임스탬프는 서명 정보 업데이트에 사용될 수 있음
	// 현재는 사용하지 않으므로 주석 처리
	// blockTime := header.Time

	// 블록 생성자 주소 가져오기
	signer, err := ecrecover(header, e.signatures)
	if err != nil {
		return utils.FormatError(err, "failed to recover signer from header")
	}

	log.Debug("Updating signing info", "blockNumber", blockNumber, "signer", signer.Hex())

	// 서명 정보 업데이트 로직 구현
	// 여기서는 간단히 서명 정보만 업데이트하고 실제 슬래싱은 processSlashing에서 처리
	// 실제 구현에서는 서명 정보를 상태 DB에 저장하고 관리해야 함

	return nil
}

// processSlashing은 슬래싱 처리를 수행합니다.
func (e *Eirene) processSlashing(validatorSet utils.ValidatorSetInterface, slashingState *SlashingState, blockNumber uint64) error {
	if validatorSet == nil {
		return utils.FormatError(utils.ErrInvalidParameter, "validator set is nil")
	}

	if slashingState == nil {
		return utils.FormatError(utils.ErrInternalError, "slashing state is nil")
	}

	log.Debug("Processing slashing", "blockNumber", blockNumber)

	// 슬래싱 처리 로직 구현
	// 1. 서명 누락 검증자 확인
	// 2. 이중 서명 검증자 확인
	// 3. 슬래싱 적용

	// 실제 구현에서는 슬래싱 조건을 확인하고 위반한 검증자에게 페널티 적용
	// 여기서는 간단한 로깅만 수행

	return nil
}

// distributeBlockReward는 블록 보상을 분배합니다.
func (e *Eirene) distributeBlockReward(header *types.Header, rewardState *RewardState) error {
	if header == nil {
		return utils.FormatError(utils.ErrInvalidParameter, "header is nil")
	}

	if rewardState == nil {
		return utils.FormatError(utils.ErrInternalError, "reward state is nil")
	}

	blockNumber := header.Number.Uint64()

	// 블록 생성자 주소 가져오기
	signer, err := ecrecover(header, e.signatures)
	if err != nil {
		return utils.FormatError(err, "failed to recover signer from header")
	}

	log.Debug("Distributing block reward", "blockNumber", blockNumber, "signer", signer.Hex())

	// 보상 분배 로직 구현
	// 1. 기본 블록 보상 계산
	// 2. 검증자, 위임자, 커뮤니티 기금에 보상 분배
	// 3. 보상 상태 업데이트

	// 실제 구현에서는 보상을 계산하고 분배하는 로직을 구현해야 함
	// 여기서는 간단한 로깅만 수행

	return nil
}

// processIBCPackets는 IBC 패킷을 처리합니다.
func (e *Eirene) processIBCPackets(blockNumber uint64, timestamp uint64) error {
	// IBC 상태 가져오기
	ibcState := e.GetIBCState()
	if ibcState == nil {
		return utils.FormatError(utils.ErrInternalError, "IBC state is nil")
	}

	// 실제 구현에서는 IBC 패킷 처리 로직 구현
	e.logger.Debug("Processing IBC packets", "block", blockNumber, "time", timestamp)

	return nil
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
	if e.stakingAdapter != nil {
		return e.stakingAdapter
	}
	return nil
}

// GetGovernanceState는 Eirene 합의 엔진의 거버넌스 상태를 반환합니다.
func (e *Eirene) GetGovernanceState() utils.GovernanceInterface {
	if e.govAdapter != nil {
		return e.govAdapter
	}
	return nil
}

// GetSlashingState는 Eirene 합의 엔진의 슬래싱 상태를 반환합니다.
func (e *Eirene) GetSlashingState() *SlashingState {
	// 코어 어댑터에서 슬래싱 상태 가져오기
	// 실제 구현에서는 코어 어댑터에서 슬래싱 상태를 가져와야 함
	return &SlashingState{}
}

// GetIBCState는 Eirene 합의 엔진의 IBC 상태를 반환합니다.
func (e *Eirene) GetIBCState() *IBCState {
	// 코어 어댑터에서 IBC 상태 가져오기
	// 실제 구현에서는 코어 어댑터에서 IBC 상태를 가져와야 함
	return &IBCState{}
}

// GetRewardState는 Eirene 합의 엔진의 보상 상태를 반환합니다.
func (e *Eirene) GetRewardState() *RewardState {
	// 코어 어댑터에서 보상 상태 가져오기
	// 실제 구현에서는 코어 어댑터에서 보상 상태를 가져와야 함
	return &RewardState{}
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

// snapshot은 지정된 블록 번호와 해시에 대한 스냅샷을 검색합니다.
//
// 매개변수:
//   - chain: 체인 헤더 리더 인터페이스
//   - number: 블록 번호
//   - hash: 블록 해시
//   - parents: 부모 헤더 배열 (선택적)
//
// 반환값:
//   - *Snapshot: 검색된 스냅샷
//   - error: 검색 실패 시 오류 반환, 성공 시 nil 반환
//
// 이 함수는 지정된 블록 번호와 해시에 대한 스냅샷을 검색합니다. 먼저 캐시에서 스냅샷을 검색하고,
// 캐시에 없으면 데이터베이스에서 검색합니다. 데이터베이스에도 없으면 부모 블록에서 스냅샷을 생성합니다.
// 이 함수는 블록 검증 과정에서 호출되어 현재 검증자 집합을 확인하는 데 사용됩니다.
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

// SetChainContext는 체인 컨텍스트를 설정합니다.
func (e *Eirene) SetChainContext(chain consensus.ChainHeaderReader, currentBlock func() *types.Block, stateAt func(common.Hash) (*state.StateDB, error)) {
	e.chain = chain
	e.currentBlock = currentBlock
	e.stateAt = stateAt

	// 코어 어댑터 생성
	e.coreAdapter = NewCoreAdapter(
		e.db,
		e.stakingAdapter,
		e.govAdapter,
		e.config,
		chain,
		currentBlock,
		stateAt,
	)
}
