package cli

import (
	"context"
	"strings"

	"github.com/zenanetwork/go-zenanet/internal/cli/flagset"
	"github.com/zenanetwork/go-zenanet/internal/cli/server/proto"
)

// PeersAddCommand is the command to group the peers commands
type PeersAddCommand struct {
	*Meta2

	trusted bool
}

// MarkDown implements cli.MarkDown interface
func (p *PeersAddCommand) MarkDown() string {
	items := []string{
		"# Peers add",
		"The ```peers add <enode>``` command joins the local client to another remote peer.",
		p.Flags().MarkDown(),
	}

	return strings.Join(items, "\n\n")
}

// Help implements the cli.Command interface
func (p *PeersAddCommand) Help() string {
	return `Usage: zena peers add <enode>

  Joins the local client to another remote peer.

  ` + p.Flags().Help()
}

func (p *PeersAddCommand) Flags() *flagset.Flagset {
	flags := p.NewFlagSet("peers add")

	flags.BoolFlag(&flagset.BoolFlag{
		Name:  "trusted",
		Usage: "Add the peer as a trusted",
		Value: &p.trusted,
	})

	return flags
}

// Synopsis implements the cli.Command interface
func (p *PeersAddCommand) Synopsis() string {
	return "Join the client to a remote peer"
}

// Run implements the cli.Command interface
func (p *PeersAddCommand) Run(args []string) int {
	flags := p.Flags()
	if err := flags.Parse(args); err != nil {
		p.UI.Error(err.Error())
		return 1
	}

	args = flags.Args()
	if len(args) != 1 {
		p.UI.Error("No enode address provided")
		return 1
	}

	zenaClt, err := p.ZenaConn()
	if err != nil {
		p.UI.Error(err.Error())
		return 1
	}

	req := &proto.PeersAddRequest{
		Enode:   args[0],
		Trusted: p.trusted,
	}
	if _, err := zenaClt.PeersAdd(context.Background(), req); err != nil {
		p.UI.Error(err.Error())
		return 1
	}

	return 0
}
