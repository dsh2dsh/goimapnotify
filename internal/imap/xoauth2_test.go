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
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewXoauth2Client(t *testing.T) {
	client := NewXoauth2Client("user@example.com", "token123")
	if client == nil {
		t.Fatal("NewXoauth2Client returned nil")
	}
}

func TestXoauth2Client_Start(t *testing.T) {
	tests := []struct {
		name     string
		username string
		token    string
		wantMech string
		wantIR   string
		wantErr  bool
	}{
		{
			name:     "standard credentials",
			username: "user@example.com",
			token:    "ya29.token123",
			wantMech: "XOAUTH2",
			wantIR:   "user=user@example.com\x01auth=Bearer ya29.token123\x01\x01",
			wantErr:  false,
		},
		{
			name:     "empty username",
			username: "",
			token:    "token123",
			wantMech: "XOAUTH2",
			wantIR:   "user=\x01auth=Bearer token123\x01\x01",
			wantErr:  false,
		},
		{
			name:     "empty token",
			username: "user@example.com",
			token:    "",
			wantMech: "XOAUTH2",
			wantIR:   "user=user@example.com\x01auth=Bearer \x01\x01",
			wantErr:  false,
		},
		{
			name:     "username with special characters",
			username: "user+tag@example.com",
			token:    "token123",
			wantMech: "XOAUTH2",
			wantIR:   "user=user+tag@example.com\x01auth=Bearer token123\x01\x01",
			wantErr:  false,
		},
		{
			name:     "long token",
			username: "user@example.com",
			token:    "ya29.a0ARrdaM8_VERY_LONG_TOKEN_STRING_1234567890",
			wantMech: "XOAUTH2",
			wantIR:   "user=user@example.com\x01auth=Bearer ya29.a0ARrdaM8_VERY_LONG_TOKEN_STRING_1234567890\x01\x01",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewXoauth2Client(tt.username, tt.token)
			mech, ir, err := client.Start()

			if (err != nil) != tt.wantErr {
				t.Errorf("Start() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if mech != tt.wantMech {
				t.Errorf("Start() mech = %q, want %q", mech, tt.wantMech)
			}

			if string(ir) != tt.wantIR {
				t.Errorf("Start() ir = %q, want %q", string(ir), tt.wantIR)
			}
		})
	}
}

func TestXoauth2Client_Next(t *testing.T) {
	tests := []struct {
		name      string
		challenge []byte
		wantErr   any
	}{
		{
			name: "valid error response",
			challenge: []byte(
				`{"status":"401","schemes":"Bearer","scope":"https://mail.google.com/"}`,
			),
			wantErr: &Xoauth2Error{},
		},
		{
			name: "error response with different status",
			challenge: []byte(
				`{"status":"403","schemes":"Bearer","scope":"https://mail.google.com/"}`,
			),
			wantErr: &Xoauth2Error{},
		},
		{
			name:      "empty JSON object",
			challenge: []byte(`{}`),
			wantErr:   &Xoauth2Error{},
		},
		{
			name:      "invalid JSON",
			challenge: []byte(`not valid json`),
			wantErr:   &jsontext.SyntacticError{}, // JSON parse error
		},
		{
			name:      "empty challenge",
			challenge: []byte(``),
			wantErr:   &jsontext.SyntacticError{}, // JSON parse error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewXoauth2Client("user@example.com", "token123")
			// Call Start first as required by SASL protocol
			_, _, err := client.Start()
			require.NoError(t, err)

			resp, err := client.Next(tt.challenge)
			if tt.wantErr != nil {
				require.Error(t, err)
				switch v := tt.wantErr.(type) {
				case *Xoauth2Error:
					require.ErrorAs(t, err, &v)
				case *jsontext.SyntacticError:
					require.ErrorAs(t, err, &v)
				default:
					t.Errorf("Unrecognized error type: %T", err)
				}
				return
			}
			require.NotNil(t, resp)
		})
	}
}

func TestXoauth2Error_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *Xoauth2Error
		wantMsg string
	}{
		{
			name: "401 status",
			err: &Xoauth2Error{
				Status:  "401",
				Schemes: "Bearer",
				Scope:   "https://mail.google.com/",
			},
			wantMsg: "XOAUTH2 authentication error (401)",
		},
		{
			name: "403 status",
			err: &Xoauth2Error{
				Status:  "403",
				Schemes: "Bearer",
				Scope:   "https://mail.google.com/",
			},
			wantMsg: "XOAUTH2 authentication error (403)",
		},
		{
			name:    "empty error",
			err:     &Xoauth2Error{},
			wantMsg: "XOAUTH2 authentication error ()",
		},
		{
			name: "custom status message",
			err: &Xoauth2Error{
				Status: "invalid_grant",
			},
			wantMsg: "XOAUTH2 authentication error (invalid_grant)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestXoauth2Error_ImplementsError(t *testing.T) {
	var _ error = &Xoauth2Error{}
}

func TestXoauth2Error_JSONUnmarshal(t *testing.T) {
	jsonData := `{"status":"401","schemes":"Bearer","scope":"https://mail.google.com/"}`

	var err Xoauth2Error
	if unmarshalErr := json.Unmarshal([]byte(jsonData), &err); unmarshalErr != nil {
		t.Fatalf("json.Unmarshal() error = %v", unmarshalErr)
	}

	if err.Status != "401" {
		t.Errorf("Status = %q, want %q", err.Status, "401")
	}
	if err.Schemes != "Bearer" {
		t.Errorf("Schemes = %q, want %q", err.Schemes, "Bearer")
	}
	if err.Scope != "https://mail.google.com/" {
		t.Errorf("Scope = %q, want %q", err.Scope, "https://mail.google.com/")
	}
}
