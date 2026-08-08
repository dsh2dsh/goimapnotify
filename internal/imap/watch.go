package imap

// This file is part of goimapnotify
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
	"log/slog"
	"sync"
	"time"

	goimap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/dsh2dsh/goimapnotify/internal/box"
)

const maxBackoff = 5 * time.Minute

// WatchMailBox keeps track of the IDLE state of one Mailbox
type WatchMailBox struct {
	box         *box.Box
	events      chan<- *box.IDLE
	startupSync bool

	ctx     context.Context
	retries int
	once    *sync.Once

	client *imapclient.Client
}

// NewWatchBox creates a new instance of WatchMailBox and launches it
func NewWatchBox(m *box.Box, events chan<- *box.IDLE) *WatchMailBox {
	return &WatchMailBox{
		box:    m,
		events: events,
	}
}

func (self *WatchMailBox) WithStartupSync(v bool) *WatchMailBox {
	self.startupSync = v
	return self
}

func (self *WatchMailBox) Connect(ctx context.Context, retries int,
	once *sync.Once,
) error {
	self.ctx = ctx
	self.retries = retries
	self.once = once

	account := self.box.Account()

	c, err := New(account, self.retries, self.unilateralDataHandler)
	if errors.Is(err, ErrLoginFailed) {
		self.client = c
		return err
	} else if err != nil {
		slog.Error("Initial connection failed, retrying in background",
			slog.String("account", account.Alias), slog.Any("error", err))
		return nil
	}

	self.client = c
	self.once.Do(self.printConnected)
	return nil
}

func (self *WatchMailBox) unilateralDataHandler(o *imapclient.Options) {
	o.UnilateralDataHandler = &imapclient.UnilateralDataHandler{
		Expunge: self.expunge,
		Mailbox: self.mailbox,
	}
}

func (self *WatchMailBox) expunge(seqNum uint32) {
	self.box.ExistingEmail = max(0, self.box.ExistingEmail-1)
	slog.Info("IDLE expunge",
		slog.String("account", self.box.Alias()),
		slog.String("mailbox", self.box.Mailbox),
		slog.Uint64("seq", uint64(seqNum)),
		slog.Uint64("messages", uint64(self.box.ExistingEmail)))

	// messages deleted
	self.sendEvent(box.EventDeletedMail)
}

func (self *WatchMailBox) sendEvent(reason box.EventType) {
	select {
	case <-self.ctx.Done():
	case self.events <- box.NewEvent(self.box, reason):
	}
}

func (self *WatchMailBox) mailbox(data *imapclient.UnilateralDataMailbox) {
	l := slog.With(
		slog.String("account", self.box.Alias()),
		slog.String("mailbox", self.box.Mailbox))

	switch {
	case data.NumMessages != nil:
		// if the server messages are greater than current no of messages in RAM
		// only then take it as a new email otherwise ignore and update the
		// current copy from the server.
		numMessages := *data.NumMessages
		l = l.With(slog.Uint64("messages", uint64(numMessages)))

		if numMessages > self.box.ExistingEmail {
			// messages arrived
			self.box.ExistingEmail = numMessages
			self.sendEvent(box.EventNewMail)
		}

	case data.Flags != nil || data.PermanentFlags != nil:
		if len(data.Flags) != 0 {
			l = l.With(slog.Any("flags", data.Flags))
		}
		if len(data.PermanentFlags) != 0 {
			l = l.With(slog.Any("permanentFlags", data.PermanentFlags))
		}

		// messages flags updated
		self.sendEvent(box.EventFlagChanged)
	}
	l.Info("IDLE mailbox")
}

func (self *WatchMailBox) printConnected() {
	l := slog.With(slog.String("account", self.box.Alias()))
	if caps := AllCaps(self.client); len(caps) != 0 {
		l = l.With(slog.Any("capabilities", caps))
	}
	l.Info("connected")
}

func (self *WatchMailBox) Watch() {
	defer self.Close()
	for self.reconnect() {
		if self.ctx.Err() != nil {
			return
		} else if !self.watch() {
			break
		}
		self.Close()
	}

	if self.ctx.Err() == nil {
		self.sendEvent(box.StopWatching)
	}
}

func (self *WatchMailBox) reconnect() bool {
	if self.client != nil {
		return true
	}

	l := slog.With(
		slog.String("alias", self.box.Alias()),
		slog.String("mailbox", self.box.Mailbox))
	backoff := time.Second

	for {
		select {
		case <-time.After(backoff):
		case <-self.ctx.Done():
			l.Info("Reconnection cancelled, shutting down")
			return false
		}

		c, err := New(self.box.Account(), self.retries, self.unilateralDataHandler)
		if errors.Is(err, ErrLoginFailed) {
			l.Error("Reconnection failed", slog.Any("error", err))
			self.client = c
			return false
		} else if err != nil {
			backoff = min(backoff*2, maxBackoff)
			l.Error("Reconnection failed",
				slog.Duration("retrying", backoff),
				slog.Any("error", err))
			continue
		}

		l.Info("Reconnected successfully")
		self.client = c
		self.once.Do(self.printConnected)
		return true
	}
}

func (self *WatchMailBox) watch() bool {
	l := slog.With(
		slog.String("alias", self.box.Alias()),
		slog.String("mailbox", self.box.Mailbox))

	status, err := self.client.Select(self.box.Mailbox, &goimap.SelectOptions{
		ReadOnly: true,
	}).Wait()
	if err != nil {
		l.Warn("cannot select mailbox, skipped!", slog.Any("error", err))
		return false
	}

	self.box.ExistingEmail = status.NumMessages
	l.Info("SELECT mailbox",
		slog.Uint64("messages", uint64(self.box.ExistingEmail)))

	// Start idling
	idleCmd, err := self.client.Idle()
	if err != nil {
		l.Error("IDLE command failed", slog.Any("error", err))
		return true
	}
	defer idleCmd.Close()

	// issue fake event to trigger a first time sync
	if self.startupSync {
		l.Info(
			"issuing fake IMAP Event for first time sync (skipping post-commands)")
		self.sendEvent(box.EventSync)
	}

	done := make(chan error, 1)
	go func() {
		l.Info("Watching mailbox")
		done <- idleCmd.Wait()
	}()

	select {
	case <-self.ctx.Done():
		// the main event loop is asking us to stop
		l.Info("stopping client watching mailbox")
		if err := idleCmd.Close(); err != nil {
			l.Error("stop idling finished with error", slog.Any("error", err))
			return false
		}

		if err := <-done; err != nil {
			l.Error("IDLE command failed", slog.Any("error", err))
		}
		return false

	case err := <-done:
		if err != nil {
			l.Info("watching stopped because of an error",
				slog.Any("error", err))
			break
		}
		l.Info("done watching mailbox")
	}
	return true
}

func (self *WatchMailBox) Close() {
	if self.client == nil {
		return
	}

	c := self.client
	self.client = nil

	if err := c.Close(); err != nil {
		slog.Error("closing connection finished with error",
			slog.String("account", self.box.Alias()),
			slog.String("mailbox", self.box.Mailbox),
			slog.Any("error", err))
	}
}
