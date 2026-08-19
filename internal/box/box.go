package box

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/dsh2dsh/goimapnotify/internal/config"
	"github.com/dsh2dsh/goimapnotify/internal/config/command"
)

type Box struct {
	*config.Box

	ExistingEmail uint32

	account *config.NotifyConfig
}

func CompileBoxes(accountConfig *config.NotifyConfig,
	configuredBoxes []*config.Box,
) ([]*Box, error) {
	mailboxConfig := config.Box{Mailbox: "Inbox"}
	notifyConfig := config.NotifyConfig{
		Alias: "example@example.com",
		Boxes: []*config.Box{&mailboxConfig},
	}
	box := Box{Box: &mailboxConfig}
	data := IDLE{box: box.WithAccount(&notifyConfig)}

	if err := accountConfig.CompileTemplates(&data); err != nil {
		return nil, fmt.Errorf("parse account commands: %s: %w",
			accountConfig.Alias, err)
	}

	boxes := make([]*Box, len(configuredBoxes))
	for i, in := range configuredBoxes {
		if err := in.CompileTemplates(&data); err != nil {
			return nil, fmt.Errorf(
				"parse mailbox commands: account=%s, mailbox=%s: %w",
				accountConfig.Alias, in.Mailbox, err)
		}
		boxes[i] = (&Box{Box: in}).WithAccount(accountConfig)
	}
	return boxes, nil
}

func (self *Box) WithAccount(account *config.NotifyConfig) *Box {
	self.account = account
	return self
}

func (self *Box) Account() *config.NotifyConfig { return self.account }

func (self *Box) Alias() string { return self.account.Alias }

func (self *Box) SkipNewMail() bool {
	return self.OnNewMail() == nil || self.OnNewMail().Skip()
}

func (self *Box) OnNewMail() *command.Templated {
	if self.Box.OnNewMail != nil {
		return self.Box.OnNewMail
	}
	return self.account.OnNewMail
}

func (self *Box) OnNewMailPost() *command.Templated {
	if self.Box.OnNewMailPost != nil {
		return self.Box.OnNewMailPost
	}
	return self.account.OnNewMailPost
}

func (self *Box) SkipChangedMail() bool {
	return self.OnChangedMail() == nil || self.OnChangedMail().Skip()
}

func (self *Box) OnChangedMail() *command.Templated {
	if self.Box.OnChangedMail != nil {
		return self.Box.OnChangedMail
	}
	return self.account.OnChangedMail
}

func (self *Box) OnChangedMailPost() *command.Templated {
	if self.Box.OnChangedMailPost != nil {
		return self.Box.OnChangedMailPost
	}
	return self.account.OnChangedMailPost
}

func (self *Box) SkipDeletedMail() bool {
	return self.OnDeletedMail() == nil || self.OnDeletedMail().Skip()
}

func (self *Box) OnDeletedMail() *command.Templated {
	if self.Box.OnDeletedMail != nil {
		return self.Box.OnDeletedMail
	}
	return self.account.OnDeletedMail
}

func (self *Box) OnDeletedMailPost() *command.Templated {
	if self.Box.OnDeletedMailPost != nil {
		return self.Box.OnDeletedMailPost
	}
	return self.account.OnDeletedMailPost
}

func (self *Box) Cmd(ctx context.Context, e *IDLE) (*exec.Cmd, error) {
	return self.buildCmd(ctx, e, false)
}

func (self *Box) PostCmd(ctx context.Context, e *IDLE) (*exec.Cmd, error) {
	if e.Reason() == EventSync {
		return nil, nil
	}
	return self.buildCmd(ctx, e, true)
}

func (self *Box) buildCmd(ctx context.Context, e *IDLE, post bool,
) (*exec.Cmd, error) {
	var t *command.Templated
	switch e.Reason() {
	case EventSync, EventNewMail:
		t = self.OnNewMail()
		if post {
			t = self.OnNewMailPost()
		}
	case EventDeletedMail:
		t = self.OnDeletedMail()
		if post {
			t = self.OnDeletedMailPost()
		}
	case EventFlagChanged:
		t = self.OnChangedMail()
		if post {
			t = self.OnChangedMailPost()
		}
	default:
		return nil, fmt.Errorf(
			"command not found, unknown IDLE reason=%v, post=%v", e.Reason(), post)
	}

	if t == nil || t.Skip() {
		return nil, nil
	}

	cmd, err := t.Cmd(ctx, e)
	if err != nil {
		if post {
			return nil, fmt.Errorf("build %s command: %w", e.OnReasonPost(), err)
		}
		return nil, fmt.Errorf("build %s command: %w", e.OnReason(), err)
	}
	return cmd, nil
}
