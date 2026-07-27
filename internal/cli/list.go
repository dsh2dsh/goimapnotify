package cli

import (
	"fmt"
	"log/slog"
	"strings"

	goimap "github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
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
		defer client.Logout()

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

		err = walkMailbox(client, "", 0, mailboxCount)
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
func printDelimiter(c *client.Client) (int, error) {
	mailboxes := make(chan *goimap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.List("", "*", mailboxes)
	}()

	i := 0
	m := <-mailboxes
	for range mailboxes {
		i += 1
	}
	if err := <-done; err != nil {
		return 0, err
	}

	fmt.Println("Hierarchy delimiter is:", m.Delimiter)
	return i, nil
}

// walkMailbox recursively lists mailboxes with tree visualization
func walkMailbox(c *client.Client, b string, l, mailboxCount int) error {
	// FIXME: This can be done better
	mailboxes := make(chan *goimap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.List(b, "*", mailboxes)
	}()

	pos := 0
	for m := range mailboxes {
		box := boxchar(pos, l, mailboxCount)
		fmt.Println(box, m.Name)
		pos += 1
		// Check if mailbox has children mailboxes
		for _, attr := range m.Attributes {
			if attr == "\\Haschildren" {
				err := walkMailbox(c, m.Name, l+1, mailboxCount)
				if err != nil {
					slog.Error("cannot keep walking mailboxes", slog.Any("error", err))
					return err
				}
				break
			}
		}
	}

	if err := <-done; err != nil {
		return err
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
