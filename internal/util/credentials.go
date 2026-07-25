package util

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
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/dsh2dsh/goimapnotify/internal/config"
)

// RetrievePasswordCmd executes passwordCMD and returns the config with the password
func RetrievePasswordCmd(conf config.NotifyConfig) config.NotifyConfig {
	if conf.PasswordCMD != "" {
		cmd := PrepareCommand(conf.PasswordCMD, config.IDLEEvent{})
		// Avoid leaking the password
		cmd.Stdout = nil
		buf, err := cmd.Output()
		if err == nil {
			conf.Password = strings.Trim(string(buf[:]), "\n")
		} else {
			logrus.WithError(err).Fatal("cannot retrieve password from command")
		}
	}
	return conf
}

// RetrieveUsernameCmd executes usernameCMD and returns the config with the username
func RetrieveUsernameCmd(conf config.NotifyConfig) config.NotifyConfig {
	if conf.UsernameCMD != "" {
		cmd := PrepareCommand(conf.UsernameCMD, config.IDLEEvent{})
		// Avoid leaking the username
		cmd.Stdout = nil
		buf, err := cmd.Output()
		if err == nil {
			conf.Username = strings.Trim(string(buf[:]), "\n")
		} else {
			logrus.WithError(err).Fatal("cannot retrieve username from command")
		}
	}
	return conf
}

// RetrieveHostCmd executes hostCMD and returns the config with the host
func RetrieveHostCmd(conf config.NotifyConfig) config.NotifyConfig {
	if conf.HostCMD != "" {
		cmd := PrepareCommand(conf.HostCMD, config.IDLEEvent{})
		// Avoid leaking the hostname
		cmd.Stdout = nil
		buf, err := cmd.Output()
		if err == nil {
			conf.Host = strings.Trim(string(buf[:]), "\n")
		} else {
			logrus.WithError(err).Fatal("cannot retrieve host from command")
		}
	}
	return conf
}

// RetrieveCmd executes all credential commands and returns the updated config
func RetrieveCmd(conf config.NotifyConfig) config.NotifyConfig {
	if conf.PasswordCMD != "" {
		conf = RetrievePasswordCmd(conf)
	}
	if conf.UsernameCMD != "" {
		conf = RetrieveUsernameCmd(conf)
	}
	if conf.HostCMD != "" {
		conf = RetrieveHostCmd(conf)
	}
	return conf
}
