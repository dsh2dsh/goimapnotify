package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.yaml.in/yaml/v4"
)

func LoadYAML(filename string) (*Configuration, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("config: read %q: %w", filename, err)
	}

	cfg, err := LoadBytes(b)
	if err != nil {
		return nil, fmt.Errorf("parse configuration: %s: %w", filename, err)
	}
	return cfg, nil
}

func LoadBytes(b []byte) (*Configuration, error) {
	cfg := &Configuration{
		MaxDelay:    5 * time.Minute,
		StartupSync: true,
		DesktopNotify: DesktopNotification{
			AppName:       "goimapnotify",
			ActionTimeout: 10 * time.Second,
			NewMail: NotificationTemplate{
				Summary: "{{ .Mailbox }} ({{ .Count }})",
				Body:    "<i>{{ .Authors }}</i>\n{{ .Subject }}",
			},
		},
	}
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	if cfg.Configurations == nil {
		var legacy ConfigurationLegacy
		if err := yaml.Unmarshal(b, &legacy); err != nil {
			return nil, fmt.Errorf("parse yaml in 'legacy' format: %w", err)
		}
		slog.Info("legacy format configuration detected")
		cfg.Configurations = LegacyConverter(&legacy)
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LegacyConverter converts old format configuration to new format
func LegacyConverter(legacy *ConfigurationLegacy) []*NotifyConfig {
	c := &NotifyConfig{
		Host:              legacy.Host,
		HostCMD:           legacy.HostCMD,
		Port:              legacy.Port,
		TLS:               legacy.TLS,
		TLSOptions:        legacy.TLSOptions,
		Username:          legacy.Username,
		UsernameCMD:       legacy.UsernameCMD,
		Password:          legacy.Password,
		PasswordCMD:       legacy.PasswordCMD,
		XOAuth2:           legacy.XOAuth2,
		OnNewMail:         legacy.OnNewMail,
		OnNewMailPost:     legacy.OnNewMailPost,
		OnChangedMail:     legacy.OnChangedMail,
		OnChangedMailPost: legacy.OnChangedMailPost,
		OnDeletedMail:     legacy.OnDeletedMail,
		OnDeletedMailPost: legacy.OnDeletedMailPost,
		IDLELogoutTimeout: legacy.IDLELogoutTimeout,
		EnableIDCommand:   legacy.EnableIDCommand,
	}

	c.Boxes = make([]*Box, len(legacy.Boxes))
	for i, mailbox := range legacy.Boxes {
		c.Boxes[i] = &Box{Mailbox: mailbox}
	}
	return []*NotifyConfig{c}
}
