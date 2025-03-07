# Eirene 테스트넷 사용자 가이드

이 가이드는 Eirene 테스트넷에 참여하고 사용하는 방법을 설명합니다.

## 목차

1. [소개](#소개)
2. [시스템 요구사항](#시스템-요구사항)
3. [설치 방법](#설치-방법)
4. [테스트넷 노드 실행](#테스트넷-노드-실행)
5. [계정 관리](#계정-관리)
6. [토큰 요청 및 전송](#토큰-요청-및-전송)
7. [스테이킹 및 위임](#스테이킹-및-위임)
8. [거버넌스 참여](#거버넌스-참여)
9. [문제 해결](#문제-해결)
10. [자주 묻는 질문](#자주-묻는-질문)

## 소개

Eirene는 Zenanet 블록체인을 위한 Proof-of-Stake(PoS) 합의 알고리즘입니다. 테스트넷은 메인넷 출시 전에 Eirene의 기능을 테스트하고 검증하기 위한 환경입니다.

테스트넷에 참여함으로써 다음과 같은 이점이 있습니다:
- 실제 환경과 유사한 조건에서 Eirene의 기능 테스트
- 네트워크 안정성 및 성능 검증에 기여
- 버그 발견 및 보고를 통한 생태계 기여
- 메인넷 출시 전 경험 축적

## 시스템 요구사항

Eirene 테스트넷 노드를 실행하기 위한 최소 시스템 요구사항은 다음과 같습니다:

- **운영체제**: Ubuntu 20.04 LTS 이상, macOS 10.15 이상, Windows 10 이상
- **CPU**: 4코어 이상
- **메모리**: 8GB RAM 이상
- **디스크**: 100GB 이상의 SSD (빠른 I/O 성능 필요)
- **네트워크**: 안정적인 인터넷 연결, 최소 10Mbps 대역폭
- **포트**: 26656(P2P), 26657(RPC), 26660(프로메테우스) 포트 개방 필요

검증자 노드를 실행하려면 다음과 같은 추가 요구사항이 있습니다:
- **CPU**: 8코어 이상
- **메모리**: 16GB RAM 이상
- **디스크**: 200GB 이상의 SSD
- **네트워크**: 안정적인 인터넷 연결, 최소 50Mbps 대역폭
- **가용성**: 99.9% 이상의 업타임 유지 필요

## 설치 방법

### 사전 요구사항 설치

```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y build-essential git curl jq

# macOS
brew install git curl jq

# Go 설치 (1.20 이상)
wget https://go.dev/dl/go1.20.5.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.20.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.profile
source ~/.profile
```

### Eirene 소스 코드 다운로드 및 빌드

```bash
# 저장소 클론
git clone https://github.com/zenanetwork/go-zenanet.git
cd go-zenanet

# CLI 도구 빌드
cd consensus/eirene/cli
go build
```

### 테스트넷 설정 파일 다운로드

```bash
# 테스트넷 디렉토리로 이동
cd ../testnet

# 테스트넷 초기화 스크립트 실행 권한 부여
chmod +x scripts/init_testnet.sh
chmod +x scripts/start_testnet.sh
chmod +x scripts/stop_testnet.sh
chmod +x scripts/status_testnet.sh
chmod +x scripts/monitor_testnet.sh
```

## 테스트넷 노드 실행

### 로컬 테스트넷 실행

로컬 테스트넷은 개발 및 테스트 목적으로 단일 머신에서 여러 노드를 실행하는 환경입니다.

```bash
# 테스트넷 초기화
./scripts/init_testnet.sh

# 테스트넷 시작
./scripts/start_testnet.sh

# 테스트넷 상태 확인
./scripts/status_testnet.sh

# 테스트넷 중지
./scripts/stop_testnet.sh
```

### 공개 테스트넷 참여

공개 테스트넷은 여러 참여자가 함께 운영하는 분산 네트워크입니다.

```bash
# 노드 초기화
../cli/cli node init --chain-id="eirene-testnet-1" --home="$HOME/.eirene"

# 제네시스 파일 및 seed 노드 설정 다운로드
curl -s https://raw.githubusercontent.com/zenanetwork/testnet/main/eirene-testnet-1/genesis.json > $HOME/.eirene/config/genesis.json
curl -s https://raw.githubusercontent.com/zenanetwork/testnet/main/eirene-testnet-1/seeds.txt > $HOME/.eirene/config/seeds.txt

# seed 노드 설정
SEEDS=$(cat $HOME/.eirene/config/seeds.txt)
../cli/cli node config set --home="$HOME/.eirene" --key="p2p.seeds" --value="$SEEDS"

# 노드 시작
../cli/cli node start --home="$HOME/.eirene"
```

## 계정 관리

### 계정 생성

```bash
../cli/cli account create --name="my-account"
```

### 계정 목록 조회

```bash
../cli/cli account list
```

### 계정 정보 조회

```bash
../cli/cli account show --name="my-account"
```

### 계정 복구

```bash
../cli/cli account recover --name="my-account" --mnemonic="your mnemonic words here"
```

## 토큰 요청 및 전송

### 테스트넷 토큰 요청

테스트넷 토큰은 [Eirene 테스트넷 Faucet](https://faucet.eirene.zenanetwork.com)에서 요청할 수 있습니다.

1. 웹 브라우저에서 Faucet 웹사이트 방문
2. 계정 주소 입력
3. "Request Tokens" 버튼 클릭
4. 하루에 최대 100 ZENA 토큰 요청 가능

### 토큰 전송

```bash
../cli/cli tx send --from="my-account" --to="recipient-address" --amount="10zena"
```

## 스테이킹 및 위임

### 검증자 생성

```bash
../cli/cli staking create-validator --from="my-account" --amount="100000zena" --pubkey="validator-pubkey" --moniker="my-validator" --website="https://example.com" --details="My validator details" --commission-rate="0.1" --commission-max-rate="0.2" --commission-max-change-rate="0.01" --min-self-delegation="1"
```

### 토큰 위임

```bash
../cli/cli staking delegate --from="my-account" --validator="validator-address" --amount="10zena"
```

### 위임 취소

```bash
../cli/cli staking unbond --from="my-account" --validator="validator-address" --amount="10zena"
```

### 재위임

```bash
../cli/cli staking redelegate --from="my-account" --src-validator="source-validator-address" --dst-validator="destination-validator-address" --amount="10zena"
```

### 보상 인출

```bash
../cli/cli staking withdraw-rewards --from="my-account" --validator="validator-address"
```

## 거버넌스 참여

### 제안 생성

```bash
../cli/cli governance submit-proposal --from="my-account" --type="text" --title="My Proposal" --description="This is my proposal description" --deposit="10zena"
```

### 제안 조회

```bash
../cli/cli governance query-proposal --proposal-id=1
```

### 제안 목록 조회

```bash
../cli/cli governance list-proposals
```

### 투표

```bash
../cli/cli governance vote --from="my-account" --proposal-id=1 --option="yes"
```

### 예치금 추가

```bash
../cli/cli governance deposit --from="my-account" --proposal-id=1 --amount="10zena"
```

## 문제 해결

### 노드 연결 문제

- **문제**: 노드가 피어에 연결되지 않습니다.
- **해결 방법**:
  1. 방화벽 설정 확인: 26656 포트가 열려 있는지 확인
  2. `seeds` 및 `persistent_peers` 설정 확인
  3. 네트워크 연결 상태 확인

### 동기화 문제

- **문제**: 노드가 블록체인과 동기화되지 않습니다.
- **해결 방법**:
  1. 로그 확인: `tail -f $HOME/.eirene/eirene.log`
  2. 디스크 공간 확인: `df -h`
  3. 시스템 리소스 확인: `top` 또는 `htop`

### 트랜잭션 실패

- **문제**: 트랜잭션이 실패합니다.
- **해결 방법**:
  1. 계정 잔액 확인: `../cli/cli account show --name="my-account"`
  2. 트랜잭션 로그 확인: `../cli/cli tx show --hash="tx-hash"`
  3. 노드 상태 확인: `../cli/cli node status`

## 자주 묻는 질문

### 테스트넷 토큰은 실제 가치가 있나요?

아니요, 테스트넷 토큰은 테스트 목적으로만 사용되며 실제 가치는 없습니다.

### 검증자가 되려면 어떻게 해야 하나요?

검증자가 되려면 충분한 토큰을 스테이킹하고 검증자 노드를 설정해야 합니다. 자세한 내용은 [스테이킹 및 위임](#스테이킹-및-위임) 섹션을 참조하세요.

### 테스트넷은 얼마나 자주 리셋되나요?

테스트넷은 주요 업데이트나 문제 발생 시 리셋될 수 있습니다. 리셋 일정은 [공식 Discord 채널](https://discord.gg/zenanetwork)에서 공지됩니다.

### 버그를 발견했을 때 어떻게 보고하나요?

버그를 발견하면 [GitHub 이슈](https://github.com/zenanetwork/go-zenanet/issues)에 보고해 주세요. 버그 보고 시 다음 정보를 포함해 주세요:
- 버그 설명
- 재현 방법
- 예상 동작과 실제 동작
- 시스템 환경 (OS, Go 버전 등)
- 로그 파일 또는 오류 메시지

### 테스트넷 참여에 대한 보상이 있나요?

테스트넷 참여자들은 버그 바운티 프로그램이나 테스트넷 인센티브 프로그램을 통해 보상을 받을 수 있습니다. 자세한 내용은 [공식 웹사이트](https://zenanetwork.com/testnet)를 참조하세요. 