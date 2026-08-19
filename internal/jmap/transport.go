package jmap

import (
	"fmt"
	"net/http"
)

var Version = "dev"

type httpTransport struct {
	http.RoundTripper

	ua string
}

var _ http.RoundTripper = (*httpTransport)(nil)

func (self *httpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", self.getUA())

	resp, err := self.RoundTripper.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("http transport for JMAP: %w", err)
	}
	return resp, nil
}

func (self *httpTransport) getUA() string {
	if self.ua == "" {
		self.ua = "goimapnotify/dsh2dsh-" + Version +
			" (https://github.com/dsh2dsh/goimapnotify)"
	}
	return self.ua
}
