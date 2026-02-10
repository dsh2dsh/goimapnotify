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
	"reflect"
	"testing"

	"gitlab.com/shackra/goimapnotify/internal/config"
)

func TestPrepareCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		event    config.IDLEEvent
		wantArgs []string
	}{
		{
			name:     "simple command",
			command:  "echo hello",
			event:    config.IDLEEvent{},
			wantArgs: []string{"sh", "-c", "echo hello"},
		},
		{
			name:    "command with %s placeholder",
			command: "mbsync %s",
			event: config.IDLEEvent{
				Mailbox: "INBOX",
			},
			wantArgs: []string{"sh", "-c", "mbsync INBOX"},
		},
		{
			name:     "command with quotes",
			command:  "emacsclient -e '(something)'",
			event:    config.IDLEEvent{},
			wantArgs: []string{"sh", "-c", "emacsclient -e '(something)'"},
		},
		{
			name:    "command with multiple %s",
			command: "echo %s %s",
			event: config.IDLEEvent{
				Mailbox: "INBOX",
			},
			wantArgs: []string{"sh", "-c", "echo INBOX %!s(MISSING)"},
		},
		{
			name:     "command with pipes",
			command:  "echo hello | grep hello",
			event:    config.IDLEEvent{},
			wantArgs: []string{"sh", "-c", "echo hello | grep hello"},
		},
		{
			name:     "command with semicolon",
			command:  "echo first; echo second",
			event:    config.IDLEEvent{},
			wantArgs: []string{"sh", "-c", "echo first; echo second"},
		},
		{
			name:     "command with environment variable",
			command:  "echo $HOME",
			event:    config.IDLEEvent{},
			wantArgs: []string{"sh", "-c", "echo $HOME"},
		},
		{
			name:    "command with special mailbox name",
			command: "mbsync '%s'",
			event: config.IDLEEvent{
				Mailbox: "INBOX/Subfolder",
			},
			wantArgs: []string{"sh", "-c", "mbsync 'INBOX/Subfolder'"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := PrepareCommand(tt.command, tt.event)

			if !reflect.DeepEqual(cmd.Args, tt.wantArgs) {
				t.Errorf("PrepareCommand() Args = %v, want %v", cmd.Args, tt.wantArgs)
			}
		})
	}
}

func TestPrepareCommand_ReturnsValidCmd(t *testing.T) {
	cmd := PrepareCommand("echo test", config.IDLEEvent{})

	if cmd == nil {
		t.Fatal("PrepareCommand() returned nil")
	}

	// Cmd should be executable
	if cmd.Path == "" {
		t.Error("PrepareCommand() returned cmd with empty Path")
	}
}

func TestPrepareCommand_StdoutStderrAreNil(t *testing.T) {
	cmd := PrepareCommand("echo test", config.IDLEEvent{})

	if cmd.Stdout != nil {
		t.Error("PrepareCommand() Stdout should be nil")
	}
	if cmd.Stderr != nil {
		t.Error("PrepareCommand() Stderr should be nil")
	}
}

func TestPrepareCommand_CanExecute(t *testing.T) {
	cmd := PrepareCommand("echo hello", config.IDLEEvent{})

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("PrepareCommand() returned cmd that failed to execute: %v", err)
	}

	if string(output) != "hello\n" {
		t.Errorf("Command output = %q, want %q", string(output), "hello\n")
	}
}

func TestPrepareCommand_WithMailboxSubstitution(t *testing.T) {
	event := config.IDLEEvent{
		Mailbox: "TestMailbox",
	}

	cmd := PrepareCommand("echo %s", event)

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	if string(output) != "TestMailbox\n" {
		t.Errorf("Command output = %q, want %q", string(output), "TestMailbox\n")
	}
}

func TestPrepareCommand_WithoutSubstitution(t *testing.T) {
	event := config.IDLEEvent{
		Mailbox: "INBOX",
	}

	// Command without %s should not substitute
	cmd := PrepareCommand("echo static", event)

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	if string(output) != "static\n" {
		t.Errorf("Command output = %q, want %q", string(output), "static\n")
	}
}

// TestBugArgs is the original test (kept for compatibility)
func TestBugArgs(t *testing.T) {
	args := []string{"sh", "-c", "emacsclient -e '(something)'"}
	cmd := PrepareCommand("emacsclient -e '(something)'", config.IDLEEvent{})
	if !reflect.DeepEqual(cmd.Args, args) {
		t.Errorf("*cmd.Args are %+v, expected %+v", cmd.Args, args)
	}
}

func TestPrepareCommand_EmptyMailbox(t *testing.T) {
	event := config.IDLEEvent{
		Mailbox: "",
	}

	cmd := PrepareCommand("echo '%s'", event)

	// Should still work with empty mailbox
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	// echo '' produces just a newline (empty string between quotes is empty output)
	if string(output) != "\n" {
		t.Errorf("Command output = %q, want %q", string(output), "\n")
	}
}

func TestPrepareCommand_SpecialCharactersInMailbox(t *testing.T) {
	testCases := []struct {
		name    string
		mailbox string
	}{
		{"with space", "INBOX Folder"},
		{"with slash", "INBOX/Subfolder"},
		{"with dot", "INBOX.Subfolder"},
		{"with ampersand", "Test&Archive"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			event := config.IDLEEvent{
				Mailbox: tc.mailbox,
			}

			cmd := PrepareCommand("echo %s", event)

			// Command should at least be created without error
			if cmd == nil {
				t.Error("PrepareCommand() returned nil")
			}
		})
	}
}
