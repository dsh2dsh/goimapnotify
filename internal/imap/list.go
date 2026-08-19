package imap

import (
	"fmt"
	"iter"

	"github.com/emersion/go-imap/v2"

	"github.com/dsh2dsh/goimapnotify/internal/box"
	"github.com/dsh2dsh/goimapnotify/internal/config"
)

func Mailboxes(account *config.NotifyConfig, retries int,
) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		c, err := New(account, retries)
		if err != nil {
			yield("", fmt.Errorf("something went wrong creating IMAP client: %w", err))
			return
		}
		defer c.Close()

		listCmd := c.List("", "*", nil)

		for {
			m := listCmd.Next()
			if m == nil {
				break
			} else if specialUse(m) {
				continue
			}

			if !yield(m.Mailbox, nil) {
				_ = listCmd.Close()
				return
			}
		}

		if err := listCmd.Close(); err != nil {
			yield("", err)
		}
	}
}

func List(account *config.NotifyConfig, retries int) (*box.List, error) {
	c, err := New(account, retries)
	if err != nil {
		return nil, fmt.Errorf("something went wrong creating IMAP client: %w", err)
	}
	defer c.Close()

	listCmd := c.List("", "*", nil)
	listBoxes := new(box.List)

	for {
		m := listCmd.Next()
		if m == nil {
			break
		} else if specialUse(m) {
			continue
		}
		listBoxes.Delim = m.Delim
		listBoxes.Boxes = append(listBoxes.Boxes, m.Mailbox)
	}

	if err := listCmd.Close(); err != nil {
		return nil, fmt.Errorf("listing mailboxes finished with error: %w", err)
	}
	return listBoxes, nil
}

func specialUse(m *imap.ListData) bool {
	for _, attr := range m.Attrs {
		switch attr {
		case
			imap.MailboxAttrAll,
			imap.MailboxAttrArchive,
			imap.MailboxAttrDrafts,
			imap.MailboxAttrFlagged,
			imap.MailboxAttrJunk,
			imap.MailboxAttrNoSelect,
			imap.MailboxAttrSent,
			imap.MailboxAttrTrash:
			return true
		case "\\Memos", "\\Scheduled", "\\XNotes":
			return true
		}
	}
	return false
}
