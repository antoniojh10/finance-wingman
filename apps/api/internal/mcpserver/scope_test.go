package mcpserver

import (
	"context"
	"slices"
	"strings"
	"testing"

	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

var readTools = []string{
	"list_accounts", "list_categories", "get_summary", "list_transactions", "list_recurring",
	"list_upcoming_recurring", "list_recurring_suggestions", "get_budget_status", "suggest_budgets",
}

// connect opens another client session on the harness server, so the
// initialize result reflects the harness' current scope.
func (h *harness) connect() *mcp.ClientSession {
	h.t.Helper()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	if _, err := h.server.MCP().Connect(context.Background(), serverTransport, nil); err != nil {
		h.t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil).Connect(context.Background(), clientTransport, nil)
	if err != nil {
		h.t.Fatal(err)
	}
	h.t.Cleanup(func() { session.Close() })
	return session
}

func (h *harness) toolNames() []string {
	h.t.Helper()
	res, err := h.session.ListTools(context.Background(), nil)
	if err != nil {
		h.t.Fatal(err)
	}
	names := make([]string, len(res.Tools))
	for i, tool := range res.Tools {
		names[i] = tool.Name
	}
	slices.Sort(names)
	return names
}

func TestReadOnlyConnectionListsOnlyReadTools(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	full := h.toolNames()
	if len(full) != len(h.server.readOnly) || !slices.Contains(full, "add_expense") {
		t.Fatalf("a read and write connection should list every tool, got %v", full)
	}

	h.readOnly = true
	got := h.toolNames()
	want := slices.Clone(readTools)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("read-only connection lists %v, want %v", got, want)
	}
	// Every tool flagged read-only is annotated as such.
	for name, readOnly := range h.server.readOnly {
		if readOnly != slices.Contains(readTools, name) {
			t.Errorf("tool %s: read-only = %v", name, readOnly)
		}
	}
}

func TestReadOnlyConnectionRefusesWriteTools(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	account := h.account("Checking", "EUR")
	h.category("Food", "expense")
	var created transactionOut
	h.mustCall("add_expense", map[string]any{"amount": 10, "account": "Checking"}, &created)

	h.readOnly = true
	h.mustFail("add_expense", map[string]any{"amount": 5, "account": "Checking"}, "read-only")
	h.mustFail("delete_transaction", map[string]any{"id": created.ID}, "Read and write")
	h.mustFail("create_category", map[string]any{"name": "Rent", "kind": "expense"}, "nothing was changed")
	h.mustFail("set_budgets", map[string]any{"items": []any{}}, "read-only")

	page, err := h.svc.ListTransactions(h.ctx, finance.TransactionFilter{AccountID: &account.ID})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 {
		t.Fatalf("read-only calls must not change data, got %d transactions", page.Total)
	}
	categories, err := h.svc.ListCategories(h.ctx, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) != 1 {
		t.Fatalf("read-only calls must not create categories: %+v", categories)
	}

	// Reading still works, and unknown tools stay protocol errors.
	h.mustCall("list_transactions", map[string]any{}, nil)
	h.mustCall("get_summary", map[string]any{}, nil)
	if _, err := h.session.CallTool(context.Background(), &mcp.CallToolParams{Name: "no_such_tool"}); err == nil {
		t.Fatal("unknown tool should fail")
	}
}

func TestReadOnlyConnectionInstructions(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	if strings.Contains(h.session.InitializeResult().Instructions, "read-only") {
		t.Fatal("a read and write connection should not be told it is read-only")
	}
	h.readOnly = true
	instructions := h.connect().InitializeResult().Instructions
	if !strings.Contains(instructions, "This connection is read-only") || !strings.Contains(instructions, `"Read and write"`) {
		t.Fatalf("read-only instructions should explain how to allow changes:\n%s", instructions)
	}
}

func TestTokenCanWrite(t *testing.T) {
	t.Parallel()
	request := func(info *mcpauth.TokenInfo) mcp.Request {
		return &mcp.CallToolRequest{Extra: &mcp.RequestExtra{TokenInfo: info}}
	}
	cases := []struct {
		name string
		req  mcp.Request
		want bool
	}{
		{"no extra", &mcp.CallToolRequest{}, false},
		{"no token", request(nil), false},
		{"read only", request(&mcpauth.TokenInfo{Scopes: []string{"finance:read"}}), false},
		{"read and write", request(&mcpauth.TokenInfo{Scopes: []string{"finance:read", "finance:write"}}), true},
		{"legacy scope is resolved by the verifier", request(&mcpauth.TokenInfo{Scopes: []string{"finance"}}), false},
	}
	for _, c := range cases {
		if got := tokenCanWrite(c.req); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
