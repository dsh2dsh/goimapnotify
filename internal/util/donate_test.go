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

func TestDonateMessage(t *testing.T) {
	tests := []struct {
		name    string
		padding int
	}{
		{
			name:    "zero padding",
			padding: 0,
		},
		{
			name:    "small padding",
			padding: 2,
		},
		{
			name:    "large padding",
			padding: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DonateMessage(tt.padding)

			// Should contain the donation URL
			if !strings.Contains(result, "https://ko-fi.com/K3K1XEZCQ") {
				t.Error("DonateMessage() should contain ko-fi URL")
			}

			// Should contain emoji
			if !strings.Contains(result, "✨") {
				t.Error("DonateMessage() should contain sparkle emoji")
			}

			// Should not be empty
			if result == "" {
				t.Error("DonateMessage() should not return empty string")
			}
		})
	}
}

func TestPrintDonate(t *testing.T) {
	tests := []struct {
		name    string
		padding int
	}{
		{
			name:    "zero padding",
			padding: 0,
		},
		{
			name:    "small padding",
			padding: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}

			PrintDonate(buf, tt.padding)

			result := buf.String()

			// Should contain the donation URL
			if !strings.Contains(result, "https://ko-fi.com/K3K1XEZCQ") {
				t.Error("PrintDonate() should contain ko-fi URL")
			}

			// Should contain star border
			if !strings.Contains(result, "*****") {
				t.Error("PrintDonate() should contain star border")
			}

			// Should not be empty
			if result == "" {
				t.Error("PrintDonate() should not write empty string")
			}
		})
	}
}

func TestPrintDonate_WritesToBuffer(t *testing.T) {
	buf := &bytes.Buffer{}

	PrintDonate(buf, 0)

	if buf.Len() == 0 {
		t.Error("PrintDonate() should write to the buffer")
	}
}

func TestDonateMessage_ContainsRequiredText(t *testing.T) {
	result := DonateMessage(0)

	requiredTexts := []string{
		"donation",
		"ko-fi.com",
		":D",
	}

	for _, text := range requiredTexts {
		if !strings.Contains(result, text) {
			t.Errorf("DonateMessage() should contain %q", text)
		}
	}
}
