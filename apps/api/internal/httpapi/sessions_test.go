package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
)

const firefoxUA = "Mozilla/5.0 (X11; Linux x86_64; rv:131.0) Gecko/20100101 Firefox/131.0"

type webSessionBody struct {
	ID         string    `json:"id"`
	UserAgent  string    `json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	Current    bool      `json:"current"`
}

func (a *testAPI) listSessions() []webSessionBody {
	a.t.Helper()
	var list struct{ Items []webSessionBody }
	a.do(http.MethodGet, "/api/v1/auth/sessions", nil).expect(http.StatusOK).decode(&list)
	return list.Items
}

// signInWithLink signs in through the magic link endpoint from a browser
// with the given User-Agent and returns the session token.
func (a *testAPI) signInWithLink(email, userAgent string) string {
	a.t.Helper()
	if err := a.auth.RequestLogin(context.Background(), email, ""); err != nil {
		a.t.Fatal(err)
	}
	token, _ := a.mail.LastLogin(a.t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify", strings.NewReader(`{"token":"`+token+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	rec := a.raw(req)
	if rec.Code != http.StatusOK {
		a.t.Fatalf("verify: %d %s", rec.Code, rec.Body)
	}
	var session struct{ Token string }
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil {
		a.t.Fatal(err)
	}
	return session.Token
}

func TestListSessions(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	laptop := api.signInWithLink("owner@example.com", firefoxUA)
	// Another user's sessions are never listed.
	api.newUser("bob@example.com", "Bob's")

	sessions := api.as(laptop).listSessions()
	if len(sessions) != 2 {
		t.Fatalf("expected the owner's 2 sessions, got %+v", sessions)
	}
	current := sessions[0]
	if !current.Current || current.UserAgent != firefoxUA || current.ExpiresAt.Before(time.Now().Add(29*24*time.Hour)) {
		t.Fatalf("unexpected current session %+v", current)
	}
	if sessions[1].Current || sessions[1].UserAgent != "" {
		t.Fatalf("the test client's session has no user agent: %+v", sessions[1])
	}

	// Connected apps' access tokens are not sessions.
	client := newOAuthClient(t, api, "none")
	client.exchange(client.authorize(t, "owner@example.com"))
	if got := api.listSessions(); len(got) != 2 {
		t.Fatalf("expected the 2 browser sessions only, got %+v", got)
	}

	api.as("").do(http.MethodGet, "/api/v1/auth/sessions", nil).expectError(http.StatusUnauthorized)
}

func TestRevokeSession(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	phone := api.signInWithLink("owner@example.com", firefoxUA)
	bob := api.newUser("bob@example.com", "Bob's")

	var phoneID string
	for _, s := range api.listSessions() {
		if s.UserAgent == firefoxUA {
			phoneID = s.ID
		}
	}

	// Users can't sign out other users' sessions.
	bob.do(http.MethodDelete, "/api/v1/auth/sessions/"+phoneID, nil).expectError(http.StatusNotFound)
	api.as(phone).do(http.MethodGet, "/api/v1/auth/me", nil).expect(http.StatusOK)

	api.do(http.MethodDelete, "/api/v1/auth/sessions/"+phoneID, nil).expect(http.StatusNoContent)
	api.as(phone).do(http.MethodGet, "/api/v1/auth/me", nil).expectError(http.StatusUnauthorized)
	api.do(http.MethodDelete, "/api/v1/auth/sessions/"+phoneID, nil).expectError(http.StatusNotFound)
	api.do(http.MethodDelete, "/api/v1/auth/sessions/"+missingID, nil).expectError(http.StatusNotFound)
	api.do(http.MethodDelete, "/api/v1/auth/sessions/not-a-uuid", nil).expectError(http.StatusUnprocessableEntity)
	if got := api.listSessions(); len(got) != 1 || !got[0].Current {
		t.Fatalf("expected only the current session, got %+v", got)
	}

	// Revoking the current session signs out.
	api.do(http.MethodDelete, "/api/v1/auth/sessions/"+api.listSessions()[0].ID, nil).expect(http.StatusNoContent)
	api.do(http.MethodGet, "/api/v1/auth/me", nil).expectError(http.StatusUnauthorized)
}

func TestRevokeOtherSessions(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	others := []string{
		api.signInWithLink("owner@example.com", firefoxUA),
		api.signInWithLink("owner@example.com", "curl/8.0"),
	}
	bob := api.newUser("bob@example.com", "Bob's")
	client := newOAuthClient(t, api, "none")
	tokens := client.exchange(client.authorize(t, "owner@example.com"))

	var out struct{ Revoked int }
	api.do(http.MethodPost, "/api/v1/auth/sessions/revoke-others", nil).expect(http.StatusOK).decode(&out)
	if out.Revoked != 2 {
		t.Fatalf("expected 2 sessions revoked, got %d", out.Revoked)
	}
	for _, token := range others {
		api.as(token).do(http.MethodGet, "/api/v1/auth/me", nil).expectError(http.StatusUnauthorized)
	}
	api.do(http.MethodGet, "/api/v1/auth/me", nil).expect(http.StatusOK)
	// Other users and connected apps are not affected.
	bob.do(http.MethodGet, "/api/v1/auth/me", nil).expect(http.StatusOK)
	api.as(tokens.AccessToken).do(http.MethodGet, "/api/v1/auth/me", nil).expect(http.StatusOK)
	if res := client.refresh(tokens.RefreshToken); res.Status != http.StatusOK {
		t.Fatalf("connected app should keep refreshing: %+v", res)
	}

	api.do(http.MethodPost, "/api/v1/auth/sessions/revoke-others", nil).expect(http.StatusOK).decode(&out)
	if out.Revoked != 0 {
		t.Fatalf("expected nothing left to revoke, got %d", out.Revoked)
	}
}

func TestSessionManagementRequiresBrowserSession(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")
	app := api.as(client.exchange(client.authorize(t, "owner@example.com")).AccessToken)

	app.do(http.MethodGet, "/api/v1/auth/sessions", nil).expectError(http.StatusForbidden)
	app.do(http.MethodPost, "/api/v1/auth/sessions/revoke-others", nil).expectError(http.StatusForbidden)
	app.do(http.MethodDelete, "/api/v1/auth/sessions/"+api.listSessions()[0].ID, nil).expectError(http.StatusForbidden)
	app.do(http.MethodGet, "/api/v1/auth/connections", nil).expectError(http.StatusForbidden)
	app.do(http.MethodDelete, "/api/v1/auth/connections/"+missingID, nil).expectError(http.StatusForbidden)
	api.do(http.MethodGet, "/api/v1/auth/me", nil).expect(http.StatusOK)
}

func TestInvitationSessionRecordsUserAgent(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	api.do(http.MethodPost, "/api/v1/workspaces/"+api.workspace.ID.String()+"/invitations", map[string]any{"email": "carl@example.com"}).
		expect(http.StatusCreated)
	body := `{"token":"` + api.mail.LastInvitationToken(t) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/invitations/accept", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", firefoxUA)
	rec := api.raw(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("accept: %d %s", rec.Code, rec.Body)
	}
	var session auth.Session
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	if got := api.as(session.Token).listSessions(); len(got) != 1 || got[0].UserAgent != firefoxUA {
		t.Fatalf("expected the invitee's session with its user agent, got %+v", got)
	}
}
