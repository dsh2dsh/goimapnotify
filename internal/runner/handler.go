package runner

import (
	"log/slog"
	"sync"
	"time"

	"github.com/dsh2dsh/goimapnotify/internal/box"
	"github.com/dsh2dsh/goimapnotify/internal/command"
)

type handler struct {
	box     *box.Box
	wait    time.Duration
	maxWait time.Duration
	t       *time.Timer

	events box.EventSet

	mu      sync.Mutex
	delayed time.Duration
	nextRun time.Time
}

func NewHandler(b *box.Box, wait time.Duration) *handler {
	return &handler{box: b, wait: wait}
}

func (self *handler) WithMaxDelay(v time.Duration) *handler {
	self.maxWait = v
	return self
}

func (self *handler) Schedule(e *box.IDLE) {
	self.events.Add(e)
	d, ok := self.reschedule()

	l := slog.With(
		slog.String("reason", e.Reason.String()),
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

func (self *handler) Run(done <-chan struct{}) {
	for {
		select {
		case <-self.t.C:
			if err := self.processEvents(); err != nil {
				slog.Error("an error was encountered while executing commands for",
					slog.String("alias", self.box.Alias()),
					slog.String("box", self.box.Mailbox),
					slog.Any("error", err))
			}
		case <-done:
			return
		}
	}
}

func (self *handler) processEvents() error {
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

		cmd := command.New(s)
		if err := execCommand(cmd, s); err != nil {
			return err
		}
	}
	return nil
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
