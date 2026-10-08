package mcpserver

import (
	"slices"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
)

func TestToolChangesAreLoggedAsMCP(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.actor = testutil.NewMember(t, h.pool, h.workspaceID, "ana@example.com", "Ana")
	h.account("Cash", "MXN")

	var created transactionOut
	h.mustCall("add_expense", map[string]any{"amount": 42, "description": "Secret dinner"}, &created)
	h.mustCall("update_transaction", map[string]any{"id": created.ID, "amount": 50}, nil)
	h.mustCall("delete_transaction", map[string]any{"id": created.ID}, nil)
	h.mustFail("delete_transaction", map[string]any{"id": created.ID}, "not found")

	entityType := finance.EntityTransaction
	page, err := h.svc.ListActivity(h.ctx, finance.ActivityFilter{EntityType: &entityType})
	if err != nil {
		t.Fatal(err)
	}
	var actions []string
	for _, e := range page.Items {
		actions = append(actions, e.Action)
		if e.Channel != finance.ChannelMCP || e.Actor == nil || e.Actor.ID != h.actor || e.EntityID.String() != created.ID {
			t.Fatalf("entry should be an MCP change by the caller on the transaction: %+v", e)
		}
	}
	want := []string{finance.ActionTransactionDeleted, finance.ActionTransactionUpdated, finance.ActionTransactionCreated}
	if !slices.Equal(actions, want) {
		t.Fatalf("expected %v, got %v", want, actions)
	}
	if changed := page.Items[1].Details.Changed; !slices.Equal(changed, []string{"amount"}) {
		t.Fatalf("update should name the changed field only: %v", changed)
	}
}
