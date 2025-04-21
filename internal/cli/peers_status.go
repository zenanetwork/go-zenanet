package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/zenanetwork/go-zenanet/internal/cli/flagset"
	"github.com/zenanetwork/go-zenanet/internal/cli/server/proto"
)

// PeersStatusCommand is the command to group the peers commands
type PeersStatusCommand struct {
	*Meta2
}

// MarkDown implements cli.MarkDown interface
func (p *PeersStatusCommand) MarkDown() string {
	items := []string{
		"# Peers status",
		"The ```peers status <peer id>``` command displays the status of a peer by its id.",
		p.Flags().MarkDown(),
	}

	return strings.Join(items, "\n\n")
}

// Help implements the cli.Command interface
func (p *PeersStatusCommand) Help() string {
	return `Usage: zena peers status <peer id>

  Display the status of a peer by its id.

  ` + p.Flags().Help()
}

func (p *PeersStatusCommand) Flags() *flagset.Flagset {
	flags := p.NewFlagSet("peers status")

	return flags
}

// Synopsis implements the cli.Command interface
func (p *PeersStatusCommand) Synopsis() string {
	return "Display the status of a peer"
}

// Run implements the cli.Command interface
func (p *PeersStatusCommand) Run(args []string) int {
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

	req := &proto.PeersStatusRequest{
		Enode: args[0],
	}
	resp, err := zenaClt.PeersStatus(context.Background(), req)

	if err != nil {
		p.UI.Error(err.Error())
		return 1
	}

	p.UI.Output(formatPeer(resp.Peer))

	return 0
}

func formatPeer(peer *proto.Peer) string {
	base := formatKV([]string{
		fmt.Sprintf("Name|%s", peer.Name),
		fmt.Sprintf("ID|%s", peer.Id),
		fmt.Sprintf("ENR|%s", peer.Enr),
		fmt.Sprintf("Capabilities|%s", strings.Join(peer.Caps, ",")),
		fmt.Sprintf("Enode|%s", peer.Enode),
		fmt.Sprintf("Static|%v", peer.Static),
		fmt.Sprintf("Trusted|%v", peer.Trusted),
	})

	return base
}
