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
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestTemplated(t *testing.T) {
	data := struct {
		Mailbox string
	}{
		Mailbox: "INBOX",
	}

	tests := []struct {
		name     string
		yaml     string
		expected []string
		str      string
		shell    bool
	}{
		{
			name:     "with shell",
			yaml:     `onNewMail: "exec imapsync '{{ .Mailbox }}' '%s'"`,
			expected: slices.Concat(Shell, []string{"exec imapsync 'INBOX' 'INBOX'"}),
			str: strings.Join(Shell, " ") + " " +
				"exec imapsync '{{ .Mailbox }}' '%s'",
			shell: true,
		},
		{
			name:     "direct",
			yaml:     `onNewMail: [ "imapsync", "{{ .Mailbox }}" ]`,
			expected: []string{"imapsync", data.Mailbox},
			str:      "imapsync {{ .Mailbox }}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := struct {
				OnNewMail *Templated `yaml:"onNewMail"`
			}{}

			require.NoError(t, yaml.Unmarshal([]byte(tt.yaml), &c))
			require.NotNil(t, c.OnNewMail)
			assert.Equal(t, tt.str, c.OnNewMail.String())
			assert.Equal(t, tt.shell, c.OnNewMail.shell)

			require.NoError(t, c.OnNewMail.Compile())
			cmd, err := c.OnNewMail.Cmd(t.Context(), &data)
			require.NoError(t, err)
			require.NotNil(t, cmd)

			assert.Equal(t, tt.expected[0], filepath.Base(cmd.Path))
			assert.Equal(t, tt.expected, cmd.Args)
		})
	}
}
