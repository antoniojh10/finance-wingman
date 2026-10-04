package httpapi

import (
	"net/http"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

func TestCreateCategory(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)

	var category finance.Category
	api.do(http.MethodPost, "/api/v1/categories", map[string]any{
		"name": "Groceries", "kind": "expense", "color": "#22c55e", "icon": "cart",
	}).expect(http.StatusCreated).decode(&category)
	if category.Name != "Groceries" || category.Kind != "expense" || *category.Color != "#22c55e" || *category.Icon != "cart" {
		t.Fatalf("unexpected category: %+v", category)
	}

	tests := []struct {
		name   string
		body   map[string]any
		status int
	}{
		{"duplicate name and kind", map[string]any{"name": "groceries", "kind": "expense"}, http.StatusConflict},
		{"invalid kind", map[string]any{"name": "X", "kind": "transfer"}, http.StatusUnprocessableEntity},
		{"invalid color", map[string]any{"name": "X", "kind": "expense", "color": "green"}, http.StatusUnprocessableEntity},
		{"missing name", map[string]any{"kind": "income"}, http.StatusUnprocessableEntity},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			api.do(http.MethodPost, "/api/v1/categories", tc.body).expect(tc.status)
		})
	}

	// The same name is allowed for a different kind.
	api.createCategory("Groceries", "income")
}

func TestListCategories(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	api.createCategory("Food", "expense")
	api.createCategory("Salary", "income")
	old := api.createCategory("Old", "expense")
	api.do(http.MethodPatch, "/api/v1/categories/"+old.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK)

	var body struct{ Items []finance.Category }
	api.do(http.MethodGet, "/api/v1/categories", nil).expect(http.StatusOK).decode(&body)
	if len(body.Items) != 2 {
		t.Fatalf("expected 2 active categories, got %d", len(body.Items))
	}

	api.do(http.MethodGet, "/api/v1/categories?kind=income", nil).expect(http.StatusOK).decode(&body)
	if len(body.Items) != 1 || body.Items[0].Name != "Salary" {
		t.Fatalf("unexpected income categories: %+v", body.Items)
	}

	api.do(http.MethodGet, "/api/v1/categories?kind=expense&include_archived=true", nil).expect(http.StatusOK).decode(&body)
	if len(body.Items) != 2 {
		t.Fatalf("expected 2 expense categories including archived, got %d", len(body.Items))
	}

	api.do(http.MethodGet, "/api/v1/categories?kind=bogus", nil).expect(http.StatusUnprocessableEntity)
}

func TestGetCategory(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	created := api.createCategory("Rent", "expense")

	var got finance.Category
	api.do(http.MethodGet, "/api/v1/categories/"+created.ID.String(), nil).expect(http.StatusOK).decode(&got)
	if got.ID != created.ID {
		t.Fatalf("unexpected category: %+v", got)
	}
	api.do(http.MethodGet, "/api/v1/categories/"+missingID, nil).expectError(http.StatusNotFound)
	api.do(http.MethodGet, "/api/v1/categories/123", nil).expect(http.StatusUnprocessableEntity)
}

func TestUpdateCategory(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	category := api.createCategory("Fun", "expense")
	api.createCategory("Taken", "expense")
	path := "/api/v1/categories/" + category.ID.String()

	var updated finance.Category
	api.do(http.MethodPatch, path, map[string]any{"name": "Entertainment", "color": "#ff0000", "icon": "film"}).
		expect(http.StatusOK).decode(&updated)
	if updated.Name != "Entertainment" || *updated.Color != "#ff0000" || *updated.Icon != "film" {
		t.Fatalf("unexpected update: %+v", updated)
	}

	// Empty strings clear optional fields; omitted fields stay untouched.
	api.do(http.MethodPatch, path, map[string]any{"color": ""}).expect(http.StatusOK).decode(&updated)
	if updated.Color != nil || updated.Icon == nil {
		t.Fatalf("expected color cleared and icon kept: %+v", updated)
	}

	api.do(http.MethodPatch, path, map[string]any{"name": "TAKEN"}).expectError(http.StatusConflict)
	api.do(http.MethodPatch, path, map[string]any{"color": "#12"}).expect(http.StatusUnprocessableEntity)
	api.do(http.MethodPatch, "/api/v1/categories/"+missingID, map[string]any{"name": "x"}).expectError(http.StatusNotFound)
}

func TestDeleteCategory(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Cash", "MXN", 0)
	category := api.createCategory("Snacks", "expense")
	tx := api.createTransaction(map[string]any{"type": "expense", "account_id": account.ID, "amount": 100, "category_id": category.ID})

	api.do(http.MethodDelete, "/api/v1/categories/"+category.ID.String(), nil).expect(http.StatusNoContent)
	api.do(http.MethodDelete, "/api/v1/categories/"+category.ID.String(), nil).expectError(http.StatusNotFound)

	var got finance.Transaction
	api.do(http.MethodGet, "/api/v1/transactions/"+tx.ID.String(), nil).expect(http.StatusOK).decode(&got)
	if got.CategoryID != nil {
		t.Fatal("expected transaction to become uncategorized")
	}
}
