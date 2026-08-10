package config

import "time"

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

// Configuration holds the top-level configuration
type Configuration struct {
	MaxDelay       time.Duration       `yaml:"maxDelay" validate:"gt=0"`
	StartupSync    bool                `yaml:"startupSync"`
	DesktopNotify  DesktopNotification `yaml:"desktopNotify"`
	Configurations []*NotifyConfig     `yaml:"configurations" validate:"gt=0,dive,required"`
}

type DesktopNotification struct {
	Enable       bool   `yaml:"enable"`
	AppName      string `yaml:"appName" validate:"required_with=Enable"`
	AppIcon      string `yaml:"appIcon"`
	Category     string `yaml:"category"`
	DesktopEntry string `yaml:"desktopEntry"`
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
	Host              string           `yaml:"host"              json:"host" validate:"required"`
	HostCMD           string           `yaml:"hostCMD"           json:"hostCMD"`
	Port              int              `yaml:"port"              json:"port" validate:"gt=0"`
	TLS               bool             `yaml:"tls"               json:"tls"`
	TLSOptions        TLSOptionsStruct `yaml:"tlsOptions"        json:"tlsOptions"`
	IDLELogoutTimeout int              `yaml:"idleLogoutTimeout" json:"idleLogoutTimeout" validate:"min=0"`
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
	Boxes             []*Box           `yaml:"boxes"             json:"boxes" validate:"dive"`
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
	Mailbox           string `json:"mailbox"           yaml:"mailbox" validate:"required"`
	OnNewMail         string `json:"onNewMail"         yaml:"onNewMail"`
	OnNewMailPost     string `json:"onNewMailPost"     yaml:"onNewMailPost"`
	OnChangedMail     string `json:"onChangedMail"     yaml:"onChangedMail"`
	OnChangedMailPost string `json:"onChangedMailPost" yaml:"onChangedMailPost"`
	OnDeletedMail     string `json:"onDeletedMail"     yaml:"onDeletedMail"`
	OnDeletedMailPost string `json:"onDeletedMailPost" yaml:"onDeletedMailPost"`

	NotificationActions []*NotificationAction `yaml:"notificationActions" validate:"dive"`
}

type NotificationAction struct {
	Key   string   `yaml:"key"   validate:"required"`
	Label string   `yaml:"label" validate:"required"`
	Exec  []string `yaml:"exec"  validate:"min=1,dive,required"`
}
