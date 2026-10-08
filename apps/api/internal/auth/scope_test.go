package auth

import (
	"slices"
	"testing"
)

func TestParseScope(t *testing.T) {
	t.Parallel()
	full := []string{ScopeRead, ScopeWrite}
	cases := map[string][]string{
		"":                            full, // sign-in sessions and tokens from before scopes
		"finance":                     full, // legacy scope
		"finance:read finance:write":  full,
		"finance:write":               full, // write implies read
		"  finance:write   other ":    full,
		"finance:read":                {ScopeRead},
		"openid finance:read":         {ScopeRead},
		"openid":                      {},
		"finance:admin finance:reads": {},
	}
	for scope, want := range cases {
		if got := ParseScope(scope); !slices.Equal(got, want) || got == nil {
			t.Errorf("ParseScope(%q) = %v, want %v", scope, got, want)
		}
	}
	if FullScope != "finance:read finance:write" {
		t.Errorf("FullScope = %q", FullScope)
	}
}

func TestSessionCanWrite(t *testing.T) {
	t.Parallel()
	if !(Session{Scopes: ParseScope("")}).CanWrite() {
		t.Error("sign-in sessions can write")
	}
	if (Session{Scopes: ParseScope("finance:read")}).CanWrite() {
		t.Error("read-only tokens cannot write")
	}
	if (Session{}).CanWrite() {
		t.Error("a session without scopes cannot write")
	}
}
