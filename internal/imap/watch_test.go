package imap

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
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"

	"github.com/dsh2dsh/goimapnotify/internal/config"
)

// MockWatchClient implements WatchClientInterface for testing
type MockWatchClient struct {
	*MoqWatchClient

	// Internal state
	updates      chan client.Update
	loggedOutCh  chan struct{}
	idleStopChan <-chan struct{}
	mu           sync.Mutex
}

func NewMockWatchClient() *MockWatchClient {
	return &MockWatchClient{
		MoqWatchClient: &MoqWatchClient{},
		loggedOutCh:    make(chan struct{}),
	}
}

func (m *MockWatchClient) LoggedOut() <-chan struct{} {
	if m.LoggedOutFunc != nil {
		return m.MoqWatchClient.LoggedOut()
	}
	m.MoqWatchClient.LoggedOut()

	m.mu.Lock()
	ch := m.loggedOutCh
	m.mu.Unlock()
	return ch
}

func (m *MockWatchClient) IdleWithFallback(stop <-chan struct{}, pollInterval time.Duration) error {
	m.mu.Lock()
	m.idleStopChan = stop
	m.mu.Unlock()

	if m.IdleWithFallbackFunc != nil {
		return m.MoqWatchClient.IdleWithFallback(stop, pollInterval)
	}
	m.MoqWatchClient.IdleWithFallback(stop, pollInterval)

	// Block until stop is closed
	<-stop
	return nil
}

func (m *MockWatchClient) SetUpdatesChannel(updates chan client.Update) {
	m.mu.Lock()
	m.updates = updates
	m.mu.Unlock()
	m.MoqWatchClient.SetUpdatesChannel(updates)
}

// SendUpdate sends an update through the updates channel
func (m *MockWatchClient) SendUpdate(update client.Update) {
	m.mu.Lock()
	if m.updates != nil {
		m.updates <- update
	}
	m.mu.Unlock()
}

// CloseLoggedOut closes the logged out channel to simulate disconnect
func (m *MockWatchClient) CloseLoggedOut() {
	m.mu.Lock()
	close(m.loggedOutCh)
	m.mu.Unlock()
}

// TestIDLEEvent tests the IDLEEvent struct
func TestIDLEEvent(t *testing.T) {
	event := IDLEEvent{
		Alias:         "test@example.com",
		Mailbox:       "INBOX",
		Reason:        config.NEWMAIL,
		ExistingEmail: 5,
		Box: config.Box{
			Mailbox: "INBOX",
		},
	}

	if event.Alias != "test@example.com" {
		t.Errorf("Alias = %q, want %q", event.Alias, "test@example.com")
	}
	if event.Mailbox != "INBOX" {
		t.Errorf("Mailbox = %q, want %q", event.Mailbox, "INBOX")
	}
	if event.Reason != config.NEWMAIL {
		t.Errorf("Reason = %v, want %v", event.Reason, config.NEWMAIL)
	}
	if event.ExistingEmail != 5 {
		t.Errorf("ExistingEmail = %d, want %d", event.ExistingEmail, 5)
	}
}

// TestBoxEvent tests the BoxEvent struct
func TestBoxEvent(t *testing.T) {
	box := config.Box{Mailbox: "INBOX", Alias: "test"}
	event := BoxEvent{
		UniqID:  "testINBOX",
		Mailbox: box,
	}

	if event.UniqID != "testINBOX" {
		t.Errorf("UniqID = %q, want %q", event.UniqID, "testINBOX")
	}
	if event.Mailbox.Mailbox != "INBOX" {
		t.Errorf("Mailbox.Mailbox = %q, want %q", event.Mailbox.Mailbox, "INBOX")
	}
}

// TestWatchMailBox_Watch_InitialSync tests that Watch sends an initial sync event
func TestWatchMailBox_Watch_InitialSync(t *testing.T) {
	mockClient := NewMockWatchClient()
	mockClient.SelectFunc = func(name string, readOnly bool) (*imap.MailboxStatus, error) {
		return &imap.MailboxStatus{Messages: 10}, nil
	}

	idleEvents := make(chan IDLEEvent, 10)
	boxEvents := make(chan BoxEvent, 10)
	quit := make(chan struct{})

	box := config.Box{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
	}

	var wg sync.WaitGroup
	NewWatchBoxWithClient(mockClient, config.NotifyConfig{}, box, idleEvents, boxEvents, quit, &wg)

	// Wait for the initial sync event
	select {
	case event := <-idleEvents:
		if event.Reason != config.SYNC {
			t.Errorf("Initial event Reason = %v, want %v", event.Reason, config.SYNC)
		}
		if event.Alias != "test@example.com" {
			t.Errorf("Initial event Alias = %q, want %q", event.Alias, "test@example.com")
		}
		if event.Mailbox != "INBOX" {
			t.Errorf("Initial event Mailbox = %q, want %q", event.Mailbox, "INBOX")
		}
		if event.ExistingEmail != 0 {
			t.Errorf("Initial event ExistingEmail = %d, want %d", event.ExistingEmail, 0)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for initial sync event")
	}

	// Cleanup
	close(quit)
	wg.Wait()
}

// TestWatchMailBox_Watch_NewMail tests handling of new mail events
func TestWatchMailBox_Watch_NewMail(t *testing.T) {
	mockClient := NewMockWatchClient()
	mockClient.SelectFunc = func(name string, readOnly bool) (*imap.MailboxStatus, error) {
		return &imap.MailboxStatus{Messages: 5}, nil
	}

	idleEvents := make(chan IDLEEvent, 10)
	boxEvents := make(chan BoxEvent, 10)
	quit := make(chan struct{})

	box := config.Box{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
	}

	var wg sync.WaitGroup
	NewWatchBoxWithClient(mockClient, config.NotifyConfig{}, box, idleEvents, boxEvents, quit, &wg)

	// Drain initial sync event
	select {
	case <-idleEvents:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for initial sync event")
	}

	// Give time for idle to start
	time.Sleep(50 * time.Millisecond)

	// Send a mailbox update with more messages
	mailboxUpdate := &client.MailboxUpdate{
		Mailbox: &imap.MailboxStatus{Messages: 8},
	}
	mockClient.SendUpdate(mailboxUpdate)

	// Wait for the new mail event
	select {
	case event := <-idleEvents:
		if event.Reason != config.NEWMAIL {
			t.Errorf("Event Reason = %v, want %v", event.Reason, config.NEWMAIL)
		}
		if event.ExistingEmail != 8 {
			t.Errorf("Event ExistingEmail = %d, want %d", event.ExistingEmail, 8)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for new mail event")
	}

	// Cleanup
	close(quit)
	wg.Wait()
}

// TestWatchMailBox_Watch_NoEventOnSameCount tests that no event is sent when message count doesn't increase
func TestWatchMailBox_Watch_NoEventOnSameCount(t *testing.T) {
	mockClient := NewMockWatchClient()
	mockClient.SelectFunc = func(name string, readOnly bool) (*imap.MailboxStatus, error) {
		return &imap.MailboxStatus{Messages: 5}, nil
	}

	idleEvents := make(chan IDLEEvent, 10)
	boxEvents := make(chan BoxEvent, 10)
	quit := make(chan struct{})

	box := config.Box{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
	}

	var wg sync.WaitGroup
	NewWatchBoxWithClient(mockClient, config.NotifyConfig{}, box, idleEvents, boxEvents, quit, &wg)

	// Drain initial sync event
	select {
	case <-idleEvents:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for initial sync event")
	}

	// Give time for idle to start
	time.Sleep(50 * time.Millisecond)

	// Send a mailbox update with same message count
	mailboxUpdate := &client.MailboxUpdate{
		Mailbox: &imap.MailboxStatus{Messages: 5},
	}
	mockClient.SendUpdate(mailboxUpdate)

	// Should not receive any event (same count)
	select {
	case event := <-idleEvents:
		t.Errorf("unexpected event received: %+v", event)
	case <-time.After(100 * time.Millisecond):
		// Expected - no event
	}

	// Cleanup
	close(quit)
	wg.Wait()
}

// TestWatchMailBox_Watch_FlagChanged tests handling of message flag changes
func TestWatchMailBox_Watch_FlagChanged(t *testing.T) {
	mockClient := NewMockWatchClient()
	mockClient.SelectFunc = func(name string, readOnly bool) (*imap.MailboxStatus, error) {
		return &imap.MailboxStatus{Messages: 5}, nil
	}

	idleEvents := make(chan IDLEEvent, 10)
	boxEvents := make(chan BoxEvent, 10)
	quit := make(chan struct{})

	box := config.Box{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
	}

	var wg sync.WaitGroup
	NewWatchBoxWithClient(mockClient, config.NotifyConfig{}, box, idleEvents, boxEvents, quit, &wg)

	// Drain initial sync event
	select {
	case <-idleEvents:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for initial sync event")
	}

	// Give time for idle to start
	time.Sleep(50 * time.Millisecond)

	// Send a message update (flag change)
	messageUpdate := &client.MessageUpdate{
		Message: &imap.Message{SeqNum: 1},
	}
	mockClient.SendUpdate(messageUpdate)

	// Wait for the flag changed event
	select {
	case event := <-idleEvents:
		if event.Reason != config.FLAGCHANGED {
			t.Errorf("Event Reason = %v, want %v", event.Reason, config.FLAGCHANGED)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for flag changed event")
	}

	// Cleanup
	close(quit)
	wg.Wait()
}

// TestWatchMailBox_Watch_DeletedMail tests handling of message deletion
func TestWatchMailBox_Watch_DeletedMail(t *testing.T) {
	mockClient := NewMockWatchClient()
	mockClient.SelectFunc = func(name string, readOnly bool) (*imap.MailboxStatus, error) {
		return &imap.MailboxStatus{Messages: 5}, nil
	}

	idleEvents := make(chan IDLEEvent, 10)
	boxEvents := make(chan BoxEvent, 10)
	quit := make(chan struct{})

	box := config.Box{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
	}

	var wg sync.WaitGroup
	NewWatchBoxWithClient(mockClient, config.NotifyConfig{}, box, idleEvents, boxEvents, quit, &wg)

	// Drain initial sync event
	select {
	case <-idleEvents:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for initial sync event")
	}

	// Give time for idle to start
	time.Sleep(50 * time.Millisecond)

	// Send an expunge update (message deleted)
	expungeUpdate := &client.ExpungeUpdate{
		SeqNum: 3,
	}
	mockClient.SendUpdate(expungeUpdate)

	// Wait for the deleted mail event
	select {
	case event := <-idleEvents:
		if event.Reason != config.DELETEDMAIL {
			t.Errorf("Event Reason = %v, want %v", event.Reason, config.DELETEDMAIL)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for deleted mail event")
	}

	// Cleanup
	close(quit)
	wg.Wait()
}

// TestWatchMailBox_Watch_Quit tests that Watch stops when quit channel is closed
func TestWatchMailBox_Watch_Quit(t *testing.T) {
	mockClient := NewMockWatchClient()
	mockClient.SelectFunc = func(name string, readOnly bool) (*imap.MailboxStatus, error) {
		return &imap.MailboxStatus{Messages: 0}, nil
	}

	idleEvents := make(chan IDLEEvent, 10)
	boxEvents := make(chan BoxEvent, 10)
	quit := make(chan struct{})

	box := config.Box{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
	}

	var wg sync.WaitGroup
	NewWatchBoxWithClient(mockClient, config.NotifyConfig{}, box, idleEvents, boxEvents, quit, &wg)

	// Drain initial sync event
	select {
	case <-idleEvents:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for initial sync event")
	}

	// Give time for idle to start
	time.Sleep(50 * time.Millisecond)

	// Close quit channel
	close(quit)

	// Wait for goroutine to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for Watch to stop")
	}
}

// TestWatchMailBox_Watch_LoggedOut tests that Watch handles server disconnect
func TestWatchMailBox_Watch_LoggedOut(t *testing.T) {
	mockClient := NewMockWatchClient()
	mockClient.SelectFunc = func(name string, readOnly bool) (*imap.MailboxStatus, error) {
		return &imap.MailboxStatus{Messages: 0}, nil
	}
	// Make IdleWithFallback block indefinitely
	mockClient.IdleWithFallbackFunc = func(stop <-chan struct{}, pollInterval time.Duration) error {
		<-stop
		return nil
	}

	idleEvents := make(chan IDLEEvent, 10)
	boxEvents := make(chan BoxEvent, 10)
	quit := make(chan struct{})

	box := config.Box{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
	}

	var wg sync.WaitGroup
	NewWatchBoxWithClient(mockClient, config.NotifyConfig{}, box, idleEvents, boxEvents, quit, &wg)

	// Drain initial sync event
	select {
	case <-idleEvents:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for initial sync event")
	}

	// Give time for idle to start
	time.Sleep(50 * time.Millisecond)

	// Simulate server disconnect
	mockClient.CloseLoggedOut()

	// Should receive a box event
	select {
	case event := <-boxEvents:
		expectedUniqID := "test@example.comINBOX"
		if event.UniqID != expectedUniqID {
			t.Errorf("BoxEvent UniqID = %q, want %q", event.UniqID, expectedUniqID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for box event")
	}

	// Cleanup
	close(quit)
	wg.Wait()
}

// TestWatchMailBox_Watch_IdleError tests that Watch handles IDLE errors
func TestWatchMailBox_Watch_IdleError(t *testing.T) {
	mockClient := NewMockWatchClient()
	mockClient.SelectFunc = func(name string, readOnly bool) (*imap.MailboxStatus, error) {
		return &imap.MailboxStatus{Messages: 0}, nil
	}
	// Make IdleWithFallback return an error after a short delay
	mockClient.IdleWithFallbackFunc = func(stop <-chan struct{}, pollInterval time.Duration) error {
		time.Sleep(50 * time.Millisecond)
		return errors.New("IDLE error")
	}

	idleEvents := make(chan IDLEEvent, 10)
	boxEvents := make(chan BoxEvent, 10)
	quit := make(chan struct{})

	box := config.Box{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
	}

	var wg sync.WaitGroup
	NewWatchBoxWithClient(mockClient, config.NotifyConfig{}, box, idleEvents, boxEvents, quit, &wg)

	// Drain initial sync event
	select {
	case <-idleEvents:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for initial sync event")
	}

	// Should receive a box event due to error
	select {
	case event := <-boxEvents:
		expectedUniqID := "test@example.comINBOX"
		if event.UniqID != expectedUniqID {
			t.Errorf("BoxEvent UniqID = %q, want %q", event.UniqID, expectedUniqID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for box event")
	}

	// Cleanup
	close(quit)
	wg.Wait()
}

// TestWatchMailBox_Watch_UnknownMailbox tests handling of unknown mailbox error
func TestWatchMailBox_Watch_UnknownMailbox(t *testing.T) {
	mockClient := NewMockWatchClient()
	mockClient.SelectFunc = func(name string, readOnly bool) (*imap.MailboxStatus, error) {
		return nil, errors.New("reason: Unknown Mailbox: NONEXISTENT")
	}

	idleEvents := make(chan IDLEEvent, 10)
	boxEvents := make(chan BoxEvent, 10)
	quit := make(chan struct{})

	box := config.Box{
		Alias:   "test@example.com",
		Mailbox: "NONEXISTENT",
	}

	var wg sync.WaitGroup
	NewWatchBoxWithClient(mockClient, config.NotifyConfig{}, box, idleEvents, boxEvents, quit, &wg)

	// Wait for goroutine to finish (should return early due to unknown mailbox)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - Watch returned early
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for Watch to return")
	}

	// Should not receive any events
	select {
	case event := <-idleEvents:
		t.Errorf("unexpected idle event: %+v", event)
	case event := <-boxEvents:
		t.Errorf("unexpected box event: %+v", event)
	default:
		// Expected - no events
	}
}

// TestWatchMailBox_Watch_SelectCallsCorrectly tests that Select is called with correct parameters
func TestWatchMailBox_Watch_SelectCallsCorrectly(t *testing.T) {
	mockClient := NewMockWatchClient()

	var selectName string
	var selectReadOnly bool
	var mu sync.Mutex
	mockClient.SelectFunc = func(name string, readOnly bool) (*imap.MailboxStatus, error) {
		mu.Lock()
		selectName = name
		selectReadOnly = readOnly
		mu.Unlock()
		return &imap.MailboxStatus{Messages: 0}, nil
	}

	idleEvents := make(chan IDLEEvent, 10)
	boxEvents := make(chan BoxEvent, 10)
	quit := make(chan struct{})

	box := config.Box{
		Alias:   "test@example.com",
		Mailbox: "Sent",
	}

	var wg sync.WaitGroup
	NewWatchBoxWithClient(mockClient, config.NotifyConfig{}, box, idleEvents, boxEvents, quit, &wg)

	// Give time for Select to be called
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	name := selectName
	readOnly := selectReadOnly
	mu.Unlock()
	if name != "Sent" {
		t.Errorf("Select name = %q, want %q", name, "Sent")
	}
	if !readOnly {
		t.Error("Select readOnly = false, want true")
	}

	// Cleanup
	close(quit)
	wg.Wait()
}

// TestWatchMailBox_Watch_MessageCountDecrease tests that message count decrease doesn't trigger new mail event
func TestWatchMailBox_Watch_MessageCountDecrease(t *testing.T) {
	mockClient := NewMockWatchClient()
	mockClient.SelectFunc = func(name string, readOnly bool) (*imap.MailboxStatus, error) {
		return &imap.MailboxStatus{Messages: 10}, nil
	}

	idleEvents := make(chan IDLEEvent, 10)
	boxEvents := make(chan BoxEvent, 10)
	quit := make(chan struct{})

	box := config.Box{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
	}

	var wg sync.WaitGroup
	NewWatchBoxWithClient(mockClient, config.NotifyConfig{}, box, idleEvents, boxEvents, quit, &wg)

	// Drain initial sync event
	select {
	case <-idleEvents:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for initial sync event")
	}

	// Give time for idle to start
	time.Sleep(50 * time.Millisecond)

	// Send a mailbox update with fewer messages (e.g., after deletion sync)
	mailboxUpdate := &client.MailboxUpdate{
		Mailbox: &imap.MailboxStatus{Messages: 8},
	}
	mockClient.SendUpdate(mailboxUpdate)

	// Should not receive a new mail event (message count decreased)
	select {
	case event := <-idleEvents:
		if event.Reason == config.NEWMAIL {
			t.Error("unexpected NEWMAIL event for message count decrease")
		}
	case <-time.After(100 * time.Millisecond):
		// Expected - no new mail event
	}

	// Cleanup
	close(quit)
	wg.Wait()
}

// TestNewWatchBoxWithClient_WaitGroupManagement tests that WaitGroup is properly managed
func TestNewWatchBoxWithClient_WaitGroupManagement(t *testing.T) {
	mockClient := NewMockWatchClient()
	mockClient.SelectFunc = func(name string, readOnly bool) (*imap.MailboxStatus, error) {
		return &imap.MailboxStatus{Messages: 0}, nil
	}

	idleEvents := make(chan IDLEEvent, 10)
	boxEvents := make(chan BoxEvent, 10)
	quit := make(chan struct{})

	box := config.Box{
		Alias:   "test@example.com",
		Mailbox: "INBOX",
	}

	var wg sync.WaitGroup

	// Create multiple watchers
	for range 3 {
		NewWatchBoxWithClient(
			mockClient,
			config.NotifyConfig{},
			box,
			idleEvents,
			boxEvents,
			quit,
			&wg,
		)
	}

	// Give time for all to start
	time.Sleep(100 * time.Millisecond)

	// Close quit to stop all watchers
	close(quit)

	// Wait should complete without hanging
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for all watchers to stop")
	}
}

// TestMockWatchClient_ImplementsInterface verifies the mock implements the interface
func TestMockWatchClient_ImplementsInterface(t *testing.T) {
	var _ WatchClientInterface = &MockWatchClient{}
}

// TestIMAPIDLEClientWrapper_ImplementsInterface verifies the wrapper implements the interface
func TestIMAPIDLEClientWrapper_ImplementsInterface(t *testing.T) {
	var _ WatchClientInterface = &IMAPIDLEClientWrapper{}
}
