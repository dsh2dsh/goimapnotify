package imap

// This file is part of goimapnotify
// Copyright (C) 2017-2025  Jorge Javier Araya Navarro

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
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	imapid "github.com/emersion/go-imap-id"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-sasl"

	"github.com/dsh2dsh/goimapnotify/internal/config"
	"github.com/dsh2dsh/goimapnotify/internal/util"
)

// Version is set at build time
var Version string = "unknown"

// New creates a new IMAP client with the given configuration.
func New(conf *config.NotifyConfig, retries int,
) (c *client.Client, err error) {
	server := conf.Host + ":" + strconv.Itoa(conf.Port)

	for wait := 1; wait <= retries; wait++ {
		if conf.TLS && !conf.TLSOptions.STARTTLS {
			c, err = client.DialTLS(server, &tls.Config{
				ServerName:         conf.Host,
				InsecureSkipVerify: !conf.TLSOptions.RejectUnauthorized,
				MinVersion:         tls.VersionTLS12,
			})
		} else {
			c, err = client.Dial(server)
		}

		if err == nil {
			break
		}

		slog.Error(
			"there was an error while dialing to host, waiting to try again",
			slog.Int("wait_seconds", wait),
			slog.String("host", server),
			slog.Bool("tls", conf.TLS),
			slog.Bool("startTLS", conf.TLSOptions.STARTTLS),
			slog.Any("error", err),
		)
		time.Sleep(time.Second * time.Duration(wait))
	}

	if err != nil {
		return nil, fmt.Errorf(
			"cannot dial to %s:%d, tls: %t, start TLS: %t. error: %w",
			conf.Host,
			conf.Port,
			conf.TLS,
			conf.TLSOptions.STARTTLS,
			err,
		)
	}

	// turn on debugging
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		pr, pw := io.Pipe()
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigChan
			_ = pr.Close() // close the pipe when the program is about to close
		}()

		go util.CensorCredentials(pr, os.Stdout)
		c.SetDebug(pw)
	}

	if conf.TLS && conf.TLSOptions.STARTTLS {
		err = c.StartTLS(&tls.Config{
			ServerName:         conf.Host,
			InsecureSkipVerify: !conf.TLSOptions.RejectUnauthorized,
		})
		if err != nil {
			return nil, fmt.Errorf("imap: %w", err)
		}
	}

	// Handle ID command - only works with real client
	if ok, err := c.Support(imapid.Capability); err != nil {
		return nil, fmt.Errorf(
			"unable to check support for capability %s, error: %w",
			imapid.Capability,
			err,
		)
	} else if ok && conf.EnableIDCommand {
		idClient := imapid.NewClient(c)
		_, err := idClient.ID(imapid.ID{
			imapid.FieldName:    "goimapnotify",
			imapid.FieldVersion: Version,
		})
		if err != nil {
			if !strings.Contains(err.Error(), "Parameter list contains a non-string: expected a string") && !strings.Contains(err.Error(), "Unrecognised command") {
				return nil, fmt.Errorf("imap: %w", err)
			}
			slog.Debug(
				"IMAP server supports ID command but gave malformed response, ignoring...",
				slog.Any("error", err),
			)
		}
	}

	if conf.XOAuth2 {
		okBearer, err := c.SupportAuth(sasl.OAuthBearer)
		if err != nil {
			return nil, ErrCannotCheckSupportedAuth
		}
		okXOAuth2, err := c.SupportAuth(Xoauth2)
		if err != nil {
			return nil, ErrCannotCheckSupportedAuth
		}

		if !okXOAuth2 && !okBearer {
			return nil, ErrTokenAuthNotSupported
		}

		if okBearer {
			sasl_oauth := &sasl.OAuthBearerOptions{
				Username: conf.Username,
				// Use something like https://github.com/google/oauth2l
				// in your passwordCmd to grab the token as a password
				Token: conf.Password,
				Host:  conf.Host,
				Port:  conf.Port,
			}
			sasl_client := sasl.NewOAuthBearerClient(sasl_oauth)
			if err := c.Authenticate(sasl_client); err != nil {
				return nil, fmt.Errorf("imap: %w", err)
			}
		} else if okXOAuth2 {
			sasl_xoauth2 := NewXoauth2Client(conf.Username, conf.Password)
			if err := c.Authenticate(sasl_xoauth2); err != nil {
				return nil, fmt.Errorf("imap: %w", err)
			}
		}
	} else {
		if err := c.Login(conf.Username, conf.Password); err != nil {
			return nil, fmt.Errorf("imap: %w", err)
		}
	}
	return c, nil
}
