package runner

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"

	"github.com/dsh2dsh/goimapnotify/internal/config"
)

type notificationTemplate struct {
	summary *template.Template
	body    *template.Template
}

func NewNotificationTemplate(cfg config.NotificationTemplate,
) (*notificationTemplate, error) {
	t := new(notificationTemplate)
	if err := t.parse(cfg); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return t, nil
}

func (self *notificationTemplate) parse(cfg config.NotificationTemplate) error {
	if s := strings.TrimSpace(cfg.Summary); s != "" {
		t, err := template.New("").Parse(s)
		if err != nil {
			return fmt.Errorf("summary template: %w", err)
		}
		self.summary = t
	}

	if s := strings.TrimSpace(cfg.Body); s != "" {
		t, err := template.New("").Parse(s)
		if err != nil {
			return fmt.Errorf("body template: %w", err)
		}
		self.body = t
	}
	return nil
}

func (self *notificationTemplate) Execute(data any) (summary, body string,
	_ error,
) {
	var b bytes.Buffer
	if t := self.summary; t != nil {
		if err := t.Execute(&b, data); err != nil {
			return "", "", fmt.Errorf("summary template: %w", err)
		}
		summary = b.String()
	}

	if t := self.body; t != nil {
		b.Reset()
		if err := t.Execute(&b, data); err != nil {
			return "", "", fmt.Errorf("body template: %w", err)
		}
		body = b.String()
	}
	return summary, body, nil
}
