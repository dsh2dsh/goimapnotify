package config

// This file is part of goimapnotify
// Copyright (C) 2017-2026	Jorge Javier Araya Navarro

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
	"text/template"
)

// EventType represents the type of IMAP event
type EventType int

const (
	NEWMAIL EventType = iota + 1
	DELETEDMAIL
	FLAGCHANGED
	SYNC
)

func (e EventType) String() string {
	switch e {
	case NEWMAIL:
		return "New Email"
	case DELETEDMAIL:
		return "Deleted Email"
	case FLAGCHANGED:
		return "Changed Flag on Email"
	case SYNC:
		return "Synchronize mailboxes without post-steps"
	default:
		return "Unknown Event"
	}
}

// Configuration holds the top-level configuration
type Configuration struct {
	Configurations []NotifyConfig `json:"configurations" yaml:"configurations"`
}

// ConfigurationLegacy holds the old configuration format
type ConfigurationLegacy struct {
	Host              string           `yaml:"host"              json:"host"`
	HostCMD           string           `yaml:"hostCMD"           json:"hostCMD"`
	Port              int              `yaml:"port"              json:"port"`
	TLS               bool             `yaml:"tls"               json:"tls"`
	TLSOptions        TLSOptionsStruct `yaml:"tlsOptions"        json:"tlsOptions"`
	IDLELogoutTimeout int              `yaml:"idleLogoutTimeout" json:"idleLogoutTimeout"`
	EnableIDCommand   bool             `yaml:"enableIDCommand"   json:"enableIDCommand"`
	Username          string           `yaml:"username"          json:"username"`
	UsernameCMD       string           `yaml:"usernameCMD"       json:"usernameCMD"`
	Password          string           `yaml:"password"          json:"password"`
	PasswordCMD       string           `yaml:"passwordCMD"       json:"passwordCMD"`
	XOAuth2           bool             `yaml:"xoAuth2"           json:"xoAuth2"`
	OnNewMail         string           `yaml:"onNewMail"         json:"onNewMail"`
	OnNewMailPost     string           `yaml:"onNewMailPost"     json:"onNewMailPost"`
	OnChangedMail     string           `yaml:"onChangedMail"     json:"onChangedMail"`
	OnChangedMailPost string           `yaml:"onChangedMailPost" json:"onChangedMailPost"`
	OnDeletedMail     string           `yaml:"onDeletedMail"     json:"onDeletedMail"`
	OnDeletedMailPost string           `yaml:"onDeletedMailPost" json:"onDeletedMailPost"`
	Boxes             []string         `yaml:"boxes"             json:"boxes"`
}

// NotifyConfig holds the configuration for a single account
type NotifyConfig struct {
	Host              string           `yaml:"host"              json:"host"`
	HostCMD           string           `yaml:"hostCMD"           json:"hostCMD"`
	Port              int              `yaml:"port"              json:"port"`
	TLS               bool             `yaml:"tls"               json:"tls"`
	TLSOptions        TLSOptionsStruct `yaml:"tlsOptions"        json:"tlsOptions"`
	IDLELogoutTimeout int              `yaml:"idleLogoutTimeout" json:"idleLogoutTimeout"`
	EnableIDCommand   bool             `yaml:"enableIDCommand"   json:"enableIDCommand"`
	Username          string           `yaml:"username"          json:"username"`
	UsernameCMD       string           `yaml:"usernameCMD"       json:"usernameCMD"`
	Alias             string           `yaml:"alias"             json:"alias"`
	Password          string           `yaml:"password"          json:"password"`
	PasswordCMD       string           `yaml:"passwordCMD"       json:"passwordCMD"`
	XOAuth2           bool             `yaml:"xoAuth2"           json:"xoAuth2"`
	OnNewMail         string           `yaml:"onNewMail"         json:"onNewMail"`
	OnNewMailPost     string           `yaml:"onNewMailPost"     json:"onNewMailPost"`
	OnChangedMail     string           `yaml:"onChangedMail"     json:"onChangedMail"`
	OnChangedMailPost string           `yaml:"onChangedMailPost" json:"onChangedMailPost"`
	OnDeletedMail     string           `yaml:"onDeletedMail"     json:"onDeletedMail"`
	OnDeletedMailPost string           `yaml:"onDeletedMailPost" json:"onDeletedMailPost"`
	Boxes             []Box            `yaml:"boxes"             json:"boxes"`
}

// TLSOptionsStruct holds TLS configuration options
type TLSOptionsStruct struct {
	RejectUnauthorized bool `yaml:"rejectUnauthorized" json:"rejectUnauthorized"`
	STARTTLS           bool `yaml:"starttls"           json:"starttls"`
}

// Box stores all the necessary info needed to be passed in an
// IDLEEvent handler routine, in order to schedule commands and
// print informative messages
type Box struct {
	Alias             string    `json:"-"                 yaml:"-"`
	Mailbox           string    `json:"mailbox"           yaml:"mailbox"`
	Reason            EventType `json:"-"                 yaml:"-"`
	OnNewMail         string    `json:"onNewMail"         yaml:"onNewMail"`
	OnNewMailPost     string    `json:"onNewMailPost"     yaml:"onNewMailPost"`
	OnChangedMail     string    `json:"onChangedMail"     yaml:"onChangedMail"`
	OnChangedMailPost string    `json:"onChangedMailPost" yaml:"onChangedMailPost"`
	OnDeletedMail     string    `json:"onDeletedMail"     yaml:"onDeletedMail"`
	OnDeletedMailPost string    `json:"onDeletedMailPost" yaml:"onDeletedMailPost"`
	ExistingEmail     uint32    `json:"-"                 yaml:"-"`
}

// IDLEEvent models an IDLE event (needed for template compilation test)
type IDLEEvent struct {
	Alias         string
	Mailbox       string
	Reason        EventType
	ExistingEmail int
	Box           Box
}

// CompileTemplate tests that the string template is valid, if any was
// provided.
func CompileTemplate(i string) error {
	t, err := template.New("test").Parse(i)
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(nil)

	input := IDLEEvent{
		Alias:   "example@example.com",
		Mailbox: "Inbox",
	}

	return t.Execute(buf, input)
}

// LegacyConverter converts old format configuration to new format
func LegacyConverter(conf ConfigurationLegacy) []NotifyConfig {
	var r []NotifyConfig
	var c NotifyConfig
	c.Host = conf.Host
	c.HostCMD = conf.HostCMD
	c.Port = conf.Port
	c.TLS = conf.TLS
	c.TLSOptions = conf.TLSOptions
	c.Username = conf.Username
	c.UsernameCMD = conf.UsernameCMD
	c.Password = conf.Password
	c.PasswordCMD = conf.PasswordCMD
	c.XOAuth2 = conf.XOAuth2
	c.OnNewMail = conf.OnNewMail
	c.OnNewMailPost = conf.OnNewMailPost
	c.OnChangedMail = conf.OnChangedMail
	c.OnChangedMailPost = conf.OnChangedMailPost
	c.OnDeletedMail = conf.OnDeletedMail
	c.OnDeletedMailPost = conf.OnDeletedMailPost
	c.IDLELogoutTimeout = conf.IDLELogoutTimeout
	c.EnableIDCommand = conf.EnableIDCommand
	for _, mailbox := range conf.Boxes {
		c.Boxes = append(c.Boxes, Box{Mailbox: mailbox})
	}
	return append(r, c)
}

// SetFromConfig inherits config values to Box and validates templates
func SetFromConfig(conf NotifyConfig, box Box) (Box, error) {
	if box.OnNewMail == "" {
		box.OnNewMail = conf.OnNewMail
	}
	err := CompileTemplate(box.OnNewMail)
	if err != nil {
		return box, err
	}
	if box.OnNewMailPost == "" {
		box.OnNewMailPost = conf.OnNewMailPost
	}
	err = CompileTemplate(box.OnNewMailPost)
	if err != nil {
		return box, err
	}

	// for deleted email
	if box.OnDeletedMail == "" {
		box.OnDeletedMail = conf.OnDeletedMail
	}
	err = CompileTemplate(box.OnDeletedMail)
	if err != nil {
		return box, err
	}
	if box.OnDeletedMailPost == "" {
		box.OnDeletedMailPost = conf.OnDeletedMailPost
	}
	err = CompileTemplate(box.OnDeletedMailPost)
	if err != nil {
		return box, err
	}

	box.Alias = conf.Alias
	return box, nil
}
