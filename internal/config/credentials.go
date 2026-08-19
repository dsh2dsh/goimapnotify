package config

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
	"context"
	"fmt"
	"strings"

	"github.com/dsh2dsh/goimapnotify/internal/config/command"
)

// RetrieveCmd executes all credential commands and updates the config
func (self *NotifyConfig) RetrieveCmd() error {
	cmds := [...]struct {
		name  string
		t     *command.Templated
		value *string
	}{
		{"passwordCMD", self.PasswordCMD, &self.Password},
		{"usernameCMD", self.UsernameCMD, &self.Username},
		{"hostCMD", self.HostCMD, &self.Host},
	}

	for _, c := range cmds {
		if err := retrieveCmd(c.t, c.value); err != nil {
			return fmt.Errorf("retrieve value from %s: %w", c.name, err)
		}
	}
	return nil
}

func retrieveCmd(t *command.Templated, value *string) error {
	if t == nil || t.Skip() {
		return nil
	}

	if err := t.Compile(); err != nil {
		return err
	}

	cmd, err := t.Cmd(context.Background(), nil)
	if err != nil {
		return err
	}

	// Avoid leaking the password
	cmd.Stdout = nil
	buf, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("exec command template: %w", err)
	}

	*value = strings.TrimSpace(string(buf))
	return nil
}
