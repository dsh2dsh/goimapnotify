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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestRetrieveCmd(t *testing.T) {
	tests := []struct {
		name         string
		conf         NotifyConfig
		yaml         string
		wantPassword string
		wantUsername string
		wantHost     string
	}{
		{
			name: "all CMDs set",
			yaml: `
passwordCMD: "echo secret"
usernameCMD: "echo user@example.com"
hostCMD:		 "echo imap.example.com"`,
			wantPassword: "secret",
			wantUsername: "user@example.com",
			wantHost:     "imap.example.com",
		},
		{
			name: "only PasswordCMD set",
			yaml: `
passwordCMD: "echo secret"
username:		 "static@example.com"
host:				 "static.example.com"`,
			wantPassword: "secret",
			wantUsername: "static@example.com",
			wantHost:     "static.example.com",
		},
		{
			name: "only UsernameCMD set",
			yaml: `
password:		 "staticpass"
usernameCMD: "echo dynamic@example.com"
host:				 "static.example.com"`,
			wantPassword: "staticpass",
			wantUsername: "dynamic@example.com",
			wantHost:     "static.example.com",
		},
		{
			name: "only HostCMD set",
			yaml: `
password: "staticpass"
username: "static@example.com"
hostCMD:	"echo dynamic.example.com"`,
			wantPassword: "staticpass",
			wantUsername: "static@example.com",
			wantHost:     "dynamic.example.com",
		},
		{
			name: "no CMDs set",
			yaml: `
password: "staticpass"
username: "static@example.com"
host:			"static.example.com"`,
			wantPassword: "staticpass",
			wantUsername: "static@example.com",
			wantHost:     "static.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result NotifyConfig
			require.NoError(t, yaml.Unmarshal([]byte(tt.yaml), &result))
			result.RetrieveCmd()

			assert.Equal(t, tt.wantPassword, result.Password,
				"RetrieveCmd() Password")
			assert.Equal(t, tt.wantUsername, result.Username,
				"RetrieveCmd() Username")
			assert.Equal(t, tt.wantHost, result.Host, "RetrieveCmd() Host")
		})
	}
}
