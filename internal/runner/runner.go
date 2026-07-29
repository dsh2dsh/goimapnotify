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
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"sync"
	"text/template"
	"time"

	"github.com/dsh2dsh/goimapnotify/internal/command"
	"github.com/dsh2dsh/goimapnotify/internal/config"
)

// RunningBox manages command scheduling and execution
type RunningBox struct {
	Debug bool
	Wait  int
	/*
	 * Use map to create a different timer for each
	 * username-mailbox combination
	 */
	timer  sync.Map
	Config map[string]*config.NotifyConfig
}

// NewRunningBox creates a new RunningBox instance
func NewRunningBox(debug bool, wait int) *RunningBox {
	return &RunningBox{
		Debug:  debug,
		Wait:   wait,
		timer:  sync.Map{},
		Config: make(map[string]*config.NotifyConfig),
	}
}

// Schedule debounces events before queueing them for execution
func (r *RunningBox) Schedule(rsp *config.IDLEEvent, done <-chan struct{}, queue chan *config.IDLEEvent) {
	l := slog.With(
		slog.String("alias", rsp.Alias),
		slog.String("mailbox", rsp.Mailbox),
	)
	if ShouldSkip(rsp.Box) {
		l.Warn("No command for event, skipping scheduling...",
			slog.String("reason", rsp.Reason.String()))
		return
	}

	key := rsp.Alias + rsp.Mailbox
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
func (r *RunningBox) Run(rsp *config.IDLEEvent) error {
	l := slog.With(
		slog.String("alias", rsp.Alias),
		slog.String("mailbox", rsp.Mailbox),
	)
	if r.Debug {
		l.Info("Running synchronization...")
	}

	switch rsp.Reason {
	case config.NEWMAIL:
		err := prepareAndRun(rsp.Box.OnNewMail, rsp.Box.OnNewMailPost, rsp)
		if err != nil {
			return err
		}
	case config.FLAGCHANGED:
		err := prepareAndRun(rsp.Box.OnChangedMail, rsp.Box.OnChangedMailPost, rsp)
		if err != nil {
			return err
		}
	case config.DELETEDMAIL:
		err := prepareAndRun(rsp.Box.OnDeletedMail, rsp.Box.OnDeletedMailPost, rsp)
		if err != nil {
			return err
		}
	case config.SYNC:
		err := prepareAndRun(rsp.Box.OnNewMail, "", rsp)
		if err != nil {
			return err
		}
	default:
		l.Error("unknown reason value, ignoring...",
			slog.String("reason", rsp.Reason.String()))
	}
	return nil
}

func prepareAndRun(on, onpost string, event *config.IDLEEvent) error {
	callKind := "New"
	if event.Reason == config.DELETEDMAIL {
		callKind = "Deleted"
	}
	if event.Reason == config.FLAGCHANGED {
		callKind = "Changed"
	}
	if event.Reason == config.SYNC {
		callKind = "Sync"
	}

	if on == "SKIP" || on == "" {
		return nil
	}

	var bufOn bytes.Buffer
	tOn, err := template.New("run").Parse(on)
	if err != nil {
		return fmt.Errorf("cannot compile template for 'on' command, error: %w", err)
	}
	err = tOn.Execute(&bufOn, event)
	if err != nil {
		return fmt.Errorf("there was an error while executing the template, error: %w", err)
	}

	call := command.New(bufOn.String())
	out, err := call.Output()
	if err != nil {
		if exiterr, ok := errors.AsType[*exec.ExitError](err); ok {
			slog.Error("stderror: " + string(exiterr.Stderr))
		}
		return fmt.Errorf("On%sMail command failed: %w", callKind, err)
	}
	slog.Info("stdout: " + string(out))

	if onpost == "SKIP" || onpost == "" {
		return nil
	}

	bufOnPost := bytes.NewBuffer(nil)
	tOnPost, err := template.New("run").Parse(onpost)
	if err != nil {
		return fmt.Errorf("cannot compile template for 'onPost' command, error: %w", err)
	}
	err = tOnPost.Execute(bufOnPost, event)
	if err != nil {
		return fmt.Errorf("there was an error while executing the template, error: %w", err)
	}

	call = command.New(bufOnPost.String())
	out, err = call.Output()
	if err != nil {
		if exiterr, ok := errors.AsType[*exec.ExitError](err); ok {
			slog.Error("stderror: " + string(exiterr.Stderr))
		}
		return fmt.Errorf("On%sMailPost command failed: %w", callKind, err)
	}
	slog.Info("stdout: " + string(out))
	return nil
}

// ShouldSkip checks if the event should be skipped based on the Box configuration
func ShouldSkip(b *config.Box) bool {
	switch b.Reason {
	case config.NEWMAIL:
		return b.OnNewMail == "" || b.OnNewMail == "SKIP"
	case config.FLAGCHANGED:
		return b.OnChangedMail == "" || b.OnChangedMail == "SKIP"
	case config.DELETEDMAIL:
		return b.OnDeletedMail == "" || b.OnDeletedMail == "SKIP"
	}

	return false
}
