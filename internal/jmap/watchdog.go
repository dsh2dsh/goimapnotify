package jmap

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

func (self *WatchMailboxes) startWatchdog(ctx context.Context,
) (deferFunc func()) {
	d := self.watchdogInterval()
	l := self.logger()
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
		self.logger().Info("ping watchdog stopped", slog.Any("reason", ctx.Err()))
	case <-self.watchdogTicker.C:
		self.stopWatchdog()
		self.logger().Info("ping watchdog timed out, reconnect to event source")
	}
}
