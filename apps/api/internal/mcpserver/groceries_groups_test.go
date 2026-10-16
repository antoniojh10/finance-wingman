package mcpserver

import (
	"strings"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/groceries"
)

// ungroupedProduct creates a product without a group, as the web app and
// older MCP calls could.
func (h *harness) ungroupedProduct(name, unit string) {
	h.t.Helper()
	if _, err := h.server.groc.CreateProduct(h.ctx, groceries.CreateProductInput{Name: name, CompareUnit: unit}); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) productGroup(name string) string {
	h.t.Helper()
	var cat catalogOut
	h.mustCall("list_grocery_catalog", map[string]any{}, &cat)
	for _, p := range cat.Products {
		if p.Name == name {
			return p.Group
		}
	}
	h.t.Fatalf("product %s not found in %+v", name, cat.Products)
	return ""
}

func TestReceiptHintsListGroupsAndAskForConfirmation(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	out, text := h.receipt(map[string]any{"store": "Mercadona", "date": "2026-10-01", "total": 1.75, "currency": "EUR", "lines": sampleLines()})
	for _, s := range []string{"Produce", "Dairy & eggs", "Cleaning & household", "Other", "group", "confirmation"} {
		if !strings.Contains(text, s) || !strings.Contains(out.Hint, s) {
			t.Errorf("add_receipt hint should mention %q: %s", s, text)
		}
	}
	text = h.mustCall("get_receipt", map[string]any{"id": out.ID}, nil)
	if !strings.Contains(text, "Dairy & eggs") || !strings.Contains(text, "confirmation") {
		t.Errorf("get_receipt hint should list groups: %s", text)
	}
	text = h.mustCall("assign_receipt_lines", map[string]any{"receipt_id": out.ID, "assignments": []any{
		map[string]any{"line": 1, "product": "Whole milk", "compare_unit": "l", "group": "Dairy & eggs"},
	}}, nil)
	if !strings.Contains(text, "Bakery") || !strings.Contains(text, "confirmation") {
		t.Errorf("assign_receipt_lines hint should list groups while lines remain: %s", text)
	}
}

func TestCatalogTextListsGroupsAndProductGroups(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.ungroupedProduct("Panko", "kg")
	h.mustCall("add_price_check", map[string]any{"store": "Mercadona", "product": "Whole milk", "compare_unit": "l", "group": "Dairy & eggs", "price": 1, "currency": "EUR", "pack_size": 1}, nil)

	var cat catalogOut
	text := h.mustCall("list_grocery_catalog", map[string]any{}, &cat)
	for _, g := range cat.Groups {
		if !strings.Contains(text, g.Name) {
			t.Errorf("text should list group %q: %s", g.Name, text)
		}
	}
	for _, s := range []string{"Whole milk (Dairy & eggs", "Panko (no group", "Mercadona"} {
		if !strings.Contains(text, s) {
			t.Errorf("text should contain %q: %s", s, text)
		}
	}
}

func TestCreatingAProductRequiresAGroup(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r, _ := h.receipt(map[string]any{"store": "Mercadona", "date": "2026-10-01", "total": 1.75, "currency": "EUR", "lines": sampleLines()})

	text, isErr := h.call("assign_receipt_lines", map[string]any{"receipt_id": r.ID, "assignments": []any{
		map[string]any{"line": 1, "product": "Whole milk", "compare_unit": "l", "group": "Dairy & eggs"},
		map[string]any{"line": 2, "product": "Bread", "compare_unit": "unit"},
	}}, nil)
	if !isErr || !strings.Contains(text, "assignments[1].group is required") || !strings.Contains(text, `"Bread"`) || !strings.Contains(text, "Other") || !strings.Contains(text, "Bakery") {
		t.Fatalf("expected a group error naming the product and the valid groups: %s", text)
	}
	var got receiptOut
	h.mustCall("get_receipt", map[string]any{"id": r.ID}, &got)
	if got.UnassignedCount != 2 {
		t.Fatalf("nothing must be assigned when a group is missing: %+v", got)
	}

	h.mustFail("add_price_check", map[string]any{"store": "Mercadona", "product": "Cheese", "compare_unit": "kg", "price": 5, "currency": "EUR"}, "product.group is required")
	h.mustFail("add_price_check", map[string]any{"store": "Mercadona", "product": "Cheese", "compare_unit": "kg", "group": "Spaceships", "price": 5, "currency": "EUR"}, "valid groups")

	// The same new product twice in one call needs the group once.
	h.mustCall("assign_receipt_lines", map[string]any{"receipt_id": r.ID, "assignments": []any{
		map[string]any{"line": 1, "product": "Bread", "compare_unit": "unit", "group": "Bakery"},
		map[string]any{"line": 2, "product": "bread", "brand": "Bimbo"},
	}}, nil)
}

func TestExistingProductGroupIsSetOrReportedNeverIgnored(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.ungroupedProduct("Yellow onion", "kg")
	r, _ := h.receipt(map[string]any{"store": "Mercadona", "date": "2026-10-01", "total": 1.75, "currency": "EUR", "lines": sampleLines()})
	assign := func(line int, product, group string) string {
		return h.mustCall("assign_receipt_lines", map[string]any{"receipt_id": r.ID, "assignments": []any{
			map[string]any{"line": line, "product": product, "group": group},
		}}, nil)
	}

	// No group yet: the given one is set.
	text := assign(1, "yellow onion", "Produce")
	if !strings.Contains(text, "Set the group of Yellow onion to Produce") || h.productGroup("Yellow onion") != "Produce" {
		t.Fatalf("the group should be set: %s", text)
	}
	// Same group: nothing to say.
	if text = assign(1, "Yellow onion", "produce"); strings.Contains(text, "Kept") || strings.Contains(text, "Set the group") {
		t.Fatalf("same group should be a no-op: %s", text)
	}
	// Different group: kept and reported.
	text = assign(2, "Yellow onion", "Pantry")
	if !strings.Contains(text, "Kept Produce for Yellow onion instead of Pantry") || !strings.Contains(text, "update_grocery_product") || h.productGroup("Yellow onion") != "Produce" {
		t.Fatalf("the conflicting group should be reported, not applied: %s", text)
	}
	// No group given for an existing product is fine.
	assign(2, "Yellow onion", "")

	// Same rules for price checks.
	h.ungroupedProduct("Panko", "kg")
	pc := map[string]any{"store": "Mercadona", "product": "Panko", "compare_unit": "kg", "price": 2, "currency": "EUR", "pack_size": 0.5}
	pc["group"] = "Pantry"
	if text = h.mustCall("add_price_check", pc, nil); !strings.Contains(text, "Set the group of Panko to Pantry") || h.productGroup("Panko") != "Pantry" {
		t.Fatalf("price check should set the group: %s", text)
	}
	pc["group"] = "Snacks & sweets"
	if text = h.mustCall("add_price_check", pc, nil); !strings.Contains(text, "Kept Pantry for Panko") || h.productGroup("Panko") != "Pantry" {
		t.Fatalf("price check should report the conflict: %s", text)
	}
}

func TestUpdateGroceryProduct(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.ungroupedProduct("Yellow onion", "kg")
	h.ungroupedProduct("Red onion", "kg")

	var out updateProductOut
	text := h.mustCall("update_grocery_product", map[string]any{"product": "yellow ONION", "group": "Produce"}, &out)
	if out.Name != "Yellow onion" || out.Group != "Produce" || !strings.Contains(text, "group Produce") || h.productGroup("Yellow onion") != "Produce" {
		t.Fatalf("regroup failed: %s / %+v", text, out)
	}
	h.mustCall("update_grocery_product", map[string]any{"product": "Yellow onion", "new_name": "Onion", "group": "Pantry"}, &out)
	if out.Name != "Onion" || out.Group != "Pantry" || h.productGroup("Onion") != "Pantry" {
		t.Fatalf("rename and regroup failed: %+v", out)
	}
	h.mustCall("update_grocery_product", map[string]any{"product": "Onion", "new_name": "Sweet onion"}, &out)
	if out.Name != "Sweet onion" || out.Group != "Pantry" {
		t.Fatalf("rename alone should keep the group: %+v", out)
	}

	h.mustFail("update_grocery_product", map[string]any{"product": "Sweet carrot", "group": "Produce"}, "no product named")
	h.mustFail("update_grocery_product", map[string]any{"product": "red onions", "group": "Produce"}, "similar products: Red onion")
	h.mustFail("update_grocery_product", map[string]any{"product": "Red onion", "group": "Spaceships"}, "valid groups: Produce")
	h.mustFail("update_grocery_product", map[string]any{"product": "Red onion", "new_name": "sweet ONION"}, "already exists")
	h.mustFail("update_grocery_product", map[string]any{"product": "Red onion"}, "nothing to change")
	h.mustFail("update_grocery_product", map[string]any{"product": " ", "group": "Produce"}, "product is required")

	h.readOnly = true
	h.mustFail("update_grocery_product", map[string]any{"product": "Red onion", "group": "Produce"}, "read-only")
	h.readOnly = false
	if h.productGroup("Red onion") != "" {
		t.Fatal("a read-only connection must not change products")
	}
}
