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
	"encoding/json"
	"testing"

	"go.yaml.in/yaml/v4"
)

// TestLegacyConverter tests the LegacyConverter function
func TestLegacyConverter(t *testing.T) {
	legacy := ConfigurationLegacy{
		Host:    "imap.example.com",
		HostCMD: "echo imap.example.com",
		Port:    993,
		TLS:     true,
		TLSOptions: TLSOptionsStruct{
			RejectUnauthorized: true,
			STARTTLS:           false,
		},
		IDLELogoutTimeout: 30,
		EnableIDCommand:   true,
		Username:          "user@example.com",
		UsernameCMD:       "echo user@example.com",
		Password:          "secret",
		PasswordCMD:       "pass show email",
		XOAuth2:           true,
		OnNewMail:         "mbsync -a",
		OnNewMailPost:     "notmuch new",
		OnChangedMail:     "echo changed",
		OnChangedMailPost: "echo changed post",
		OnDeletedMail:     "echo deleted",
		OnDeletedMailPost: "echo deleted post",
		Boxes:             []string{"INBOX", "Sent", "Drafts"},
	}

	result := LegacyConverter(&legacy)

	if len(result) != 1 {
		t.Fatalf("LegacyConverter() returned %d configs, want 1", len(result))
	}

	conf := result[0]

	// Verify all fields are copied correctly
	if conf.Host != legacy.Host {
		t.Errorf("Host = %q, want %q", conf.Host, legacy.Host)
	}
	if conf.HostCMD != legacy.HostCMD {
		t.Errorf("HostCMD = %q, want %q", conf.HostCMD, legacy.HostCMD)
	}
	if conf.Port != legacy.Port {
		t.Errorf("Port = %d, want %d", conf.Port, legacy.Port)
	}
	if conf.TLS != legacy.TLS {
		t.Errorf("TLS = %v, want %v", conf.TLS, legacy.TLS)
	}
	if conf.TLSOptions.RejectUnauthorized != legacy.TLSOptions.RejectUnauthorized {
		t.Errorf("TLSOptions.RejectUnauthorized = %v, want %v",
			conf.TLSOptions.RejectUnauthorized, legacy.TLSOptions.RejectUnauthorized)
	}
	if conf.TLSOptions.STARTTLS != legacy.TLSOptions.STARTTLS {
		t.Errorf("TLSOptions.STARTTLS = %v, want %v",
			conf.TLSOptions.STARTTLS, legacy.TLSOptions.STARTTLS)
	}
	if conf.IDLELogoutTimeout != legacy.IDLELogoutTimeout {
		t.Errorf(
			"IDLELogoutTimeout = %d, want %d",
			conf.IDLELogoutTimeout,
			legacy.IDLELogoutTimeout,
		)
	}
	if conf.EnableIDCommand != legacy.EnableIDCommand {
		t.Errorf("EnableIDCommand = %v, want %v", conf.EnableIDCommand, legacy.EnableIDCommand)
	}
	if conf.Username != legacy.Username {
		t.Errorf("Username = %q, want %q", conf.Username, legacy.Username)
	}
	if conf.UsernameCMD != legacy.UsernameCMD {
		t.Errorf("UsernameCMD = %q, want %q", conf.UsernameCMD, legacy.UsernameCMD)
	}
	if conf.Password != legacy.Password {
		t.Errorf("Password = %q, want %q", conf.Password, legacy.Password)
	}
	if conf.PasswordCMD != legacy.PasswordCMD {
		t.Errorf("PasswordCMD = %q, want %q", conf.PasswordCMD, legacy.PasswordCMD)
	}
	if conf.XOAuth2 != legacy.XOAuth2 {
		t.Errorf("XOAuth2 = %v, want %v", conf.XOAuth2, legacy.XOAuth2)
	}
	if conf.OnNewMail != legacy.OnNewMail {
		t.Errorf("OnNewMail = %q, want %q", conf.OnNewMail, legacy.OnNewMail)
	}
	if conf.OnNewMailPost != legacy.OnNewMailPost {
		t.Errorf("OnNewMailPost = %q, want %q", conf.OnNewMailPost, legacy.OnNewMailPost)
	}
	if conf.OnChangedMail != legacy.OnChangedMail {
		t.Errorf("OnChangedMail = %q, want %q", conf.OnChangedMail, legacy.OnChangedMail)
	}
	if conf.OnChangedMailPost != legacy.OnChangedMailPost {
		t.Errorf(
			"OnChangedMailPost = %q, want %q",
			conf.OnChangedMailPost,
			legacy.OnChangedMailPost,
		)
	}
	if conf.OnDeletedMail != legacy.OnDeletedMail {
		t.Errorf("OnDeletedMail = %q, want %q", conf.OnDeletedMail, legacy.OnDeletedMail)
	}
	if conf.OnDeletedMailPost != legacy.OnDeletedMailPost {
		t.Errorf(
			"OnDeletedMailPost = %q, want %q",
			conf.OnDeletedMailPost,
			legacy.OnDeletedMailPost,
		)
	}

	// Verify boxes are converted correctly
	if len(conf.Boxes) != len(legacy.Boxes) {
		t.Fatalf("Boxes length = %d, want %d", len(conf.Boxes), len(legacy.Boxes))
	}
	for i, box := range conf.Boxes {
		if box.Mailbox != legacy.Boxes[i] {
			t.Errorf("Boxes[%d].Mailbox = %q, want %q", i, box.Mailbox, legacy.Boxes[i])
		}
	}
}

// TestLegacyConverter_EmptyBoxes tests LegacyConverter with no boxes
func TestLegacyConverter_EmptyBoxes(t *testing.T) {
	legacy := ConfigurationLegacy{
		Host:  "imap.example.com",
		Port:  993,
		Boxes: []string{},
	}

	result := LegacyConverter(&legacy)

	if len(result) != 1 {
		t.Fatalf("LegacyConverter() returned %d configs, want 1", len(result))
	}

	if len(result[0].Boxes) != 0 {
		t.Errorf("Boxes length = %d, want 0", len(result[0].Boxes))
	}
}

// TestLegacyConverter_MinimalConfig tests LegacyConverter with minimal config
func TestLegacyConverter_MinimalConfig(t *testing.T) {
	legacy := ConfigurationLegacy{}

	result := LegacyConverter(&legacy)

	if len(result) != 1 {
		t.Fatalf("LegacyConverter() returned %d configs, want 1", len(result))
	}

	// All fields should be zero values
	conf := result[0]
	if conf.Host != "" {
		t.Errorf("Host = %q, want empty", conf.Host)
	}
	if conf.Port != 0 {
		t.Errorf("Port = %d, want 0", conf.Port)
	}
}

// TestNotifyConfig_JSONSerialization tests JSON marshaling/unmarshaling
func TestNotifyConfig_JSONSerialization(t *testing.T) {
	original := NotifyConfig{
		Host:     "imap.example.com",
		Port:     993,
		TLS:      true,
		Username: "user@example.com",
		Password: "secret",
		Boxes: []*Box{
			{Mailbox: "INBOX", OnNewMail: "echo new"},
			{Mailbox: "Sent", OnNewMail: "echo sent"},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Unmarshal back
	var result NotifyConfig
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Verify fields
	if result.Host != original.Host {
		t.Errorf("Host = %q, want %q", result.Host, original.Host)
	}
	if result.Port != original.Port {
		t.Errorf("Port = %d, want %d", result.Port, original.Port)
	}
	if result.TLS != original.TLS {
		t.Errorf("TLS = %v, want %v", result.TLS, original.TLS)
	}
	if len(result.Boxes) != len(original.Boxes) {
		t.Errorf("Boxes length = %d, want %d", len(result.Boxes), len(original.Boxes))
	}
}

// TestNotifyConfig_YAMLSerialization tests YAML marshaling/unmarshaling
func TestNotifyConfig_YAMLSerialization(t *testing.T) {
	original := NotifyConfig{
		Host:     "imap.example.com",
		Port:     993,
		TLS:      true,
		Username: "user@example.com",
		Password: "secret",
		TLSOptions: TLSOptionsStruct{
			RejectUnauthorized: true,
			STARTTLS:           false,
		},
		Boxes: []*Box{
			{Mailbox: "INBOX", OnNewMail: "echo new"},
		},
	}

	// Marshal to YAML
	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	// Unmarshal back
	var result NotifyConfig
	err = yaml.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	// Verify fields
	if result.Host != original.Host {
		t.Errorf("Host = %q, want %q", result.Host, original.Host)
	}
	if result.Port != original.Port {
		t.Errorf("Port = %d, want %d", result.Port, original.Port)
	}
	if result.TLSOptions.RejectUnauthorized != original.TLSOptions.RejectUnauthorized {
		t.Errorf("TLSOptions.RejectUnauthorized = %v, want %v",
			result.TLSOptions.RejectUnauthorized, original.TLSOptions.RejectUnauthorized)
	}
}

// TestConfiguration_JSONSerialization tests Configuration JSON serialization
func TestConfiguration_JSONSerialization(t *testing.T) {
	original := Configuration{
		Configurations: []*NotifyConfig{
			{
				Host:     "imap1.example.com",
				Port:     993,
				Username: "user1@example.com",
			},
			{
				Host:     "imap2.example.com",
				Port:     993,
				Username: "user2@example.com",
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Unmarshal back
	var result Configuration
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(result.Configurations) != len(original.Configurations) {
		t.Fatalf("Configurations length = %d, want %d",
			len(result.Configurations), len(original.Configurations))
	}

	for i, conf := range result.Configurations {
		if conf.Host != original.Configurations[i].Host {
			t.Errorf("Configurations[%d].Host = %q, want %q",
				i, conf.Host, original.Configurations[i].Host)
		}
	}
}

// TestTLSOptionsStructg tests TLSOptionsStruct serialization
func TestTLSOptionsStruct_Serialization(t *testing.T) {
	original := TLSOptionsStruct{
		RejectUnauthorized: true,
		STARTTLS:           true,
	}

	// JSON
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var jsonResult TLSOptionsStruct
	err = json.Unmarshal(jsonData, &jsonResult)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if jsonResult.RejectUnauthorized != original.RejectUnauthorized {
		t.Errorf("JSON RejectUnauthorized = %v, want %v",
			jsonResult.RejectUnauthorized, original.RejectUnauthorized)
	}
	if jsonResult.STARTTLS != original.STARTTLS {
		t.Errorf("JSON STARTTLS = %v, want %v", jsonResult.STARTTLS, original.STARTTLS)
	}

	// YAML
	yamlData, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}

	var yamlResult TLSOptionsStruct
	err = yaml.Unmarshal(yamlData, &yamlResult)
	if err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if yamlResult.RejectUnauthorized != original.RejectUnauthorized {
		t.Errorf("YAML RejectUnauthorized = %v, want %v",
			yamlResult.RejectUnauthorized, original.RejectUnauthorized)
	}
	if yamlResult.STARTTLS != original.STARTTLS {
		t.Errorf("YAML STARTTLS = %v, want %v", yamlResult.STARTTLS, original.STARTTLS)
	}
}

// TestConfigurationLegacy_AllFields tests that all ConfigurationLegacy fields work
func TestConfigurationLegacy_AllFields(t *testing.T) {
	legacy := ConfigurationLegacy{
		Host:              "imap.example.com",
		HostCMD:           "echo host",
		Port:              993,
		TLS:               true,
		TLSOptions:        TLSOptionsStruct{RejectUnauthorized: true, STARTTLS: true},
		IDLELogoutTimeout: 25,
		EnableIDCommand:   true,
		Username:          "user",
		UsernameCMD:       "echo user",
		Password:          "pass",
		PasswordCMD:       "echo pass",
		XOAuth2:           true,
		OnNewMail:         "new",
		OnNewMailPost:     "new post",
		OnChangedMail:     "changed",
		OnChangedMailPost: "changed post",
		OnDeletedMail:     "deleted",
		OnDeletedMailPost: "deleted post",
		Boxes:             []string{"INBOX"},
	}

	// Just verify it can be serialized
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var result ConfigurationLegacy
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if result.Host != legacy.Host {
		t.Errorf("Host mismatch after round-trip")
	}
}

// TestNotifyConfig_AllFields tests that all NotifyConfig fields work
func TestNotifyConfig_AllFields(t *testing.T) {
	conf := NotifyConfig{
		Host:              "imap.example.com",
		HostCMD:           "echo host",
		Port:              993,
		TLS:               true,
		TLSOptions:        TLSOptionsStruct{RejectUnauthorized: true, STARTTLS: true},
		IDLELogoutTimeout: 25,
		EnableIDCommand:   true,
		Username:          "user",
		UsernameCMD:       "echo user",
		Alias:             "alias",
		Password:          "pass",
		PasswordCMD:       "echo pass",
		XOAuth2:           true,
		OnNewMail:         "new",
		OnNewMailPost:     "new post",
		OnChangedMail:     "changed",
		OnChangedMailPost: "changed post",
		OnDeletedMail:     "deleted",
		OnDeletedMailPost: "deleted post",
		Boxes:             []*Box{{Mailbox: "INBOX"}},
	}

	// Verify it can be serialized and deserialized
	data, err := json.Marshal(conf)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var result NotifyConfig
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if result.Alias != conf.Alias {
		t.Errorf("Alias = %q, want %q", result.Alias, conf.Alias)
	}
}
