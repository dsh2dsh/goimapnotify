package runner

import (
	"bytes"
	"context"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/esiqveland/notify"

	"github.com/dsh2dsh/goimapnotify/internal/box"
	"github.com/dsh2dsh/goimapnotify/internal/command"
	"github.com/dsh2dsh/goimapnotify/internal/config"
)

type handler struct {
	box     *box.Box
	wait    time.Duration
	maxWait time.Duration

	events box.EventSet
	t      *time.Timer

	mu      sync.Mutex
	delayed time.Duration
	nextRun time.Time

	notifier      *notifier
	notifyActions []notify.Action
	boxActions    map[string]*config.NotificationAction
}

func NewHandler(b *box.Box, wait time.Duration) *handler {
	return &handler{box: b, wait: wait}
}

func (self *handler) WithMaxDelay(v time.Duration) *handler {
	self.maxWait = v
	return self
}

func (self *handler) WithNotifier(n *notifier) *handler {
	self.notifier = n

	boxActions := self.box.NotificationActions
	if len(boxActions) == 0 {
		return self
	}

	self.boxActions = make(map[string]*config.NotificationAction, len(boxActions))
	self.notifyActions = make([]notify.Action, len(boxActions))
	for i, act := range boxActions {
		self.boxActions[act.Key] = act
		self.notifyActions[i] = notify.Action{Key: act.Key, Label: act.Label}
	}
	return self
}

func (self *handler) Schedule(e *box.IDLE) {
	self.events.Add(e)
	d, ok := self.reschedule()

	l := slog.With(
		slog.String("reason", e.Reason().String()),
		slog.String("alias", e.Alias()),
		slog.String("mailbox", e.Mailbox()),
		slog.Duration("wait", self.wait),
		slog.Duration("delayed", d))

	if !ok {
		l.Info("keep scheduled syncing", slog.Duration("maxWait", self.maxWait))
		return
	}

	l = l.With(slog.Time("when", time.Now().Add(self.wait)))
	switch {
	case self.t == nil:
		self.t = time.NewTimer(self.wait)
		fallthrough
	case !self.t.Reset(self.wait):
		l.Info("scheduled syncing")
	default:
		l.Info("rescheduled syncing")
	}
}

func (self *handler) reschedule() (time.Duration, bool) {
	self.mu.Lock()
	defer self.mu.Unlock()

	if self.maxWait > self.wait && self.delayed >= self.maxWait {
		return self.delayed, false
	}

	self.delayed += self.wait
	self.nextRun = time.Now().Add(self.wait)
	return self.delayed, true
}

func (self *handler) Run(ctx context.Context) {
	for {
		select {
		case <-self.t.C:
			if err := self.processEvents(ctx); err != nil {
				slog.Error("an error was encountered while executing commands for",
					slog.String("alias", self.box.Alias()),
					slog.String("box", self.box.Mailbox),
					slog.Any("error", err))
			}
		case <-ctx.Done():
			return
		}
	}
}

func (self *handler) processEvents(ctx context.Context) error {
	self.reset()
	defer self.completed()

	l := slog.With(
		slog.String("alias", self.box.Alias()),
		slog.String("mailbox", self.box.Mailbox))
	l.Debug("Running synchronization...")

	for s, err := range self.events.Commands(l) {
		if err != nil {
			return err
		}

		cmd := command.NewContext(ctx, s)
		output, err := execCommand(cmd, s)
		if err != nil {
			return err
		}
		self.notify(output)
	}
	return nil
}

func (self *handler) notify(output []byte) {
	if len(output) == 0 {
		return
	}

	if self.notifier == nil {
		printCommandOutput(slog.LevelInfo, "stdout: ", output)
		return
	}

	n := notify.Notification{Actions: self.notifyActions}

	before, after, found := bytes.Cut(output, []byte("\n"))
	if found {
		switch body := string(bytes.TrimSpace(after)); body {
		case "":
			n.Body = string(bytes.TrimSpace(before))
		default:
			n.Summary = string(bytes.TrimSpace(before))
			n.Body = body
		}
	} else {
		n.Body = string(bytes.TrimSpace(before))
	}

	l := slog.With(
		slog.String("alias", self.box.Alias()),
		slog.String("mailbox", self.box.Mailbox))
	l.Info("send desktop notification")

	if err := self.notifier.Send(n, self, l); err != nil {
		printCommandOutput(slog.LevelInfo, "stdout: ", output)
		l.Error("unable send desktop notification", slog.Any("error", err))
	}
}

func (self *handler) reset() {
	self.mu.Lock()
	self.delayed = 0
	self.nextRun = time.Now()
	self.mu.Unlock()
}

func (self *handler) completed() {
	self.mu.Lock()
	defer self.mu.Unlock()

	if self.delayed == 0 {
		return
	}

	d := time.Until(self.nextRun)
	if d >= self.wait {
		self.delayed = d
		return
	}

	self.delayed = self.wait
	self.nextRun = time.Now().Add(self.wait)
	self.t.Reset(self.wait)

	slog.Info("rescheduled next syncing after completed",
		slog.String("alias", self.box.Alias()),
		slog.String("mailbox", self.box.Mailbox),
		slog.Duration("was", d),
		slog.Duration("wait", self.wait),
		slog.Time("when", self.nextRun))
}

func (self *handler) OnAction(ctx context.Context, actionKey string,
	timeout time.Duration,
) {
	act := self.boxActions[actionKey]
	if act == nil {
		slog.Warn("desktop notification action not configured",
			slog.String("actionKey", actionKey),
			slog.String("alias", self.box.Alias()),
			slog.String("mailbox", self.box.Mailbox))
		return
	}

	slog.Info("run desktop notification action",
		slog.String("actionKey", actionKey),
		slog.String("alias", self.box.Alias()),
		slog.String("mailbox", self.box.Mailbox),
		slog.Duration("timeout", timeout))

	if timeout > 0 {
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		ctx = timeoutCtx
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, act.Exec[0], act.Exec[1:]...)
	output, err := execCommand(cmd, strings.Join(act.Exec, " "))
	if err != nil {
		slog.Error("desktop notification action failed", slog.Any("error", err))
		return
	}
	printCommandOutput(slog.LevelInfo, "stdout: ", output)
}

func (self *handler) ActionConfig(key string) *config.NotificationAction {
	return self.boxActions[key]
}

func (self *handler) Alias() string { return self.box.Alias() }

func (self *handler) Mailbox() string { return self.box.Mailbox }
