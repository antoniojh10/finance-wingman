package mcpserver

import (
	"slices"
	"strings"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

func (h *harness) receipt(args map[string]any) (receiptOut, string) {
	h.t.Helper()
	var out receiptOut
	text := h.mustCall("add_receipt", args, &out)
	return out, text
}

func sampleLines() []any {
	return []any{
		map[string]any{"text": "LECHE ENT. HACENDADO 1L", "amount": 0.95, "quantity": 2},
		map[string]any{"text": "PAN BARRA", "amount": 0.80, "discount": 0.10},
	}
}

func TestGroceryToolsAreListed(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	names := h.toolNames()
	for _, name := range []string{"add_receipt", "assign_receipt_lines", "list_grocery_catalog", "list_receipts", "get_receipt", "update_grocery_product"} {
		if !slices.Contains(names, name) {
			t.Errorf("missing tool %s", name)
		}
	}
	if !strings.Contains(h.session.InitializeResult().Instructions, "assign_receipt_lines") {
		t.Error("instructions should explain the receipts flow")
	}
}

func TestAddReceiptCreatesExpenseAndConvertsAmounts(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Checking", "EUR")
	h.category("Groceries", "expense")

	out, text := h.receipt(map[string]any{
		"store": "Mercadona", "date": "2026-10-10", "total": 2.60, "account": "Checking", "source": "photo", "lines": sampleLines(),
	})
	if out.Total != "2.60" || out.Currency != "EUR" || out.Store != "Mercadona" || out.Source != "photo" || out.TransactionID == "" {
		t.Fatalf("unexpected receipt: %+v", out)
	}
	if len(out.Lines) != 2 || out.Lines[0].Amount != "0.95" || out.Lines[0].Quantity != 2 || out.Lines[1].Discount != "0.10" || out.Lines[0].Match != "none" {
		t.Fatalf("unexpected lines: %+v", out.Lines)
	}
	if out.UnassignedCount != 2 || !strings.Contains(out.Hint, "assign_receipt_lines") || !strings.Contains(text, "2 of 2 lines need a product") {
		t.Fatalf("unmatched lines should come with a hint: %s / %+v", text, out)
	}
	// 0.95 + 0.80 != 2.60: warning, not an error.
	if len(out.Warnings) != 1 || out.Warnings[0].Code != "lines_mismatch" {
		t.Fatalf("expected a lines_mismatch warning: %+v", out.Warnings)
	}

	page, err := h.svc.ListTransactions(h.ctx, finance.TransactionFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].Amount != 260 || page.Items[0].Type != finance.TypeExpense || page.Items[0].CategoryName == nil || *page.Items[0].CategoryName != "Groceries" {
		t.Fatalf("expected a 2.60 expense in the Groceries category: %+v", page.Items)
	}
}

func TestAddReceiptLinksExistingExpense(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Checking", "EUR")
	var tx transactionOut
	h.mustCall("add_expense", map[string]any{"amount": 12.40, "account": "Checking"}, &tx)

	out, _ := h.receipt(map[string]any{"store": "Lidl", "date": "2026-10-10", "total": 12.40, "transaction_id": tx.ID})
	if out.TransactionID != tx.ID || out.Currency != "EUR" || len(out.Warnings) != 0 {
		t.Fatalf("unexpected receipt: %+v", out)
	}

	// Linking the same expense twice is refused.
	h.mustFail("add_receipt", map[string]any{"store": "Lidl", "date": "2026-10-10", "total": 12.40, "transaction_id": tx.ID}, "already")

	// A different total produces a warning.
	var other transactionOut
	h.mustCall("add_expense", map[string]any{"amount": 10, "account": "Checking"}, &other)
	out, _ = h.receipt(map[string]any{"store": "Lidl", "date": "2026-10-11", "total": 11, "transaction_id": other.ID})
	if len(out.Warnings) != 1 || out.Warnings[0].Code != "total_mismatch" || !strings.Contains(out.Warnings[0].Message, "11.00") {
		t.Fatalf("expected total_mismatch: %+v", out.Warnings)
	}
}

func TestAddReceiptWithoutTransaction(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	out, _ := h.receipt(map[string]any{"store": "Mercadona", "date": "2026-10-10", "total": 5, "currency": "eur"})
	if out.TransactionID != "" || out.Currency != "EUR" || out.Source != "manual" || len(out.Lines) != 0 {
		t.Fatalf("unexpected receipt: %+v", out)
	}
}

func TestAddReceiptErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Checking", "EUR")
	h.account("Cash", "MXN")
	h.category("Salary", "income")
	h.receipt(map[string]any{"store": "Mercadona", "date": "2026-10-01", "total": 1, "currency": "EUR"})
	var income transactionOut
	h.mustCall("add_income", map[string]any{"amount": 5, "account": "Checking"}, &income)

	base := func(extra map[string]any) map[string]any {
		args := map[string]any{"store": "Mercadona", "date": "2026-10-10", "total": 5, "account": "Checking"}
		for k, v := range extra {
			args[k] = v
		}
		return args
	}
	h.mustFail("add_receipt", base(map[string]any{"store": "Mercadonna"}), "known stores: Mercadona")
	h.mustFail("add_receipt", base(map[string]any{"account": "Nope"}), "available accounts")
	h.mustFail("add_receipt", base(map[string]any{"category": "Salary"}), "expense category")
	h.mustFail("add_receipt", base(map[string]any{"currency": "USD"}), "does not match account")
	h.mustFail("add_receipt", base(map[string]any{"date": ""}), "date is required")
	h.mustFail("add_receipt", base(map[string]any{"total": 0}), "greater than zero")
	h.mustFail("add_receipt", base(map[string]any{"total": 1.234}), "too many decimals")
	h.mustFail("add_receipt", base(map[string]any{"transaction_id": income.ID}), "not both")
	h.mustFail("add_receipt", map[string]any{"store": "Mercadona", "date": "2026-10-10", "total": 5}, "currency is required")
	h.mustFail("add_receipt", map[string]any{"store": "Mercadona", "date": "2026-10-10", "total": 5, "transaction_id": income.ID}, "expense")
	h.mustFail("add_receipt", map[string]any{"store": "Mercadona", "date": "2026-10-10", "total": 5, "transaction_id": "nope"}, "list_transactions")
	h.mustFail("add_receipt", base(map[string]any{"lines": []any{map[string]any{"text": "X", "amount": 1.234}}}), "lines[0].amount")
	h.mustFail("add_receipt", base(map[string]any{"store": "  "}), "store is required")

	// A store the user confirmed is created.
	out, _ := h.receipt(base(map[string]any{"store": "Aldi", "new_store": true}))
	if out.Store != "Aldi" {
		t.Fatalf("new store should be created: %+v", out)
	}
	// Store names match case-insensitively.
	out, _ = h.receipt(base(map[string]any{"store": "mercadona"}))
	if out.Store != "Mercadona" {
		t.Fatalf("store should match case-insensitively: %+v", out)
	}
}

func TestAssignReceiptLinesAndAliasMatch(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	first, _ := h.receipt(map[string]any{"store": "Mercadona", "date": "2026-10-01", "total": 1.75, "currency": "EUR", "lines": sampleLines()})

	var assigned receiptOut
	text := h.mustCall("assign_receipt_lines", map[string]any{
		"receipt_id": first.ID,
		"assignments": []any{
			map[string]any{"line": 1, "product": "Whole milk", "brand": "Hacendado", "pack_size": 1, "pack_label": "1 l", "compare_unit": "l", "group": "Dairy & eggs"},
		},
	}, &assigned)
	if assigned.Lines[0].Product != "Whole milk" || assigned.Lines[0].Brand != "Hacendado" || assigned.UnassignedCount != 1 {
		t.Fatalf("unexpected assignment result: %+v", assigned)
	}
	if !strings.Contains(text, "1 of 2 lines") || !strings.Contains(assigned.Hint, "2 (PAN BARRA)") {
		t.Fatalf("the remaining line should be reported: %s / %s", text, assigned.Hint)
	}

	// The same text in the same store matches automatically, in another it does not.
	second, _ := h.receipt(map[string]any{"store": "mercadona", "date": "2026-10-08", "total": 0.95, "currency": "EUR", "lines": []any{
		map[string]any{"text": "leche ent.  hacendado 1l", "amount": 0.95},
	}})
	if second.Lines[0].Match != "alias" || second.Lines[0].Product != "Whole milk" || second.UnassignedCount != 0 || second.Hint != "" {
		t.Fatalf("expected an alias match: %+v", second)
	}
	third, _ := h.receipt(map[string]any{"store": "Lidl", "date": "2026-10-09", "total": 0.95, "currency": "EUR", "new_store": true, "lines": []any{
		map[string]any{"text": "LECHE ENT. HACENDADO 1L", "amount": 0.95},
	}})
	if third.Lines[0].Match != "none" {
		t.Fatalf("another store must not match: %+v", third)
	}
}

func TestAssignReceiptLinesErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r, _ := h.receipt(map[string]any{"store": "Mercadona", "date": "2026-10-01", "total": 1.75, "currency": "EUR", "lines": sampleLines()})
	assign := func(a map[string]any) map[string]any {
		return map[string]any{"receipt_id": r.ID, "assignments": []any{a}}
	}

	h.mustFail("assign_receipt_lines", map[string]any{"receipt_id": "nope", "assignments": []any{}}, "add_receipt or list_receipts")
	h.mustFail("assign_receipt_lines", map[string]any{"receipt_id": "00000000-0000-0000-0000-000000000000", "assignments": []any{map[string]any{"line": 1, "product": "X"}}}, "not found")
	h.mustFail("assign_receipt_lines", map[string]any{"receipt_id": r.ID, "assignments": []any{}}, "at least one")
	h.mustFail("assign_receipt_lines", assign(map[string]any{"line": 5, "product": "Milk"}), "lines 1 to 2")
	h.mustFail("assign_receipt_lines", assign(map[string]any{"line": 1, "product": " "}), "product is required")
	h.mustFail("assign_receipt_lines", assign(map[string]any{"line": 1, "product": "Milk", "group": "Dairy & eggs"}), "compare_unit")
	h.mustFail("assign_receipt_lines", assign(map[string]any{"line": 1, "product": "Milk", "compare_unit": "l", "group": "Spaceships"}), "valid groups")

	// Failures change nothing.
	var got receiptOut
	h.mustCall("get_receipt", map[string]any{"id": r.ID}, &got)
	if got.UnassignedCount != 2 {
		t.Fatalf("a failed assignment must not change the receipt: %+v", got)
	}
}

func TestListGroceryCatalog(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	var empty catalogOut
	h.mustCall("list_grocery_catalog", map[string]any{}, &empty)
	if len(empty.Products) != 0 || len(empty.Groups) == 0 {
		t.Fatalf("a new workspace has groups but no products: %+v", empty)
	}

	r, _ := h.receipt(map[string]any{"store": "Mercadona", "date": "2026-10-01", "total": 1.75, "currency": "EUR", "lines": sampleLines()})
	h.mustCall("assign_receipt_lines", map[string]any{"receipt_id": r.ID, "assignments": []any{
		map[string]any{"line": 1, "product": "Whole milk", "brand": "Hacendado", "pack_size": 1, "compare_unit": "l", "group": "Dairy & eggs"},
		map[string]any{"line": 2, "product": "Bread", "compare_unit": "unit", "group": "Bakery"},
	}}, nil)

	var all catalogOut
	h.mustCall("list_grocery_catalog", map[string]any{}, &all)
	if len(all.Stores) != 1 || all.Stores[0] != "Mercadona" || len(all.Products) != 2 {
		t.Fatalf("unexpected catalog: %+v", all)
	}
	var milk *catalogProductOut
	for i := range all.Products {
		if all.Products[i].Name == "Whole milk" {
			milk = &all.Products[i]
		}
	}
	if milk == nil || milk.Group != "Dairy & eggs" || milk.CompareUnit != "l" || len(milk.Variants) != 1 || milk.Variants[0].Brand != "Hacendado" {
		t.Fatalf("unexpected product: %+v", milk)
	}

	var byBrand catalogOut
	h.mustCall("list_grocery_catalog", map[string]any{"query": "hacen"}, &byBrand)
	if len(byBrand.Products) != 1 || byBrand.Products[0].Name != "Whole milk" {
		t.Fatalf("query should match brands: %+v", byBrand.Products)
	}
	var none catalogOut
	h.mustCall("list_grocery_catalog", map[string]any{"query": "caviar"}, &none)
	if len(none.Products) != 0 {
		t.Fatalf("query should filter: %+v", none.Products)
	}
}

func TestListAndGetReceipts(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	a, _ := h.receipt(map[string]any{"store": "Mercadona", "date": "2026-10-01", "total": 1.75, "currency": "EUR", "lines": sampleLines()})
	h.receipt(map[string]any{"store": "Lidl", "date": "2026-09-15", "total": 20, "currency": "EUR", "new_store": true})

	var all receiptsOut
	h.mustCall("list_receipts", map[string]any{}, &all)
	if len(all.Receipts) != 2 {
		t.Fatalf("expected 2 receipts: %+v", all)
	}
	var oct receiptsOut
	h.mustCall("list_receipts", map[string]any{"month": "2026-10"}, &oct)
	if len(oct.Receipts) != 1 || oct.Receipts[0].ID != a.ID || oct.Receipts[0].Total != "1.75" || oct.Receipts[0].UnassignedCount != 2 || oct.Receipts[0].LineCount != 2 {
		t.Fatalf("unexpected month filter result: %+v", oct)
	}
	var lidl receiptsOut
	h.mustCall("list_receipts", map[string]any{"store": "lidl"}, &lidl)
	if len(lidl.Receipts) != 1 || lidl.Receipts[0].Store != "Lidl" || lidl.Receipts[0].Total != "20.00" {
		t.Fatalf("unexpected store filter result: %+v", lidl)
	}
	var pending receiptsOut
	h.mustCall("list_receipts", map[string]any{"unassigned_only": true}, &pending)
	if len(pending.Receipts) != 1 {
		t.Fatalf("only the receipt with lines still unassigned should be listed: %+v", pending)
	}

	var got receiptOut
	h.mustCall("get_receipt", map[string]any{"id": a.ID}, &got)
	if got.ID != a.ID || len(got.Lines) != 2 || got.Lines[1].Text != "PAN BARRA" {
		t.Fatalf("unexpected receipt: %+v", got)
	}

	h.mustFail("list_receipts", map[string]any{"month": "October"}, "YYYY-MM")
	h.mustFail("list_receipts", map[string]any{"store": "Carrefour"}, "known stores: ")
	h.mustFail("get_receipt", map[string]any{"id": "nope"}, "list_receipts")
	h.mustFail("get_receipt", map[string]any{"id": "00000000-0000-0000-0000-000000000000"}, "not found")
}

func TestReadOnlyConnectionRefusesReceiptWrites(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r, _ := h.receipt(map[string]any{"store": "Mercadona", "date": "2026-10-01", "total": 1.75, "currency": "EUR", "lines": sampleLines()})

	h.readOnly = true
	h.mustFail("add_receipt", map[string]any{"store": "Mercadona", "date": "2026-10-02", "total": 3, "currency": "EUR"}, "read-only")
	h.mustFail("assign_receipt_lines", map[string]any{"receipt_id": r.ID, "assignments": []any{map[string]any{"line": 1, "product": "Milk", "compare_unit": "l"}}}, "read-only")

	var all receiptsOut
	h.mustCall("list_receipts", map[string]any{}, &all)
	if len(all.Receipts) != 1 || all.Receipts[0].UnassignedCount != 2 {
		t.Fatalf("read-only calls must not change data: %+v", all)
	}
	h.mustCall("get_receipt", map[string]any{"id": r.ID}, nil)
	h.mustCall("list_grocery_catalog", map[string]any{}, nil)
}
