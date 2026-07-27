package config

// EventType represents the type of IMAP event
type EventType int

const (
	NEWMAIL EventType = iota + 1
	DELETEDMAIL
	FLAGCHANGED
	SYNC
)

func (e EventType) String() string {
	switch e {
	case NEWMAIL:
		return "New Email"
	case DELETEDMAIL:
		return "Deleted Email"
	case FLAGCHANGED:
		return "Changed Flag on Email"
	case SYNC:
		return "Synchronize mailboxes without post-steps"
	default:
		return "Unknown Event"
	}
}

// IDLEEvent models an IDLE event
type IDLEEvent struct {
	Alias         string
	Mailbox       string
	Reason        EventType
	ExistingEmail int
	Box           Box
}
