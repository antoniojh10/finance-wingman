package mcpserver

import (
	"context"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

func TestTransactionsExposeRecurringID(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	account := h.account("Card", "MXN")
	ctx := context.Background()

	item, err := h.svc.CreateRecurringItem(ctx, finance.CreateRecurringItemInput{
		Name: "Netflix", Type: "expense", AccountID: account.ID, Amount: 18900, IntervalUnit: "month",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.RegisterRecurringPayment(ctx, item.ID, finance.RecurringPaymentInput{}); err != nil {
		t.Fatal(err)
	}
	h.mustCall("add_expense", map[string]any{"amount": 10, "account": "Card", "description": "Coffee"}, nil)

	var out transactionsOut
	h.mustCall("list_transactions", nil, &out)
	linked := 0
	for _, tx := range out.Transactions {
		switch tx.Description {
		case "Netflix":
			if tx.RecurringID != item.ID.String() {
				t.Fatalf("expected recurring_id %s, got %q", item.ID, tx.RecurringID)
			}
			linked++
		case "Coffee":
			if tx.RecurringID != "" {
				t.Fatalf("unlinked transaction should have no recurring_id, got %q", tx.RecurringID)
			}
		}
	}
	if linked != 1 {
		t.Fatalf("expected one linked transaction: %+v", out)
	}
}
