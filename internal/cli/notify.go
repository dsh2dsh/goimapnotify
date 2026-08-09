package cli

import (
	"context"
	"fmt"

	"github.com/esiqveland/notify"
	"github.com/spf13/cobra"

	"github.com/dsh2dsh/goimapnotify/internal/config"
	"github.com/dsh2dsh/goimapnotify/internal/runner"
)

var notifyCmd = cobra.Command{
	Use:   "test-notify",
	Short: "Show test desktop notification",
	Args:  cobra.ExactArgs(0),

	RunE: func(cmd *cobra.Command, args []string) error {
		return showTestNotification(topConfig.DesktopNotify)
	},
}

func showTestNotification(cfg config.DesktopNotification) error {
	conn, err := runner.DBusConnect(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()

	n := runner.DesktopNotificationFrom(cfg)
	n.Summary = "New Emails in INBOX"
	n.Body = "Lorem ipsum dolor sit amet consectetur adipiscing elit. Sit amet consectetur adipiscing elit quisque faucibus ex."

	_, err = notify.SendNotification(conn, n)
	if err != nil {
		return fmt.Errorf("send desktop notification: %w", err)
	}
	return nil
}
