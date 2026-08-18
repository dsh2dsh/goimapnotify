package runner

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/esiqveland/notify"
	"github.com/godbus/dbus/v5"

	"github.com/dsh2dsh/goimapnotify/internal/config"
)

type notifier struct {
	ctx context.Context

	dbus          *dbus.Conn
	notifier      notify.Notifier
	notification  notify.Notification
	actionTimeout time.Duration

	mu       sync.Mutex
	handlers map[uint32]*handler

	wg      sync.WaitGroup
	running atomic.Int64
}

func NewNotifier(n int) *notifier {
	return &notifier{handlers: make(map[uint32]*handler, n)}
}

func (self *notifier) Connect(ctx context.Context,
	cfg config.DesktopNotification,
) error {
	self.ctx = ctx

	conn, err := DBusConnect(ctx)
	if err != nil {
		return err
	}
	self.dbus = conn

	n, err := notify.New(self.dbus,
		notify.WithOnAction(self.notificationAction),
		notify.WithOnClosed(self.notificationClosed))
	if err != nil {
		return fmt.Errorf("create desktop notifier: %w", err)
	}
	self.notifier = n

	self.notification = DesktopNotificationFrom(cfg)
	self.actionTimeout = cfg.ActionTimeout
	return nil
}

func DBusConnect(ctx context.Context) (conn *dbus.Conn,
	err error,
) {
	conn, err = dbus.SessionBusPrivate(dbus.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("dbus connect: %w", err)
	}

	defer func() {
		if err != nil {
			_ = conn.Close()
		}
	}()

	if err = conn.Auth(nil); err != nil {
		return nil, fmt.Errorf("dbus auth: %w", err)
	}

	if err = conn.Hello(); err != nil {
		return nil, fmt.Errorf("dbus hello: %w", err)
	}
	return conn, nil
}

func (self *notifier) notificationAction(sig *notify.ActionInvokedSignal) {
	l := slog.With(
		slog.Uint64("id", uint64(sig.ID)),
		slog.String("actionKey", sig.ActionKey))
	l.Info("got desktop notification action",
		slog.Int64("running", self.running.Load()))

	h := self.actionHandler(sig)
	if h == nil {
		slog.Warn("unknown desktop notification action")
		return
	}

	self.running.Add(1)
	self.wg.Go(func() {
		h.OnAction(self.ctx, sig.ActionKey, self.actionTimeout)
		self.running.Add(-1)
	})
}

func (self *notifier) actionHandler(sig *notify.ActionInvokedSignal) *handler {
	self.mu.Lock()
	defer self.mu.Unlock()

	h, ok := self.handlers[sig.ID]
	if !ok {
		return nil
	}
	self.closeNotifications(h, sig)
	return h
}

func (self *notifier) closeNotifications(h *handler,
	sig *notify.ActionInvokedSignal,
) {
	var idSeq iter.Seq[uint32]
	switch c := h.ActionConfig(sig.ActionKey); {
	case c.CloseAll:
		idSeq = maps.Keys(self.handlers)
	case c.CloseSame:
		idSeq = func(yield func(uint32) bool) {
			for id, h2 := range self.handlers {
				if h2 == h && !yield(id) {
					return
				}
			}
		}
	case c.Close:
		idSeq = func(yield func(uint32) bool) { yield(sig.ID) }
	default:
		delete(self.handlers, sig.ID)
		return
	}

	for id := range idSeq {
		_, err := self.notifier.CloseNotification(id)
		if err != nil {
			slog.Error("unable close desktop notification",
				slog.Uint64("id", uint64(id)), slog.Any("error", err))
			break
		}
		delete(self.handlers, id)
	}
}

func (self *notifier) notificationClosed(sig *notify.NotificationClosedSignal) {
	slog.Debug("desktop notification closed",
		slog.Uint64("id", uint64(sig.ID)),
		slog.String("reason", sig.Reason.String()))
	self.mu.Lock()
	delete(self.handlers, sig.ID)
	self.mu.Unlock()
}

func DesktopNotificationFrom(cfg config.DesktopNotification,
) notify.Notification {
	hints := make(map[string]dbus.Variant, 2)
	if cfg.Category != "" {
		hints["category"] = dbus.MakeVariant(cfg.Category)
	}
	if cfg.DesktopEntry != "" {
		hints["desktop-entry"] = dbus.MakeVariant(cfg.DesktopEntry)
	}

	return notify.Notification{
		AppName:       cfg.AppName,
		AppIcon:       cfg.AppIcon,
		Hints:         hints,
		ExpireTimeout: notify.ExpireTimeoutSetByNotificationServer,
	}
}

func (self *notifier) Close() {
	self.wg.Wait()

	if self.notifier != nil {
		err := self.notifier.Close()
		if err != nil && !errors.Is(err, dbus.ErrClosed) {
			slog.Error("failed close desktop notifier", slog.Any("error", err))
		}
	}

	if self.dbus != nil {
		if err := self.dbus.Close(); err != nil {
			slog.Error("failed close dbus", slog.Any("error", err))
		}
	}
}

func (self *notifier) Send(n notify.Notification, h *handler, l *slog.Logger,
) error {
	if self.notifier == nil {
		return errors.New("not connected to notifier")
	}

	merged := self.notification
	merged.Summary = n.Summary
	merged.Body = n.Body
	merged.Actions = n.Actions

	id, err := self.notifier.SendNotification(merged)
	if err != nil {
		return err //nolint:wrapcheck // caller handles it
	}

	if len(n.Actions) == 0 {
		l.Info("sent desktop notification without actions",
			slog.Uint64("id", uint64(id)))
		return nil
	}

	var watching int
	self.mu.Lock()
	self.handlers[id] = h
	watching = len(self.handlers)
	self.mu.Unlock()

	l.Info("sent desktop notification with actions",
		slog.Uint64("id", uint64(id)),
		slog.Int("watching", watching),
		slog.Int64("running", self.running.Load()))
	return nil
}
