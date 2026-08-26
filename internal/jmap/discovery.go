package jmap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

func autoDiscovery(ctx context.Context, username string) (string, error) {
	_, domain, _ := strings.CutLast(username, "@")
	if domain == "" {
		return "", nil
	}

	var resolver net.Resolver
	_, addrs, err := resolver.LookupSRV(ctx, "jmap", "tcp", domain)
	if err != nil {
		dnsError, ok := errors.AsType[*net.DNSError](err)
		if ok && dnsError.IsNotFound {
			return "https://" + domain + "/.well-known/jmap", nil
		}
		return "", fmt.Errorf("resolving jmap for %s: %w", domain, err)
	}

	if len(addrs) == 0 {
		return "https://" + domain + "/.well-known/jmap", nil
	}

	s := addrs[0]
	target, _ := strings.CutSuffix(s.Target, ".")

	if s.Port == 443 {
		return "https://" + target + "/.well-known/jmap", nil
	}
	return "https://" + net.JoinHostPort(target, strconv.Itoa(int(s.Port))) +
		"/.well-known/jmap", nil
}
