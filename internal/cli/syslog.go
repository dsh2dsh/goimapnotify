package cli

import (
	"fmt"
	"io"
	"log/syslog"

	"github.com/sirupsen/logrus"
	lSyslog "github.com/sirupsen/logrus/hooks/syslog"
)

// enableSyslog configures logrus to send logs to syslog and discard stderr
// output.
func enableSyslog() error {
	hook, err := lSyslog.NewSyslogHook("", "", syslog.LOG_MAIL|syslog.LOG_INFO,
		"goimapnotify")
	if err != nil {
		return fmt.Errorf("failed to connect to syslog: %w", err)
	}

	logrus.AddHook(hook)
	logrus.SetOutput(io.Discard)
	return nil
}
