# Eirene 개발자 가이드

이 문서는 Eirene 합의 알고리즘을 개발하고 확장하기 위한 가이드입니다. Eirene는 Zenanet 블록체인을 위한 Proof-of-Stake(PoS) 합의 알고리즘으로, Cosmos SDK와 Tendermint의 합의 메커니즘을 기반으로 합니다.

## 목차

1. [아키텍처 개요](#아키텍처-개요)
2. [개발 환경 설정](#개발-환경-설정)
3. [주요 모듈 설명](#주요-모듈-설명)
4. [API 사용 가이드](#api-사용-가이드)
5. [테스트 작성 가이드](#테스트-작성-가이드)
6. [기여 가이드](#기여-가이드)
7. [문제 해결](#문제-해결)
8. [API 문서 생성](#api-문서-생성)

## 아키텍처 개요

Eirene 합의 알고리즘은 다음과 같은 주요 모듈로 구성되어 있습니다:

```consensus/eirene/
├── core/           # 합의 엔진의 핵심 기능
├── governance/     # 온체인 거버넌스 시스템
├── staking/        # 검증자 관리 및 스테이킹
├── abci/           # Tendermint ABCI 어댑터
├── ibc/            # IBC 프로토콜 구현
└── utils/          # 공통 유틸리티 및 인터페이스
```

### 주요 컴포넌트 간 상호작용

```
                  ┌─────────────┐
                  │   Eirene    │
                  │   (core)    │
                  └─────┬───────┘
                        │
        ┌───────┬───────┼───────┬───────┐
        │       │       │       │       │
┌───────▼─┐ ┌───▼───┐ ┌─▼───┐ ┌─▼────┐ ┌▼────┐
│Governance│ │Staking│ │ABCI │ │ IBC  │ │Utils│
└───────┬─┘ └───┬───┘ └─┬───┘ └─┬────┘ └┬────┘
        │       │       │       │       │
        └───────┴───────┼───────┴───────┘
                        │
                  ┌─────▼─────┐
                  │  Zenanet  │
                  │ Blockchain│
                  └───────────┘
```

## 개발 환경 설정

### 필수 요구 사항

- Go 1.18 이상
- Git
- Make

### 개발 환경 설정 단계

1. 저장소 클론:
   ```bash
   git clone https://github.com/zenanetwork/go-zenanet.git
   cd go-zenanet
   ```

2. 의존성 설치:
   ```bash
   go mod download
   ```

3. 빌드:
   ```bash
   make all
   ```

4. 테스트:
   ```bash
   go test ./consensus/eirene/...
   ```

## 주요 모듈 설명

### Core 모듈

Core 모듈은 Eirene 합의 알고리즘의 핵심 기능을 구현합니다. 주요 파일 및 구성 요소는 다음과 같습니다:

- `eirene.go`: 합의 엔진의 주요 구현
- `api.go`: 기본 RPC API 구현
- `snapshot.go`: 검증자 상태 관리를 위한 스냅샷 시스템
- `types.go`: 핵심 타입 정의

#### 주요 구조체 및 인터페이스

- `Eirene`: 합의 엔진의 주요 구조체
- `Snapshot`: 검증자 상태의 스냅샷
- `SignerFn`: 서명 함수 타입

#### 주요 메서드

- `New(config *params.EireneConfig, db ethdb.Database) *Eirene`: 새로운 Eirene 합의 엔진 생성
- `Author(header *types.Header) (common.Address, error)`: 블록 작성자 주소 반환
- `VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header, seal bool) error`: 헤더 검증
- `Seal(chain consensus.ChainHeaderReader, block *types.Block, results chan<- *types.Block, stop <-chan struct{}) error`: 블록 서명

### Governance 모듈

Governance 모듈은 온체인 거버넌스 시스템을 구현합니다. 주요 파일 및 구성 요소는 다음과 같습니다:

- `governance.go`: 기본 거버넌스 시스템 구현
- `governance_enhanced.go`: 확장된 거버넌스 기능 구현
- `gov_api.go`: 거버넌스 관련 RPC API 구현
- `gov_adapter.go`: Cosmos SDK 거버넌스 모듈과의 연동

#### 주요 구조체 및 인터페이스

- `GovernanceManager`: 거버넌스 관리자
- `Proposal`: 제안 구조체
- `ProposalContent`: 제안 내용 인터페이스
- `GovernanceParams`: 거버넌스 매개변수

#### 주요 메서드

- `SubmitProposal(proposalType string, title string, description string, proposer common.Address, content ProposalContent, initialDeposit *big.Int, state *state.StateDB) (uint64, error)`: 제안 제출
- `Vote(proposalID uint64, voter common.Address, option string) error`: 제안에 투표
- `ExecuteProposal(proposalID uint64, state *state.StateDB) error`: 제안 실행

### Staking 모듈

Staking 모듈은 검증자 관리, 스테이킹, 위임, 보상 분배, 슬래싱 등의 기능을 구현합니다. 주요 파일 및 구성 요소는 다음과 같습니다:

- `validator.go`: 기본 검증자 선택 및 관리 시스템 구현
- `slashing.go`: 슬래싱 메커니즘 구현
- `reward.go`: 보상 분배 시스템 구현
- `staking_adapter.go`: Cosmos SDK 스테이킹 모듈과의 연동

#### 주요 구조체 및 인터페이스

- `ValidatorSet`: 검증자 집합
- `Validator`: 검증자 구조체
- `SlashingAdapter`: 슬래싱 어댑터
- `RewardAdapter`: 보상 어댑터

#### 주요 메서드

- `Stake(state *state.StateDB, address common.Address, amount *big.Int, pubKey []byte, description ValidatorDescription, commission *big.Int) error`: 토큰 스테이킹
- `Unstake(state *state.StateDB, address common.Address) error`: 스테이킹 해제
- `Delegate(state *state.StateDB, delegator common.Address, validator common.Address, amount *big.Int) error`: 토큰 위임
- `Undelegate(state *state.StateDB, delegator common.Address, validator common.Address, amount *big.Int) error`: 위임 해제

### ABCI 모듈

ABCI 모듈은 Tendermint의 ABCI(Application BlockChain Interface)와의 통합을 위한 어댑터를 구현합니다. 주요 파일 및 구성 요소는 다음과 같습니다:

- `abci_adapter.go`: Tendermint의 ABCI 인터페이스와 go-zenanet 연결

#### 주요 구조체 및 인터페이스

- `ABCIAdapter`: ABCI 어댑터
- `Validator`: ABCI 검증자

#### 주요 메서드

- `ConvertHeader(header *types.Header) *tmtypes.Header`: 헤더 변환
- `ConvertBlock(block *types.Block) *tmtypes.Block`: 블록 변환
- `ProcessBlock(chain consensus.ChainHeaderReader, block *types.Block, state *state.StateDB) error`: 블록 처리

### IBC 모듈

IBC 모듈은 IBC(Inter-Blockchain Communication) 프로토콜을 구현합니다. 주요 파일 및 구성 요소는 다음과 같습니다:

- `ibc.go`: IBC 프로토콜 구현

#### 주요 구조체 및 인터페이스

- `IBCState`: IBC 상태
- `IBCClient`: IBC 클라이언트
- `IBCConnection`: IBC 연결
- `IBCChannel`: IBC 채널
- `IBCPacket`: IBC 패킷

#### 주요 메서드

- `createClient(id string, clientType string, consensusState []byte, trustingPeriod uint64) (*IBCClient, error)`: 클라이언트 생성
- `createConnection(id string, clientID string, counterpartyClientID string, counterpartyConnectionID string, version string) (*IBCConnection, error)`: 연결 생성
- `createChannel(portID string, channelID string, connectionID string, counterpartyPortID string, counterpartyChannelID string, version string) (*IBCChannel, error)`: 채널 생성
- `sendPacket(sourcePort string, sourceChannel string, destPort string, destChannel string, data []byte, timeoutHeight uint64, timeoutTimestamp uint64) (*IBCPacket, error)`: 패킷 전송

### Utils 모듈

Utils 모듈은 여러 모듈에서 공통으로 사용하는 유틸리티 함수와 구조체를 제공합니다. 주요 파일 및 구성 요소는 다음과 같습니다:

- `types.go`: 공통 타입 정의 및 인터페이스 선언

#### 주요 구조체 및 인터페이스

- `GovernanceInterface`: 거버넌스 인터페이스
- `ValidatorSetInterface`: 검증자 집합 인터페이스
- `ValidatorInterface`: 검증자 인터페이스
- `ProposalInterface`: 제안 인터페이스

## API 사용 가이드

### Core API

#### 합의 엔진 생성 및 설정

```go
// 합의 엔진 생성
config := &params.EireneConfig{
    Period: 15,  // 블록 간 시간(초)
    Epoch:  30000,  // 에포크 길이(블록 수)
}
db := ethdb.NewMemoryDatabase()
engine := core.New(config, db)

// 서명자 설정
privateKey, _ := crypto.GenerateKey()
address := crypto.PubkeyToAddress(privateKey.PublicKey)
engine.SetSigner(address)
engine.SetSignerFn(func(signer common.Address, hash []byte) ([]byte, error) {
    return crypto.Sign(hash, privateKey)
})
```

#### 검증자 정보 조회

```go
// API 생성
api := &core.API{
    chain:  chain,
    eirene: engine,
}

// 검증자 목록 조회
validators, err := api.GetValidators()
if err != nil {
    log.Fatalf("검증자 목록 조회 실패: %v", err)
}

// 특정 검증자 정보 조회
validator, err := api.GetValidator(address)
if err != nil {
    log.Fatalf("검증자 정보 조회 실패: %v", err)
}
```

### Governance API

#### 제안 제출 및 투표

```go
// API 생성
govAPI := core.NewGovernanceAPI(chain, engine)

// 제안 제출
proposalID, err := govAPI.SubmitProposal(
    proposer,
    "제안 제목",
    "제안 설명",
    0, // 제안 유형 (0: 텍스트, 1: 매개변수 변경, 2: 업그레이드, 3: 자금 지원)
    parameters,
    nil,
    nil,
    deposit,
)
if err != nil {
    log.Fatalf("제안 제출 실패: %v", err)
}

// 제안에 투표
err = govAPI.Vote(
    proposalID,
    voter,
    0, // 투표 옵션 (0: 찬성, 1: 반대, 2: 기권, 3: 거부권)
    weight,
)
if err != nil {
    log.Fatalf("투표 실패: %v", err)
}
```

#### 제안 조회

```go
// 제안 조회
proposal, err := govAPI.GetProposal(proposalID)
if err != nil {
    log.Fatalf("제안 조회 실패: %v", err)
}

// 제안 목록 조회
proposals := govAPI.GetProposals()

// 투표 조회
votes, err := govAPI.GetVotes(proposalID)
if err != nil {
    log.Fatalf("투표 조회 실패: %v", err)
}
```

### Staking API

#### 스테이킹 및 위임

```go
// API 생성
validatorAPI := core.NewValidatorAPI(chain, engine)

// 검증자 목록 조회
validators := validatorAPI.GetValidators()

// 특정 검증자 정보 조회
validator, err := validatorAPI.GetValidator(address)
if err != nil {
    log.Fatalf("검증자 정보 조회 실패: %v", err)
}

// 위임 정보 조회
delegations, err := validatorAPI.GetDelegations(validator)
if err != nil {
    log.Fatalf("위임 정보 조회 실패: %v", err)
}
```

### IBC API

#### IBC 클라이언트, 연결, 채널 관리

```go
// API 생성
ibcAPI := core.NewIBCAPI(chain, engine)

// 클라이언트 생성
err := ibcAPI.CreateClient(
    "07-tendermint-0",
    "tendermint",
    consensusState,
    100000,
)
if err != nil {
    log.Fatalf("클라이언트 생성 실패: %v", err)
}

// 연결 생성
err = ibcAPI.CreateConnection(
    "connection-0",
    "07-tendermint-0",
    "07-tendermint-1",
    "connection-1",
    "1.0",
)
if err != nil {
    log.Fatalf("연결 생성 실패: %v", err)
}

// 채널 생성
err = ibcAPI.CreateChannel(
    "transfer",
    "channel-0",
    "connection-0",
    "transfer",
    "channel-1",
    "ics20-1",
)
if err != nil {
    log.Fatalf("채널 생성 실패: %v", err)
}

// 토큰 전송
err = ibcAPI.TransferToken(
    "transfer",
    "channel-0",
    token,
    amount,
    sender,
    receiver,
)
if err != nil {
    log.Fatalf("토큰 전송 실패: %v", err)
}
```

## 테스트 작성 가이드

Eirene 합의 알고리즘의 테스트는 Go의 표준 테스트 프레임워크를 사용합니다. 테스트 파일은 각 모듈의 디렉토리에 위치하며, 파일 이름은 `*_test.go` 형식입니다.

### 테스트 구조

테스트 파일은 다음과 같은 구조로 작성됩니다:

```go
package mypackage

import (
    "testing"
    // 필요한 패키지 임포트
)

// 테스트 함수
func TestMyFunction(t *testing.T) {
    // 테스트 준비
    // ...

    // 테스트 실행
    result := MyFunction()

    // 결과 검증
    if result != expectedResult {
        t.Errorf("기대한 결과 %v, 실제 결과 %v", expectedResult, result)
    }
}
```

### 테스트 실행

테스트는 다음 명령으로 실행할 수 있습니다:

```bash
# 모든 테스트 실행
go test ./consensus/eirene/...

# 특정 패키지의 테스트 실행
go test ./consensus/eirene/core/...

# 특정 테스트 실행
go test ./consensus/eirene/core/... -run TestMyFunction

# 자세한 출력으로 테스트 실행
go test -v ./consensus/eirene/...
```

### 테스트 작성 팁

1. **단위 테스트**: 각 함수나 메서드의 기능을 독립적으로 테스트합니다.
2. **통합 테스트**: 여러 컴포넌트가 함께 작동하는 방식을 테스트합니다.
3. **엣지 케이스 테스트**: 경계 조건, 오류 조건 등을 테스트합니다.
4. **모킹**: 외부 의존성을 모킹하여 테스트를 독립적으로 만듭니다.
5. **테이블 기반 테스트**: 여러 입력과 예상 출력을 테이블로 정의하여 테스트합니다.

## 기여 가이드

Eirene 합의 알고리즘에 기여하려면 다음 단계를 따르세요:

1. 저장소를 포크합니다.
2. 새 브랜치를 생성합니다: `git checkout -b feature/my-feature`
3. 변경 사항을 커밋합니다: `git commit -am 'Add my feature'`
4. 브랜치를 푸시합니다: `git push origin feature/my-feature`
5. Pull Request를 생성합니다.

### 코드 스타일

- Go 표준 코드 스타일을 따릅니다.
- `gofmt`를 사용하여 코드를 포맷팅합니다.
- 주석은 한국어로 작성합니다.
- 함수, 메서드, 구조체 등에 주석을 추가합니다.

### 커밋 메시지

커밋 메시지는 다음 형식을 따릅니다:

```
모듈: 변경 내용 요약

변경 내용 상세 설명
```

예:
```
governance: 투표 기능 개선

- 투표 옵션 검증 로직 추가
- 투표 기간 체크 로직 개선
- 투표 집계 성능 최적화
```

## 문제 해결

### 일반적인 문제

#### 빌드 오류

- Go 버전이 1.18 이상인지 확인합니다.
- 의존성이 올바르게 설치되었는지 확인합니다: `go mod tidy`
- 캐시를 정리합니다: `go clean -cache`

#### 테스트 실패

- 테스트 환경이 올바르게 설정되었는지 확인합니다.
- 테스트 로그를 자세히 확인합니다: `go test -v ./...`
- 특정 테스트만 실행해 봅니다: `go test -v -run TestMyFunction ./...`

#### 순환 참조 오류

- 패키지 간 의존성 구조를 확인합니다.
- 인터페이스를 사용하여 의존성을 역전시킵니다.
- 공통 타입을 `utils` 패키지로 이동합니다.

### 디버깅 팁

- `fmt.Printf` 또는 `log.Printf`를 사용하여 디버그 정보를 출력합니다.
- `go test -v` 옵션을 사용하여 자세한 테스트 로그를 확인합니다.
- `delve` 디버거를 사용하여 코드를 단계별로 실행합니다.

## API 문서 생성

Eirene 합의 알고리즘은 Go의 표준 문서화 도구인 `godoc`을 사용하여 API 문서를 생성합니다. 이 섹션에서는 API 문서를 생성하고 확인하는 방법을 설명합니다.

### 필요한 도구 설치

API 문서를 생성하려면 다음 도구가 필요합니다:

```bash
# godoc 설치
go install golang.org/x/tools/cmd/godoc@latest

# godoc2md 설치 (마크다운 문서 생성용)
go install github.com/davecheney/godoc2md@latest
```

### 문서 생성 스크립트 사용

Eirene 합의 알고리즘은 API 문서를 자동으로 생성하는 스크립트를 제공합니다:

```bash
# 스크립트 실행
./consensus/eirene/scripts/generate_docs.sh
```

이 스크립트는 다음 작업을 수행합니다:

1. `godoc` 도구가 설치되어 있는지 확인하고, 필요한 경우 설치합니다.
2. `./consensus/eirene/docs/api` 디렉토리를 생성합니다.
3. HTML 형식의 API 문서를 생성합니다.
4. `godoc2md` 도구가 설치되어 있는 경우, 마크다운 형식의 API 문서도 생성합니다.

### 생성된 문서 확인

문서는 다음 위치에 생성됩니다:

- HTML 문서: `./consensus/eirene/docs/api/*.html`
- 마크다운 문서: `./consensus/eirene/docs/api/*.md`

다음 명령으로 문서를 확인할 수 있습니다:

```bash
# HTML 문서 확인 (브라우저로 열기)
open ./consensus/eirene/docs/api/index.html

# 마크다운 문서 확인
less ./consensus/eirene/docs/api/eirene.md
```

### 로컬 godoc 서버 실행

`godoc` 서버를 로컬에서 실행하여 웹 브라우저에서 API 문서를 확인할 수도 있습니다:

```bash
# godoc 서버 실행
godoc -http=:6060
```

그런 다음 웹 브라우저에서 다음 URL로 접속하여 문서를 확인할 수 있습니다:

```
http://localhost:6060/pkg/github.com/zenanetwork/go-zenanet/consensus/eirene/
```

### 문서화 모범 사례

API 문서의 품질을 높이기 위해 다음 모범 사례를 따르세요:

1. **패키지 문서화**: 각 패키지의 첫 번째 주석은 패키지 전체에 대한 설명이어야 합니다.

   ```go
   // Package core는 Eirene 합의 알고리즘의 핵심 기능을 구현합니다.
   // 이 패키지는 합의 엔진의 주요 인터페이스, 블록 검증 및 생성 메커니즘,
   // 스냅샷 시스템 등을 포함합니다.
   package core
   ```

2. **구조체 및 인터페이스 문서화**: 각 구조체와 인터페이스에 대한 설명을 제공하세요.

   ```go
   // Eirene는 Proof-of-Stake 합의 엔진을 구현합니다.
   // 이 구조체는 블록 생성, 검증, 검증자 관리, 거버넌스, 슬래싱, 보상 분배 등의
   // 기능을 제공합니다.
   type Eirene struct {
       // ...
   }
   ```

3. **함수 및 메서드 문서화**: 각 함수와 메서드에 대한 설명, 매개변수, 반환값을 문서화하세요.

   ```go
   // GetSnapshot은 지정된 블록 번호에서 검증자 상태의 스냅샷을 반환합니다.
   // 
   // 매개변수:
   //   - number: 스냅샷을 가져올 블록 번호. nil인 경우 최신 블록 사용
   //
   // 반환값:
   //   - *Snapshot: 검증자 상태의 스냅샷
   //   - error: 오류 발생 시 반환
   func (api *API) GetSnapshot(number *uint64) (*Snapshot, error) {
       // ...
   }
   ```

4. **예제 코드 추가**: 복잡한 기능의 경우 예제 코드를 추가하세요.

   ```go
   // Example_getValidators는 GetValidators 함수의 사용 예시를 보여줍니다.
   func Example_getValidators() {
       // 예제 코드
       // ...
   }
   ```

## 추가 자료

- [Go 언어 문서](https://golang.org/doc/)
- [Tendermint 문서](https://docs.tendermint.com/)
- [Cosmos SDK 문서](https://docs.cosmos.network/)
- [IBC 프로토콜 문서](https://github.com/cosmos/ibc) 