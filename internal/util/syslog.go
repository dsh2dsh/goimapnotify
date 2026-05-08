//go:build !windows

package util

import (
	"fmt"
	"io"
	"log/syslog"

	lSyslog "github.com/sirupsen/logrus/hooks/syslog"

	"github.com/sirupsen/logrus"
)

// EnableSyslog configures logrus to send logs to syslog and discard stderr output.
func EnableSyslog() error {
	hook, err := lSyslog.NewSyslogHook("", "", syslog.LOG_MAIL|syslog.LOG_INFO, "goimapnotify")
	if err != nil {
		return fmt.Errorf("failed to connect to syslog: %w", err)
	}

	logrus.AddHook(hook)
	logrus.SetOutput(io.Discard)

	return nil
}
