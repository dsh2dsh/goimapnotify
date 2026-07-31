package box

type EventType int

const (
	NewMail EventType = iota + 1
	DeletedMail
	FlagChanged
	Sync
)

func (self EventType) String() string {
	switch self {
	case NewMail:
		return "New Email"
	case DeletedMail:
		return "Deleted Email"
	case FlagChanged:
		return "Changed Flag on Email"
	case Sync:
		return "Synchronize mailboxes without post-steps"
	default:
		return "Unknown Event"
	}
}

type IDLE struct {
	Reason EventType
	Box    *Box
}

func (self *IDLE) Alias() string { return self.Box.Alias() }

func (self *IDLE) Mailbox() string { return self.Box.Mailbox }

func (self *IDLE) Skip() bool {
	switch self.Reason {
	case Sync, NewMail:
		return self.Box.SkipNewMail()
	case FlagChanged:
		return self.Box.SkipChangedMail()
	case DeletedMail:
		return self.Box.SkipDeletedMail()
	}
	return true
}

func (self *IDLE) CommandName() string {
	switch self.Reason {
	case Sync, NewMail:
		return "onNewMail"
	case DeletedMail:
		return "onDeletedMail"
	case FlagChanged:
		return "onChangedMail"
	}
	return "unknown command"
}
