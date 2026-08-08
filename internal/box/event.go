package box

import (
	"bytes"
	"io"
	"iter"
	"log/slog"
	"sync"
)

type EventType int

const (
	EventSync EventType = iota
	EventDeletedMail
	EventFlagChanged
	EventNewMail
	idleEvents

	StopWatching
	RestartWatching
)

func (self EventType) String() string {
	switch self {
	case EventSync:
		return "Synchronize mailboxes without post-steps"
	case EventDeletedMail:
		return "Deleted Email"
	case EventFlagChanged:
		return "Changed Flag on Email"
	case EventNewMail:
		return "New Email"
	default:
		return "Unknown Event"
	}
}

type IDLE struct {
	reason EventType
	box    *Box
}

func NewEvent(box *Box, reason EventType) *IDLE {
	return &IDLE{reason: reason, box: box}
}

func (self *IDLE) Reason() EventType { return self.reason }

func (self *IDLE) Box() *Box { return self.box }

func (self *IDLE) Alias() string { return self.box.Alias() }

func (self *IDLE) Mailbox() string { return self.box.Mailbox }

func (self *IDLE) Skip() bool {
	switch self.reason {
	case EventSync, EventNewMail:
		return self.box.SkipNewMail()
	case EventFlagChanged:
		return self.box.SkipChangedMail()
	case EventDeletedMail:
		return self.box.SkipDeletedMail()
	}
	return true
}

func (self *IDLE) OnReason() string {
	switch self.reason {
	case EventSync, EventNewMail:
		return "onNewMail"
	case EventDeletedMail:
		return "onDeletedMail"
	case EventFlagChanged:
		return "onChangedMail"
	}
	return "unknown reason"
}

func (self *IDLE) OnReasonPost() string {
	switch self.reason {
	case EventSync, EventNewMail:
		return "onNewMailPost"
	case EventDeletedMail:
		return "onDeletedMailPost"
	case EventFlagChanged:
		return "onChangedMailPost"
	}
	return "unknown reason"
}

type EventSet struct {
	events [idleEvents]*IDLE
	mu     sync.Mutex
}

func (self *EventSet) Add(e *IDLE) {
	self.mu.Lock()
	self.events[e.reason] = e
	self.mu.Unlock()
}

func (self *EventSet) Commands(l *slog.Logger) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		self.mu.Lock()
		events := self.events
		clear(self.events[:])
		self.mu.Unlock()

		seen := make(map[string]bool, len(events)*2)
		var b bytes.Buffer

	eventLoop:
		for _, e := range events {
			if e == nil {
				continue
			}

			renderers := [...]struct {
				onCommand func(io.Writer, *IDLE) error
				onReason  func() string
			}{
				{
					onCommand: e.Box().RenderCommandTo,
					onReason:  e.OnReason,
				},
				{
					onCommand: e.Box().RenderPostCommandTo,
					onReason:  e.OnReasonPost,
				},
			}

			for _, r := range renderers {
				if err := r.onCommand(&b, e); err != nil {
					yield("", err)
					return
				}

				cmd := b.String()
				b.Reset()
				if cmd == "" || cmd == "SKIP" {
					continue eventLoop
				}

				if seen[cmd] {
					l.Debug("skip duplicate command for this mailbox",
						slog.String("on", r.onReason()))
					continue
				}

				seen[cmd] = true
				if !yield(cmd, nil) {
					return
				}
			}
		}
	}
}
