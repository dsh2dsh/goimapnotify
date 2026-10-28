package runner

import (
	"context"
	"fmt"

	"github.com/esiqveland/notify"

	"github.com/dsh2dsh/goimapnotify/internal/config"
	"github.com/dsh2dsh/goimapnotify/internal/logging"
	"github.com/dsh2dsh/goimapnotify/internal/model"
)

type Notification struct {
	notifier     *notifier
	handler      *handler
	notification notify.Notification
}

func NewUnreadEmails(h *handler, unreadEmails uint64) (*Notification, error) {
	n, err := h.Notifier().renderUnreadEmails(h.Box(), unreadEmails)
	if err != nil {
		return nil, err
	}

	n.Actions = h.Actions()
	return &Notification{handler: h, notification: n}, nil
}

func NewNewMail(h *handler, thread model.Thread) (*Notification, error) {
	n, err := h.Notifier().renderNewMail(h.Box(), thread)
	if err != nil {
		return nil, err
	}

	n.Actions = h.Actions()
	return &Notification{handler: h, notification: n}, nil
}

func NewErrorNotification(n *notifier, accountConfig *config.NotifyConfig,
	summary, body string,
) (*Notification, error) {
	notification, err := n.renderErrorState(accountConfig, summary, body)
	if err != nil {
		return nil, err
	}
	return &Notification{notifier: n, notification: notification}, nil
}

func (self *Notification) Send(ctx context.Context) (s SentNotification,
	_ error,
) {
	n := self.notification
	if n.Summary == "" && n.Body == "" {
		return s, nil
	}

	l := logging.FromContext(ctx)
	l.Debug("send desktop notification")

	id, err := self.handlerNotifier().Send(n, self.handler, l)
	if err != nil {
		return s, fmt.Errorf("send desktop notification: %w", err)
	}
	return SentNotification{id, self.handler}, nil
}

func (self *Notification) handlerNotifier() *notifier {
	if self.handler != nil {
		return self.handler.Notifier()
	}
	return self.notifier
}

type SentNotification struct {
	id      uint32
	handler *handler
}

func (self *SentNotification) Close() error {
	return self.handler.Notifier().CloseNotification(self.id)
}
