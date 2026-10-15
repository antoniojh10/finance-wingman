package httpapi

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/groceries"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
)

const gp = "/api/v1/groceries"

func (a *testAPI) createGroceryStore(name string) groceries.Store {
	a.t.Helper()
	var s groceries.Store
	a.do(http.MethodPost, gp+"/stores", map[string]any{"name": name}).expect(http.StatusCreated).decode(&s)
	return s
}

func (a *testAPI) createGroceryProduct(body map[string]any) groceries.Product {
	a.t.Helper()
	var p groceries.Product
	a.do(http.MethodPost, gp+"/products", body).expect(http.StatusCreated).decode(&p)
	return p
}

func (a *testAPI) groceryGroups() []groceries.ProductGroup {
	a.t.Helper()
	var out struct{ Items []groceries.ProductGroup }
	a.do(http.MethodGet, gp+"/groups", nil).expect(http.StatusOK).decode(&out)
	return out.Items
}

func TestGroceriesRequireAuthentication(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t).as("")
	id := uuid.NewString()
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, gp + "/stores"}, {http.MethodPost, gp + "/stores"}, {http.MethodPatch, gp + "/stores/" + id},
		{http.MethodGet, gp + "/groups"}, {http.MethodPost, gp + "/groups"}, {http.MethodPatch, gp + "/groups/" + id},
		{http.MethodGet, gp + "/products"}, {http.MethodPost, gp + "/products"}, {http.MethodPatch, gp + "/products/" + id},
		{http.MethodGet, gp + "/products/" + id + "/variants"}, {http.MethodPost, gp + "/products/" + id + "/variants"},
		{http.MethodPatch, gp + "/variants/" + id}, {http.MethodDelete, gp + "/variants/" + id},
	} {
		api.do(c.method, c.path, nil).expect(http.StatusUnauthorized)
	}
}

func TestGroceryStores(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	mercadona := api.createGroceryStore("  Mercadona ")
	if mercadona.Name != "Mercadona" || mercadona.Archived {
		t.Fatalf("unexpected store %+v", mercadona)
	}

	api.do(http.MethodPost, gp+"/stores", map[string]any{"name": "mercadona"}).expect(http.StatusConflict)
	api.do(http.MethodPost, gp+"/stores", map[string]any{"name": "   "}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPost, gp+"/stores", map[string]any{}).expect(http.StatusUnprocessableEntity)

	var renamed groceries.Store
	api.do(http.MethodPatch, gp+"/stores/"+mercadona.ID.String(), map[string]any{"name": "Mercadona 2"}).expect(http.StatusOK).decode(&renamed)
	if renamed.Name != "Mercadona 2" {
		t.Fatalf("rename failed: %+v", renamed)
	}
	lidl := api.createGroceryStore("Lidl")
	api.do(http.MethodPatch, gp+"/stores/"+lidl.ID.String(), map[string]any{"name": "mercadona 2"}).expect(http.StatusConflict)
	api.do(http.MethodPatch, gp+"/stores/"+lidl.ID.String(), map[string]any{"name": ""}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPatch, gp+"/stores/"+uuid.NewString(), map[string]any{"name": "X"}).expect(http.StatusNotFound)
	api.do(http.MethodPatch, gp+"/stores/not-a-uuid", map[string]any{"name": "X"}).expect(http.StatusUnprocessableEntity)

	// Archived stores are hidden by default and free their name.
	api.do(http.MethodPatch, gp+"/stores/"+lidl.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK)
	var list struct{ Items []groceries.Store }
	api.do(http.MethodGet, gp+"/stores", nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 1 {
		t.Fatalf("expected only the active store, got %+v", list.Items)
	}
	api.do(http.MethodGet, gp+"/stores?include_archived=true", nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 2 || !list.Items[1].Archived {
		t.Fatalf("expected the archived store last, got %+v", list.Items)
	}
	api.createGroceryStore("Lidl")
	api.do(http.MethodPatch, gp+"/stores/"+lidl.ID.String(), map[string]any{"archived": false}).expect(http.StatusConflict)
}

func TestProductGroupsSeedDefaultsOnce(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	groups := api.groceryGroups()
	if len(groups) != 12 {
		t.Fatalf("expected 12 default groups, got %d", len(groups))
	}
	food := 0
	for _, g := range groups {
		if g.IsFood {
			food++
		}
	}
	if groups[0].Name != "Produce" || groups[11].Name != "Other" || food != 9 {
		t.Fatalf("unexpected defaults (food=%d): %+v", food, groups)
	}
	if again := api.groceryGroups(); len(again) != 12 {
		t.Fatalf("listing again must not duplicate, got %d", len(again))
	}

	// Concurrent first listings in a fresh workspace stay idempotent.
	fresh := api.newUser("fresh@example.com", "Fresh")
	var wg sync.WaitGroup
	for range 6 {
		wg.Go(func() {
			fresh.do(http.MethodGet, gp+"/groups", nil)
		})
	}
	wg.Wait()
	if got := fresh.groceryGroups(); len(got) != 12 {
		t.Fatalf("concurrent seeding produced %d groups", len(got))
	}
}

func TestProductGroups(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)

	var created groceries.ProductGroup
	api.do(http.MethodPost, gp+"/groups", map[string]any{"name": "Pets", "is_food": false, "color": "#112233"}).expect(http.StatusCreated).decode(&created)
	if created.Name != "Pets" || created.IsFood || created.Color == nil || created.Position != 12 {
		t.Fatalf("unexpected group %+v", created)
	}
	if len(api.groceryGroups()) != 13 {
		t.Fatal("custom group should come after the defaults")
	}

	api.do(http.MethodPost, gp+"/groups", map[string]any{"name": "pets", "is_food": false}).expect(http.StatusConflict)
	api.do(http.MethodPost, gp+"/groups", map[string]any{"name": "Bad", "is_food": true, "color": "red"}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPost, gp+"/groups", map[string]any{"name": "", "is_food": true}).expect(http.StatusUnprocessableEntity)

	var updated groceries.ProductGroup
	api.do(http.MethodPatch, gp+"/groups/"+created.ID.String(), map[string]any{"name": "Pet food", "is_food": true, "color": "", "position": 0}).expect(http.StatusOK).decode(&updated)
	if updated.Name != "Pet food" || !updated.IsFood || updated.Color != nil || updated.Position != 0 {
		t.Fatalf("unexpected update %+v", updated)
	}
	api.do(http.MethodPatch, gp+"/groups/"+created.ID.String(), map[string]any{"name": "produce"}).expect(http.StatusConflict)
	api.do(http.MethodPatch, gp+"/groups/"+created.ID.String(), map[string]any{"color": "nope"}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPatch, gp+"/groups/"+uuid.NewString(), map[string]any{"name": "X"}).expect(http.StatusNotFound)
}

func TestGroceryProducts(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	dairy := api.groceryGroups()[2]

	milk := api.createGroceryProduct(map[string]any{"name": "Whole milk", "group_id": dairy.ID, "compare_unit": "l"})
	if milk.GroupName == nil || *milk.GroupName != dairy.Name || milk.DisplayMultiplier != 1 || milk.VariantCount != 0 {
		t.Fatalf("unexpected product %+v", milk)
	}
	eggs := api.createGroceryProduct(map[string]any{"name": "Eggs", "compare_unit": "unit", "display_multiplier": 12})
	if eggs.GroupID != nil || eggs.DisplayMultiplier != 12 {
		t.Fatalf("unexpected product %+v", eggs)
	}
	api.createGroceryProduct(map[string]any{"name": "Semi-skimmed milk", "compare_unit": "l"})

	api.do(http.MethodPost, gp+"/products", map[string]any{"name": "whole MILK", "compare_unit": "l"}).expect(http.StatusConflict)
	api.do(http.MethodPost, gp+"/products", map[string]any{"name": "Rice", "compare_unit": "bag"}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPost, gp+"/products", map[string]any{"name": "Rice", "compare_unit": "kg", "display_multiplier": 0}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPost, gp+"/products", map[string]any{"name": " ", "compare_unit": "kg"}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPost, gp+"/products", map[string]any{"name": "Rice", "compare_unit": "kg", "group_id": uuid.NewString()}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPost, gp+"/products", map[string]any{"name": "Rice", "compare_unit": "kg", "group_id": "nope"}).expect(http.StatusUnprocessableEntity)

	var list struct{ Items []groceries.Product }
	api.do(http.MethodGet, gp+"/products", nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 3 || list.Items[0].Name != "Eggs" {
		t.Fatalf("unexpected list %+v", list.Items)
	}
	api.do(http.MethodGet, gp+"/products?group_id="+dairy.ID.String(), nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 1 || list.Items[0].ID != milk.ID {
		t.Fatalf("group filter failed: %+v", list.Items)
	}
	api.do(http.MethodGet, gp+"/products?group_id=nope", nil).expect(http.StatusUnprocessableEntity)

	var updated groceries.Product
	api.do(http.MethodPatch, gp+"/products/"+eggs.ID.String(), map[string]any{
		"name": "Free-range eggs", "group_id": dairy.ID, "compare_unit": "unit", "display_multiplier": 6,
	}).expect(http.StatusOK).decode(&updated)
	if updated.Name != "Free-range eggs" || updated.GroupID == nil || updated.DisplayMultiplier != 6 {
		t.Fatalf("unexpected update %+v", updated)
	}
	api.do(http.MethodPatch, gp+"/products/"+eggs.ID.String(), map[string]any{"group_id": ""}).expect(http.StatusOK).decode(&updated)
	if updated.GroupID != nil {
		t.Fatalf("group should be cleared: %+v", updated)
	}
	api.do(http.MethodPatch, gp+"/products/"+eggs.ID.String(), map[string]any{"name": "whole milk"}).expect(http.StatusConflict)
	api.do(http.MethodPatch, gp+"/products/"+eggs.ID.String(), map[string]any{"compare_unit": "bag"}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPatch, gp+"/products/"+eggs.ID.String(), map[string]any{"group_id": uuid.NewString()}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPatch, gp+"/products/"+uuid.NewString(), map[string]any{"name": "X"}).expect(http.StatusNotFound)

	api.do(http.MethodPatch, gp+"/products/"+eggs.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK).decode(&updated)
	if !updated.Archived {
		t.Fatal("product should be archived")
	}
	api.do(http.MethodGet, gp+"/products", nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 2 {
		t.Fatalf("archived products should be hidden: %+v", list.Items)
	}
	api.do(http.MethodGet, gp+"/products?include_archived=true", nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 3 {
		t.Fatalf("expected archived products too: %+v", list.Items)
	}
}

func TestProductVariants(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	store := api.createGroceryStore("Mercadona")
	milk := api.createGroceryProduct(map[string]any{"name": "Whole milk", "compare_unit": "l"})
	path := gp + "/products/" + milk.ID.String() + "/variants"

	var v groceries.Variant
	api.do(http.MethodPost, path, map[string]any{"brand": "Hacendado", "pack_size": 6, "pack_label": "6x1 l", "store_id": store.ID}).expect(http.StatusCreated).decode(&v)
	if v.ProductID != milk.ID || v.PackSize == nil || *v.PackSize != 6 || v.StoreID == nil || *v.StoreID != store.ID {
		t.Fatalf("unexpected variant %+v", v)
	}
	var loose groceries.Variant
	api.do(http.MethodPost, path, map[string]any{"brand": "Generic"}).expect(http.StatusCreated).decode(&loose)
	if loose.PackSize != nil {
		t.Fatalf("loose variant has a pack size: %+v", loose)
	}
	var fractional groceries.Variant
	api.do(http.MethodPost, path, map[string]any{"brand": "Generic", "pack_size": 0.125}).expect(http.StatusCreated).decode(&fractional)
	if fractional.PackSize == nil || *fractional.PackSize != 0.125 {
		t.Fatalf("unexpected pack size: %+v", fractional)
	}

	// Same brand (any case) and pack size, including two loose ones.
	api.do(http.MethodPost, path, map[string]any{"brand": "hacendado", "pack_size": 6}).expect(http.StatusConflict)
	api.do(http.MethodPost, path, map[string]any{"brand": "GENERIC"}).expect(http.StatusConflict)
	api.do(http.MethodPost, path, map[string]any{"brand": "Hacendado", "pack_size": 1}).expect(http.StatusCreated)
	api.do(http.MethodPost, path, map[string]any{"brand": ""}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPost, path, map[string]any{"brand": "X", "pack_size": 0}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPost, path, map[string]any{"brand": "X", "pack_size": -1}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPost, path, map[string]any{"brand": "X", "store_id": uuid.NewString()}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPost, gp+"/products/"+uuid.NewString()+"/variants", map[string]any{"brand": "X"}).expect(http.StatusNotFound)

	var list struct{ Items []groceries.Variant }
	api.do(http.MethodGet, path, nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 4 || list.Items[0].PackSize != nil && list.Items[0].Brand != "Generic" {
		t.Fatalf("unexpected list %+v", list.Items)
	}
	api.do(http.MethodGet, gp+"/products/"+uuid.NewString()+"/variants", nil).expect(http.StatusNotFound)
	var products struct{ Items []groceries.Product }
	api.do(http.MethodGet, gp+"/products", nil).expect(http.StatusOK).decode(&products)
	if products.Items[0].VariantCount != 4 {
		t.Fatalf("variant count: %+v", products.Items[0])
	}

	var updated groceries.Variant
	api.do(http.MethodPatch, gp+"/variants/"+v.ID.String(), map[string]any{"brand": "Carrefour", "pack_size": 4, "pack_label": "4x1 l", "store_id": ""}).expect(http.StatusOK).decode(&updated)
	if updated.Brand != "Carrefour" || *updated.PackSize != 4 || updated.StoreID != nil {
		t.Fatalf("unexpected update %+v", updated)
	}
	api.do(http.MethodPatch, gp+"/variants/"+v.ID.String(), map[string]any{"clear_pack_size": true, "pack_label": ""}).expect(http.StatusOK).decode(&updated)
	if updated.PackSize != nil || updated.PackLabel != nil {
		t.Fatalf("pack size should be cleared: %+v", updated)
	}
	api.do(http.MethodPatch, gp+"/variants/"+v.ID.String(), map[string]any{"brand": "Generic"}).expect(http.StatusConflict)
	api.do(http.MethodPatch, gp+"/variants/"+v.ID.String(), map[string]any{"pack_size": 2, "clear_pack_size": true}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPatch, gp+"/variants/"+v.ID.String(), map[string]any{"store_id": uuid.NewString()}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPatch, gp+"/variants/"+uuid.NewString(), map[string]any{"brand": "X"}).expect(http.StatusNotFound)

	api.do(http.MethodDelete, gp+"/variants/"+loose.ID.String(), nil).expect(http.StatusNoContent)
	api.do(http.MethodDelete, gp+"/variants/"+loose.ID.String(), nil).expect(http.StatusNotFound)
	api.do(http.MethodGet, path, nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 3 {
		t.Fatalf("expected 3 variants after delete, got %d", len(list.Items))
	}
}

func TestGroceriesAreIsolatedByWorkspace(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	store := api.createGroceryStore("Secret store")
	group := api.groceryGroups()[0]
	product := api.createGroceryProduct(map[string]any{"name": "Secret product", "compare_unit": "kg"})
	var v groceries.Variant
	api.do(http.MethodPost, gp+"/products/"+product.ID.String()+"/variants", map[string]any{"brand": "Secret brand"}).expect(http.StatusCreated).decode(&v)

	other := api.newUser("other@example.com", "Other")
	for _, path := range []string{"/stores?include_archived=true", "/groups", "/products?include_archived=true"} {
		body := string(other.do(http.MethodGet, gp+path, nil).expect(http.StatusOK).Body)
		for _, secret := range []string{"Secret", store.ID.String(), product.ID.String()} {
			if strings.Contains(body, secret) {
				t.Fatalf("GET %s leaks %q: %s", path, secret, body)
			}
		}
	}
	other.do(http.MethodGet, gp+"/products/"+product.ID.String()+"/variants", nil).expect(http.StatusNotFound)
	other.do(http.MethodPatch, gp+"/stores/"+store.ID.String(), map[string]any{"name": "Hijacked"}).expect(http.StatusNotFound)
	other.do(http.MethodPatch, gp+"/groups/"+group.ID.String(), map[string]any{"name": "Hijacked"}).expect(http.StatusNotFound)
	other.do(http.MethodPatch, gp+"/products/"+product.ID.String(), map[string]any{"name": "Hijacked"}).expect(http.StatusNotFound)
	other.do(http.MethodPatch, gp+"/variants/"+v.ID.String(), map[string]any{"brand": "Hijacked"}).expect(http.StatusNotFound)
	other.do(http.MethodDelete, gp+"/variants/"+v.ID.String(), nil).expect(http.StatusNotFound)
	// References to another workspace's rows are rejected.
	other.do(http.MethodPost, gp+"/products", map[string]any{"name": "Mine", "compare_unit": "kg", "group_id": group.ID}).expect(http.StatusUnprocessableEntity)

	// The same names are free in another workspace.
	other.createGroceryStore("Secret store")
}

// TestGroceriesTablesEnforceRowLevelSecurity checks the tables directly:
// another workspace sees no rows and cannot insert rows for this one.
func TestGroceriesTablesEnforceRowLevelSecurity(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	api.createGroceryStore("Mercadona")
	api.groceryGroups()
	api.createGroceryProduct(map[string]any{"name": "Milk", "compare_unit": "l"})

	otherID := testutil.NewWorkspace(t, api.pool, "Other")
	other := db.WithWorkspace(context.Background(), otherID)
	for _, table := range []string{
		"grocery_stores", "product_groups", "products", "product_variants", "grocery_receipts",
		"grocery_receipt_lines", "store_item_aliases", "price_checks",
	} {
		var n int
		if err := api.pool.QueryRow(other, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Fatalf("other workspace sees %d rows of %s", n, table)
		}
	}
	_, err := api.pool.Exec(other, `INSERT INTO grocery_stores (workspace_id, name) VALUES ($1, 'Smuggled')`, api.workspace.ID)
	if err == nil {
		t.Fatal("inserting a row for another workspace must fail")
	}
}
