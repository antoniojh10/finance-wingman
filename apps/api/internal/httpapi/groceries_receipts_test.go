package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/groceries"
)

func (a *testAPI) createReceipt(body map[string]any) groceries.Receipt {
	a.t.Helper()
	var r groceries.Receipt
	a.do(http.MethodPost, gp+"/receipts", body).expect(http.StatusCreated).decode(&r)
	return r
}

func (a *testAPI) getReceipt(id uuid.UUID) groceries.Receipt {
	a.t.Helper()
	var r groceries.Receipt
	a.do(http.MethodGet, gp+"/receipts/"+id.String(), nil).expect(http.StatusOK).decode(&r)
	return r
}

func (a *testAPI) assign(receiptID uuid.UUID, assignments ...map[string]any) response {
	a.t.Helper()
	return a.do(http.MethodPost, gp+"/receipts/"+receiptID.String()+"/assign", map[string]any{"assignments": assignments})
}

func (a *testAPI) createExpenseTx(accountID uuid.UUID, amount int64) finance.Transaction {
	a.t.Helper()
	return a.createTransaction(map[string]any{"type": "expense", "account_id": accountID, "amount": amount, "occurred_on": "2026-10-04"})
}

func receiptBody(extra map[string]any) map[string]any {
	body := map[string]any{
		"store_name": "Mercadona", "purchased_on": "2026-10-04", "currency": "EUR", "total": 1000, "source": "photo",
		"lines": []map[string]any{
			{"raw_text": "Leche  ent. HACENDADO 1l", "amount": 600},
			{"raw_text": "PAN BARRA", "quantity": 2, "amount": 400, "discount": 50},
		},
	}
	for k, v := range extra {
		if v == nil {
			delete(body, k)
			continue
		}
		body[k] = v
	}
	return body
}

func lineByText(t *testing.T, r groceries.Receipt, text string) groceries.ReceiptLine {
	t.Helper()
	for _, l := range r.Lines {
		if l.RawText == text {
			return l
		}
	}
	t.Fatalf("no line %q in %+v", text, r.Lines)
	return groceries.ReceiptLine{}
}

func TestGroceryReceiptsRequireAuthentication(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t).as("")
	id := uuid.NewString()
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, gp + "/receipts"}, {http.MethodPost, gp + "/receipts"},
		{http.MethodGet, gp + "/receipts/" + id}, {http.MethodPatch, gp + "/receipts/" + id},
		{http.MethodDelete, gp + "/receipts/" + id}, {http.MethodPost, gp + "/receipts/" + id + "/assign"},
	} {
		api.do(c.method, c.path, nil).expect(http.StatusUnauthorized)
	}
}

func TestGroceryReceiptCreateAndAliasMatching(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)

	first := api.createReceipt(receiptBody(nil))
	if first.StoreName != "Mercadona" || first.Total != 1000 || first.Currency != "EUR" || first.Source != "photo" ||
		first.PurchasedOn != "2026-10-04" || first.TransactionID != nil {
		t.Fatalf("unexpected receipt %+v", first)
	}
	if first.LineCount != 2 || first.UnassignedCount != 2 || first.LinesTotal != 1000 || len(first.Warnings) != 0 {
		t.Fatalf("unexpected counts %+v", first)
	}
	milk := lineByText(t, first, "Leche  ent. HACENDADO 1l")
	bread := lineByText(t, first, "PAN BARRA")
	if milk.Match != groceries.MatchNone || milk.Quantity != 1 || bread.Quantity != 2 || bread.Discount != 50 || bread.Position != 2 {
		t.Fatalf("unexpected lines %+v %+v", milk, bread)
	}
	// The store was created from its name.
	var stores struct{ Items []groceries.Store }
	api.do(http.MethodGet, gp+"/stores", nil).expect(http.StatusOK).decode(&stores)
	if len(stores.Items) != 1 || stores.Items[0].ID != first.StoreID {
		t.Fatalf("expected the store to be created, got %+v", stores.Items)
	}

	// Assign creates product, variant and alias.
	assigned := func() groceries.Receipt {
		var r groceries.Receipt
		api.assign(first.ID,
			map[string]any{"line_id": milk.ID, "product": "Whole milk", "brand": "Hacendado", "pack_size": 1, "pack_label": "1 l", "compare_unit": "l", "group": "dairy & eggs"},
			map[string]any{"line_id": bread.ID, "product": "whole milk", "brand": "Hacendado", "pack_size": 1},
		).expect(http.StatusOK).decode(&r)
		return r
	}()
	if assigned.UnassignedCount != 0 {
		t.Fatalf("expected all lines assigned, got %+v", assigned)
	}
	got := lineByText(t, assigned, "Leche  ent. HACENDADO 1l")
	if got.ProductName == nil || *got.ProductName != "Whole milk" || got.Brand == nil || *got.Brand != "Hacendado" || got.PackLabel == nil || *got.PackLabel != "1 l" {
		t.Fatalf("unexpected assigned line %+v", got)
	}
	var products struct{ Items []groceries.Product }
	api.do(http.MethodGet, gp+"/products", nil).expect(http.StatusOK).decode(&products)
	if len(products.Items) != 1 || products.Items[0].VariantCount != 1 || products.Items[0].GroupName == nil || *products.Items[0].GroupName != "Dairy & eggs" {
		t.Fatalf("expected a single product with one variant in the Dairy group, got %+v", products.Items)
	}

	// The next receipt from the same store matches through the alias (even
	// with different spacing and case); a given variant is kept; unknown text stays unmatched.
	second := api.createReceipt(receiptBody(map[string]any{
		"store_id": first.StoreID, "store_name": nil, "total": 1500,
		"lines": []map[string]any{
			{"raw_text": "leche ENT. hacendado   1L", "amount": 600},
			{"raw_text": "PAPEL", "amount": 500, "variant_id": got.VariantID},
			{"raw_text": "UNKNOWN THING", "amount": 400},
		},
	}))
	if m := lineByText(t, second, "leche ENT. hacendado   1L"); m.Match != groceries.MatchAlias || m.VariantID == nil || *m.VariantID != *got.VariantID {
		t.Fatalf("expected alias match, got %+v", m)
	}
	if m := lineByText(t, second, "PAPEL"); m.Match != groceries.MatchGiven {
		t.Fatalf("expected given match, got %+v", m)
	}
	if m := lineByText(t, second, "UNKNOWN THING"); m.Match != groceries.MatchNone || m.VariantID != nil {
		t.Fatalf("expected no match, got %+v", m)
	}
	if second.UnassignedCount != 1 {
		t.Fatalf("expected one unassigned line, got %+v", second)
	}

	// The same text in another store does not match.
	other := api.createReceipt(receiptBody(map[string]any{
		"store_name": "Lidl", "lines": []map[string]any{{"raw_text": "LECHE ENT. HACENDADO 1L", "amount": 1000}},
	}))
	if m := other.Lines[0]; m.Match != groceries.MatchNone || m.VariantID != nil {
		t.Fatalf("alias must be per store, got %+v", m)
	}

	// Unassign keeps the alias.
	var unassigned groceries.Receipt
	api.assign(first.ID, map[string]any{"line_id": milk.ID}).expect(http.StatusOK).decode(&unassigned)
	if l := lineByText(t, unassigned, "Leche  ent. HACENDADO 1l"); l.VariantID != nil || unassigned.UnassignedCount != 1 {
		t.Fatalf("expected the line to be unassigned, got %+v", unassigned)
	}
	third := api.createReceipt(receiptBody(map[string]any{
		"store_id": first.StoreID, "store_name": nil, "lines": []map[string]any{{"raw_text": "LECHE ENT. HACENDADO 1L", "amount": 1000}},
	}))
	if third.Lines[0].Match != groceries.MatchAlias {
		t.Fatalf("expected the alias to be kept, got %+v", third.Lines[0])
	}

	// Assigning to an existing variant by id.
	api.assign(other.ID, map[string]any{"line_id": other.Lines[0].ID, "variant_id": got.VariantID}).expect(http.StatusOK)
}

func TestGroceryReceiptCreateValidation(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	store := api.createGroceryStore("Lidl")
	for name, body := range map[string]map[string]any{
		"neither store":         receiptBody(map[string]any{"store_name": nil}),
		"both stores":           receiptBody(map[string]any{"store_id": store.ID}),
		"blank store name":      receiptBody(map[string]any{"store_name": "  "}),
		"unknown store id":      receiptBody(map[string]any{"store_name": nil, "store_id": uuid.NewString()}),
		"bad store id":          receiptBody(map[string]any{"store_name": nil, "store_id": "nope"}),
		"unknown store no flag": receiptBody(map[string]any{"store_name": "Aldi", "create_store_if_missing": false}),
		"bad date":              receiptBody(map[string]any{"purchased_on": "04/10/2026"}),
		"zero total":            receiptBody(map[string]any{"total": 0}),
		"bad source":            receiptBody(map[string]any{"source": "fax"}),
		"unknown currency":      receiptBody(map[string]any{"currency": "ZZZ"}),
		"short currency":        receiptBody(map[string]any{"currency": "EU"}),
		"blank raw text":        receiptBody(map[string]any{"lines": []map[string]any{{"raw_text": " ", "amount": 1}}}),
		"negative amount":       receiptBody(map[string]any{"lines": []map[string]any{{"raw_text": "X", "amount": -1}}}),
		"zero quantity":         receiptBody(map[string]any{"lines": []map[string]any{{"raw_text": "X", "quantity": 0, "amount": 1}}}),
		"negative discount":     receiptBody(map[string]any{"lines": []map[string]any{{"raw_text": "X", "amount": 1, "discount": -1}}}),
		"unknown variant":       receiptBody(map[string]any{"lines": []map[string]any{{"raw_text": "X", "amount": 1, "variant_id": uuid.NewString()}}}),
		"both transaction args": receiptBody(map[string]any{"transaction_id": uuid.NewString(), "create_expense": map[string]any{"account_id": uuid.NewString()}}),
		"unknown transaction":   receiptBody(map[string]any{"transaction_id": uuid.NewString()}),
		"unknown account":       receiptBody(map[string]any{"create_expense": map[string]any{"account_id": uuid.NewString()}}),
	} {
		api.do(http.MethodPost, gp+"/receipts", body).expect(http.StatusUnprocessableEntity)
		_ = name
	}

	// Nothing was left behind by the failed requests (the store existed already).
	var list struct{ Items []groceries.ReceiptSummary }
	api.do(http.MethodGet, gp+"/receipts", nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 0 {
		t.Fatalf("expected no receipts, got %+v", list.Items)
	}
	var stores struct{ Items []groceries.Store }
	api.do(http.MethodGet, gp+"/stores", nil).expect(http.StatusOK).decode(&stores)
	if len(stores.Items) != 1 {
		t.Fatalf("failed requests must not create stores, got %+v", stores.Items)
	}

	// A receipt without lines is fine.
	api.createReceipt(receiptBody(map[string]any{"store_name": "lidl", "lines": nil}))
	api.do(http.MethodGet, gp+"/stores", nil).expect(http.StatusOK).decode(&stores)
	if len(stores.Items) != 1 {
		t.Fatalf("store name matching must be case-insensitive, got %+v", stores.Items)
	}
}

func TestGroceryReceiptTransactionLinking(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	eur := api.createAccount("Checking", "EUR", 100000)
	savings := api.createAccount("Savings", "EUR", 0)
	usd := api.createAccount("Dollars", "USD", 0)

	expense := api.createExpenseTx(eur.ID, 1000)
	linked := api.createReceipt(receiptBody(map[string]any{"transaction_id": expense.ID}))
	if linked.TransactionID == nil || *linked.TransactionID != expense.ID || len(linked.Warnings) != 0 {
		t.Fatalf("unexpected linked receipt %+v", linked)
	}

	// Already linked, wrong type, wrong currency.
	api.do(http.MethodPost, gp+"/receipts", receiptBody(map[string]any{"transaction_id": expense.ID})).expect(http.StatusConflict)
	income := api.createTransaction(map[string]any{"type": "income", "account_id": eur.ID, "amount": 1000})
	api.do(http.MethodPost, gp+"/receipts", receiptBody(map[string]any{"transaction_id": income.ID})).expect(http.StatusUnprocessableEntity)
	transfer := api.createTransaction(map[string]any{"type": "transfer", "account_id": eur.ID, "destination_account_id": savings.ID, "amount": 1000})
	api.do(http.MethodPost, gp+"/receipts", receiptBody(map[string]any{"transaction_id": transfer.ID})).expect(http.StatusUnprocessableEntity)
	usdExpense := api.createExpenseTx(usd.ID, 1000)
	api.do(http.MethodPost, gp+"/receipts", receiptBody(map[string]any{"transaction_id": usdExpense.ID})).expect(http.StatusUnprocessableEntity)

	// Mismatches are allowed and reported.
	other := api.createExpenseTx(eur.ID, 1200)
	warned := api.createReceipt(receiptBody(map[string]any{
		"transaction_id": other.ID, "total": 1100, // lines add up to 1000
	}))
	codes := map[string]groceries.Warning{}
	for _, w := range warned.Warnings {
		codes[w.Code] = w
	}
	tm, lm := codes[groceries.WarningTotalMismatch], codes[groceries.WarningLinesMismatch]
	if len(codes) != 2 || tm.ReceiptTotal == nil || *tm.ReceiptTotal != 1100 || tm.TransactionAmount == nil || *tm.TransactionAmount != 1200 ||
		lm.LinesTotal == nil || *lm.LinesTotal != 1000 || lm.ReceiptTotal == nil || *lm.ReceiptTotal != 1100 {
		t.Fatalf("unexpected warnings %+v", warned.Warnings)
	}
	if got := api.getReceipt(warned.ID); len(got.Warnings) != 2 {
		t.Fatalf("GET must report the warnings too, got %+v", got.Warnings)
	}

	// Deleting the receipt keeps the transaction.
	api.do(http.MethodDelete, gp+"/receipts/"+linked.ID.String(), nil).expect(http.StatusNoContent)
	api.do(http.MethodDelete, gp+"/receipts/"+linked.ID.String(), nil).expect(http.StatusNotFound)
	api.do(http.MethodGet, gp+"/receipts/"+linked.ID.String(), nil).expect(http.StatusNotFound)
	api.do(http.MethodGet, "/api/v1/transactions/"+expense.ID.String(), nil).expect(http.StatusOK)
	// ...and frees it for another receipt.
	api.createReceipt(receiptBody(map[string]any{"transaction_id": expense.ID}))

	// Deleting the transaction keeps the receipt, unlinked.
	api.do(http.MethodDelete, "/api/v1/transactions/"+other.ID.String(), nil).expect(http.StatusNoContent)
	if got := api.getReceipt(warned.ID); got.TransactionID != nil || len(got.Warnings) != 1 || got.Warnings[0].Code != groceries.WarningLinesMismatch {
		t.Fatalf("expected an unlinked receipt, got %+v", got)
	}
}

func TestGroceryReceiptCreateExpense(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	eur := api.createAccount("Checking", "EUR", 100000)
	usd := api.createAccount("Dollars", "USD", 0)

	// Without a Groceries category the expense is uncategorized.
	plain := api.createReceipt(receiptBody(map[string]any{"create_expense": map[string]any{"account_id": eur.ID}}))
	if plain.TransactionID == nil || len(plain.Warnings) != 0 {
		t.Fatalf("unexpected receipt %+v", plain)
	}
	var tx finance.Transaction
	api.do(http.MethodGet, "/api/v1/transactions/"+plain.TransactionID.String(), nil).expect(http.StatusOK).decode(&tx)
	if tx.Type != "expense" || tx.Amount != 1000 || tx.OccurredOn != "2026-10-04" || tx.Description != "Mercadona" || tx.CategoryID != nil || tx.AccountID != eur.ID {
		t.Fatalf("unexpected transaction %+v", tx)
	}

	// The "Groceries" expense category (case-insensitive) is picked; an income category of that name is not.
	api.createCategory("Groceries", "income")
	food := api.createCategory("GROCERIES", "expense")
	picked := api.createReceipt(receiptBody(map[string]any{"create_expense": map[string]any{"account_id": eur.ID}}))
	api.do(http.MethodGet, "/api/v1/transactions/"+picked.TransactionID.String(), nil).expect(http.StatusOK).decode(&tx)
	if tx.CategoryID == nil || *tx.CategoryID != food.ID {
		t.Fatalf("expected the Groceries category, got %+v", tx)
	}

	// An explicit category wins.
	other := api.createCategory("Treats", "expense")
	explicit := api.createReceipt(receiptBody(map[string]any{"create_expense": map[string]any{"account_id": eur.ID, "category_id": other.ID}}))
	api.do(http.MethodGet, "/api/v1/transactions/"+explicit.TransactionID.String(), nil).expect(http.StatusOK).decode(&tx)
	if tx.CategoryID == nil || *tx.CategoryID != other.ID {
		t.Fatalf("expected the explicit category, got %+v", tx)
	}

	// Errors create nothing: currency mismatch, income category, bad account id.
	before := countTransactions(t, api)
	api.do(http.MethodPost, gp+"/receipts", receiptBody(map[string]any{"create_expense": map[string]any{"account_id": usd.ID}})).expect(http.StatusUnprocessableEntity)
	income := api.createCategory("Salary", "income")
	api.do(http.MethodPost, gp+"/receipts", receiptBody(map[string]any{"create_expense": map[string]any{"account_id": eur.ID, "category_id": income.ID}})).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPost, gp+"/receipts", receiptBody(map[string]any{"create_expense": map[string]any{"account_id": "nope"}})).expect(http.StatusUnprocessableEntity)
	if after := countTransactions(t, api); after != before {
		t.Fatalf("failed requests created transactions: %d -> %d", before, after)
	}
}

func countTransactions(t *testing.T, api *testAPI) int64 {
	t.Helper()
	var page finance.TransactionPage
	api.do(http.MethodGet, "/api/v1/transactions?limit=1", nil).expect(http.StatusOK).decode(&page)
	return page.Total
}

func TestGroceryReceiptListAndUpdate(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	eur := api.createAccount("Checking", "EUR", 100000)
	oct := api.createReceipt(receiptBody(nil))
	sep := api.createReceipt(receiptBody(map[string]any{"store_name": "Lidl", "purchased_on": "2026-09-30"}))
	api.assign(sep.ID,
		map[string]any{"line_id": sep.Lines[0].ID, "product": "Milk", "compare_unit": "l"},
		map[string]any{"line_id": sep.Lines[1].ID, "product": "Bread", "compare_unit": "unit", "group": "Bakery"},
	).expect(http.StatusOK)

	list := func(query string) []groceries.ReceiptSummary {
		var out struct{ Items []groceries.ReceiptSummary }
		api.do(http.MethodGet, gp+"/receipts"+query, nil).expect(http.StatusOK).decode(&out)
		return out.Items
	}
	if items := list(""); len(items) != 2 || items[0].ID != oct.ID || items[0].UnassignedCount != 2 || items[0].LineCount != 2 || items[1].UnassignedCount != 0 {
		t.Fatalf("unexpected list %+v", items)
	}
	if items := list("?month=2026-09"); len(items) != 1 || items[0].ID != sep.ID {
		t.Fatalf("unexpected month filter %+v", items)
	}
	if items := list("?month=2026-10&store_id=" + oct.StoreID.String()); len(items) != 1 || items[0].ID != oct.ID {
		t.Fatalf("unexpected store filter %+v", items)
	}
	if items := list("?unassigned_only=true"); len(items) != 1 || items[0].ID != oct.ID {
		t.Fatalf("unexpected unassigned filter %+v", items)
	}
	if items := list("?month=2025-01"); len(items) != 0 {
		t.Fatalf("expected nothing, got %+v", items)
	}
	api.do(http.MethodGet, gp+"/receipts?month=2026-13", nil).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodGet, gp+"/receipts?store_id=nope", nil).expect(http.StatusUnprocessableEntity)

	// Update store, date, total and link / unlink.
	expense := api.createExpenseTx(eur.ID, 1000)
	var updated groceries.Receipt
	api.do(http.MethodPatch, gp+"/receipts/"+oct.ID.String(), map[string]any{
		"store_id": sep.StoreID, "purchased_on": "2026-10-05", "total": 1000, "transaction_id": expense.ID,
	}).expect(http.StatusOK).decode(&updated)
	if updated.StoreID != sep.StoreID || updated.StoreName != "Lidl" || updated.PurchasedOn != "2026-10-05" ||
		updated.TransactionID == nil || *updated.TransactionID != expense.ID {
		t.Fatalf("unexpected update %+v", updated)
	}
	// Relinking the same transaction to itself is fine; to another receipt it conflicts.
	api.do(http.MethodPatch, gp+"/receipts/"+oct.ID.String(), map[string]any{"transaction_id": expense.ID}).expect(http.StatusOK)
	api.do(http.MethodPatch, gp+"/receipts/"+sep.ID.String(), map[string]any{"transaction_id": expense.ID}).expect(http.StatusConflict)
	api.do(http.MethodPatch, gp+"/receipts/"+oct.ID.String(), map[string]any{"total": 1300}).expect(http.StatusOK).decode(&updated)
	if len(updated.Warnings) != 2 {
		t.Fatalf("expected both warnings after changing the total, got %+v", updated.Warnings)
	}
	api.do(http.MethodPatch, gp+"/receipts/"+oct.ID.String(), map[string]any{"transaction_id": ""}).expect(http.StatusOK).decode(&updated)
	if updated.TransactionID != nil {
		t.Fatalf("expected the receipt to be unlinked, got %+v", updated)
	}

	for name, body := range map[string]map[string]any{
		"zero total":    {"total": 0},
		"bad date":      {"purchased_on": "x"},
		"unknown store": {"store_id": uuid.NewString()},
		"empty store":   {"store_id": ""},
		"unknown tx":    {"transaction_id": uuid.NewString()},
	} {
		api.do(http.MethodPatch, gp+"/receipts/"+oct.ID.String(), body).expect(http.StatusUnprocessableEntity)
		_ = name
	}
	api.do(http.MethodPatch, gp+"/receipts/"+uuid.NewString(), map[string]any{"total": 5}).expect(http.StatusNotFound)
	api.do(http.MethodGet, gp+"/receipts/"+uuid.NewString(), nil).expect(http.StatusNotFound)
	api.do(http.MethodGet, gp+"/receipts/nope", nil).expect(http.StatusUnprocessableEntity)

	// An archived store cannot be used for new receipts.
	api.do(http.MethodPatch, gp+"/stores/"+sep.StoreID.String(), map[string]any{"archived": true}).expect(http.StatusOK)
	api.do(http.MethodPost, gp+"/receipts", receiptBody(map[string]any{"store_name": nil, "store_id": sep.StoreID})).expect(http.StatusUnprocessableEntity)
}

func TestGroceryReceiptAssignErrors(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	receipt := api.createReceipt(receiptBody(nil))
	otherReceipt := api.createReceipt(receiptBody(nil))
	line := receipt.Lines[0].ID

	for name, a := range map[string]map[string]any{
		"unknown line":        {"line_id": uuid.NewString(), "product": "X", "compare_unit": "kg"},
		"line of other":       {"line_id": otherReceipt.Lines[0].ID, "product": "X", "compare_unit": "kg"},
		"bad line id":         {"line_id": "nope"},
		"new product no unit": {"line_id": line, "product": "Cheese"},
		"bad unit":            {"line_id": line, "product": "Cheese", "compare_unit": "lb"},
		"unknown group":       {"line_id": line, "product": "Cheese", "compare_unit": "kg", "group": "Nope"},
		"unknown variant":     {"line_id": line, "variant_id": uuid.NewString()},
		"variant and product": {"line_id": line, "variant_id": uuid.NewString(), "product": "X"},
		"bad pack size":       {"line_id": line, "product": "Cheese", "compare_unit": "kg", "pack_size": 0},
	} {
		api.assign(receipt.ID, a).expect(http.StatusUnprocessableEntity)
		_ = name
	}
	api.assign(uuid.New(), map[string]any{"line_id": line}).expect(http.StatusNotFound)
	api.do(http.MethodPost, gp+"/receipts/"+receipt.ID.String()+"/assign", map[string]any{"assignments": []any{}}).expect(http.StatusUnprocessableEntity)

	// Atomic: a bad second assignment rolls back the first.
	api.assign(receipt.ID,
		map[string]any{"line_id": line, "product": "Milk", "compare_unit": "l"},
		map[string]any{"line_id": receipt.Lines[1].ID, "product": "Bread"},
	).expect(http.StatusUnprocessableEntity)
	if got := api.getReceipt(receipt.ID); got.UnassignedCount != 2 {
		t.Fatalf("expected nothing assigned, got %+v", got)
	}
	var products struct{ Items []groceries.Product }
	api.do(http.MethodGet, gp+"/products", nil).expect(http.StatusOK).decode(&products)
	if len(products.Items) != 0 {
		t.Fatalf("expected no products, got %+v", products.Items)
	}

	// Existing products are reused (case-insensitive) with new variants; same pack reuses the variant.
	api.assign(receipt.ID,
		map[string]any{"line_id": line, "product": "Milk", "compare_unit": "l", "brand": "Hacendado", "pack_size": 1},
		map[string]any{"line_id": receipt.Lines[1].ID, "product": "MILK", "brand": "Other", "pack_size": 1.5},
	).expect(http.StatusOK)
	api.assign(otherReceipt.ID,
		map[string]any{"line_id": otherReceipt.Lines[0].ID, "product": "milk", "brand": "hacendado", "pack_size": 1},
	).expect(http.StatusOK)
	api.do(http.MethodGet, gp+"/products", nil).expect(http.StatusOK).decode(&products)
	if len(products.Items) != 1 || products.Items[0].VariantCount != 2 {
		t.Fatalf("expected one product with two variants, got %+v", products.Items)
	}
}

func TestGroceryReceiptsAreIsolatedByWorkspace(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	receipt := api.createReceipt(receiptBody(nil))
	other := api.newUser("other@example.com", "Other")

	var list struct{ Items []groceries.ReceiptSummary }
	other.do(http.MethodGet, gp+"/receipts", nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 0 {
		t.Fatalf("other workspace sees receipts: %+v", list.Items)
	}
	id := receipt.ID.String()
	other.do(http.MethodGet, gp+"/receipts/"+id, nil).expect(http.StatusNotFound)
	other.do(http.MethodPatch, gp+"/receipts/"+id, map[string]any{"total": 5}).expect(http.StatusNotFound)
	other.do(http.MethodDelete, gp+"/receipts/"+id, nil).expect(http.StatusNotFound)
	other.assign(receipt.ID, map[string]any{"line_id": receipt.Lines[0].ID}).expect(http.StatusNotFound)
	// Rows of another workspace cannot be referenced.
	eur := api.createAccount("Checking", "EUR", 1000)
	expense := api.createExpenseTx(eur.ID, 1000)
	other.do(http.MethodPost, gp+"/receipts", receiptBody(map[string]any{"transaction_id": expense.ID})).expect(http.StatusUnprocessableEntity)
	other.do(http.MethodPost, gp+"/receipts", receiptBody(map[string]any{"store_name": nil, "store_id": receipt.StoreID})).expect(http.StatusUnprocessableEntity)
	if body := string(other.do(http.MethodGet, gp+"/stores", nil).expect(http.StatusOK).Body); strings.Contains(body, "Mercadona") {
		t.Fatalf("other workspace sees stores: %s", body)
	}
}
