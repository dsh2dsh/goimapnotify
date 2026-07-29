package imap

import (
	"slices"

	"github.com/emersion/go-imap/v2/imapclient"
)

func AllCaps(c *imapclient.Client) []string {
	caps := c.Caps()
	if caps == nil {
		return nil
	}

	sortedCaps := make([]string, 0, len(caps))
	for cap := range caps {
		sortedCaps = append(sortedCaps, string(cap))
	}
	slices.Sort(sortedCaps)
	return sortedCaps
}
