package runner

// This file is part of goimapnotify
// Copyright (C) 2017-2026	Jorge Javier Araya Navarro

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
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dsh2dsh/goimapnotify/internal/config"
	"github.com/dsh2dsh/goimapnotify/internal/logging"
	"github.com/dsh2dsh/goimapnotify/internal/model"
)

type Runner struct {
	wait     time.Duration
	maxWait  time.Duration
	notifier *notifier

	handlers map[*model.Box]*handler
	wg       sync.WaitGroup
}

func New(n int, wait time.Duration) *Runner {
	return &Runner{
		wait:     wait * time.Second,
		handlers: make(map[*model.Box]*handler, n),
	}
}

func (self *Runner) WithMaxDelay(v time.Duration) *Runner {
	self.maxWait = v
	return self
}

func (self *Runner) EnableDesktopNotifications(ctx context.Context,
	cfg config.DesktopNotification, n int,
) error {
	self.notifier = NewNotifier(n)
	if err := self.notifier.Connect(ctx, cfg); err != nil {
		return fmt.Errorf("create desktop notifier: %w", err)
	}
	return nil
}

func (self *Runner) WithBoxes(boxes []*model.Box) *Runner {
	for _, b := range boxes {
		h := NewHandler(b, self.wait).WithMaxDelay(self.maxWait)
		if self.notifier != nil {
			h.WithNotifier(self.notifier)
		}
		self.handlers[b] = h
	}
	return self
}

func (self *Runner) Schedule(ctx context.Context, e *model.IDLE) {
	h := self.handlers[e.Box()]
	if h.Schedule(e) {
		self.wg.Go(func() { h.Run(ctx) })
	}
}

func (self *Runner) Wait() { self.wg.Wait() }

func (self *Runner) Close() {
	if self.notifier != nil {
		self.notifier.Close()
	}
}

func (self *Runner) NotifyNewMails(ctx context.Context, b *model.Box,
	threads []model.Thread,
) error {
	if self.notifier == nil {
		return nil
	}

	logging.FromContext(ctx).Debug("notify new mails",
		slog.Int("threads", len(threads)))

	h := self.handlers[b]
	for _, thread := range threads {
		n, err := NewNewMail(h, thread)
		if err != nil {
			return err
		}
		if _, err := n.Send(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (self *Runner) NotifyError(ctx context.Context,
	accountConfig *config.NotifyConfig, summary string, bodyErr error,
) {
	if self.notifier == nil {
		return
	}

	l := logging.FromContext(ctx)
	l.Debug("notify jmap error state", slog.Any("error", bodyErr))

	n, err := NewErrorNotification(self.notifier, accountConfig, summary,
		bodyErr.Error())
	if err != nil {
		l.Error("unable notify error state", slog.Any("error", err))
		return
	}

	if _, err := n.Send(ctx); err != nil {
		l.Error("unable notify error state", slog.Any("error", err))
	}
}

func (self *Runner) NotifyOK(ctx context.Context,
	accountConfig *config.NotifyConfig, summary, body string,
) {
	if self.notifier == nil {
		return
	}

	l := logging.FromContext(ctx)
	l.Debug("notify jmap OK state")

	n, err := NewErrorNotification(self.notifier, accountConfig, summary, body)
	if err != nil {
		l.Error("unable notify OK state", slog.Any("error", err))
		return
	}

	if _, err := n.Send(ctx); err != nil {
		l.Error("unable notify OK state", slog.Any("error", err))
	}
}

func (self *Runner) NotifyUnreadEmails(ctx context.Context, b *model.Box,
	unreadEmails uint64,
) (sent SentNotification, _ error) {
	if self.notifier == nil {
		return sent, nil
	}

	logging.FromContext(ctx).Debug("notify unread emails",
		slog.Uint64("unreadEmails", unreadEmails))

	n, err := NewUnreadEmails(self.handlers[b], unreadEmails)
	if err != nil {
		return sent, err
	}
	return n.Send(ctx)
}
