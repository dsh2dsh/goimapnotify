package command

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
	"context"
	"log/slog"
	"os/exec"
	"slices"
)

func New(command string) *exec.Cmd {
	return NewContext(context.Background(), command)
}

func NewContext(ctx context.Context, command string) *exec.Cmd {
	args := slices.Concat(Shell, []string{command})
	slog.Debug("prepare command for exec",
		slog.String("shell", args[0]), slog.Any("args", args[1:]))
	return exec.CommandContext(ctx, args[0], args[1:]...)
}
