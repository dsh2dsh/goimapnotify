//go:build !windows

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
	"reflect"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		wantArgs []string
	}{
		{
			name:     "simple command",
			command:  "echo hello",
			wantArgs: []string{"sh", "-c", "echo hello"},
		},
		{
			name:     "command with quotes",
			command:  "emacsclient -e '(something)'",
			wantArgs: []string{"sh", "-c", "emacsclient -e '(something)'"},
		},
		{
			name:     "command with pipes",
			command:  "echo hello | grep hello",
			wantArgs: []string{"sh", "-c", "echo hello | grep hello"},
		},
		{
			name:     "command with semicolon",
			command:  "echo first; echo second",
			wantArgs: []string{"sh", "-c", "echo first; echo second"},
		},
		{
			name:     "command with environment variable",
			command:  "echo $HOME",
			wantArgs: []string{"sh", "-c", "echo $HOME"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := New(tt.command)

			if !reflect.DeepEqual(cmd.Args, tt.wantArgs) {
				t.Errorf("New() Args = %v, want %v", cmd.Args, tt.wantArgs)
			}
		})
	}
}

func TestNew_ReturnsValidCmd(t *testing.T) {
	cmd := New("echo test")

	if cmd == nil {
		t.Fatal("New() returned nil")
	}

	// Cmd should be executable
	if cmd.Path == "" {
		t.Error("New() returned cmd with empty Path")
	}
}

func TestNew_StdoutStderrAreNil(t *testing.T) {
	cmd := New("echo test")

	if cmd.Stdout != nil {
		t.Error("New() Stdout should be nil")
	}
	if cmd.Stderr != nil {
		t.Error("New() Stderr should be nil")
	}
}

func TestNew_CanExecute(t *testing.T) {
	cmd := New("echo hello")

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("New() returned cmd that failed to execute: %v", err)
	}

	if string(output) != "hello\n" {
		t.Errorf("Command output = %q, want %q", string(output), "hello\n")
	}
}

// TestBugArgs is the original test (kept for compatibility)
func TestBugArgs(t *testing.T) {
	args := []string{"sh", "-c", "emacsclient -e '(something)'"}
	cmd := New("emacsclient -e '(something)'")
	if !reflect.DeepEqual(cmd.Args, args) {
		t.Errorf("*cmd.Args are %+v, expected %+v", cmd.Args, args)
	}
}
