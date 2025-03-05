#!/bin/bash

# API 문서 자동 생성 스크립트
# 이 스크립트는 godoc 도구를 사용하여 Eirene 합의 알고리즘의 API 문서를 생성합니다.

# 필요한 도구 설치 확인
if ! command -v godoc &> /dev/null; then
    echo "godoc이 설치되어 있지 않습니다. 설치 중..."
    go install golang.org/x/tools/cmd/godoc@latest
fi

# 문서 생성 디렉토리 생성
DOCS_DIR="./consensus/eirene/docs/api"
mkdir -p $DOCS_DIR

echo "Eirene 합의 알고리즘 API 문서 생성 중..."

# godoc을 사용하여 HTML 문서 생성
echo "HTML 문서 생성 중..."
godoc -html -url="/pkg/github.com/zenanetwork/go-zenanet/consensus/eirene/" > $DOCS_DIR/index.html
godoc -html -url="/pkg/github.com/zenanetwork/go-zenanet/consensus/eirene/core/" > $DOCS_DIR/core.html
godoc -html -url="/pkg/github.com/zenanetwork/go-zenanet/consensus/eirene/governance/" > $DOCS_DIR/governance.html
godoc -html -url="/pkg/github.com/zenanetwork/go-zenanet/consensus/eirene/staking/" > $DOCS_DIR/staking.html
godoc -html -url="/pkg/github.com/zenanetwork/go-zenanet/consensus/eirene/abci/" > $DOCS_DIR/abci.html
godoc -html -url="/pkg/github.com/zenanetwork/go-zenanet/consensus/eirene/ibc/" > $DOCS_DIR/ibc.html
godoc -html -url="/pkg/github.com/zenanetwork/go-zenanet/consensus/eirene/utils/" > $DOCS_DIR/utils.html

# 마크다운 문서 생성 (godoc2md 도구 필요)
if command -v godoc2md &> /dev/null; then
    echo "마크다운 문서 생성 중..."
    godoc2md github.com/zenanetwork/go-zenanet/consensus/eirene > $DOCS_DIR/eirene.md
    godoc2md github.com/zenanetwork/go-zenanet/consensus/eirene/core > $DOCS_DIR/core.md
    godoc2md github.com/zenanetwork/go-zenanet/consensus/eirene/governance > $DOCS_DIR/governance.md
    godoc2md github.com/zenanetwork/go-zenanet/consensus/eirene/staking > $DOCS_DIR/staking.md
    godoc2md github.com/zenanetwork/go-zenanet/consensus/eirene/abci > $DOCS_DIR/abci.md
    godoc2md github.com/zenanetwork/go-zenanet/consensus/eirene/ibc > $DOCS_DIR/ibc.md
    godoc2md github.com/zenanetwork/go-zenanet/consensus/eirene/utils > $DOCS_DIR/utils.md
else
    echo "godoc2md가 설치되어 있지 않습니다. 마크다운 문서는 생성되지 않습니다."
    echo "마크다운 문서를 생성하려면 다음 명령을 실행하세요:"
    echo "go install github.com/davecheney/godoc2md@latest"
fi

echo "API 문서 생성이 완료되었습니다."
echo "문서는 $DOCS_DIR 디렉토리에 저장되었습니다." 