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
	"bytes"
	"strings"
	"testing"
)

func TestCensorEmailAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple email",
			input:    "user@example.com",
			expected: "*******@*****.***",
		},
		{
			name:     "email in text",
			input:    "Hello user@example.com how are you?",
			expected: "Hello *******@*****.*** how are you?",
		},
		{
			name:     "multiple emails",
			input:    "From: sender@example.com To: receiver@test.org",
			expected: "From: *******@*****.*** To: *******@*****.***",
		},
		{
			name:     "no email",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "email with plus sign",
			input:    "user+tag@example.com",
			expected: "*******@*****.***",
		},
		{
			name:     "email with dots in local part",
			input:    "first.last@example.com",
			expected: "*******@*****.***",
		},
		{
			name:     "email with subdomain",
			input:    "user@mail.example.com",
			expected: "*******@*****.***",
		},
		{
			name:     "email with numbers",
			input:    "user123@example456.com",
			expected: "*******@*****.***",
		},
		{
			name:     "IMAP log line with email",
			input:    "C: a]$ LOGIN user@example.com password",
			expected: "C: a]$ LOGIN *******@*****.*** password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CensorEmailAddress(tt.input)
			if result != tt.expected {
				t.Errorf("CensorEmailAddress(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCensorPasswordInLogin(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "LOGIN with password",
			input:    `C: a]$ LOGIN username "secretpassword"`,
			expected: `C: a]$ LOGIN username "****"`,
		},
		{
			name:     "LOGIN with password and trailing text",
			input:    `C: a]$ LOGIN user "pass" extra`,
			expected: `C: a]$ LOGIN user "****" extra`,
		},
		{
			name:     "no LOGIN command",
			input:    "C: a]$ SELECT INBOX",
			expected: "C: a]$ SELECT INBOX",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "LOGIN without password quotes",
			input:    "C: a]$ LOGIN user password",
			expected: "C: a]$ LOGIN user password",
		},
		{
			name:     "LOGIN with complex password",
			input:    `C: a]$ LOGIN myuser "p@ssw0rd!#$%"`,
			expected: `C: a]$ LOGIN myuser "****"`,
		},
		{
			name:     "lowercase login (matched)",
			input:    `C: a]$ login user "password"`,
			expected: `C: a]$ login user "****"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CensorPasswordInLogin(tt.input)
			if result != tt.expected {
				t.Errorf("CensorPasswordInLogin(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCensorOAuthToken(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "XOAUTH2 token",
			input:    "C: a001 AUTHENTICATE XOAUTH2 user=user@example.com\x01auth=Bearer ya29.token123\x01\x01",
			expected: "C: a001 AUTHENTICATE XOAUTH2 user=user@example.com\x01auth=Bearer ****\x01\x01",
		},
		{
			name:     "OAUTHBEARER token",
			input:    "n,a=user,\x01host=example.com\x01port=993\x01auth=Bearer token.abc\x01\x01",
			expected: "n,a=user,\x01host=example.com\x01port=993\x01auth=Bearer ****\x01\x01",
		},
		{
			name:     "no token",
			input:    "C: a002 SELECT INBOX",
			expected: "C: a002 SELECT INBOX",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CensorOAuthToken(tt.input)
			if result != tt.expected {
				t.Errorf("CensorOAuthToken(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCensorCredentials(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "line with email",
			input:    "Connecting as user@example.com\n",
			expected: "Connecting as *******@*****.***\n",
		},
		{
			name:     "line with LOGIN and password",
			input:    "C: a]$ LOGIN user \"secret\"\n",
			expected: "C: a]$ LOGIN user \"****\"\n",
		},
		{
			name:     "line with both email and password",
			input:    "C: a]$ LOGIN user@example.com \"secret\"\n",
			expected: "C: a]$ LOGIN *******@*****.*** \"****\"\n",
		},
		{
			name:     "multiple lines",
			input:    "Line 1\nuser@example.com\nLine 3\n",
			expected: "Line 1\n*******@*****.***\nLine 3\n",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
		{
			name:     "line without sensitive data",
			input:    "* OK IMAP server ready\n",
			expected: "* OK IMAP server ready\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := strings.NewReader(tt.input)
			out := &bytes.Buffer{}

			CensorCredentials(in, out)

			result := out.String()
			if result != tt.expected {
				t.Errorf("CensorCredentials() output = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestCensorCredentials_MultipleLines(t *testing.T) {
	input := `* OK IMAP server ready
C: a001 LOGIN user@example.com "password123"
S: a001 OK LOGIN completed
C: a002 SELECT INBOX
S: * 5 EXISTS
`
	expected := `* OK IMAP server ready
C: a001 LOGIN *******@*****.*** "****"
S: a001 OK LOGIN completed
C: a002 SELECT INBOX
S: * 5 EXISTS
`

	in := strings.NewReader(input)
	out := &bytes.Buffer{}

	CensorCredentials(in, out)

	result := out.String()
	if result != expected {
		t.Errorf("CensorCredentials() output:\n%s\nwant:\n%s", result, expected)
	}
}

func TestEmailRegexp(t *testing.T) {
	validEmails := []string{
		"test@example.com",
		"user.name@example.com",
		"user+tag@example.com",
		"user123@example123.com",
		"a@b.co",
		"very.long.email@subdomain.example.org",
	}

	for _, email := range validEmails {
		t.Run("valid_"+email, func(t *testing.T) {
			if !emailRegexp.MatchString(email) {
				t.Errorf("emailRegexp should match %q", email)
			}
		})
	}

	invalidEmails := []string{
		"notanemail",
		"@example.com",
		"user@",
		"user@.com",
		"user@example",
	}

	for _, email := range invalidEmails {
		t.Run("invalid_"+email, func(t *testing.T) {
			if emailRegexp.MatchString(email) {
				t.Errorf("emailRegexp should not match %q", email)
			}
		})
	}
}

func TestDetectPasswordInLOGINRegexp(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMatch bool
	}{
		{
			name:      "valid LOGIN with password",
			input:     `LOGIN user "password"`,
			wantMatch: true,
		},
		{
			name:      "LOGIN with prefix",
			input:     `C: a001 LOGIN user "password"`,
			wantMatch: true,
		},
		{
			name:      "no LOGIN",
			input:     `SELECT INBOX`,
			wantMatch: false,
		},
		{
			name:      "LOGIN without quotes",
			input:     `LOGIN user password`,
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched := detectPasswordInLOGIN.MatchString(tt.input)
			if matched != tt.wantMatch {
				t.Errorf("detectPasswordInLOGIN.MatchString(%q) = %v, want %v",
					tt.input, matched, tt.wantMatch)
			}
		})
	}
}
