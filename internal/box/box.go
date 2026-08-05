package box

import (
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/dsh2dsh/goimapnotify/internal/config"
)

type Box struct {
	*config.Box

	ExistingEmail uint32

	account *config.NotifyConfig

	templateNewMail         *template.Template
	templateNewMailPost     *template.Template
	templateChangedMail     *template.Template
	templateChangedMailPost *template.Template
	templateDeletedMail     *template.Template
	templateDeletedMailPost *template.Template
}

func NewFromConfig(cfg *config.NotifyConfig) ([]*Box, error) {
	return CompileBoxes(cfg, cfg.Boxes)
}

func CompileBoxes(accountConfig *config.NotifyConfig,
	configuredBoxes []*config.Box,
) ([]*Box, error) {
	accountBox := config.Box{
		OnNewMail:         accountConfig.OnNewMail,
		OnNewMailPost:     accountConfig.OnNewMailPost,
		OnChangedMail:     accountConfig.OnChangedMail,
		OnChangedMailPost: accountConfig.OnChangedMailPost,
		OnDeletedMail:     accountConfig.OnDeletedMail,
		OnDeletedMailPost: accountConfig.OnDeletedMailPost,
	}
	def := Box{Box: &accountBox}

	if err := def.compileTemplates(&def); err != nil {
		return nil, fmt.Errorf("parse commands: account=%s: %w",
			accountConfig.Alias, err)
	}

	boxes := make([]*Box, 0, len(configuredBoxes))
	for _, in := range configuredBoxes {
		box := (&Box{Box: in}).WithAccount(accountConfig)
		if err := box.compileTemplates(&def); err != nil {
			return nil, fmt.Errorf("parse commands: account=%s, mailbox=%s: %w",
				accountConfig.Alias, in.Mailbox, err)
		}
		boxes = append(boxes, box)
	}
	return boxes, nil
}

func (self *Box) compileTemplates(def *Box) error {
	commands := []struct {
		name            string
		value           *string
		template        *(*template.Template)
		defaultValue    string
		defaultTemplate *template.Template
	}{
		{
			name:            "OnNewMail",
			value:           &self.OnNewMail,
			template:        &self.templateNewMail,
			defaultValue:    def.OnNewMail,
			defaultTemplate: def.templateNewMail,
		},
		{
			name:            "OnNewMailPost",
			value:           &self.OnNewMailPost,
			template:        &self.templateNewMailPost,
			defaultValue:    def.OnNewMailPost,
			defaultTemplate: def.templateNewMailPost,
		},
		{
			name:            "OnChangedMail",
			value:           &self.OnChangedMail,
			template:        &self.templateChangedMail,
			defaultValue:    def.OnChangedMail,
			defaultTemplate: def.templateChangedMail,
		},
		{
			name:            "OnChangedMailPost",
			value:           &self.OnChangedMailPost,
			template:        &self.templateChangedMailPost,
			defaultValue:    def.OnChangedMailPost,
			defaultTemplate: def.templateChangedMailPost,
		},
		{
			name:            "OnDeletedMail",
			value:           &self.OnDeletedMail,
			template:        &self.templateDeletedMail,
			defaultValue:    def.OnDeletedMail,
			defaultTemplate: def.templateDeletedMail,
		},
		{
			name:            "OnDeletedMailPost",
			value:           &self.OnDeletedMailPost,
			template:        &self.templateDeletedMailPost,
			defaultValue:    def.OnDeletedMailPost,
			defaultTemplate: def.templateDeletedMailPost,
		},
	}

	for _, c := range commands {
		command := *c.value
		if command == "" {
			*c.value, *c.template = c.defaultValue, c.defaultTemplate
			continue
		} else if command == "SKIP" {
			continue
		}

		t, err := compileTemplate(replaceMailboxPlaceholder(command))
		if err != nil {
			return fmt.Errorf("parse %s command: %w", c.name, err)
		}
		*c.template = t
	}
	return nil
}

func replaceMailboxPlaceholder(command string) string {
	if command == "" {
		return command
	}
	return strings.ReplaceAll(command, "%s", "{{ .Mailbox }}")
}

// compileTemplate tests that the string template is valid, if any was provided.
func compileTemplate(s string) (*template.Template, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}

	t, err := template.New("").Parse(s)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	mailboxConfig := config.Box{Mailbox: "Inbox"}
	notifyConfig := config.NotifyConfig{
		Alias: "example@example.com",
		Boxes: []*config.Box{&mailboxConfig},
	}
	box := Box{Box: &mailboxConfig}
	data := IDLE{Box: box.WithAccount(&notifyConfig)}

	if err := t.Execute(io.Discard, &data); err != nil {
		return nil, fmt.Errorf("exec template: %w", err)
	}
	return t, nil
}

func (self *Box) WithAccount(account *config.NotifyConfig) *Box {
	self.account = account
	return self
}

func (self *Box) Account() *config.NotifyConfig { return self.account }

func (self *Box) Alias() string { return self.account.Alias }

func (self *Box) SkipNewMail() bool { return self.templateNewMail == nil }

func (self *Box) SkipChangedMail() bool {
	return self.templateChangedMail == nil
}

func (self *Box) SkipDeletedMail() bool {
	return self.templateDeletedMail == nil
}

func (self *Box) RenderCommandTo(w io.Writer, e *IDLE) error {
	return self.renderCommandTo(w, e, false)
}

func (self *Box) RenderPostCommandTo(w io.Writer, e *IDLE) error {
	if e.Reason == EventSync {
		return nil
	}
	return self.renderCommandTo(w, e, true)
}

func (self *Box) renderCommandTo(w io.Writer, e *IDLE, post bool) error {
	var t *template.Template
	switch e.Reason {
	case EventSync, EventNewMail:
		t = self.templateNewMail
		if post {
			t = self.templateNewMailPost
		}
	case EventDeletedMail:
		t = self.templateDeletedMail
		if post {
			t = self.templateDeletedMailPost
		}
	case EventFlagChanged:
		t = self.templateChangedMail
		if post {
			t = self.templateChangedMailPost
		}
	default:
		return fmt.Errorf(
			"template not found, unknown IDLE reason=%v, post=%v", e.Reason, post)
	}

	if t == nil {
		return nil
	}

	if err := t.Execute(w, e); err != nil {
		var name string
		if post {
			name = e.OnReasonPost()
		} else {
			name = e.OnReason()
		}
		return fmt.Errorf("executing %s template: %w", name, err)
	}
	return nil
}
