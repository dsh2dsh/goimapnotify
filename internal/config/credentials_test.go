//go:build !windows

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
	"testing"
)

func TestRetrieveCmd(t *testing.T) {
	tests := []struct {
		name         string
		conf         NotifyConfig
		wantPassword string
		wantUsername string
		wantHost     string
	}{
		{
			name: "all CMDs set",
			conf: NotifyConfig{
				PasswordCMD: "echo secret",
				UsernameCMD: "echo user@example.com",
				HostCMD:     "echo imap.example.com",
			},
			wantPassword: "secret",
			wantUsername: "user@example.com",
			wantHost:     "imap.example.com",
		},
		{
			name: "only PasswordCMD set",
			conf: NotifyConfig{
				PasswordCMD: "echo secret",
				Username:    "static@example.com",
				Host:        "static.example.com",
			},
			wantPassword: "secret",
			wantUsername: "static@example.com",
			wantHost:     "static.example.com",
		},
		{
			name: "only UsernameCMD set",
			conf: NotifyConfig{
				Password:    "staticpass",
				UsernameCMD: "echo dynamic@example.com",
				Host:        "static.example.com",
			},
			wantPassword: "staticpass",
			wantUsername: "dynamic@example.com",
			wantHost:     "static.example.com",
		},
		{
			name: "only HostCMD set",
			conf: NotifyConfig{
				Password: "staticpass",
				Username: "static@example.com",
				HostCMD:  "echo dynamic.example.com",
			},
			wantPassword: "staticpass",
			wantUsername: "static@example.com",
			wantHost:     "dynamic.example.com",
		},
		{
			name: "no CMDs set",
			conf: NotifyConfig{
				Password: "staticpass",
				Username: "static@example.com",
				Host:     "static.example.com",
			},
			wantPassword: "staticpass",
			wantUsername: "static@example.com",
			wantHost:     "static.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.conf
			result.RetrieveCmd()

			if result.Password != tt.wantPassword {
				t.Errorf("RetrieveCmd() Password = %q, want %q",
					result.Password, tt.wantPassword)
			}
			if result.Username != tt.wantUsername {
				t.Errorf("RetrieveCmd() Username = %q, want %q",
					result.Username, tt.wantUsername)
			}
			if result.Host != tt.wantHost {
				t.Errorf("RetrieveCmd() Host = %q, want %q",
					result.Host, tt.wantHost)
			}
		})
	}
}
