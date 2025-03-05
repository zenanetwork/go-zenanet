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

# Eirene 합의 알고리즘

Eirene는 Zenanet 블록체인을 위한 Proof-of-Stake(PoS) 합의 알고리즘입니다. 이 알고리즘은 Cosmos SDK와 Tendermint의 합의 메커니즘을 기반으로 하며, go-zenanet의 합의 엔진 인터페이스를 구현합니다.

## 주요 특징

- **Proof-of-Stake**: 검증자는 네트워크에 토큰을 스테이킹하여 블록 생성 및 검증에 참여합니다.
- **검증자 순환**: 검증자들은 정해진 순서에 따라 블록을 생성합니다.
- **투표 기반 합의**: 검증자들은 블록의 유효성에 대해 투표합니다.
- **에포크 기반 체크포인트**: 주기적으로 체크포인트를 생성하여 네트워크 상태를 안정화합니다.
- **온체인 거버넌스**: 네트워크 매개변수 변경, 업그레이드, 자금 지원 등을 위한 거버넌스 시스템을 제공합니다.
- **슬래싱 메커니즘**: 악의적인 행동에 대한 페널티 시스템을 구현합니다.
- **위임 기능**: 토큰 소유자가 검증자에게 스테이킹을 위임할 수 있는 기능을 제공합니다.
- **보상 분배 시스템**: 블록 생성 및 검증에 대한 보상 분배 메커니즘을 구현합니다.
- **크로스체인 통합**: IBC(Inter-Blockchain Communication) 프로토콜을 통한 다른 블록체인과의 통합을 지원합니다.

## 구현 상태

현재 구현된 기능:

- 기본 합의 엔진 인터페이스 구현
- 블록 서명 및 검증 메커니즘
- 검증자 상태 관리를 위한 스냅샷 시스템
- 온체인 거버넌스 시스템 (제안, 투표, 실행)
- 검증자 선택 알고리즘 개선 (스테이킹 양과 성능 기반)
- 슬래싱 메커니즘 (이중 서명, 다운타임 등에 대한 페널티)
- 위임 기능 (토큰 소유자가 검증자에게 스테이킹 위임)
- 보상 분배 시스템 (검증자, 위임자, 커뮤니티 기금 간 보상 분배)
- IBC 프로토콜 지원 (크로스체인 통신 및 자산 전송)
- **Cosmos SDK 및 Tendermint 통합 진행 중**:
  - Tendermint v0.37.0-rc2 및 Cosmos SDK v0.52.0-rc2 의존성 추가
  - ABCI 어댑터 구현 (Tendermint의 ABCI 인터페이스와 go-zenanet 연결)
  - 스테이킹 어댑터 구현 (Cosmos SDK의 스테이킹 모듈과 go-zenanet 연결)
  - 거버넌스 어댑터 구현 (Cosmos SDK의 거버넌스 모듈과 go-zenanet 연결)
  - 거버넌스 API 구현 (제안 제출, 투표, 보증금 예치 등의 기능 제공)

## Tendermint v0.37.0-rc2 업그레이드 정보

### 주요 변경 사항

Tendermint v0.37.0-rc2 버전으로 업그레이드하면서 다음과 같은 주요 변경 사항이 있습니다:

1. **패키지 구조 변경**:

   - 많은 타입들이 `github.com/tendermint/tendermint/proto/tendermint/types` 패키지로 이동했습니다.
   - 버전 관련 타입은 `github.com/tendermint/tendermint/proto/tendermint/version` 패키지로 이동했습니다.

2. **타입 이름 및 필드 변경**:
   - `tmtypes.Version` → `tmversion.Consensus`로 변경
   - `abci.Header` → `tmproto.Header`로 변경
   - `abci.BlockID` → `tmproto.BlockID`로 변경
   - `LastBlockID` → `LastBlockId`로 필드명 변경
   - `abci.ConsensusParams` → `tmproto.ConsensusParams`로 변경
   - `abci.BlockParams` → `tmproto.BlockParams`로 변경
   - `abci.EvidenceParams` → `tmproto.EvidenceParams`로 변경
   - `abci.ValidatorParams` → `tmproto.ValidatorParams`로 변경
   - `abci.Evidence` → `abci.Misbehavior`로 변경
   - `abci.LastCommitInfo` → `abci.CommitInfo`로 변경

### 호환성 유지 방법

Tendermint v0.37.0-rc2 버전과의 호환성을 유지하기 위해 다음 사항을 주의해야 합니다:

1. **임포트 경로 업데이트**:

   ```go
   import (
       abci "github.com/tendermint/tendermint/abci/types"
       tmtypes "github.com/tendermint/tendermint/types"
       tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
       tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
   )
   ```

2. **타입 변환 로직 수정**:

   - 헤더, 블록, 트랜잭션 등의 변환 로직에서 새로운 타입 구조를 사용해야 합니다.
   - 특히 `Version`, `Header`, `BlockID` 등의 타입을 사용할 때 주의해야 합니다.

3. **필드 이름 변경 주의**:
   - `LastBlockID` → `LastBlockId`와 같은 필드 이름 변경에 주의해야 합니다.
   - 카멜 케이스와 스네이크 케이스 혼용에 주의해야 합니다.

### 알려진 이슈

현재 Tendermint v0.37.0-rc2 버전으로 업그레이드하면서 다음과 같은 이슈가 있습니다:

1. **core 패키지 호환성 문제**:

   - IBC 관련 필드 접근 오류가 있습니다.
   - 예: `client.State`, `client.LatestHeight`, `connection.Versions` 등의 필드가 정의되지 않았습니다.

2. **staking 패키지 호환성 문제**:
   - `Validator` 구조체가 중복 선언되는 문제가 있습니다.
   - `Eirene` 타입이 정의되지 않은 문제가 있습니다.

이러한 이슈들은 추후 업데이트에서 해결될 예정입니다.

## 향후 개발 계획

- **검증자 선택 알고리즘 추가 개선**: 더 복잡한 성능 지표와 평판 시스템 도입
- **거버넌스 기능 확장**: 더 복잡한 거버넌스 기능 및 투표 메커니즘 구현
- **성능 최적화**: 합의 알고리즘의 성능 및 확장성 개선
- **Cosmos SDK 및 Tendermint 통합 강화**:
  - 어댑터 초기화 및 상태 DB 접근 구현 완료
  - 테스트 작성 및 성능 최적화
  - IBC 프로토콜 완전 지원
  - Tendermint의 P2P 네트워킹 기능 통합
  - **Tendermint v0.37.0-rc2 호환성 이슈 해결**

## 사용 방법

Eirene 합의 알고리즘을 사용하려면 체인 구성에서 Eirene 엔진을 활성화해야 합니다:

```go
config := &params.ChainConfig{
    // 다른 구성 매개변수...
    Eirene: &params.EireneConfig{
        Period: 15,  // 블록 간 시간(초)
        Epoch:  30000,  // 에포크 길이(블록 수)
    },
}
```

## 파일 구조

Eirene 합의 알고리즘은 다음과 같은 폴더 구조로 구성되어 있습니다:

### core/

합의 엔진의 핵심 기능을 구현합니다.

- `eirene.go`: 합의 엔진의 주요 구현
- `api.go`: 기본 RPC API 구현
- `snapshot.go`: 검증자 상태 관리를 위한 스냅샷 시스템
- `eirene_test.go`: 합의 엔진 테스트 케이스

### governance/

온체인 거버넌스 시스템을 구현합니다.

- `governance.go`: 기본 거버넌스 시스템 구현
- `governance_enhanced.go`: 확장된 거버넌스 기능 구현
- `governance_test.go`: 거버넌스 시스템 테스트 케이스
- `gov_adapter.go`: Cosmos SDK의 거버넌스 모듈과 연결하는 어댑터
- `gov_api.go`: 거버넌스 관련 RPC API 구현

### staking/

검증자 관리, 스테이킹, 위임, 보상 분배, 슬래싱 등의 기능을 구현합니다.

- `validator.go`: 검증자 선택 및 관리 시스템 구현
- `validator_enhanced.go`: 확장된 검증자 관리 기능 구현
- `staking_adapter.go`: Cosmos SDK의 스테이킹 모듈과 연결하는 어댑터
- `slashing.go`: 슬래싱 메커니즘 구현
- `reward.go`: 보상 분배 시스템 구현

### abci/

Tendermint의 ABCI(Application BlockChain Interface)와의 통합을 위한 어댑터를 구현합니다.

- `abci_adapter.go`: Tendermint의 ABCI 인터페이스와 go-zenanet 연결

### ibc/

IBC(Inter-Blockchain Communication) 프로토콜을 구현합니다.

- `ibc.go`: IBC 프로토콜 구현
- `ibc_test.go`: IBC 프로토콜 테스트 케이스

### utils/

여러 모듈에서 공통으로 사용하는 유틸리티 함수와 구조체를 제공합니다.

## 거버넌스 시스템

Eirene의 거버넌스 시스템은 다음과 같은 기능을 제공합니다:

### 제안 유형

- **매개변수 변경**: 합의 알고리즘의 매개변수(투표 기간, 쿼럼, 임계값 등)를 변경하는 제안
- **업그레이드**: 네트워크 업그레이드를 위한 제안
- **자금 지원**: 커뮤니티 기금에서 특정 프로젝트나 개발자에게 자금을 지원하는 제안
- **텍스트**: 단순 텍스트 제안으로, 네트워크 방향성이나 정책에 대한 의견을 제시

### 제안 생명 주기

1. **제안 제출**: 검증자는 보증금을 예치하고 제안을 제출합니다.
2. **대기 기간**: 제안은 일정 기간 동안 대기 상태로 유지됩니다.
3. **투표 기간**: 검증자들은 제안에 대해 찬성, 반대, 기권, 거부권 행사 중 하나로 투표합니다.
4. **처리**: 투표 기간이 끝나면 제안은 결과에 따라 통과 또는 거부됩니다.
5. **실행**: 통과된 제안은 일정 기간 후 자동으로 실행됩니다.

## 슬래싱 메커니즘

Eirene의 슬래싱 메커니즘은 다음과 같은 기능을 제공합니다:

### 슬래싱 유형

- **이중 서명**: 동일한 블록 높이에서 두 개의 다른 블록에 서명한 경우
- **다운타임**: 일정 기간 동안 블록 생성 및 검증에 참여하지 않은 경우
- **기타 악의적 행동**: 네트워크 규칙을 위반한 기타 행동

### 슬래싱 결과

- **스테이킹 양 감소**: 위반 유형에 따라 스테이킹된 토큰의 일부가 삭감됩니다.
- **감금**: 일정 기간 동안 검증자 활동이 제한됩니다.
- **평판 감소**: 검증자의 성능 지표가 감소합니다.

## 위임 시스템

Eirene의 위임 시스템은 다음과 같은 기능을 제공합니다:

- **토큰 위임**: 토큰 소유자는 검증자에게 토큰을 위임하여 보상을 받을 수 있습니다.
- **위임 철회**: 위임자는 언제든지 위임을 철회할 수 있습니다.
- **보상 분배**: 블록 보상은 검증자와 위임자 간에 분배됩니다.

## 보상 분배 시스템

Eirene의 보상 분배 시스템은 다음과 같은 기능을 제공합니다:

- **블록 보상**: 블록 생성 및 검증에 대한 보상이 지급됩니다.
- **보상 분배**: 보상은 검증자(70%), 위임자(20%), 커뮤니티 기금(10%)으로 분배됩니다.
- **보상 감소**: 시간이 지남에 따라 블록 보상이 점진적으로 감소합니다.
- **보상 청구**: 검증자와 위임자는 누적된 보상을 청구할 수 있습니다.

## IBC 프로토콜

Eirene의 IBC 프로토콜은 다음과 같은 기능을 제공합니다:

### IBC 구성 요소

- **클라이언트**: 다른 체인의 합의 상태를 추적합니다.
- **연결**: 두 체인 간의 연결을 설정합니다.
- **채널**: 두 모듈 간의 통신 경로를 설정합니다.
- **패킷**: 채널을 통해 전송되는 데이터입니다.

### IBC 기능

- **자산 전송**: 서로 다른 체인 간에 토큰을 전송할 수 있습니다.
- **크로스체인 통신**: 서로 다른 체인의 애플리케이션 간에 메시지를 교환할 수 있습니다.
- **타임아웃 처리**: 패킷 전송이 실패할 경우 자동으로 타임아웃 처리됩니다.
- **확인 응답**: 패킷 수신 시 확인 응답을 보내 성공적인 전송을 확인합니다.

## API

Eirene는 다음과 같은 RPC API를 제공합니다:

- **거버넌스 API**: 제안 제출, 투표, 조회 등의 기능을 제공합니다.
- **슬래싱 API**: 이중 서명 신고, 검증자 감금 해제 등의 기능을 제공합니다.
- **검증자 API**: 검증자 정보 조회, 위임 관리 등의 기능을 제공합니다.
- **보상 API**: 보상 청구, 커뮤니티 기금 관리 등의 기능을 제공합니다.
- **IBC API**: IBC 클라이언트, 연결, 채널 관리 및 자산 전송 등의 기능을 제공합니다.

## Cosmos SDK 및 Tendermint 통합 현황

Eirene 합의 알고리즘은 Cosmos SDK와 Tendermint의 핵심 기능을 go-zenanet에 통합하기 위한 작업을 진행 중입니다:

### 1. 현재 통합 상태

#### 1.1 의존성 추가

- Tendermint v0.37.0-rc2 및 Cosmos SDK v0.52.0 의존성 추가 완료

#### 1.2 어댑터 구현

- **ABCI 어댑터**: Tendermint의 ABCI 인터페이스와 go-zenanet의 합의 엔진을 연결하는 어댑터 구현
- **스테이킹 어댑터**: Cosmos SDK의 스테이킹 모듈과 go-zenanet의 검증자 관리 시스템을 연결하는 어댑터 구현
- **거버넌스 어댑터**: Cosmos SDK의 거버넌스 모듈과 go-zenanet의 Eirene 합의 알고리즘을 연결하는 어댑터 구현

#### 1.3 API 구현

- **거버넌스 API**: 거버넌스 관련 RPC API 구현 (제안 제출, 투표, 보증금 예치 등의 기능 제공)

### 2. 다음 단계

#### 2.1 단기 계획

- **상태 DB 접근 구현**: 현재는 `GetStateDB` 메서드가 임시로 구현되어 있습니다. 실제 구현에서는 적절한 방식으로 상태 DB에 접근해야 합니다.
- **어댑터 초기화 완료**: 현재는 어댑터 초기화 코드가 임시로 생략되어 있습니다. 실제 구현에서는 적절한 인자를 전달하여 어댑터를 초기화해야 합니다.
- **테스트 작성**: 통합된 코드의 정확성을 검증하기 위한 단위 테스트와 통합 테스트를 작성해야 합니다.

#### 2.2 중기 계획

- **성능 최적화**: 실제 사용 환경에서의 성능을 고려하여 코드를 최적화해야 합니다.
- **Cosmos SDK의 distribution 모듈 통합**: 보상 분배 시스템을 강화합니다.
- **Tendermint의 P2P 네트워킹 기능 통합**: 네트워크 통신을 개선합니다.

#### 2.3 장기 계획

- **Cosmos SDK의 ibc 모듈 통합**: 크로스체인 통신 및 자산 전송 기능을 강화합니다.
- **Tendermint의 증거 관리 시스템 통합**: 이중 서명 등의 증거를 관리하는 시스템을 강화합니다.

### 3. 기술적 과제 및 해결 방안

#### 3.1 인터페이스 호환성

- **과제**: go-zenanet의 합의 엔진 인터페이스와 Tendermint의 ABCI 인터페이스 간의 호환성
- **해결 방안**: 어댑터 패턴을 사용하여 두 인터페이스 간의 변환 레이어 구현 (현재 진행 중)

#### 3.2 상태 관리

- **과제**: go-zenanet의 상태 관리 시스템과 Cosmos SDK의 상태 관리 시스템 간의 통합
- **해결 방안**: 공통 상태 인터페이스 정의 및 상태 변환 메커니즘 구현 (계획 중)

#### 3.3 트랜잭션 처리

- **과제**: go-zenanet의 트랜잭션 처리 시스템과 Cosmos SDK의 트랜잭션 처리 시스템 간의 통합
- **해결 방안**: 트랜잭션 변환 레이어 구현 및 공통 트랜잭션 인터페이스 정의 (계획 중)

## 라이센스

Eirene 합의 알고리즘은 go-zenanet 라이브러리의 일부로, GNU Lesser General Public License에 따라 배포됩니다.
