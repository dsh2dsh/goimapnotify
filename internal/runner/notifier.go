package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/esiqveland/notify"
	"github.com/godbus/dbus/v5"

	"github.com/dsh2dsh/goimapnotify/internal/config"
)

type notifier struct {
	ctx context.Context

	dbus         *dbus.Conn
	notifier     notify.Notifier
	notification notify.Notification

	mu       sync.Mutex
	handlers map[uint32]*handler
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
	slog.Info("got desktop notification action",
		slog.Uint64("id", uint64(sig.ID)),
		slog.String("actionKey", sig.ActionKey))

	h := self.actionHandler(sig.ID)
	if h == nil {
		slog.Warn("unknown desktop notification action",
			slog.Uint64("id", uint64(sig.ID)),
			slog.String("actionKey", sig.ActionKey))
		return
	}
	h.OnAction(self.ctx, sig.ActionKey)
}

func (self *notifier) actionHandler(id uint32) *handler {
	self.mu.Lock()
	h, ok := self.handlers[id]
	if ok {
		delete(self.handlers, id)
	}
	self.mu.Unlock()
	return h
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

func (self *notifier) Send(n notify.Notification, h *handler) error {
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
		return nil
	}

	self.mu.Lock()
	self.handlers[id] = h
	self.mu.Unlock()
	return nil
}
