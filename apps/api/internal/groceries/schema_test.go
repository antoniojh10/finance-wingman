package groceries_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/groceries"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
)

func scan(t *testing.T, ctx context.Context, pool *pgxpool.Pool, dest any, sql string, args ...any) {
	t.Helper()
	if err := pool.QueryRow(ctx, sql, args...).Scan(dest); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func exec(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// TestReceiptSchema covers the tables the receipt and analytics work builds
// on: constraints and the SET NULL / CASCADE actions.
func TestReceiptSchema(t *testing.T) {
	t.Parallel()
	pool := testutil.NewDatabase(t, true)
	ctx := db.WithWorkspace(context.Background(), testutil.NewWorkspace(t, pool, "Home"))

	var store, product, variant, receipt, line, account, tx string
	scan(t, ctx, pool, &store, `INSERT INTO grocery_stores (name) VALUES ('Mercadona') RETURNING id`)
	scan(t, ctx, pool, &product, `INSERT INTO products (name, compare_unit) VALUES ('Milk', 'l') RETURNING id`)
	scan(t, ctx, pool, &variant, `INSERT INTO product_variants (product_id, brand, pack_size) VALUES ($1, 'Hacendado', 6) RETURNING id`, product)
	scan(t, ctx, pool, &account, `INSERT INTO accounts (name, type, currency) VALUES ('Checking', 'checking', 'EUR') RETURNING id`)
	scan(t, ctx, pool, &tx, `INSERT INTO transactions (type, account_id, amount, occurred_on) VALUES ('expense', $1, 1000, CURRENT_DATE) RETURNING id`, account)
	scan(t, ctx, pool, &receipt, `INSERT INTO grocery_receipts (store_id, transaction_id, purchased_on, currency, total_minor, source)
		VALUES ($1, $2, CURRENT_DATE, 'EUR', 1000, 'manual') RETURNING id`, store, tx)
	scan(t, ctx, pool, &line, `INSERT INTO grocery_receipt_lines (receipt_id, position, raw_text, quantity, amount_minor, variant_id)
		VALUES ($1, 1, 'LECHE', 1, 600, $2) RETURNING id`, receipt, variant)
	exec(t, ctx, pool, `INSERT INTO store_item_aliases (store_id, raw_text_key, variant_id) VALUES ($1, $2, $3)`, store, groceries.NormalizeRawText("leche"), variant)
	exec(t, ctx, pool, `INSERT INTO price_checks (store_id, variant_id, observed_on, price_minor, currency) VALUES ($1, $2, CURRENT_DATE, 500, 'EUR')`, store, variant)

	rejected := map[string]string{
		"second receipt for the same transaction": `INSERT INTO grocery_receipts (store_id, transaction_id, purchased_on, currency, total_minor, source) VALUES ('` + store + `', '` + tx + `', CURRENT_DATE, 'EUR', 1, 'manual')`,
		"zero total":              `INSERT INTO grocery_receipts (store_id, purchased_on, currency, total_minor, source) VALUES ('` + store + `', CURRENT_DATE, 'EUR', 0, 'manual')`,
		"unknown source":          `INSERT INTO grocery_receipts (store_id, purchased_on, currency, total_minor, source) VALUES ('` + store + `', CURRENT_DATE, 'EUR', 1, 'fax')`,
		"unknown currency":        `INSERT INTO grocery_receipts (store_id, purchased_on, currency, total_minor, source) VALUES ('` + store + `', CURRENT_DATE, 'XXX', 1, 'manual')`,
		"duplicate line position": `INSERT INTO grocery_receipt_lines (receipt_id, position, raw_text, quantity, amount_minor) VALUES ('` + receipt + `', 1, 'X', 1, 1)`,
		"zero quantity":           `INSERT INTO grocery_receipt_lines (receipt_id, position, raw_text, quantity, amount_minor) VALUES ('` + receipt + `', 2, 'X', 0, 1)`,
		"negative amount":         `INSERT INTO grocery_receipt_lines (receipt_id, position, raw_text, quantity, amount_minor) VALUES ('` + receipt + `', 2, 'X', 1, -1)`,
		"negative discount":       `INSERT INTO grocery_receipt_lines (receipt_id, position, raw_text, quantity, amount_minor, discount_minor) VALUES ('` + receipt + `', 2, 'X', 1, 1, -1)`,
		"duplicate alias":         `INSERT INTO store_item_aliases (store_id, raw_text_key, variant_id) VALUES ('` + store + `', 'LECHE', '` + variant + `')`,
		"zero price check":        `INSERT INTO price_checks (store_id, variant_id, observed_on, price_minor, currency) VALUES ('` + store + `', '` + variant + `', CURRENT_DATE, 0, 'EUR')`,
		"bad compare unit":        `INSERT INTO products (name, compare_unit) VALUES ('Bad', 'lb')`,
		"zero pack size":          `INSERT INTO product_variants (product_id, brand, pack_size) VALUES ('` + product + `', 'X', 0)`,
		"bad group color":         `INSERT INTO product_groups (name, is_food, color) VALUES ('Bad', true, 'red')`,
	}
	for name, sql := range rejected {
		if _, err := pool.Exec(ctx, sql); err == nil {
			t.Errorf("%s: expected the insert to be rejected", name)
		}
	}

	// Deleting the variant keeps the line (variant cleared) and drops aliases
	// and price checks.
	exec(t, ctx, pool, `DELETE FROM product_variants WHERE id = $1`, variant)
	var variantID *string
	scan(t, ctx, pool, &variantID, `SELECT variant_id::text FROM grocery_receipt_lines WHERE id = $1`, line)
	if variantID != nil {
		t.Fatalf("line should lose its variant, got %s", *variantID)
	}
	var n int
	scan(t, ctx, pool, &n, `SELECT (SELECT count(*) FROM store_item_aliases) + (SELECT count(*) FROM price_checks)`)
	if n != 0 {
		t.Fatalf("aliases and price checks should go with the variant, %d left", n)
	}

	// Deleting the transaction keeps the receipt.
	exec(t, ctx, pool, `DELETE FROM transactions WHERE id = $1`, tx)
	var txID *string
	scan(t, ctx, pool, &txID, `SELECT transaction_id::text FROM grocery_receipts WHERE id = $1`, receipt)
	if txID != nil {
		t.Fatalf("receipt should lose its transaction, got %s", *txID)
	}

	// Deleting the receipt removes its lines.
	exec(t, ctx, pool, `DELETE FROM grocery_receipts WHERE id = $1`, receipt)
	scan(t, ctx, pool, &n, `SELECT count(*) FROM grocery_receipt_lines`)
	if n != 0 {
		t.Fatalf("lines should be deleted with the receipt, %d left", n)
	}
}
