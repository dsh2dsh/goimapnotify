package jmap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrLoginFailed = errors.New("authentication failed")
	ErrUnableWatch = errors.New("unable watch mailboxes")
)

type HttpError struct {
	StatusCode int
}

var _ error = (*HttpError)(nil)

func NewHttpError(statusCode int) *HttpError {
	return &HttpError{StatusCode: statusCode}
}

func (self *HttpError) Error() string {
	return fmt.Sprintf("http status: %d %s", self.StatusCode,
		http.StatusText(self.StatusCode))
}

func unableWatch(err error) bool {
	return errors.Is(err, ErrLoginFailed) ||
		errors.Is(err, ErrUnableWatch) ||
		errors.Is(err, context.Canceled)
}
