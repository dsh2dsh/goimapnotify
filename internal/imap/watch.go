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
	"strings"
	"sync"

	"github.com/emersion/go-imap/client"

	"github.com/dsh2dsh/goimapnotify/internal/config"
)

// BoxEvent helps in communication between the box watch launcher and the box
// watching goroutines
type BoxEvent struct {
	UniqID  string
	Mailbox config.Box
}

// WatchMailBox keeps track of the IDLE state of one Mailbox
type WatchMailBox struct {
	client    *client.Client
	box       config.Box
	idleEvent chan<- config.IDLEEvent
	boxEvent  chan<- BoxEvent
	quit      <-chan struct{}
}

// NewWatchBox creates a new instance of WatchMailBox and launches it
func NewWatchBox(
	c *client.Client,
	f config.NotifyConfig,
	m config.Box,
	i chan<- config.IDLEEvent,
	b chan<- BoxEvent,
	q <-chan struct{},
	wg *sync.WaitGroup,
) {
	w := WatchMailBox{
		client:    c,
		box:       m,
		idleEvent: i,
		boxEvent:  b,
		quit:      q,
	}
	wg.Go(w.Watch)
}

// Watch starts watching the mailbox for IDLE events
func (w *WatchMailBox) Watch() {
	l := slog.With(
		slog.String("alias", w.box.Alias),
		slog.String("mailbox", w.box.Mailbox),
	)

	status, err := w.client.Select(w.box.Mailbox, true)
	if err != nil {
		if strings.Contains(err.Error(), "reason: Unknown Mailbox") {
			l.Warn("cannot select mailbox, skipped!", slog.Any("error", err))
			return
		}
		l.Error("cannot select mailbox", slog.Any("error", err))
		os.Exit(1)
	}
	w.box.ExistingEmail = status.Messages
	l.Debug("existing mail", slog.Uint64("count", uint64(w.box.ExistingEmail)))

	updates := make(chan client.Update)
	w.client.Updates = updates

	done := make(chan error, 1)
	go func() {
		l.Info("Watching mailbox")
		done <- w.client.Idle(w.quit, nil)
	}()

	// issue fake event to trigger a first time sync
	go func() {
		l.Info("issuing fake IMAP Event for first time sync (skipping post-commands)")
		w.idleEvent <- config.IDLEEvent{
			Alias:         w.box.Alias,
			Mailbox:       w.box.Mailbox,
			Reason:        config.SYNC,
			ExistingEmail: 0,
			Box:           w.box,
		}
	}()

	kickedOut := w.client.LoggedOut()

	// Block and process IDLE events
	run := true
idleLoop:
	for run {
		select {
		case update := <-updates:
			if mu, ok := update.(*client.MailboxUpdate); ok {
				// if the server messages are greater than current no of messages in RAM
				// only then take it as a new email otherwise ignore and update the
				// current copy from the server.
				if mu.Mailbox.Messages > w.box.ExistingEmail {
					// messages arrived
					w.idleEvent <- config.IDLEEvent{
						Alias:         w.box.Alias,
						Mailbox:       w.box.Mailbox,
						Reason:        config.NEWMAIL,
						ExistingEmail: int(mu.Mailbox.Messages),
						Box:           w.box,
					}
				}
				w.box.ExistingEmail = mu.Mailbox.Messages
			}

			if _, ok := update.(*client.MessageUpdate); ok {
				// messages flags updated
				w.idleEvent <- config.IDLEEvent{
					Alias:   w.box.Alias,
					Mailbox: w.box.Mailbox,
					Reason:  config.FLAGCHANGED,
					Box:     w.box,
				}
			}

			if _, ok := update.(*client.ExpungeUpdate); ok {
				// messages deleted
				w.idleEvent <- config.IDLEEvent{
					Alias:   w.box.Alias,
					Mailbox: w.box.Mailbox,
					Reason:  config.DELETEDMAIL,
					Box:     w.box,
				}
			}

		case <-w.quit:
			// the main event loop is asking us to stop
			l.Warn("stopping client watching mailbox")
			break idleLoop

		case finished := <-done:
			l.Warn("done watching mailbox")
			if finished != nil {
				l.Info("watching stopped because of an error",
					slog.Any("error", finished))
				w.boxEvent <- BoxEvent{UniqID: w.box.Alias + w.box.Mailbox, Mailbox: w.box}
			}
			break idleLoop

		case <-kickedOut:
			l.Info("connection to the server closed")
			w.boxEvent <- BoxEvent{UniqID: w.box.Alias + w.box.Mailbox, Mailbox: w.box}
			break idleLoop
		}
	}
}
