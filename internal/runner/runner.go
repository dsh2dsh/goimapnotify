package runner

// This file is part of goimapnotify
// Copyright (C) 2017-2026	Jorge Javier Araya Navarro

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
	"time"

	"github.com/dsh2dsh/goimapnotify/internal/box"
)

type Runner struct {
	wait     time.Duration
	handlers map[*box.Box]*handler
}

func New(n int, wait time.Duration) *Runner {
	return &Runner{
		wait:     wait * time.Second,
		handlers: make(map[*box.Box]*handler, n),
	}
}

func (self *Runner) Schedule(e *box.IDLE, done <-chan struct{}) {
	if h, ok := self.handlers[e.Box]; ok {
		h.Schedule(e)
		return
	}

	h := NewHandler(e.Box, self.wait)
	self.handlers[e.Box] = h
	h.Schedule(e)
	go h.Run(done)
}
