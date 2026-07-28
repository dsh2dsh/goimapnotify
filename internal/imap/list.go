package imap

import (
	"iter"

	goimap "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
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
