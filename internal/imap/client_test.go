package imap

// This file is part of goimapnotify
// Copyright (C) 2017-2025  Jorge Javier Araya Navarro

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

import (
	"crypto/tls"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-sasl"

	"gitlab.com/shackra/goimapnotify/internal/config"
)

// MockIMAPClient is a mock implementation of IMAPClientInterface for testing
type MockIMAPClient struct {
	// Configurable behavior
	LoginFunc        func(username, password string) error
	AuthenticateFunc func(auth sasl.Client) error
	SupportAuthFunc  func(mech string) (bool, error)
	StartTLSFunc     func(config *tls.Config) error
	SupportFunc      func(cap string) (bool, error)
	LogoutFunc       func() error
	LoggedOutFunc    func() <-chan struct{}
	SelectFunc       func(name string, readOnly bool) (*imap.MailboxStatus, error)
	SetDebugFunc     func(w io.Writer)

	// Call tracking
	LoginCalls        []loginCall
	AuthenticateCalls []sasl.Client
	SupportAuthCalls  []string
	StartTLSCalls     int
	SupportCalls      []string
	LogoutCalls       int
	SelectCalls       []selectCall
	SetDebugCalls     int
}

type loginCall struct {
	Username string
	Password string
}

type selectCall struct {
	Name     string
	ReadOnly bool
}

func (m *MockIMAPClient) Login(username, password string) error {
	m.LoginCalls = append(m.LoginCalls, loginCall{username, password})
	if m.LoginFunc != nil {
		return m.LoginFunc(username, password)
	}
	return nil
}

func (m *MockIMAPClient) Authenticate(auth sasl.Client) error {
	m.AuthenticateCalls = append(m.AuthenticateCalls, auth)
	if m.AuthenticateFunc != nil {
		return m.AuthenticateFunc(auth)
	}
	return nil
}

func (m *MockIMAPClient) SupportAuth(mech string) (bool, error) {
	m.SupportAuthCalls = append(m.SupportAuthCalls, mech)
	if m.SupportAuthFunc != nil {
		return m.SupportAuthFunc(mech)
	}
	return false, nil
}

func (m *MockIMAPClient) StartTLS(config *tls.Config) error {
	m.StartTLSCalls++
	if m.StartTLSFunc != nil {
		return m.StartTLSFunc(config)
	}
	return nil
}

func (m *MockIMAPClient) Support(cap string) (bool, error) {
	m.SupportCalls = append(m.SupportCalls, cap)
	if m.SupportFunc != nil {
		return m.SupportFunc(cap)
	}
	return false, nil
}

func (m *MockIMAPClient) Logout() error {
	m.LogoutCalls++
	if m.LogoutFunc != nil {
		return m.LogoutFunc()
	}
	return nil
}

func (m *MockIMAPClient) LoggedOut() <-chan struct{} {
	if m.LoggedOutFunc != nil {
		return m.LoggedOutFunc()
	}
	return make(chan struct{})
}

func (m *MockIMAPClient) Select(name string, readOnly bool) (*imap.MailboxStatus, error) {
	m.SelectCalls = append(m.SelectCalls, selectCall{name, readOnly})
	if m.SelectFunc != nil {
		return m.SelectFunc(name, readOnly)
	}
	return &imap.MailboxStatus{Messages: 0}, nil
}

func (m *MockIMAPClient) SetDebug(w io.Writer) {
	m.SetDebugCalls++
	if m.SetDebugFunc != nil {
		m.SetDebugFunc(w)
	}
}

// MockDialer is a mock implementation of IMAPDialer for testing
type MockDialer struct {
	DialFunc    func(addr string) (IMAPClientInterface, error)
	DialTLSFunc func(addr string, config *tls.Config) (IMAPClientInterface, error)

	DialCalls    []string
	DialTLSCalls []dialTLSCall
}

type dialTLSCall struct {
	Addr   string
	Config *tls.Config
}

func (m *MockDialer) Dial(addr string) (IMAPClientInterface, error) {
	m.DialCalls = append(m.DialCalls, addr)
	if m.DialFunc != nil {
		return m.DialFunc(addr)
	}
	return &MockIMAPClient{}, nil
}

func (m *MockDialer) DialTLS(addr string, config *tls.Config) (IMAPClientInterface, error) {
	m.DialTLSCalls = append(m.DialTLSCalls, dialTLSCall{addr, config})
	if m.DialTLSFunc != nil {
		return m.DialTLSFunc(addr, config)
	}
	return &MockIMAPClient{}, nil
}

// TestIMAPIDLEClient_Structure tests the IMAPIDLEClient struct
func TestIMAPIDLEClient_Structure(t *testing.T) {
	// Verify the struct can be instantiated
	c := &IMAPIDLEClient{}
	if c == nil {
		t.Fatal("IMAPIDLEClient should be instantiable")
	}
}

// TestVersion tests the Version variable
func TestVersion(t *testing.T) {
	// Version should have a default value
	if Version == "" {
		t.Error("Version should not be empty")
	}

	// Default value should be "unknown"
	if Version != "unknown" {
		t.Logf("Version is set to: %s (may be set at build time)", Version)
	}
}

// TestMockIMAPClient_ImplementsInterface verifies the mock implements the interface
func TestMockIMAPClient_ImplementsInterface(t *testing.T) {
	var _ IMAPClientInterface = &MockIMAPClient{}
}

// TestMockDialer_ImplementsInterface verifies the mock dialer implements the interface
func TestMockDialer_ImplementsInterface(t *testing.T) {
	var _ IMAPDialer = &MockDialer{}
}

// TestDefaultDialer tests that DefaultDialer returns a valid dialer
func TestDefaultDialer(t *testing.T) {
	dialer := DefaultDialer()
	if dialer == nil {
		t.Fatal("DefaultDialer() returned nil")
	}
}

// TestNewClientWithDialer tests the NewClientWithDialer function
func TestNewClientWithDialer(t *testing.T) {
	tests := []struct {
		name        string
		conf        config.NotifyConfig
		setupDialer func() *MockDialer
		setupClient func() *MockIMAPClient
		wantErr     bool
		errContains string
	}{
		{
			name: "successful login without TLS",
			conf: config.NotifyConfig{
				Host:     "imap.example.com",
				Port:     143,
				TLS:      false,
				Username: "user@example.com",
				Password: "password123",
			},
			setupDialer: func() *MockDialer {
				return &MockDialer{}
			},
			setupClient: func() *MockIMAPClient {
				return &MockIMAPClient{}
			},
			wantErr: false,
		},
		{
			name: "successful login with TLS",
			conf: config.NotifyConfig{
				Host:     "imap.example.com",
				Port:     993,
				TLS:      true,
				Username: "user@example.com",
				Password: "password123",
				TLSOptions: config.TLSOptionsStruct{
					RejectUnauthorized: true,
					STARTTLS:           false,
				},
			},
			setupDialer: func() *MockDialer {
				return &MockDialer{}
			},
			setupClient: func() *MockIMAPClient {
				return &MockIMAPClient{}
			},
			wantErr: false,
		},
		{
			name: "successful login with STARTTLS",
			conf: config.NotifyConfig{
				Host:     "imap.example.com",
				Port:     143,
				TLS:      true,
				Username: "user@example.com",
				Password: "password123",
				TLSOptions: config.TLSOptionsStruct{
					RejectUnauthorized: true,
					STARTTLS:           true,
				},
			},
			setupDialer: func() *MockDialer {
				return &MockDialer{}
			},
			setupClient: func() *MockIMAPClient {
				return &MockIMAPClient{}
			},
			wantErr: false,
		},
		{
			name: "STARTTLS failure",
			conf: config.NotifyConfig{
				Host:     "imap.example.com",
				Port:     143,
				TLS:      true,
				Username: "user@example.com",
				Password: "password123",
				TLSOptions: config.TLSOptionsStruct{
					STARTTLS: true,
				},
			},
			setupDialer: func() *MockDialer {
				return &MockDialer{}
			},
			setupClient: func() *MockIMAPClient {
				return &MockIMAPClient{
					StartTLSFunc: func(config *tls.Config) error {
						return errors.New("STARTTLS failed")
					},
				}
			},
			wantErr:     true,
			errContains: "STARTTLS failed",
		},
		{
			name: "dial failure after retries",
			conf: config.NotifyConfig{
				Host:     "imap.example.com",
				Port:     993,
				TLS:      true,
				Username: "user@example.com",
				Password: "password123",
			},
			setupDialer: func() *MockDialer {
				return &MockDialer{
					DialTLSFunc: func(addr string, config *tls.Config) (IMAPClientInterface, error) {
						return nil, errors.New("connection refused")
					},
				}
			},
			setupClient: func() *MockIMAPClient {
				return &MockIMAPClient{}
			},
			wantErr:     true,
			errContains: "cannot dial",
		},
		{
			name: "login failure",
			conf: config.NotifyConfig{
				Host:     "imap.example.com",
				Port:     143,
				TLS:      false,
				Username: "user@example.com",
				Password: "wrongpassword",
			},
			setupDialer: func() *MockDialer {
				return &MockDialer{}
			},
			setupClient: func() *MockIMAPClient {
				return &MockIMAPClient{
					LoginFunc: func(username, password string) error {
						return errors.New("invalid credentials")
					},
				}
			},
			wantErr:     true,
			errContains: "invalid credentials",
		},
		{
			name: "XOAuth2 with OAuthBearer support",
			conf: config.NotifyConfig{
				Host:     "imap.gmail.com",
				Port:     993,
				TLS:      true,
				Username: "user@gmail.com",
				Password: "oauth_token",
				XOAuth2:  true,
			},
			setupDialer: func() *MockDialer {
				return &MockDialer{}
			},
			setupClient: func() *MockIMAPClient {
				return &MockIMAPClient{
					SupportAuthFunc: func(mech string) (bool, error) {
						return mech == sasl.OAuthBearer, nil
					},
				}
			},
			wantErr: false,
		},
		{
			name: "XOAuth2 with only XOAUTH2 support",
			conf: config.NotifyConfig{
				Host:     "imap.gmail.com",
				Port:     993,
				TLS:      true,
				Username: "user@gmail.com",
				Password: "oauth_token",
				XOAuth2:  true,
			},
			setupDialer: func() *MockDialer {
				return &MockDialer{}
			},
			setupClient: func() *MockIMAPClient {
				return &MockIMAPClient{
					SupportAuthFunc: func(mech string) (bool, error) {
						return mech == Xoauth2, nil
					},
				}
			},
			wantErr: false,
		},
		{
			name: "XOAuth2 with no token auth support",
			conf: config.NotifyConfig{
				Host:     "imap.example.com",
				Port:     993,
				TLS:      true,
				Username: "user@example.com",
				Password: "oauth_token",
				XOAuth2:  true,
			},
			setupDialer: func() *MockDialer {
				return &MockDialer{}
			},
			setupClient: func() *MockIMAPClient {
				return &MockIMAPClient{
					SupportAuthFunc: func(mech string) (bool, error) {
						return false, nil
					},
				}
			},
			wantErr:     true,
			errContains: "XOAUTH2 and OAUTHBEARER are not supported",
		},
		{
			name: "XOAuth2 SupportAuth error",
			conf: config.NotifyConfig{
				Host:     "imap.example.com",
				Port:     993,
				TLS:      true,
				Username: "user@example.com",
				Password: "oauth_token",
				XOAuth2:  true,
			},
			setupDialer: func() *MockDialer {
				return &MockDialer{}
			},
			setupClient: func() *MockIMAPClient {
				return &MockIMAPClient{
					SupportAuthFunc: func(mech string) (bool, error) {
						return false, errors.New("capability check failed")
					},
				}
			},
			wantErr:     true,
			errContains: "checking supported authentication",
		},
		{
			name: "Support capability check error",
			conf: config.NotifyConfig{
				Host:     "imap.example.com",
				Port:     143,
				TLS:      false,
				Username: "user@example.com",
				Password: "password123",
			},
			setupDialer: func() *MockDialer {
				return &MockDialer{}
			},
			setupClient: func() *MockIMAPClient {
				return &MockIMAPClient{
					SupportFunc: func(cap string) (bool, error) {
						return false, errors.New("capability error")
					},
				}
			},
			wantErr:     true,
			errContains: "unable to check support for capability",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDialer := tt.setupDialer()
			mockClient := tt.setupClient()

			// Set up the dialer to return our mock client
			if mockDialer.DialFunc == nil {
				mockDialer.DialFunc = func(addr string) (IMAPClientInterface, error) {
					return mockClient, nil
				}
			}
			if mockDialer.DialTLSFunc == nil {
				mockDialer.DialTLSFunc = func(addr string, config *tls.Config) (IMAPClientInterface, error) {
					return mockClient, nil
				}
			}

			client, err := NewClientWithDialer(mockDialer, tt.conf, 1)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewClientWithDialer() error = nil, wantErr %v", tt.wantErr)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf(
						"NewClientWithDialer() error = %v, want error containing %q",
						err,
						tt.errContains,
					)
				}
				return
			}

			if err != nil {
				t.Errorf("NewClientWithDialer() unexpected error = %v", err)
				return
			}

			if client == nil {
				t.Error("NewClientWithDialer() returned nil client")
			}
		})
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchString(s, substr)))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestNewClientWithDialer_TLSConfig tests that TLS configuration is correctly applied
func TestNewClientWithDialer_TLSConfig(t *testing.T) {
	tests := []struct {
		name                   string
		conf                   config.NotifyConfig
		wantTLSDial            bool
		wantInsecureSkipVerify bool
		wantServerName         string
	}{
		{
			name: "TLS with certificate verification",
			conf: config.NotifyConfig{
				Host: "imap.example.com",
				Port: 993,
				TLS:  true,
				TLSOptions: config.TLSOptionsStruct{
					RejectUnauthorized: true,
					STARTTLS:           false,
				},
				Username: "user",
				Password: "pass",
			},
			wantTLSDial:            true,
			wantInsecureSkipVerify: false,
			wantServerName:         "imap.example.com",
		},
		{
			name: "TLS without certificate verification",
			conf: config.NotifyConfig{
				Host: "imap.example.com",
				Port: 993,
				TLS:  true,
				TLSOptions: config.TLSOptionsStruct{
					RejectUnauthorized: false,
					STARTTLS:           false,
				},
				Username: "user",
				Password: "pass",
			},
			wantTLSDial:            true,
			wantInsecureSkipVerify: true,
			wantServerName:         "imap.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDialer := &MockDialer{
				DialTLSFunc: func(addr string, config *tls.Config) (IMAPClientInterface, error) {
					// Verify TLS config
					if config.InsecureSkipVerify != tt.wantInsecureSkipVerify {
						t.Errorf(
							"InsecureSkipVerify = %v, want %v",
							config.InsecureSkipVerify,
							tt.wantInsecureSkipVerify,
						)
					}
					if config.ServerName != tt.wantServerName {
						t.Errorf("ServerName = %q, want %q", config.ServerName, tt.wantServerName)
					}
					if config.MinVersion != tls.VersionTLS12 {
						t.Errorf("MinVersion = %d, want %d", config.MinVersion, tls.VersionTLS12)
					}
					return &MockIMAPClient{}, nil
				},
			}

			_, err := NewClientWithDialer(mockDialer, tt.conf, 1)
			if err != nil {
				t.Errorf("NewClientWithDialer() error = %v", err)
			}

			if tt.wantTLSDial && len(mockDialer.DialTLSCalls) == 0 {
				t.Error("expected DialTLS to be called")
			}
		})
	}
}

// TestNewClientWithDialer_Retries tests the retry mechanism
func TestNewClientWithDialer_Retries(t *testing.T) {
	tests := []struct {
		name          string
		retries       int
		failCount     int
		wantDialCalls int
		wantErr       bool
	}{
		{
			name:          "success on first try",
			retries:       3,
			failCount:     0,
			wantDialCalls: 1,
			wantErr:       false,
		},
		{
			name:          "success after one retry",
			retries:       3,
			failCount:     1,
			wantDialCalls: 2,
			wantErr:       false,
		},
		{
			name:          "failure after all retries",
			retries:       2,
			failCount:     5,
			wantDialCalls: 2,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialCount := 0
			mockDialer := &MockDialer{
				DialFunc: func(addr string) (IMAPClientInterface, error) {
					dialCount++
					if dialCount <= tt.failCount {
						return nil, errors.New("connection failed")
					}
					return &MockIMAPClient{}, nil
				},
			}

			conf := config.NotifyConfig{
				Host:     "imap.example.com",
				Port:     143,
				TLS:      false,
				Username: "user",
				Password: "pass",
			}

			_, err := NewClientWithDialer(mockDialer, conf, tt.retries)

			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}

			if dialCount != tt.wantDialCalls {
				t.Errorf("dial calls = %d, want %d", dialCount, tt.wantDialCalls)
			}
		})
	}
}

// TestNewClientWithDialer_AuthenticationMethods tests different authentication methods
func TestNewClientWithDialer_AuthenticationMethods(t *testing.T) {
	t.Run("standard login", func(t *testing.T) {
		mockClient := &MockIMAPClient{}
		mockDialer := &MockDialer{
			DialFunc: func(addr string) (IMAPClientInterface, error) {
				return mockClient, nil
			},
		}

		conf := config.NotifyConfig{
			Host:     "imap.example.com",
			Port:     143,
			Username: "testuser",
			Password: "testpass",
			XOAuth2:  false,
		}

		_, err := NewClientWithDialer(mockDialer, conf, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(mockClient.LoginCalls) != 1 {
			t.Errorf("Login called %d times, want 1", len(mockClient.LoginCalls))
		}
		if mockClient.LoginCalls[0].Username != "testuser" {
			t.Errorf("Login username = %q, want %q", mockClient.LoginCalls[0].Username, "testuser")
		}
		if mockClient.LoginCalls[0].Password != "testpass" {
			t.Errorf("Login password = %q, want %q", mockClient.LoginCalls[0].Password, "testpass")
		}
	})

	t.Run("OAuthBearer authentication", func(t *testing.T) {
		mockClient := &MockIMAPClient{
			SupportAuthFunc: func(mech string) (bool, error) {
				return mech == sasl.OAuthBearer, nil
			},
		}
		mockDialer := &MockDialer{
			DialTLSFunc: func(addr string, config *tls.Config) (IMAPClientInterface, error) {
				return mockClient, nil
			},
		}

		conf := config.NotifyConfig{
			Host:     "imap.gmail.com",
			Port:     993,
			TLS:      true,
			Username: "testuser@gmail.com",
			Password: "oauth_token",
			XOAuth2:  true,
		}

		_, err := NewClientWithDialer(mockDialer, conf, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(mockClient.LoginCalls) != 0 {
			t.Errorf(
				"Login should not be called for OAuth, got %d calls",
				len(mockClient.LoginCalls),
			)
		}
		if len(mockClient.AuthenticateCalls) != 1 {
			t.Errorf("Authenticate called %d times, want 1", len(mockClient.AuthenticateCalls))
		}
	})

	t.Run("XOAUTH2 authentication fallback", func(t *testing.T) {
		mockClient := &MockIMAPClient{
			SupportAuthFunc: func(mech string) (bool, error) {
				// Only support XOAUTH2, not OAuthBearer
				return mech == Xoauth2, nil
			},
		}
		mockDialer := &MockDialer{
			DialTLSFunc: func(addr string, config *tls.Config) (IMAPClientInterface, error) {
				return mockClient, nil
			},
		}

		conf := config.NotifyConfig{
			Host:     "imap.example.com",
			Port:     993,
			TLS:      true,
			Username: "testuser",
			Password: "oauth_token",
			XOAuth2:  true,
		}

		_, err := NewClientWithDialer(mockDialer, conf, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(mockClient.AuthenticateCalls) != 1 {
			t.Errorf("Authenticate called %d times, want 1", len(mockClient.AuthenticateCalls))
		}
	})
}

// TestNewClientWithDialer_STARTTLS tests STARTTLS upgrade
func TestNewClientWithDialer_STARTTLS(t *testing.T) {
	mockClient := &MockIMAPClient{}
	mockDialer := &MockDialer{
		DialFunc: func(addr string) (IMAPClientInterface, error) {
			return mockClient, nil
		},
	}

	conf := config.NotifyConfig{
		Host:     "imap.example.com",
		Port:     143,
		TLS:      true,
		Username: "user",
		Password: "pass",
		TLSOptions: config.TLSOptionsStruct{
			STARTTLS:           true,
			RejectUnauthorized: true,
		},
	}

	_, err := NewClientWithDialer(mockDialer, conf, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use plain dial first
	if len(mockDialer.DialCalls) != 1 {
		t.Errorf("Dial called %d times, want 1", len(mockDialer.DialCalls))
	}

	// Then upgrade with STARTTLS
	if mockClient.StartTLSCalls != 1 {
		t.Errorf("StartTLS called %d times, want 1", mockClient.StartTLSCalls)
	}
}

// TestServerAddressFormatting tests server address string formatting
func TestServerAddressFormatting(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     int
		expected string
	}{
		{
			name:     "standard IMAPS port",
			host:     "imap.gmail.com",
			port:     993,
			expected: "imap.gmail.com:993",
		},
		{
			name:     "standard IMAP port",
			host:     "imap.example.com",
			port:     143,
			expected: "imap.example.com:143",
		},
		{
			name:     "custom port",
			host:     "mail.example.com",
			port:     2993,
			expected: "mail.example.com:2993",
		},
		{
			name:     "localhost",
			host:     "localhost",
			port:     1143,
			expected: "localhost:1143",
		},
		{
			name:     "IP address",
			host:     "192.168.1.100",
			port:     993,
			expected: "192.168.1.100:993",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test by checking what address is passed to the dialer
			var dialedAddr string
			mockDialer := &MockDialer{
				DialFunc: func(addr string) (IMAPClientInterface, error) {
					dialedAddr = addr
					return &MockIMAPClient{}, nil
				},
			}

			conf := config.NotifyConfig{
				Host:     tt.host,
				Port:     tt.port,
				TLS:      false,
				Username: "user",
				Password: "pass",
			}

			_, _ = NewClientWithDialer(mockDialer, conf, 1)

			if dialedAddr != tt.expected {
				t.Errorf("dialed address = %q, want %q", dialedAddr, tt.expected)
			}
		})
	}
}

// TestIDLELogoutTimeout tests the IDLE logout timeout configuration
func TestIDLELogoutTimeout(t *testing.T) {
	tests := []struct {
		name            string
		configTimeout   int
		expectedTimeout time.Duration
	}{
		{
			name:            "default timeout (0 means use default 25)",
			configTimeout:   0,
			expectedTimeout: 25 * time.Minute,
		},
		{
			name:            "custom timeout 10 minutes",
			configTimeout:   10,
			expectedTimeout: 10 * time.Minute,
		},
		{
			name:            "custom timeout 30 minutes",
			configTimeout:   30,
			expectedTimeout: 30 * time.Minute,
		},
		{
			name:            "custom timeout 60 minutes",
			configTimeout:   60,
			expectedTimeout: 60 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeout := calculateIDLETimeout(tt.configTimeout)
			if timeout != tt.expectedTimeout {
				t.Errorf("calculateIDLETimeout() = %v, want %v", timeout, tt.expectedTimeout)
			}
		})
	}
}

// calculateIDLETimeout calculates the IDLE logout timeout based on config
func calculateIDLETimeout(configTimeout int) time.Duration {
	amount := 25 // default
	if configTimeout > 0 {
		amount = configTimeout
	}
	return time.Duration(amount) * time.Minute
}

// TestMockIMAPClient_DefaultBehavior tests that mock has sensible defaults
func TestMockIMAPClient_DefaultBehavior(t *testing.T) {
	mock := &MockIMAPClient{}

	// Login should succeed by default
	if err := mock.Login("user", "pass"); err != nil {
		t.Errorf("Login() error = %v, want nil", err)
	}

	// Authenticate should succeed by default
	if err := mock.Authenticate(nil); err != nil {
		t.Errorf("Authenticate() error = %v, want nil", err)
	}

	// SupportAuth should return false by default
	if ok, err := mock.SupportAuth("PLAIN"); ok || err != nil {
		t.Errorf("SupportAuth() = (%v, %v), want (false, nil)", ok, err)
	}

	// StartTLS should succeed by default
	if err := mock.StartTLS(nil); err != nil {
		t.Errorf("StartTLS() error = %v, want nil", err)
	}

	// Support should return false by default
	if ok, err := mock.Support("ID"); ok || err != nil {
		t.Errorf("Support() = (%v, %v), want (false, nil)", ok, err)
	}

	// Logout should succeed by default
	if err := mock.Logout(); err != nil {
		t.Errorf("Logout() error = %v, want nil", err)
	}

	// Select should return empty status by default
	status, err := mock.Select("INBOX", true)
	if err != nil {
		t.Errorf("Select() error = %v, want nil", err)
	}
	if status == nil {
		t.Error("Select() status is nil, want non-nil")
	}

	// LoggedOut should return a channel
	ch := mock.LoggedOut()
	if ch == nil {
		t.Error("LoggedOut() returned nil channel")
	}

	// SetDebug should not panic
	mock.SetDebug(nil)
	if mock.SetDebugCalls != 1 {
		t.Errorf("SetDebugCalls = %d, want 1", mock.SetDebugCalls)
	}
}

// TestMockIMAPClient_CallTracking tests that mock tracks calls correctly
func TestMockIMAPClient_CallTracking(t *testing.T) {
	mock := &MockIMAPClient{}

	// Make some calls
	_ = mock.Login("user1", "pass1")
	_ = mock.Login("user2", "pass2")
	_, _ = mock.SupportAuth("XOAUTH2")
	_, _ = mock.SupportAuth("OAUTHBEARER")
	_ = mock.StartTLS(nil)
	_, _ = mock.Support("ID")
	_, _ = mock.Support("IDLE")
	_ = mock.Logout()
	_, _ = mock.Select("INBOX", true)
	_, _ = mock.Select("Sent", false)
	mock.SetDebug(nil)

	// Verify tracking
	if len(mock.LoginCalls) != 2 {
		t.Errorf("LoginCalls count = %d, want 2", len(mock.LoginCalls))
	}
	if len(mock.SupportAuthCalls) != 2 {
		t.Errorf("SupportAuthCalls count = %d, want 2", len(mock.SupportAuthCalls))
	}
	if mock.StartTLSCalls != 1 {
		t.Errorf("StartTLSCalls = %d, want 1", mock.StartTLSCalls)
	}
	if len(mock.SupportCalls) != 2 {
		t.Errorf("SupportCalls count = %d, want 2", len(mock.SupportCalls))
	}
	if mock.LogoutCalls != 1 {
		t.Errorf("LogoutCalls = %d, want 1", mock.LogoutCalls)
	}
	if len(mock.SelectCalls) != 2 {
		t.Errorf("SelectCalls count = %d, want 2", len(mock.SelectCalls))
	}
	if mock.SetDebugCalls != 1 {
		t.Errorf("SetDebugCalls = %d, want 1", mock.SetDebugCalls)
	}

	// Verify call details
	if mock.LoginCalls[0].Username != "user1" {
		t.Errorf("First login username = %q, want %q", mock.LoginCalls[0].Username, "user1")
	}
	if mock.SelectCalls[1].Name != "Sent" {
		t.Errorf("Second select name = %q, want %q", mock.SelectCalls[1].Name, "Sent")
	}
}

// TestGetUnderlyingClient tests the GetUnderlyingClient function
func TestGetUnderlyingClient(t *testing.T) {
	t.Run("returns nil for mock client", func(t *testing.T) {
		mock := &MockIMAPClient{}
		result := GetUnderlyingClient(mock)
		if result != nil {
			t.Errorf("GetUnderlyingClient(mock) = %v, want nil", result)
		}
	})
}
