#!/bin/bash
# Eirene 테스트넷 모니터링 스크립트
# 이 스크립트는 Eirene 테스트넷의 상태를 모니터링하고 주요 지표를 수집합니다.

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
HOME_PREFIX="$HOME/.eirene-testnet"
MONITOR_INTERVAL=60  # 초 단위
OUTPUT_DIR="$HOME/.eirene-testnet/monitor"
VALIDATOR_COUNT=4
NODE_COUNT=2
TOTAL_NODES=$((VALIDATOR_COUNT + NODE_COUNT))

# 명령줄 인자 파싱
while [[ $# -gt 0 ]]; do
    case $1 in
        --home-prefix)
            HOME_PREFIX="$2"
            shift 2
            ;;
        --interval)
            MONITOR_INTERVAL="$2"
            shift 2
            ;;
        --output-dir)
            OUTPUT_DIR="$2"
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
        --help)
            echo "사용법: $0 [옵션]"
            echo "옵션:"
            echo "  --home-prefix <path>  노드 홈 디렉토리 접두사 (기본값: $HOME/.eirene-testnet)"
            echo "  --interval <seconds>  모니터링 간격 (초) (기본값: 60)"
            echo "  --output-dir <path>   출력 디렉토리 (기본값: $HOME/.eirene-testnet/monitor)"
            echo "  --validators <count>  검증자 수 (기본값: 4)"
            echo "  --nodes <count>       일반 노드 수 (기본값: 2)"
            echo "  --help                이 도움말 표시"
            exit 0
            ;;
        *)
            log_error "알 수 없는 옵션: $1"
            exit 1
            ;;
    esac
done

TOTAL_NODES=$((VALIDATOR_COUNT + NODE_COUNT))

# CLI 도구 확인
CLI_PATH="$(dirname "$(dirname "$(dirname "$0")")")/cli/cli"
if [ ! -f "$CLI_PATH" ]; then
    log_error "CLI 도구를 찾을 수 없습니다: $CLI_PATH"
    log_info "먼저 CLI 도구를 빌드하세요: cd $(dirname "$(dirname "$(dirname "$0")")")/cli && go build"
    exit 1
fi

# 필요한 명령어 확인
for cmd in jq curl; do
    if ! command -v $cmd &> /dev/null; then
        log_error "$cmd 명령어를 찾을 수 없습니다. 설치 후 다시 시도하세요."
        exit 1
    fi
done

# 출력 디렉토리 생성
mkdir -p "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR/blocks"
mkdir -p "$OUTPUT_DIR/validators"
mkdir -p "$OUTPUT_DIR/transactions"
mkdir -p "$OUTPUT_DIR/network"

log_info "Eirene 테스트넷 모니터링 시작"
log_info "홈 디렉토리 접두사: $HOME_PREFIX"
log_info "모니터링 간격: $MONITOR_INTERVAL초"
log_info "출력 디렉토리: $OUTPUT_DIR"
log_info "검증자 수: $VALIDATOR_COUNT"
log_info "일반 노드 수: $NODE_COUNT"
log_info "총 노드 수: $TOTAL_NODES"

# 모니터링 함수
monitor_node() {
    local node_index=$1
    local node_home="${HOME_PREFIX}${node_index}"
    local timestamp=$(date +"%Y-%m-%d %H:%M:%S")
    local status_file="${OUTPUT_DIR}/node_${node_index}_status.json"
    
    # 노드 상태 확인
    if [ -f "${node_home}/node.pid" ]; then
        local pid=$(cat "${node_home}/node.pid")
        if ps -p $pid > /dev/null; then
            log_info "노드 $node_index (PID: $pid): 실행 중"
            
            # 노드 상태 정보 수집
            local node_status
            node_status=$($CLI_PATH node status --home="${node_home}" 2>/dev/null || echo '{"error": "상태 확인 실패"}')
            
            # 상태 정보 저장
            echo "{\"timestamp\": \"$timestamp\", \"node_index\": $node_index, \"status\": \"running\", \"pid\": $pid, \"node_info\": $node_status}" > "$status_file"
        else
            log_warn "노드 $node_index (PID: $pid): 실행 중이 아님"
            echo "{\"timestamp\": \"$timestamp\", \"node_index\": $node_index, \"status\": \"not_running\", \"pid\": $pid}" > "$status_file"
        fi
    else
        log_warn "노드 $node_index: PID 파일 없음"
        echo "{\"timestamp\": \"$timestamp\", \"node_index\": $node_index, \"status\": \"no_pid_file\"}" > "$status_file"
    fi
}

monitor_blocks() {
    local node_index=$1
    local node_home="${HOME_PREFIX}${node_index}"
    local timestamp=$(date +"%Y-%m-%d %H:%M:%S")
    
    # 최신 블록 정보 수집
    local latest_block
    latest_block=$($CLI_PATH network latest-block --home="${node_home}" 2>/dev/null || echo '{"error": "블록 정보 수집 실패"}')
    
    # 블록 높이 추출
    local block_height
    block_height=$(echo "$latest_block" | jq -r '.block.header.height // "unknown"')
    
    if [ "$block_height" != "unknown" ] && [ "$block_height" != "null" ]; then
        log_info "노드 $node_index: 최신 블록 높이 $block_height"
        
        # 블록 정보 저장
        echo "{\"timestamp\": \"$timestamp\", \"node_index\": $node_index, \"block_height\": $block_height, \"block_info\": $latest_block}" > "${OUTPUT_DIR}/blocks/block_${block_height}.json"
        
        # 블록 높이 추적
        echo "$block_height" > "${OUTPUT_DIR}/latest_block_height.txt"
    else
        log_warn "노드 $node_index: 블록 정보 수집 실패"
    fi
}

monitor_validators() {
    local node_index=$1
    local node_home="${HOME_PREFIX}${node_index}"
    local timestamp=$(date +"%Y-%m-%d %H:%M:%S")
    
    # 검증자 정보 수집
    local validators
    validators=$($CLI_PATH network validators --home="${node_home}" 2>/dev/null || echo '{"error": "검증자 정보 수집 실패"}')
    
    # 검증자 정보 저장
    echo "{\"timestamp\": \"$timestamp\", \"node_index\": $node_index, \"validators\": $validators}" > "${OUTPUT_DIR}/validators/validators_${timestamp//[: ]/_}.json"
}

monitor_network() {
    local node_index=$1
    local node_home="${HOME_PREFIX}${node_index}"
    local timestamp=$(date +"%Y-%m-%d %H:%M:%S")
    
    # 네트워크 상태 정보 수집
    local network_status
    network_status=$($CLI_PATH network status --home="${node_home}" 2>/dev/null || echo '{"error": "네트워크 상태 수집 실패"}')
    
    # 피어 정보 수집
    local peers
    peers=$($CLI_PATH network peers --home="${node_home}" 2>/dev/null || echo '{"error": "피어 정보 수집 실패"}')
    
    # 네트워크 정보 저장
    echo "{\"timestamp\": \"$timestamp\", \"node_index\": $node_index, \"network_status\": $network_status, \"peers\": $peers}" > "${OUTPUT_DIR}/network/network_${timestamp//[: ]/_}.json"
}

generate_report() {
    local timestamp=$(date +"%Y-%m-%d %H:%M:%S")
    local report_file="${OUTPUT_DIR}/report_${timestamp//[: ]/_}.txt"
    
    log_step "모니터링 보고서 생성 중..."
    
    # 보고서 헤더
    echo "Eirene 테스트넷 모니터링 보고서" > "$report_file"
    echo "생성 시간: $timestamp" >> "$report_file"
    echo "----------------------------------------" >> "$report_file"
    
    # 노드 상태 요약
    echo "노드 상태 요약:" >> "$report_file"
    for i in $(seq 0 $((TOTAL_NODES-1))); do
        local status_file="${OUTPUT_DIR}/node_${i}_status.json"
        if [ -f "$status_file" ]; then
            local status=$(jq -r '.status' "$status_file")
            local pid=$(jq -r '.pid // "N/A"' "$status_file")
            echo "  노드 $i: $status (PID: $pid)" >> "$report_file"
        else
            echo "  노드 $i: 상태 정보 없음" >> "$report_file"
        fi
    done
    echo "" >> "$report_file"
    
    # 블록 정보 요약
    local latest_block_height
    if [ -f "${OUTPUT_DIR}/latest_block_height.txt" ]; then
        latest_block_height=$(cat "${OUTPUT_DIR}/latest_block_height.txt")
        echo "최신 블록 높이: $latest_block_height" >> "$report_file"
    else
        echo "최신 블록 높이: 정보 없음" >> "$report_file"
    fi
    echo "" >> "$report_file"
    
    # 검증자 정보 요약
    local latest_validator_file=$(ls -t "${OUTPUT_DIR}/validators/" | head -n 1)
    if [ -n "$latest_validator_file" ]; then
        echo "검증자 정보:" >> "$report_file"
        local validator_count=$(jq '.validators | length // 0' "${OUTPUT_DIR}/validators/$latest_validator_file")
        echo "  총 검증자 수: $validator_count" >> "$report_file"
    else
        echo "검증자 정보: 정보 없음" >> "$report_file"
    fi
    echo "" >> "$report_file"
    
    # 네트워크 정보 요약
    local latest_network_file=$(ls -t "${OUTPUT_DIR}/network/" | head -n 1)
    if [ -n "$latest_network_file" ]; then
        echo "네트워크 정보:" >> "$report_file"
        local peer_count=$(jq '.peers | length // 0' "${OUTPUT_DIR}/network/$latest_network_file")
        echo "  연결된 피어 수: $peer_count" >> "$report_file"
    else
        echo "네트워크 정보: 정보 없음" >> "$report_file"
    fi
    
    log_info "모니터링 보고서 생성 완료: $report_file"
}

# 메인 모니터링 루프
log_step "모니터링 루프 시작..."
while true; do
    log_info "모니터링 실행 중... (${MONITOR_INTERVAL}초 간격)"
    
    # 노드 0을 기준으로 모니터링 수행
    monitor_node 0
    monitor_blocks 0
    monitor_validators 0
    monitor_network 0
    
    # 다른 노드들의 상태만 확인
    for i in $(seq 1 $((TOTAL_NODES-1))); do
        monitor_node $i
    done
    
    # 보고서 생성
    generate_report
    
    # 대기
    sleep $MONITOR_INTERVAL
done 