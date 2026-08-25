package jmap

import (
	"context"
	"time"
)

func timeAfter(ctx context.Context, d time.Duration) bool {
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		return false
	}
}
