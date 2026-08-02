package box

type EventType int

const (
	EventSync EventType = iota
	EventDeletedMail
	EventFlagChanged
	EventNewMail
)

func (self EventType) String() string {
	switch self {
	case EventNewMail:
		return "New Email"
	case EventDeletedMail:
		return "Deleted Email"
	case EventFlagChanged:
		return "Changed Flag on Email"
	case EventSync:
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
	case EventSync, EventNewMail:
		return self.Box.SkipNewMail()
	case EventFlagChanged:
		return self.Box.SkipChangedMail()
	case EventDeletedMail:
		return self.Box.SkipDeletedMail()
	}
	return true
}

func (self *IDLE) CommandName() string {
	switch self.Reason {
	case EventSync, EventNewMail:
		return "onNewMail"
	case EventDeletedMail:
		return "onDeletedMail"
	case EventFlagChanged:
		return "onChangedMail"
	}
	return "unknown command"
}
