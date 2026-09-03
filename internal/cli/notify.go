package cli

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/cobra"

	"github.com/dsh2dsh/goimapnotify/internal/config"
	"github.com/dsh2dsh/goimapnotify/internal/model"
	"github.com/dsh2dsh/goimapnotify/internal/runner"
)

var notifyCmd = cobra.Command{
	Use:   "test-notify",
	Short: "Show test desktop notification",
	Args:  cobra.ExactArgs(0),

	RunE: func(cmd *cobra.Command, args []string) error {
		return showTestNotification()
	},
}

func showTestNotification() error {
	ctx := context.Background()
	slog.SetDefault(slog.New(slog.DiscardHandler))

	running := runner.New(1, time.Duration(flagWait)).
		WithMaxDelay(topConfig.MaxDelay)
	defer running.Close()

	err := running.EnableDesktopNotifications(ctx, topConfig.DesktopNotify, 1)
	if err != nil {
		return fmt.Errorf("trying to enable desktop notifications: %w", err)
	}

	b := model.Box{
		Box: &config.Box{Mailbox: "Inbox"},
	}

	threads := []model.Thread{
		{
			From: map[string]string{
				"john@localhost": "John Doe",
				"jane@localhost": "Jane Doe",
			},
			Subject: "Lorem ipsum dolor sit amet consectetur adipiscing elit.",
			Count:   2,
		},
	}

	err = running.NotifyNewMails(ctx, &b, threads)
	if err != nil {
		return err
	}
	return nil
}
