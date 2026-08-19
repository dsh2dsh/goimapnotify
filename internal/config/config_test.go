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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dsh2dsh/goimapnotify/internal/config/command"
)

func TestLoadBytes(t *testing.T) {
	confYAML := `
startupSync: false # don't run onNewMail right after connection

desktopNotify:
  enable: true
  appName: "goimapnotify"
  appIcon: "mail-unread-new"
  category: "email.arrived"
  desktopEntry: "goimapnotify"

configurations:
  - host: "localhost"
    port: 993
    tls: true
    username: "username@localhost"
    password: "abrakadabra"
    onNewMail: [ "imapnotify.sh", "{{ .Mailbox }}" ]
    boxes:
      - mailbox: "INBOX"
        notificationActions:
          - key: "default"
            label: "View"
            exec: [ "xdg-open", "https://localhost/Inbox/" ]
            closeAll: true`

	cfg, err := LoadBytes([]byte(confYAML))
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.EqualExportedValues(t, &Configuration{
		MaxDelay: 5 * time.Minute,
		DesktopNotify: DesktopNotification{
			Enable:        true,
			AppName:       "goimapnotify",
			AppIcon:       "mail-unread-new",
			Category:      "email.arrived",
			DesktopEntry:  "goimapnotify",
			ActionTimeout: 10 * time.Second,
		},
		Configurations: []*NotifyConfig{
			{
				Host:      "localhost",
				Port:      993,
				TLS:       true,
				Username:  "username@localhost",
				Password:  "abrakadabra",
				OnNewMail: &command.Templated{},
				Boxes: []*Box{
					{
						Mailbox: "INBOX",
						NotificationActions: []*NotificationAction{
							{
								Key:      "default",
								Label:    "View",
								Exec:     []string{"xdg-open", "https://localhost/Inbox/"},
								CloseAll: true,
							},
						},
					},
				},
			},
		},
	}, cfg)
}

func TestLoadBytes_legacy(t *testing.T) {
	confYAML := `
host: "localhost"
port: 993
tls: true
username: "username@localhost"
password: "abrakadabra"
onNewMail: [ "imapnotify.sh", "{{ .Mailbox }}" ]
boxes: [ "INBOX" ]`

	cfg, err := LoadBytes([]byte(confYAML))
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.EqualExportedValues(t, &Configuration{
		MaxDelay:    5 * time.Minute,
		StartupSync: true,
		DesktopNotify: DesktopNotification{
			AppName:       "goimapnotify",
			ActionTimeout: 10 * time.Second,
		},
		Configurations: []*NotifyConfig{
			{
				Host:      "localhost",
				Port:      993,
				TLS:       true,
				Username:  "username@localhost",
				Password:  "abrakadabra",
				OnNewMail: &command.Templated{},
				Boxes: []*Box{
					{
						Mailbox: "INBOX",
					},
				},
			},
		},
	}, cfg)
}
