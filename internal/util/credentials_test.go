//go:build !windows
// +build !windows

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
	"fmt"
	"testing"

	"github.com/dsh2dsh/goimapnotify/internal/config"
)

func TestRetrievePasswordCmd(t *testing.T) {
	tests := []struct {
		name         string
		conf         config.NotifyConfig
		wantPassword string
	}{
		{
			name: "simple password",
			conf: config.NotifyConfig{
				PasswordCMD: "echo secret123",
			},
			wantPassword: "secret123",
		},
		{
			name: "password with spaces trimmed",
			conf: config.NotifyConfig{
				PasswordCMD: "echo '  password  '",
			},
			wantPassword: "  password  ", // Only newlines are trimmed
		},
		{
			name: "empty PasswordCMD returns original",
			conf: config.NotifyConfig{
				Password:    "original",
				PasswordCMD: "",
			},
			wantPassword: "original",
		},
		{
			name: "complex password",
			conf: config.NotifyConfig{
				PasswordCMD: "echo 'p@ssw0rd!#$%'",
			},
			wantPassword: "p@ssw0rd!#$%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RetrievePasswordCmd(tt.conf)
			if result.Password != tt.wantPassword {
				t.Errorf("RetrievePasswordCmd() Password = %q, want %q",
					result.Password, tt.wantPassword)
			}
		})
	}
}

func TestRetrieveUsernameCmd(t *testing.T) {
	tests := []struct {
		name         string
		conf         config.NotifyConfig
		wantUsername string
	}{
		{
			name: "simple username",
			conf: config.NotifyConfig{
				UsernameCMD: "echo user@example.com",
			},
			wantUsername: "user@example.com",
		},
		{
			name: "empty UsernameCMD returns original",
			conf: config.NotifyConfig{
				Username:    "original@example.com",
				UsernameCMD: "",
			},
			wantUsername: "original@example.com",
		},
		{
			name: "username with special characters",
			conf: config.NotifyConfig{
				UsernameCMD: "echo 'user+tag@example.com'",
			},
			wantUsername: "user+tag@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RetrieveUsernameCmd(tt.conf)
			if result.Username != tt.wantUsername {
				t.Errorf("RetrieveUsernameCmd() Username = %q, want %q",
					result.Username, tt.wantUsername)
			}
		})
	}
}

func TestRetrieveHostCmd(t *testing.T) {
	tests := []struct {
		name     string
		conf     config.NotifyConfig
		wantHost string
	}{
		{
			name: "simple host",
			conf: config.NotifyConfig{
				HostCMD: "echo imap.example.com",
			},
			wantHost: "imap.example.com",
		},
		{
			name: "empty HostCMD returns original",
			conf: config.NotifyConfig{
				Host:    "original.example.com",
				HostCMD: "",
			},
			wantHost: "original.example.com",
		},
		{
			name: "host with subdomain",
			conf: config.NotifyConfig{
				HostCMD: "echo mail.subdomain.example.com",
			},
			wantHost: "mail.subdomain.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RetrieveHostCmd(tt.conf)
			if result.Host != tt.wantHost {
				t.Errorf("RetrieveHostCmd() Host = %q, want %q",
					result.Host, tt.wantHost)
			}
		})
	}
}

func TestRetrieveCmd(t *testing.T) {
	tests := []struct {
		name         string
		conf         config.NotifyConfig
		wantPassword string
		wantUsername string
		wantHost     string
	}{
		{
			name: "all CMDs set",
			conf: config.NotifyConfig{
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
			conf: config.NotifyConfig{
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
			conf: config.NotifyConfig{
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
			conf: config.NotifyConfig{
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
			conf: config.NotifyConfig{
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
			result := RetrieveCmd(tt.conf)

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

// TestPasswordCMD is the original test (kept for compatibility)
func TestPasswordCMD(t *testing.T) {
	password := "secret123"
	c := RetrievePasswordCmd(config.NotifyConfig{PasswordCMD: fmt.Sprintf("echo %s", password)})
	if c.Password != password {
		t.Fatalf("'%s' != '%s'", c.Password, password)
	}
}

func TestRetrieveCmd_PreservesOtherFields(t *testing.T) {
	conf := config.NotifyConfig{
		Port:              993,
		TLS:               true,
		XOAuth2:           true,
		OnNewMail:         "echo new",
		IDLELogoutTimeout: 30,
		PasswordCMD:       "echo secret",
	}

	result := RetrieveCmd(conf)

	// Verify other fields are preserved
	if result.Port != 993 {
		t.Errorf("Port not preserved: got %d, want 993", result.Port)
	}
	if !result.TLS {
		t.Error("TLS not preserved")
	}
	if !result.XOAuth2 {
		t.Error("XOAuth2 not preserved")
	}
	if result.OnNewMail != "echo new" {
		t.Errorf("OnNewMail not preserved: got %q", result.OnNewMail)
	}
	if result.IDLELogoutTimeout != 30 {
		t.Errorf("IDLELogoutTimeout not preserved: got %d", result.IDLELogoutTimeout)
	}
}
