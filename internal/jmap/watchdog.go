package jmap

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/dsh2dsh/goimapnotify/internal/logging"
)

func (self *WatchMailboxes) startWatchdog(ctx context.Context,
) (deferFunc func()) {
	d := self.watchdogInterval()
	l := logging.FromContext(ctx)
	l.Info("start ping watchdog", slog.Duration("timeout", d))

	self.watchdogTicker = time.NewTicker(d)
	wg := new(sync.WaitGroup)
	wg.Go(func() { self.watchdog(ctx) })

	return func() {
		self.stopWatchdog()
		l.Info("waiting watchdog goroutine to stop...")
		wg.Wait()
	}
}

func (self *WatchMailboxes) watchdogInterval() time.Duration {
	return time.Duration(self.pingInterval)*time.Second + time.Minute
}

func (self *WatchMailboxes) watchdog(ctx context.Context) {
	select {
	case <-ctx.Done():
		self.watchdogTicker.Stop()
		logging.FromContext(ctx).Info("ping watchdog stopped", slog.Any("reason", ctx.Err()))
	case <-self.watchdogTicker.C:
		self.stopWatchdog()
		logging.FromContext(ctx).Info("ping watchdog timed out, reconnect to event source")
	}
}
