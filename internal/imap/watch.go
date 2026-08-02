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
	"log/slog"
	"os"

	goimap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/dsh2dsh/goimapnotify/internal/box"
)

func WithWatcher(w *WatchMailBox) option {
	return func(o *imapclient.Options) {
		if o.UnilateralDataHandler != nil {
			o.UnilateralDataHandler.Expunge = w.expunge
			o.UnilateralDataHandler.Mailbox = w.mailbox
			return
		}

		o.UnilateralDataHandler = &imapclient.UnilateralDataHandler{
			Expunge: w.expunge,
			Mailbox: w.mailbox,
		}
	}
}

// BoxEvent helps in communication between the box watch launcher and the box
// watching goroutines
type BoxEvent struct {
	Mailbox *box.Box
	Skipped bool
}

// WatchMailBox keeps track of the IDLE state of one Mailbox
type WatchMailBox struct {
	client    *imapclient.Client
	box       *box.Box
	idleEvent chan<- *box.IDLE
	boxEvent  chan<- *BoxEvent
	quit      <-chan struct{}

	startupSync bool
}

// NewWatchBox creates a new instance of WatchMailBox and launches it
func NewWatchBox(
	m *box.Box,
	i chan<- *box.IDLE,
	b chan<- *BoxEvent,
	q <-chan struct{},
) *WatchMailBox {
	return &WatchMailBox{
		box:       m,
		idleEvent: i,
		boxEvent:  b,
		quit:      q,
	}
}

func (self *WatchMailBox) WithStartupSync(v bool) *WatchMailBox {
	self.startupSync = v
	return self
}

func (self *WatchMailBox) expunge(seqNum uint32) {
	self.box.ExistingEmail = max(0, self.box.ExistingEmail-1)
	slog.Info("IDLE expunge",
		slog.String("account", self.box.Alias()),
		slog.String("mailbox", self.box.Mailbox),
		slog.Uint64("seq", uint64(seqNum)),
		slog.Uint64("messages", uint64(self.box.ExistingEmail)))

	// messages deleted
	self.idleEvent <- &box.IDLE{
		Reason: box.EventDeletedMail,
		Box:    self.box,
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
			self.idleEvent <- &box.IDLE{
				Reason: box.EventNewMail,
				Box:    self.box,
			}
		}

	case data.Flags != nil || data.PermanentFlags != nil:
		if len(data.Flags) != 0 {
			l = l.With(slog.Any("flags", data.Flags))
		}
		if len(data.PermanentFlags) != 0 {
			l = l.With(slog.Any("permanentFlags", data.PermanentFlags))
		}

		// messages flags updated
		self.idleEvent <- &box.IDLE{
			Reason: box.EventFlagChanged,
			Box:    self.box,
		}
	}
	l.Info("IDLE mailbox")
}

// Watch starts watching the mailbox for IDLE events
func (self *WatchMailBox) Watch(c *imapclient.Client) {
	self.client = c
	l := slog.With(
		slog.String("alias", self.box.Alias()),
		slog.String("mailbox", self.box.Mailbox))

	status, err := self.client.Select(self.box.Mailbox, &goimap.SelectOptions{
		ReadOnly: true,
	}).Wait()
	if err != nil {
		l.Warn("cannot select mailbox, skipped!", slog.Any("error", err))
		self.boxEvent <- &BoxEvent{Mailbox: self.box, Skipped: true}
		return
	}

	self.box.ExistingEmail = status.NumMessages
	l.Info("SELECT mailbox",
		slog.Uint64("messages", uint64(self.box.ExistingEmail)))

	// Start idling
	idleCmd, err := c.Idle()
	if err != nil {
		l.Error("IDLE command failed", slog.Any("error", err))
		os.Exit(1)
	}
	defer idleCmd.Close()

	done := make(chan error, 1)
	go func() {
		l.Info("Watching mailbox")
		done <- idleCmd.Wait()
	}()

	// issue fake event to trigger a first time sync
	if self.startupSync {
		go func() {
			l.Info(
				"issuing fake IMAP Event for first time sync (skipping post-commands)")
			self.idleEvent <- &box.IDLE{
				Reason: box.EventSync,
				Box:    self.box,
			}
		}()
	}

	select {
	case <-self.quit:
		// the main event loop is asking us to stop
		l.Info("stopping client watching mailbox")
		if err := idleCmd.Close(); err != nil {
			l.Error("failed to stop idling", slog.Any("error", err))
			return
		}

		if err := <-done; err != nil {
			l.Error("IDLE command failed", slog.Any("error", err))
		}

	case err := <-done:
		l.Info("done watching mailbox")
		if err != nil {
			l.Info("watching stopped because of an error",
				slog.Any("error", err))
			self.boxEvent <- &BoxEvent{Mailbox: self.box}
		}
	}
}
