package jmap

import (
	"cmp"
	"context"
	"fmt"
	"iter"
	"path"
	"slices"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"

	"github.com/dsh2dsh/goimapnotify/internal/box"
	"github.com/dsh2dsh/goimapnotify/internal/config"
)

func List(ctx context.Context, account *config.NotifyConfig,
) (*box.List, error) {
	c, err := New(ctx, account)
	if err != nil {
		return nil, fmt.Errorf("something went wrong creating JMAP client: %w", err)
	}

	mailboxes, err := list(ctx, c)
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

func list(ctx context.Context, c *jmap.Client) (*mailboxes, error) {
	req := jmap.Request{Context: ctx}
	req.Invoke(&mailbox.Get{Account: c.Session.PrimaryAccounts[mail.URI]})
	resp, err := c.Do(&req)
	if err != nil {
		return nil, fmt.Errorf("do JMAP request: %w", err)
	}

	r, ok := resp.Responses[0].Args.(*mailbox.GetResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected JMAP response %T",
			resp.Responses[0].Args)
	}
	return NewMailboxes(r), nil
}

func Mailboxes(ctx context.Context, account *config.NotifyConfig,
) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		c, err := New(ctx, account)
		if err != nil {
			yield("", fmt.Errorf("something went wrong creating JMAP client: %w", err))
			return
		}

		mailboxes, err := list(ctx, c)
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

type mailboxes struct {
	mailboxes map[jmap.ID]*Mailbox
	tree      []*Mailbox
}

type Mailbox struct {
	*mailbox.Mailbox

	path     string
	children []*Mailbox
}

func NewMailboxes(r *mailbox.GetResponse) *mailboxes {
	return (&mailboxes{
		mailboxes: make(map[jmap.ID]*Mailbox, len(r.List)),
		tree:      make([]*Mailbox, 0, len(r.List)),
	}).init(r)
}

func (self *mailboxes) init(r *mailbox.GetResponse) *mailboxes {
	for _, mbox := range r.List {
		m := &Mailbox{Mailbox: mbox}
		self.mailboxes[mbox.ID] = m

		if parent := self.mailboxes[mbox.ParentID]; parent != nil {
			m.path = path.Join(parent.path, mbox.Name)
			parent.children = append(parent.children, m)
		} else {
			m.path = mbox.Name
			self.tree = append(self.tree, m)
		}
	}
	return self.sort()
}

func (self *mailboxes) sort() *mailboxes {
	fn := func(a, b *Mailbox) int { return cmp.Compare(a.SortOrder, b.SortOrder) }
	slices.SortFunc(self.tree, fn)

	for m := range self.All() {
		slices.SortFunc(m.children, fn)
	}
	return self
}

func (self *mailboxes) All() iter.Seq[*Mailbox] {
	return func(yield func(*Mailbox) bool) {
		for _, m := range self.tree {
			switch m.Role {
			case
				mailbox.RoleAll,
				mailbox.RoleArchive,
				mailbox.RoleDrafts,
				mailbox.RoleFlagged,
				mailbox.RoleJunk,
				mailbox.RoleNoSelect,
				mailbox.RoleSent,
				mailbox.RoleTrash:
				continue
			case "memos", "scheduled":
				continue
			}

			if !self.every(m, yield) {
				return
			}
		}
	}
}

func (self *mailboxes) every(m *Mailbox, yield func(*Mailbox) bool) bool {
	if !yield(m) {
		return false
	}

	for _, child := range m.children {
		if !self.every(child, yield) {
			return false
		}
	}
	return true
}

func (self *mailboxes) Len() int { return len(self.mailboxes) }
