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
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/spf13/cobra"

	"github.com/dsh2dsh/goimapnotify/internal/cli/logger"
	"github.com/dsh2dsh/goimapnotify/internal/config"
	"github.com/dsh2dsh/goimapnotify/internal/imap"
	"github.com/dsh2dsh/goimapnotify/internal/runner"
)

const maxBackoff = 5 * time.Minute

var (
	commit, gittag, branch string

	flagConfig   string
	flagLogLevel string
	flagWait     int
	flagRetries  int
	flagSyslog   bool
	flagList     bool

	debug     bool
	topConfig *config.Configuration
)

var Cmd = cobra.Command{
	Use:     "goimapnotify",
	Short:   "goimapnotify executes scripts on IMAP mailbox changes (new/deleted/updated messages) using IDLE.",
	Args:    cobra.ExactArgs(0),
	Version: gittag,

	PersistentPreRunE: persistentPreRunE,

	RunE: func(cmd *cobra.Command, args []string) error { return Run() },
}

func init() {
	// Set the version for the imap package
	imap.Version = gittag

	Cmd.PersistentFlags().StringVarP(&flagLogLevel, "log-level", "l", "info",
		"change the logging level (error|warn|info|debug)")

	Cmd.PersistentFlags().IntVarP(&flagWait, "wait", "w", 1,
		"delay in seconds between the IDLE event and the execution of the scripts")

	Cmd.PersistentFlags().IntVarP(&flagRetries, "dial-retry-attempts", "r", 5,
		"number of attempts when connecting to an IMAP server, using exponential backoff")

	if logger.HasSyslog() {
		Cmd.PersistentFlags().BoolVarP(&flagSyslog, "syslog", "s", false,
			"send log output to syslog instead of stderr")
	}

	Cmd.Flags().BoolVar(&flagList, "list", false, "List all mailboxes and exit")
	Cmd.AddCommand(&listCmd)
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
		debug = true
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

	if err := logger.InitializeDefaultLogger(logLevel, flagSyslog); err != nil {
		return err
	}

	cfg, err := loadConfiguration(flagConfig, flagRetries)
	if err != nil {
		return fmt.Errorf("can't load the configuration %q: %w", flagConfig, err)
	}
	topConfig = cfg
	slog.Debug("configuration loaded successfully",
		slog.String("file", flagConfig))
	return nil
}

func Run() error {
	slog.Info("Running",
		slog.String("commit", commit),
		slog.String("tag", gittag),
		slog.String("branch", branch))

	idleChan := make(chan *config.IDLEEvent)
	queueChan := make(chan *config.IDLEEvent, 100)
	boxChan := make(chan *imap.BoxEvent, 1)
	quit := make(chan os.Signal, 1)
	quitChan := make(chan struct{})

	running := runner.NewRunningBox(debug, flagWait)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	if flagList {
		return listMailboxes(topConfig)
	}

	// Watch mailboxes events
	// This kick-starts the watching
	//
	// I really doubt it that creating a new client for each mailbox that we want
	// to listen for events is healthy, or elegant... but, if the connection
	// fails, what the program does right now is exactly that: it creates a new
	// client for that failing mailbox only, lol!

	var wg sync.WaitGroup
	var watching int

accountLoop:
	for _, account := range topConfig.Configurations {
		var connected bool
		for _, mailbox := range account.Boxes {
			key := account.Alias + mailbox.Mailbox
			running.Config[key] = account

			wb := imap.NewWatchBox(mailbox, idleChan, boxChan, quitChan)
			client, err := imap.New(account, flagRetries, imap.WithWatcher(wb))
			if errors.Is(err, imap.ErrLoginFailed) {
				slog.Error("Initial connection failed, skip all account mailboxes",
					slog.String("account", account.Alias), slog.Any("error", err))
				continue accountLoop
			} else if err != nil {
				slog.Error("Initial connection failed, retrying in background",
					slog.String("account", account.Alias), slog.Any("error", err))
				mailbox.Alias = account.Alias
				wg.Go(func() {
					reconnectWatcher(&imap.BoxEvent{UniqID: key, Mailbox: mailbox},
						account, idleChan, boxChan, quitChan, flagRetries)
				})
				watching++
				continue
			}

			if !connected {
				printConnected(slog.With(slog.String("account", account.Alias)),
					client)
				connected = true
			}

			wg.Go(func() {
				wb.Watch(client)
				_ = client.Close()
			})
			watching++
		}
	}

	if watching == 0 {
		slog.Error("nothing left to watch, exiting")
		return errors.New("nothing left to watch")
	}

idleLoop:
	for {
		select {
		case boxEvent := <-boxChan:
			if boxEvent.Skipped {
				watching--
				if watching == 0 {
					close(quitChan)
					slog.Error("nothing left to watch, exiting")
				}
				break idleLoop
			}

			slog.Info("Restarting watcher for mailbox",
				slog.String("alias", boxEvent.Mailbox.Alias),
				slog.String("mailbox", boxEvent.Mailbox.Mailbox))

			key := boxEvent.Mailbox.Alias + boxEvent.Mailbox.Mailbox
			wg.Go(func() {
				reconnectWatcher(boxEvent, running.Config[key], idleChan, boxChan,
					quitChan, flagRetries)
			})

		case <-quit:
			// OS asked nicely to close, we ask our goroutines to do the same
			close(quitChan)
			break idleLoop

		case idleEvent := <-idleChan:
			wg.Go(func() { running.Schedule(idleEvent, quitChan, queueChan) })

		case event := <-queueChan:
			if err := running.Run(event); err != nil {
				slog.Error("an error was encountered while executing commands for",
					slog.String("reason", event.Reason.String()),
					slog.String("alias", event.Alias),
					slog.String("box", event.Box.Mailbox),
					slog.Any("error", err))
			}
		}
	}

	slog.Info("waiting other goroutines to stop...")
	wg.Wait()
	slog.Info("bye")

	if watching == 0 {
		return errors.New("nothing left to watch")
	}
	return nil
}

func loadConfiguration(filename string, retries int,
) (*config.Configuration, error) {
	cfg, err := config.LoadYAML(filename)
	if err != nil {
		return nil, err
	}

	for _, conf := range cfg.Configurations {
		conf.RetrieveCmd()
		if conf.Alias == "" {
			conf.Alias = conf.Username
		}
		if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
			conf.Alias = "<?>"
		}

		if len(conf.Boxes) != 0 {
			// replace all listed mailboxes with the same mailboxes carrying values
			// from the configuration
			for _, mailbox := range conf.Boxes {
				if err := conf.FillBox(mailbox); err != nil {
					return nil, fmt.Errorf("template is invalid: %w", err)
				}
			}
			continue
		}

		// If there is no mailboxes, watch over all mailboxes of the account
		c, err := imap.New(conf, retries)
		if err != nil {
			return nil, fmt.Errorf(
				"account %q, failed to create IMAP client, error: %w",
				conf.Username, err,
			)
		}
		defer c.Close()

	mailboxLoop:
		for mailbox, err := range imap.Mailboxes(c) {
			if err != nil {
				return nil, fmt.Errorf(
					"failed to list all mailboxes, account=%s: %w", conf.Username, err)
			}
			// Ignore mailboxes with attributes `\All` and `\Noselect`
			for _, attr := range mailbox.Attrs {
				if attr == "\\All" || attr == "\\Noselect" {
					continue mailboxLoop
				}
			}

			box := &config.Box{Mailbox: mailbox.Mailbox}
			if err := conf.FillBox(box); err != nil {
				return nil, fmt.Errorf("template is invalid: %w", err)
			}
			conf.Boxes = append(conf.Boxes, box)
		}
	}
	return cfg, nil
}

func reconnectWatcher(
	event *imap.BoxEvent,
	cfg *config.NotifyConfig,
	idleChan chan<- *config.IDLEEvent,
	boxChan chan<- *imap.BoxEvent,
	quitChan <-chan struct{},
	retries int,
) {
	l := slog.With(
		slog.String("alias", event.Mailbox.Alias),
		slog.String("mailbox", event.Mailbox.Mailbox))
	backoff := time.Second

	for {
		select {
		case <-time.After(backoff):
		case <-quitChan:
			l.Info("Reconnection cancelled, shutting down")
			return
		}

		wb := imap.NewWatchBox(event.Mailbox, idleChan, boxChan, quitChan)
		client, err := imap.New(cfg, retries, imap.WithWatcher(wb))
		if errors.Is(err, imap.ErrLoginFailed) {
			l.Error("Reconnection failed", slog.Any("error", err))
			boxChan <- &imap.BoxEvent{
				UniqID:  event.UniqID,
				Mailbox: event.Mailbox,
				Skipped: true,
			}
			return
		} else if err != nil {
			backoff = min(backoff*2, maxBackoff)
			l.Error("Reconnection failed",
				slog.Duration("retrying", backoff),
				slog.Any("error", err))
			continue
		}

		l.Info("Reconnected successfully")
		wb.Watch(client)
		_ = client.Close()
		return
	}
}

func printConnected(l *slog.Logger, c *imapclient.Client) {
	if caps := imap.AllCaps(c); len(caps) != 0 {
		l = l.With(slog.Any("capabilities", caps))
	}
	l.Info("connected")
}
