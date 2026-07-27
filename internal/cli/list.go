package cli

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/dsh2dsh/goimapnotify/internal/config"
	"github.com/dsh2dsh/goimapnotify/internal/imap"
	"github.com/dsh2dsh/goimapnotify/internal/util"
)

var listCmd = cobra.Command{
	Use:   "list",
	Short: "List all mailboxes and exit",
	Args:  cobra.ExactArgs(0),

	RunE: func(cmd *cobra.Command, args []string) error {
		return listMailboxes(topConfig)
	},
}

func listMailboxes(topConfig *config.Configuration) error {
	for _, account := range topConfig.Configurations {
		client, err := imap.New(account, flagRetries)
		if err != nil {
			return fmt.Errorf(
				"something went wrong creating IMAP client, account=%s: %w",
				account.Alias, err,
			)
		}
		defer client.Logout()

		mailboxCount, err := util.PrintDelimiter(client)
		if err != nil {
			return fmt.Errorf(
				"listing mailboxes finished with error, account=%s: %w",
				account.Alias, err,
			)
		}

		slog.Info("walking through the account mailboxes",
			slog.String("account", account.Alias))
		err = util.WalkMailbox(client, "", 0, mailboxCount)
		if err != nil {
			return fmt.Errorf(
				"something went wrong while walking on the account listing all mailboxes, account=%s: %w",
				account.Alias, err,
			)
		}
	}
	return nil
}
