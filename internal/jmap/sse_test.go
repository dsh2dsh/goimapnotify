package jmap

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_http2GoAwayNoError(t *testing.T) {
	assert.False(t, http2GoAwayNoError(nil), "with nil error")
	assert.False(t, http2GoAwayNoError(errors.New("oops")), "with some error")

	const errCodeError = `http2: server sent GOAWAY and closed the connection; LastStreamID=13, ErrCode=ERROR, debug=""`
	assert.False(t, http2GoAwayNoError(errors.New(errCodeError)),
		"ErrCode=ERROR")

	const errCodeNoError = `http2: server sent GOAWAY and closed the connection; LastStreamID=13, ErrCode=NO_ERROR, debug=""`
	assert.True(t, http2GoAwayNoError(errors.New(errCodeNoError)),
		"ErrCode=NO_ERROR")
}
