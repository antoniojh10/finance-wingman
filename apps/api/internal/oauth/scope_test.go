package oauth

import "testing"

func TestRequestedScope(t *testing.T) {
	cases := map[string]string{
		"":                           "finance:read finance:write",
		"finance":                    "finance:read finance:write",
		"finance:write":              "finance:read finance:write",
		"finance:read finance:write": "finance:read finance:write",
		"openid profile":             "finance:read finance:write",
		"finance:read":               "finance:read",
		"openid finance:read":        "finance:read",
		"finance:read finance":       "finance:read finance:write",
	}
	for raw, want := range cases {
		if got := requestedScope(raw); got != want {
			t.Errorf("requestedScope(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestGrantedScope(t *testing.T) {
	full, read := "finance:read finance:write", "finance:read"
	cases := []struct{ allowed, access, want string }{
		{full, accessReadWrite, full},
		{full, "", full},
		{full, "anything", full},
		{full, accessReadOnly, read},
		{read, accessReadWrite, read}, // never more than the client asked for
		{read, "", read},
		{"finance", accessReadWrite, full}, // requests from before scopes
	}
	for _, c := range cases {
		if got := grantedScope(c.allowed, c.access); got != c.want {
			t.Errorf("grantedScope(%q, %q) = %q, want %q", c.allowed, c.access, got, c.want)
		}
	}
	if canOfferWrite(read) || !canOfferWrite(full) {
		t.Error("canOfferWrite should follow the allowed scope")
	}
}
