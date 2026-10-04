package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
)

func TestProtectedEndpointsRequireSession(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	anon := api.as("")

	for _, path := range []string{"/api/v1/accounts", "/api/v1/categories", "/api/v1/transactions", "/api/v1/summary", "/api/v1/currencies", "/api/v1/auth/me"} {
		t.Run(path, func(t *testing.T) {
			anon.do(http.MethodGet, path, nil).expectError(http.StatusUnauthorized)
			api.as("not-a-real-token").do(http.MethodGet, path, nil).expectError(http.StatusUnauthorized)
		})
	}

	// Malformed Authorization headers are rejected too.
	req := map[string]string{"Basic abc": "", "Bearer": "", "Bearer   ": ""}
	for header := range req {
		if _, ok := bearerToken(header); ok {
			t.Fatalf("bearerToken(%q) should fail", header)
		}
	}
	if token, ok := bearerToken("bearer abc"); !ok || token != "abc" {
		t.Fatal("scheme should be case-insensitive")
	}
}

func TestPublicEndpointsDoNotRequireSession(t *testing.T) {
	t.Parallel()
	anon := newTestAPI(t).as("")
	anon.do(http.MethodGet, "/healthz", nil).expect(http.StatusOK)
	anon.do(http.MethodGet, "/openapi.json", nil).expect(http.StatusOK)
	anon.do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "nobody@example.com"}).expect(http.StatusAccepted)
}

func TestMagicLinkLogin(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	anon := api.as("")

	anon.do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "  OWNER@example.com "}).expect(http.StatusAccepted)
	msgs := api.mail.Messages()
	if len(msgs) != 1 || msgs[0].To != "owner@example.com" {
		t.Fatalf("expected one email to the owner, got %+v", msgs)
	}
	if !strings.Contains(msgs[0].Text, "http://web.test/auth/verify?token=") {
		t.Fatalf("expected link to the web app:\n%s", msgs[0].Text)
	}
	token, _ := api.mail.LastLogin(t)

	var session auth.Session
	anon.do(http.MethodPost, "/api/v1/auth/verify", map[string]any{"token": token}).expect(http.StatusOK).decode(&session)
	if session.Token == "" || session.User.Email != "owner@example.com" || session.ExpiresAt.IsZero() {
		t.Fatalf("unexpected session: %+v", session)
	}

	// The new session works, and the link cannot be reused.
	var me auth.User
	api.as(session.Token).do(http.MethodGet, "/api/v1/auth/me", nil).expect(http.StatusOK).decode(&me)
	if me.ID != api.owner.ID {
		t.Fatalf("unexpected user: %+v", me)
	}
	anon.do(http.MethodPost, "/api/v1/auth/verify", map[string]any{"token": token}).expectError(http.StatusUnauthorized)
}

func TestCodeLogin(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	anon := api.as("")

	anon.do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "owner@example.com", "locale": "es"}).expect(http.StatusAccepted)
	token, code := api.mail.LastLogin(t)
	if msgs := api.mail.Messages(); !strings.Contains(msgs[0].Subject, "Finance Wingman") || !strings.HasPrefix(msgs[0].Text, "Hola") {
		t.Fatalf("expected a Spanish email, got %q", msgs[0].Text)
	}

	wrong := "000000"
	if code == wrong {
		wrong = "111111"
	}
	anon.do(http.MethodPost, "/api/v1/auth/verify", map[string]any{"email": "owner@example.com", "code": wrong}).expectError(http.StatusUnauthorized)

	var session auth.Session
	anon.do(http.MethodPost, "/api/v1/auth/verify", map[string]any{"email": "Owner@Example.com", "code": code}).expect(http.StatusOK).decode(&session)
	if session.User.ID != api.owner.ID {
		t.Fatalf("unexpected session: %+v", session)
	}

	// Redeeming with the code also invalidates the link from the same email.
	anon.do(http.MethodPost, "/api/v1/auth/verify", map[string]any{"token": token}).expectError(http.StatusUnauthorized)
	anon.do(http.MethodPost, "/api/v1/auth/verify", map[string]any{"email": "owner@example.com", "code": code}).expectError(http.StatusUnauthorized)
}

func TestLoginRequestValidation(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	anon := api.as("")

	// Unknown addresses get the same response and no email.
	anon.do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "stranger@example.com"}).expect(http.StatusAccepted)
	if n := len(api.mail.Messages()); n != 0 {
		t.Fatalf("expected no email for unknown address, got %d", n)
	}

	anon.do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "not-an-email"}).expect(http.StatusUnprocessableEntity)
	anon.do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": "a@b.co", "locale": "fr"}).expect(http.StatusUnprocessableEntity)
	anon.do(http.MethodPost, "/api/v1/auth/verify", map[string]any{}).expectError(http.StatusUnprocessableEntity)
	anon.do(http.MethodPost, "/api/v1/auth/verify", map[string]any{"email": "owner@example.com", "code": "12"}).expect(http.StatusUnprocessableEntity)
	anon.do(http.MethodPost, "/api/v1/auth/verify", map[string]any{"token": "forged"}).expectError(http.StatusUnauthorized)
	anon.do(http.MethodPost, "/api/v1/auth/verify", map[string]any{"email": "stranger@example.com", "code": "123456"}).expectError(http.StatusUnauthorized)
}

func TestUpdateCurrentUser(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)

	var user auth.User
	api.do(http.MethodPatch, "/api/v1/auth/me", map[string]any{"name": " Ana ", "locale": "es"}).expect(http.StatusOK).decode(&user)
	if user.Name != "Ana" || user.Locale != "es" || user.Email != "owner@example.com" {
		t.Fatalf("unexpected user: %+v", user)
	}
	api.do(http.MethodPatch, "/api/v1/auth/me", map[string]any{"locale": "de"}).expect(http.StatusUnprocessableEntity)
	api.as("").do(http.MethodPatch, "/api/v1/auth/me", map[string]any{"name": "x"}).expectError(http.StatusUnauthorized)
}

func TestLogout(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)

	api.do(http.MethodPost, "/api/v1/auth/logout", nil).expect(http.StatusNoContent)
	api.do(http.MethodGet, "/api/v1/auth/me", nil).expectError(http.StatusUnauthorized)
	api.do(http.MethodPost, "/api/v1/auth/logout", nil).expectError(http.StatusUnauthorized)
}

func TestOpenAPIDocumentsSecurity(t *testing.T) {
	t.Parallel()
	var spec struct {
		Security   []map[string][]string `json:"security"`
		Components struct {
			SecuritySchemes map[string]any `json:"securitySchemes"`
		} `json:"components"`
	}
	newTestAPI(t).as("").do(http.MethodGet, "/openapi.json", nil).expect(http.StatusOK).decode(&spec)
	if _, ok := spec.Components.SecuritySchemes["bearer"]; !ok || len(spec.Security) == 0 {
		t.Fatalf("expected global bearer security, got %+v", spec)
	}
}
