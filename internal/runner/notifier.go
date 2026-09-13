package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/esiqveland/notify"
	"github.com/godbus/dbus/v5"

	"github.com/dsh2dsh/goimapnotify/internal/config"
	"github.com/dsh2dsh/goimapnotify/internal/logging"
	"github.com/dsh2dsh/goimapnotify/internal/model"
)

type notifier struct {
	ctx context.Context

	unread  *notificationTemplate
	newMail *notificationTemplate

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

	if err := self.compileTemplates(cfg); err != nil {
		return fmt.Errorf("compile new mail notification: %w", err)
	}

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

func (self *notifier) compileTemplates(cfg config.DesktopNotification) error {
	templates := []struct {
		name   string
		cfg    config.NotificationTemplate
		parsed **notificationTemplate
	}{
		{"newMail", cfg.NewMail, &self.newMail},
		{"unreadEmails", cfg.UnreadEmails, &self.unread},
	}

	for _, tt := range templates {
		t, err := NewNotificationTemplate(tt.cfg)
		if err != nil {
			return fmt.Errorf("parse %s notification template: %w", tt.name, err)
		}
		*tt.parsed = t
	}

	b := model.Box{Box: &config.Box{}}
	b.WithAccount(&config.NotifyConfig{})

	if _, _, err := self.renderNewMail(&b, model.Thread{}); err != nil {
		return err
	}

	if _, _, err := self.renderUnreadEmails(&b, 0); err != nil {
		return err
	}
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

	h, expired := self.actionHandler(sig)
	if h == nil {
		slog.Warn("unknown desktop notification action")
		return
	}

	self.running.Add(1)
	self.wg.Go(func() {
		h.OnAction(self.ctx, sig.ActionKey, self.actionTimeout)
		self.running.Add(-1)
	})

	switch len(expired) {
	case 0:
		return
	case 1:
		self.closeNotifications(expired)
		return
	}
	self.wg.Go(func() { self.closeNotifications(expired) })
}

func (self *notifier) actionHandler(sig *notify.ActionInvokedSignal,
) (*handler, []uint32) {
	self.mu.Lock()
	defer self.mu.Unlock()

	h, ok := self.handlers[sig.ID]
	if !ok {
		return nil, nil
	}
	return h, self.expireNotifications(h, sig)
}

func (self *notifier) expireNotifications(h *handler,
	sig *notify.ActionInvokedSignal,
) (expired []uint32) {
	c := h.ActionConfig(sig.ActionKey)
	if c.CloseAll {
		expired := slices.Collect(maps.Keys(self.handlers))
		clear(self.handlers)
		return expired
	}

	if c.CloseSame {
		for id, sender := range self.handlers {
			if sender == h {
				expired = append(expired, id)
				delete(self.handlers, id)
			}
		}
		return expired
	}

	if c.Close {
		delete(self.handlers, sig.ID)
		return []uint32{sig.ID}
	}
	return nil
}

func (self *notifier) closeNotifications(expired []uint32) {
	for i, id := range expired {
		if self.ctx.Err() != nil {
			slog.Error("stop closing desktop notifications",
				slog.Int("expired", len(expired)),
				slog.Int("index", i),
				slog.Uint64("id", uint64(id)),
				slog.Any("reason", self.ctx.Err()))
			return
		}

		_, err := self.notifier.CloseNotification(id)
		if err != nil {
			slog.Error("unable close desktop notification",
				slog.Int("expired", len(expired)),
				slog.Int("index", i),
				slog.Uint64("id", uint64(id)),
				slog.Any("error", err))
			return
		}
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

func (self *notifier) NotifyNewMail(ctx context.Context, b *model.Box,
	h *handler, thread model.Thread,
) error {
	summary, body, err := self.renderNewMail(b, thread)
	if err != nil {
		return err
	}
	return self.Notify(ctx, h, summary, body)
}

func (self *notifier) renderNewMail(b *model.Box, thread model.Thread) (summary,
	body string, err error,
) {
	data := struct {
		Alias   string
		Mailbox string
		Count   int
		Authors string
		Subject string
	}{
		Alias:   b.Alias(),
		Mailbox: b.Mailbox,
		Count:   thread.Count,
		Authors: strings.Join(thread.From, ", "),
		Subject: thread.Subject,
	}

	summary, body, err = self.newMail.Execute(&data)
	if err != nil {
		return "", "", fmt.Errorf("execute newMail template: %w", err)
	}
	return summary, body, nil
}

func (self *notifier) Notify(ctx context.Context, h *handler, summary,
	body string,
) error {
	if summary == "" && body == "" {
		return nil
	}

	n := notify.Notification{Summary: summary, Body: body}
	if h != nil {
		n.Actions = h.Actions()
	}

	l := logging.FromContext(ctx)
	l.Debug("send desktop notification")

	if err := self.Send(n, h, l); err != nil {
		return fmt.Errorf("send desktop notification: %w", err)
	}
	return nil
}

func (self *notifier) renderUnreadEmails(b *model.Box, unreadEmails uint64,
) (summary, body string, err error) {
	data := struct {
		Alias        string
		Mailbox      string
		UnreadEmails uint64
	}{
		Alias:        b.Alias(),
		Mailbox:      b.Mailbox,
		UnreadEmails: unreadEmails,
	}

	summary, body, err = self.unread.Execute(&data)
	if err != nil {
		return "", "", fmt.Errorf("execute unreadEmails template: %w", err)
	}
	return summary, body, nil
}

func (self *notifier) NotifyUnreadEmails(ctx context.Context, b *model.Box,
	h *handler, unreadEmails uint64,
) error {
	summary, body, err := self.renderUnreadEmails(b, unreadEmails)
	if err != nil {
		return err
	}
	return self.Notify(ctx, h, summary, body)
}
