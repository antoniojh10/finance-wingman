package auth

import (
	"slices"
	"strings"
)

// OAuth scopes a connected app can be granted. Write implies read: a token
// that can write always carries both.
const (
	// ScopeRead lets an app read the workspace's finance data.
	ScopeRead = "finance:read"
	// ScopeWrite lets an app record and change the workspace's data.
	ScopeWrite = "finance:write"
	// legacyScope is the single scope granted before read-only connections
	// existed; it keeps full access.
	legacyScope = "finance"
)

// SupportedScopes are advertised in the OAuth metadata.
var SupportedScopes = []string{ScopeRead, ScopeWrite}

// FullScope is read and write access, as a space-separated scope string.
var FullScope = FormatScope([]string{ScopeRead, ScopeWrite})

// ParseScope turns a stored, space-separated scope into the scopes it
// grants, ordered as SupportedScopes. Grants and tokens issued before
// scopes existed ("" or "finance") have full access; unknown values are
// ignored, so a scope naming nothing known grants nothing.
func ParseScope(scope string) []string {
	fields := strings.Fields(scope)
	if len(fields) == 0 || slices.Contains(fields, legacyScope) || slices.Contains(fields, ScopeWrite) {
		return []string{ScopeRead, ScopeWrite}
	}
	if slices.Contains(fields, ScopeRead) {
		return []string{ScopeRead}
	}
	return []string{}
}

// FormatScope joins scopes into the space-separated form used by OAuth.
func FormatScope(scopes []string) string { return strings.Join(scopes, " ") }

// CanWrite reports whether the session may change data. Sign-in sessions
// always can; OAuth access tokens only with ScopeWrite.
func (s Session) CanWrite() bool { return slices.Contains(s.Scopes, ScopeWrite) }
