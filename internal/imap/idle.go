package imap

import (
	"time"

	idle "github.com/emersion/go-imap-idle"
	"github.com/emersion/go-imap/client"

	"github.com/dsh2dsh/goimapnotify/internal/config"
	"github.com/dsh2dsh/goimapnotify/internal/util"
)

// IDLE wraps the IMAP client with IDLE support
type IDLE struct {
	*client.Client
	*idle.IdleClient
}

// NewIDLE creates a new IMAP client with IDLE support
func NewIDLE(conf config.NotifyConfig, retries int) (*IDLE, error) {
	confCMDExecuted := util.RetrieveCmd(conf)
	i, err := New(confCMDExecuted, retries)
	if err != nil {
		return nil, err
	}

	idleC := idle.NewClient(i)

	amount := 25 // default per https://github.com/emersion/go-imap-idle/blob/db256843144576c70e551f0732f1d1d3b5bec67e/client.go#L11
	if conf.IDLELogoutTimeout > 0 {
		amount = conf.IDLELogoutTimeout
	}
	idleC.LogoutTimeout = time.Duration(amount) * time.Minute

	return &IDLE{i, idleC}, nil
}
