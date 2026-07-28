package cli

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/spf13/cobra"

	"github.com/dsh2dsh/goimapnotify/internal/config"
	"github.com/dsh2dsh/goimapnotify/internal/imap"
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
		defer client.Close()

		mailboxCount, err := printDelimiter(client)
		if err != nil {
			return fmt.Errorf(
				"listing mailboxes finished with error, account=%s: %w",
				account.Alias, err,
			)
		}

		slog.Info("walking through the account mailboxes",
			slog.String("account", account.Alias),
			slog.Int("count", mailboxCount))

		err = printMailbox(client, mailboxCount)
		if err != nil {
			return fmt.Errorf(
				"something went wrong while walking on the account listing all mailboxes, account=%s: %w",
				account.Alias, err,
			)
		}
	}
	return nil
}

// printDelimiter prints the hierarchy delimiter and returns mailbox count
func printDelimiter(c *imapclient.Client) (int, error) {
	var count int
	var delimiter string

	for m, err := range imap.Mailboxes(c) {
		if err != nil {
			return 0, err
		}
		if count == 0 {
			delimiter = string(m.Delim)
		}
		count++
	}

	fmt.Println("Hierarchy delimiter is:", delimiter)
	return count, nil
}

// printMailbox recursively lists mailboxes with tree visualization
func printMailbox(c *imapclient.Client, mailboxCount int) error {
	var pos int
	for m, err := range imap.Mailboxes(c) {
		if err != nil {
			return err
		}
		box := boxchar(pos, 0, mailboxCount-1)
		fmt.Println(box, m.Mailbox)
		pos++
	}
	return nil
}

func boxchar(p, l, b int) string {
	var drawthis string
	switch {
	case p == b || p == 0 && l > 0:
		drawthis = "└─"
	case p == 0 && p < b:
		drawthis = "┌─"
	case p > 0 && p < b:
		drawthis = "├─"
	case l > 0:
		drawthis = "│" + strings.Repeat(" ", l) + drawthis
	default:
		drawthis = "├─"
	}
	return drawthis
}
