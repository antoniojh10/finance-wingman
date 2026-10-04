package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (b bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(req)
}

func TestMCPRequiresBearerToken(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)

	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := api.raw(req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	challenge := rec.Header().Get("WWW-Authenticate")
	if !strings.Contains(challenge, `resource_metadata="`+testIssuer+`/.well-known/oauth-protected-resource/mcp"`) {
		t.Fatalf("challenge must point to the resource metadata, got %q", challenge)
	}

	req = httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer invalid")
	if rec := api.raw(req); rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token: expected 401, got %d", rec.Code)
	}
}

// TestMCPEndToEnd connects a real MCP client over HTTP with an OAuth access
// token, records an expense, and checks it is attributed to the user.
func TestMCPEndToEnd(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Checking", "MXN", 0)
	api.createCategory("Food", "expense")

	client := newOAuthClient(t, api, "none")
	tokens := client.exchange(client.authorize(t, "owner@example.com"))

	server := httptest.NewServer(api.handler)
	defer server.Close()

	ctx := context.Background()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "claude-test"}, nil).Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   server.URL + "/mcp",
		HTTPClient: &http.Client{Transport: bearerTransport{token: tokens.AccessToken, base: http.DefaultTransport}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "add_expense", Arguments: map[string]any{
		"amount": 89.9, "category": "Food", "description": "Lunch",
	}})
	if err != nil || res.IsError {
		t.Fatalf("add_expense: %v %+v", err, res)
	}

	page, err := api.svc.ListTransactions(ctx, finance.TransactionFilter{AccountID: &account.ID})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].Amount != 8990 || page.Items[0].CreatedBy == nil || page.Items[0].CreatedBy.ID != api.owner.ID {
		t.Fatalf("expense should be stored and attributed to the owner: %+v", page.Items)
	}

	summary, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_summary", Arguments: map[string]any{}})
	if err != nil || summary.IsError {
		t.Fatalf("get_summary: %v %+v", err, summary)
	}
	if text := summary.Content[0].(*mcp.TextContent).Text; !strings.Contains(text, "expenses 89.90") {
		t.Fatalf("unexpected summary: %s", text)
	}
}
