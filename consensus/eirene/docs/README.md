# Eirene 합의 알고리즘 문서

이 디렉토리는 Eirene 합의 알고리즘의 문서를 포함합니다.

## 문서 구조

- `api/`: API 문서 (자동 생성)
  - HTML 형식의 API 문서
  - 마크다운 형식의 API 문서

## API 문서 생성 방법

API 문서를 생성하려면 다음 단계를 따르세요:

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

## 로컬 godoc 서버 실행

`godoc` 서버를 로컬에서 실행하여 웹 브라우저에서 API 문서를 확인할 수도 있습니다:

```bash
# godoc 서버 실행
godoc -http=:6060
```

그런 다음 웹 브라우저에서 다음 URL로 접속하여 문서를 확인할 수 있습니다:

```
http://localhost:6060/pkg/github.com/zenanetwork/go-zenanet/consensus/eirene/
```

## 추가 문서

- [개발자 가이드](../DEVELOPER.md): Eirene 합의 알고리즘을 개발하고 확장하기 위한 가이드
- [README.md](../README.md): 프로젝트 개요 및 기능 설명 