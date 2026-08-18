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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	goimap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-sasl"

	"github.com/dsh2dsh/goimapnotify/internal/config"
)

type option func(*imapclient.Options)

// Version is set at build time
var Version string = "unknown"

// New creates a new IMAP client with the given configuration.
func New(conf *config.NotifyConfig, retries int, opts ...option,
) (c *imapclient.Client, err error) {
	imapOpts := &imapclient.Options{}
	for _, fn := range opts {
		fn(imapOpts)
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

		go censorCredentials(pr, os.Stdout)
		imapOpts.DebugWriter = pw
	}

	server := conf.Host + ":" + strconv.Itoa(conf.Port)
	for wait := 1; wait <= retries; wait++ {
		switch {
		case conf.TLS && !conf.TLSOptions.STARTTLS:
			imapOpts.TLSConfig = &tls.Config{
				ServerName:         conf.Host,
				InsecureSkipVerify: !conf.TLSOptions.GetRejectUnauthorized(),
				MinVersion:         tls.VersionTLS12,
			}
			c, err = imapclient.DialTLS(server, imapOpts)
		case conf.TLS && conf.TLSOptions.STARTTLS:
			imapOpts.TLSConfig = &tls.Config{
				ServerName:         conf.Host,
				InsecureSkipVerify: !conf.TLSOptions.GetRejectUnauthorized(),
			}
			c, err = imapclient.DialStartTLS(server, imapOpts)
		default:
			c, err = imapclient.DialInsecure(server, imapOpts)
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
			"cannot dial to %s:%d, tls: %t, start TLS: %t: %w",
			conf.Host,
			conf.Port,
			conf.TLS,
			conf.TLSOptions.STARTTLS,
			err,
		)
	}

	// Handle ID command - only works with real client
	if conf.EnableIDCommand {
		if caps := c.Caps(); caps != nil && caps.Has(goimap.CapID) {
			_, err := c.ID(&goimap.IDData{
				Name:    "goimapnotify",
				Version: Version,
			}).Wait()
			if err != nil {
				slog.Error("IMAP ID command failed", slog.Any("error", err))
			}
		}
	}

	if !conf.XOAuth2 {
		if err := c.Login(conf.Username, conf.Password).Wait(); err != nil {
			if resp, ok := errors.AsType[*goimap.Error](err); ok {
				if resp.Type == goimap.StatusResponseTypeNo {
					return nil, fmt.Errorf("%w: %w", ErrLoginFailed, err)
				}
			}
			return nil, fmt.Errorf("login authentication: %w", err)
		}
		return c, nil
	}

	var saslClient sasl.Client
	caps := c.Caps()
	hasBearerAuth := caps != nil && caps.Has(goimap.AuthCap(sasl.OAuthBearer))

	if hasBearerAuth {
		saslClient = sasl.NewOAuthBearerClient(&sasl.OAuthBearerOptions{
			Username: conf.Username,
			// Use something like https://github.com/google/oauth2l in your
			// passwordCmd to grab the token as a password
			Token: conf.Password,
			Host:  conf.Host,
			Port:  conf.Port,
		})
	} else {
		saslClient = NewXoauth2Client(conf.Username, conf.Password)
	}

	if err := c.Authenticate(saslClient); err != nil {
		return nil, fmt.Errorf(
			"%w: SASL authentication: %w", ErrLoginFailed, err)
	}
	return c, nil
}
