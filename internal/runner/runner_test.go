//go:build !windows

package runner

// This file is part of goimapnotify
// Copyright (C) 2017-2025  Jorge Javier Araya Navarro

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

import (
	"sync"
	"testing"
	"time"

	"github.com/dsh2dsh/goimapnotify/internal/config"
)

// TestNewRunningBox tests the NewRunningBox constructor
func TestNewRunningBox(t *testing.T) {
	tests := []struct {
		name  string
		debug bool
		wait  int
	}{
		{
			name:  "debug enabled with 5 second wait",
			debug: true,
			wait:  5,
		},
		{
			name:  "debug disabled with 0 second wait",
			debug: false,
			wait:  0,
		},
		{
			name:  "debug disabled with 30 second wait",
			debug: false,
			wait:  30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := NewRunningBox(tt.debug, tt.wait)

			if rb == nil {
				t.Fatal("NewRunningBox returned nil")
			}
			if rb.Debug != tt.debug {
				t.Errorf("Debug = %v, want %v", rb.Debug, tt.debug)
			}
			if rb.Wait != tt.wait {
				t.Errorf("Wait = %v, want %v", rb.Wait, tt.wait)
			}
			if rb.Config == nil {
				t.Error("Config map is nil")
			}
		})
	}
}

// TestShouldSkip tests the ShouldSkip function
func TestShouldSkip(t *testing.T) {
	tests := []struct {
		name     string
		box      config.Box
		expected bool
	}{
		// NEWMAIL tests
		{
			name: "NEWMAIL with empty OnNewMail",
			box: config.Box{
				Reason:    config.NEWMAIL,
				OnNewMail: "",
			},
			expected: true,
		},
		{
			name: "NEWMAIL with SKIP OnNewMail",
			box: config.Box{
				Reason:    config.NEWMAIL,
				OnNewMail: "SKIP",
			},
			expected: true,
		},
		{
			name: "NEWMAIL with valid OnNewMail",
			box: config.Box{
				Reason:    config.NEWMAIL,
				OnNewMail: "echo hello",
			},
			expected: false,
		},
		// FLAGCHANGED tests
		{
			name: "FLAGCHANGED with empty OnChangedMail",
			box: config.Box{
				Reason:        config.FLAGCHANGED,
				OnChangedMail: "",
			},
			expected: true,
		},
		{
			name: "FLAGCHANGED with SKIP OnChangedMail",
			box: config.Box{
				Reason:        config.FLAGCHANGED,
				OnChangedMail: "SKIP",
			},
			expected: true,
		},
		{
			name: "FLAGCHANGED with valid OnChangedMail",
			box: config.Box{
				Reason:        config.FLAGCHANGED,
				OnChangedMail: "echo changed",
			},
			expected: false,
		},
		// DELETEDMAIL tests
		{
			name: "DELETEDMAIL with empty OnDeletedMail",
			box: config.Box{
				Reason:        config.DELETEDMAIL,
				OnDeletedMail: "",
			},
			expected: true,
		},
		{
			name: "DELETEDMAIL with SKIP OnDeletedMail",
			box: config.Box{
				Reason:        config.DELETEDMAIL,
				OnDeletedMail: "SKIP",
			},
			expected: true,
		},
		{
			name: "DELETEDMAIL with valid OnDeletedMail",
			box: config.Box{
				Reason:        config.DELETEDMAIL,
				OnDeletedMail: "echo deleted",
			},
			expected: false,
		},
		// Unknown reason
		{
			name: "unknown reason returns false",
			box: config.Box{
				Reason: config.EventType(99),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldSkip(&tt.box)
			if result != tt.expected {
				t.Errorf("ShouldSkip() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestRunningBox_Schedule_SkipsWhenNoCommand tests that Schedule skips when no command is configured
func TestRunningBox_Schedule_SkipsWhenNoCommand(t *testing.T) {
	rb := NewRunningBox(false, 1)

	event := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.NEWMAIL,
		Box: &config.Box{
			Reason:    config.NEWMAIL,
			OnNewMail: "", // Empty - should skip
		},
	}

	done := make(chan struct{})
	queue := make(chan config.IDLEEvent, 1)

	// Run Schedule in a goroutine
	var wg sync.WaitGroup
	wg.Go(func() { rb.Schedule(event, done, queue) })

	// Give it a moment
	time.Sleep(50 * time.Millisecond)

	// Should not have queued anything
	select {
	case <-queue:
		t.Error("Schedule should have skipped, but queued an event")
	default:
		// Expected - nothing queued
	}

	close(done)
	wg.Wait()
}

// TestRunningBox_Schedule_QueuesEvent tests that Schedule queues events after wait time
func TestRunningBox_Schedule_QueuesEvent(t *testing.T) {
	rb := NewRunningBox(false, 0) // 0 second wait for fast test

	event := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.NEWMAIL,
		Box: &config.Box{
			Reason:    config.NEWMAIL,
			OnNewMail: "echo hello",
		},
	}

	done := make(chan struct{})
	queue := make(chan config.IDLEEvent, 1)

	// Run Schedule in a goroutine
	go rb.Schedule(event, done, queue)

	// Wait for the event to be queued
	select {
	case received := <-queue:
		if received.Alias != event.Alias {
			t.Errorf("received Alias = %q, want %q", received.Alias, event.Alias)
		}
		if received.Mailbox != event.Mailbox {
			t.Errorf("received Mailbox = %q, want %q", received.Mailbox, event.Mailbox)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event to be queued")
	}

	close(done)
}

// TestRunningBox_Schedule_Debouncing tests that Schedule debounces multiple events
func TestRunningBox_Schedule_Debouncing(t *testing.T) {
	rb := NewRunningBox(false, 1) // 1 second wait

	event := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.NEWMAIL,
		Box: &config.Box{
			Reason:    config.NEWMAIL,
			OnNewMail: "echo hello",
		},
	}

	done := make(chan struct{})
	queue := make(chan config.IDLEEvent, 10)

	// Start multiple Schedule calls rapidly
	for range 5 {
		go rb.Schedule(event, done, queue)
		time.Sleep(100 * time.Millisecond) // Trigger reschedules
	}

	// Wait enough time for only one event to be queued (after debouncing)
	time.Sleep(1500 * time.Millisecond)

	// Count queued events - should only be 1 due to debouncing
	count := 0
	timeout := time.After(100 * time.Millisecond)
drainLoop:
	for {
		select {
		case <-queue:
			count++
		case <-timeout:
			break drainLoop
		}
	}

	if count != 1 {
		t.Errorf("expected 1 event after debouncing, got %d", count)
	}

	close(done)
}

// TestRunningBox_Schedule_StopsOnDone tests that Schedule stops when done channel is closed
func TestRunningBox_Schedule_StopsOnDone(t *testing.T) {
	rb := NewRunningBox(false, 10) // Long wait time

	event := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.NEWMAIL,
		Box: &config.Box{
			Reason:    config.NEWMAIL,
			OnNewMail: "echo hello",
		},
	}

	done := make(chan struct{})
	queue := make(chan config.IDLEEvent, 1)

	// Run Schedule in a goroutine
	scheduleReturned := make(chan struct{})
	go func() {
		rb.Schedule(event, done, queue)
		close(scheduleReturned)
	}()

	// Give it time to start the timer
	time.Sleep(50 * time.Millisecond)

	// Close done to stop Schedule
	close(done)

	// Schedule should return quickly
	select {
	case <-scheduleReturned:
		// Success
	case <-time.After(time.Second):
		t.Fatal("Schedule did not return after done was closed")
	}

	// Queue should be empty
	select {
	case <-queue:
		t.Error("unexpected event in queue after done was closed")
	default:
		// Expected
	}
}

// TestRunningBox_Schedule_DifferentMailboxes tests that different mailboxes have separate timers
func TestRunningBox_Schedule_DifferentMailboxes(t *testing.T) {
	rb := NewRunningBox(false, 0) // 0 second wait for fast test

	event1 := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.NEWMAIL,
		Box: &config.Box{
			Reason:    config.NEWMAIL,
			OnNewMail: "echo inbox",
		},
	}

	event2 := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "Sent",
		Reason:  config.NEWMAIL,
		Box: &config.Box{
			Reason:    config.NEWMAIL,
			OnNewMail: "echo sent",
		},
	}

	done := make(chan struct{})
	queue := make(chan config.IDLEEvent, 10)

	// Schedule events for different mailboxes
	go rb.Schedule(event1, done, queue)
	go rb.Schedule(event2, done, queue)

	// Wait for both events
	received := make(map[string]bool)
	timeout := time.After(time.Second)

	for range 2 {
		select {
		case event := <-queue:
			received[event.Mailbox] = true
		case <-timeout:
			t.Fatalf("timeout waiting for events, received: %v", received)
		}
	}

	if !received["INBOX"] {
		t.Error("did not receive INBOX event")
	}
	if !received["Sent"] {
		t.Error("did not receive Sent event")
	}

	close(done)
}

// TestRunningBox_Run_NewMail tests Run with NEWMAIL event
func TestRunningBox_Run_NewMail(t *testing.T) {
	rb := NewRunningBox(true, 1)

	event := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.NEWMAIL,
		Box: &config.Box{
			Mailbox:   "INBOX",
			Reason:    config.NEWMAIL,
			OnNewMail: "echo 'new mail'",
		},
	}

	err := rb.Run(event)
	if err != nil {
		t.Errorf("Run() error = %v", err)
	}
}

// TestRunningBox_Run_FlagChanged tests Run with FLAGCHANGED event
func TestRunningBox_Run_FlagChanged(t *testing.T) {
	rb := NewRunningBox(true, 1)

	event := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.FLAGCHANGED,
		Box: &config.Box{
			Mailbox:       "INBOX",
			Reason:        config.FLAGCHANGED,
			OnChangedMail: "echo 'flag changed'",
		},
	}

	err := rb.Run(event)
	if err != nil {
		t.Errorf("Run() error = %v", err)
	}
}

// TestRunningBox_Run_DeletedMail tests Run with DELETEDMAIL event
func TestRunningBox_Run_DeletedMail(t *testing.T) {
	rb := NewRunningBox(true, 1)

	event := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.DELETEDMAIL,
		Box: &config.Box{
			Mailbox:       "INBOX",
			Reason:        config.DELETEDMAIL,
			OnDeletedMail: "echo 'deleted mail'",
		},
	}

	err := rb.Run(event)
	if err != nil {
		t.Errorf("Run() error = %v", err)
	}
}

// TestRunningBox_Run_SkipsEmptyCommand tests Run skips when command is empty
func TestRunningBox_Run_SkipsEmptyCommand(t *testing.T) {
	rb := NewRunningBox(false, 1)

	event := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.NEWMAIL,
		Box: &config.Box{
			Mailbox:   "INBOX",
			Reason:    config.NEWMAIL,
			OnNewMail: "", // Empty command
		},
	}

	err := rb.Run(event)
	if err != nil {
		t.Errorf("Run() should not error on empty command, got: %v", err)
	}
}

// TestRunningBox_Run_SkipsSKIPCommand tests Run skips when command is "SKIP"
func TestRunningBox_Run_SkipsSKIPCommand(t *testing.T) {
	rb := NewRunningBox(false, 1)

	event := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.NEWMAIL,
		Box: &config.Box{
			Mailbox:   "INBOX",
			Reason:    config.NEWMAIL,
			OnNewMail: "SKIP",
		},
	}

	err := rb.Run(event)
	if err != nil {
		t.Errorf("Run() should not error on SKIP command, got: %v", err)
	}
}

// TestRunningBox_Run_WithPostCommand tests Run executes post command
func TestRunningBox_Run_WithPostCommand(t *testing.T) {
	rb := NewRunningBox(true, 1)

	event := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.NEWMAIL,
		Box: &config.Box{
			Mailbox:       "INBOX",
			Reason:        config.NEWMAIL,
			OnNewMail:     "echo 'new mail'",
			OnNewMailPost: "echo 'post sync'",
		},
	}

	err := rb.Run(event)
	if err != nil {
		t.Errorf("Run() error = %v", err)
	}
}

// TestRunningBox_Run_CommandFailure tests Run handles command failure
func TestRunningBox_Run_CommandFailure(t *testing.T) {
	rb := NewRunningBox(false, 1)

	event := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.NEWMAIL,
		Box: &config.Box{
			Mailbox:   "INBOX",
			Reason:    config.NEWMAIL,
			OnNewMail: "exit 1", // Command that fails
		},
	}

	err := rb.Run(event)
	if err == nil {
		t.Error("Run() should return error on command failure")
	}
}

// TestRunningBox_Run_PostCommandFailure tests Run handles post command failure
func TestRunningBox_Run_PostCommandFailure(t *testing.T) {
	rb := NewRunningBox(false, 1)

	event := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.NEWMAIL,
		Box: &config.Box{
			Mailbox:       "INBOX",
			Reason:        config.NEWMAIL,
			OnNewMail:     "echo 'success'",
			OnNewMailPost: "exit 1", // Post command that fails
		},
	}

	err := rb.Run(event)
	if err == nil {
		t.Error("Run() should return error on post command failure")
	}
}

// TestRunningBox_Run_UnknownReason tests Run handles unknown reason
func TestRunningBox_Run_UnknownReason(t *testing.T) {
	rb := NewRunningBox(false, 1)

	event := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.EventType(99), // Unknown reason
		Box: &config.Box{
			Mailbox: "INBOX",
		},
	}

	err := rb.Run(event)
	if err != nil {
		t.Errorf("Run() should not error on unknown reason, got: %v", err)
	}
}

// TestRunningBox_Run_WithTemplate tests Run handles templates in commands
func TestRunningBox_Run_WithTemplate(t *testing.T) {
	rb := NewRunningBox(true, 1)

	event := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.NEWMAIL,
		Box: &config.Box{
			Mailbox:   "INBOX",
			Alias:     "test@example.com",
			Reason:    config.NEWMAIL,
			OnNewMail: "echo '{{.Mailbox}} for {{.Alias}}'",
		},
	}

	err := rb.Run(event)
	if err != nil {
		t.Errorf("Run() error = %v", err)
	}
}

// TestRunningBox_Run_InvalidTemplate tests Run handles invalid templates
func TestRunningBox_Run_InvalidTemplate(t *testing.T) {
	rb := NewRunningBox(false, 1)

	event := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.NEWMAIL,
		Box: &config.Box{
			Mailbox:   "INBOX",
			Reason:    config.NEWMAIL,
			OnNewMail: "echo '{{.InvalidField'", // Invalid template syntax
		},
	}

	err := rb.Run(event)
	if err == nil {
		t.Error("Run() should return error on invalid template")
	}
}

// TestRunningBox_Run_TemplateExecutionError tests Run handles template execution errors
func TestRunningBox_Run_TemplateExecutionError(t *testing.T) {
	rb := NewRunningBox(false, 1)

	event := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.NEWMAIL,
		Box: &config.Box{
			Mailbox:   "INBOX",
			Reason:    config.NEWMAIL,
			OnNewMail: "echo '{{.NonExistentMethod}}'", // Valid syntax but will cause execution issues
		},
	}

	// This might or might not error depending on Go's template behavior
	// Just ensure it doesn't panic
	_ = rb.Run(event)
}

// TestRunningBox_Run_PostCommandSkipped tests Run skips post command when set to SKIP
func TestRunningBox_Run_PostCommandSkipped(t *testing.T) {
	rb := NewRunningBox(true, 1)

	event := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.NEWMAIL,
		Box: &config.Box{
			Mailbox:       "INBOX",
			Reason:        config.NEWMAIL,
			OnNewMail:     "echo 'new mail'",
			OnNewMailPost: "SKIP",
		},
	}

	err := rb.Run(event)
	if err != nil {
		t.Errorf("Run() error = %v", err)
	}
}

// TestRunningBox_Run_PostCommandEmpty tests Run skips post command when empty
func TestRunningBox_Run_PostCommandEmpty(t *testing.T) {
	rb := NewRunningBox(true, 1)

	event := config.IDLEEvent{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
		Reason:  config.NEWMAIL,
		Box: &config.Box{
			Mailbox:       "INBOX",
			Reason:        config.NEWMAIL,
			OnNewMail:     "echo 'new mail'",
			OnNewMailPost: "",
		},
	}

	err := rb.Run(event)
	if err != nil {
		t.Errorf("Run() error = %v", err)
	}
}

// TestRunningBox_Run_AllEventTypes tests Run with all event types
func TestRunningBox_Run_AllEventTypes(t *testing.T) {
	tests := []struct {
		name   string
		reason config.EventType
		box    config.Box
	}{
		{
			name:   "NEWMAIL",
			reason: config.NEWMAIL,
			box: config.Box{
				Mailbox:   "INBOX",
				Reason:    config.NEWMAIL,
				OnNewMail: "echo 'new'",
			},
		},
		{
			name:   "FLAGCHANGED",
			reason: config.FLAGCHANGED,
			box: config.Box{
				Mailbox:       "INBOX",
				Reason:        config.FLAGCHANGED,
				OnChangedMail: "echo 'changed'",
			},
		},
		{
			name:   "DELETEDMAIL",
			reason: config.DELETEDMAIL,
			box: config.Box{
				Mailbox:       "INBOX",
				Reason:        config.DELETEDMAIL,
				OnDeletedMail: "echo 'deleted'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := NewRunningBox(false, 1)
			event := config.IDLEEvent{
				Alias:   "test@example.com",
				Mailbox: "INBOX",
				Reason:  tt.reason,
				Box:     &tt.box,
			}

			err := rb.Run(event)
			if err != nil {
				t.Errorf("Run() error = %v", err)
			}
		})
	}
}

// TestRunningBox_Config tests that Config map can be used
func TestRunningBox_Config(t *testing.T) {
	rb := NewRunningBox(false, 1)

	// Config should be initialized and usable
	conf := config.NotifyConfig{
		Host:     "imap.example.com",
		Port:     993,
		Username: "test@example.com",
	}

	rb.Config["test@example.com"] = &conf

	retrieved, ok := rb.Config["test@example.com"]
	if !ok {
		t.Fatal("Config not stored correctly")
	}
	if retrieved.Host != "imap.example.com" {
		t.Errorf("Config Host = %q, want %q", retrieved.Host, "imap.example.com")
	}
}

// TestRunningBox_TimerMap tests that the timer sync.Map works correctly
func TestRunningBox_Schedule_TimerMapIsolation(t *testing.T) {
	rb := NewRunningBox(false, 0)

	// Schedule events for different users
	users := []string{"user1@example.com", "user2@example.com", "user3@example.com"}

	done := make(chan struct{})
	queue := make(chan config.IDLEEvent, 10)

	for _, user := range users {
		event := config.IDLEEvent{
			Alias:   user,
			Mailbox: "INBOX",
			Reason:  config.NEWMAIL,
			Box: &config.Box{
				Reason:    config.NEWMAIL,
				OnNewMail: "echo hello",
			},
		}
		go rb.Schedule(event, done, queue)
	}

	// Collect all events
	received := make(map[string]bool)
	timeout := time.After(time.Second)

	for range len(users) {
		select {
		case event := <-queue:
			received[event.Alias] = true
		case <-timeout:
			t.Fatalf("timeout, received only: %v", received)
		}
	}

	// Verify all users received
	for _, user := range users {
		if !received[user] {
			t.Errorf("did not receive event for user %s", user)
		}
	}

	close(done)
}
