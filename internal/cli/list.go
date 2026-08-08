package cli

import (
	"fmt"
	"log/slog"
	"strings"

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
		boxes, err := imap.List(account, flagRetries)
		if err != nil {
			return err
		}

		count := len(boxes.Boxes)
		slog.Info("walking through the account mailboxes",
			slog.String("account", account.Alias),
			slog.Int("count", count))
		fmt.Println("Hierarchy delimiter is:", string(boxes.Delim))

		var pos int
		for _, name := range boxes.Boxes {
			box := boxchar(pos, 0, count-1)
			fmt.Println(box, name)
			pos++
		}
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
