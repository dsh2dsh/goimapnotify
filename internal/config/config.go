package config

import (
	"context"
	"fmt"
	"time"

	"github.com/dsh2dsh/goimapnotify/internal/config/command"
)

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
	Enable        bool          `yaml:"enable"`
	AppName       string        `yaml:"appName" validate:"required_with=Enable"`
	AppIcon       string        `yaml:"appIcon"`
	Category      string        `yaml:"category"`
	DesktopEntry  string        `yaml:"desktopEntry"`
	ActionTimeout time.Duration `yaml:"actionTimeout" validate:"min=0"`
}

// ConfigurationLegacy holds the old configuration format
type ConfigurationLegacy struct {
	Host              string             `yaml:"host"`
	HostCMD           *command.Templated `yaml:"hostCMD"`
	Port              int                `yaml:"port"`
	TLS               bool               `yaml:"tls"`
	TLSOptions        TLSOptionsStruct   `yaml:"tlsOptions"`
	IDLELogoutTimeout int                `yaml:"idleLogoutTimeout"`
	EnableIDCommand   bool               `yaml:"enableIDCommand"`
	Username          string             `yaml:"username"`
	UsernameCMD       *command.Templated `yaml:"usernameCMD"`
	Password          string             `yaml:"password"`
	PasswordCMD       *command.Templated `yaml:"passwordCMD"`
	XOAuth2           bool               `yaml:"xoAuth2"`
	OnNewMail         *command.Templated `yaml:"onNewMail"`
	OnNewMailPost     *command.Templated `yaml:"onNewMailPost"`
	OnChangedMail     *command.Templated `yaml:"onChangedMail"`
	OnChangedMailPost *command.Templated `yaml:"onChangedMailPost"`
	OnDeletedMail     *command.Templated `yaml:"onDeletedMail"`
	OnDeletedMailPost *command.Templated `yaml:"onDeletedMailPost"`
	Boxes             []string           `yaml:"boxes"`
}

// NotifyConfig holds the configuration for a single account
type NotifyConfig struct {
	JMAP              bool               `yaml:"jmap"`
	Ping              *int               `yaml:"ping"`
	Host              string             `yaml:"host"`
	HostCMD           *command.Templated `yaml:"hostCMD" validate:"omitnil,validateFn"`
	Port              int                `yaml:"port" validate:"min=0"`
	TLS               bool               `yaml:"tls"`
	TLSOptions        TLSOptionsStruct   `yaml:"tlsOptions"`
	IDLELogoutTimeout int                `yaml:"idleLogoutTimeout" validate:"min=0"`
	EnableIDCommand   bool               `yaml:"enableIDCommand"`
	Username          string             `yaml:"username" validate:"required"`
	UsernameCMD       *command.Templated `yaml:"usernameCMD" validate:"omitnil,validateFn"`
	Alias             string             `yaml:"alias"`
	Password          string             `yaml:"password"`
	PasswordCMD       *command.Templated `yaml:"passwordCMD" validate:"omitnil,validateFn"`
	XOAuth2           bool               `yaml:"xoAuth2"`
	OnNewMail         *command.Templated `yaml:"onNewMail" validate:"omitnil,validateFn"`
	OnNewMailPost     *command.Templated `yaml:"onNewMailPost" validate:"omitnil,validateFn"`
	OnChangedMail     *command.Templated `yaml:"onChangedMail" validate:"omitnil,validateFn"`
	OnChangedMailPost *command.Templated `yaml:"onChangedMailPost" validate:"omitnil,validateFn"`
	OnDeletedMail     *command.Templated `yaml:"onDeletedMail" validate:"omitnil,validateFn"`
	OnDeletedMailPost *command.Templated `yaml:"onDeletedMailPost" validate:"omitnil,validateFn"`
	Boxes             []*Box             `yaml:"boxes" validate:"dive"`
}

// TLSOptionsStruct holds TLS configuration options
type TLSOptionsStruct struct {
	RejectUnauthorized *bool `yaml:"rejectUnauthorized"`
	STARTTLS           bool  `yaml:"starttls"`
}

func (self *TLSOptionsStruct) GetRejectUnauthorized() bool {
	if self.RejectUnauthorized == nil {
		return true
	}
	return *self.RejectUnauthorized
}

func (self *NotifyConfig) CompileTemplates(data any) error {
	cmds := [...]struct {
		name string
		t    *command.Templated
		data any
	}{
		{"onNewMail", self.OnNewMail, data},
		{"onNewMailPost", self.OnNewMailPost, data},
		{"onChangedMail", self.OnChangedMail, data},
		{"onChangedMailPost", self.OnChangedMailPost, data},
		{"onDeletedMail", self.OnDeletedMail, data},
		{"onDeletedMailPost", self.OnDeletedMailPost, data},
	}

	for _, c := range cmds {
		if c.t == nil {
			continue
		}
		if err := c.t.Compile(); err != nil {
			return fmt.Errorf("parse %s template: %w", c.name, err)
		}
		if _, err := c.t.Cmd(context.Background(), c.data); err != nil {
			return fmt.Errorf("execute %s template: %w", c.name, err)
		}
	}

	for _, b := range self.Boxes {
		if err := b.CompileTemplates(data); err != nil {
			return fmt.Errorf("compile mailbox templates: %s: %w", b.Mailbox, err)
		}
	}
	return nil
}

// Box stores all the necessary info needed to be passed in an
// IDLEEvent handler routine, in order to schedule commands and
// print informative messages
type Box struct {
	Mailbox           string             `yaml:"mailbox" validate:"required"`
	OnNewMail         *command.Templated `yaml:"onNewMail" validate:"omitnil,validateFn"`
	OnNewMailPost     *command.Templated `yaml:"onNewMailPost" validate:"omitnil,validateFn"`
	OnChangedMail     *command.Templated `yaml:"onChangedMail" validate:"omitnil,validateFn"`
	OnChangedMailPost *command.Templated `yaml:"onChangedMailPost" validate:"omitnil,validateFn"`
	OnDeletedMail     *command.Templated `yaml:"onDeletedMail" validate:"omitnil,validateFn"`
	OnDeletedMailPost *command.Templated `yaml:"onDeletedMailPost" validate:"omitnil,validateFn"`

	NotificationActions []*NotificationAction `yaml:"notificationActions" validate:"dive"`
}

type NotificationAction struct {
	Key       string   `yaml:"key"   validate:"required"`
	Label     string   `yaml:"label" validate:"required"`
	Exec      []string `yaml:"exec"  validate:"min=1,dive,required"`
	Close     bool     `yaml:"close"`
	CloseSame bool     `yaml:"closeSame"`
	CloseAll  bool     `yaml:"closeAll"`
}

func (self *Box) CompileTemplates(data any) error {
	cmds := [...]struct {
		name string
		t    *command.Templated
		data any
	}{
		{"onNewMail", self.OnNewMail, data},
		{"onNewMailPost", self.OnNewMailPost, data},
		{"onChangedMail", self.OnChangedMail, data},
		{"onChangedMailPost", self.OnChangedMailPost, data},
		{"onDeletedMail", self.OnDeletedMail, data},
		{"onDeletedMailPost", self.OnDeletedMailPost, data},
	}

	for _, c := range cmds {
		if c.t == nil {
			continue
		}
		if err := c.t.Compile(); err != nil {
			return fmt.Errorf("parse %s template: %w", c.name, err)
		}
		if _, err := c.t.Cmd(context.Background(), c.data); err != nil {
			return fmt.Errorf("execute %s template: %w", c.name, err)
		}
	}
	return nil
}
