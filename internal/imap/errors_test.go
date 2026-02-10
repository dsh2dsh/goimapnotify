package imap

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
	"errors"
	"fmt"
	"testing"
)

func TestErrCannotCheckSupportedAuth(t *testing.T) {
	if ErrCannotCheckSupportedAuth == nil {
		t.Fatal("ErrCannotCheckSupportedAuth is nil")
	}

	expectedMsg := "there was an error while checking supported authentication mechanism"
	if ErrCannotCheckSupportedAuth.Error() != expectedMsg {
		t.Errorf("ErrCannotCheckSupportedAuth.Error() = %q, want %q",
			ErrCannotCheckSupportedAuth.Error(), expectedMsg)
	}
}

func TestErrTokenAuthNotSupported(t *testing.T) {
	if ErrTokenAuthNotSupported == nil {
		t.Fatal("ErrTokenAuthNotSupported is nil")
	}

	expectedMsg := "XOAUTH2 and OAUTHBEARER are not supported by the server"
	if ErrTokenAuthNotSupported.Error() != expectedMsg {
		t.Errorf("ErrTokenAuthNotSupported.Error() = %q, want %q",
			ErrTokenAuthNotSupported.Error(), expectedMsg)
	}
}

func TestSentinelErrors_AreDistinct(t *testing.T) {
	if errors.Is(ErrCannotCheckSupportedAuth, ErrTokenAuthNotSupported) {
		t.Error("ErrCannotCheckSupportedAuth should not match ErrTokenAuthNotSupported")
	}

	if errors.Is(ErrTokenAuthNotSupported, ErrCannotCheckSupportedAuth) {
		t.Error("ErrTokenAuthNotSupported should not match ErrCannotCheckSupportedAuth")
	}
}

func TestSentinelErrors_WorkWithErrorsIs(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		target    error
		wantMatch bool
	}{
		{
			name:      "ErrCannotCheckSupportedAuth matches itself",
			err:       ErrCannotCheckSupportedAuth,
			target:    ErrCannotCheckSupportedAuth,
			wantMatch: true,
		},
		{
			name:      "ErrTokenAuthNotSupported matches itself",
			err:       ErrTokenAuthNotSupported,
			target:    ErrTokenAuthNotSupported,
			wantMatch: true,
		},
		{
			name:      "wrapped ErrCannotCheckSupportedAuth matches",
			err:       fmt.Errorf("connection failed: %w", ErrCannotCheckSupportedAuth),
			target:    ErrCannotCheckSupportedAuth,
			wantMatch: true,
		},
		{
			name:      "wrapped ErrTokenAuthNotSupported matches",
			err:       fmt.Errorf("auth failed: %w", ErrTokenAuthNotSupported),
			target:    ErrTokenAuthNotSupported,
			wantMatch: true,
		},
		{
			name:      "double wrapped error matches",
			err:       fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", ErrTokenAuthNotSupported)),
			target:    ErrTokenAuthNotSupported,
			wantMatch: true,
		},
		{
			name:      "unrelated error does not match ErrCannotCheckSupportedAuth",
			err:       errors.New("some other error"),
			target:    ErrCannotCheckSupportedAuth,
			wantMatch: false,
		},
		{
			name:      "unrelated error does not match ErrTokenAuthNotSupported",
			err:       errors.New("some other error"),
			target:    ErrTokenAuthNotSupported,
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errors.Is(tt.err, tt.target); got != tt.wantMatch {
				t.Errorf("errors.Is() = %v, want %v", got, tt.wantMatch)
			}
		})
	}
}

func TestSentinelErrors_CanBeUsedInSwitch(t *testing.T) {
	checkError := func(err error) string {
		switch {
		case errors.Is(err, ErrCannotCheckSupportedAuth):
			return "auth_check_failed"
		case errors.Is(err, ErrTokenAuthNotSupported):
			return "token_not_supported"
		default:
			return "unknown"
		}
	}

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "ErrCannotCheckSupportedAuth",
			err:  ErrCannotCheckSupportedAuth,
			want: "auth_check_failed",
		},
		{
			name: "ErrTokenAuthNotSupported",
			err:  ErrTokenAuthNotSupported,
			want: "token_not_supported",
		},
		{
			name: "wrapped ErrCannotCheckSupportedAuth",
			err:  fmt.Errorf("failed: %w", ErrCannotCheckSupportedAuth),
			want: "auth_check_failed",
		},
		{
			name: "other error",
			err:  errors.New("something else"),
			want: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkError(tt.err); got != tt.want {
				t.Errorf("checkError() = %q, want %q", got, tt.want)
			}
		})
	}
}
