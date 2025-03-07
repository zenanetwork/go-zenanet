# Eirene 메인넷 배포 가이드

이 문서는 Eirene 합의 알고리즘을 사용하는 Zenanet 메인넷을 배포하는 방법을 설명합니다.

## 목차

1. [개요](#개요)
2. [사전 준비 사항](#사전-준비-사항)
3. [제네시스 파일 생성](#제네시스-파일-생성)
4. [검증자 노드 설정](#검증자-노드-설정)
5. [일반 노드 설정](#일반-노드-설정)
6. [네트워크 시작](#네트워크-시작)
7. [네트워크 모니터링](#네트워크-모니터링)
8. [문제 해결](#문제-해결)
9. [업그레이드 절차](#업그레이드-절차)

## 개요

Eirene 메인넷 배포는 다음과 같은 주요 단계로 구성됩니다:

1. 제네시스 파일 생성 및 배포
2. 초기 검증자 노드 설정
3. 네트워크 시작 및 동기화
4. 추가 노드 온보딩
5. 네트워크 모니터링 및 유지 관리

이 가이드는 각 단계를 자세히 설명하고, 메인넷을 성공적으로 배포하기 위한 모범 사례를 제공합니다.

## 사전 준비 사항

### 하드웨어 요구사항

**검증자 노드:**
- CPU: 8코어 이상
- RAM: 32GB 이상
- 디스크: 1TB SSD 이상
- 네트워크: 100Mbps 이상, 안정적인 연결

**일반 노드:**
- CPU: 4코어 이상
- RAM: 16GB 이상
- 디스크: 500GB SSD 이상
- 네트워크: 50Mbps 이상

### 소프트웨어 요구사항

- 운영체제: Ubuntu 20.04 LTS 이상
- Go 버전: 1.20 이상
- Eirene 릴리스 버전: v1.0.0

### 네트워크 요구사항

- 고정 IP 주소
- 다음 포트 개방:
  - P2P 통신: 26656
  - RPC 서버: 26657
  - gRPC 서버: 9090
  - API 서버: 1317
  - 프로메테우스: 26660

### 보안 요구사항

- SSH 키 기반 인증
- 방화벽 설정
- 시스템 업데이트 자동화
- 하드웨어 보안 모듈(HSM) 또는 키 관리 솔루션 (권장)

## 제네시스 파일 생성

### 1. 제네시스 파일 템플릿 준비

```bash
# Eirene 소스 코드 다운로드
git clone https://github.com/zenanetwork/go-zenanet.git
cd go-zenanet

# 메인넷 릴리스 체크아웃
git checkout v1.0.0

# CLI 도구 빌드
cd consensus/eirene/cli
go build
```

### 2. 제네시스 파일 초기화

```bash
# 제네시스 파일 초기화
./cli genesis init --chain-id="zenanet-1" --output="genesis.json"

# 제네시스 시간 설정 (UTC 기준)
./cli genesis set-time --genesis="genesis.json" --time="2025-01-01T00:00:00Z"
```

### 3. 초기 매개변수 설정

```bash
# 합의 매개변수 설정
./cli genesis set-consensus-params --genesis="genesis.json" \
  --block-max-bytes="22020096" \
  --block-max-gas="-1" \
  --block-time-iota-ms="1000" \
  --evidence-max-age-num-blocks="100000" \
  --evidence-max-age-duration="172800000000000"

# Eirene 합의 매개변수 설정
./cli genesis set-eirene-params --genesis="genesis.json" \
  --period="4" \
  --epoch="30000" \
  --slashing-threshold="100" \
  --slashing-rate="10" \
  --missed-block-penalty="1"

# 스테이킹 매개변수 설정
./cli genesis set-staking-params --genesis="genesis.json" \
  --unbonding-time="1814400000000000" \
  --max-validators="100" \
  --max-entries="7" \
  --historical-entries="10000" \
  --bond-denom="uzena"

# 거버넌스 매개변수 설정
./cli genesis set-gov-params --genesis="genesis.json" \
  --min-deposit="10000000000uzena" \
  --max-deposit-period="1209600000000000" \
  --voting-period="1209600000000000" \
  --quorum="0.400000000000000000" \
  --threshold="0.500000000000000000" \
  --veto-threshold="0.334000000000000000"
```

### 4. 초기 토큰 분배 설정

```bash
# 생태계 기금 계정 추가
./cli genesis add-account --genesis="genesis.json" \
  --address="zena1ecosystem..." \
  --amount="300000000000000uzena" \
  --vesting-start-time="2025-01-01T00:00:00Z" \
  --vesting-end-time="2029-01-01T00:00:00Z"

# 팀 및 어드바이저 계정 추가
./cli genesis add-account --genesis="genesis.json" \
  --address="zena1team..." \
  --amount="200000000000000uzena" \
  --vesting-start-time="2025-01-01T00:00:00Z" \
  --vesting-end-time="2027-01-01T00:00:00Z"

# 초기 투자자 계정 추가
./cli genesis add-account --genesis="genesis.json" \
  --address="zena1investors..." \
  --amount="150000000000000uzena" \
  --vesting-start-time="2025-01-01T00:00:00Z" \
  --vesting-end-time="2026-01-01T00:00:00Z"

# 커뮤니티 보상 계정 추가
./cli genesis add-account --genesis="genesis.json" \
  --address="zena1community..." \
  --amount="250000000000000uzena"

# 검증자 인센티브 계정 추가
./cli genesis add-account --genesis="genesis.json" \
  --address="zena1validators..." \
  --amount="100000000000000uzena"
```

### 5. 초기 검증자 설정

```bash
# 초기 검증자 추가 (각 검증자마다 반복)
./cli genesis add-validator --genesis="genesis.json" \
  --pubkey="zenavalconspub1..." \
  --amount="10000000000uzena" \
  --moniker="validator-1" \
  --commission-rate="0.100000000000000000" \
  --commission-max-rate="0.200000000000000000" \
  --commission-max-change-rate="0.010000000000000000" \
  --min-self-delegation="1000000" \
  --website="https://validator1.example.com" \
  --details="Validator 1 description" \
  --security-contact="security@validator1.example.com"
```

### 6. 제네시스 파일 검증

```bash
# 제네시스 파일 검증
./cli genesis validate --genesis="genesis.json"
```

### 7. 제네시스 파일 배포

제네시스 파일을 다음 채널을 통해 배포합니다:
- 공식 웹사이트
- GitHub 저장소
- IPFS
- 검증자 커뮤니티 채널

각 검증자는 제네시스 파일의 SHA256 해시를 확인해야 합니다:

```bash
sha256sum genesis.json
```

## 검증자 노드 설정

### 1. 노드 초기화

```bash
# 노드 홈 디렉토리 설정
export EIRENE_HOME="$HOME/.eirene"

# 노드 초기화
./cli node init --home="$EIRENE_HOME" --chain-id="zenanet-1"
```

### 2. 제네시스 파일 복사

```bash
# 제네시스 파일 다운로드 및 복사
curl -s https://raw.githubusercontent.com/zenanetwork/mainnet/main/genesis.json > $EIRENE_HOME/config/genesis.json

# 해시 확인
echo "$(sha256sum $EIRENE_HOME/config/genesis.json) | grep <공식 해시>"
```

### 3. 노드 구성

```bash
# 시드 노드 설정
./cli node config set --home="$EIRENE_HOME" --key="p2p.seeds" --value="id1@seed1.zenanet.com:26656,id2@seed2.zenanet.com:26656"

# 피어 노드 설정
./cli node config set --home="$EIRENE_HOME" --key="p2p.persistent_peers" --value="id1@peer1.zenanet.com:26656,id2@peer2.zenanet.com:26656"

# P2P 설정
./cli node config set --home="$EIRENE_HOME" --key="p2p.laddr" --value="tcp://0.0.0.0:26656"
./cli node config set --home="$EIRENE_HOME" --key="p2p.external_address" --value="tcp://<your-public-ip>:26656"
./cli node config set --home="$EIRENE_HOME" --key="p2p.max_num_inbound_peers" --value="100"
./cli node config set --home="$EIRENE_HOME" --key="p2p.max_num_outbound_peers" --value="50"

# RPC 설정
./cli node config set --home="$EIRENE_HOME" --key="rpc.laddr" --value="tcp://0.0.0.0:26657"

# 상태 동기화 설정 (선택 사항)
./cli node config set --home="$EIRENE_HOME" --key="statesync.enable" --value="true"
./cli node config set --home="$EIRENE_HOME" --key="statesync.rpc_servers" --value="rpc1.zenanet.com:26657,rpc2.zenanet.com:26657"
```

### 4. 검증자 키 설정

```bash
# 검증자 키 생성 (또는 기존 키 복구)
./cli account new --name="validator" --home="$EIRENE_HOME"
# 또는
./cli account recover --name="validator" --home="$EIRENE_HOME"

# 검증자 키 확인
./cli account show --name="validator" --home="$EIRENE_HOME"
```

### 5. 시스템 서비스 설정

```bash
# 시스템 서비스 파일 생성
sudo tee /etc/systemd/system/eirene.service > /dev/null << EOF
[Unit]
Description=Eirene Node
After=network-online.target

[Service]
User=$USER
ExecStart=$(which ./cli) node start --home="$EIRENE_HOME"
Restart=always
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

# 시스템 서비스 활성화
sudo systemctl daemon-reload
sudo systemctl enable eirene
```

## 일반 노드 설정

일반 노드 설정은 검증자 노드 설정과 유사하지만, 검증자 키 설정 단계를 건너뛸 수 있습니다. 또한 다음과 같은 추가 설정을 고려할 수 있습니다:

```bash
# API 서버 활성화
./cli node config set --home="$EIRENE_HOME" --key="api.enable" --value="true"
./cli node config set --home="$EIRENE_HOME" --key="api.address" --value="tcp://0.0.0.0:1317"

# gRPC 서버 활성화
./cli node config set --home="$EIRENE_HOME" --key="grpc.enable" --value="true"
./cli node config set --home="$EIRENE_HOME" --key="grpc.address" --value="0.0.0.0:9090"

# 프로메테우스 메트릭 활성화
./cli node config set --home="$EIRENE_HOME" --key="instrumentation.prometheus" --value="true"
./cli node config set --home="$EIRENE_HOME" --key="instrumentation.prometheus_listen_addr" --value=":26660"
```

## 네트워크 시작

### 1. 제네시스 시간 확인

제네시스 파일에 지정된 시간을 확인하고, 해당 시간에 맞춰 노드를 시작합니다:

```bash
# 제네시스 시간 확인
cat $EIRENE_HOME/config/genesis.json | jq '.genesis_time'
```

### 2. 노드 시작

```bash
# 시스템 서비스로 노드 시작
sudo systemctl start eirene

# 로그 확인
sudo journalctl -u eirene -f
```

### 3. 노드 상태 확인

```bash
# 노드 상태 확인
./cli node status --home="$EIRENE_HOME"

# 동기화 상태 확인
./cli node status --home="$EIRENE_HOME" | jq '.sync_info'
```

### 4. 검증자 생성 (검증자 노드만 해당)

노드가 완전히 동기화된 후, 검증자를 생성합니다:

```bash
# 검증자 생성
./cli staking create-validator \
  --from="validator" \
  --amount="10000000000uzena" \
  --pubkey=$(./cli node show-validator --home="$EIRENE_HOME") \
  --moniker="<your-validator-name>" \
  --website="<your-website>" \
  --details="<your-description>" \
  --commission-rate="0.10" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --min-self-delegation="1000000" \
  --home="$EIRENE_HOME" \
  --chain-id="zenanet-1"
```

## 네트워크 모니터링

### 1. 로그 모니터링

```bash
# 실시간 로그 확인
sudo journalctl -u eirene -f

# 오류 로그 확인
sudo journalctl -u eirene -f | grep -i error
```

### 2. 노드 상태 모니터링

```bash
# 노드 상태 확인 스크립트
cat > monitor.sh << 'EOF'
#!/bin/bash
while true; do
  echo "$(date) - Node Status:"
  ./cli node status --home="$EIRENE_HOME" | jq '{catching_up: .sync_info.catching_up, latest_block_height: .sync_info.latest_block_height, latest_block_time: .sync_info.latest_block_time, voting_power: .validator_info.voting_power, peers: .n_peers}'
  echo ""
  sleep 60
done
EOF
chmod +x monitor.sh
```

### 3. 프로메테우스 및 Grafana 설정

프로메테우스 설정 파일 (`prometheus.yml`):

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'eirene'
    static_configs:
      - targets: ['localhost:26660']
```

Grafana 대시보드를 설정하여 다음 지표를 모니터링합니다:
- 블록 높이 및 생성 시간
- 검증자 상태 및 투표력
- 피어 연결 수
- 메모리 및 CPU 사용량
- 디스크 사용량
- 네트워크 트래픽

### 4. 알림 설정

중요한 이벤트에 대한 알림을 설정합니다:
- 노드 다운
- 검증자 미서명 블록
- 디스크 공간 부족
- 높은 CPU 또는 메모리 사용량
- 네트워크 연결 문제

## 문제 해결

### 1. 동기화 문제

노드가 동기화되지 않는 경우:

```bash
# 상태 동기화 재설정
./cli node reset-state --home="$EIRENE_HOME"

# 피어 목록 확인 및 업데이트
./cli node config set --home="$EIRENE_HOME" --key="p2p.persistent_peers" --value="<updated-peers>"
```

### 2. 검증자 문제

검증자가 블록에 서명하지 않는 경우:

```bash
# 검증자 상태 확인
./cli staking validator --address=$(./cli account show --name="validator" --home="$EIRENE_HOME" --bech=val)

# 검증자 키 상태 확인
./cli node show-validator --home="$EIRENE_HOME"
```

### 3. 네트워크 문제

네트워크 연결 문제가 있는 경우:

```bash
# 피어 상태 확인
./cli network peers --home="$EIRENE_HOME"

# 방화벽 설정 확인
sudo ufw status
```

### 4. 디스크 공간 문제

디스크 공간이 부족한 경우:

```bash
# 디스크 사용량 확인
df -h

# 데이터베이스 압축 (선택 사항)
./cli node compact --home="$EIRENE_HOME"
```

## 업그레이드 절차

### 1. 소프트웨어 업그레이드 제안

거버넌스 제안을 통해 소프트웨어 업그레이드를 제안합니다:

```bash
# 소프트웨어 업그레이드 제안
./cli governance submit-proposal software-upgrade \
  --title="Upgrade to v1.1.0" \
  --description="This upgrade includes performance improvements and bug fixes" \
  --upgrade-height=1000000 \
  --upgrade-info='{"binaries":{"linux/amd64":"https://github.com/zenanetwork/go-zenanet/releases/download/v1.1.0/eirene_1.1.0_linux_amd64.tar.gz"}}' \
  --from="validator" \
  --deposit="10000000000uzena" \
  --home="$EIRENE_HOME" \
  --chain-id="zenanet-1"
```

### 2. 투표 및 승인

제안에 대한 투표를 진행합니다:

```bash
# 제안 조회
./cli governance query-proposal --proposal-id=1

# 제안에 투표
./cli governance vote \
  --proposal-id=1 \
  --option="yes" \
  --from="validator" \
  --home="$EIRENE_HOME" \
  --chain-id="zenanet-1"
```

### 3. 업그레이드 준비

제안이 승인되면 업그레이드를 준비합니다:

```bash
# 새 버전 다운로드
wget https://github.com/zenanetwork/go-zenanet/releases/download/v1.1.0/eirene_1.1.0_linux_amd64.tar.gz
tar -xzf eirene_1.1.0_linux_amd64.tar.gz

# 백업 생성
cp -r $EIRENE_HOME $EIRENE_HOME.backup
```

### 4. 업그레이드 실행

지정된 블록 높이에 도달하면 노드가 자동으로 중지됩니다. 이때 새 버전으로 업그레이드합니다:

```bash
# 노드 중지
sudo systemctl stop eirene

# 바이너리 교체
sudo cp ./eirene_1.1.0/cli /usr/local/bin/eirene

# 노드 재시작
sudo systemctl start eirene

# 로그 확인
sudo journalctl -u eirene -f
```

---

이 가이드는 Eirene 메인넷 배포를 위한 기본 지침을 제공합니다. 실제 배포 과정에서는 네트워크 상황과 요구사항에 따라 조정이 필요할 수 있습니다. 항상 최신 문서와 커뮤니티 채널을 참조하여 최신 정보를 확인하세요. 