package logger

import (
	"log/slog"
	"os"

	"golang.org/x/term"
)

func InitializeDefaultLogger(logLevel slog.Level, syslog bool) error {
	opts := &slog.HandlerOptions{Level: logLevel}
	if syslog {
		h, err := NewSyslogHandler(opts)
		if err != nil {
			return err
		}
		slog.SetDefault(slog.New(h))
		return nil
	}

	logTime := !term.IsTerminal(int(os.Stderr.Fd()))
	if !logTime {
		opts.ReplaceAttr = hideTime
	}

	h := NewHumanTextHandler(os.Stderr, opts, logTime)
	slog.SetDefault(slog.New(h))
	return nil
}

func hideTime(groups []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey {
		return slog.Attr{}
	}
	return a
}
