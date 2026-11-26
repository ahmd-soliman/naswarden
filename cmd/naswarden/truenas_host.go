package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// truenasAddress turns TRUENAS_HOST and TRUENAS_TLS into the host[:port] to
// dial and whether to use TLS. TRUENAS_HOST should be host[:port], but the
// address copied from a browser bar (https://host:8443/) is accepted too:
// the scheme and any path are dropped, with a warning, and the scheme
// decides TLS when TRUENAS_TLS is not set. An explicit TRUENAS_TLS always
// wins, so an existing setup behaves as before.
func truenasAddress(rawHost, tlsSetting string) (host string, useTLS bool, warnings []string, err error) {
	rawHost = strings.TrimSpace(rawHost)
	host = rawHost
	schemeTLS, hasScheme := false, false

	if scheme, rest, ok := strings.Cut(host, "://"); ok {
		switch strings.ToLower(scheme) {
		case "https", "wss":
			schemeTLS = true
		case "http", "ws":
		default:
			return "", false, nil, fmt.Errorf("TRUENAS_HOST %q: unsupported scheme %q; use host:port, for example 192.168.1.10:443", rawHost, scheme)
		}
		hasScheme = true
		host = rest
	}
	// Anything after the host is not part of the address (/, /websocket, ?x).
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}

	if err := validHostPort(host); err != nil {
		return "", false, nil, fmt.Errorf("TRUENAS_HOST %q: %w; use host:port, for example 192.168.1.10:443", rawHost, err)
	}

	useTLS = strings.TrimSpace(tlsSetting) == "true"
	switch {
	case hasScheme && strings.TrimSpace(tlsSetting) == "":
		useTLS = schemeTLS
		warnings = append(warnings, fmt.Sprintf("TRUENAS_HOST should be host:port, without a scheme; using %s with TLS %v from its scheme (set TRUENAS_TLS to choose explicitly)", host, schemeTLS))
	case hasScheme && schemeTLS != useTLS:
		warnings = append(warnings, fmt.Sprintf("TRUENAS_HOST should be host:port, without a scheme; using %s; its scheme says TLS %v but TRUENAS_TLS says %v, so TRUENAS_TLS wins", host, schemeTLS, useTLS))
	case hasScheme:
		warnings = append(warnings, fmt.Sprintf("TRUENAS_HOST should be host:port, without a scheme; using %s", host))
	case host != rawHost:
		warnings = append(warnings, fmt.Sprintf("TRUENAS_HOST should be host:port; using %s", host))
	}
	return host, useTLS, warnings, nil
}

// validHostPort accepts host or host:port, including a bracketed IPv6
// address. It only checks the shape; whether the host exists is for the dial.
func validHostPort(hp string) error {
	if hp == "" {
		return fmt.Errorf("no host")
	}
	if strings.ContainsAny(hp, " \t@\\") {
		return fmt.Errorf("not a host name")
	}
	host, port := hp, ""
	if strings.HasPrefix(hp, "[") {
		end := strings.Index(hp, "]")
		if end < 0 {
			return fmt.Errorf("unclosed [ in IPv6 address")
		}
		host = hp[1:end]
		if net.ParseIP(host) == nil {
			return fmt.Errorf("not an IPv6 address")
		}
		switch rest := hp[end+1:]; {
		case rest == "":
		case strings.HasPrefix(rest, ":"):
			port = rest[1:]
		default:
			return fmt.Errorf("unexpected text after ]")
		}
	} else if h, p, ok := strings.Cut(hp, ":"); ok {
		if strings.Contains(p, ":") {
			return fmt.Errorf("an IPv6 address needs brackets, [::1]:443")
		}
		host, port = h, p
	}
	if host == "" {
		return fmt.Errorf("no host")
	}
	if port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("the port %q is not a number from 1 to 65535", port)
		}
	} else if strings.HasSuffix(hp, ":") {
		return fmt.Errorf("empty port")
	}
	return nil
}
