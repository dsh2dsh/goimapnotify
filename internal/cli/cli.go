package cli

// Execute scripts on events using IDLE imap command (Go version)
// Copyright (C) 2017-2026  Jorge Javier Araya Navarro

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/dsh2dsh/goimapnotify/internal/box"
	"github.com/dsh2dsh/goimapnotify/internal/cli/logger"
	"github.com/dsh2dsh/goimapnotify/internal/config"
	"github.com/dsh2dsh/goimapnotify/internal/imap"
	"github.com/dsh2dsh/goimapnotify/internal/jmap"
	"github.com/dsh2dsh/goimapnotify/internal/logging"
	"github.com/dsh2dsh/goimapnotify/internal/runner"
)

var (
	commit, gittag, branch string

	flagConfig   string
	flagLogLevel string
	flagWait     int
	flagRetries  int
	flagSyslog   bool
	flagList     bool

	topConfig *config.Configuration
)

var Cmd = cobra.Command{
	Use:     "goimapnotify",
	Short:   "goimapnotify executes scripts on IMAP (using IDLE) or JMAP mailbox changes (new/deleted/updated messages).",
	Args:    cobra.ExactArgs(0),
	Version: gittag,

	PersistentPreRunE: persistentPreRunE,

	RunE: func(cmd *cobra.Command, args []string) error { return Run() },
}

func init() {
	// Set the version for the imap package
	imap.Version = gittag
	jmap.Version = gittag

	Cmd.PersistentFlags().StringVarP(&flagLogLevel, "log-level", "l", "info",
		"change the logging level (error|warn|info|debug)")

	Cmd.PersistentFlags().IntVarP(&flagWait, "wait", "w", 1,
		"delay in seconds between the IDLE event and the execution of the scripts")

	Cmd.PersistentFlags().IntVarP(&flagRetries, "dial-retry-attempts", "r", 5,
		"number of attempts when connecting to a server, using exponential backoff")

	if logger.HasSyslog() {
		Cmd.PersistentFlags().BoolVarP(&flagSyslog, "syslog", "s", false,
			"send log output to syslog instead of stderr")
	}

	Cmd.Flags().BoolVar(&flagList, "list", false, "List all mailboxes and exit")
	Cmd.AddCommand(&listCmd)
	Cmd.AddCommand(&notifyCmd)
}

func Execute() {
	configPath, err := defaultConfigPath()
	cobra.CheckErr(err)

	Cmd.PersistentFlags().StringVarP(&flagConfig, "conf", "c",
		filepath.Join(configPath, "goimapnotify.yaml"), "Configuration file")

	if err := Cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func defaultConfigPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config: unable get user config dir: %w", err)
	}
	return filepath.Join(dir, "goimapnotify"), nil
}

func persistentPreRunE(cmd *cobra.Command, args []string) error {
	// Don't show usage on app errors.
	// https://github.com/spf13/cobra/issues/340#issuecomment-378726225
	cmd.SilenceUsage = true

	var logLevel slog.Level
	switch strings.ToLower(flagLogLevel) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info", "information":
		logLevel = slog.LevelInfo
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		return fmt.Errorf("invalid --log-level: %s (want error|warn|info|debug)",
			flagLogLevel)
	}

	if flagWait < 1 {
		return fmt.Errorf("invalid --wait: %d (minimum value 1 second)", flagWait)
	}

	if err := logger.InitializeDefaultLogger(logLevel, flagSyslog); err != nil {
		return err
	}

	cfg, err := loadConfiguration(flagConfig)
	if err != nil {
		return fmt.Errorf("can't load the configuration %q: %w", flagConfig, err)
	}
	topConfig = cfg
	slog.Debug("configuration loaded successfully",
		slog.String("file", flagConfig))
	return nil
}

func loadConfiguration(filename string) (*config.Configuration, error) {
	cfg, err := config.LoadYAML(filename)
	if err != nil {
		return nil, err
	}

	for _, conf := range cfg.Configurations {
		if err := conf.RetrieveCmd(); err != nil {
			return nil, err
		}

		if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
			conf.Alias = "<?>"
		} else if conf.Alias == "" {
			conf.Alias = conf.Username
		}
	}
	return cfg, nil
}

func Run() error {
	if flagList {
		return listMailboxes(topConfig)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt,
		syscall.SIGTERM)
	defer stop()

	n, boxes, err := boxesFromConfig(ctx, topConfig.Configurations, flagRetries)
	if err != nil {
		return fmt.Errorf("parse configurations: %w", err)
	}

	slog.Info("Running",
		slog.String("commit", commit),
		slog.String("tag", gittag),
		slog.String("branch", branch),
		slog.Int("boxes", len(boxes)))

	running := runner.New(n, time.Duration(flagWait)).
		WithMaxDelay(topConfig.MaxDelay)
	defer running.Close()
	if topConfig.DesktopNotify.Enable {
		err := running.EnableDesktopNotifications(ctx, topConfig.DesktopNotify, n)
		if err != nil {
			return fmt.Errorf("trying to enable desktop notifications: %w", err)
		}
	}

	events := make(chan *box.IDLE)
	var wg sync.WaitGroup
	var watching int

accountLoop:
	for _, account := range boxes {
		if ctx.Err() != nil {
			break
		}

		if account.JMAP {
			wb := jmap.NewWatchMailboxes(account.Boxes, events).
				WithStartupSync(topConfig.StartupSync)

			l := logging.FromContext(ctx).With(slog.String("account", account.Alias))
			ctx = logging.WithLogger(ctx, l)

			if err := wb.Connect(ctx, flagRetries); err != nil {
				l.Error("Initial connection failed, skip all account mailboxes",
					slog.Any("error", err))
				continue
			}

			watching++
			wg.Go(func() { wb.Watch(ctx) })
			continue
		}

		once := new(sync.Once)
		for _, mailbox := range account.Boxes {
			if ctx.Err() != nil {
				break accountLoop
			}

			wb := imap.NewWatchBox(mailbox, events).
				WithStartupSync(topConfig.StartupSync)

			err := wb.Connect(ctx, flagRetries, once)
			if err != nil {
				slog.Error("Initial connection failed, skip all account mailboxes",
					slog.String("account", account.Alias), slog.Any("error", err))
				wb.Close()
				continue accountLoop
			}

			watching++
			wg.Go(wb.Watch)
		}
	}

	if watching == 0 {
		slog.Error("nothing left to watch, exiting")
		return errors.New("nothing left to watch")
	}

idleLoop:
	for {
		select {
		case <-ctx.Done():
			stop()
			break idleLoop

		case e := <-events:
			switch e.Reason() {
			case box.StopWatching:
				watching--
				if watching == 0 {
					slog.Error("nothing left to watch, exiting")
					stop()
					break idleLoop
				}
			default:
				if !e.Skip() {
					running.Schedule(ctx, e)
				}
			}
		}
	}

	slog.Info("waiting other goroutines to stop...")
	gracefulWait(&wg, running)
	slog.Info("bye")

	if watching == 0 {
		return errors.New("nothing left to watch")
	}
	return nil
}

type accountBoxes struct {
	*config.NotifyConfig

	Boxes []*box.Box
}

func boxesFromConfig(ctx context.Context, c []*config.NotifyConfig, retries int,
) (int, []accountBoxes, error) {
	var n int
	ab := make([]accountBoxes, len(c))
	for i, accountConfig := range c {
		configuredBoxes := accountConfig.Boxes
		if len(configuredBoxes) == 0 {
			// If there is no mailboxes, watch over all mailboxes of the account
			for mailbox, err := range remoteBoxes(ctx, accountConfig, retries) {
				if err != nil {
					return 0, nil, fmt.Errorf(
						"generate boxes configuration, account=%s: %w",
						accountConfig.Username, err)
				}
				configuredBoxes = append(configuredBoxes, &config.Box{Mailbox: mailbox})
			}
		}

		boxes, err := box.CompileBoxes(accountConfig, configuredBoxes)
		if err != nil {
			return 0, nil, err
		}
		ab[i] = accountBoxes{NotifyConfig: accountConfig, Boxes: boxes}
		n += len(boxes)
	}
	return n, ab, nil
}

func remoteBoxes(ctx context.Context, account *config.NotifyConfig, retries int,
) iter.Seq2[string, error] {
	if account.JMAP {
		return jmap.Mailboxes(ctx, account, retries)
	}
	return imap.Mailboxes(account, retries)
}

func gracefulWait(wg *sync.WaitGroup, running *runner.Runner) {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		running.Wait()
		close(done)
	}()

	select {
	case <-time.After(10 * time.Second):
		slog.Warn("timed out waiting for other goroutines, abort")
	case <-done:
	}
}
