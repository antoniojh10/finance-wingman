package mcpserver

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
)

// toolMutation is one MCP tool that changes finance data: prepare sets up
// what the call needs and returns the call's arguments together with the
// actions it must log.
type toolMutation struct {
	tool    string
	prepare func(h *harness) (args map[string]any, actions []string)
}

// readOnlyPrefixes name the tools that only read. Any other tool must be
// listed in toolMutations, so a new write tool cannot ship without logging.
var readOnlyPrefixes = []string{"list_", "get_", "suggest_"}

func toolMutations() []toolMutation {
	created := []string{finance.ActionTransactionCreated}
	updated := []string{finance.ActionTransactionUpdated}
	return []toolMutation{
		{"add_expense", func(h *harness) (map[string]any, []string) {
			h.account("Cash", "MXN")
			return map[string]any{"amount": 5}, created
		}},
		{"add_income", func(h *harness) (map[string]any, []string) {
			h.account("Cash", "MXN")
			return map[string]any{"amount": 5}, created
		}},
		{"add_transfer", func(h *harness) (map[string]any, []string) {
			h.account("Cash", "MXN")
			h.account("Card", "MXN")
			return map[string]any{"amount": 5, "from_account": "Cash", "to_account": "Card"}, created
		}},
		{"add_transactions", func(h *harness) (map[string]any, []string) {
			h.account("Cash", "MXN")
			return map[string]any{"items": []map[string]any{{"type": "expense", "amount": 5}, {"type": "income", "amount": 6}}},
				[]string{finance.ActionTransactionCreated, finance.ActionTransactionCreated}
		}},
		{"update_transaction", func(h *harness) (map[string]any, []string) {
			h.account("Cash", "MXN")
			tx := h.addExpense(map[string]any{"amount": 5})
			return map[string]any{"id": tx.ID, "amount": 6}, updated
		}},
		{"update_transactions", func(h *harness) (map[string]any, []string) {
			h.account("Cash", "MXN")
			tx := h.addExpense(map[string]any{"amount": 5})
			return map[string]any{"items": []map[string]any{{"id": tx.ID, "amount": 6}}}, updated
		}},
		{"delete_transaction", func(h *harness) (map[string]any, []string) {
			h.account("Cash", "MXN")
			tx := h.addExpense(map[string]any{"amount": 5})
			return map[string]any{"id": tx.ID}, []string{finance.ActionTransactionDeleted}
		}},
		{"create_account", func(h *harness) (map[string]any, []string) {
			return map[string]any{"name": "Cash", "type": "cash", "currency": "MXN"}, []string{finance.ActionAccountCreated}
		}},
		{"create_accounts", func(h *harness) (map[string]any, []string) {
			return map[string]any{"items": []map[string]any{
					{"name": "Cash", "type": "cash", "currency": "MXN"}, {"name": "Card", "type": "checking", "currency": "EUR"},
				}},
				[]string{finance.ActionAccountCreated, finance.ActionAccountCreated}
		}},
		{"update_account", func(h *harness) (map[string]any, []string) {
			h.account("Cash", "MXN")
			return map[string]any{"account": "Cash", "name": "Wallet"}, []string{finance.ActionAccountUpdated}
		}},
		{"create_category", func(h *harness) (map[string]any, []string) {
			return map[string]any{"name": "Food", "kind": "expense"}, []string{finance.ActionCategoryCreated}
		}},
		{"create_categories", func(h *harness) (map[string]any, []string) {
			return map[string]any{"items": []map[string]any{{"name": "Food", "kind": "expense"}, {"name": "Pay", "kind": "income"}}},
				[]string{finance.ActionCategoryCreated, finance.ActionCategoryCreated}
		}},
		{"update_category", func(h *harness) (map[string]any, []string) {
			h.category("Food", "expense")
			return map[string]any{"category": "Food", "archived": true}, []string{finance.ActionCategoryArchived}
		}},
		{"set_budgets", func(h *harness) (map[string]any, []string) {
			h.category("Food", "expense")
			return map[string]any{"items": []map[string]any{{"category": "Food", "currency": "MXN", "amount": 100}}},
				[]string{finance.ActionBudgetSet}
		}},
		{"create_recurring", func(h *harness) (map[string]any, []string) {
			h.account("Card", "MXN")
			return map[string]any{"name": "Gym", "type": "expense", "amount": 5, "frequency": "week", "account": "Card"},
				[]string{finance.ActionRecurringCreated}
		}},
		{"update_recurring", func(h *harness) (map[string]any, []string) {
			h.account("Card", "MXN")
			h.weekly("Gym", 5)
			return map[string]any{"recurring": "Gym", "status": "cancelled"}, []string{finance.ActionRecurringUpdated}
		}},
		{"mark_recurring_paid", func(h *harness) (map[string]any, []string) {
			h.account("Card", "MXN")
			h.weekly("Gym", 5)
			return map[string]any{"recurring": "Gym"}, created
		}},
		{"link_transaction_to_recurring", func(h *harness) (map[string]any, []string) {
			h.account("Card", "MXN")
			h.weekly("Gym", 5)
			tx := h.addExpense(map[string]any{"amount": 5, "account": "Card", "date": h.svc.Today().AddDate(0, 0, -9).Format(time.DateOnly)})
			return map[string]any{"transaction": tx.ID, "recurring": "Gym"}, updated
		}},
		{"accept_recurring_suggestion", func(h *harness) (map[string]any, []string) {
			card := h.account("Card", "MXN")
			h.seedMonthly(card, "Netflix", nil, 1000, 1000, 1000)
			return map[string]any{"key": h.suggestions()[0].Key},
				[]string{finance.ActionRecurringCreated, finance.ActionTransactionUpdated, finance.ActionTransactionUpdated, finance.ActionTransactionUpdated}
		}},
		{"dismiss_recurring_suggestion", func(h *harness) (map[string]any, []string) {
			card := h.account("Card", "MXN")
			h.seedMonthly(card, "Netflix", nil, 1000, 1000, 1000)
			return map[string]any{"key": h.suggestions()[0].Key}, []string{finance.ActionSuggestionDismissed}
		}},
	}
}

func TestEveryWriteToolIsCovered(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	covered := map[string]bool{}
	for _, m := range toolMutations() {
		covered[m.tool] = true
	}
	listed := h.toolNames()
	for _, name := range listed {
		readOnly := slices.ContainsFunc(readOnlyPrefixes, func(p string) bool { return strings.HasPrefix(name, p) })
		if !readOnly && !covered[name] {
			t.Errorf("tool %s is not read-only but has no entry in toolMutations: record its changes in the activity log and add it", name)
		}
	}
	for tool := range covered {
		if !slices.Contains(listed, tool) {
			t.Errorf("toolMutations lists %q, which is not an MCP tool", tool)
		}
	}
}

func TestWriteToolsRecordActivityAsMCP(t *testing.T) {
	t.Parallel()
	for _, m := range toolMutations() {
		t.Run(m.tool, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.actor = testutil.NewMember(t, h.pool, h.workspaceID, "ana@example.com", "Ana")
			args, want := m.prepare(h)
			before, err := h.svc.ListActivity(h.ctx, finance.ActivityFilter{Limit: finance.MaxPageSize})
			if err != nil {
				t.Fatal(err)
			}
			h.mustCall(m.tool, args, nil)
			after, err := h.svc.ListActivity(h.ctx, finance.ActivityFilter{Limit: finance.MaxPageSize})
			if err != nil {
				t.Fatal(err)
			}
			added := len(after.Items) - len(before.Items)
			var got []string
			for _, e := range after.Items[:max(added, 0)] {
				got = append(got, e.Action)
				if e.Channel != finance.ChannelMCP || e.Actor == nil || e.Actor.ID != h.actor {
					t.Fatalf("entry should be an MCP change by the caller: %+v", e)
				}
			}
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Fatalf("expected actions %v, got %v", want, got)
			}
		})
	}
}
