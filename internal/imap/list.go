package imap

import (
	"fmt"
	"iter"

	goimap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"github.com/dsh2dsh/goimapnotify/internal/box"
	"github.com/dsh2dsh/goimapnotify/internal/config"
)

func Mailboxes(c *imapclient.Client) iter.Seq2[*goimap.ListData, error] {
	return func(yield func(*goimap.ListData, error) bool) {
		listCmd := c.List("", "*", nil)
		for {
			m := listCmd.Next()
			if m == nil {
				break
			} else if !yield(m, nil) {
				_ = listCmd.Close()
				return
			}
		}

		if err := listCmd.Close(); err != nil {
			yield(nil, err)
		}
	}
}

func List(account *config.NotifyConfig, retries int) (*box.List, error) {
	c, err := New(account, retries)
	if err != nil {
		return nil, fmt.Errorf(
			"something went wrong creating IMAP client, account=%s: %w",
			account.Alias, err)
	}
	defer c.Close()

	listCmd := c.List("", "*", nil)
	listBoxes := new(box.List)

	for {
		m := listCmd.Next()
		if m == nil {
			break
		}
		listBoxes.Delim = m.Delim
		listBoxes.Boxes = append(listBoxes.Boxes, m.Mailbox)
	}

	if err := listCmd.Close(); err != nil {
		return nil, fmt.Errorf(
			"listing mailboxes finished with error, account=%s: %w",
			account.Alias, err)
	}
	return listBoxes, nil
}
