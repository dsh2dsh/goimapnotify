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
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	goimap "github.com/emersion/go-imap"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/dsh2dsh/goimapnotify/internal/config"
	"github.com/dsh2dsh/goimapnotify/internal/imap"
	"github.com/dsh2dsh/goimapnotify/internal/runner"
	"github.com/dsh2dsh/goimapnotify/internal/util"
)

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

	Cmd.PersistentFlags().BoolVarP(&flagSyslog, "syslog", "s", false,
		"send log output to syslog instead of stderr")

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

	switch strings.ToLower(flagLogLevel) {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
		debug = true
	case "info", "information":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn", "warning":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	default:
		return fmt.Errorf("invalid --log-level: %s (want error|warn|info|debug)",
			flagLogLevel)
	}

	if flagSyslog {
		if err := enableSyslog(); err != nil {
			return err
		}
	}

	cfg, err := loadConfiguration(flagConfig, flagRetries)
	if err != nil {
		return fmt.Errorf("can't load the configuration %q: %w", flagConfig, err)
	}
	topConfig = cfg
	logrus.Debugf("configuration loaded successfully: %q", flagConfig)
	return nil
}

func Run() error {
	logrus.Infof("ℹ Running commit %s, tag %s, branch %s", commit, gittag, branch)

	idleChan := make(chan imap.IDLEEvent)
	queueChan := make(chan imap.IDLEEvent, 100)
	boxChan := make(chan imap.BoxEvent, 1)
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
	for _, account := range topConfig.Configurations {
		for _, mailbox := range account.Boxes {
			key := account.Alias + mailbox.Mailbox
			running.Config[key] = account

			client, err := imap.NewIMAPIDLEClient(account, flagRetries)
			if err != nil {
				logrus.WithError(err).
					WithField("account", account.Alias).
					Warn("Initial connection failed, retrying in background")
				mailbox.Alias = account.Alias
				wg.Add(1)
				go reconnectWatcher(
					imap.BoxEvent{UniqID: key, Mailbox: mailbox},
					account, idleChan, boxChan, quitChan, &wg, flagRetries,
				)
				continue
			}
			imap.NewWatchBox(client, account, mailbox, idleChan, boxChan, quitChan, &wg)
		}
	}

idleLoop:
	for {
		select {
		case boxEvent := <-boxChan:
			l := logrus.WithField("alias", boxEvent.Mailbox.Alias).
				WithField("mailbox", boxEvent.Mailbox.Mailbox)
			l.Info("Restarting watcher for mailbox")
			key := boxEvent.Mailbox.Alias + boxEvent.Mailbox.Mailbox
			wg.Add(1)
			go reconnectWatcher(
				boxEvent,
				running.Config[key],
				idleChan,
				boxChan,
				quitChan,
				&wg,
				flagRetries,
			)
		case <-quit:
			// OS asked nicely to close, we ask our
			// goroutines to do the same
			close(quitChan)
			break idleLoop
		case idleEvent := <-idleChan:
			wg.Go(func() { running.Schedule(idleEvent, quitChan, queueChan) })
		case event := <-queueChan:
			wg.Add(1)
			err := running.Run(event)
			wg.Done()
			if err != nil {
				logrus.WithError(err).
					WithFields(logrus.Fields{"alias": event.Alias, "box": event.Box.Mailbox}).
					Errorf("an error was encountered while executing commands for %q", event.Reason)
			}
		}
	}

	logrus.Info("waiting other goroutines to stop...")
	wg.Wait()
	logrus.Info("bye")
	return nil
}

func loadConfiguration(filename string, retries int,
) (*config.Configuration, error) {
	cfg, err := config.LoadYAML(filename)
	if err != nil {
		return nil, err
	}

	for i := range cfg.Configurations {
		cfg.Configurations[i] = util.RetrieveCmd(cfg.Configurations[i])
		if cfg.Configurations[i].Alias == "" {
			cfg.Configurations[i].Alias = cfg.Configurations[i].Username
		}
		if logrus.GetLevel() == logrus.DebugLevel {
			cfg.Configurations[i].Alias = "<?>"
		}

		conf := cfg.Configurations[i]

		// If there is no mailboxes, watch over all mailboxes of the account
		if len(conf.Boxes) == 0 {
			client, err := imap.NewIMAPIDLEClient(conf, retries)
			if err != nil {
				return nil, fmt.Errorf(
					"account %q, failed to create IMAP client, error: %w",
					conf.Username,
					err,
				)
			}
			defer client.Logout()

			// NOTE(shackra): Having to do this is really disgusting, v2 offers a better way for listing mailboxes. I should consider updating.
			ch := make(chan *goimap.MailboxInfo)
			go func() {
				err := client.List("", "*", ch)
				if err != nil {
					logrus.WithError(err).
						WithField("account", conf.Username).
						Fatal("failed to list all mailboxes")
				}
			}()

			for mailbox := range ch {
				// Ignore mailboxes with attributes `\All` and `\Noselect`
				if slices.Contains(mailbox.Attributes, "\\All") ||
					slices.Contains(mailbox.Attributes, "\\Noselect") {
					continue
				}

				box, err := config.SetFromConfig(conf, config.Box{
					Mailbox: mailbox.Name,
				})
				if err != nil {
					logrus.WithError(err).Fatal("template is invalid")
				}
				cfg.Configurations[i].Boxes = append(
					cfg.Configurations[i].Boxes,
					box,
				)
			}
		} else {
			// replace all listed mailboxes with the same mailboxes carrying values from the configuration
			for mailbox := range cfg.Configurations[i].Boxes {
				box, err := config.SetFromConfig(
					conf,
					cfg.Configurations[i].Boxes[mailbox],
				)
				if err != nil {
					logrus.WithError(err).Fatal("template is invalid")
				}
				cfg.Configurations[i].Boxes[mailbox] = box
			}
		}
	}
	return cfg, nil
}

func reconnectWatcher(
	event imap.BoxEvent,
	cfg config.NotifyConfig,
	idleChan chan<- imap.IDLEEvent,
	boxChan chan<- imap.BoxEvent,
	quitChan <-chan struct{},
	wg *sync.WaitGroup,
	retries int,
) {
	defer wg.Done()

	l := logrus.WithField("alias", event.Mailbox.Alias).
		WithField("mailbox", event.Mailbox.Mailbox)

	backoff := time.Second
	maxBackoff := 5 * time.Minute

	for {
		select {
		case <-quitChan:
			l.Info("Reconnection cancelled, shutting down")
			return
		default:
		}

		client, err := imap.NewIMAPIDLEClient(cfg, retries)
		if err != nil {
			if isAuthError(err) && backoff < 30*time.Second {
				backoff = 30 * time.Second
			}
			l.WithError(err).Warnf("Reconnection failed, retrying in %s", backoff)
			select {
			case <-time.After(backoff):
			case <-quitChan:
				l.Info("Reconnection cancelled, shutting down")
				return
			}
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		l.Info("Reconnected successfully")
		imap.NewWatchBox(client, cfg, event.Mailbox, idleChan, boxChan, quitChan, wg)
		return
	}
}

func isAuthError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such user") ||
		strings.Contains(msg, "too many login attempts") ||
		strings.Contains(msg, "authentication failed") ||
		strings.Contains(msg, "invalid credentials")
}
