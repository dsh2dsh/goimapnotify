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
	"io"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-sasl"

	"gitlab.com/shackra/goimapnotify/internal/config"
)

// IMAPClientInterface defines the methods required from an IMAP client.
// This interface allows for mocking in tests.
type IMAPClientInterface interface {
	// Authentication methods
	Login(username, password string) error
	Authenticate(auth sasl.Client) error
	SupportAuth(mech string) (bool, error)

	// Connection methods
	StartTLS(config *tls.Config) error
	Support(cap string) (bool, error)
	Logout() error
	LoggedOut() <-chan struct{}
	SetDebug(w io.Writer)

	// Mailbox methods
	Select(name string, readOnly bool) (*imap.MailboxStatus, error)
}

// IMAPDialer defines the interface for creating IMAP connections.
// This allows tests to inject mock dialers instead of making real connections.
type IMAPDialer interface {
	// Dial connects to an IMAP server without TLS
	Dial(addr string) (IMAPClientInterface, error)
	// DialTLS connects to an IMAP server with TLS
	DialTLS(addr string, config *tls.Config) (IMAPClientInterface, error)
}

// IdleClientInterface defines the methods required from an IDLE client.
// This interface allows for mocking IDLE functionality in tests.
type IdleClientInterface interface {
	// IdleWithFallback starts the IDLE command with a fallback to polling
	IdleWithFallback(stop <-chan struct{}, pollInterval time.Duration) error
}

// WatchClientInterface defines the methods required for watching mailboxes.
// This combines the IMAP client and IDLE client functionality needed by WatchMailBox.
type WatchClientInterface interface {
	// Select selects a mailbox
	Select(name string, readOnly bool) (*imap.MailboxStatus, error)
	// LoggedOut returns a channel that is closed when the client is logged out
	LoggedOut() <-chan struct{}
	// IdleWithFallback starts the IDLE command with a fallback to polling
	IdleWithFallback(stop <-chan struct{}, pollInterval time.Duration) error
	// SetUpdatesChannel sets the channel for receiving updates
	SetUpdatesChannel(updates chan client.Update)
}

// ClientFactory is a function type that creates IMAP clients.
// It allows dependency injection for testing purposes.
type ClientFactory func(conf config.NotifyConfig, retries int) (IMAPClientInterface, error)

// IdleClientFactory is a function type that creates IDLE-enabled IMAP clients.
// It allows dependency injection for testing purposes.
type IdleClientFactory func(client IMAPClientInterface) IdleClientInterface

// defaultDialer implements IMAPDialer using the real go-imap client
type defaultDialer struct{}

// clientWrapper wraps *client.Client to implement IMAPClientInterface
type clientWrapper struct {
	*client.Client
}

// Dial connects to an IMAP server without TLS
func (d *defaultDialer) Dial(addr string) (IMAPClientInterface, error) {
	c, err := client.Dial(addr)
	if err != nil {
		return nil, err
	}
	return &clientWrapper{c}, nil
}

// DialTLS connects to an IMAP server with TLS
func (d *defaultDialer) DialTLS(addr string, config *tls.Config) (IMAPClientInterface, error) {
	c, err := client.DialTLS(addr, config)
	if err != nil {
		return nil, err
	}
	return &clientWrapper{c}, nil
}

// DefaultDialer returns the default IMAPDialer that uses the real go-imap client
func DefaultDialer() IMAPDialer {
	return &defaultDialer{}
}

// GetUnderlyingClient returns the underlying *client.Client from a clientWrapper.
// This is needed for creating IdleClient and ID client.
// Returns nil if the client is not a clientWrapper.
func GetUnderlyingClient(c IMAPClientInterface) *client.Client {
	if wrapper, ok := c.(*clientWrapper); ok {
		return wrapper.Client
	}
	return nil
}

// IMAPIDLEClientWrapper wraps IMAPIDLEClient to implement WatchClientInterface
type IMAPIDLEClientWrapper struct {
	*IMAPIDLEClient
}

// SetUpdatesChannel sets the channel for receiving updates
func (w *IMAPIDLEClientWrapper) SetUpdatesChannel(updates chan client.Update) {
	w.IMAPIDLEClient.Updates = updates
}

// WrapIMAPIDLEClient wraps an IMAPIDLEClient to implement WatchClientInterface
func WrapIMAPIDLEClient(c *IMAPIDLEClient) WatchClientInterface {
	return &IMAPIDLEClientWrapper{c}
}
