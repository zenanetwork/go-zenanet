# Eirene 개발자 가이드

이 문서는 Eirene 합의 알고리즘의 개발자를 위한 가이드입니다. 코드 구조, 주요 컴포넌트, 개발 가이드라인 등을 설명합니다.

## 코드 구조

Eirene 합의 알고리즘은 다음과 같은 주요 모듈로 구성되어 있습니다:

- **core**: 합의 알고리즘의 핵심 로직을 구현합니다.
- **validator**: 검증자 관리 및 검증자 집합 업데이트를 담당합니다.
- **staking**: 토큰 스테이킹 및 위임 기능을 구현합니다.
- **governance**: 거버넌스 제안 및 투표 기능을 구현합니다.
- **abci**: Tendermint ABCI 인터페이스를 구현합니다.
- **p2p**: 피어 간 통신 기능을 구현합니다.
- **ibc**: 체인 간 통신 기능을 구현합니다.
- **utils**: 공통 유틸리티 함수 및 타입을 제공합니다.

## 주요 컴포넌트

### 어댑터 패턴

Eirene는 어댑터 패턴을 사용하여 다양한 구현체를 지원합니다. 주요 어댑터 인터페이스는 다음과 같습니다:

- **StakingAdapterInterface**: 스테이킹 기능을 위한 인터페이스
- **GovernanceAdapterInterface**: 거버넌스 기능을 위한 인터페이스
- **ValidatorSetInterface**: 검증자 집합 관리를 위한 인터페이스

각 인터페이스는 기본 구현체와 Cosmos SDK 기반 구현체를 가지고 있습니다:

- **StakingAdapter** / **CosmosStakingAdapter**
- **GovernanceAdapter** / **CosmosGovernanceAdapter**
- **ValidatorSet** / **CosmosValidatorSet**

### 베이스 어댑터 패턴

코드 중복을 줄이기 위해 베이스 어댑터 패턴을 도입했습니다. 베이스 어댑터는 공통 기능을 구현하고, 구체적인 어댑터는 베이스 어댑터를 상속받아 특화된 기능을 추가합니다:

- **BaseStakingAdapter**: StakingAdapter와 CosmosStakingAdapter의 공통 기능 구현
- **BaseGovernanceAdapter**: GovernanceAdapter와 CosmosGovernanceAdapter의 공통 기능 구현

## 오류 처리

Eirene는 일관된 오류 처리를 위해 `utils/errors.go`에 정의된 오류 상수와 함수를 사용합니다:

- **오류 상수**: `ErrInvalidParameter`, `ErrValidatorNotFound` 등
- **오류 래핑 함수**: `WrapError`, `FormatError` 등

오류 처리 예시:

```go
if validator == nil {
    return utils.WrapError(utils.ErrValidatorNotFound, 
        fmt.Sprintf("validator not found: %s", address.Hex()))
}
```

## 로깅

Eirene는 구조화된 로깅을 위해 `utils/logging.go`에 정의된 로깅 인터페이스와 구현체를 사용합니다:

- **Logger 인터페이스**: 다양한 로그 레벨과 컨텍스트 추가 메서드 제공
- **EireneLogger**: Logger 인터페이스의 기본 구현체

로깅 예시:

```go
logger := utils.NewLogger("module-name")
logger.Info("Operation completed", "key", value)
```

## 개발 가이드라인

### 코드 스타일

- Go 표준 코드 스타일을 따릅니다.
- 모든 공개 함수와 타입에는 주석을 추가합니다.
- 복잡한 로직에는 인라인 주석을 추가합니다.

### 테스트

- 모든 기능에 대한 단위 테스트를 작성합니다.
- 통합 테스트는 `*_test.go` 파일에 작성합니다.
- 테스트 커버리지는 80% 이상을 유지합니다.

### 리팩토링 가이드라인

코드 리팩토링 시 다음 가이드라인을 따릅니다:

1. **중복 코드 제거**: 공통 기능은 베이스 클래스로 추출합니다.
2. **인터페이스 분리**: 큰 인터페이스보다 작고 집중된 인터페이스를 선호합니다.
3. **일관된 오류 처리**: `utils/errors.go`에 정의된 오류 상수와 함수를 사용합니다.
4. **로깅 표준화**: `utils/logging.go`에 정의된 로깅 인터페이스를 사용합니다.
5. **의존성 주입**: 하드코딩된 의존성보다 의존성 주입을 선호합니다.

## 기여 방법

1. 이슈 생성: 새로운 기능이나 버그 수정을 위한 이슈를 생성합니다.
2. 브랜치 생성: `feature/기능명` 또는 `fix/버그명` 형식의 브랜치를 생성합니다.
3. 코드 작성: 위의 가이드라인에 따라 코드를 작성합니다.
4. 테스트 작성: 새로운 기능이나 버그 수정에 대한 테스트를 작성합니다.
5. 풀 리퀘스트 생성: 작업이 완료되면 풀 리퀘스트를 생성합니다.

## 문서화

- 모든 공개 API에 대한 문서를 작성합니다.
- 복잡한 알고리즘이나 데이터 구조에 대한 설명을 추가합니다.
- 변경 사항은 CHANGELOG.md에 기록합니다.

## 성능 최적화

- 성능 병목 지점을 식별하기 위해 프로파일링을 수행합니다.
- 메모리 사용량을 최적화하기 위해 불필요한 할당을 피합니다.
- 동시성 이슈를 방지하기 위해 적절한 동기화 메커니즘을 사용합니다.

## CLI 도구 사용 방법

Eirene CLI 도구는 개발 및 테스트를 위한 다양한 명령어를 제공합니다. 주요 명령어는 다음과 같습니다:

### 노드 관리

```bash
# 노드 시작
eirene node start --home=<노드_홈_디렉토리>

# 노드 상태 확인
eirene node status

# 노드 중지
eirene node stop

# 노드 초기화
eirene node init --chain-id=<체인_ID> --home=<노드_홈_디렉토리>

# 노드 리셋
eirene node reset --home=<노드_홈_디렉토리>
```

### 계정 관리

```bash
# 계정 생성
eirene account create --name=<계정_이름>

# 계정 목록 조회
eirene account list

# 계정 정보 조회
eirene account show --name=<계정_이름>

# 계정 복구
eirene account recover --name=<계정_이름> --mnemonic="<니모닉_단어>"

# 계정 삭제
eirene account delete --name=<계정_이름>
```

### 트랜잭션 관리

```bash
# 토큰 전송
eirene tx send --from=<보내는_계정> --to=<받는_주소> --amount=<금액>

# 트랜잭션 조회
eirene tx show --hash=<트랜잭션_해시>

# 트랜잭션 브로드캐스트
eirene tx broadcast --file=<트랜잭션_파일>

# 트랜잭션 서명
eirene tx sign --from=<서명_계정> --file=<트랜잭션_파일>
```

### 스테이킹 관리

```bash
# 검증자 생성
eirene staking create-validator --from=<계정_이름> --amount=<스테이킹_금액> --pubkey=<검증자_공개키>

# 토큰 위임
eirene staking delegate --from=<계정_이름> --validator=<검증자_주소> --amount=<위임_금액>

# 위임 취소
eirene staking unbond --from=<계정_이름> --validator=<검증자_주소> --amount=<취소_금액>

# 재위임
eirene staking redelegate --from=<계정_이름> --src-validator=<원본_검증자> --dst-validator=<대상_검증자> --amount=<재위임_금액>

# 보상 인출
eirene staking withdraw-rewards --from=<계정_이름> --validator=<검증자_주소>
```

### 거버넌스 관리

```bash
# 제안 생성
eirene governance submit-proposal --from=<계정_이름> --type=<제안_유형> --title=<제안_제목> --description=<제안_설명> --deposit=<초기_예치금>

# 제안 조회
eirene governance query-proposal --proposal-id=<제안_ID>

# 제안 목록 조회
eirene governance list-proposals

# 투표
eirene governance vote --from=<계정_이름> --proposal-id=<제안_ID> --option=<투표_옵션>

# 예치금 추가
eirene governance deposit --from=<계정_이름> --proposal-id=<제안_ID> --amount=<예치금_금액>

# 거버넌스 파라미터 조회
eirene governance params

# 투표 집계 결과 조회
eirene governance tally --proposal-id=<제안_ID>
```

### 네트워크 모니터링

```bash
# 블록 조회
eirene network block --height=<블록_높이>

# 최신 블록 조회
eirene network latest-block

# 검증자 목록 조회
eirene network validators

# 네트워크 상태 조회
eirene network status

# 피어 목록 조회
eirene network peers

# 트랜잭션 검색
eirene network tx-search --query=<검색_쿼리>
```

### 버전 정보

```bash
# 버전 정보 표시
eirene version

# 상세 버전 정보 표시
eirene version --detailed
```

## 고급 개발 주제

### 성능 프로파일링

성능 최적화를 위해 다음과 같은 프로파일링 도구를 사용할 수 있습니다:

```bash
# CPU 프로파일링
go tool pprof http://localhost:6060/debug/pprof/profile

# 메모리 프로파일링
go tool pprof http://localhost:6060/debug/pprof/heap

# 블록 트레이싱
eirene debug trace-block --height=<블록_높이> --output=<출력_파일>
```

### 병렬 처리 최적화

트랜잭션 처리 성능을 향상시키기 위해 다음과 같은 병렬 처리 기법을 사용합니다:

1. **트랜잭션 의존성 분석**: 서로 독립적인 트랜잭션을 식별하여 병렬로 처리합니다.
2. **워커 풀 최적화**: 시스템 리소스에 따라 워커 풀 크기를 동적으로 조정합니다.
3. **배치 처리**: 유사한 작업을 그룹화하여 배치로 처리합니다.

### 커스텀 모듈 개발

Eirene 합의 알고리즘에 새로운 모듈을 추가하는 방법:

1. 적절한 디렉토리에 새 패키지 생성
2. 필요한 인터페이스 구현
3. 모듈 등록 및 초기화 코드 작성
4. 단위 테스트 및 통합 테스트 작성
5. 문서화

## 문제 해결

### 일반적인 문제

- **노드 연결 문제**: 방화벽 설정, P2P 포트 확인, 피어 주소 확인
- **합의 참여 문제**: 검증자 등록 상태, 스테이킹 금액, 프라이빗 키 확인
- **트랜잭션 실패**: 가스 설정, 계정 잔액, 서명 확인
- **성능 저하**: 디스크 I/O, 메모리 사용량, CPU 사용량 확인

### 디버깅 도구

- **로그 분석**: `eirene node logs --tail=100`
- **상태 덤프**: `eirene debug dump-state --height=<블록_높이>`
- **메모리 덤프**: `eirene debug dump-memory`
- **트레이스 활성화**: `eirene node start --trace`

## 추가 자료

- [API 문서](../docs/api.md)
- [아키텍처 문서](../docs/architecture.md)
- [성능 최적화 가이드](../docs/performance.md)
- [보안 가이드](../docs/security.md)
- [테스트 가이드](../docs/testing.md) 