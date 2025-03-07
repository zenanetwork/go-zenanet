#!/bin/bash
# Eirene 메인넷 성능 테스트 스크립트
# 이 스크립트는 Eirene 합의 알고리즘의 성능을 테스트하기 위한 도구입니다.

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
NODE_HOME="$HOME/.eirene"
TEST_DURATION=3600  # 1시간 (초 단위)
TPS_TARGET=1000     # 목표 TPS
REPORT_INTERVAL=60  # 보고서 생성 간격 (초 단위)
OUTPUT_DIR="$HOME/.eirene/performance_test"
TEST_MODE="standard"  # standard, stress, endurance

# 명령줄 인자 파싱
while [[ $# -gt 0 ]]; do
    case $1 in
        --node-home)
            NODE_HOME="$2"
            shift 2
            ;;
        --duration)
            TEST_DURATION="$2"
            shift 2
            ;;
        --tps)
            TPS_TARGET="$2"
            shift 2
            ;;
        --interval)
            REPORT_INTERVAL="$2"
            shift 2
            ;;
        --output)
            OUTPUT_DIR="$2"
            shift 2
            ;;
        --mode)
            TEST_MODE="$2"
            shift 2
            ;;
        --help)
            echo "사용법: $0 [옵션]"
            echo "옵션:"
            echo "  --node-home <path>   노드 홈 디렉토리 (기본값: $HOME/.eirene)"
            echo "  --duration <seconds> 테스트 지속 시간 (초) (기본값: 3600)"
            echo "  --tps <number>       목표 TPS (기본값: 1000)"
            echo "  --interval <seconds> 보고서 생성 간격 (초) (기본값: 60)"
            echo "  --output <path>      출력 디렉토리 (기본값: $HOME/.eirene/performance_test)"
            echo "  --mode <mode>        테스트 모드 (standard, stress, endurance) (기본값: standard)"
            echo "  --help               이 도움말 표시"
            exit 0
            ;;
        *)
            log_error "알 수 없는 옵션: $1"
            exit 1
            ;;
    esac
done

# 필요한 명령어 확인
for cmd in jq curl bc; do
    if ! command -v $cmd &> /dev/null; then
        log_error "$cmd 명령어를 찾을 수 없습니다. 설치 후 다시 시도하세요."
        exit 1
    fi
done

# CLI 도구 확인
CLI_PATH="$(dirname "$(dirname "$(dirname "$0")")")/cli/cli"
if [ ! -f "$CLI_PATH" ]; then
    log_error "CLI 도구를 찾을 수 없습니다: $CLI_PATH"
    log_info "먼저 CLI 도구를 빌드하세요: cd $(dirname "$(dirname "$(dirname "$0")")")/cli && go build"
    exit 1
fi

# 출력 디렉토리 생성
mkdir -p "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR/blocks"
mkdir -p "$OUTPUT_DIR/transactions"
mkdir -p "$OUTPUT_DIR/metrics"
mkdir -p "$OUTPUT_DIR/reports"

# 테스트 모드에 따른 설정
case $TEST_MODE in
    standard)
        log_info "표준 테스트 모드로 실행합니다."
        # 표준 테스트 설정
        TX_BATCH_SIZE=100
        TX_INTERVAL=0.1
        ;;
    stress)
        log_info "스트레스 테스트 모드로 실행합니다."
        # 스트레스 테스트 설정
        TX_BATCH_SIZE=500
        TX_INTERVAL=0.05
        ;;
    endurance)
        log_info "지구력 테스트 모드로 실행합니다."
        # 지구력 테스트 설정
        TX_BATCH_SIZE=50
        TX_INTERVAL=0.2
        TEST_DURATION=$((24*3600))  # 24시간
        ;;
    *)
        log_error "알 수 없는 테스트 모드: $TEST_MODE"
        exit 1
        ;;
esac

log_info "Eirene 메인넷 성능 테스트 시작"
log_info "노드 홈 디렉토리: $NODE_HOME"
log_info "테스트 지속 시간: $TEST_DURATION초"
log_info "목표 TPS: $TPS_TARGET"
log_info "보고서 생성 간격: $REPORT_INTERVAL초"
log_info "출력 디렉토리: $OUTPUT_DIR"

# 테스트 시작 시간 기록
START_TIME=$(date +%s)
TEST_ID="test_$(date +%Y%m%d_%H%M%S)"
log_info "테스트 ID: $TEST_ID"

# 테스트 설정 저장
cat > "$OUTPUT_DIR/test_config_$TEST_ID.json" << EOF
{
  "test_id": "$TEST_ID",
  "start_time": "$(date -d @$START_TIME '+%Y-%m-%d %H:%M:%S')",
  "node_home": "$NODE_HOME",
  "test_duration": $TEST_DURATION,
  "tps_target": $TPS_TARGET,
  "report_interval": $REPORT_INTERVAL,
  "test_mode": "$TEST_MODE",
  "tx_batch_size": $TX_BATCH_SIZE,
  "tx_interval": $TX_INTERVAL
}
EOF

# 초기 상태 저장
log_step "초기 네트워크 상태 저장 중..."
$CLI_PATH network status > "$OUTPUT_DIR/initial_status_$TEST_ID.json"
$CLI_PATH network validators > "$OUTPUT_DIR/initial_validators_$TEST_ID.json"

# 테스트 계정 준비
log_step "테스트 계정 준비 중..."
TEST_ACCOUNT="test_account_$TEST_ID"
$CLI_PATH account new --name="$TEST_ACCOUNT" --password="testpassword" > "$OUTPUT_DIR/test_account_$TEST_ID.txt"
TEST_ADDRESS=$($CLI_PATH account show --name="$TEST_ACCOUNT" | grep "주소:" | awk '{print $2}')
log_info "테스트 계정 주소: $TEST_ADDRESS"

# 테스트 계정에 토큰 전송 (실제 구현에서는 faucet 또는 다른 방법으로 대체)
log_info "테스트 계정에 토큰 전송 중..."
# $CLI_PATH tx send --from="main_account" --to="$TEST_ADDRESS" --amount="1000000zena"

# 성능 측정 함수
measure_performance() {
    local timestamp=$(date +%s)
    local elapsed=$((timestamp - START_TIME))
    
    # 블록 정보 수집
    local latest_block=$($CLI_PATH network latest-block)
    local block_height=$(echo "$latest_block" | jq -r '.block.header.height')
    local block_time=$(echo "$latest_block" | jq -r '.block.header.time')
    
    # 트랜잭션 수 계산
    local tx_count=$(echo "$latest_block" | jq -r '.block.data.txs | length')
    
    # 메모리 사용량 확인
    local memory_usage=$(ps -o rss= -p $(pgrep -f "$NODE_HOME"))
    
    # CPU 사용량 확인
    local cpu_usage=$(ps -o %cpu= -p $(pgrep -f "$NODE_HOME"))
    
    # 디스크 I/O 확인
    local disk_read=$(iostat -d -x | grep sda | awk '{print $6}')
    local disk_write=$(iostat -d -x | grep sda | awk '{print $7}')
    
    # 네트워크 연결 수 확인
    local connections=$(netstat -an | grep ESTABLISHED | wc -l)
    
    # 피어 수 확인
    local peers=$($CLI_PATH network peers | jq -r '. | length')
    
    # 메트릭 저장
    cat > "$OUTPUT_DIR/metrics/metrics_${timestamp}.json" << EOF
{
  "timestamp": $timestamp,
  "elapsed": $elapsed,
  "block_height": $block_height,
  "block_time": "$block_time",
  "tx_count": $tx_count,
  "memory_usage_kb": $memory_usage,
  "cpu_usage_percent": $cpu_usage,
  "disk_read_kbps": $disk_read,
  "disk_write_kbps": $disk_write,
  "connections": $connections,
  "peers": $peers
}
EOF
    
    # 블록 정보 저장
    echo "$latest_block" > "$OUTPUT_DIR/blocks/block_${block_height}.json"
    
    # 현재 TPS 계산 (최근 10개 블록 기준)
    local recent_blocks=()
    for i in {1..10}; do
        if [ $((block_height - i)) -gt 0 ]; then
            local prev_block_file="$OUTPUT_DIR/blocks/block_$((block_height - i)).json"
            if [ -f "$prev_block_file" ]; then
                recent_blocks+=("$prev_block_file")
            fi
        fi
    done
    
    local total_tx=0
    local first_block_time=""
    local last_block_time="$block_time"
    
    if [ ${#recent_blocks[@]} -gt 0 ]; then
        for block_file in "${recent_blocks[@]}"; do
            local block_tx_count=$(jq -r '.block.data.txs | length' "$block_file")
            total_tx=$((total_tx + block_tx_count))
            
            if [ -z "$first_block_time" ]; then
                first_block_time=$(jq -r '.block.header.time' "$block_file")
            fi
        done
        
        # 시간 차이 계산 (초 단위)
        local first_time=$(date -d "$first_block_time" +%s)
        local last_time=$(date -d "$last_block_time" +%s)
        local time_diff=$((last_time - first_time))
        
        # TPS 계산
        if [ $time_diff -gt 0 ]; then
            local current_tps=$(echo "scale=2; $total_tx / $time_diff" | bc)
            log_info "현재 TPS: $current_tps (최근 ${#recent_blocks[@]} 블록 기준)"
            echo "$current_tps" > "$OUTPUT_DIR/metrics/tps_${timestamp}.txt"
        fi
    fi
}

# 트랜잭션 생성 함수
generate_transactions() {
    local batch_size=$1
    local interval=$2
    
    log_info "$batch_size개의 트랜잭션 생성 중..."
    
    for i in $(seq 1 $batch_size); do
        # 랜덤 주소 생성 (실제 구현에서는 유효한 주소 사용)
        local random_address="zena1$(openssl rand -hex 20)"
        
        # 트랜잭션 전송
        $CLI_PATH tx send --from="$TEST_ACCOUNT" --to="$random_address" --amount="1zena" --password="testpassword" > "$OUTPUT_DIR/transactions/tx_$(date +%s)_$i.json" 2>/dev/null &
        
        # 부하 조절을 위한 대기
        sleep $interval
    done
    
    # 모든 백그라운드 작업이 완료될 때까지 대기
    wait
}

# 보고서 생성 함수
generate_report() {
    local timestamp=$(date +%s)
    local elapsed=$((timestamp - START_TIME))
    local report_file="$OUTPUT_DIR/reports/report_${timestamp}.txt"
    
    log_step "성능 테스트 보고서 생성 중... (경과 시간: ${elapsed}초)"
    
    # 최신 메트릭 파일 찾기
    local latest_metrics=$(ls -t "$OUTPUT_DIR/metrics/metrics_"*.json | head -n 1)
    
    if [ -f "$latest_metrics" ]; then
        # 메트릭 데이터 로드
        local block_height=$(jq -r '.block_height' "$latest_metrics")
        local tx_count=$(jq -r '.tx_count' "$latest_metrics")
        local memory_usage=$(jq -r '.memory_usage_kb' "$latest_metrics")
        local cpu_usage=$(jq -r '.cpu_usage_percent' "$latest_metrics")
        local connections=$(jq -r '.connections' "$latest_metrics")
        local peers=$(jq -r '.peers' "$latest_metrics")
        
        # 평균 TPS 계산
        local tps_files=$(ls -t "$OUTPUT_DIR/metrics/tps_"*.txt)
        local tps_count=0
        local tps_sum=0
        
        for tps_file in $tps_files; do
            local tps_value=$(cat "$tps_file")
            tps_sum=$(echo "scale=2; $tps_sum + $tps_value" | bc)
            tps_count=$((tps_count + 1))
        done
        
        local avg_tps=0
        if [ $tps_count -gt 0 ]; then
            avg_tps=$(echo "scale=2; $tps_sum / $tps_count" | bc)
        fi
        
        # 보고서 작성
        cat > "$report_file" << EOF
Eirene 메인넷 성능 테스트 보고서
================================
테스트 ID: $TEST_ID
보고서 생성 시간: $(date -d @$timestamp '+%Y-%m-%d %H:%M:%S')
경과 시간: ${elapsed}초 / ${TEST_DURATION}초 ($(echo "scale=2; $elapsed * 100 / $TEST_DURATION" | bc)%)

성능 지표:
---------
현재 블록 높이: $block_height
최근 블록 트랜잭션 수: $tx_count
평균 TPS: $avg_tps
목표 TPS 달성률: $(echo "scale=2; $avg_tps * 100 / $TPS_TARGET" | bc)%

리소스 사용량:
------------
메모리 사용량: $(echo "scale=2; $memory_usage / 1024" | bc) MB
CPU 사용률: ${cpu_usage}%
네트워크 연결 수: $connections
피어 수: $peers

테스트 진행 상황:
--------------
생성된 트랜잭션 수: $(ls "$OUTPUT_DIR/transactions/" | wc -l)
수집된 블록 수: $(ls "$OUTPUT_DIR/blocks/" | wc -l)
EOF
        
        log_info "보고서가 생성되었습니다: $report_file"
        
        # 요약 출력
        log_info "현재 블록 높이: $block_height, 평균 TPS: $avg_tps, 목표 TPS 달성률: $(echo "scale=2; $avg_tps * 100 / $TPS_TARGET" | bc)%"
    else
        log_warn "메트릭 데이터를 찾을 수 없습니다."
    fi
}

# 최종 보고서 생성 함수
generate_final_report() {
    local timestamp=$(date +%s)
    local elapsed=$((timestamp - START_TIME))
    local final_report_file="$OUTPUT_DIR/final_report_$TEST_ID.txt"
    
    log_step "최종 성능 테스트 보고서 생성 중..."
    
    # 모든 TPS 파일 분석
    local tps_files=$(ls -t "$OUTPUT_DIR/metrics/tps_"*.txt)
    local tps_count=0
    local tps_sum=0
    local tps_max=0
    local tps_min=9999999
    
    for tps_file in $tps_files; do
        local tps_value=$(cat "$tps_file")
        tps_sum=$(echo "scale=2; $tps_sum + $tps_value" | bc)
        tps_count=$((tps_count + 1))
        
        if (( $(echo "$tps_value > $tps_max" | bc -l) )); then
            tps_max=$tps_value
        fi
        
        if (( $(echo "$tps_value < $tps_min" | bc -l) )); then
            tps_min=$tps_value
        fi
    done
    
    local avg_tps=0
    if [ $tps_count -gt 0 ]; then
        avg_tps=$(echo "scale=2; $tps_sum / $tps_count" | bc)
    fi
    
    # 블록 생성 시간 분석
    local block_files=$(ls -t "$OUTPUT_DIR/blocks/block_"*.json)
    local block_times=()
    local prev_time=""
    
    for block_file in $block_files; do
        local block_time=$(jq -r '.block.header.time' "$block_file")
        local block_time_sec=$(date -d "$block_time" +%s)
        
        if [ -n "$prev_time" ]; then
            local time_diff=$((prev_time - block_time_sec))
            block_times+=($time_diff)
        fi
        
        prev_time=$block_time_sec
    done
    
    local block_time_sum=0
    local block_time_count=0
    local block_time_max=0
    local block_time_min=9999999
    
    for time_diff in "${block_times[@]}"; do
        block_time_sum=$((block_time_sum + time_diff))
        block_time_count=$((block_time_count + 1))
        
        if [ $time_diff -gt $block_time_max ]; then
            block_time_max=$time_diff
        fi
        
        if [ $time_diff -lt $block_time_min ]; then
            block_time_min=$time_diff
        fi
    done
    
    local avg_block_time=0
    if [ $block_time_count -gt 0 ]; then
        avg_block_time=$(echo "scale=2; $block_time_sum / $block_time_count" | bc)
    fi
    
    # 최종 보고서 작성
    cat > "$final_report_file" << EOF
Eirene 메인넷 성능 테스트 최종 보고서
===================================
테스트 ID: $TEST_ID
시작 시간: $(date -d @$START_TIME '+%Y-%m-%d %H:%M:%S')
종료 시간: $(date -d @$timestamp '+%Y-%m-%d %H:%M:%S')
총 테스트 시간: ${elapsed}초

테스트 설정:
----------
테스트 모드: $TEST_MODE
목표 TPS: $TPS_TARGET
트랜잭션 배치 크기: $TX_BATCH_SIZE
트랜잭션 간격: $TX_INTERVAL초

성능 요약:
--------
평균 TPS: $avg_tps
최대 TPS: $tps_max
최소 TPS: $tps_min
목표 TPS 달성률: $(echo "scale=2; $avg_tps * 100 / $TPS_TARGET" | bc)%

블록 생성 시간:
------------
평균 블록 생성 시간: ${avg_block_time}초
최대 블록 생성 시간: ${block_time_max}초
최소 블록 생성 시간: ${block_time_min}초

리소스 사용량 요약:
---------------
최대 메모리 사용량: $(cat "$OUTPUT_DIR/metrics/metrics_"*.json | jq -r '.memory_usage_kb' | sort -nr | head -n 1 | awk '{print $1/1024}') MB
최대 CPU 사용률: $(cat "$OUTPUT_DIR/metrics/metrics_"*.json | jq -r '.cpu_usage_percent' | sort -nr | head -n 1)%
평균 네트워크 연결 수: $(cat "$OUTPUT_DIR/metrics/metrics_"*.json | jq -r '.connections' | awk '{ sum += $1 } END { print sum/NR }')

테스트 통계:
----------
생성된 총 트랜잭션 수: $(ls "$OUTPUT_DIR/transactions/" | wc -l)
수집된 총 블록 수: $(ls "$OUTPUT_DIR/blocks/" | wc -l)
생성된 보고서 수: $(ls "$OUTPUT_DIR/reports/" | wc -l)

결론:
----
$(if (( $(echo "$avg_tps >= $TPS_TARGET" | bc -l) )); then
    echo "테스트 성공: 목표 TPS를 달성했습니다."
else
    echo "테스트 실패: 목표 TPS를 달성하지 못했습니다."
fi)

$(if [ $avg_block_time -le 4 ]; then
    echo "블록 생성 시간이 목표 범위 내에 있습니다."
else
    echo "블록 생성 시간이 목표보다 깁니다. 네트워크 최적화가 필요합니다."
fi)

추가 관찰 사항:
------------
- 테스트 중 발생한 오류 수: $(grep -c "ERROR" "$OUTPUT_DIR/transactions/"*.json 2>/dev/null || echo "0")
- 네트워크 안정성: $(if [ $(cat "$OUTPUT_DIR/metrics/metrics_"*.json | jq -r '.peers' | sort -n | head -n 1) -eq $(cat "$OUTPUT_DIR/metrics/metrics_"*.json | jq -r '.peers' | sort -n | tail -n 1) ]; then echo "안정적"; else echo "불안정"; fi)
EOF
    
    log_info "최종 보고서가 생성되었습니다: $final_report_file"
    cat "$final_report_file"
}

# 메인 테스트 루프
log_step "성능 테스트 시작..."
END_TIME=$((START_TIME + TEST_DURATION))
NEXT_REPORT_TIME=$((START_TIME + REPORT_INTERVAL))
NEXT_TX_TIME=$START_TIME

while true; do
    CURRENT_TIME=$(date +%s)
    
    # 테스트 종료 조건 확인
    if [ $CURRENT_TIME -ge $END_TIME ]; then
        log_info "테스트 지속 시간에 도달했습니다. 테스트를 종료합니다."
        break
    fi
    
    # 성능 측정
    measure_performance
    
    # 트랜잭션 생성 시간 확인
    if [ $CURRENT_TIME -ge $NEXT_TX_TIME ]; then
        generate_transactions $TX_BATCH_SIZE $TX_INTERVAL
        NEXT_TX_TIME=$((CURRENT_TIME + 5))  # 5초마다 트랜잭션 배치 생성
    fi
    
    # 보고서 생성 시간 확인
    if [ $CURRENT_TIME -ge $NEXT_REPORT_TIME ]; then
        generate_report
        NEXT_REPORT_TIME=$((CURRENT_TIME + REPORT_INTERVAL))
    fi
    
    # 잠시 대기
    sleep 1
done

# 최종 보고서 생성
generate_final_report

log_info "성능 테스트가 완료되었습니다."
log_info "테스트 결과는 $OUTPUT_DIR 디렉토리에 저장되었습니다." 