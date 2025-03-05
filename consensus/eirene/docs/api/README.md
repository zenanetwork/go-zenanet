# Eirene 합의 알고리즘 API 문서

이 디렉토리는 Eirene 합의 알고리즘의 API 문서를 포함합니다. 이 문서는 `godoc` 도구를 사용하여 자동으로 생성됩니다.

## 문서 파일

- `index.html`: Eirene 패키지의 메인 API 문서
- `core.html`: Core 모듈의 API 문서
- `governance.html`: Governance 모듈의 API 문서
- `staking.html`: Staking 모듈의 API 문서
- `abci.html`: ABCI 모듈의 API 문서
- `ibc.html`: IBC 모듈의 API 문서
- `utils.html`: Utils 모듈의 API 문서

- `eirene.md`: Eirene 패키지의 메인 API 문서 (마크다운 형식)
- `core.md`: Core 모듈의 API 문서 (마크다운 형식)
- `governance.md`: Governance 모듈의 API 문서 (마크다운 형식)
- `staking.md`: Staking 모듈의 API 문서 (마크다운 형식)
- `abci.md`: ABCI 모듈의 API 문서 (마크다운 형식)
- `ibc.md`: IBC 모듈의 API 문서 (마크다운 형식)
- `utils.md`: Utils 모듈의 API 문서 (마크다운 형식)

## 문서 생성 방법

이 문서는 다음 명령으로 생성됩니다:

```bash
./consensus/eirene/scripts/generate_docs.sh
```

## 문서 업데이트

코드가 변경되면 문서를 다시 생성하여 최신 상태로 유지해야 합니다. 문서는 코드의 주석을 기반으로 자동으로 생성되므로, 코드 주석을 잘 작성하는 것이 중요합니다.

## 추가 문서

- [개발자 가이드](../../DEVELOPER.md): Eirene 합의 알고리즘을 개발하고 확장하기 위한 가이드
- [README.md](../../README.md): 프로젝트 개요 및 기능 설명 