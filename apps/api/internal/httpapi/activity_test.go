package httpapi

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
)

func (a *testAPI) activity(query string) finance.ActivityPage {
	a.t.Helper()
	var page finance.ActivityPage
	a.do(http.MethodGet, "/api/v1/activity"+query, nil).expect(http.StatusOK).decode(&page)
	return page
}

func actions(page finance.ActivityPage) []string {
	out := make([]string, len(page.Items))
	for i, e := range page.Items {
		out[i] = e.Action
	}
	return out
}

func TestActivityRecordsTransactionChanges(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Checking", "MXN", 0)
	savings := api.createAccount("Savings", "MXN", 0)

	tx := api.createTransaction(map[string]any{
		"type": "expense", "account_id": account.ID, "amount": 123456, "description": "Secret lunch",
	})
	api.do(http.MethodPut, "/api/v1/transactions/"+tx.ID.String(), map[string]any{
		"type": "expense", "account_id": account.ID, "amount": 654321, "description": "Secret dinner",
	}).expect(http.StatusOK)
	// Saving it unchanged is not a change.
	api.do(http.MethodPut, "/api/v1/transactions/"+tx.ID.String(), map[string]any{
		"type": "expense", "account_id": account.ID, "amount": 654321, "description": "Secret dinner",
	}).expect(http.StatusOK)
	api.do(http.MethodDelete, "/api/v1/transactions/"+tx.ID.String(), nil).expect(http.StatusNoContent)
	transfer := api.createTransaction(map[string]any{
		"type": "transfer", "account_id": account.ID, "destination_account_id": savings.ID, "amount": 777,
	})

	res := api.do(http.MethodGet, "/api/v1/activity", nil).expect(http.StatusOK)
	// Metadata only: no descriptions, account names or amounts.
	for _, secret := range []string{"Secret", "Checking", "Savings", `"amount":`, `"destination_amount":`} {
		if strings.Contains(string(res.Body), secret) {
			t.Fatalf("the log must hold metadata only, found %q in %s", secret, res.Body)
		}
	}
	var page finance.ActivityPage
	res.decode(&page)
	want := []string{"transaction.created", "transaction.deleted", "transaction.updated", "transaction.created"}
	if got := actions(page); !slices.Equal(got, want) {
		t.Fatalf("expected %v (newest first), got %v", want, got)
	}
	for _, e := range page.Items {
		if e.Channel != "web" || e.ClientID != nil || e.ClientName != nil || e.EntityType != "transaction" || e.Actor == nil || e.Actor.ID != api.owner.ID {
			t.Fatalf("entry should be a web change by the owner: %+v", e)
		}
	}
	if d := page.Items[0].Details; page.Items[0].EntityID != transfer.ID || d.Type != "transfer" ||
		d.AccountID == nil || *d.AccountID != account.ID || d.DestinationAccountID == nil || *d.DestinationAccountID != savings.ID {
		t.Fatalf("transfer entry should name both accounts: %+v", page.Items[0])
	}
	if d := page.Items[1].Details; d.Type != "expense" || d.AccountID == nil || *d.AccountID != account.ID || d.Changed != nil {
		t.Fatalf("delete entry should keep the type and account: %+v", d)
	}
	if changed := page.Items[2].Details.Changed; !slices.Equal(changed, []string{"amount", "description"}) {
		t.Fatalf("update should name the changed fields: %v", changed)
	}
	if page.NextCursor != nil {
		t.Fatalf("a single page should have no cursor: %v", *page.NextCursor)
	}
}

func TestActivityIgnoresFailedChanges(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Checking", "MXN", 0)
	tx := api.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 1})

	api.do(http.MethodPost, "/api/v1/transactions", map[string]any{
		"type": "expense", "account_id": account.ID, "amount": 1, "category_id": missingID,
	}).expectError(http.StatusUnprocessableEntity)
	api.do(http.MethodPut, "/api/v1/transactions/"+tx.ID.String(), map[string]any{
		"type": "expense", "account_id": missingID, "amount": 1,
	}).expectError(http.StatusUnprocessableEntity)
	api.do(http.MethodDelete, "/api/v1/transactions/"+missingID, nil).expectError(http.StatusNotFound)

	if page := api.activity(""); len(page.Items) != 1 {
		t.Fatalf("failed changes must not be logged: %v", actions(page))
	}
}

func TestActivityBatchIsAtomic(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Checking", "MXN", 0)

	valid := map[string]any{"type": "expense", "account_id": account.ID, "amount": 100}
	api.do(http.MethodPost, "/api/v1/transactions/batch", map[string]any{"items": []any{valid, valid, map[string]any{"type": "bogus"}}}).
		expectError(http.StatusUnprocessableEntity)
	if page := api.activity(""); len(page.Items) != 0 {
		t.Fatalf("a rolled back batch must leave no entries: %v", actions(page))
	}

	var created struct {
		Items []finance.Transaction `json:"items"`
	}
	api.do(http.MethodPost, "/api/v1/transactions/batch", map[string]any{"items": []any{valid, valid}}).
		expect(http.StatusCreated).decode(&created)
	page := api.activity("")
	if len(page.Items) != 2 || len(created.Items) != 2 {
		t.Fatalf("expected one entry per item: %v", actions(page))
	}
	// Entries of one database transaction keep their order.
	if page.Items[0].EntityID != created.Items[1].ID || page.Items[1].EntityID != created.Items[0].ID {
		t.Fatalf("batch entries should be newest first: %+v", page.Items)
	}
}

func TestActivityRecordsRecurringLinks(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	item := api.weeklyItem(account, nil)
	tx := api.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 50000})

	path := "/api/v1/transactions/" + tx.ID.String() + "/recurring"
	api.do(http.MethodPut, path, map[string]any{"recurring_id": item.ID}).expect(http.StatusOK)
	api.do(http.MethodDelete, path, nil).expect(http.StatusOK)
	// Unlinking an unlinked transaction changes nothing.
	api.do(http.MethodDelete, path, nil).expect(http.StatusOK)
	// Paying the item creates a transaction already linked: one entry.
	api.do(http.MethodPost, "/api/v1/recurring/"+item.ID.String()+"/payments", map[string]any{}).expect(http.StatusCreated)

	page := api.activity("")
	want := []string{"transaction.created", "transaction.updated", "transaction.updated", "transaction.created"}
	if got := actions(page); !slices.Equal(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for _, e := range page.Items[1:3] {
		if e.EntityID != tx.ID || !slices.Equal(e.Details.Changed, []string{"recurring_id", "recurring_due_on"}) {
			t.Fatalf("link changes should name the link fields: %+v", e)
		}
	}
}

func TestActivityRecordsAcceptedSuggestionLinks(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Card", "MXN", 0)
	txs := api.seedMonthly(account, "Netflix", nil, 1000, 1000, 1000)
	s := api.suggestions()
	if len(s) != 1 {
		t.Fatalf("expected one suggestion: %+v", s)
	}
	api.do(http.MethodPost, "/api/v1/recurring/suggestions/accept", map[string]any{"key": s[0].Key}).expect(http.StatusCreated)

	page := api.activity("?entity_id=" + txs[0].ID.String())
	if got := actions(page); !slices.Equal(got, []string{"transaction.updated", "transaction.created"}) {
		t.Fatalf("linking a suggestion should log the transaction update: %v", got)
	}
}

func TestActivityRecordsMCPClient(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	api.createAccount("Checking", "MXN", 0)

	client := newOAuthClient(t, api, "none")
	tokens := client.exchange(client.authorize(t, "owner@example.com"))
	session := connectMCP(t, api, tokens.AccessToken)

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "add_expense", Arguments: map[string]any{"amount": 12.5, "description": "Coffee"}})
	if err != nil || res.IsError {
		t.Fatalf("add_expense: %v %+v", err, res)
	}
	created := api.activity("?channel=mcp")
	if len(created.Items) != 1 {
		t.Fatalf("expected one MCP entry: %v", actions(created))
	}
	res, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "delete_transaction", Arguments: map[string]any{"id": created.Items[0].EntityID.String()}})
	if err != nil || res.IsError {
		t.Fatalf("delete_transaction: %v %+v", err, res)
	}
	// The OAuth token also works on the REST API: still the MCP client.
	account := api.createAccount("Cash", "MXN", 0)
	api.as(tokens.AccessToken).createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 1})

	page := api.activity("?channel=mcp")
	if got := actions(page); !slices.Equal(got, []string{"transaction.created", "transaction.deleted", "transaction.created"}) {
		t.Fatalf("unexpected MCP actions: %v", got)
	}
	for _, e := range page.Items {
		if e.ClientID == nil || *e.ClientID != client.clientID || e.ClientName == nil || *e.ClientName != "Claude" || e.Actor == nil || e.Actor.ID != api.owner.ID {
			t.Fatalf("entry should name the OAuth client and the owner: %+v", e)
		}
	}
	if web := api.activity("?channel=web"); len(web.Items) != 0 {
		t.Fatalf("MCP changes must not be listed as web: %v", actions(web))
	}
}

func TestListActivityFilters(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	ana := testutil.NewMember(t, api.pool, api.workspace.ID, "ana@example.com", "Ana")
	account := api.createAccount("Checking", "MXN", 0)
	first := api.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 1})
	if _, err := api.svc.CreateTransaction(finance.WithActor(api.ctx, ana), finance.TransactionInput{Type: "expense", AccountID: account.ID, Amount: 2}); err != nil {
		t.Fatal(err)
	}

	if page := api.activity("?entity_id=" + first.ID.String()); len(page.Items) != 1 || page.Items[0].EntityID != first.ID {
		t.Fatalf("entity filter: %+v", page)
	}
	if page := api.activity("?entity_type=transaction"); len(page.Items) != 2 {
		t.Fatalf("entity type filter: %+v", page)
	}
	if page := api.activity("?actor_id=" + ana.String()); len(page.Items) != 1 || page.Items[0].Actor.Name != "Ana" {
		t.Fatalf("actor filter: %+v", page)
	}
	if page := api.activity("?actor_id=" + missingID); len(page.Items) != 0 {
		t.Fatalf("unknown actor: %+v", page)
	}
	if page := api.activity("?channel=mcp"); len(page.Items) != 0 {
		t.Fatalf("channel filter: %+v", page)
	}

	for _, query := range []string{"?channel=carrier-pigeon", "?entity_type=account", "?actor_id=nope", "?entity_id=nope", "?limit=0", "?limit=201", "?cursor=nope"} {
		api.do(http.MethodGet, "/api/v1/activity"+query, nil).expectError(http.StatusUnprocessableEntity)
	}
	api.as("").do(http.MethodGet, "/api/v1/activity", nil).expectError(http.StatusUnauthorized)
}

func TestListActivityPaginatesWithCursor(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Checking", "MXN", 0)
	var ids []uuid.UUID
	for range 5 {
		ids = append(ids, api.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 1}).ID)
	}
	slices.Reverse(ids)

	var seen []uuid.UUID
	query := "?limit=2"
	for pages := 0; ; pages++ {
		if pages > 3 {
			t.Fatal("pagination does not end")
		}
		page := api.activity(query)
		if page.Limit != 2 || len(page.Items) > 2 {
			t.Fatalf("page should hold at most 2 entries: %+v", page)
		}
		for _, e := range page.Items {
			seen = append(seen, e.EntityID)
		}
		if page.NextCursor == nil {
			break
		}
		query = "?limit=2&cursor=" + *page.NextCursor
	}
	if !slices.Equal(seen, ids) {
		t.Fatalf("pages should list every entry once, newest first: got %v, want %v", seen, ids)
	}

	// Cursors combine with filters.
	page := api.activity("?limit=1&entity_type=transaction")
	next := api.activity("?limit=1&entity_type=transaction&cursor=" + *page.NextCursor)
	if len(next.Items) != 1 || next.Items[0].EntityID != ids[1] {
		t.Fatalf("filtered second page: %+v", next)
	}
}

func TestActivityIsIsolatedPerWorkspace(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Checking", "MXN", 0)
	tx := api.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 1})

	other := api.newUser("other@example.com", "Other")
	if page := other.activity(""); len(page.Items) != 0 {
		t.Fatalf("another workspace must not see the log of %s: %+v", tx.ID, page)
	}
	if page := other.activity("?entity_id=" + tx.ID.String()); len(page.Items) != 0 {
		t.Fatalf("another workspace must not find the entry by id: %+v", page)
	}
	// Even with RLS bypassed by a wrong context, rows cannot be written
	// into another workspace.
	_, err := api.pool.Exec(other.ctx,
		`INSERT INTO activity_log (workspace_id, channel, action, entity_type, entity_id) VALUES ($1, 'web', 'x', 'transaction', $2)`,
		api.workspace.ID, tx.ID)
	if err == nil || !strings.Contains(err.Error(), "row-level security") {
		t.Fatalf("writing into another workspace should be rejected, got %v", err)
	}
	if mine := api.activity(""); len(mine.Items) != 1 {
		t.Fatalf("the owner should still see their entry: %+v", mine)
	}
}

func TestActivityLogIsAppendOnly(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Checking", "MXN", 0)
	api.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 1})

	ctx := db.WithWorkspace(context.Background(), api.workspace.ID)
	for _, stmt := range []string{
		"UPDATE activity_log SET action = 'tampered'",
		"DELETE FROM activity_log",
	} {
		if _, err := api.pool.Exec(ctx, stmt); err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("%q should be denied to the API role, got %v", stmt, err)
		}
	}
	if page := api.activity(""); len(page.Items) != 1 || page.Items[0].Action != "transaction.created" {
		t.Fatalf("the entry should be intact: %+v", page)
	}
}

func TestActivityKeepsEntriesOfErasedUsers(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	ana := testutil.NewMember(t, api.pool, api.workspace.ID, "ana@example.com", "Ana")
	account := api.createAccount("Checking", "MXN", 0)
	if _, err := api.svc.CreateTransaction(finance.WithActor(api.ctx, ana), finance.TransactionInput{Type: "expense", AccountID: account.ID, Amount: 2}); err != nil {
		t.Fatal(err)
	}

	if err := api.auth.RemoveUser(context.Background(), "ana@example.com"); err != nil {
		t.Fatal(err)
	}
	page := api.activity("")
	if len(page.Items) != 1 || page.Items[0].Actor != nil || page.Items[0].Action != "transaction.created" {
		t.Fatalf("the entry should stay without its actor: %+v", page)
	}
}
