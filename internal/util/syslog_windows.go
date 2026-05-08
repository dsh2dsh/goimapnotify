//go:build windows

package util

import "fmt"

// EnableSyslog is not supported on Windows.
func EnableSyslog() error {
	return fmt.Errorf("syslog is not supported on Windows")
}
