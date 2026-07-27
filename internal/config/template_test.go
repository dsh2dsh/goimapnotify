package config

import "testing"

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
			name:     "valid template with ExistingEmail",
			template: "echo {{.ExistingEmail}}",
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
			err := compileTemplate(tt.template)
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
		"{{.ExistingEmail}}",
		"{{.Box}}",
		"{{.Box.Mailbox}}",
		"{{.Box.OnNewMail}}",
		"{{.Box.Alias}}",
	}

	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			err := compileTemplate(tmpl)
			if err != nil {
				t.Errorf("CompileTemplate(%q) error = %v", tmpl, err)
			}
		})
	}
}

// TestSetFromConfig tests the SetFromConfig function
func TestSetFromConfig(t *testing.T) {
	tests := []struct {
		name    string
		conf    NotifyConfig
		box     Box
		wantBox Box
		wantErr bool
	}{
		{
			name: "inherits all from config when box is empty",
			conf: NotifyConfig{
				Alias:             "test@example.com",
				OnNewMail:         "echo new",
				OnNewMailPost:     "echo new post",
				OnDeletedMail:     "echo deleted",
				OnDeletedMailPost: "echo deleted post",
			},
			box: Box{
				Mailbox: "INBOX",
			},
			wantBox: Box{
				Mailbox:           "INBOX",
				Alias:             "test@example.com",
				OnNewMail:         "echo new",
				OnNewMailPost:     "echo new post",
				OnDeletedMail:     "echo deleted",
				OnDeletedMailPost: "echo deleted post",
			},
			wantErr: false,
		},
		{
			name: "box values override config values",
			conf: NotifyConfig{
				Alias:             "test@example.com",
				OnNewMail:         "echo config new",
				OnNewMailPost:     "echo config new post",
				OnDeletedMail:     "echo config deleted",
				OnDeletedMailPost: "echo config deleted post",
			},
			box: Box{
				Mailbox:           "INBOX",
				OnNewMail:         "echo box new",
				OnNewMailPost:     "echo box new post",
				OnDeletedMail:     "echo box deleted",
				OnDeletedMailPost: "echo box deleted post",
			},
			wantBox: Box{
				Mailbox:           "INBOX",
				Alias:             "test@example.com",
				OnNewMail:         "echo box new",
				OnNewMailPost:     "echo box new post",
				OnDeletedMail:     "echo box deleted",
				OnDeletedMailPost: "echo box deleted post",
			},
			wantErr: false,
		},
		{
			name: "partial override - only OnNewMail",
			conf: NotifyConfig{
				Alias:             "test@example.com",
				OnNewMail:         "echo config new",
				OnNewMailPost:     "echo config new post",
				OnDeletedMail:     "echo config deleted",
				OnDeletedMailPost: "echo config deleted post",
			},
			box: Box{
				Mailbox:   "INBOX",
				OnNewMail: "echo box new",
			},
			wantBox: Box{
				Mailbox:           "INBOX",
				Alias:             "test@example.com",
				OnNewMail:         "echo box new",
				OnNewMailPost:     "echo config new post",
				OnDeletedMail:     "echo config deleted",
				OnDeletedMailPost: "echo config deleted post",
			},
			wantErr: false,
		},
		{
			name: "invalid OnNewMail template",
			conf: NotifyConfig{
				Alias:     "test@example.com",
				OnNewMail: "echo {{.Invalid",
			},
			box: Box{
				Mailbox: "INBOX",
			},
			wantErr: true,
		},
		{
			name: "invalid OnNewMailPost template",
			conf: NotifyConfig{
				Alias:         "test@example.com",
				OnNewMail:     "echo valid",
				OnNewMailPost: "echo {{.Invalid",
			},
			box: Box{
				Mailbox: "INBOX",
			},
			wantErr: true,
		},
		{
			name: "invalid OnDeletedMail template",
			conf: NotifyConfig{
				Alias:         "test@example.com",
				OnNewMail:     "echo valid",
				OnNewMailPost: "echo valid",
				OnDeletedMail: "echo {{.Invalid",
			},
			box: Box{
				Mailbox: "INBOX",
			},
			wantErr: true,
		},
		{
			name: "invalid OnDeletedMailPost template",
			conf: NotifyConfig{
				Alias:             "test@example.com",
				OnNewMail:         "echo valid",
				OnNewMailPost:     "echo valid",
				OnDeletedMail:     "echo valid",
				OnDeletedMailPost: "echo {{.Invalid",
			},
			box: Box{
				Mailbox: "INBOX",
			},
			wantErr: true,
		},
		{
			name: "invalid box OnNewMail template",
			conf: NotifyConfig{
				Alias: "test@example.com",
			},
			box: Box{
				Mailbox:   "INBOX",
				OnNewMail: "echo {{.Invalid",
			},
			wantErr: true,
		},
		{
			name: "valid templates with Go template syntax",
			conf: NotifyConfig{
				Alias:             "test@example.com",
				OnNewMail:         "echo {{.Mailbox}}",
				OnNewMailPost:     "echo {{.Alias}}",
				OnDeletedMail:     "echo {{.Reason}}",
				OnDeletedMailPost: "echo {{.ExistingEmail}}",
			},
			box: Box{
				Mailbox: "INBOX",
			},
			wantBox: Box{
				Mailbox:           "INBOX",
				Alias:             "test@example.com",
				OnNewMail:         "echo {{.Mailbox}}",
				OnNewMailPost:     "echo {{.Alias}}",
				OnDeletedMail:     "echo {{.Reason}}",
				OnDeletedMailPost: "echo {{.ExistingEmail}}",
			},
			wantErr: false,
		},
		{
			name: "empty config and box",
			conf: NotifyConfig{
				Alias: "test@example.com",
			},
			box: Box{
				Mailbox: "INBOX",
			},
			wantBox: Box{
				Mailbox: "INBOX",
				Alias:   "test@example.com",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.box
			err := tt.conf.FillBox(&result)

			if (err != nil) != tt.wantErr {
				t.Errorf("SetFromConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if result.Mailbox != tt.wantBox.Mailbox {
				t.Errorf("Mailbox = %q, want %q", result.Mailbox, tt.wantBox.Mailbox)
			}
			if result.Alias != tt.wantBox.Alias {
				t.Errorf("Alias = %q, want %q", result.Alias, tt.wantBox.Alias)
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
