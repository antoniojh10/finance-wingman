package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
)

// TestWorkspacesAreIsolated seeds data in the owner's workspace and checks
// that a user of another workspace can neither see nor change it, through
// any finance endpoint.
func TestWorkspacesAreIsolated(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Secret checking", "MXN", 1000)
	category := api.createCategory("Secret food", "expense")
	tx := api.createTransaction(map[string]any{
		"type": "expense", "account_id": account.ID, "amount": 500, "category_id": category.ID,
		"description": "Secret lunch", "occurred_on": "2026-01-15",
	})
	item := api.createRecurring(map[string]any{
		"name": "Secret rent", "type": "expense", "account_id": account.ID, "amount": 100000,
		"interval_unit": "month", "start_on": "2026-01-01",
	})
	secrets := []string{"Secret", account.ID.String(), category.ID.String(), tx.ID.String(), item.ID.String()}

	other := api.newUser("other@example.com", "Other")
	other.createAccount("Other checking", "MXN", 0)

	for _, path := range []string{
		"/api/v1/accounts?include_archived=true", "/api/v1/categories?include_archived=true",
		"/api/v1/transactions", "/api/v1/summary", "/api/v1/recurring", "/api/v1/recurring/summary",
		"/api/v1/recurring/upcoming", "/api/v1/recurring/suggestions",
	} {
		body := string(other.do(http.MethodGet, path, nil).expect(http.StatusOK).Body)
		for _, secret := range secrets {
			if strings.Contains(body, secret) {
				t.Fatalf("GET %s leaks %q: %s", path, secret, body)
			}
		}
	}

	notFound := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/api/v1/accounts/" + account.ID.String(), nil},
		{http.MethodPatch, "/api/v1/accounts/" + account.ID.String(), map[string]any{"name": "Hijacked"}},
		{http.MethodDelete, "/api/v1/accounts/" + account.ID.String(), nil},
		{http.MethodGet, "/api/v1/categories/" + category.ID.String(), nil},
		{http.MethodPatch, "/api/v1/categories/" + category.ID.String(), map[string]any{"name": "Hijacked"}},
		{http.MethodDelete, "/api/v1/categories/" + category.ID.String(), nil},
		{http.MethodGet, "/api/v1/transactions/" + tx.ID.String(), nil},
		{http.MethodDelete, "/api/v1/transactions/" + tx.ID.String(), nil},
		{http.MethodGet, "/api/v1/transactions/" + tx.ID.String() + "/recurring-match", nil},
		{http.MethodGet, "/api/v1/recurring/" + item.ID.String(), nil},
		{http.MethodPatch, "/api/v1/recurring/" + item.ID.String(), map[string]any{"name": "Hijacked"}},
		{http.MethodPost, "/api/v1/recurring/" + item.ID.String() + "/payments", map[string]any{}},
	}
	for _, req := range notFound {
		if res := other.do(req.method, req.path, req.body); res.Status != http.StatusNotFound {
			t.Fatalf("%s %s: expected 404, got %d: %s", req.method, req.path, res.Status, res.Body)
		}
	}

	// Ids from another workspace can't be referenced either.
	res := other.do(http.MethodPost, "/api/v1/transactions", map[string]any{
		"type": "expense", "account_id": account.ID, "amount": 100,
	})
	if res.Status < 400 || res.Status >= 500 {
		t.Fatalf("transaction on another workspace's account: expected a client error, got %d: %s", res.Status, res.Body)
	}

	// The owner's data is untouched.
	if got := api.getAccount(account.ID.String()); got.Name != "Secret checking" || got.Balance != 500 {
		t.Fatalf("owner's account changed: %+v", got)
	}
	api.do(http.MethodGet, "/api/v1/transactions/"+tx.ID.String(), nil).expect(http.StatusOK)
}

func TestFinanceRequiresWorkspace(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	user, err := api.auth.AddUser(context.Background(), "nomad@example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	session, err := api.auth.CreateSession(context.Background(), user.ID, auth.ClientWeb, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if session.Workspace != nil {
		t.Fatalf("a user without workspaces should get a session without one: %+v", session.Workspace)
	}
	nomad := api.as(session.Token)

	for _, path := range []string{"/api/v1/accounts", "/api/v1/transactions", "/api/v1/summary"} {
		if body := nomad.do(http.MethodGet, path, nil).expectError(http.StatusConflict); body.Detail != errNoWorkspace {
			t.Fatalf("GET %s: unexpected error %q", path, body.Detail)
		}
	}
	nomad.do(http.MethodPost, "/api/v1/accounts", map[string]any{"name": "Cash", "type": "cash", "currency": "MXN"}).
		expectError(http.StatusConflict)
	// Endpoints about the user still work.
	nomad.do(http.MethodGet, "/api/v1/auth/me", nil).expect(http.StatusOK)
}

// TestMCPActsOnTheSessionWorkspace checks that MCP tools only see the
// workspace of the bearer session.
func TestMCPActsOnTheSessionWorkspace(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	api.createAccount("Secret checking", "MXN", 0)
	other := api.newUser("other@example.com", "Other")
	other.createAccount("Other checking", "MXN", 0)

	session := connectMCP(t, api, other.token)
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_accounts", Arguments: map[string]any{}})
	if err != nil || res.IsError {
		t.Fatalf("list_accounts: %v %+v", err, res)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if strings.Contains(text, "Secret") || !strings.Contains(text, "Other checking") {
		t.Fatalf("list_accounts should only show the other workspace: %s", text)
	}

	// A session without a workspace gets an actionable tool error.
	user, err := api.auth.AddUser(context.Background(), "nomad@example.com", "")
	if err != nil {
		t.Fatal(err)
	}
	nomad, err := api.auth.CreateSession(context.Background(), user.ID, auth.ClientWeb, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	res, err = connectMCP(t, api, nomad.Token).CallTool(context.Background(), &mcp.CallToolParams{Name: "list_accounts", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "reconnect") {
		t.Fatalf("expected a no-workspace tool error, got %+v", res)
	}
}
