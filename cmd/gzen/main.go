// Copyright 2014 The go-zenanet Authors
// This file is part of go-zenanet.
//
// go-zenanet is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-zenanet is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-zenanet. If not, see <http://www.gnu.org/licenses/>.

// gzen is a command-line client for Zenanet.
package main

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/zenanetwork/go-zenanet/accounts"
	"github.com/zenanetwork/go-zenanet/cmd/utils"
	"github.com/zenanetwork/go-zenanet/common"
	"github.com/zenanetwork/go-zenanet/console/prompt"
	"github.com/zenanetwork/go-zenanet/eth/downloader"
	"github.com/zenanetwork/go-zenanet/ethclient"
	"github.com/zenanetwork/go-zenanet/internal/debug"
	"github.com/zenanetwork/go-zenanet/internal/flags"
	"github.com/zenanetwork/go-zenanet/log"
	"github.com/zenanetwork/go-zenanet/node"
	"go.uber.org/automaxprocs/maxprocs"

	// Force-load the tracer engines to trigger registration
	_ "github.com/zenanetwork/go-zenanet/eth/tracers/js"
	_ "github.com/zenanetwork/go-zenanet/eth/tracers/live"
	_ "github.com/zenanetwork/go-zenanet/eth/tracers/native"

	"github.com/urfave/cli/v2"
)

const (
	clientIdentifier = "gzen" // Client identifier to advertise over the network
)

var (
	// flags that configure the node
	nodeFlags = slices.Concat([]cli.Flag{
		utils.IdentityFlag,
		utils.UnlockedAccountFlag,
		utils.PasswordFileFlag,
		utils.BootnodesFlag,
		utils.MinFreeDiskSpaceFlag,
		utils.KeyStoreDirFlag,
		utils.ExternalSignerFlag,
		utils.NoUSBFlag, // deprecated
		utils.USBFlag,
		utils.SmartCardDaemonPathFlag,
		utils.OverrideCancun,
		utils.OverrideVerkle,
		utils.EnablePersonal, // deprecated
		utils.TxPoolLocalsFlag,
		utils.TxPoolNoLocalsFlag,
		utils.TxPoolJournalFlag,
		utils.TxPoolRejournalFlag,
		utils.TxPoolPriceLimitFlag,
		utils.TxPoolPriceBumpFlag,
		utils.TxPoolAccountSlotsFlag,
		utils.TxPoolGlobalSlotsFlag,
		utils.TxPoolAccountQueueFlag,
		utils.TxPoolGlobalQueueFlag,
		utils.TxPoolLifetimeFlag,
		utils.BlobPoolDataDirFlag,
		utils.BlobPoolDataCapFlag,
		utils.BlobPoolPriceBumpFlag,
		utils.SyncModeFlag,
		utils.SyncTargetFlag,
		utils.ExitWhenSyncedFlag,
		utils.GCModeFlag,
		utils.SnapshotFlag,
		utils.TxLookupLimitFlag, // deprecated
		utils.TransactionHistoryFlag,
		utils.StateHistoryFlag,
		utils.LightServeFlag,    // deprecated
		utils.LightIngressFlag,  // deprecated
		utils.LightEgressFlag,   // deprecated
		utils.LightMaxPeersFlag, // deprecated
		utils.LightNoPruneFlag,  // deprecated
		utils.LightKDFFlag,
		utils.LightNoSyncServeFlag, // deprecated
		utils.EthRequiredBlocksFlag,
		utils.LegacyWhitelistFlag, // deprecated
		utils.BloomFilterSizeFlag,
		utils.CacheFlag,
		utils.CacheDatabaseFlag,
		utils.CacheTrieFlag,
		utils.CacheTrieJournalFlag,   // deprecated
		utils.CacheTrieRejournalFlag, // deprecated
		utils.CacheGCFlag,
		utils.CacheSnapshotFlag,
		utils.CacheNoPrefetchFlag,
		utils.CachePreimagesFlag,
		utils.CacheLogSizeFlag,
		utils.FDLimitFlag,
		utils.CryptoKZGFlag,
		utils.ListenPortFlag,
		utils.DiscoveryPortFlag,
		utils.MaxPeersFlag,
		utils.MaxPendingPeersFlag,
		utils.MiningEnabledFlag, // deprecated
		utils.MinerGasLimitFlag,
		utils.MinerGasPriceFlag,
		utils.MinerZenbaseFlag, // deprecated
		utils.MinerExtraDataFlag,
		utils.MinerRecommitIntervalFlag,
		utils.MinerPendingFeeRecipientFlag,
		utils.MinerNewPayloadTimeoutFlag, // deprecated
		utils.NATFlag,
		utils.NoDiscoverFlag,
		utils.DiscoveryV4Flag,
		utils.DiscoveryV5Flag,
		utils.LegacyDiscoveryV5Flag, // deprecated
		utils.NetrestrictFlag,
		utils.NodeKeyFileFlag,
		utils.NodeKeyHexFlag,
		utils.DNSDiscoveryFlag,
		utils.DeveloperFlag,
		utils.DeveloperGasLimitFlag,
		utils.DeveloperPeriodFlag,
		utils.VMEnableDebugFlag,
		utils.VMTraceFlag,
		utils.VMTraceJsonConfigFlag,
		utils.NetworkIdFlag,
		utils.EthStatsURLFlag,
		utils.NoCompactionFlag,
		utils.GpoBlocksFlag,
		utils.GpoPercentileFlag,
		utils.GpoMaxGasPriceFlag,
		utils.GpoIgnoreGasPriceFlag,
		configFileFlag,
		utils.LogDebugFlag,
		utils.LogBacktraceAtFlag,
		utils.BeaconApiFlag,
		utils.BeaconApiHeaderFlag,
		utils.BeaconThresholdFlag,
		utils.BeaconNoFilterFlag,
		utils.BeaconConfigFlag,
		utils.BeaconGenesisRootFlag,
		utils.BeaconGenesisTimeFlag,
		utils.BeaconCheckpointFlag,
	}, utils.NetworkFlags, utils.DatabaseFlags)

	// Eirene 관련 플래그
	eireneFlags = []cli.Flag{
		&cli.UintFlag{
			Name:     "eirene.period",
			Usage:    "블록 생성 주기(초)",
			Value:    4,
			Category: flags.EthCategory,
		},
		&cli.UintFlag{
			Name:     "eirene.epoch",
			Usage:    "에포크 길이(블록 수)",
			Value:    30000,
			Category: flags.EthCategory,
		},
		&cli.UintFlag{
			Name:     "eirene.slashing-threshold",
			Usage:    "슬래싱 임계값",
			Value:    100,
			Category: flags.EthCategory,
		},
		&cli.UintFlag{
			Name:     "eirene.slashing-rate",
			Usage:    "슬래싱 비율(1/1000)",
			Value:    10,
			Category: flags.EthCategory,
		},
		&cli.UintFlag{
			Name:     "eirene.missed-block-penalty",
			Usage:    "블록 생성 실패 시 페널티",
			Value:    1,
			Category: flags.EthCategory,
		},
	}

	rpcFlags = []cli.Flag{
		utils.HTTPEnabledFlag,
		utils.HTTPListenAddrFlag,
		utils.HTTPPortFlag,
		utils.HTTPCORSDomainFlag,
		utils.AuthListenFlag,
		utils.AuthPortFlag,
		utils.AuthVirtualHostsFlag,
		utils.JWTSecretFlag,
		utils.HTTPVirtualHostsFlag,
		utils.GraphQLEnabledFlag,
		utils.GraphQLCORSDomainFlag,
		utils.GraphQLVirtualHostsFlag,
		utils.HTTPApiFlag,
		utils.HTTPPathPrefixFlag,
		utils.WSEnabledFlag,
		utils.WSListenAddrFlag,
		utils.WSPortFlag,
		utils.WSApiFlag,
		utils.WSAllowedOriginsFlag,
		utils.WSPathPrefixFlag,
		utils.IPCDisabledFlag,
		utils.IPCPathFlag,
		utils.InsecureUnlockAllowedFlag,
		utils.RPCGlobalGasCapFlag,
		utils.RPCGlobalEVMTimeoutFlag,
		utils.RPCGlobalTxFeeCapFlag,
		utils.AllowUnprotectedTxs,
		utils.BatchRequestLimit,
		utils.BatchResponseMaxSize,
	}

	metricsFlags = []cli.Flag{
		utils.MetricsEnabledFlag,
		utils.MetricsEnabledExpensiveFlag,
		utils.MetricsHTTPFlag,
		utils.MetricsPortFlag,
		utils.MetricsEnableInfluxDBFlag,
		utils.MetricsInfluxDBEndpointFlag,
		utils.MetricsInfluxDBDatabaseFlag,
		utils.MetricsInfluxDBUsernameFlag,
		utils.MetricsInfluxDBPasswordFlag,
		utils.MetricsInfluxDBTagsFlag,
		utils.MetricsEnableInfluxDBV2Flag,
		utils.MetricsInfluxDBTokenFlag,
		utils.MetricsInfluxDBBucketFlag,
		utils.MetricsInfluxDBOrganizationFlag,
	}
)

// Eirene 관련 명령어
var eireneCommand = &cli.Command{
	Name:      "eirene",
	Usage:     "Eirene 합의 알고리즘 관련 명령어",
	ArgsUsage: "",
	Category:  "EIRENE COMMANDS",
	Description: `
Eirene 합의 알고리즘을 사용하는 Zenanet 블록체인을 관리하기 위한 명령어입니다.
`,
	Subcommands: []*cli.Command{
		{
			Name:      "mainnet",
			Usage:     "Eirene 메인넷 실행",
			ArgsUsage: "",
			Action:    runEireneMainnet,
			Flags:     slices.Concat(nodeFlags, rpcFlags, eireneFlags),
			Description: `
Eirene 합의 알고리즘을 사용하는 Zenanet 메인넷을 실행합니다.
`,
		},
		{
			Name:      "testnet",
			Usage:     "Eirene 테스트넷 실행",
			ArgsUsage: "",
			Action:    runEireneTestnet,
			Flags:     slices.Concat(nodeFlags, rpcFlags, eireneFlags),
			Description: `
Eirene 합의 알고리즘을 사용하는 Zenanet 테스트넷을 실행합니다.
`,
		},
		{
			Name:      "local",
			Usage:     "Eirene 로컬 테스트넷 실행",
			ArgsUsage: "",
			Action:    runEireneLocalTestnet,
			Flags:     slices.Concat(nodeFlags, rpcFlags, eireneFlags),
			Description: `
Eirene 합의 알고리즘을 사용하는 로컬 테스트넷을 실행합니다.
`,
		},
	},
}

var app = flags.NewApp("the go-zenanet command line interface")

func init() {
	// Initialize the CLI app and start Gzen
	app.Action = gzen
	app.Commands = []*cli.Command{
		// See chaincmd.go:
		initCommand,
		importCommand,
		exportCommand,
		importHistoryCommand,
		exportHistoryCommand,
		importPreimagesCommand,
		removedbCommand,
		dumpCommand,
		dumpGenesisCommand,
		// See accountcmd.go:
		accountCommand,
		walletCommand,
		// See consolecmd.go:
		consoleCommand,
		attachCommand,
		javascriptCommand,
		// See misccmd.go:
		versionCommand,
		versionCheckCommand,
		licenseCommand,
		// See config.go
		dumpConfigCommand,
		// see dbcmd.go
		dbCommand,
		// See cmd/utils/flags_legacy.go
		utils.ShowDeprecated,
		// See snapshot.go
		snapshotCommand,
		// See verkle.go
		verkleCommand,
		// Eirene 명령어 추가
		eireneCommand,
	}
	if logTestCommand != nil {
		app.Commands = append(app.Commands, logTestCommand)
	}
	sort.Sort(cli.CommandsByName(app.Commands))

	app.Flags = slices.Concat(
		nodeFlags,
		rpcFlags,
		consoleFlags,
		debug.Flags,
		metricsFlags,
	)
	flags.AutoEnvVars(app.Flags, "GZEN")

	app.Before = func(ctx *cli.Context) error {
		maxprocs.Set() // Automatically set GOMAXPROCS to match Linux container CPU quota.
		flags.MigrateGlobalFlags(ctx)
		if err := debug.Setup(ctx); err != nil {
			return err
		}
		flags.CheckEnvVars(ctx, app.Flags, "GZEN")
		return nil
	}
	app.After = func(ctx *cli.Context) error {
		debug.Exit()
		prompt.Stdin.Close() // Resets terminal mode.
		return nil
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// prepare manipulates memory cache allowance and setups metric system.
// This function should be called before launching devp2p stack.
func prepare(ctx *cli.Context) {
	// If we're running a known preset, log it for convenience.
	switch {
	case ctx.IsSet(utils.SepoliaFlag.Name):
		log.Info("Starting Gzen on Sepolia testnet...")

	case ctx.IsSet(utils.HoleskyFlag.Name):
		log.Info("Starting Gzen on Holesky testnet...")

	case ctx.IsSet(utils.DeveloperFlag.Name):
		log.Info("Starting Gzen in ephemeral dev mode...")
		log.Warn(`You are running Gzen in --dev mode. Please note the following:

  1. This mode is only intended for fast, iterative development without assumptions on
     security or persistence.
  2. The database is created in memory unless specified otherwise. Therefore, shutting down
     your computer or losing power will wipe your entire block data and chain state for
     your dev environment.
  3. A random, pre-allocated developer account will be available and unlocked as
     eth.coinbase, which can be used for testing. The random dev account is temporary,
     stored on a ramdisk, and will be lost if your machine is restarted.
  4. Mining is enabled by default. However, the client will only seal blocks if transactions
     are pending in the mempool. The miner's minimum accepted gas price is 1.
  5. Networking is disabled; there is no listen-address, the maximum number of peers is set
     to 0, and discovery is disabled.
`)

	case !ctx.IsSet(utils.NetworkIdFlag.Name):
		log.Info("Starting Gzen on Zenanet mainnet...")
	}
	// If we're a full node on mainnet without --cache specified, bump default cache allowance
	if !ctx.IsSet(utils.CacheFlag.Name) && !ctx.IsSet(utils.NetworkIdFlag.Name) {
		// Make sure we're not on any supported preconfigured testnet either
		if !ctx.IsSet(utils.HoleskyFlag.Name) &&
			!ctx.IsSet(utils.SepoliaFlag.Name) &&
			!ctx.IsSet(utils.DeveloperFlag.Name) {
			// Nope, we're really on mainnet. Bump that cache up!
			log.Info("Bumping default cache on mainnet", "provided", ctx.Int(utils.CacheFlag.Name), "updated", 4096)
			ctx.Set(utils.CacheFlag.Name, strconv.Itoa(4096))
		}
	}
}

// gzen is the main entry point into the system if no special subcommand is run.
// It creates a default node based on the command line arguments and runs it in
// blocking mode, waiting for it to be shut down.
func gzen(ctx *cli.Context) error {
	if args := ctx.Args().Slice(); len(args) > 0 {
		return fmt.Errorf("invalid command: %q", args[0])
	}

	prepare(ctx)
	stack := makeFullNode(ctx)
	defer stack.Close()

	startNode(ctx, stack, false)
	stack.Wait()
	return nil
}

// startNode boots up the system node and all registered protocols, after which
// it starts the RPC/IPC interfaces and the miner.
func startNode(ctx *cli.Context, stack *node.Node, isConsole bool) {
	// Start up the node itself
	utils.StartNode(ctx, stack, isConsole)

	if ctx.IsSet(utils.UnlockedAccountFlag.Name) {
		log.Warn(`The "unlock" flag has been deprecated and has no effect`)
	}

	// Register wallet event handlers to open and auto-derive wallets
	events := make(chan accounts.WalletEvent, 16)
	stack.AccountManager().Subscribe(events)

	// Create a client to interact with local gzen node.
	rpcClient := stack.Attach()
	ethClient := ethclient.NewClient(rpcClient)

	go func() {
		// Open any wallets already attached
		for _, wallet := range stack.AccountManager().Wallets() {
			if err := wallet.Open(""); err != nil {
				log.Warn("Failed to open wallet", "url", wallet.URL(), "err", err)
			}
		}
		// Listen for wallet event till termination
		for event := range events {
			switch event.Kind {
			case accounts.WalletArrived:
				if err := event.Wallet.Open(""); err != nil {
					log.Warn("New wallet appeared, failed to open", "url", event.Wallet.URL(), "err", err)
				}
			case accounts.WalletOpened:
				status, _ := event.Wallet.Status()
				log.Info("New wallet appeared", "url", event.Wallet.URL(), "status", status)

				var derivationPaths []accounts.DerivationPath
				if event.Wallet.URL().Scheme == "ledger" {
					derivationPaths = append(derivationPaths, accounts.LegacyLedgerBaseDerivationPath)
				}
				derivationPaths = append(derivationPaths, accounts.DefaultBaseDerivationPath)

				event.Wallet.SelfDerive(derivationPaths, ethClient)

			case accounts.WalletDropped:
				log.Info("Old wallet dropped", "url", event.Wallet.URL())
				event.Wallet.Close()
			}
		}
	}()

	// Spawn a standalone goroutine for status synchronization monitoring,
	// close the node when synchronization is complete if user required.
	if ctx.Bool(utils.ExitWhenSyncedFlag.Name) {
		go func() {
			sub := stack.EventMux().Subscribe(downloader.DoneEvent{})
			defer sub.Unsubscribe()
			for {
				event := <-sub.Chan()
				if event == nil {
					continue
				}
				done, ok := event.Data.(downloader.DoneEvent)
				if !ok {
					continue
				}
				if timestamp := time.Unix(int64(done.Latest.Time), 0); time.Since(timestamp) < 10*time.Minute {
					log.Info("Synchronisation completed", "latestnum", done.Latest.Number, "latesthash", done.Latest.Hash(),
						"age", common.PrettyAge(timestamp))
					stack.Close()
				}
			}
		}()
	}
}

// runEireneMainnet은 Eirene 메인넷을 실행합니다.
func runEireneMainnet(ctx *cli.Context) error {
	// 기본 설정 로드
	cfg := loadBaseConfig(ctx)

	// Eirene 메인넷 설정 적용
	cfg.Node.P2P.DiscoveryV5 = true
	cfg.Node.P2P.BootstrapNodes = nil // 실제 메인넷 부트스트랩 노드로 업데이트 필요
	cfg.Node.P2P.StaticNodes = nil    // 실제 메인넷 스태틱 노드로 업데이트 필요
	cfg.Node.HTTPModules = append(cfg.Node.HTTPModules, "eirene")
	cfg.Node.WSModules = append(cfg.Node.WSModules, "eirene")

	// Eirene 설정 적용
	if ctx.IsSet("eirene.period") {
		cfg.Eirene.Period = ctx.Uint64("eirene.period")
	}
	if ctx.IsSet("eirene.epoch") {
		cfg.Eirene.Epoch = ctx.Uint64("eirene.epoch")
	}
	if ctx.IsSet("eirene.slashing-threshold") {
		cfg.Eirene.SlashingThreshold = ctx.Uint64("eirene.slashing-threshold")
	}
	if ctx.IsSet("eirene.slashing-rate") {
		cfg.Eirene.SlashingRate = ctx.Uint64("eirene.slashing-rate")
	}
	if ctx.IsSet("eirene.missed-block-penalty") {
		cfg.Eirene.MissedBlockPenalty = ctx.Uint64("eirene.missed-block-penalty")
	}

	// 노드 생성
	stack, err := node.New(&cfg.Node)
	if err != nil {
		utils.Fatalf("Failed to create the protocol stack: %v", err)
	}

	// Eirene 합의 엔진 등록
	utils.RegisterEireneService(stack, &cfg.Eth, &cfg.Eirene, true)

	// 노드 시작
	startNode(ctx, stack, false)
	stack.Wait()
	return nil
}

// runEireneTestnet은 Eirene 테스트넷을 실행합니다.
func runEireneTestnet(ctx *cli.Context) error {
	// 기본 설정 로드
	cfg := loadBaseConfig(ctx)

	// Eirene 테스트넷 설정 적용
	cfg.Node.P2P.DiscoveryV5 = true
	cfg.Node.P2P.BootstrapNodes = nil // 실제 테스트넷 부트스트랩 노드로 업데이트 필요
	cfg.Node.P2P.StaticNodes = nil    // 실제 테스트넷 스태틱 노드로 업데이트 필요
	cfg.Node.HTTPModules = append(cfg.Node.HTTPModules, "eirene")
	cfg.Node.WSModules = append(cfg.Node.WSModules, "eirene")

	// 테스트넷 설정
	cfg.Eirene.Period = 6      // 테스트넷은 6초마다 블록 생성
	cfg.Eirene.Epoch = 10000   // 약 16시간마다 에포크 변경

	// Eirene 설정 적용
	if ctx.IsSet("eirene.period") {
		cfg.Eirene.Period = ctx.Uint64("eirene.period")
	}
	if ctx.IsSet("eirene.epoch") {
		cfg.Eirene.Epoch = ctx.Uint64("eirene.epoch")
	}
	if ctx.IsSet("eirene.slashing-threshold") {
		cfg.Eirene.SlashingThreshold = ctx.Uint64("eirene.slashing-threshold")
	}
	if ctx.IsSet("eirene.slashing-rate") {
		cfg.Eirene.SlashingRate = ctx.Uint64("eirene.slashing-rate")
	}
	if ctx.IsSet("eirene.missed-block-penalty") {
		cfg.Eirene.MissedBlockPenalty = ctx.Uint64("eirene.missed-block-penalty")
	}

	// 노드 생성
	stack, err := node.New(&cfg.Node)
	if err != nil {
		utils.Fatalf("Failed to create the protocol stack: %v", err)
	}

	// Eirene 합의 엔진 등록
	utils.RegisterEireneService(stack, &cfg.Eth, &cfg.Eirene, false)

	// 노드 시작
	startNode(ctx, stack, false)
	stack.Wait()
	return nil
}

// runEireneLocalTestnet은 Eirene 로컬 테스트넷을 실행합니다.
func runEireneLocalTestnet(ctx *cli.Context) error {
	// 기본 설정 로드
	cfg := loadBaseConfig(ctx)

	// 로컬 테스트넷 설정 적용
	cfg.Node.P2P.NoDiscovery = true
	cfg.Node.P2P.BootstrapNodes = nil
	cfg.Node.P2P.StaticNodes = nil
	cfg.Node.HTTPModules = append(cfg.Node.HTTPModules, "eirene")
	cfg.Node.WSModules = append(cfg.Node.WSModules, "eirene")

	// 로컬 테스트넷 설정
	cfg.Eirene.Period = 2      // 로컬 테스트넷은 2초마다 블록 생성
	cfg.Eirene.Epoch = 1000    // 약 33분마다 에포크 변경

	// Eirene 설정 적용
	if ctx.IsSet("eirene.period") {
		cfg.Eirene.Period = ctx.Uint64("eirene.period")
	}
	if ctx.IsSet("eirene.epoch") {
		cfg.Eirene.Epoch = ctx.Uint64("eirene.epoch")
	}
	if ctx.IsSet("eirene.slashing-threshold") {
		cfg.Eirene.SlashingThreshold = ctx.Uint64("eirene.slashing-threshold")
	}
	if ctx.IsSet("eirene.slashing-rate") {
		cfg.Eirene.SlashingRate = ctx.Uint64("eirene.slashing-rate")
	}
	if ctx.IsSet("eirene.missed-block-penalty") {
		cfg.Eirene.MissedBlockPenalty = ctx.Uint64("eirene.missed-block-penalty")
	}

	// 노드 생성
	stack, err := node.New(&cfg.Node)
	if err != nil {
		utils.Fatalf("Failed to create the protocol stack: %v", err)
	}

	// Eirene 합의 엔진 등록
	utils.RegisterEireneService(stack, &cfg.Eth, &cfg.Eirene, false)

	// 노드 시작
	startNode(ctx, stack, false)
	stack.Wait()
	return nil
}
