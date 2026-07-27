package config

import (
	"fmt"
	"io"
	"strings"
	"text/template"
)

// FillBox inherits config values to Box and validates templates
func (self *NotifyConfig) FillBox(box *Box) error {
	if box.OnNewMail == "" {
		box.OnNewMail = self.OnNewMail
	}

	if err := compileTemplate(box.OnNewMail); err != nil {
		return err
	}

	if box.OnNewMailPost == "" {
		box.OnNewMailPost = self.OnNewMailPost
	}

	if err := compileTemplate(box.OnNewMailPost); err != nil {
		return err
	}

	// for deleted email
	if box.OnDeletedMail == "" {
		box.OnDeletedMail = self.OnDeletedMail
	}

	if err := compileTemplate(box.OnDeletedMail); err != nil {
		return err
	}

	if box.OnDeletedMailPost == "" {
		box.OnDeletedMailPost = self.OnDeletedMailPost
	}

	if err := compileTemplate(box.OnDeletedMailPost); err != nil {
		return err
	}

	box.Alias = self.Alias
	return nil
}

// compileTemplate tests that the string template is valid, if any was provided.
func compileTemplate(s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}

	t, err := template.New("").Parse(s)
	if err != nil {
		return fmt.Errorf("config: parse template: %w", err)
	}

	input := IDLEEvent{
		Alias:   "example@example.com",
		Mailbox: "Inbox",
	}

	if err := t.Execute(io.Discard, &input); err != nil {
		return fmt.Errorf("config: exec template: %w", err)
	}
	return nil
}
