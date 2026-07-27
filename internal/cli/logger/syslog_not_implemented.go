//go:build windows || plan9

package logger

import (
	"errors"
	"log/slog"
)

func HasSyslog() bool { return false }

func NewSyslogHandler(opts *slog.HandlerOptions) (slog.Handler, error) {
	return nil, errors.New(
		"logger: syslog package is not implemented on Windows",
	)
}
