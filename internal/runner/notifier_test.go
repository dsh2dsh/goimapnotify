package runner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dsh2dsh/goimapnotify/internal/config"
	"github.com/dsh2dsh/goimapnotify/internal/model"
)

func Test_notifier_renderNewMail(t *testing.T) {
	const subject = "Lorem ipsum dolor sit amet consectetur adipiscing elit."

	tests := []struct {
		name        string
		summary     string
		body        string
		wantSummary string
		wantBody    string
	}{
		{
			name:        "summary and body",
			summary:     "{{ .Mailbox }} ({{ .Count }})",
			body:        "<i>{{ .Authors }}</i>\n{{ .Subject }}",
			wantSummary: "Inbox (2)",
			wantBody:    "<i>John Doe, Jane Doe</i>\n" + subject,
		},
		{
			name:        "without body",
			summary:     "{{ .Mailbox }} ({{ .Count }})",
			wantSummary: "Inbox (2)",
		},
		{
			name:     "without summary",
			body:     "<i>{{ .Authors }}</i>\n{{ .Subject }}",
			wantBody: "<i>John Doe, Jane Doe</i>\n" + subject,
		},
		{name: "without summary and body"},
	}

	b := model.Box{
		Box: &config.Box{Mailbox: "Inbox"},
	}

	thread := model.Thread{
		From:    []string{"John Doe", "Jane Doe"},
		Subject: subject,
		Count:   2,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DesktopNotification{
				NewMail: config.NotificationTemplate{
					Summary: tt.summary,
					Body:    tt.body,
				},
			}

			var n notifier
			require.NoError(t, n.compileTemplates(cfg))

			summary, body, err := n.renderNewMail(&b, thread)
			require.NoError(t, err)
			assert.Equal(t, tt.wantSummary, summary)
			assert.Equal(t, tt.wantBody, body)
		})
	}
}
