//go:build !windows && !plan9

package logger

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"log/syslog"
	"os"
	"path"
	"sync"
)

func HasSyslog() bool { return true }

type SyslogHandler struct {
	b *BytesBuffer
	w *syslog.Writer

	h    slog.Handler
	opts slog.HandlerOptions

	mu *sync.Mutex
}

var _ slog.Handler = (*SyslogHandler)(nil)

func NewSyslogHandler(opts *slog.HandlerOptions) (slog.Handler, error) {
	w, err := syslog.New(syslog.LOG_INFO|syslog.LOG_MAIL, path.Base(os.Args[0]))
	if err != nil {
		return nil, fmt.Errorf("logger: create syslog writer: %w", err)
	}

	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	self := &SyslogHandler{
		b:    NewBytesBuffer(),
		w:    w,
		opts: *opts,
		mu:   new(sync.Mutex),
	}
	return self.init(), nil
}

func (self *SyslogHandler) init() *SyslogHandler {
	opts := self.opts
	opts.ReplaceAttr = self.replace
	self.h = slog.NewTextHandler(self.b, &opts)
	return self
}

func (self *SyslogHandler) replace(groups []string, a slog.Attr) slog.Attr {
	if len(groups) == 0 {
		switch a.Key {
		case slog.TimeKey, slog.LevelKey, slog.MessageKey:
			return slog.Attr{}
		}
	}
	if self.opts.ReplaceAttr != nil {
		return self.opts.ReplaceAttr(groups, a)
	}
	return a
}

func (self *SyslogHandler) Enabled(ctx context.Context, level slog.Level,
) bool {
	return self.h.Enabled(ctx, level)
}

func (self *SyslogHandler) Handle(ctx context.Context, r slog.Record) error {
	self.lock()
	defer self.unlock()

	if err := self.formatStd(r); err != nil {
		return err
	}

	if err := self.h.Handle(ctx, r); err != nil {
		return fmt.Errorf("logger: failed slog handler: %w", err)
	}

	// Discard trailing '\n', added by slog.TextHandler, and trailing ' ' added by
	// formatStd.
	b := bytes.TrimSpace(self.b.Bytes())
	self.b.Truncate(len(b))

	self.b.WriteByte('\n')
	if err := self.write(r.Level); err != nil {
		return fmt.Errorf("logger: failed write formatted entry: %w", err)
	}
	return nil
}

func (self *SyslogHandler) lock() {
	self.mu.Lock()
	self.b.Alloc()
}

func (self *SyslogHandler) unlock() {
	self.b.Free()
	self.mu.Unlock()
}

func (self *SyslogHandler) formatStd(r slog.Record) error {
	self.b.WriteString(r.Level.String())
	self.b.WriteByte(' ')
	self.b.WriteString(r.Message)
	self.b.WriteByte(' ')
	return nil
}

func (self *SyslogHandler) write(level slog.Level) error {
	var fn func(m string) error
	switch level {
	case slog.LevelDebug:
		fn = self.w.Debug
	case slog.LevelInfo:
		fn = self.w.Info
	case slog.LevelWarn:
		fn = self.w.Warning
	default:
		fn = self.w.Err
	}

	if err := fn(self.b.String()); err != nil {
		return fmt.Errorf("logger: write to syslog: %w", err)
	}
	return nil
}

func (self *SyslogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h := *self
	h.h = self.h.WithAttrs(attrs)
	return &h
}

func (self *SyslogHandler) WithGroup(name string) slog.Handler {
	h := *self
	h.h = self.h.WithGroup(name)
	return &h
}
