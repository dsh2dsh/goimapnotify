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
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dsh2dsh/goimapnotify/internal/box"
	"github.com/dsh2dsh/goimapnotify/internal/command"
)

// RunningBox manages command scheduling and execution
type RunningBox struct {
	Debug bool
	Wait  int
	/*
	 * Use map to create a different timer for each
	 * username-mailbox combination
	 */
	timer sync.Map
}

// NewRunningBox creates a new RunningBox instance
func NewRunningBox(debug bool, wait int) *RunningBox {
	return &RunningBox{
		Debug: debug,
		Wait:  wait,
		timer: sync.Map{},
	}
}

// Schedule debounces events before queueing them for execution
func (r *RunningBox) Schedule(rsp *box.IDLE, done <-chan struct{},
	queue chan *box.IDLE,
) {
	l := slog.With(
		slog.String("alias", rsp.Alias()),
		slog.String("mailbox", rsp.Mailbox()))

	if rsp.Skip() {
		l.Warn("No command for event, skipping scheduling...",
			slog.String("reason", rsp.Reason.String()))
		return
	}

	key := rsp.Alias() + rsp.Mailbox()
	wait := time.Duration(r.Wait) * time.Second
	when := time.Now().Add(wait).Format(time.RFC850)

	value, exists := r.timer.LoadOrStore(key, time.NewTimer(wait))
	wristwatch := value.(*time.Timer)

	main := true // main is true for the goroutine that will run sync
	if exists {
		// Stop should be called before Reset according to go docs
		if wristwatch.Stop() {
			main = false // stopped running timer -> main is another goroutine
		}
		wristwatch.Reset(wait)
		r.timer.Store(key, wristwatch)
	}

	if main {
		l.Info("scheduled syncing",
			slog.String("reason", rsp.Reason.String()),
			slog.String("when", when),
			slog.Duration("wait", wait))
		select {
		case <-wristwatch.C:
			queue <- rsp
		case <-done:
			// just get out
		}
	} else {
		l.Info("rescheduled syncing",
			slog.String("reason", rsp.Reason.String()),
			slog.String("when", when),
			slog.Duration("wait", wait))
	}
}

// Run executes commands based on the event type
func (r *RunningBox) Run(rsp *box.IDLE) error {
	l := slog.With(
		slog.String("alias", rsp.Alias()),
		slog.String("mailbox", rsp.Mailbox()))
	if r.Debug {
		l.Info("Running synchronization...")
	}

	onCommand, err := rsp.Box.RenderCommand(rsp)
	if err != nil {
		return err
	} else if onCommand == "" || onCommand == "SKIP" {
		return nil
	}

	cmd := command.New(onCommand)
	if err := execCommand(cmd, onCommand); err != nil {
		return fmt.Errorf("%s command failed: %w", rsp.CommandName(), err)
	}

	postCommand, err := rsp.Box.RenderPostCommand(rsp)
	if err != nil {
		return err
	} else if postCommand == "" || postCommand == "SKIP" {
		return nil
	}

	cmd = command.New(postCommand)
	if err := execCommand(cmd, postCommand); err != nil {
		return fmt.Errorf("%sPost command failed: %w", rsp.CommandName(), err)
	}
	return nil
}
