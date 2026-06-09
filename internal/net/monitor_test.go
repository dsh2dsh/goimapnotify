package net

import (
	"net"
	"sync"
	"testing"
	"time"
)

func startListener(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start listener: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	return ln
}

func hostPortFromListener(ln net.Listener) HostPort {
	addr := ln.Addr().(*net.TCPAddr)
	return HostPort{Host: addr.IP.String(), Port: addr.Port}
}

func TestNetworkMonitor_DetectsDownTransition(t *testing.T) {
	ln := startListener(t)
	hp := hostPortFromListener(ln)

	eventCh := make(chan NetworkEvent, 10)
	quit := make(chan struct{})
	wg := &sync.WaitGroup{}

	m := NewNetworkMonitor([]HostPort{hp}, 50*time.Millisecond, time.Second, eventCh, quit)
	m.Start(wg)

	// Let a few successful polls happen
	time.Sleep(150 * time.Millisecond)

	// Kill the listener -> should trigger NetworkDown
	ln.Close()

	select {
	case ev := <-eventCh:
		if ev.State != NetworkDown {
			t.Fatalf("expected NetworkDown, got %v", ev.State)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for NetworkDown event")
	}

	close(quit)
	wg.Wait()
}

func TestNetworkMonitor_DetectsUpTransition(t *testing.T) {
	// Start with a listener, then kill it, then restart
	ln := startListener(t)
	hp := hostPortFromListener(ln)

	eventCh := make(chan NetworkEvent, 10)
	quit := make(chan struct{})
	wg := &sync.WaitGroup{}

	m := NewNetworkMonitor([]HostPort{hp}, 50*time.Millisecond, time.Second, eventCh, quit)
	m.Start(wg)

	// Let a successful poll happen
	time.Sleep(100 * time.Millisecond)

	// Kill listener
	ln.Close()

	// Wait for NetworkDown
	select {
	case ev := <-eventCh:
		if ev.State != NetworkDown {
			t.Fatalf("expected NetworkDown, got %v", ev.State)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for NetworkDown")
	}

	// Restart listener on same address
	ln2, err := net.Listen("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("failed to restart listener: %v", err)
	}
	defer ln2.Close()
	go func() {
		for {
			conn, err := ln2.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	// Wait for NetworkUp
	select {
	case ev := <-eventCh:
		if ev.State != NetworkUp {
			t.Fatalf("expected NetworkUp, got %v", ev.State)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for NetworkUp")
	}

	close(quit)
	wg.Wait()
}

func TestNetworkMonitor_NoDuplicateEvents(t *testing.T) {
	// Host that will never be reachable
	hp := HostPort{Host: "127.0.0.1", Port: 1} // port 1 should refuse

	eventCh := make(chan NetworkEvent, 10)
	quit := make(chan struct{})
	wg := &sync.WaitGroup{}

	m := NewNetworkMonitor([]HostPort{hp}, 50*time.Millisecond, 100*time.Millisecond, eventCh, quit)
	m.Start(wg)

	// Wait for first NetworkDown
	select {
	case ev := <-eventCh:
		if ev.State != NetworkDown {
			t.Fatalf("expected NetworkDown, got %v", ev.State)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for NetworkDown")
	}

	// Wait a few more poll cycles - should NOT get another event
	time.Sleep(200 * time.Millisecond)

	select {
	case ev := <-eventCh:
		t.Fatalf("unexpected duplicate event: %v", ev.State)
	default:
		// good - no duplicate
	}

	close(quit)
	wg.Wait()
}

func TestNetworkMonitor_QuitStopsMonitor(t *testing.T) {
	ln := startListener(t)
	defer ln.Close()
	hp := hostPortFromListener(ln)

	eventCh := make(chan NetworkEvent, 10)
	quit := make(chan struct{})
	wg := &sync.WaitGroup{}

	m := NewNetworkMonitor([]HostPort{hp}, 50*time.Millisecond, time.Second, eventCh, quit)
	m.Start(wg)

	time.Sleep(100 * time.Millisecond)
	close(quit)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("monitor did not stop after quit")
	}
}
