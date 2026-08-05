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

	maxEvents
)

func (self EventType) String() string {
	switch self {
	case EventNewMail:
		return "New Email"
	case EventDeletedMail:
		return "Deleted Email"
	case EventFlagChanged:
		return "Changed Flag on Email"
	case EventSync:
		return "Synchronize mailboxes without post-steps"
	default:
		return "Unknown Event"
	}
}

type IDLE struct {
	Reason EventType
	Box    *Box
}

func (self *IDLE) Alias() string { return self.Box.Alias() }

func (self *IDLE) Mailbox() string { return self.Box.Mailbox }

func (self *IDLE) Skip() bool {
	switch self.Reason {
	case EventSync, EventNewMail:
		return self.Box.SkipNewMail()
	case EventFlagChanged:
		return self.Box.SkipChangedMail()
	case EventDeletedMail:
		return self.Box.SkipDeletedMail()
	}
	return true
}

func (self *IDLE) OnReason() string {
	switch self.Reason {
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
	switch self.Reason {
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
	events [maxEvents]*IDLE
	mu     sync.Mutex
}

func (self *EventSet) Add(e *IDLE) {
	self.mu.Lock()
	self.events[e.Reason] = e
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
					onCommand: e.Box.RenderCommandTo,
					onReason:  e.OnReason,
				},
				{
					onCommand: e.Box.RenderPostCommandTo,
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
