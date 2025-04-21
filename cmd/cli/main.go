package main

import (
	"os"

	"github.com/zenanetwork/go-zenanet/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
