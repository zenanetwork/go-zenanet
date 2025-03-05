# Eirene 합의 알고리즘

Eirene는 Zenanet 블록체인을 위한 Proof-of-Stake(PoS) 합의 알고리즘입니다. 이 알고리즘은 Cosmos SDK와 Tendermint의 합의 메커니즘을 기반으로 하며, go-zenanet의 합의 엔진 인터페이스를 구현합니다.

## 문서

- [개발자 가이드](./DEVELOPER.md): Eirene 합의 알고리즘을 개발하고 확장하기 위한 가이드
  - 아키텍처 개요
  - 개발 환경 설정
  - 주요 모듈 설명 (Core, Governance, Staking, ABCI, IBC, Utils)
  - API 사용 가이드
  - 테스트 작성 가이드
  - 기여 가이드
  - 문제 해결
- [README.md](./README.md): 프로젝트 개요 및 기능 설명

### 개발 시작하기

Eirene 합의 알고리즘 개발을 시작하려면 다음 단계를 따르세요:

1. 저장소 클론:
   ```bash
   git clone https://github.com/zenanetwork/go-zenanet.git
   cd go-zenanet
   ```

2. 의존성 설치:
   ```bash
   go mod download
   ```

3. 개발자 가이드 확인:
   ```bash
   less consensus/eirene/DEVELOPER.md
   ```

4. 테스트 실행:
   ```bash
   go test ./consensus/eirene/...
   ```

### API 문서 생성

Eirene 합의 알고리즘의 API 문서를 생성하려면 다음 단계를 따르세요:

1. 필요한 도구 설치:
   ```bash
   go install golang.org/x/tools/cmd/godoc@latest
   go install github.com/davecheney/godoc2md@latest
   ```

2. 문서 생성 스크립트 실행:
   ```bash
   ./consensus/eirene/scripts/generate_docs.sh
   ```

3. 생성된 문서 확인:
   ```bash
   # HTML 문서
   open ./consensus/eirene/docs/api/index.html
   
   # 마크다운 문서
   less ./consensus/eirene/docs/api/eirene.md
   ```

자세한 내용은 [개발자 가이드](./DEVELOPER.md)를 참조하세요.

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
- **Cosmos SDK 및 Tendermint 통합 완료**:
  - Tendermint v0.37.0-rc2 및 Cosmos SDK v0.52.0-rc2 의존성 추가
  - ABCI 어댑터 구현 (Tendermint의 ABCI 인터페이스와 go-zenanet 연결)
  - 스테이킹 어댑터 구현 (Cosmos SDK의 스테이킹 모듈과 go-zenanet 연결)
  - 거버넌스 어댑터 구현 (Cosmos SDK의 거버넌스 모듈과 go-zenanet 연결)
  - 거버넌스 API 구현 (제안 제출, 투표, 보증금 예치 등의 기능 제공)
  - 슬래싱 메커니즘 구현 (이중 서명, 다운타임 등에 대한 페널티)
  - 보상 분배 시스템 구현 (검증자, 위임자, 커뮤니티 기금 간 보상 분배)
  - IBC 프로토콜 구현 (크로스체인 통신 및 자산 전송)
  - 어댑터 초기화 및 상태 DB 접근 구현 완료
  - Tendermint v0.37.0-rc2 호환성 이슈 해결
  - Cosmos SDK v0.52.0-rc2 패키지 구조 변경 대응 완료
    - `x/staking` → `cosmossdk.io/x/staking` 패키지 경로 변경 적용
    - 스테이킹 모듈 타입 및 인터페이스 변경 대응
    - 상태 DB 어댑터 확장으로 키 접두사 검색 기능 추가

### 최근 업데이트 (2024년 5월)

- **코드 안정성 향상**:
  - 모든 모듈에서 빌드 오류 제거 완료
  - 타입 안전성 강화 완료
  - 일관된 코딩 스타일 적용 완료
  - 패키지 간 의존성 구조 개선 완료
  - 임포트 사이클 문제 해결 완료

- **Cosmos SDK v0.52.0-rc2 호환성 개선**:
  - Cosmos SDK 패키지 구조 변경에 대응 완료
  - `cosmossdk.io/x/staking` 모듈 통합 완료
  - `go.mod` 파일에 정확한 버전 의존성 설정 완료
  - `replace` 지시문을 통한 버전 충돌 해결 완료
  - `cosmos_staking_adapter.go` 파일 업데이트 완료
  - `store_adapter.go` 파일에 `GetKeysWithPrefix` 메서드 추가 완료

- **테스트 코드 개선**: 
  - 모든 테스트가 성공적으로 통과하도록 수정 완료
  - 비공개 필드 접근 문제 해결을 위한 리플렉션 기반 테스트 헬퍼 함수 구현 완료
  - 테스트 커버리지 향상 완료
  - 엣지 케이스에 대한 추가 테스트 작성 완료

- **버그 수정**:
  - 거버넌스 모듈의 상수 정의 충돌 문제 해결 완료
  - IBC 모듈의 상태 상수 참조 오류 수정 완료
  - 임포트 경로 오류 수정 완료 (`github.com/zenanet/go-zenanet` → `github.com/zenanetwork/go-zenanet`)
  - 보상 분배 시스템의 타입 변환 오류 수정 완료
  - 패키지 간 순환 참조 문제 (core ↔ governance) 해결 완료
    - 인터페이스 기반 설계로 전환하여 순환 참조 제거
    - `EireneInterface` 인터페이스 도입으로 의존성 역전 원칙 적용

- **기능 구현 완료**:
  - API 모듈의 `SetSignerPrivateKey` 함수 구현 완료
  - 검증자 확인 로직 구현 완료
  - 업그레이드 로직 구현 완료
  - 매개변수 변경 실행 로직 구현 완료
  - 상태 DB 저장 및 로드 로직 구현 완료
  - 공개 API 확장 (`GetSigner`, `GetSignerFn`, `SetSigner`, `SetSignerFn` 메서드 추가) 완료

- **문서화 개선**:
  - 코드 주석 보강 완료
  - 개발자 가이드 작성 완료
  - API 문서 자동화 완료

### 향후 개발 계획

#### 단기 계획 (1-3개월)

- **패키지 구조 개선**:
  - ✅ 순환 참조 문제 해결을 위한 패키지 구조 재설계 완료
  - ✅ 공통 타입을 utils 패키지로 이동 완료
  - ✅ 인터페이스 기반 설계 강화 완료

- **테스트 강화**:
  - ✅ 기본 테스트 코드 수정 완료
  - ✅ 엣지 케이스에 대한 추가 테스트 작성 완료
  - 통합 테스트 시나리오 확장 예정
  - 스트레스 테스트 및 성능 테스트 추가 예정

- **코드 리팩토링**:
  - ✅ 중복 코드 제거 및 모듈화 개선 완료
  - ✅ 패키지 간 의존성 구조 개선 완료
  - ✅ 임포트 사이클 문제 해결 완료
  - ✅ 비공개 필드 접근 문제 해결을 위한 공개 API 확장 완료

- **문서화 개선**:
  - ✅ 코드 주석 보강 완료
  - ✅ 개발자 가이드 작성 완료
  - ✅ API 문서 자동화 완료
  - 사용자 튜토리얼 작성 예정
  - 예제 코드 추가 예정

#### 중기 계획 (3-6개월)

- **검증자 선택 알고리즘 추가 개선**: 더 복잡한 성능 지표와 평판 시스템 도입
- **거버넌스 기능 확장**: 더 복잡한 거버넌스 기능 및 투표 메커니즘 구현
- **성능 최적화**: 합의 알고리즘의 성능 및 확장성 개선
- **IBC 프로토콜 기능 확장**:
  - 더 다양한 크로스체인 애플리케이션 지원
  - IBC 릴레이어 구현

#### 장기 계획 (6개월 이상)

- **Tendermint의 P2P 네트워킹 기능 강화**:
  - 피어 검색 및 연결 관리 개선
  - 네트워크 보안 강화
- **Cosmos SDK의 distribution 모듈 확장**: 보상 분배 시스템을 강화
- **Cosmos SDK의 ibc 모듈 확장**: 크로스체인 통신 및 자산 전송 기능을 강화
- **Tendermint의 증거 관리 시스템 확장**: 이중 서명 등의 증거를 관리하는 시스템을 강화
- **새로운 합의 알고리즘 연구**: 더 효율적이고 안전한 합의 알고리즘 연구

### 개발 로드맵

다음은 Eirene 합의 알고리즘의 개발 로드맵입니다. 각 작업은 우선순위와 의존성에 따라 순서대로 나열되어 있습니다.

#### 1. 코어 인프라 완성

1. **스테이킹 모듈 완성**
   - ✅ `cosmos_staking_adapter.go` 구현 완료
   - ✅ `store_adapter.go`에 `GetKeysWithPrefix` 메서드 구현 완료
   - `validator.go` 및 `delegation.go` 타입 업데이트
   - 스테이킹 모듈 단위 테스트 작성

2. **슬래싱 모듈 구현**
   - `cosmos_slashing_adapter.go` 구현
   - 이중 서명 감지 및 처리 로직 구현
   - 다운타임 감지 및 처리 로직 구현
   - 슬래싱 모듈 단위 테스트 작성

3. **보상 분배 모듈 구현**
   - `cosmos_distribution_adapter.go` 구현
   - 블록 보상 계산 및 분배 로직 구현
   - 위임자 보상 계산 및 분배 로직 구현
   - 커뮤니티 기금 적립 로직 구현
   - 보상 분배 모듈 단위 테스트 작성

4. **거버넌스 모듈 확장**
   - `cosmos_gov_adapter.go` 확장
   - 제안 제출 및 처리 로직 구현
   - 투표 및 결과 집계 로직 구현
   - 통과된 제안 실행 로직 구현
   - 거버넌스 모듈 단위 테스트 작성

#### 2. ABCI 통합 및 블록체인 연동

5. **ABCI 어댑터 확장**
   - `cosmos_abci_adapter.go` 확장
   - `InitChain` 메서드 구현
   - `BeginBlock` 메서드 구현
   - `DeliverTx` 메서드 구현
   - `EndBlock` 메서드 구현
   - `Commit` 메서드 구현
   - ABCI 어댑터 단위 테스트 작성

6. **블록체인 상태 관리**
   - 블록체인 상태 초기화 로직 구현
   - 블록 처리 파이프라인 구현
   - 트랜잭션 처리 로직 구현
   - 상태 커밋 및 롤백 로직 구현
   - 블록체인 상태 관리 단위 테스트 작성

7. **검증자 관리 시스템**
   - 검증자 집합 업데이트 로직 구현
   - 검증자 순위 계산 로직 구현
   - 검증자 교체 로직 구현
   - 검증자 관리 시스템 단위 테스트 작성

#### 3. 네트워크 및 보안

8. **P2P 네트워킹 기능**
   - 피어 검색 및 연결 관리 구현
   - 블록 및 트랜잭션 전파 로직 구현
   - 네트워크 보안 강화 구현
   - P2P 네트워킹 단위 테스트 작성

9. **증거 관리 시스템**
   - 이중 서명 증거 수집 및 처리 로직 구현
   - 다운타임 증거 수집 및 처리 로직 구현
   - 증거 검증 및 처리 로직 구현
   - 증거 관리 시스템 단위 테스트 작성

10. **보안 감사 및 취약점 분석**
    - 코드 보안 감사 수행
    - 취약점 분석 및 수정
    - 보안 테스트 자동화
    - 보안 문서 작성

#### 4. 크로스체인 및 확장성

11. **IBC 프로토콜 구현**
    - IBC 핸들러 구현
    - IBC 릴레이어 구현
    - 크로스체인 자산 전송 구현
    - IBC 프로토콜 단위 테스트 작성

12. **성능 최적화**
    - 트랜잭션 처리 성능 최적화
    - 상태 DB 접근 최적화
    - 메모리 사용량 최적화
    - 성능 테스트 및 벤치마크 작성

13. **확장성 개선**
    - 샤딩 기술 연구 및 구현
    - 레이어 2 솔루션 연구 및 구현
    - 상태 채널 구현
    - 확장성 테스트 및 벤치마크 작성

#### 5. 사용자 경험 및 도구

14. **API 및 CLI 도구**
    - RESTful API 구현
    - gRPC API 구현
    - CLI 도구 구현
    - API 및 CLI 문서 작성

15. **개발자 도구**
    - SDK 개발
    - 개발자 문서 작성
    - 예제 코드 작성
    - 튜토리얼 작성

16. **모니터링 및 분석 도구**
    - 블록 탐색기 구현
    - 네트워크 모니터링 도구 구현
    - 성능 분석 도구 구현
    - 모니터링 및 분석 문서 작성

#### 6. 테스트넷 및 메인넷

17. **테스트넷 배포**
    - 테스트넷 구성 및 배포
    - 테스트넷 모니터링 시스템 구축
    - 테스트넷 사용자 가이드 작성
    - 테스트넷 피드백 수집 및 개선

18. **메인넷 준비**
    - 메인넷 배포 계획 수립
    - 메인넷 보안 감사
    - 메인넷 성능 테스트
    - 메인넷 배포 문서 작성

19. **커뮤니티 및 생태계**
    - 커뮤니티 구축 및 지원
    - 생태계 파트너십 구축
    - 개발자 교육 및 지원
    - 사용자 교육 및 지원

## PoS 빠른 구축 계획 (2024년 5월)

Tendermint와 Cosmos SDK를 연결하여 PoS 시스템을 빠르게 구축하기 위한 구체적인 실행 계획입니다.

### 1. 의존성 설정 (1일)

- **Cosmos SDK 추가**: 현재 Tendermint v0.37.0-rc2가 설치되어 있으므로, 호환되는 Cosmos SDK v0.52.0-rc2를 추가합니다.
  ```bash
  go get github.com/cosmos/cosmos-sdk@v0.52.0-rc2
  go mod tidy
  ```

- **필요한 Cosmos SDK 모듈 식별**: 다음 모듈들을 중점적으로 활용합니다.
  - `x/staking`: 검증자 관리 및 위임 기능
  - `x/slashing`: 악의적 행동에 대한 페널티 시스템
  - `x/distribution`: 보상 분배 시스템
  - `x/gov`: 온체인 거버넌스 시스템

### 2. 코어 어댑터 구현 (3일)

- **Cosmos SDK 어댑터 구현**: Cosmos SDK의 핵심 기능과 go-zenanet을 연결하는 어댑터를 구현합니다.
  ```go
  // consensus/eirene/cosmos/cosmos_adapter.go
  package cosmos

  import (
      sdk "github.com/cosmos/cosmos-sdk/types"
      "github.com/zenanetwork/go-zenanet/consensus/eirene/core"
  )

  type CosmosAdapter struct {
      // 필요한 필드 정의
  }

  func NewCosmosAdapter() *CosmosAdapter {
      // 구현
  }
  ```

- **상태 DB 어댑터 구현**: go-zenanet의 상태 DB와 Cosmos SDK의 KVStore를 연결하는 어댑터를 구현합니다.
  ```go
  // consensus/eirene/cosmos/store_adapter.go
  package cosmos

  import (
      sdk "github.com/cosmos/cosmos-sdk/types"
      "github.com/cosmos/cosmos-sdk/store"
      "github.com/zenanetwork/go-zenanet/core/state"
  )

  type StateDBAdapter struct {
      // 필요한 필드 정의
  }

  func NewStateDBAdapter(stateDB *state.StateDB) *StateDBAdapter {
      // 구현
  }
  ```

### 3. 스테이킹 모듈 통합 (3일)

- **스테이킹 어댑터 확장**: 기존 `staking_adapter.go`를 확장하여 Cosmos SDK의 staking 모듈과 연동합니다.
  ```go
  // consensus/eirene/staking/cosmos_staking_adapter.go
  package staking

  import (
      "github.com/cosmos/cosmos-sdk/x/staking"
      "github.com/cosmos/cosmos-sdk/x/staking/types"
  )

  type CosmosStakingAdapter struct {
      // 필요한 필드 정의
  }

  func NewCosmosStakingAdapter() *CosmosStakingAdapter {
      // 구현
  }
  ```

- **검증자 관리 기능 구현**: Cosmos SDK의 staking 모듈을 활용하여 검증자 관리 기능을 구현합니다.
  - 검증자 생성, 수정, 삭제 기능
  - 위임 및 위임 철회 기능
  - 재위임 기능

### 4. 슬래싱 모듈 통합 (2일)

- **슬래싱 어댑터 구현**: Cosmos SDK의 slashing 모듈과 연동하는 어댑터를 구현합니다.
  ```go
  // consensus/eirene/staking/cosmos_slashing_adapter.go
  package staking

  import (
      "github.com/cosmos/cosmos-sdk/x/slashing"
      "github.com/cosmos/cosmos-sdk/x/slashing/types"
  )

  type CosmosSlashingAdapter struct {
      // 필요한 필드 정의
  }

  func NewCosmosSlashingAdapter() *CosmosSlashingAdapter {
      // 구현
  }
  ```

- **슬래싱 조건 및 처리 로직 구현**: 다음 슬래싱 조건에 대한 처리 로직을 구현합니다.
  - 이중 서명 (Double Sign)
  - 다운타임 (Downtime)
  - 기타 악의적 행동

### 5. 보상 분배 모듈 통합 (2일)

- **보상 분배 어댑터 구현**: Cosmos SDK의 distribution 모듈과 연동하는 어댑터를 구현합니다.
  ```go
  // consensus/eirene/staking/cosmos_distribution_adapter.go
  package staking

  import (
      "github.com/cosmos/cosmos-sdk/x/distribution"
      "github.com/cosmos/cosmos-sdk/x/distribution/types"
  )

  type CosmosDistributionAdapter struct {
      // 필요한 필드 정의
  }

  func NewCosmosDistributionAdapter() *CosmosDistributionAdapter {
      // 구현
  }
  ```

- **보상 분배 로직 구현**: 블록 보상을 검증자, 위임자, 커뮤니티 기금 간에 분배하는 로직을 구현합니다.
  - 검증자 보상 계산 및 분배
  - 위임자 보상 계산 및 분배
  - 커뮤니티 기금 적립

### 6. 거버넌스 모듈 통합 (3일)

- **거버넌스 어댑터 확장**: 기존 `gov_adapter.go`를 확장하여 Cosmos SDK의 gov 모듈과 연동합니다.
  ```go
  // consensus/eirene/governance/cosmos_gov_adapter.go
  package governance

  import (
      "github.com/cosmos/cosmos-sdk/x/gov"
      "github.com/cosmos/cosmos-sdk/x/gov/types"
  )

  type CosmosGovAdapter struct {
      // 필요한 필드 정의
  }

  func NewCosmosGovAdapter() *CosmosGovAdapter {
      // 구현
  }
  ```

- **제안 및 투표 기능 구현**: Cosmos SDK의 gov 모듈을 활용하여 제안 및 투표 기능을 구현합니다.
  - 제안 제출 및 처리
  - 투표 및 결과 집계
  - 통과된 제안 실행

### 7. ABCI 어댑터 확장 (2일)

- **ABCI 어댑터 확장**: 기존 `abci_adapter.go`를 확장하여 Cosmos SDK의 모듈들과 연동합니다.
  ```go
  // consensus/eirene/abci/cosmos_abci_adapter.go
  package abci

  import (
      sdk "github.com/cosmos/cosmos-sdk/types"
      abci "github.com/tendermint/tendermint/abci/types"
  )

  type CosmosABCIAdapter struct {
      // 필요한 필드 정의
  }

  func NewCosmosABCIAdapter() *CosmosABCIAdapter {
      // 구현
  }
  ```

- **ABCI 메서드 구현**: Tendermint의 ABCI 인터페이스에 필요한 메서드들을 구현합니다.
  - `InitChain`: 체인 초기화
  - `BeginBlock`: 블록 시작
  - `DeliverTx`: 트랜잭션 처리
  - `EndBlock`: 블록 종료
  - `Commit`: 상태 커밋

### 8. 통합 테스트 (3일)

- **단위 테스트 작성**: 각 어댑터 및 모듈에 대한 단위 테스트를 작성합니다.
  ```go
  // consensus/eirene/cosmos/cosmos_adapter_test.go
  package cosmos

  import (
      "testing"
  )

  func TestCosmosAdapter(t *testing.T) {
      // 테스트 구현
  }
  ```

- **통합 테스트 작성**: 전체 시스템에 대한 통합 테스트를 작성합니다.
  ```go
  // consensus/eirene/tests/integration_test.go
  package tests

  import (
      "testing"
  )

  func TestIntegration(t *testing.T) {
      // 테스트 구현
  }
  ```

- **테스트 네트워크 구성**: 로컬 테스트 네트워크를 구성하여 PoS 시스템을 테스트합니다.
  ```bash
  ./scripts/setup_test_network.sh
  ```

### 9. 문서화 및 예제 작성 (2일)

- **API 문서 작성**: 구현된 API에 대한 문서를 작성합니다.
  ```go
  // consensus/eirene/docs/api/cosmos.md
  # Cosmos SDK 통합 API
  
  ## CosmosAdapter
  ...
  ```

- **사용 예제 작성**: PoS 시스템 사용 예제를 작성합니다.
  ```go
  // consensus/eirene/examples/pos_example.go
  package main

  import (
      "github.com/zenanetwork/go-zenanet/consensus/eirene/core"
      "github.com/zenanetwork/go-zenanet/consensus/eirene/cosmos"
  )

  func main() {
      // 예제 구현
  }
  ```

- **튜토리얼 작성**: PoS 시스템 구축 및 사용에 대한 튜토리얼을 작성합니다.
  ```markdown
  // consensus/eirene/docs/tutorials/pos_tutorial.md
  # PoS 시스템 구축 튜토리얼
  
  ## 1. 의존성 설치
  ...
  ```

### 10. 배포 및 테스트넷 운영 (1일)

- **테스트넷 구성**: PoS 시스템을 테스트넷에 배포하고 운영합니다.
  ```bash
  ./scripts/deploy_testnet.sh
  ```

- **모니터링 시스템 구축**: 테스트넷 모니터링 시스템을 구축합니다.
  ```bash
  ./scripts/setup_monitoring.sh
  ```

- **피드백 수집 및 개선**: 테스트넷 운영 결과를 바탕으로 피드백을 수집하고 시스템을 개선합니다.

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

## Cosmos SDK v0.52.0-rc2 업그레이드 정보

### 주요 변경 사항

Cosmos SDK v0.52.0-rc2 버전으로 업그레이드하면서 다음과 같은 주요 변경 사항이 있습니다:

1. **패키지 구조 변경**:
   - 많은 모듈들이 `github.com/cosmos/cosmos-sdk/x/...` 패키지에서 `cosmossdk.io/x/...` 패키지로 이동했습니다.
   - 스테이킹 모듈은 `github.com/cosmos/cosmos-sdk/x/staking` → `cosmossdk.io/x/staking`으로 이동했습니다.
   - 수학 관련 유틸리티는 `github.com/cosmos/cosmos-sdk/types` → `cosmossdk.io/math`로 이동했습니다.

2. **타입 및 인터페이스 변경**:
   - `sdk.Int` → `math.Int`로 변경
   - `sdk.Dec` → `math.LegacyDec`로 변경
   - 스테이킹 모듈의 타입 및 인터페이스 구조 변경
   - 검증자 및 위임 관련 타입 필드 변경

### 호환성 유지 방법

Cosmos SDK v0.52.0-rc2 버전과의 호환성을 유지하기 위해 다음 사항을 주의해야 합니다:

1. **임포트 경로 업데이트**:
   ```go
   import (
       "cosmossdk.io/math"
       "cosmossdk.io/x/staking/types"
       sdk "github.com/cosmos/cosmos-sdk/types"
   )
   ```

2. **go.mod 파일 업데이트**:
   ```
   require (
       github.com/cosmos/cosmos-sdk v0.52.0-rc.2
       cosmossdk.io/x/staking v0.2.0-rc.1
       cosmossdk.io/math v1.5.0
   )
   ```

3. **replace 지시문 사용**:
   ```
   replace (
       github.com/cosmos/cosmos-sdk => github.com/cosmos/cosmos-sdk v0.52.0-rc.2
       cosmossdk.io/x/staking => cosmossdk.io/x/staking v0.2.0-rc.1
   )
   ```

4. **타입 변환 로직 수정**:
   - 스테이킹 모듈의 타입 및 인터페이스 변경에 맞게 코드 수정
   - 수학 관련 타입 변환 로직 수정
