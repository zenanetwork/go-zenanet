#!/bin/bash
# Eirene 테스트넷 초기화 스크립트
# 이 스크립트는 Eirene 테스트넷을 초기화하고 구성하는 데 사용됩니다.

set -e

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 로그 함수
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# 기본 설정값
CHAIN_ID="eirene-testnet-1"
VALIDATOR_COUNT=4
NODE_COUNT=2
TOTAL_NODES=$((VALIDATOR_COUNT + NODE_COUNT))
TOKEN_DENOM="zena"
STAKE_AMOUNT="100000000000${TOKEN_DENOM}"
HOME_PREFIX="$HOME/.eirene-testnet"
CONFIG_DIR="$(dirname "$(dirname "$0")")/config"
GENESIS_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# 명령줄 인자 파싱
while [[ $# -gt 0 ]]; do
    case $1 in
        --chain-id)
            CHAIN_ID="$2"
            shift 2
            ;;
        --validators)
            VALIDATOR_COUNT="$2"
            shift 2
            ;;
        --nodes)
            NODE_COUNT="$2"
            shift 2
            ;;
        --home-prefix)
            HOME_PREFIX="$2"
            shift 2
            ;;
        --stake-amount)
            STAKE_AMOUNT="$2"
            shift 2
            ;;
        --help)
            echo "사용법: $0 [옵션]"
            echo "옵션:"
            echo "  --chain-id <id>       체인 ID (기본값: eirene-testnet-1)"
            echo "  --validators <count>  검증자 수 (기본값: 4)"
            echo "  --nodes <count>       일반 노드 수 (기본값: 2)"
            echo "  --home-prefix <path>  노드 홈 디렉토리 접두사 (기본값: $HOME/.eirene-testnet)"
            echo "  --stake-amount <amt>  초기 스테이킹 금액 (기본값: 100000000000zena)"
            echo "  --help                이 도움말 표시"
            exit 0
            ;;
        *)
            log_error "알 수 없는 옵션: $1"
            exit 1
            ;;
    esac
done

# 필요한 명령어 확인
for cmd in jq curl; do
    if ! command -v $cmd &> /dev/null; then
        log_error "$cmd 명령어를 찾을 수 없습니다. 설치 후 다시 시도하세요."
        exit 1
    fi
done

# CLI 도구 확인
CLI_PATH="../cli/cli"
if [ ! -f "$CLI_PATH" ]; then
    log_error "CLI 도구를 찾을 수 없습니다: $CLI_PATH"
    log_info "먼저 CLI 도구를 빌드하세요: cd ../cli && go build"
    exit 1
fi

log_info "Eirene 테스트넷 초기화 시작"
log_info "체인 ID: $CHAIN_ID"
log_info "검증자 수: $VALIDATOR_COUNT"
log_info "일반 노드 수: $NODE_COUNT"
log_info "총 노드 수: $TOTAL_NODES"
log_info "홈 디렉토리 접두사: $HOME_PREFIX"

# 기존 데이터 정리
log_step "기존 테스트넷 데이터 정리 중..."
for i in $(seq 0 $((TOTAL_NODES-1))); do
    if [ -d "${HOME_PREFIX}${i}" ]; then
        log_warn "기존 노드 데이터 삭제 중: ${HOME_PREFIX}${i}"
        rm -rf "${HOME_PREFIX}${i}"
    fi
done

# 노드 초기화
log_step "노드 초기화 중..."
for i in $(seq 0 $((TOTAL_NODES-1))); do
    log_info "노드 $i 초기화 중..."
    $CLI_PATH node init --chain-id="$CHAIN_ID" --home="${HOME_PREFIX}${i}"
done

# 계정 생성
log_step "계정 생성 중..."
ACCOUNTS=()
for i in $(seq 0 $((TOTAL_NODES-1))); do
    log_info "노드 $i의 계정 생성 중..."
    ACCOUNT_NAME="node$i"
    $CLI_PATH account create --name="$ACCOUNT_NAME" --home="${HOME_PREFIX}${i}"
    ACCOUNTS+=("$ACCOUNT_NAME")
done

# 제네시스 파일 준비
log_step "제네시스 파일 준비 중..."
GENESIS_FILE="${HOME_PREFIX}0/config/genesis.json"
log_info "기본 제네시스 파일: $GENESIS_FILE"

# 제네시스 파일 수정
log_info "제네시스 파일 수정 중..."
jq ".chain_id = \"$CHAIN_ID\" | .genesis_time = \"$GENESIS_TIME\"" "$GENESIS_FILE" > "${GENESIS_FILE}.tmp"
mv "${GENESIS_FILE}.tmp" "$GENESIS_FILE"

# 초기 토큰 할당
log_step "초기 토큰 할당 중..."
for account in "${ACCOUNTS[@]}"; do
    log_info "$account에 초기 토큰 할당 중..."
    ACCOUNT_ADDRESS=$($CLI_PATH account show --name="$account" --home="${HOME_PREFIX}0" | grep "주소:" | awk '{print $2}')
    # 실제 구현에서는 여기에 토큰 할당 로직이 들어갑니다
    # 예: $CLI_PATH tx add-genesis-account --address="$ACCOUNT_ADDRESS" --amount="1000000000000${TOKEN_DENOM}" --home="${HOME_PREFIX}0"
done

# 검증자 설정
log_step "검증자 설정 중..."
for i in $(seq 0 $((VALIDATOR_COUNT-1))); do
    log_info "검증자 $i 설정 중..."
    VALIDATOR_NAME="node$i"
    VALIDATOR_HOME="${HOME_PREFIX}${i}"
    VALIDATOR_PUBKEY=$($CLI_PATH node show-validator --home="$VALIDATOR_HOME")
    # 실제 구현에서는 여기에 검증자 설정 로직이 들어갑니다
    # 예: $CLI_PATH tx create-validator --name="$VALIDATOR_NAME" --amount="$STAKE_AMOUNT" --pubkey="$VALIDATOR_PUBKEY" --home="$VALIDATOR_HOME"
done

# 제네시스 파일 복사
log_step "제네시스 파일 복사 중..."
for i in $(seq 1 $((TOTAL_NODES-1))); do
    log_info "노드 $i에 제네시스 파일 복사 중..."
    cp "$GENESIS_FILE" "${HOME_PREFIX}${i}/config/genesis.json"
done

# P2P 설정
log_step "P2P 설정 중..."
# 노드 ID 및 주소 수집
NODE_IDS=()
for i in $(seq 0 $((TOTAL_NODES-1))); do
    NODE_ID=$($CLI_PATH node show-node-id --home="${HOME_PREFIX}${i}")
    NODE_IDS+=("$NODE_ID")
    log_info "노드 $i ID: $NODE_ID"
done

# persistent_peers 설정
for i in $(seq 0 $((TOTAL_NODES-1))); do
    PEERS=""
    for j in $(seq 0 $((TOTAL_NODES-1))); do
        if [ $i -ne $j ]; then
            if [ -n "$PEERS" ]; then
                PEERS="${PEERS},"
            fi
            PEERS="${PEERS}${NODE_IDS[$j]}@127.0.0.1:$((26656 + $j))"
        fi
    done
    log_info "노드 $i의 persistent_peers 설정: $PEERS"
    # 실제 구현에서는 여기에 persistent_peers 설정 로직이 들어갑니다
    # 예: $CLI_PATH node config set --home="${HOME_PREFIX}${i}" --key="p2p.persistent_peers" --value="$PEERS"
done

# 포트 설정
log_step "포트 설정 중..."
for i in $(seq 0 $((TOTAL_NODES-1))); do
    log_info "노드 $i의 포트 설정 중..."
    # 실제 구현에서는 여기에 포트 설정 로직이 들어갑니다
    # 예: $CLI_PATH node config set --home="${HOME_PREFIX}${i}" --key="rpc.laddr" --value="tcp://0.0.0.0:$((26657 + $i))"
    # 예: $CLI_PATH node config set --home="${HOME_PREFIX}${i}" --key="p2p.laddr" --value="tcp://0.0.0.0:$((26656 + $i))"
done

# 시작 스크립트 생성
log_step "시작 스크립트 생성 중..."
START_SCRIPT="$(dirname "$0")/start_testnet.sh"
cat > "$START_SCRIPT" << EOF
#!/bin/bash
# Eirene 테스트넷 시작 스크립트

set -e

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 로그 함수
log_info() {
    echo -e "\${GREEN}[INFO]\${NC} \$1"
}

log_warn() {
    echo -e "\${YELLOW}[WARN]\${NC} \$1"
}

log_error() {
    echo -e "\${RED}[ERROR]\${NC} \$1"
}

log_step() {
    echo -e "\${BLUE}[STEP]\${NC} \$1"
}

# CLI 도구 확인
CLI_PATH="$CLI_PATH"
if [ ! -f "\$CLI_PATH" ]; then
    log_error "CLI 도구를 찾을 수 없습니다: \$CLI_PATH"
    exit 1
fi

# 노드 시작
log_step "테스트넷 노드 시작 중..."
for i in \$(seq 0 $((TOTAL_NODES-1))); do
    log_info "노드 \$i 시작 중..."
    \$CLI_PATH node start --home="${HOME_PREFIX}\$i" > "${HOME_PREFIX}\$i/node.log" 2>&1 &
    echo \$! > "${HOME_PREFIX}\$i/node.pid"
    log_info "노드 \$i 시작됨 (PID: \$(cat "${HOME_PREFIX}\$i/node.pid"))"
done

log_info "모든 노드가 시작되었습니다."
log_info "로그 확인: tail -f ${HOME_PREFIX}*/node.log"
EOF

chmod +x "$START_SCRIPT"

# 중지 스크립트 생성
log_step "중지 스크립트 생성 중..."
STOP_SCRIPT="$(dirname "$0")/stop_testnet.sh"
cat > "$STOP_SCRIPT" << EOF
#!/bin/bash
# Eirene 테스트넷 중지 스크립트

set -e

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 로그 함수
log_info() {
    echo -e "\${GREEN}[INFO]\${NC} \$1"
}

log_warn() {
    echo -e "\${YELLOW}[WARN]\${NC} \$1"
}

log_error() {
    echo -e "\${RED}[ERROR]\${NC} \$1"
}

log_step() {
    echo -e "\${BLUE}[STEP]\${NC} \$1"
}

# 노드 중지
log_step "테스트넷 노드 중지 중..."
for i in \$(seq 0 $((TOTAL_NODES-1))); do
    if [ -f "${HOME_PREFIX}\$i/node.pid" ]; then
        PID=\$(cat "${HOME_PREFIX}\$i/node.pid")
        if ps -p \$PID > /dev/null; then
            log_info "노드 \$i 중지 중 (PID: \$PID)..."
            kill \$PID
            rm "${HOME_PREFIX}\$i/node.pid"
        else
            log_warn "노드 \$i (PID: \$PID)가 이미 실행 중이 아닙니다."
            rm "${HOME_PREFIX}\$i/node.pid"
        fi
    else
        log_warn "노드 \$i의 PID 파일을 찾을 수 없습니다."
    fi
done

log_info "모든 노드가 중지되었습니다."
EOF

chmod +x "$STOP_SCRIPT"

# 상태 확인 스크립트 생성
log_step "상태 확인 스크립트 생성 중..."
STATUS_SCRIPT="$(dirname "$0")/status_testnet.sh"
cat > "$STATUS_SCRIPT" << EOF
#!/bin/bash
# Eirene 테스트넷 상태 확인 스크립트

set -e

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 로그 함수
log_info() {
    echo -e "\${GREEN}[INFO]\${NC} \$1"
}

log_warn() {
    echo -e "\${YELLOW}[WARN]\${NC} \$1"
}

log_error() {
    echo -e "\${RED}[ERROR]\${NC} \$1"
}

log_step() {
    echo -e "\${BLUE}[STEP]\${NC} \$1"
}

# CLI 도구 확인
CLI_PATH="$CLI_PATH"
if [ ! -f "\$CLI_PATH" ]; then
    log_error "CLI 도구를 찾을 수 없습니다: \$CLI_PATH"
    exit 1
fi

# 노드 상태 확인
log_step "테스트넷 노드 상태 확인 중..."
for i in \$(seq 0 $((TOTAL_NODES-1))); do
    if [ -f "${HOME_PREFIX}\$i/node.pid" ]; then
        PID=\$(cat "${HOME_PREFIX}\$i/node.pid")
        if ps -p \$PID > /dev/null; then
            log_info "노드 \$i (PID: \$PID): 실행 중"
            # 노드 상태 확인
            \$CLI_PATH node status --home="${HOME_PREFIX}\$i" 2>/dev/null || log_warn "노드 \$i 상태 확인 실패"
        else
            log_warn "노드 \$i (PID: \$PID): 실행 중이 아님"
        fi
    else
        log_warn "노드 \$i: PID 파일 없음"
    fi
done
EOF

chmod +x "$STATUS_SCRIPT"

log_info "테스트넷 초기화가 완료되었습니다."
log_info "테스트넷 시작: $START_SCRIPT"
log_info "테스트넷 중지: $STOP_SCRIPT"
log_info "테스트넷 상태 확인: $STATUS_SCRIPT" 