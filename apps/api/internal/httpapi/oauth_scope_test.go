package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
)

const fullScope = "finance:read finance:write"

// mcpToolNames lists the tools an MCP connection made with the token sees.
func mcpToolNames(t *testing.T, session *mcp.ClientSession) []string {
	t.Helper()
	res, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, len(res.Tools))
	for i, tool := range res.Tools {
		names[i] = tool.Name
	}
	return names
}

// assertCanWrite checks what an access token can do over REST and MCP.
func assertCanWrite(t *testing.T, api *testAPI, token string, canWrite bool) {
	t.Helper()
	asToken := api.as(token)
	asToken.do(http.MethodGet, "/api/v1/accounts", nil).expect(http.StatusOK)
	asToken.do(http.MethodGet, "/api/v1/auth/me", nil).expect(http.StatusOK)

	session := connectMCP(t, api, token)
	tools := mcpToolNames(t, session)
	if !slices.Contains(tools, "list_accounts") || slices.Contains(tools, "add_expense") != canWrite {
		t.Fatalf("canWrite=%v: unexpected tools %v", canWrite, tools)
	}
	readOnlyNote := strings.Contains(session.InitializeResult().Instructions, "This connection is read-only")
	if readOnlyNote == canWrite {
		t.Fatalf("canWrite=%v: read-only note in instructions = %v", canWrite, readOnlyNote)
	}
	if res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_accounts", Arguments: map[string]any{}}); err != nil || res.IsError {
		t.Fatalf("list_accounts: %v %+v", err, res)
	}

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "create_category", Arguments: map[string]any{"name": "Probe " + token[:8], "kind": "expense"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError == canWrite {
		t.Fatalf("canWrite=%v: create_category result %+v", canWrite, res.Content)
	}
	if !canWrite {
		if text := res.Content[0].(*mcp.TextContent).Text; !strings.Contains(text, "read-only") {
			t.Fatalf("refusal should say the connection is read-only: %s", text)
		}
		e := asToken.do(http.MethodPost, "/api/v1/accounts", map[string]any{"name": "Nope", "type": "cash", "currency": "EUR"}).expectError(http.StatusForbidden)
		if !strings.Contains(e.Detail, "read-only") {
			t.Fatalf("unexpected REST refusal %q", e.Detail)
		}
		asToken.do(http.MethodPost, "/api/v1/auth/logout", nil).expectError(http.StatusForbidden)
		asToken.do(http.MethodDelete, "/api/v1/auth/connections/"+missingID, nil).expectError(http.StatusForbidden)
	} else {
		asToken.do(http.MethodPost, "/api/v1/accounts", map[string]any{"name": "Probe " + token[:8], "type": "cash", "currency": "EUR"}).expect(http.StatusCreated)
	}
}

// TestOAuthReadOnlyConnection chooses "Read only" on the consent page: the
// tokens and their refreshes carry finance:read alone, write tools are
// hidden and refused over MCP, and REST writes are refused.
func TestOAuthReadOnlyConnection(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	api.createAccount("Checking", "EUR", 0)
	client := newOAuthClient(t, api, "none")

	tokens := client.exchange(client.authorizeAccess(t, "owner@example.com", "read_only"))
	if tokens.Status != http.StatusOK || tokens.Scope != "finance:read" {
		t.Fatalf("expected a finance:read token: %+v", tokens)
	}
	assertCanWrite(t, api, tokens.AccessToken, false)

	// Asking for more on refresh is refused, and does not consume the
	// refresh token.
	if res := client.token(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tokens.RefreshToken}, "scope": {fullScope}}); res.Status != http.StatusBadRequest || res.Error != "invalid_scope" {
		t.Fatalf("widening on refresh: %+v", res)
	}
	refreshed := client.refresh(tokens.RefreshToken)
	if refreshed.Status != http.StatusOK || refreshed.Scope != "finance:read" {
		t.Fatalf("refresh should keep finance:read: %+v", refreshed)
	}
	assertCanWrite(t, api, refreshed.AccessToken, false)
	// Asking for what was granted (or unknown scopes) is fine.
	again := client.token(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshed.RefreshToken}, "scope": {"finance:read openid"}})
	if again.Status != http.StatusOK || again.Scope != "finance:read" {
		t.Fatalf("refresh with the granted scope: %+v", again)
	}

	connections := api.listConnections()
	if len(connections) != 1 || !slices.Equal(connections[0].Scopes, []string{"finance:read"}) {
		t.Fatalf("connection should show read-only access: %+v", connections)
	}
}

// TestOAuthReadWriteConnection keeps the default (read and write): the
// consent page preselects it and tokens keep both scopes through refresh.
func TestOAuthReadWriteConnection(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")

	requestID := client.startAuthorization(t)
	page := client.submit(requestID, map[string]string{"action": "send_code", "email": "owner@example.com"}).Body.String()
	if !strings.Contains(page, `value="read_write" checked`) || !strings.Contains(page, `value="read_only">`) || !strings.Contains(page, "Read only") {
		t.Fatalf("consent page should offer both, read and write by default:\n%s", page)
	}

	for _, access := range []string{"read_write", ""} {
		tokens := client.exchange(client.authorizeAccess(t, "owner@example.com", access))
		if tokens.Status != http.StatusOK || tokens.Scope != fullScope {
			t.Fatalf("access %q: expected read and write: %+v", access, tokens)
		}
		refreshed := client.refresh(tokens.RefreshToken)
		if refreshed.Status != http.StatusOK || refreshed.Scope != fullScope {
			t.Fatalf("refresh should keep both scopes: %+v", refreshed)
		}
		assertCanWrite(t, api, refreshed.AccessToken, true)
	}
	for _, c := range api.listConnections() {
		if !slices.Equal(c.Scopes, []string{"finance:read", "finance:write"}) {
			t.Fatalf("connection should show read and write: %+v", c)
		}
	}
}

// TestOAuthClientRequestsReadOnly: a client asking for finance:read alone
// is never granted write access, whatever the form says.
func TestOAuthClientRequestsReadOnly(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")
	client.scope = "finance:read"

	requestID := client.startAuthorization(t)
	page := client.submit(requestID, map[string]string{"action": "send_code", "email": "owner@example.com"}).Body.String()
	if strings.Contains(page, `value="read_write"`) || !strings.Contains(page, `value="read_only" checked`) {
		t.Fatalf("only read access should be offered:\n%s", page)
	}
	tokens := client.exchange(client.authorizeAccess(t, "owner@example.com", "read_write"))
	if tokens.Status != http.StatusOK || tokens.Scope != "finance:read" {
		t.Fatalf("expected finance:read: %+v", tokens)
	}
}

// TestOAuthDefaultScope: clients sending no scope, the legacy one, unknown
// ones or finance:write get the default, read and write.
func TestOAuthDefaultScope(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")
	for _, scope := range []string{"", "finance", "openid profile", "finance:write"} {
		client.scope = scope
		if tokens := client.exchange(client.authorize(t, "owner@example.com")); tokens.Scope != fullScope {
			t.Fatalf("scope %q: expected read and write, got %+v", scope, tokens)
		}
	}
}

// TestOAuthReadOnlyChoiceSurvivesWorkspaceStep: with several workspaces the
// access chosen with the code applies once the workspace is picked.
func TestOAuthReadOnlyChoiceSurvivesWorkspaceStep(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	trip, err := api.workspaces.Create(context.Background(), api.owner.ID, "Trip")
	if err != nil {
		t.Fatal(err)
	}
	client := newOAuthClient(t, api, "none")
	requestID := client.startAuthorization(t)
	client.submit(requestID, map[string]string{"action": "send_code", "email": "owner@example.com"})
	if rec := client.submit(requestID, map[string]string{"action": "verify", "code": api.mail.LastCode(t), "access": "read_only"}); rec.Code != http.StatusOK {
		t.Fatalf("expected the workspace step, got %d", rec.Code)
	}
	rec := client.submit(requestID, map[string]string{"action": "choose_workspace", "workspace_id": trip.ID.String(), "access": "read_write"})
	if rec.Code != http.StatusFound {
		t.Fatalf("choose workspace: %d %s", rec.Code, rec.Body)
	}
	loc, _ := url.Parse(rec.Header().Get("Location"))
	if tokens := client.exchange(loc.Query().Get("code")); tokens.Scope != "finance:read" {
		t.Fatalf("expected finance:read after the workspace step: %+v", tokens)
	}
}

// TestOAuthInvalidCodeKeepsAccessChoice re-renders the consent page with the
// user's choice after a wrong code.
func TestOAuthInvalidCodeKeepsAccessChoice(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")
	requestID := client.startAuthorization(t)
	client.submit(requestID, map[string]string{"action": "send_code", "email": "owner@example.com"})
	rec := client.submit(requestID, map[string]string{"action": "verify", "code": "000000", "access": "read_only"})
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `value="read_only" checked`) {
		t.Fatalf("expected the code step with read only selected, got %d:\n%s", rec.Code, rec.Body)
	}
}

// TestOAuthLegacyConnectionKeepsFullAccess: connections authorized before
// scopes existed (grant scope "finance", access tokens without scope) keep
// read and write access, and refreshing gives them both new scopes.
func TestOAuthLegacyConnectionKeepsFullAccess(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")
	tokens := client.exchange(client.authorize(t, "owner@example.com"))

	ctx := context.Background()
	family, err := api.oauth.ListConnections(ctx, api.owner.ID)
	if err != nil || len(family) != 1 {
		t.Fatalf("connections: %v %v", family, err)
	}
	for _, stmt := range []string{
		`UPDATE oauth_grants SET scope = 'finance' WHERE id = $1`,
		`UPDATE oauth_refresh_tokens SET scope = 'finance' WHERE family_id = $1`,
		`UPDATE sessions SET scope = '' WHERE oauth_family_id = $1`,
	} {
		if _, err := api.pool.Exec(ctx, stmt, family[0].ID); err != nil {
			t.Fatal(err)
		}
	}

	if session, err := api.auth.Authenticate(ctx, tokens.AccessToken); err != nil || !session.CanWrite() {
		t.Fatalf("legacy token should write: %v", err)
	}
	assertCanWrite(t, api, tokens.AccessToken, true)
	if c := api.listConnections(); len(c) != 1 || !slices.Equal(c[0].Scopes, []string{auth.ScopeRead, auth.ScopeWrite}) {
		t.Fatalf("legacy connection should show read and write: %+v", c)
	}
	// Legacy clients may repeat their scope on refresh.
	refreshed := client.token(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tokens.RefreshToken}, "scope": {"finance"}})
	if refreshed.Status != http.StatusOK || refreshed.Scope != fullScope {
		t.Fatalf("legacy refresh: %+v", refreshed)
	}
	assertCanWrite(t, api, refreshed.AccessToken, true)
}

func TestOAuthConsentPageAccessIsLocalized(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	client := newOAuthClient(t, api, "none")
	requestID := client.startAuthorization(t)
	form := url.Values{"request_id": {requestID}, "action": {"send_code"}, "email": {"owner@example.com"}, "locale": {"es"}}
	req := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body := api.raw(req).Body.String()
	if !strings.Contains(body, "Solo lectura") || !strings.Contains(body, "Lectura y escritura") {
		t.Fatalf("expected Spanish access options:\n%s", body)
	}
}
