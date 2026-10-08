package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/mcpserver"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/ratelimit"
)

// withLimits returns a client whose handler enforces the given limits.
func (a *testAPI) withLimits(limits *Limits) *testAPI {
	c := *a
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	c.handler = NewHandler(Deps{
		Logger:     logger,
		DB:         a.pool,
		Auth:       a.auth,
		Finance:    a.svc,
		Workspaces: a.workspaces,
		OAuth:      a.oauth,
		MCP:        mcpserver.New(a.svc, Version).Handler(a.auth, testIssuer, a.oauth.ResourceMetadataURL(), logger),
		Limits:     limits,
	})
	return &c
}

// fromIP sends a JSON request that appears to come from the given peer, with
// an optional X-Forwarded-For header.
func (a *testAPI) fromIP(method, path, peer, forwardedFor string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = peer + ":5000"
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	return a.raw(req)
}

const loginBody = `{"email":"nobody@example.com"}`

func TestUnauthenticatedEndpointsAreLimitedPerIP(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t).withLimits(&Limits{Unauthenticated: ratelimit.New(60, 3)})

	for i := 0; i < 3; i++ {
		if rec := api.fromIP(http.MethodPost, "/api/v1/auth/login", "198.51.100.1", "", loginBody); rec.Code != http.StatusAccepted {
			t.Fatalf("request %d: expected 202, got %d", i, rec.Code)
		}
	}
	rec := api.fromIP(http.MethodPost, "/api/v1/auth/login", "198.51.100.1", "", loginBody)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Retry-After") != "1" {
		t.Fatalf("expected Retry-After: 1, got %q", rec.Header().Get("Retry-After"))
	}
	var body struct {
		Status int    `json:"status"`
		Detail string `json:"detail"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != http.StatusTooManyRequests || body.Error != "rate_limited" || !strings.Contains(body.Detail, "retry in 1 seconds") {
		t.Fatalf("unexpected body: %s", rec.Body)
	}

	// The budget is shared by every unauthenticated endpoint of the IP.
	for _, path := range []string{"/api/v1/auth/verify", "/oauth/register", "/oauth/token", "/oauth/revoke", "/oauth/authorize"} {
		if rec := api.fromIP(http.MethodPost, path, "198.51.100.1", "", "{}"); rec.Code != http.StatusTooManyRequests {
			t.Errorf("%s: expected 429, got %d", path, rec.Code)
		}
	}
	// Preflight requests and unrelated endpoints are not limited.
	if rec := api.fromIP(http.MethodOptions, "/oauth/token", "198.51.100.1", "", ""); rec.Code != http.StatusNoContent {
		t.Errorf("preflight: expected 204, got %d", rec.Code)
	}
	if rec := api.fromIP(http.MethodGet, "/healthz", "198.51.100.1", "", ""); rec.Code != http.StatusOK {
		t.Errorf("healthz: expected 200, got %d", rec.Code)
	}
	// Other clients keep their own budget.
	if rec := api.fromIP(http.MethodPost, "/api/v1/auth/login", "198.51.100.2", "", loginBody); rec.Code != http.StatusAccepted {
		t.Errorf("other IP: expected 202, got %d", rec.Code)
	}
}

func TestRateLimitIgnoresForgedForwardingHeaders(t *testing.T) {
	t.Parallel()

	// Without a trusted proxy, X-Forwarded-For cannot buy a fresh budget.
	direct := newTestAPI(t).withLimits(&Limits{Unauthenticated: ratelimit.New(60, 1)})
	direct.fromIP(http.MethodPost, "/api/v1/auth/login", "198.51.100.1", "9.9.9.1", loginBody)
	if rec := direct.fromIP(http.MethodPost, "/api/v1/auth/login", "198.51.100.1", "9.9.9.2", loginBody); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("untrusted header must be ignored, got %d", rec.Code)
	}

	// Behind one proxy, the entry the proxy appended identifies the client;
	// entries to its left are client supplied.
	proxied := newTestAPI(t).withLimits(&Limits{Unauthenticated: ratelimit.New(60, 1), TrustedProxyHops: 1})
	if rec := proxied.fromIP(http.MethodPost, "/api/v1/auth/login", "10.0.0.1", "1.1.1.1, 203.0.113.5", loginBody); rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	if rec := proxied.fromIP(http.MethodPost, "/api/v1/auth/login", "10.0.0.1", "2.2.2.2, 203.0.113.5", loginBody); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("forged leftmost entry must not reset the budget, got %d", rec.Code)
	}
	if rec := proxied.fromIP(http.MethodPost, "/api/v1/auth/login", "10.0.0.1", "203.0.113.6", loginBody); rec.Code != http.StatusAccepted {
		t.Fatalf("a different client behind the proxy has its own budget, got %d", rec.Code)
	}
}

func TestAuthenticatedRequestsAreLimitedPerSession(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t).withLimits(&Limits{Authenticated: ratelimit.New(60, 2)})
	other := api.newUser("other@example.com", "Other")

	api.do(http.MethodGet, "/api/v1/auth/me", nil).expect(http.StatusOK)
	api.do(http.MethodGet, "/api/v1/accounts", nil).expect(http.StatusOK)
	body := api.do(http.MethodGet, "/api/v1/accounts", nil).expectError(http.StatusTooManyRequests)
	if !strings.Contains(body.Detail, "rate limit exceeded") {
		t.Fatalf("unexpected detail: %q", body.Detail)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
	req.Header.Set("Authorization", "Bearer "+api.token)
	if rec := api.raw(req); rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("expected 429 with Retry-After, got %d %q", rec.Code, rec.Header().Get("Retry-After"))
	}

	// Other sessions, anonymous calls and invalid tokens are unaffected by it.
	other.do(http.MethodGet, "/api/v1/accounts", nil).expect(http.StatusOK)
	api.as("").do(http.MethodGet, "/healthz", nil).expect(http.StatusOK)
	api.as("not-a-real-token").do(http.MethodGet, "/api/v1/accounts", nil).expectError(http.StatusUnauthorized)
}

func TestNoLimitsByDefault(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	for i := 0; i < 20; i++ {
		api.do(http.MethodGet, "/api/v1/auth/me", nil).expect(http.StatusOK)
		api.as("").do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "nobody@example.com"}).expect(http.StatusAccepted)
	}
}

// TestCodeVerificationLocksAfterWrongGuesses guards against brute forcing the
// six-digit code: after five wrong guesses the challenge is burned and even
// the right code is rejected.
func TestCodeVerificationLocksAfterWrongGuesses(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	anon := api.as("")

	anon.do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "owner@example.com"}).expect(http.StatusAccepted)
	_, code := api.mail.LastLogin(t)
	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	for i := 0; i < 5; i++ {
		anon.do(http.MethodPost, "/api/v1/auth/verify", map[string]any{"email": "owner@example.com", "code": wrong}).expectError(http.StatusUnauthorized)
	}
	anon.do(http.MethodPost, "/api/v1/auth/verify", map[string]any{"email": "owner@example.com", "code": code}).expectError(http.StatusUnauthorized)

	// A fresh email starts a new challenge that works.
	anon.do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "owner@example.com"}).expect(http.StatusAccepted)
	_, code = api.mail.LastLogin(t)
	anon.do(http.MethodPost, "/api/v1/auth/verify", map[string]any{"email": "owner@example.com", "code": code}).expect(http.StatusOK)
}
