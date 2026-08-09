package runner

import (
	"context"
	"fmt"

	"github.com/esiqveland/notify"
	"github.com/godbus/dbus/v5"

	"github.com/dsh2dsh/goimapnotify/internal/config"
)

func (self *Runner) EnableDesktopNotifications(ctx context.Context,
	cfg config.DesktopNotification,
) error {
	conn, err := DBusConnect(ctx)
	if err != nil {
		return err
	}
	self.dbus = conn

	n, err := notify.New(self.dbus)
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
