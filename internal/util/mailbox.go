package util

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
	"fmt"
	"log/slog"
	"strings"

	imap "github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

// PrintDelimiter prints the hierarchy delimiter and returns mailbox count
func PrintDelimiter(c *client.Client) (int, error) {
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.List("", "*", mailboxes)
	}()

	i := 0
	m := <-mailboxes
	for range mailboxes {
		i += 1
	}
	if err := <-done; err != nil {
		return 0, err
	}

	fmt.Println("Hierarchy delimiter is:", m.Delimiter)
	return i, nil
}

// WalkMailbox recursively lists mailboxes with tree visualization
func WalkMailbox(c *client.Client, b string, l, mailboxCount int) error {
	// FIXME: This can be done better
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.List(b, "*", mailboxes)
	}()

	pos := 0
	for m := range mailboxes {
		box := boxchar(pos, l, mailboxCount)
		fmt.Println(box, m.Name)
		pos += 1
		// Check if mailbox has children mailboxes
		for _, attr := range m.Attributes {
			if attr == "\\Haschildren" {
				err := WalkMailbox(c, m.Name, l+1, mailboxCount)
				if err != nil {
					slog.Error("cannot keep walking mailboxes", slog.Any("error", err))
					return err
				}
				break
			}
		}
	}
	if err := <-done; err != nil {
		return err
	}
	return nil
}

func boxchar(p, l, b int) string {
	var drawthis string
	switch {
	case p == b || p == 0 && l > 0:
		drawthis = "└─"
	case p == 0 && p < b:
		drawthis = "┌─"
	case p > 0 && p < b:
		drawthis = "├─"
	case l > 0:
		drawthis = "│" + strings.Repeat(" ", l) + drawthis
	default:
		drawthis = "├─"
	}

	return drawthis
}
