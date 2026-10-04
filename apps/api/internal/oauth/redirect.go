package oauth

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

// validateRedirectURI accepts https URLs, http loopback URLs (native apps,
// RFC 8252 §7.3), and private-use URI schemes (RFC 8252 §7.1).
func validateRedirectURI(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() {
		return errors.New("redirect URI must be an absolute URL")
	}
	if u.Fragment != "" || strings.Contains(raw, "#") {
		return errors.New("redirect URI must not contain a fragment")
	}
	switch u.Scheme {
	case "https":
		if u.Host == "" {
			return errors.New("redirect URI must include a host")
		}
		return nil
	case "http":
		if !isLoopback(u.Hostname()) {
			return errors.New("http redirect URIs are only allowed for loopback addresses")
		}
		return nil
	case "javascript", "data", "file", "vbscript":
		return errors.New("redirect URI scheme is not allowed")
	default:
		// Private-use schemes must use reverse domain notation.
		if !strings.Contains(u.Scheme, ".") {
			return errors.New("custom redirect URI schemes must use reverse domain notation")
		}
		return nil
	}
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// matchRedirectURI reports whether requested matches one of the registered
// URIs. Loopback URIs match on any port, as native apps pick a free port at
// runtime (RFC 8252 §7.3).
func matchRedirectURI(registered []string, requested string) bool {
	req, err := url.Parse(requested)
	if err != nil {
		return false
	}
	for _, candidate := range registered {
		if candidate == requested {
			return true
		}
		reg, err := url.Parse(candidate)
		if err != nil {
			continue
		}
		if reg.Scheme == "http" && req.Scheme == "http" && isLoopback(reg.Hostname()) &&
			reg.Hostname() == req.Hostname() && reg.Path == req.Path && reg.RawQuery == req.RawQuery {
			return true
		}
	}
	return false
}
