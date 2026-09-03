package model

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dsh2dsh/goimapnotify/internal/config"
)

func TestCompileBoxes_commands(t *testing.T) {
	baseConf := `
configurations:
  - host: "localhost"
    port: 993
    username: "username@localhost"
    alias: "username@localhost"`

	tests := []struct {
		name              string
		conf              string
		onNewMail         string
		onNewMailPost     string
		onChangedMail     string
		onChangedMailPost string
		onDeletedMail     string
		onDeletedMailPost string
		wantErr           bool
	}{
		{
			name: "inherits all from config when box is empty",
			conf: `
    onNewMail: [ "echo", "new" ]
    onNewMailPost: [ "echo", "new", "post" ]
    onDeletedMail: [ "echo", "deleted" ]
    onDeletedMailPost: [ "echo", "deleted", "post" ]
    boxes:
      - mailbox: "INBOX"`,
			onNewMail:         "echo new",
			onNewMailPost:     "echo new post",
			onDeletedMail:     "echo deleted",
			onDeletedMailPost: "echo deleted post",
		},
		{
			name: "box values override config values",
			conf: `
    onNewMail: [ "echo", "config", "new" ]
    onNewMailPost: [ "echo", "config", "new", "post" ]
    onDeletedMail: [ "echo", "config", "deleted" ]
    onDeletedMailPost: [ "echo", "config", "deleted", "post" ]
    boxes:
      - mailbox: "INBOX"
        onNewMail: [ "echo", "box", "new" ]
        onNewMailPost: [ "echo", "box", "new", "post" ]
        onDeletedMail: [ "echo", "box", "deleted" ]
        onDeletedMailPost: [ "echo", "box", "deleted", "post" ]`,
			onNewMail:         "echo box new",
			onNewMailPost:     "echo box new post",
			onDeletedMail:     "echo box deleted",
			onDeletedMailPost: "echo box deleted post",
		},
		{
			name: "partial override - only OnNewMail",
			conf: `
    onNewMail: [ "echo", "config", "new" ]
    onNewMailPost: [ "echo", "config", "new", "post" ]
    onDeletedMail: [ "echo", "config", "deleted" ]
    onDeletedMailPost: [ "echo", "config", "deleted", "post" ]
    boxes:
      - mailbox: "INBOX"
        onNewMail: [ "echo", "box", "new" ]`,
			onNewMail:         "echo box new",
			onNewMailPost:     "echo config new post",
			onDeletedMail:     "echo config deleted",
			onDeletedMailPost: "echo config deleted post",
		},
		{
			name: "invalid OnNewMail template",
			conf: `
    onNewMail: [ "echo", "{{.Invalid" ]
    boxes:
      - mailbox: "INBOX"`,
			wantErr: true,
		},
		{
			name: "invalid OnNewMailPost template",
			conf: `
    onNewMail: [ "echo", "valid" ]
    onNewMailPost: [ "echo", "{{.Invalid" ]
    boxes:
      - mailbox: "INBOX"`,
			wantErr: true,
		},
		{
			name: "invalid OnDeletedMail template",
			conf: `
    onNewMail: [ "echo", "valid" ]
    onNewMailPost: [ "echo", "valid" ]
    onDeletedMail: [ "echo", "{{.Invalid" ]
    boxes:
      - mailbox: "INBOX"`,
			wantErr: true,
		},
		{
			name: "invalid OnDeletedMailPost template",
			conf: `
    onNewMail: [ "echo", "valid" ]
    onNewMailPost: [ "echo", "valid" ]
    onDeletedMail: [ "echo", "valid" ]
    onDeletedMailPost: [ "echo", "{{.Invalid" ]
    boxes:
      - mailbox: "INBOX"`,
			wantErr: true,
		},
		{
			name: "invalid box OnNewMail template",
			conf: `
    boxes:
      - mailbox: "INBOX"
        onNewMail: [ "echo", "{{.Invalid" ]`,
			wantErr: true,
		},
		{
			name: "valid templates with Go template syntax",
			conf: `
    onNewMail: [ "echo", "{{ .Mailbox }}" ]
    onNewMailPost: [ "echo", "{{ .Alias }}" ]
    onDeletedMail: [ "echo", "{{ .Reason }}" ]
    boxes:
      - mailbox: "INBOX"`,
			onNewMail:     "echo {{ .Mailbox }}",
			onNewMailPost: "echo {{ .Alias }}",
			onDeletedMail: "echo {{ .Reason }}",
		},
		{
			name: "empty config and box",
			conf: `
    boxes:
      - mailbox: "INBOX"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.LoadBytes([]byte(baseConf + tt.conf))
			require.NoError(t, err)
			require.NotNil(t, cfg)
			require.NotEmpty(t, cfg.Configurations)
			require.NotEmpty(t, cfg.Configurations[0].Boxes)

			boxes, err := CompileBoxes(cfg.Configurations[0],
				cfg.Configurations[0].Boxes)

			if tt.wantErr {
				require.Error(t, err)
				t.Log(err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, boxes)
			mbox := boxes[0]

			if tt.onNewMail != "" {
				require.NotNil(t, mbox.OnNewMail(), "OnNewMail()")
				assert.Equal(t, tt.onNewMail, mbox.OnNewMail().String(), "onNewMail")
			}
			if tt.onNewMailPost != "" {
				require.NotNil(t, mbox.OnNewMailPost(), "OnNewMailPost()")
				assert.Equal(t, tt.onNewMailPost, mbox.OnNewMailPost().String(),
					"onNewMailPost")
			}

			if tt.onChangedMail != "" {
				require.NotNil(t, mbox.OnChangedMail(), "OnChangedMail()")
				assert.Equal(t, tt.onChangedMail, mbox.OnChangedMail().String(),
					"onChangedMail")
			}
			if tt.onChangedMailPost != "" {
				require.NotNil(t, mbox.OnChangedMailPost(), "OnChangedMailPost()")
				assert.Equal(t, tt.onChangedMailPost,
					mbox.OnChangedMailPost().String(), "onChangedPostMail")
			}

			if tt.onDeletedMail != "" {
				require.NotNil(t, mbox.OnDeletedMail(), "OnDeletedMail()")
				assert.Equal(t, tt.onDeletedMail, mbox.OnDeletedMail().String(),
					"onDeletedMail")
			}
			if tt.onDeletedMailPost != "" {
				require.NotNil(t, mbox.OnDeletedMailPost(), "OnDeletedMailPost()")
				assert.Equal(t, tt.onDeletedMailPost,
					mbox.OnDeletedMailPost().String(), "onDeletedMailPost")
			}
		})
	}
}

func TestCompileBoxes_templates(t *testing.T) {
	confFmt := `
configurations:
  - host: "localhost"
    port: 993
    username: "username@localhost"
    alias: "username@localhost"
    onNewMail: %s
    boxes:
      - mailbox: "INBOX"`

	tests := []struct {
		name      string
		onNewMail string
		wantErr   bool
	}{
		{
			name:      "empty templates",
			onNewMail: "",
		},
		{
			name:      "plain string",
			onNewMail: `[ "echo", "hello" ]`,
		},
		{
			name:      "valid template with Reason",
			onNewMail: `[ "echo", "{{ .Reason }}" ]`,
		},
		{
			name:      "valid template with Box",
			onNewMail: `[ "echo", "{{ .Box.Mailbox }}"]`,
		},
		{
			name:      "invalid template syntax - unclosed brace",
			onNewMail: `[ "echo", "{{ .Mailbox" ]`,
			wantErr:   true,
		},
		{
			name:      "invalid template syntax - unclosed action",
			onNewMail: `[ "echo", "{{" ]`,
			wantErr:   true,
		},
		{
			name:      "template with conditional",
			onNewMail: `[ "echo", "{{if .Alias}}has alias{{end}}" ]`,
		},
		{
			name:      "template with range (invalid - string not iterable)",
			onNewMail: `[ "echo", "{{ range .Box.Mailbox }}{{ . }}{{ end }}" ]`,
			wantErr:   true, // Can't range over a string in this context
		},
		{
			name:      "complex template",
			onNewMail: `[ "mbsync", "-V", "{{ .Alias }}:{{ .Mailbox }}" ]`,
		},
		{
			name:      "with Alias",
			onNewMail: `[ "echo", "{{ .Alias }}" ]`,
		},
		{
			name:      "with Mailbox",
			onNewMail: `[ "echo", "{{ .Mailbox }}" ]`,
		},
		{
			name:      "with Reason",
			onNewMail: `[ "echo", "{{ .Reason }}" ]`,
		},
		{
			name:      "with Box",
			onNewMail: `[ "echo", "{{ .Box }}" ]`,
		},
		{
			name:      "with Box.Mailbox",
			onNewMail: `[ "echo", "{{ .Box.Mailbox }}" ]`,
		},
		{
			name:      "with Box.OnNewMail",
			onNewMail: `[ "echo", "{{ .Box.OnNewMail }}" ]`,
		},
		{
			name:      "with Box.Alias",
			onNewMail: `[ "echo", "{{ .Box.Alias }}" ]`,
		},
	}

	var b bytes.Buffer
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() { b.Reset() })

			_, err := fmt.Fprintf(&b, confFmt, tt.onNewMail)
			require.NoError(t, err)

			cfg, err := config.LoadBytes(b.Bytes())
			require.NoError(t, err)
			require.NotNil(t, cfg)
			require.NotEmpty(t, cfg.Configurations)
			require.NotEmpty(t, cfg.Configurations[0].Boxes)

			boxes, err := CompileBoxes(cfg.Configurations[0],
				cfg.Configurations[0].Boxes)

			if tt.wantErr {
				require.Error(t, err)
				t.Log(err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, boxes)
		})
	}
}
