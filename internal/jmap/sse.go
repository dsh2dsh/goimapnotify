package jmap

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"time"

	"git.sr.ht/~rockorager/go-jmap"
)

func (self *WatchMailboxes) readEvents(ctx context.Context, r io.Reader,
	yield func(*jmap.StateChange, error) bool,
) {
	if self.pingInterval > 0 {
		defer self.startWatchdog(ctx)()
	}

	var b bytes.Buffer
	var eventType string
	var stateChange jmap.StateChange
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Bytes()

		// Handle blank line (dispatch event)
		if len(line) == 0 {
			if !self.dispatchEvent(eventType, &b, &stateChange, yield) {
				return
			}
			eventType = ""
			continue
		}

		// Handle comment line
		if line[0] == ':' {
			continue
		}

		// Process field line
		field, val, _ := bytes.Cut(line, []byte(":"))
		if len(val) == 0 {
			switch string(field) {
			case "event":
				eventType = ""
			case "data":
				b.WriteByte('\n')
			case "id":
				self.lastEventId = ""
			}
			continue
		}
		val = bytes.TrimSpace(val)

		switch string(field) {
		case "event":
			eventType = string(val)
		case "data":
			b.Write(val)
			b.WriteByte('\n')
		case "id":
			self.processIdField(val)
		case "retry":
			self.processRetryField(string(val))
		}
	}

	if err := scanner.Err(); err != nil {
		if !errors.Is(err, context.Canceled) {
			yield(nil, fmt.Errorf("read event: %w", err))
		}
	} else if b.Len() != 0 {
		yield(nil, fmt.Errorf("read event: %w", io.ErrUnexpectedEOF))
	}
}

func (self *WatchMailboxes) dispatchEvent(eventType string, b *bytes.Buffer,
	stateChange *jmap.StateChange, yield func(*jmap.StateChange, error) bool,
) bool {
	if b.Len() == 0 {
		return true
	}
	defer b.Reset()

	switch eventType {
	case "ping":
		self.processPing(b.Bytes())
		return true
	case "state":
	default:
		return true
	}

	if err := json.Unmarshal(b.Bytes(), stateChange); err != nil {
		yield(nil, fmt.Errorf("invalid state event: %w", err))
		return false
	}

	if !yield(stateChange, nil) {
		return false
	}
	clear(stateChange.Changed)
	return true
}

func (self *WatchMailboxes) processPing(b []byte) {
	l := self.logger()
	var ping struct {
		Interval uint32 `json:"interval"`
	}

	if err := json.Unmarshal(b, &ping); err != nil {
		l.Warn("invalid JMAP ping event", slog.Any("error", err))
		return
	}

	v := int(ping.Interval)
	changed := self.pingInterval != v
	self.pingInterval = v
	timeout := self.watchdogInterval()

	if changed {
		l.Info("JMAP ping interval changed",
			slog.Duration("interval", time.Duration(self.pingInterval)*time.Second),
			slog.Duration("timeout", timeout))
	}

	l.Debug("JMAP ping, reset watchdog",
		slog.Duration("interval", time.Duration(self.pingInterval)*time.Second),
		slog.Duration("timeout", timeout))
	self.watchdogTicker.Reset(timeout)
}

func (self *WatchMailboxes) processIdField(b []byte) {
	if b[0] != 0 {
		self.lastEventId = string(b)
	}
}

func (self *WatchMailboxes) processRetryField(val string) {
	ms, err := strconv.Atoi(val)
	if err != nil || ms <= 0 {
		return
	}

	self.retryDelay = time.Duration(ms) * time.Millisecond
	self.logger().Info("got retry event",
		slog.Duration("retryDelay", self.retryDelay))
}
