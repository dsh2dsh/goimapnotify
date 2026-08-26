package jmap

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"git.sr.ht/~rockorager/go-jmap"
	"git.sr.ht/~rockorager/go-jmap/mail"
	"git.sr.ht/~rockorager/go-jmap/mail/mailbox"
	"golang.org/x/oauth2"

	"github.com/dsh2dsh/goimapnotify/internal/config"
)

const timeout = 20 * time.Second

var httpClient = &http.Client{
	Transport: &httpTransport{RoundTripper: http.DefaultTransport},
}

type client struct {
	*jmap.Client

	username string
	timeout  time.Duration
}

func New(ctx context.Context, conf *config.NotifyConfig, retries int,
) (_ *client, err error) {
	c := (&client{
		Client:   &jmap.Client{SessionEndpoint: conf.Host},
		username: conf.Username,
		timeout:  timeout,
	}).withAccessToken(ctx, conf.Password)

	if err := c.retryAuth(ctx, retries); err != nil {
		return nil, err
	}
	return c, nil
}

func (self *client) withAccessToken(ctx context.Context, token string) *client {
	t := &oauth2.Token{AccessToken: token, TokenType: "bearer"}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	self.HttpClient = oauth2.NewClient(ctx,
		new(oauth2.Config).TokenSource(ctx, t))
	return self
}

func (self *client) retryAuth(ctx context.Context, retries int) (err error) {
authLoop:
	for wait := range retries {
		switch err = self.authenticate(ctx); {
		case err == nil:
			return nil
		case wait == retries-1, ctx.Err() != nil, unableWatch(err):
			break authLoop
		}

		d := time.Second * time.Duration(wait+1)
		slog.Error(
			"there was an error while connecting to host, waiting to try again",
			slog.Duration("wait", d),
			slog.String("endpoint", self.SessionEndpoint),
			slog.Any("error", err))

		if !timeAfter(ctx, d) {
			err = ctx.Err()
			break
		}
	}
	return fmt.Errorf("jmap authentication: %w", err)
}

func (self *client) authenticate(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, self.timeout)
	defer cancel()

	if self.SessionEndpoint == "" {
		s, err := autoDiscovery(ctx, self.username)
		if err != nil {
			return fmt.Errorf("%w: autodiscovery session endpoint: %w",
				ErrUnableWatch, err)
		}
		self.SessionEndpoint = s
	}

	if self.SessionEndpoint == "" {
		return fmt.Errorf("%w: session endpoint not configured", ErrUnableWatch)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		self.SessionEndpoint, nil)
	if err != nil {
		return fmt.Errorf("building auth request: %w", err)
	}

	resp, err := self.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching jmap session resource: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return fmt.Errorf("%w: %w", ErrLoginFailed, NewHttpError(resp.StatusCode))
	default:
		return NewHttpError(resp.StatusCode)
	}

	s := new(jmap.Session)
	if err := json.UnmarshalRead(resp.Body, s); err != nil {
		return fmt.Errorf("unmarshaling jmap session resource: %w", err)
	}
	self.Session = s
	return nil
}

func (self *client) Do(ctx context.Context, req *jmap.Request,
) (*jmap.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, self.timeout)
	defer cancel()

	req.Context = ctx
	resp, err := self.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jmap do request: %w", err)
	} else if len(resp.Responses) != len(req.Calls) {
		return nil, fmt.Errorf("unexpected num of responses: %d != %d",
			len(resp.Responses), len(req.Calls))
	}
	return resp, nil
}

func (self *client) Mailboxes(ctx context.Context) (*mailboxes, error) {
	var req jmap.Request
	req.Invoke(&mailbox.Get{Account: self.AccountId()})
	resp, err := self.Do(ctx, &req)
	if err != nil {
		return nil, fmt.Errorf("list Mailboxes: %w", err)
	}

	r, ok := resp.Responses[0].Args.(*mailbox.GetResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected JMAP response %T",
			resp.Responses[0].Args)
	}
	return NewMailboxes(r), nil
}

func (self *client) AccountId() jmap.ID {
	return self.Session.PrimaryAccounts[mail.URI]
}

func (self *client) Account() jmap.Account {
	return self.Session.Accounts[self.AccountId()]
}
