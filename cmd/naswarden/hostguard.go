package main

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
)

// hostGuard answers only requests addressed to a host name NasWarden
// expects. Without it a web page on the internet can read the dashboard by
// DNS rebinding: the page's domain is first served from the attacker, then
// pointed at the NAS's LAN address, and from then on the browser sends
// requests with Host and Origin both set to that domain, which passes the
// websocket's same-origin check.
//
// Accepted by default: IP addresses, "localhost" and single-label names such
// as "nas", which cover nearly every LAN setup and cannot be used for
// rebinding (that needs a domain the attacker controls). Anything else must
// be listed in NASWARDEN_ALLOWED_HOSTS; "*" turns the check off.
//
// The check is opt-in so that upgrading never locks anyone out (someone
// reaching NasWarden by a domain name through a reverse proxy would
// otherwise get a 421): while NASWARDEN_ALLOWED_HOSTS is unset, unknown
// hosts are answered as before and each one is logged once as a warning.
// Setting the variable turns the refusal on.
type hostGuard struct {
	allowAll bool
	enforce  bool
	allowed  map[string]struct{}
	next     http.Handler

	mu     sync.Mutex
	logged map[string]struct{}
}

const maxLoggedHosts = 100

// newHostGuard builds the guard from the NASWARDEN_ALLOWED_HOSTS value, a
// comma-separated list of host names (with or without a port).
func newHostGuard(allowedHosts string, next http.Handler) *hostGuard {
	g := &hostGuard{
		allowed: map[string]struct{}{},
		next:    next,
		logged:  map[string]struct{}{},
	}
	for _, entry := range strings.Split(allowedHosts, ",") {
		entry = strings.ToLower(strings.TrimSpace(entry))
		switch entry {
		case "":
		case "*":
			g.allowAll = true
		default:
			g.allowed[entry] = struct{}{}
		}
	}
	g.enforce = len(g.allowed) > 0
	return g
}

func (g *hostGuard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if g.allowAll || g.hostAllowed(r.Host) {
		g.next.ServeHTTP(w, r)
		return
	}
	g.logUnknown(r.Host)
	if !g.enforce {
		g.next.ServeHTTP(w, r)
		return
	}
	http.Error(w, "Misdirected Request: NasWarden does not answer to the host name \""+sanitizeHost(r.Host)+
		"\". If this is the name you use to reach it (for example through a reverse proxy), "+
		"add it to NASWARDEN_ALLOWED_HOSTS.", http.StatusMisdirectedRequest)
}

func (g *hostGuard) hostAllowed(hostHeader string) bool {
	full := strings.ToLower(hostHeader)
	if _, ok := g.allowed[full]; ok {
		return true
	}
	host := hostOnly(full)
	if _, ok := g.allowed[host]; ok {
		return true
	}
	return defaultHostAllowed(host)
}

// hostOnly drops the port and the brackets of an IPv6 literal, and the
// trailing dot of a fully qualified name.
func hostOnly(hostHeader string) string {
	host := hostHeader
	if h, _, err := net.SplitHostPort(hostHeader); err == nil {
		host = h
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	return strings.TrimSuffix(host, ".")
}

// defaultHostAllowed reports whether a host name needs no configuration: an
// IP address, localhost, or a single-label name.
func defaultHostAllowed(host string) bool {
	if host == "" {
		return false
	}
	// Strip an IPv6 zone ("fe80::1%eth0") before parsing.
	ip, _, _ := strings.Cut(host, "%")
	if net.ParseIP(ip) != nil {
		return true
	}
	return host == "localhost" || !strings.Contains(host, ".")
}

// logUnknown logs each distinct unknown host once, so a scanner cannot
// flood the log. After maxLoggedHosts distinct hosts it stops logging new ones.
func (g *hostGuard) logUnknown(hostHeader string) {
	host := sanitizeHost(hostHeader)
	g.mu.Lock()
	_, seen := g.logged[host]
	if !seen && len(g.logged) < maxLoggedHosts {
		g.logged[host] = struct{}{}
	} else {
		seen = true
	}
	g.mu.Unlock()
	if !seen {
		if g.enforce {
			slog.Warn("refused a request for an unknown host name; add it to NASWARDEN_ALLOWED_HOSTS if it is yours", "host", host)
		} else {
			slog.Warn("request for a host name that is not an IP address, localhost or a single-word name; "+
				"set NASWARDEN_ALLOWED_HOSTS to your host names to refuse such requests (protects against DNS rebinding)", "host", host)
		}
	}
}

// sanitizeHost makes a client-supplied Host value safe to echo and log.
func sanitizeHost(h string) string {
	if len(h) > 100 {
		h = h[:100]
	}
	return strings.Map(func(r rune) rune {
		if r < 0x21 || r > 0x7e || r == '"' || r == '<' || r == '>' || r == '&' {
			return '_'
		}
		return r
	}, h)
}
