package cli

import (
	"strings"

	"github.com/mitchellh/cli"
)

type Account struct {
	UI cli.Ui
}

// MarkDown implements cli.MarkDown interface
func (a *Account) MarkDown() string {
	items := []string{
		"# Account",
		"The ```account``` command groups actions to interact with accounts:",
		"- [```account new```](./account_new.md): Create a new account in the Zena client.",
		"- [```account list```](./account_list.md): List the wallets in the Zena client.",
		"- [```account import```](./account_import.md): Import an account to the Zena client.",
	}

	return strings.Join(items, "\n\n")
}

// Help implements the cli.Command interface
func (a *Account) Help() string {
	return `Usage: zena account <subcommand>

  This command groups actions to interact with accounts.
  
  List the running deployments:

    $ zena account new
  
  Display the status of a specific deployment:

    $ zena account import
    
  List the imported accounts in the keystore:
    
    $ zena account list`
}

// Synopsis implements the cli.Command interface
func (a *Account) Synopsis() string {
	return "Interact with accounts"
}

// Run implements the cli.Command interface
func (a *Account) Run(args []string) int {
	return cli.RunResultHelp
}
