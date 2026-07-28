package imap

import "errors"

var (
	ErrLoginFailed     = errors.New("authentication failed")
	ErrMailboxNotFound = errors.New("select mailbox failed")
)
