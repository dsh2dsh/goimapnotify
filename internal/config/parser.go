package config

import (
	"fmt"
	"log/slog"
	"os"

	"go.yaml.in/yaml/v4"
)

func LoadYAML(filename string) (*Configuration, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("config: read %q: %w", filename, err)
	}

	cfg := &Configuration{
		StartupSync: true,
	}
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("config: parse yaml %q: %w", filename, err)
	}

	if cfg.Configurations == nil {
		var legacy ConfigurationLegacy
		if err := yaml.Unmarshal(b, &legacy); err != nil {
			return nil, fmt.Errorf("config: parse yaml in 'legacy' format %q: %w",
				filename, err)
		}
		slog.Info("legacy format configuration detected")
		cfg.Configurations = LegacyConverter(&legacy)
	}

	invalid := len(cfg.Configurations) == 0 ||
		(cfg.Configurations[0].Host == "" && cfg.Configurations[0].HostCMD == "")
	if invalid {
		return nil, fmt.Errorf(
			"configuration file %q is empty or have invalid configuration format",
			filename)
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
