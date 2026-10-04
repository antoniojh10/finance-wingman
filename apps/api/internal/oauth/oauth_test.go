package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestValidateRedirectURI(t *testing.T) {
	valid := []string{
		"https://claude.ai/api/mcp/auth_callback",
		"https://chatgpt.com/connector_platform_oauth_redirect",
		"http://localhost:6274/oauth/callback",
		"http://127.0.0.1:33418/callback",
		"http://[::1]:8080/cb",
		"com.example.app:/oauth2redirect",
	}
	for _, uri := range valid {
		if err := validateRedirectURI(uri); err != nil {
			t.Errorf("validateRedirectURI(%q) unexpected error: %v", uri, err)
		}
	}
	invalid := []string{
		"",
		"/relative/path",
		"http://example.com/callback",
		"https://example.com/cb#fragment",
		"javascript:alert(1)",
		"myapp://callback",
		"https:///no-host",
	}
	for _, uri := range invalid {
		if err := validateRedirectURI(uri); err == nil {
			t.Errorf("validateRedirectURI(%q) should fail", uri)
		}
	}
}

func TestMatchRedirectURI(t *testing.T) {
	registered := []string{"https://claude.ai/api/mcp/auth_callback", "http://127.0.0.1/callback"}
	tests := map[string]bool{
		"https://claude.ai/api/mcp/auth_callback":     true,
		"https://claude.ai/api/mcp/auth_callback/":    false,
		"https://claude.ai/api/mcp/auth_callback?x=1": false,
		"https://evil.example/api/mcp/auth_callback":  false,
		"http://127.0.0.1:51234/callback":             true, // loopback: any port
		"http://127.0.0.1:51234/other":                false,
		"http://localhost:51234/callback":             false, // host must match exactly
		"https://127.0.0.1:51234/callback":            false,
		"not a url\x7f":                               false,
	}
	for uri, want := range tests {
		if got := matchRedirectURI(registered, uri); got != want {
			t.Errorf("matchRedirectURI(%q) = %v, want %v", uri, got, want)
		}
	}
}

func TestVerifyPKCE(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])

	if !verifyPKCE(verifier, challenge) {
		t.Fatal("expected matching verifier to pass")
	}
	if verifyPKCE(verifier+"x", challenge) {
		t.Fatal("expected different verifier to fail")
	}
	if verifyPKCE("short", challenge) {
		t.Fatal("verifiers shorter than 43 characters must fail")
	}
}

func TestPickLocale(t *testing.T) {
	tests := []struct{ explicit, accept, want string }{
		{"", "", "en"},
		{"es", "en-US", "es"},
		{"fr es", "", "es"},
		{"", "es-MX,es;q=0.9,en;q=0.8", "es"},
		{"", "fr-FR,en;q=0.5", "en"},
		{"", "de", "en"},
	}
	for _, tc := range tests {
		if got := pickLocale(tc.explicit, tc.accept); got != tc.want {
			t.Errorf("pickLocale(%q, %q) = %q, want %q", tc.explicit, tc.accept, got, tc.want)
		}
	}
}

func TestWithQueryPreservesExistingParameters(t *testing.T) {
	got := withQuery("https://app.example/cb?tenant=1", map[string]string{"code": "abc", "state": "", "iss": "https://api"})
	want := "https://app.example/cb?code=abc&iss=https%3A%2F%2Fapi&tenant=1"
	if got != want {
		t.Fatalf("withQuery() = %q, want %q", got, want)
	}
}
