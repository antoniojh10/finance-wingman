package oauth

import (
	"slices"
	"strings"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
)

// Access choices on the consent page.
const (
	accessReadWrite = "read_write"
	accessReadOnly  = "read_only"
)

var readOnlyScope = auth.FormatScope([]string{auth.ScopeRead})

// requestedScope is the most a client may be granted for the scope it asked
// for: read only when it asked for finance:read alone, read and write
// otherwise. Clients that send no scope, the legacy "finance" scope, or
// only scopes this server does not know get read and write, the default;
// unknown scopes are ignored rather than rejected.
func requestedScope(raw string) string {
	fields := strings.Fields(raw)
	if slices.Contains(fields, auth.ScopeRead) && !slices.Contains(fields, auth.ScopeWrite) && !slices.Contains(fields, "finance") {
		return readOnlyScope
	}
	return auth.FullScope
}

// grantedScope applies the user's choice on the consent page to the scope
// the client may be granted. It can only narrow it: a missing or unknown
// choice keeps the default (read and write) when the client allows it.
func grantedScope(allowed, access string) string {
	scopes := auth.ParseScope(allowed)
	if access == accessReadOnly || !slices.Contains(scopes, auth.ScopeWrite) {
		return readOnlyScope
	}
	return auth.FullScope
}

// canOfferWrite reports whether the consent page offers read and write.
func canOfferWrite(allowed string) bool {
	return slices.Contains(auth.ParseScope(allowed), auth.ScopeWrite)
}
