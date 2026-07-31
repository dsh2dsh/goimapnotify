package box

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dsh2dsh/goimapnotify/internal/config"
)

// TestCompileTemplate tests the CompileTemplate function
func TestCompileTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantErr  bool
	}{
		{
			name:     "empty template",
			template: "",
			wantErr:  false,
		},
		{
			name:     "plain string",
			template: "echo hello",
			wantErr:  false,
		},
		{
			name:     "valid template with Mailbox",
			template: "echo {{.Mailbox}}",
			wantErr:  false,
		},
		{
			name:     "valid template with Alias",
			template: "echo {{.Alias}}",
			wantErr:  false,
		},
		{
			name:     "valid template with multiple fields",
			template: "echo {{.Alias}} {{.Mailbox}}",
			wantErr:  false,
		},
		{
			name:     "valid template with Reason",
			template: "echo {{.Reason}}",
			wantErr:  false,
		},
		{
			name:     "valid template with Box",
			template: "echo {{.Box.Mailbox}}",
			wantErr:  false,
		},
		{
			name:     "invalid template syntax - unclosed brace",
			template: "echo {{.Mailbox",
			wantErr:  true,
		},
		{
			name:     "invalid template syntax - unclosed action",
			template: "echo {{",
			wantErr:  true,
		},
		{
			name:     "template with conditional",
			template: "{{if .Alias}}has alias{{end}}",
			wantErr:  false,
		},
		{
			name:     "template with range (invalid - string not iterable)",
			template: "{{range .Box.Mailbox}}{{.}}{{end}}",
			wantErr:  true, // Can't range over a string in this context
		},
		{
			name:     "complex template",
			template: "mbsync -V {{.Alias}}:{{.Mailbox}}",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			box := Box{Box: &config.Box{OnNewMail: tt.template}}
			err := box.compileTemplates(&box)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompileTemplate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestCompileTemplate_WithIDLEEventFields tests templates with all IDLEEvent fields
func TestCompileTemplate_WithIDLEEventFields(t *testing.T) {
	templates := []string{
		"{{.Alias}}",
		"{{.Mailbox}}",
		"{{.Reason}}",
		"{{.Box}}",
		"{{.Box.Mailbox}}",
		"{{.Box.OnNewMail}}",
		"{{.Box.Alias}}",
	}

	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			box := Box{Box: &config.Box{OnNewMail: tmpl}}
			err := box.compileTemplates(&box)
			if err != nil {
				t.Errorf("CompileTemplate(%q) error = %v", tmpl, err)
			}
		})
	}
}

// TestSetFromConfig tests the SetFromConfig function
func TestNewFromConfig(t *testing.T) {
	tests := []struct {
		name      string
		conf      config.NotifyConfig
		wantBox   Box
		wantAlias string
		wantErr   bool
	}{
		{
			name: "inherits all from config when box is empty",
			conf: config.NotifyConfig{
				Alias:             "test@example.com",
				OnNewMail:         "echo new",
				OnNewMailPost:     "echo new post",
				OnDeletedMail:     "echo deleted",
				OnDeletedMailPost: "echo deleted post",
				Boxes:             []*config.Box{{Mailbox: "INBOX"}},
			},
			wantBox: Box{
				Box: &config.Box{
					Mailbox:           "INBOX",
					OnNewMail:         "echo new",
					OnNewMailPost:     "echo new post",
					OnDeletedMail:     "echo deleted",
					OnDeletedMailPost: "echo deleted post",
				},
			},
			wantAlias: "test@example.com",
			wantErr:   false,
		},
		{
			name: "box values override config values",
			conf: config.NotifyConfig{
				Alias:             "test@example.com",
				OnNewMail:         "echo config new",
				OnNewMailPost:     "echo config new post",
				OnDeletedMail:     "echo config deleted",
				OnDeletedMailPost: "echo config deleted post",
				Boxes: []*config.Box{
					{
						Mailbox:           "INBOX",
						OnNewMail:         "echo box new",
						OnNewMailPost:     "echo box new post",
						OnDeletedMail:     "echo box deleted",
						OnDeletedMailPost: "echo box deleted post",
					},
				},
			},
			wantBox: Box{
				Box: &config.Box{
					Mailbox:           "INBOX",
					OnNewMail:         "echo box new",
					OnNewMailPost:     "echo box new post",
					OnDeletedMail:     "echo box deleted",
					OnDeletedMailPost: "echo box deleted post",
				},
			},
			wantAlias: "test@example.com",
			wantErr:   false,
		},
		{
			name: "partial override - only OnNewMail",
			conf: config.NotifyConfig{
				Alias:             "test@example.com",
				OnNewMail:         "echo config new",
				OnNewMailPost:     "echo config new post",
				OnDeletedMail:     "echo config deleted",
				OnDeletedMailPost: "echo config deleted post",
				Boxes: []*config.Box{{
					Mailbox:   "INBOX",
					OnNewMail: "echo box new",
				}},
			},
			wantBox: Box{
				Box: &config.Box{
					Mailbox:           "INBOX",
					OnNewMail:         "echo box new",
					OnNewMailPost:     "echo config new post",
					OnDeletedMail:     "echo config deleted",
					OnDeletedMailPost: "echo config deleted post",
				},
			},
			wantAlias: "test@example.com",
			wantErr:   false,
		},
		{
			name: "invalid OnNewMail template",
			conf: config.NotifyConfig{
				Alias:     "test@example.com",
				OnNewMail: "echo {{.Invalid",
				Boxes:     []*config.Box{{Mailbox: "INBOX"}},
			},
			wantErr: true,
		},
		{
			name: "invalid OnNewMailPost template",
			conf: config.NotifyConfig{
				Alias:         "test@example.com",
				OnNewMail:     "echo valid",
				OnNewMailPost: "echo {{.Invalid",
				Boxes:         []*config.Box{{Mailbox: "INBOX"}},
			},
			wantErr: true,
		},
		{
			name: "invalid OnDeletedMail template",
			conf: config.NotifyConfig{
				Alias:         "test@example.com",
				OnNewMail:     "echo valid",
				OnNewMailPost: "echo valid",
				OnDeletedMail: "echo {{.Invalid",
				Boxes:         []*config.Box{{Mailbox: "INBOX"}},
			},
			wantErr: true,
		},
		{
			name: "invalid OnDeletedMailPost template",
			conf: config.NotifyConfig{
				Alias:             "test@example.com",
				OnNewMail:         "echo valid",
				OnNewMailPost:     "echo valid",
				OnDeletedMail:     "echo valid",
				OnDeletedMailPost: "echo {{.Invalid",
				Boxes:             []*config.Box{{Mailbox: "INBOX"}},
			},
			wantErr: true,
		},
		{
			name: "invalid box OnNewMail template",
			conf: config.NotifyConfig{
				Alias: "test@example.com",
				Boxes: []*config.Box{
					{
						Mailbox:   "INBOX",
						OnNewMail: "echo {{.Invalid",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "valid templates with Go template syntax",
			conf: config.NotifyConfig{
				Alias:         "test@example.com",
				OnNewMail:     "echo {{.Mailbox}}",
				OnNewMailPost: "echo {{.Alias}}",
				OnDeletedMail: "echo {{.Reason}}",
				Boxes:         []*config.Box{{Mailbox: "INBOX"}},
			},
			wantBox: Box{
				Box: &config.Box{
					Mailbox:       "INBOX",
					OnNewMail:     "echo {{.Mailbox}}",
					OnNewMailPost: "echo {{.Alias}}",
					OnDeletedMail: "echo {{.Reason}}",
				},
			},
			wantAlias: "test@example.com",
			wantErr:   false,
		},
		{
			name: "empty config and box",
			conf: config.NotifyConfig{
				Alias: "test@example.com",
				Boxes: []*config.Box{{Mailbox: "INBOX"}},
			},
			wantBox:   Box{Box: &config.Box{Mailbox: "INBOX"}},
			wantAlias: "test@example.com",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boxes, err := NewFromConfig(&tt.conf)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NotEmpty(t, boxes)
			result := boxes[0]

			if result.Mailbox != tt.wantBox.Mailbox {
				t.Errorf("Mailbox = %q, want %q", result.Mailbox, tt.wantBox.Mailbox)
			}
			if result.Alias() != tt.wantAlias {
				t.Errorf("Alias = %q, want %q", result.Alias(), tt.wantAlias)
			}
			if result.OnNewMail != tt.wantBox.OnNewMail {
				t.Errorf("OnNewMail = %q, want %q", result.OnNewMail, tt.wantBox.OnNewMail)
			}
			if result.OnNewMailPost != tt.wantBox.OnNewMailPost {
				t.Errorf(
					"OnNewMailPost = %q, want %q",
					result.OnNewMailPost,
					tt.wantBox.OnNewMailPost,
				)
			}
			if result.OnDeletedMail != tt.wantBox.OnDeletedMail {
				t.Errorf(
					"OnDeletedMail = %q, want %q",
					result.OnDeletedMail,
					tt.wantBox.OnDeletedMail,
				)
			}
			if result.OnDeletedMailPost != tt.wantBox.OnDeletedMailPost {
				t.Errorf(
					"OnDeletedMailPost = %q, want %q",
					result.OnDeletedMailPost,
					tt.wantBox.OnDeletedMailPost,
				)
			}
		})
	}
}
