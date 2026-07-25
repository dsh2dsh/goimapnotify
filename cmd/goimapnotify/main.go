package main

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
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	goimap "github.com/emersion/go-imap"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"gitlab.com/shackra/goimapnotify/internal/config"
	"gitlab.com/shackra/goimapnotify/internal/imap"
	netmon "gitlab.com/shackra/goimapnotify/internal/net"
	"gitlab.com/shackra/goimapnotify/internal/runner"
	"gitlab.com/shackra/goimapnotify/internal/util"
)

var (
	commit string
	gittag string
	branch string
)

func init() {
	// Set the version for the imap package
	imap.Version = gittag
}

func getDefaultConfigPath() string {
	home := os.Getenv("XDG_CONFIG_HOME")
	if home == "" {
		return filepath.Join(os.Getenv("HOME"), ".config", "goimapnotify")
	}

	return filepath.Join(home, "goimapnotify")
}

func usage() {
	_, _ = fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
	flag.PrintDefaults()
	msg := util.DonateMessage(8)
	_, _ = fmt.Fprint(flag.CommandLine.Output(), "\n"+msg)
}

func loadConfiguration(path string, retries int) (*config.Configuration, error) {
	var topConfiguration config.Configuration
	if err := viper.Unmarshal(&topConfiguration); err != nil {
		return nil, fmt.Errorf("can't parse the configuration: %q, error: %v", path, err)
	}

	if topConfiguration.Configurations == nil {
		var legacy config.ConfigurationLegacy
		if err := viper.UnmarshalExact(&legacy); err != nil {
			return nil, fmt.Errorf(
				"can't parse the configuration in 'legacy' format: %s, error: %v",
				path,
				err,
			)
		}

		logrus.Info("legacy format configuration detected")
		topConfiguration.Configurations = config.LegacyConverter(legacy)
	}

	if len(topConfiguration.Configurations) > 0 &&
		(topConfiguration.Configurations[0].Host == "" && topConfiguration.Configurations[0].HostCMD == "") {
		return nil, fmt.Errorf(
			"configuration file %q is empty or have invalid configuration format",
			path,
		)
	}

	for account := range topConfiguration.Configurations {
		topConfiguration.Configurations[account] = util.RetrieveCmd(
			topConfiguration.Configurations[account],
		)
		if topConfiguration.Configurations[account].Alias == "" {
			topConfiguration.Configurations[account].Alias = topConfiguration.Configurations[account].Username
		}
		if logrus.GetLevel() == logrus.DebugLevel {
			topConfiguration.Configurations[account].Alias = "<?>"
		}

		conf := topConfiguration.Configurations[account]

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
			// nolint
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
				topConfiguration.Configurations[account].Boxes = append(
					topConfiguration.Configurations[account].Boxes,
					box,
				)
			}
		} else {
			// replace all listed mailboxes with the same mailboxes carrying values from the configuration
			for mailbox := range topConfiguration.Configurations[account].Boxes {
				box, err := config.SetFromConfig(
					conf,
					topConfiguration.Configurations[account].Boxes[mailbox],
				)
				if err != nil {
					logrus.WithError(err).Fatal("template is invalid")
				}
				topConfiguration.Configurations[account].Boxes[mailbox] = box
			}
		}
	}

	return &topConfiguration, nil
}

func main() {
	// imap.DefaultLogMask = imap.LogConn | imap.LogRaw
	fileconf := flag.String(
		"conf",
		filepath.Join(
			getDefaultConfigPath(),
			fmt.Sprintf("goimapnotify.%s", viper.SupportedExts[2]),
		),
		"Configuration file, supported formats: json, yaml/yml, toml",
	)
	list := flag.Bool("list", false, "List all mailboxes and exit")
	loglevel := flag.String(
		"log-level",
		"info",
		"Change the logging level; possible values are: error, warn, info, debug",
	)
	wait := flag.Int(
		"wait",
		1,
		"Delay in seconds between the IDLE event and the execution of the scripts",
	)
	dialRetries := flag.Int(
		"dial-retry-attempts",
		5,
		"Number of attempts when connecting to an IMAP server, using exponential backoff",
	)
	useSyslog := flag.Bool(
		"syslog",
		false,
		"Send log output to syslog instead of stderr (not available on Windows)",
	)

	flag.Usage = usage

	flag.Parse()

	debug := false

	switch strings.ToLower(*loglevel) {
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
		logrus.Fatalf("unknown logging level %q", *loglevel)
	}

	if *useSyslog {
		if err := util.EnableSyslog(); err != nil {
			logrus.WithError(err).Fatal("failed to enable syslog")
		}
	}

	logrus.Infof("ℹ Running commit %s, tag %s, branch %s", commit, gittag, branch)

	viper.SetConfigFile(*fileconf)
	if err := viper.ReadInConfig(); err != nil {
		logrus.WithError(err).Fatalf("can't read file: %q", *fileconf)
	}

	idleChan := make(chan imap.IDLEEvent)
	queueChan := make(chan imap.IDLEEvent, 100)
	boxChan := make(chan imap.BoxEvent, 1)
	quit := make(chan os.Signal, 1)
	quitChan := make(chan struct{})

	topConfig, err := loadConfiguration(*fileconf, *dialRetries)
	if err != nil {
		logrus.WithError(err).Fatalf("can't load the configuration %q", *fileconf)
	}
	logrus.Debugf("configuration loaded successfully: %q", *fileconf)

	running := runner.NewRunningBox(debug, *wait)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	wg := &sync.WaitGroup{}

	if *list {
		for _, account := range topConfig.Configurations {
			client, err := imap.NewClient(account, *dialRetries)
			if err != nil {
				logrus.WithError(err).
					WithField("account", account.Alias).
					Fatal("something went wrong creating IMAP client")
			}
			// nolint
			defer client.Logout()

			max, err := util.PrintDelimiter(client)
			if err != nil {
				logrus.WithField("alias", account.Alias).
					WithError(err).
					Warning("listing mailboxes finished with error")
			}
			logrus.WithField("account", account.Alias).Info("walking through the account mailboxes")
			err = util.WalkMailbox(client, "", 0, max)
			if err != nil {
				logrus.WithField("account", account.Alias).
					WithError(err).
					Fatal("something went wrong while walking on the account listing all mailboxes")
			}
		}
	}

	// Watch mailboxes events
	// This kick-starts the watching
	idleForever := !*list
	if idleForever {
		/* I really doubt it that creating a new client for
		   each mailbox that we want to listen for events is
		   healthy, or elegant... but, if the connection
		   fails, what the program does right now is exactly
		   that: it creates a new client for that failing
		   mailbox only, lol!
		*/
		for _, account := range topConfig.Configurations {
			for _, mailbox := range account.Boxes {
				key := account.Alias + mailbox.Mailbox
				running.Config[key] = account

				client, err := imap.NewIMAPIDLEClient(account, *dialRetries)
				if err != nil {
					logrus.WithError(err).
						WithField("account", account.Alias).
						Warn("Initial connection failed, retrying in background")
					mailbox.Alias = account.Alias
					wg.Add(1)
					go reconnectWatcher(
						imap.BoxEvent{UniqID: key, Mailbox: mailbox},
						account, idleChan, boxChan, quitChan, wg, *dialRetries,
					)
					continue
				}
				imap.NewWatchBox(client, account, mailbox, idleChan, boxChan, quitChan, wg)
			}
		}
	}

	// Start network monitor if configured
	var netChan chan netmon.NetworkEvent
	checkInterval := topConfig.NetworkCheckInterval
	if checkInterval == 0 {
		checkInterval = 30 // default 30 seconds
	}
	if checkInterval > 0 {
		netChan = make(chan netmon.NetworkEvent, 1)
		var connectivityAddrs []string
		if len(topConfig.ConnectivityHosts) > 0 {
			connectivityAddrs = topConfig.ConnectivityHosts
		} else {
			connectivityAddrs = []string{"archlinux.org:80", "ubuntu.com:80"}
		}
		hosts := make([]netmon.HostPort, 0, len(connectivityAddrs))
		for _, addr := range connectivityAddrs {
			h, portStr, err := net.SplitHostPort(addr)
			if err != nil {
				logrus.WithError(err).Warnf("Invalid connectivityHosts entry %q, skipping", addr)
				continue
			}
			port, err := strconv.Atoi(portStr)
			if err != nil {
				logrus.WithError(err).
					Warnf("Invalid port in connectivityHosts entry %q, skipping", addr)
				continue
			}
			hosts = append(hosts, netmon.HostPort{Host: h, Port: port})
		}
		monitor := netmon.NewNetworkMonitor(
			hosts,
			time.Duration(checkInterval)*time.Second,
			5*time.Second,
			netChan,
			quitChan,
		)
		monitor.Start(wg)
		logrus.Infof("Network monitor started, checking every %d seconds", checkInterval)
	}

	var networkDown bool
	var pendingReconnects []imap.BoxEvent

	for idleForever {
		select {
		case netEvent := <-netChan:
			if netEvent.State == netmon.NetworkDown {
				networkDown = true
			} else if netEvent.State == netmon.NetworkUp {
				networkDown = false
				if len(pendingReconnects) > 0 {
					logrus.Infof(
						"Network restored, reconnecting %d watcher(s)",
						len(pendingReconnects),
					)
					for _, ev := range pendingReconnects {
						key := ev.Mailbox.Alias + ev.Mailbox.Mailbox
						wg.Add(1)
						go reconnectWatcher(
							ev,
							running.Config[key],
							idleChan,
							boxChan,
							quitChan,
							wg,
							*dialRetries,
						)
					}
					pendingReconnects = nil
				}
			}
		case boxEvent := <-boxChan:
			l := logrus.WithField("alias", boxEvent.Mailbox.Alias).
				WithField("mailbox", boxEvent.Mailbox.Mailbox)
			if networkDown {
				l.Info("Watcher stopped, deferring reconnection until network is restored")
				pendingReconnects = append(pendingReconnects, boxEvent)
				continue
			}
			l.Info("Restarting watcher for mailbox")
			key := boxEvent.Mailbox.Alias + boxEvent.Mailbox.Mailbox
			wg.Add(1)
			go reconnectWatcher(
				boxEvent,
				running.Config[key],
				idleChan,
				boxChan,
				quitChan,
				wg,
				*dialRetries,
			)
		case <-quit:
			// OS asked nicely to close, we ask our
			// goroutines to do the same
			close(quitChan)
			idleForever = false
		case idleEvent := <-idleChan:
			wg.Add(1)
			go func() {
				defer wg.Done()
				running.Schedule(idleEvent, quitChan, queueChan)
			}()
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
	util.PrintDonate(os.Stderr, 11)
	logrus.Info("bye")
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
