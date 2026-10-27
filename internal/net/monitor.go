package net

// This file is part of goimapnotify
// Copyright (C) 2017-2026  Jorge Javier Araya Navarro

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
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// NetworkState represents the current network connectivity state
type NetworkState int

const (
	NetworkDown NetworkState = iota
	NetworkUp
)

// NetworkEvent is emitted when network state changes
type NetworkEvent struct {
	State NetworkState
}

// HostPort represents a connectivity check endpoint to probe
type HostPort struct {
	Host string
	Port int
}

// NetworkMonitor periodically probes connectivity check hosts and reports state changes
type NetworkMonitor struct {
	hosts    []HostPort
	interval time.Duration
	timeout  time.Duration
	eventCh  chan<- NetworkEvent
	quit     <-chan struct{}
}

// NewNetworkMonitor creates a new NetworkMonitor
func NewNetworkMonitor(
	hosts []HostPort,
	interval time.Duration,
	timeout time.Duration,
	eventCh chan<- NetworkEvent,
	quit <-chan struct{},
) *NetworkMonitor {
	return &NetworkMonitor{
		hosts:    hosts,
		interval: interval,
		timeout:  timeout,
		eventCh:  eventCh,
		quit:     quit,
	}
}

// Start launches the monitor goroutine
func (m *NetworkMonitor) Start(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.run()
	}()
}

// run is the main polling loop
func (m *NetworkMonitor) run() {
	wasUp := true // assume up at start since we just connected
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			isUp, total, reachable := m.probeAll()
			if wasUp && !isUp {
				logrus.Warnf(
					"Network connectivity lost, only %d out of %d host(s) reachable",
					reachable,
					total,
				)
				m.eventCh <- NetworkEvent{State: NetworkDown}
				wasUp = false
			} else if !wasUp && isUp {
				logrus.Info("Network restored, probe hosts are reachable again")
				m.eventCh <- NetworkEvent{State: NetworkUp}
				wasUp = true
			}
		case <-m.quit:
			return
		}
	}
}

// probeAll probes every configured host over TCP and returns whether
// a majority of them are reachable, the total number of hosts, and
// the number of hosts that were reachable.
func (m *NetworkMonitor) probeAll() (bool, int, int) {
	if len(m.hosts) == 0 {
		return true, 0, 0
	}

	reachable := 0
	for _, hp := range m.hosts {
		addr := net.JoinHostPort(hp.Host, fmt.Sprintf("%d", hp.Port))
		conn, err := net.DialTimeout("tcp", addr, m.timeout)
		if err != nil {
			logrus.WithField("host", addr).WithError(err).Debug("network probe failed")
			continue
		}
		conn.Close()
		reachable++
	}

	majority := len(m.hosts)/2 + 1
	return reachable >= majority, len(m.hosts), reachable
}
