package imap

import (
	"iter"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

func Mailboxes(c *client.Client) iter.Seq2[*imap.MailboxInfo, error] {
	// FIXME: This can be done better
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.List("", "*", mailboxes)
	}()

	return func(yield func(*imap.MailboxInfo, error) bool) {
		var stop bool
		for m := range mailboxes {
			if !stop && !yield(m, nil) {
				stop = true
			}
		}

		if err := <-done; err != nil && !stop {
			yield(nil, err)
		}
	}
}
