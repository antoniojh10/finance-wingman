-- +goose Up
-- Every price observation of a product variant, receipt lines and manual
-- price checks, as a price per compare unit (kg, l or unit) in minor units.
-- security_invoker makes the view run with the privileges of the caller, so
-- the row-level security of the underlying tables applies.
--   receipt line: amount_minor / (quantity * coalesce(pack_size, 1))
--   price check:  price_minor / coalesce(pack_size, 1)
-- Lines without a variant and free lines (amount 0) are not observations.
CREATE VIEW grocery_unit_prices WITH (security_invoker = true) AS
SELECT
    'receipt'::text AS source,
    l.id AS observation_id,
    r.store_id,
    l.variant_id,
    v.product_id,
    r.currency,
    r.purchased_on AS observed_on,
    l.amount_minor::numeric / (l.quantity * coalesce(v.pack_size, 1)) AS unit_price_minor,
    r.created_at
FROM grocery_receipt_lines l
JOIN grocery_receipts r ON r.id = l.receipt_id
JOIN product_variants v ON v.id = l.variant_id
WHERE l.amount_minor > 0
UNION ALL
SELECT
    'price_check'::text,
    c.id,
    c.store_id,
    c.variant_id,
    v.product_id,
    c.currency,
    c.observed_on,
    c.price_minor::numeric / coalesce(v.pack_size, 1),
    c.created_at
FROM price_checks c
JOIN product_variants v ON v.id = c.variant_id;

-- +goose Down
DROP VIEW grocery_unit_prices;
