package model

import (
	"context"
	"iter"
	"log/slog"
	"os/exec"
	"strings"
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
	case StopWatching:
		return "Stop Wathing Mailbox"
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

func (self *EventSet) Commands(ctx context.Context, l *slog.Logger,
) iter.Seq2[*exec.Cmd, error] {
	return func(yield func(*exec.Cmd, error) bool) {
		self.mu.Lock()
		events := self.events
		clear(self.events[:])
		self.mu.Unlock()

		seen := make(map[string]bool, len(events)*2)
	eventLoop:
		for _, e := range events {
			if e == nil {
				continue
			}

			cmds := [...]struct {
				cmd    func(context.Context, *IDLE) (*exec.Cmd, error)
				reason func() string
			}{
				{e.Box().Cmd, e.OnReason},
				{e.Box().PostCmd, e.OnReasonPost},
			}

			for _, c := range cmds {
				cmd, err := c.cmd(ctx, e)
				if err != nil {
					yield(nil, err)
					return
				} else if cmd == nil {
					continue eventLoop
				}

				s := strings.Join(cmd.Args, " ")
				if seen[s] {
					l.Debug("skip duplicate command for this mailbox",
						slog.String("on", c.reason()))
					continue
				}

				seen[s] = true
				if !yield(cmd, nil) {
					return
				}
			}
		}
	}
}
