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