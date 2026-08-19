package jmap

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"git.sr.ht/~rockorager/go-jmap"
	"golang.org/x/oauth2"

	"github.com/dsh2dsh/goimapnotify/internal/config"
)

var httpClient = &http.Client{
	Transport: &httpTransport{RoundTripper: http.DefaultTransport},
	Timeout:   20 * time.Second,
}

func New(ctx context.Context, conf *config.NotifyConfig) (*jmap.Client, error) {
	client := withAccessToken(ctx,
		&jmap.Client{SessionEndpoint: conf.Host}, conf.Password)

	if err := authenticate(ctx, client); err != nil {
		return nil, fmt.Errorf("jmap authentication: %w", err)
	}
	return client, nil
}

func withAccessToken(ctx context.Context, client *jmap.Client, token string,
) *jmap.Client {
	t := &oauth2.Token{AccessToken: token, TokenType: "bearer"}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, httpClient)
	client.HttpClient = oauth2.NewClient(ctx,
		new(oauth2.Config).TokenSource(ctx, t))
	return client
}

func authenticate(ctx context.Context, client *jmap.Client) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		client.SessionEndpoint, nil)
	if err != nil {
		return fmt.Errorf("building auth request: %w", err)
	}

	resp, err := client.HttpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetching jmap session resource: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return fmt.Errorf("%w: %d %s", ErrLoginFailed, resp.StatusCode,
			http.StatusText(resp.StatusCode))
	default:
		return fmt.Errorf("http status: %d %s", resp.StatusCode,
			http.StatusText(resp.StatusCode))
	}

	s := new(jmap.Session)
	if err := json.NewDecoder(resp.Body).Decode(s); err != nil {
		return fmt.Errorf("unmarshaling jmap session resource: %w", err)
	}
	client.Session = s
	return nil
}
