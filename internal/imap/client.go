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
	"crypto/tls"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	imapid "github.com/emersion/go-imap-id"
	idle "github.com/emersion/go-imap-idle"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-sasl"
	"github.com/sirupsen/logrus"

	"github.com/dsh2dsh/goimapnotify/internal/config"
	"github.com/dsh2dsh/goimapnotify/internal/util"
)

// Version is set at build time
var Version string = "unknown"

// IMAPIDLEClient wraps the IMAP client with IDLE support
type IMAPIDLEClient struct {
	*client.Client
	*idle.IdleClient
}

// NewClient creates a new IMAP client with the given configuration.
// This is a convenience wrapper around NewClientWithDialer that uses the default dialer.
func NewClient(conf config.NotifyConfig, retries int) (*client.Client, error) {
	c, err := NewClientWithDialer(DefaultDialer(), conf, retries)
	if err != nil {
		return nil, err
	}

	// Get the underlying client for backward compatibility
	if underlying := GetUnderlyingClient(c); underlying != nil {
		return underlying, nil
	}

	// This shouldn't happen with the default dialer, but handle it gracefully
	return nil, fmt.Errorf("unexpected client type returned from dialer")
}

// NewClientWithDialer creates a new IMAP client with the given configuration and dialer.
// This function allows for dependency injection of the dialer for testing purposes.
func NewClientWithDialer(
	dialer IMAPDialer,
	conf config.NotifyConfig,
	retries int,
) (IMAPClientInterface, error) {
	server := fmt.Sprintf("%s:%d", conf.Host, conf.Port)

	var c IMAPClientInterface
	var err error

	for wait := 1; wait <= retries; wait++ {
		if conf.TLS && !conf.TLSOptions.STARTTLS {
			c, err = dialer.DialTLS(server, &tls.Config{
				ServerName:         conf.Host,
				InsecureSkipVerify: !conf.TLSOptions.RejectUnauthorized,
				MinVersion:         tls.VersionTLS12,
			})
		} else {
			c, err = dialer.Dial(server)
		}

		if err == nil {
			break
		}

		logrus.WithFields(logrus.Fields{"host": server, "tls": conf.TLS, "START TLS": conf.TLSOptions.STARTTLS, "error": err}).
			Errorf("there was an error while dialing to host, waiting for %d second(s) to try again", wait)
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
	if logrus.GetLevel() == logrus.DebugLevel {
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
			return nil, err
		}
	}

	// Handle ID command - only works with real client
	if underlying := GetUnderlyingClient(c); underlying != nil {
		if ok, err := c.Support(imapid.Capability); err != nil {
			return nil, fmt.Errorf(
				"unable to check support for capability %s, error: %w",
				imapid.Capability,
				err,
			)
		} else if ok && conf.EnableIDCommand {
			idClient := imapid.NewClient(underlying)
			_, err := idClient.ID(imapid.ID{
				imapid.FieldName:    "goimapnotify",
				imapid.FieldVersion: Version,
			})
			if err != nil {
				if !strings.Contains(err.Error(), "Parameter list contains a non-string: expected a string") && !strings.Contains(err.Error(), "Unrecognised command") {
					return nil, err
				}
				logrus.WithError(err).Debug("IMAP server supports ID command but gave malformed response, ignoring...")
			}
		}
	} else {
		// For mock clients, just check support without running ID command
		if _, err := c.Support(imapid.Capability); err != nil {
			return nil, fmt.Errorf(
				"unable to check support for capability %s, error: %w",
				imapid.Capability,
				err,
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
				return nil, err
			}
		} else if okXOAuth2 {
			sasl_xoauth2 := NewXoauth2Client(conf.Username, conf.Password)
			if err := c.Authenticate(sasl_xoauth2); err != nil {
				return nil, err
			}
		}
	} else {
		if err := c.Login(conf.Username, conf.Password); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// NewIMAPIDLEClient creates a new IMAP client with IDLE support
func NewIMAPIDLEClient(conf config.NotifyConfig, retries int) (*IMAPIDLEClient, error) {
	confCMDExecuted := util.RetrieveCmd(conf)
	i, err := NewClient(confCMDExecuted, retries)
	if err != nil {
		return nil, err
	}

	idleC := idle.NewClient(i)

	amount := 25 // default per https://github.com/emersion/go-imap-idle/blob/db256843144576c70e551f0732f1d1d3b5bec67e/client.go#L11
	if conf.IDLELogoutTimeout > 0 {
		amount = conf.IDLELogoutTimeout
	}
	idleC.LogoutTimeout = time.Duration(amount) * time.Minute

	return &IMAPIDLEClient{i, idleC}, nil
}
