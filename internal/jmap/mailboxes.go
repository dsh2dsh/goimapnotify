package jmap

import (
	"cmp"
	"iter"
	"path"
	"slices"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail/email"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"

	"github.com/dsh2dsh/goimapnotify/internal/box"
)

type Mailbox struct {
	*mailbox.Mailbox

	path     string
	children []*Mailbox
	watching *box.Box
}

func (self *Mailbox) Path() string { return self.path }

func (self *Mailbox) Watching() *box.Box { return self.watching }

type mailboxes struct {
	mailboxes map[jmap.ID]*Mailbox
	paths     map[string]*Mailbox
	tree      []*Mailbox

	watching []*Mailbox
	emails   map[jmap.ID]*email.Email
}

func NewMailboxes(r *mailbox.GetResponse) *mailboxes {
	return (&mailboxes{
		mailboxes: make(map[jmap.ID]*Mailbox, len(r.List)),
		paths:     make(map[string]*Mailbox, len(r.List)),
		tree:      make([]*Mailbox, 0, len(r.List)),
		emails:    make(map[jmap.ID]*email.Email),
	}).init(r)
}

func (self *mailboxes) init(r *mailbox.GetResponse) *mailboxes {
	for _, mbox := range r.List {
		m := &Mailbox{Mailbox: mbox}
		self.mailboxes[mbox.ID] = m

		if parent := self.mailboxes[mbox.ParentID]; parent != nil {
			m.path = path.Join(parent.path, mbox.Name)
			self.paths[m.path] = m
			parent.children = append(parent.children, m)
		} else {
			m.path = mbox.Name
			self.paths[m.path] = m
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

func (self *mailboxes) Watch(b *box.Box) *Mailbox {
	m := self.paths[b.Mailbox]
	if m == nil {
		return nil
	}

	m.watching = b
	self.watching = append(self.watching, m)
	return m
}

func (self *mailboxes) Mailbox(id jmap.ID) *Mailbox {
	return self.mailboxes[id]
}

func (self *mailboxes) Watching() []*Mailbox { return self.watching }

func (self *mailboxes) QueryFilter() email.Filter {
	if len(self.watching) == 1 {
		return &email.FilterCondition{InMailbox: self.watching[0].ID}
	}

	conds := make([]email.Filter, len(self.watching))
	for i, m := range self.watching {
		conds[i] = &email.FilterCondition{InMailbox: m.ID}
	}

	return &email.FilterOperator{
		Operator:   jmap.OperatorOR,
		Conditions: conds,
	}
}

func (self *mailboxes) AddEmail(m *email.Email) int {
	for id := range m.MailboxIDs {
		mb := self.mailboxes[id]
		if mb != nil && mb.Watching() != nil {
			self.emails[m.ID] = m
			break
		}
	}
	return len(self.emails)
}

func (self *mailboxes) AddEmails(list []*email.Email) int {
	for _, m := range list {
		self.emails[m.ID] = m
	}
	return len(self.emails)
}

func (self *mailboxes) ClearEmails() { clear(self.emails) }

func (self *mailboxes) Email(id jmap.ID) *email.Email { return self.emails[id] }

func (self *mailboxes) DeleteEmail(id jmap.ID) int {
	delete(self.emails, id)
	return len(self.emails)
}
