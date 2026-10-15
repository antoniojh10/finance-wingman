package mcpserver

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func daysAgo(n int) string {
	return time.Now().UTC().AddDate(0, 0, -n).Format(time.DateOnly)
}

// priceCheck calls add_price_check with EUR litres of 1 l by default; a nil
// value removes the default.
func (h *harness) priceCheck(args map[string]any) priceCheckOut {
	h.t.Helper()
	base := map[string]any{"currency": "EUR", "compare_unit": "l", "pack_size": 1}
	for k, v := range args {
		if v == nil {
			delete(base, k)
		} else {
			base[k] = v
		}
	}
	var out priceCheckOut
	h.mustCall("add_price_check", base, &out)
	return out
}

// seedPrices records milk at two stores, eggs at one and bread at the other.
func (h *harness) seedPrices() {
	h.priceCheck(map[string]any{"store": "Mercadona", "product": "Whole milk", "brand": "Hacendado", "price": 1.15, "date": daysAgo(3)})
	h.priceCheck(map[string]any{"store": "Lidl", "new_store": true, "product": "Whole milk", "brand": "Milbona", "price": 0.99, "date": daysAgo(2)})
	h.priceCheck(map[string]any{"store": "Lidl", "product": "Eggs", "compare_unit": "unit", "pack_size": 12, "pack_label": "12 eggs", "price": 3.00, "date": daysAgo(2)})
	h.priceCheck(map[string]any{"store": "Mercadona", "product": "Bread", "compare_unit": "kg", "pack_size": 0.5, "price": 0.80, "date": daysAgo(1)})
}

func TestGroceryInsightToolsAreListed(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	names := h.toolNames()
	for _, name := range []string{"add_price_check", "get_grocery_breakdown", "compare_stores", "price_shopping_list"} {
		if !slices.Contains(names, name) {
			t.Errorf("missing tool %s", name)
		}
	}
	if !strings.Contains(h.session.InitializeResult().Instructions, "price_shopping_list") {
		t.Error("instructions should explain the price tools")
	}
}

func TestAddPriceCheckCreatesCatalogAndComputesUnitPrice(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	out := h.priceCheck(map[string]any{"store": "Mercadona", "product": "Whole milk", "brand": "Hacendado", "pack_label": "1 l", "price": 1.15, "group": "Dairy & eggs"})
	if out.Store != "Mercadona" || out.Product != "Whole milk" || out.Brand != "Hacendado" || out.Price != "1.15" || out.UnitPrice != "1.150" || out.Unit != "EUR/l" || out.Date != daysAgo(0) {
		t.Fatalf("unexpected price check: %+v", out)
	}

	// A bigger pack of the same product reuses it and divides by the pack size.
	six := h.priceCheck(map[string]any{"store": "mercadona", "product": "whole MILK", "brand": "Hacendado", "pack_size": 6, "pack_label": "6x1 l", "price": 5.40, "date": daysAgo(1), "compare_unit": nil})
	if six.UnitPrice != "0.900" || six.Product != "Whole milk" || six.PackLabel != "6x1 l" {
		t.Fatalf("unexpected pack price: %+v", six)
	}

	var cat catalogOut
	h.mustCall("list_grocery_catalog", map[string]any{}, &cat)
	if len(cat.Products) != 1 || len(cat.Products[0].Variants) != 2 || len(cat.Stores) != 1 {
		t.Fatalf("price checks should create one product with two variants: %+v", cat)
	}

	// Loose products have no pack: the price is per compare unit.
	loose := h.priceCheck(map[string]any{"store": "Mercadona", "product": "Apples", "compare_unit": "kg", "pack_size": nil, "price": 2.49})
	if loose.UnitPrice != "2.490" || loose.Unit != "EUR/kg" {
		t.Fatalf("unexpected loose price: %+v", loose)
	}
}

func TestAddPriceCheckCurrencyFromReceipt(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.mustFail("add_price_check", map[string]any{"store": "Mercadona", "product": "Milk", "compare_unit": "l", "price": 1}, "currency is required")

	h.receipt(map[string]any{"store": "Mercadona", "date": daysAgo(5), "total": 5, "currency": "MXN"})
	var out priceCheckOut
	h.mustCall("add_price_check", map[string]any{"store": "Mercadona", "product": "Milk", "compare_unit": "l", "pack_size": 1, "price": 24}, &out)
	if out.Currency != "MXN" || out.Unit != "MXN/l" || out.UnitPrice != "24.000" {
		t.Fatalf("currency should default to the latest receipt: %+v", out)
	}
}

func TestAddPriceCheckErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.priceCheck(map[string]any{"store": "Mercadona", "product": "Milk", "price": 1})
	base := func(extra map[string]any) map[string]any {
		args := map[string]any{"store": "Mercadona", "product": "Milk", "price": 1.2, "currency": "EUR", "pack_size": 1}
		for k, v := range extra {
			args[k] = v
		}
		return args
	}
	h.mustFail("add_price_check", base(map[string]any{"store": "Carrefour"}), "known stores: Mercadona")
	h.mustFail("add_price_check", base(map[string]any{"store": ""}), "store is required")
	h.mustFail("add_price_check", base(map[string]any{"product": " "}), "product is required")
	h.mustFail("add_price_check", base(map[string]any{"price": 0}), "greater than zero")
	h.mustFail("add_price_check", base(map[string]any{"price": 1.234}), "too many decimals")
	h.mustFail("add_price_check", base(map[string]any{"currency": "XXX"}), "unsupported currency")
	h.mustFail("add_price_check", base(map[string]any{"date": "yesterday"}), "YYYY-MM-DD")
	h.mustFail("add_price_check", base(map[string]any{"date": time.Now().UTC().AddDate(0, 0, 3).Format(time.DateOnly)}), "future")
	h.mustFail("add_price_check", base(map[string]any{"product": "Cheese"}), "compare_unit")
	h.mustFail("add_price_check", base(map[string]any{"product": "Cheese", "compare_unit": "kg", "group": "Spaceships"}), "valid groups")
	h.mustFail("add_price_check", base(map[string]any{"pack_size": -1}), "pack_size")

	// Failed calls create no product.
	var cat catalogOut
	h.mustCall("list_grocery_catalog", map[string]any{}, &cat)
	if len(cat.Products) != 1 {
		t.Fatalf("failures must not create products: %+v", cat)
	}

	// A confirmed new store is created.
	out := h.priceCheck(base(map[string]any{"store": "Aldi", "new_store": true}))
	if out.Store != "Aldi" {
		t.Fatalf("new store should be created: %+v", out)
	}
}

func TestCompareStores(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.seedPrices()

	var out compareOut
	h.mustCall("compare_stores", map[string]any{}, &out)
	if out.Level != "product" || len(out.Currencies) != 1 {
		t.Fatalf("unexpected comparison: %+v", out)
	}
	c := out.Currencies[0]
	if c.Currency != "EUR" || !slices.Equal(c.Stores, []string{"Lidl", "Mercadona"}) || len(c.Rows) != 3 {
		t.Fatalf("unexpected currency block: %+v", c)
	}
	var milk, eggs *compareRowOut
	for i := range c.Rows {
		switch c.Rows[i].Product {
		case "Whole milk":
			milk = &c.Rows[i]
		case "Eggs":
			eggs = &c.Rows[i]
		}
	}
	if milk == nil || eggs == nil {
		t.Fatalf("missing rows: %+v", c.Rows)
	}
	if milk.CheapestStore != "Lidl" || len(milk.Prices) != 2 || milk.Prices[0].Store != "Lidl" || milk.Prices[0].UnitPrice != "0.990" || milk.Prices[0].Unit != "EUR/l" || milk.Prices[1].UnitPrice != "1.150" {
		t.Fatalf("unexpected milk row: %+v", milk)
	}
	if milk.MaxDiffPct == nil || *milk.MaxDiffPct != 16.2 || milk.Prices[0].Source != "price_check" || milk.Prices[0].Stale {
		t.Fatalf("unexpected diff or source: %+v", milk)
	}
	// Eggs: a single store has a price, so there is no winner.
	if eggs.CheapestStore != "" || len(eggs.Prices) != 1 || eggs.Prices[0].UnitPrice != "0.250" || eggs.Prices[0].Unit != "EUR/unit" {
		t.Fatalf("unexpected eggs row: %+v", eggs)
	}

	// Filtering by product is case-insensitive and reports unknown names.
	var filtered compareOut
	text := h.mustCall("compare_stores", map[string]any{"products": []any{"whole milk", "Caviar"}}, &filtered)
	if len(filtered.Currencies) != 1 || len(filtered.Currencies[0].Rows) != 1 || !slices.Equal(filtered.UnknownProducts, []string{"Caviar"}) || !strings.Contains(text, "Caviar") {
		t.Fatalf("unexpected filtered result: %s / %+v", text, filtered)
	}

	var none compareOut
	text = h.mustCall("compare_stores", map[string]any{"products": []any{"Caviar"}}, &none)
	if len(none.Currencies) != 0 || !strings.Contains(text, "list_grocery_catalog") {
		t.Fatalf("unknown products should point to the catalog: %s", text)
	}

	// Brand level: no brand is priced in two stores.
	var brand compareOut
	h.mustCall("compare_stores", map[string]any{"level": "brand"}, &brand)
	if brand.Level != "brand" || len(brand.Currencies) != 0 {
		t.Fatalf("no brand is priced in two stores: %+v", brand)
	}

	var otherCurrency compareOut
	h.mustCall("compare_stores", map[string]any{"currency": "mxn"}, &otherCurrency)
	if len(otherCurrency.Currencies) != 0 {
		t.Fatalf("no MXN prices: %+v", otherCurrency)
	}

	h.mustFail("compare_stores", map[string]any{"level": "store"}, "product or brand")
	h.mustFail("compare_stores", map[string]any{"currency": "ZZZ"}, "unsupported currency")
}

func TestCompareStoresBrandLevel(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.priceCheck(map[string]any{"store": "Mercadona", "product": "Cola", "brand": "Coca-Cola", "price": 1.80, "date": daysAgo(2)})
	h.priceCheck(map[string]any{"store": "Lidl", "new_store": true, "product": "Cola", "brand": "Coca-Cola", "price": 1.50, "date": daysAgo(1)})

	var out compareOut
	h.mustCall("compare_stores", map[string]any{"level": "brand"}, &out)
	if len(out.Currencies) != 1 || len(out.Currencies[0].Rows) != 1 {
		t.Fatalf("unexpected brand comparison: %+v", out)
	}
	row := out.Currencies[0].Rows[0]
	if row.Brand != "Coca-Cola" || row.CheapestStore != "Lidl" || row.Prices[0].UnitPrice != "1.500" {
		t.Fatalf("unexpected brand row: %+v", row)
	}
}

func TestCompareStoresEmpty(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	var out compareOut
	text := h.mustCall("compare_stores", map[string]any{}, &out)
	if len(out.Currencies) != 0 || !strings.Contains(text, "No prices") {
		t.Fatalf("unexpected empty result: %s", text)
	}
}

func TestPriceShoppingList(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.seedPrices()

	var out shoppingListOut
	text := h.mustCall("price_shopping_list", map[string]any{"items": []any{
		map[string]any{"product": "whole milk", "quantity": 6},
		map[string]any{"product": "Eggs"},
		map[string]any{"product": "Whole Milk", "quantity": 2},
		map[string]any{"product": "Milk"},
	}}, &out)
	if len(out.Items) != 2 || out.Items[0].Product != "Whole milk" || out.Items[0].Quantity != 8 || out.Items[0].Unit != "l" || out.Items[1].Quantity != 1 {
		t.Fatalf("repeated products should add up: %+v", out.Items)
	}
	if len(out.Unknown) != 1 || out.Unknown[0].Product != "Milk" || !slices.Equal(out.Unknown[0].Suggestions, []string{"Whole milk"}) || !strings.Contains(text, "no such product: Milk") {
		t.Fatalf("unknown products should come with suggestions: %s / %+v", text, out.Unknown)
	}
	if len(out.Currencies) != 1 {
		t.Fatalf("unexpected currencies: %+v", out.Currencies)
	}
	c := out.Currencies[0]
	// Lidl: 8 l * 0.99 + 1 egg * 0.25 = 8.17. Mercadona has no eggs.
	if c.Currency != "EUR" || c.BestSingleStore != "Lidl" || c.BestSingleCost != "8.17" || len(c.Stores) != 2 {
		t.Fatalf("unexpected totals: %+v", c)
	}
	if c.Stores[0].Store != "Lidl" || !c.Stores[0].Complete || c.Stores[1].Store != "Mercadona" || c.Stores[1].Complete || !slices.Equal(c.Stores[1].Missing, []string{"Eggs"}) {
		t.Fatalf("complete stores should come first, with their gaps: %+v", c.Stores)
	}
	if c.SplitCost != "8.17" || len(c.Split) != 2 || c.Split[0].Store != "Lidl" || c.Split[0].Cost != "7.92" || c.Split[0].UnitPrice != "0.990" || c.Split[0].Unit != "EUR/l" {
		t.Fatalf("unexpected split: %+v", c.Split)
	}
}

func TestPriceShoppingListWithoutCompleteStoreAndUnpriced(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.seedPrices()
	// A product with only an old observation has no recent price.
	h.priceCheck(map[string]any{"store": "Lidl", "product": "Rice", "compare_unit": "kg", "price": 1.5, "date": daysAgo(400)})

	var out shoppingListOut
	h.mustCall("price_shopping_list", map[string]any{"items": []any{
		map[string]any{"product": "Eggs"}, map[string]any{"product": "Bread"}, map[string]any{"product": "Rice"},
	}}, &out)
	if len(out.Currencies) != 1 || out.Currencies[0].BestSingleStore != "" || !slices.Equal(out.Unpriced, []string{"Rice"}) {
		t.Fatalf("no store prices everything: %+v", out)
	}
	// Eggs 0.25 at Lidl + bread 1.60 per kg at Mercadona.
	if out.Currencies[0].SplitCost != "1.85" {
		t.Fatalf("unexpected split cost: %+v", out.Currencies[0])
	}
}

func TestPriceShoppingListErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.mustFail("price_shopping_list", map[string]any{"items": []any{}}, "at least one item")
	h.mustFail("price_shopping_list", map[string]any{"items": []any{map[string]any{"product": " "}}}, "items[0].product")
	h.mustFail("price_shopping_list", map[string]any{"items": []any{map[string]any{"product": "Milk", "quantity": 0}}}, "greater than zero")
	h.mustFail("price_shopping_list", map[string]any{"items": []any{map[string]any{"product": "Milk"}}, "currency": "ZZZ"}, "unsupported currency")

	// Only unknown products: nothing priced, still a useful answer.
	var out shoppingListOut
	text := h.mustCall("price_shopping_list", map[string]any{"items": []any{map[string]any{"product": "Milk"}}}, &out)
	if len(out.Items) != 0 || len(out.Unknown) != 1 || len(out.Currencies) != 0 || !strings.Contains(text, "Milk") {
		t.Fatalf("unexpected result: %s / %+v", text, out)
	}
}

func TestGroceryBreakdown(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Checking", "EUR")
	h.category("Groceries", "expense")

	var empty breakdownOut
	text := h.mustCall("get_grocery_breakdown", map[string]any{}, &empty)
	if len(empty.Currencies) != 0 || !strings.Contains(text, "No grocery receipts") {
		t.Fatalf("unexpected empty breakdown: %s", text)
	}

	r, _ := h.receipt(map[string]any{
		"store": "Mercadona", "date": daysAgo(0), "total": 3.00, "account": "Checking", "category": "Groceries",
		"lines": []any{
			map[string]any{"text": "LECHE", "amount": 2.00},
			map[string]any{"text": "OTRO", "amount": 1.00},
		},
	})
	h.mustCall("assign_receipt_lines", map[string]any{"receipt_id": r.ID, "assignments": []any{
		map[string]any{"line": 1, "product": "Whole milk", "compare_unit": "l", "pack_size": 1, "group": "Dairy & eggs"},
	}}, nil)

	var out breakdownOut
	text = h.mustCall("get_grocery_breakdown", map[string]any{"month": daysAgo(0)[:7], "currency": "eur"}, &out)
	if len(out.Currencies) != 1 {
		t.Fatalf("unexpected breakdown: %s / %+v", text, out)
	}
	c := out.Currencies[0]
	if c.Currency != "EUR" || c.Total != "3.00" || c.ReceiptCount != 1 || c.AverageReceipt != "3.00" || c.Unassigned != "1.00" || c.GroceriesCategory != "3.00" {
		t.Fatalf("unexpected totals: %+v", c)
	}
	if c.CoveragePct < 66 || c.CoveragePct > 67 || c.Food != "2.00" {
		t.Fatalf("unexpected coverage: %+v", c)
	}
	if len(c.ByStore) != 1 || c.ByStore[0].Store != "Mercadona" || c.ByStore[0].Total != "3.00" {
		t.Fatalf("unexpected by_store: %+v", c.ByStore)
	}
	if len(c.TopProducts) != 1 || c.TopProducts[0].Product != "Whole milk" || c.TopProducts[0].Spend != "2.00" || c.TopProducts[0].Unit != "l" {
		t.Fatalf("unexpected top products: %+v", c.TopProducts)
	}
	var groups []string
	for _, g := range c.ByGroup {
		groups = append(groups, g.Name)
	}
	if !slices.Contains(groups, "Dairy & eggs") || !slices.Contains(groups, "Ungrouped") {
		t.Fatalf("unexpected groups: %v", groups)
	}

	h.mustFail("get_grocery_breakdown", map[string]any{"month": "October"}, "YYYY-MM")
	h.mustFail("get_grocery_breakdown", map[string]any{"currency": "ZZZ"}, "unsupported currency")
}

func TestReadOnlyConnectionGroceryInsights(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.seedPrices()

	h.readOnly = true
	h.mustFail("add_price_check", map[string]any{"store": "Lidl", "product": "Whole milk", "price": 1, "currency": "EUR"}, "read-only")
	h.mustCall("compare_stores", map[string]any{}, nil)
	h.mustCall("price_shopping_list", map[string]any{"items": []any{map[string]any{"product": "Eggs"}}}, nil)
	h.mustCall("get_grocery_breakdown", map[string]any{}, nil)
}
