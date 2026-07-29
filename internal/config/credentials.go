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
	"log/slog"
	"os"
	"strings"

	"github.com/dsh2dsh/goimapnotify/internal/command"
)

// RetrieveCmd executes all credential commands and updates the config
func (self *NotifyConfig) RetrieveCmd() {
	self.Password = retrieveCmd(self.PasswordCMD, self.Password)
	self.Username = retrieveCmd(self.UsernameCMD, self.Username)
	self.Host = retrieveCmd(self.HostCMD, self.Host)
}

func retrieveCmd(cmdLine, def string) string {
	if cmdLine == "" {
		return def
	}

	cmd := command.New(cmdLine, "")
	// Avoid leaking the password
	cmd.Stdout = nil
	buf, err := cmd.Output()
	if err != nil {
		slog.Error("cannot retrieve password from command", slog.Any("error", err))
		os.Exit(1)
	}
	return strings.Trim(string(buf), "\n")
}
