package utils

import (
	"os"

	"github.com/urfave/cli/v2"

	"github.com/zenanetwork/go-zenanet/eth"
	"github.com/zenanetwork/go-zenanet/eth/ethconfig"
	"github.com/zenanetwork/go-zenanet/node"
)

var (
	//
	// Zena Specific flags
	//

	// IrisURLFlag flag for iris url
	IrisURLFlag = &cli.StringFlag{
		Name:  "zena.iris",
		Usage: "URL of Iris service",
		Value: "http://localhost:1317",
	}

	// WithoutIrisFlag no iris (for testing purpose)
	WithoutIrisFlag = &cli.BoolFlag{
		Name:  "zena.withoutiris",
		Usage: "Run without iris service (for testing purpose)",
	}

	// IrisgRPCAddressFlag flag for iris gRPC address
	IrisgRPCAddressFlag = &cli.StringFlag{
		Name:  "zena.irisgRPC",
		Usage: "Address of Iris gRPC service",
		Value: "",
	}

	// RunIrisFlag flag for running iris internally from zena
	RunIrisFlag = &cli.BoolFlag{
		Name:  "zena.runiris",
		Usage: "Run Iris service as a child process",
	}

	RunIrisArgsFlag = &cli.StringFlag{
		Name:  "zena.runirisargs",
		Usage: "Arguments to pass to Iris service",
		Value: "",
	}

	// UseIrisApp flag for using internal iris app to fetch data
	UseIrisAppFlag = &cli.BoolFlag{
		Name:  "zena.useirisapp",
		Usage: "Use child iris process to fetch data, Only works when zena.runiris is true",
	}

	// ZenaFlags all zena related flags
	ZenaFlags = []cli.Flag{
		IrisURLFlag,
		WithoutIrisFlag,
		IrisgRPCAddressFlag,
		RunIrisFlag,
		RunIrisArgsFlag,
		UseIrisAppFlag,
	}
)

// SetZenaConfig sets zena config
func SetZenaConfig(ctx *cli.Context, cfg *eth.Config) {
	cfg.IrisURL = ctx.String(IrisURLFlag.Name)
	cfg.WithoutIris = ctx.Bool(WithoutIrisFlag.Name)
	cfg.IrisgRPCAddress = ctx.String(IrisgRPCAddressFlag.Name)
	cfg.RunIris = ctx.Bool(RunIrisFlag.Name)
	cfg.RunIrisArgs = ctx.String(RunIrisArgsFlag.Name)
	cfg.UseIrisApp = ctx.Bool(UseIrisAppFlag.Name)
}

// CreateZenaZenanet Creates zena zenanet object from eth.Config
func CreateZenaZenanet(cfg *ethconfig.Config) *eth.Zenanet {
	workspace, err := os.MkdirTemp("", "zena-command-node-")
	if err != nil {
		Fatalf("Failed to create temporary keystore: %v", err)
	}

	// Create a networkless protocol stack and start an Zenanet service within
	stack, err := node.New(&node.Config{DataDir: workspace, UseLightweightKDF: true, Name: "zena-command-node"})
	if err != nil {
		Fatalf("Failed to create node: %v", err)
	}

	zenanet, err := eth.New(stack, cfg)
	if err != nil {
		Fatalf("Failed to register Zenanet protocol: %v", err)
	}

	// Start the node and assemble the JavaScript console around it
	if err = stack.Start(); err != nil {
		Fatalf("Failed to start stack: %v", err)
	}

	stack.Attach()

	return zenanet
}
