package command

// This file is part of goimapnotify
// Copyright (C) 2017-2025  Jorge Javier Araya Navarro

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"text/template"

	"go.yaml.in/yaml/v4"
)

type Templated struct {
	name      string
	args      []string
	templates []*template.Template
	shell     bool
}

func (self *Templated) String() string {
	if len(self.args) == 0 {
		return self.name
	}
	return self.name + " " + strings.Join(self.args, " ")
}

func (self *Templated) UnmarshalYAML(node *yaml.Node) error {
	var args []string
	err := node.Decode(&args)
	if err == nil {
		self.name = args[0]
		if len(args) > 1 {
			self.args = append(self.args, args[1:]...)
		}
		return nil
	}

	var s string
	if err2 := node.Decode(&s); err2 != nil {
		return fmt.Errorf("unmarshal command line: %w: %w", err, err2)
	}

	self.name = Shell[0]
	self.args = slices.Concat(Shell[1:], []string{s})
	self.shell = true
	return nil
}

func (self *Templated) Compile() error {
	if self.Skip() || len(self.args) == 0 {
		return nil
	}

	self.templates = make([]*template.Template, len(self.args))
	for i, s := range self.args {
		if strings.TrimSpace(s) == "" {
			continue
		}

		if self.shell {
			s = strings.ReplaceAll(s, "%s", "{{ .Mailbox }}")
		}

		if strings.Count(s, "{{") == 0 && strings.Count(s, "}}") == 0 {
			continue
		}

		t, err := template.New("").Parse(s)
		if err != nil {
			return fmt.Errorf("parse command line template: %w", err)
		}
		self.templates[i] = t
	}
	return nil
}

func (self *Templated) Skip() bool {
	if self.name == "" || self.name == "SKIP" {
		return true
	}
	return false
}

func (self *Templated) Cmd(ctx context.Context, data any) (*exec.Cmd, error) {
	var b bytes.Buffer
	args := make([]string, len(self.templates))
	for i, t := range self.templates {
		if t == nil {
			args[i] = self.args[i]
			continue
		}

		if err := t.Execute(&b, data); err != nil {
			return nil, fmt.Errorf("render command line templates: %w", err)
		}
		args[i] = b.String()
		b.Reset()
	}
	return exec.CommandContext(ctx, self.name, args...), nil
}

func (self *Templated) Validate() error {
	switch {
	case self.name == "":
		return errors.New("empty command")
	case !self.shell:
		return nil
	case len(self.args) < 2:
		return errors.New("shell requires more args")
	}
	return nil
}
