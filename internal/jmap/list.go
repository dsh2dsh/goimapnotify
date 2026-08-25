package jmap

import (
	"context"
	"fmt"
	"iter"

	"github.com/dsh2dsh/goimapnotify/internal/box"
	"github.com/dsh2dsh/goimapnotify/internal/config"
)

func List(ctx context.Context, account *config.NotifyConfig, retries int,
) (*box.List, error) {
	c, err := New(ctx, account, retries)
	if err != nil {
		return nil, fmt.Errorf("something went wrong creating JMAP client: %w", err)
	}

	mailboxes, err := c.Mailboxes(ctx)
	if err != nil {
		return nil, err
	}

	listBoxes := &box.List{
		Delim: '/',
		Boxes: make([]string, 0, mailboxes.Len()),
	}

	for m := range mailboxes.All() {
		listBoxes.Boxes = append(listBoxes.Boxes, m.path)
	}
	return listBoxes, nil
}

func Mailboxes(ctx context.Context, account *config.NotifyConfig, retries int,
) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		c, err := New(ctx, account, retries)
		if err != nil {
			yield("", fmt.Errorf("something went wrong creating JMAP client: %w", err))
			return
		}

		mailboxes, err := c.Mailboxes(ctx)
		if err != nil {
			yield("", err)
			return
		}

		for m := range mailboxes.All() {
			if !yield(m.path, nil) {
				return
			}
		}
	}
}
