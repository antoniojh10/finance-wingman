-- +goose Up
-- Groceries module: stores (chains), a product catalog with brand/pack
-- variants, receipts with their lines, store-specific raw text aliases and
-- price observations. The only link with the finance tables is the nullable
-- grocery_receipts.transaction_id. Every table is isolated per workspace
-- like the finance tables. Composite foreign keys keep references inside
-- the workspace; SET NULL actions list the column so workspace_id (part of
-- the key) is never nulled.

-- Receipts reference transactions with a workspace-scoped composite key like
-- the other finance tables do, which needs this (additive) unique key.
ALTER TABLE transactions ADD CONSTRAINT transactions_id_workspace_id_key UNIQUE (id, workspace_id);

CREATE TABLE grocery_stores (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL DEFAULT current_workspace_id() REFERENCES workspaces (id) ON DELETE CASCADE,
    name         text NOT NULL,
    archived_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),

    UNIQUE (id, workspace_id)
);
CREATE UNIQUE INDEX grocery_stores_name_active_key ON grocery_stores (workspace_id, lower(name)) WHERE archived_at IS NULL;

CREATE TABLE product_groups (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL DEFAULT current_workspace_id() REFERENCES workspaces (id) ON DELETE CASCADE,
    name         text NOT NULL,
    is_food      boolean NOT NULL DEFAULT true,
    color        text CHECK (color ~ '^#[0-9a-fA-F]{6}$'),
    position     int NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),

    UNIQUE (id, workspace_id)
);
CREATE UNIQUE INDEX product_groups_name_key ON product_groups (workspace_id, lower(name));

CREATE TABLE products (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id       uuid NOT NULL DEFAULT current_workspace_id() REFERENCES workspaces (id) ON DELETE CASCADE,
    name               text NOT NULL,
    group_id           uuid,
    -- Unit prices are compared per kg, per litre or per unit.
    compare_unit       text NOT NULL CHECK (compare_unit IN ('kg', 'l', 'unit')),
    -- Display multiplier of the unit price (12: priced "per dozen").
    display_multiplier int NOT NULL DEFAULT 1 CHECK (display_multiplier > 0),
    archived_at        timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),

    UNIQUE (id, workspace_id),
    FOREIGN KEY (group_id, workspace_id) REFERENCES product_groups (id, workspace_id) ON DELETE SET NULL (group_id)
);
CREATE UNIQUE INDEX products_name_active_key ON products (workspace_id, lower(name)) WHERE archived_at IS NULL;

CREATE TABLE product_variants (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL DEFAULT current_workspace_id() REFERENCES workspaces (id) ON DELETE CASCADE,
    product_id   uuid NOT NULL,
    -- "Generic" for unbranded products.
    brand        text NOT NULL,
    -- Size of the pack in the product's compare unit. Null: sold loose, the
    -- quantity of a receipt line is the amount.
    pack_size    numeric(12, 3) CHECK (pack_size > 0),
    pack_label   text,
    -- Set for the store brand of a chain.
    store_id     uuid,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),

    UNIQUE (id, workspace_id),
    FOREIGN KEY (product_id, workspace_id) REFERENCES products (id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (store_id, workspace_id) REFERENCES grocery_stores (id, workspace_id) ON DELETE SET NULL (store_id)
);
CREATE UNIQUE INDEX product_variants_identity_key ON product_variants (product_id, lower(brand), pack_size) NULLS NOT DISTINCT;

CREATE TABLE grocery_receipts (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id   uuid NOT NULL DEFAULT current_workspace_id() REFERENCES workspaces (id) ON DELETE CASCADE,
    store_id       uuid NOT NULL,
    -- The expense the receipt belongs to. The total should match it, but this
    -- is not enforced.
    transaction_id uuid,
    purchased_on   date NOT NULL,
    currency       char(3) NOT NULL REFERENCES currencies (code),
    total_minor    bigint NOT NULL CHECK (total_minor > 0),
    source         text NOT NULL CHECK (source IN ('photo', 'digital', 'manual')),
    created_by     uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),

    UNIQUE (id, workspace_id),
    FOREIGN KEY (store_id, workspace_id) REFERENCES grocery_stores (id, workspace_id) ON DELETE RESTRICT,
    FOREIGN KEY (transaction_id, workspace_id) REFERENCES transactions (id, workspace_id) ON DELETE SET NULL (transaction_id)
);
CREATE UNIQUE INDEX grocery_receipts_transaction_key ON grocery_receipts (transaction_id) WHERE transaction_id IS NOT NULL;
CREATE INDEX grocery_receipts_store_date_idx ON grocery_receipts (workspace_id, store_id, purchased_on);

CREATE TABLE grocery_receipt_lines (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id   uuid NOT NULL DEFAULT current_workspace_id() REFERENCES workspaces (id) ON DELETE CASCADE,
    receipt_id     uuid NOT NULL,
    position       int NOT NULL,
    raw_text       text NOT NULL,
    quantity       numeric(10, 3) NOT NULL CHECK (quantity > 0),
    -- Paid for the line, after the discount.
    amount_minor   bigint NOT NULL CHECK (amount_minor >= 0),
    discount_minor bigint NOT NULL DEFAULT 0 CHECK (discount_minor >= 0),
    variant_id     uuid,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),

    UNIQUE (id, workspace_id),
    UNIQUE (receipt_id, position),
    FOREIGN KEY (receipt_id, workspace_id) REFERENCES grocery_receipts (id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (variant_id, workspace_id) REFERENCES product_variants (id, workspace_id) ON DELETE SET NULL (variant_id)
);
CREATE INDEX grocery_receipt_lines_variant_idx ON grocery_receipt_lines (variant_id) WHERE variant_id IS NOT NULL;

CREATE TABLE store_item_aliases (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL DEFAULT current_workspace_id() REFERENCES workspaces (id) ON DELETE CASCADE,
    store_id     uuid NOT NULL,
    -- Receipt text normalized by groceries.NormalizeRawText.
    raw_text_key text NOT NULL,
    variant_id   uuid NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),

    UNIQUE (id, workspace_id),
    UNIQUE (workspace_id, store_id, raw_text_key),
    FOREIGN KEY (store_id, workspace_id) REFERENCES grocery_stores (id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (variant_id, workspace_id) REFERENCES product_variants (id, workspace_id) ON DELETE CASCADE
);

CREATE TABLE price_checks (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL DEFAULT current_workspace_id() REFERENCES workspaces (id) ON DELETE CASCADE,
    store_id     uuid NOT NULL,
    variant_id   uuid NOT NULL,
    observed_on  date NOT NULL,
    -- Price of the whole pack.
    price_minor  bigint NOT NULL CHECK (price_minor > 0),
    currency     char(3) NOT NULL REFERENCES currencies (code),
    note         text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),

    UNIQUE (id, workspace_id),
    FOREIGN KEY (store_id, workspace_id) REFERENCES grocery_stores (id, workspace_id) ON DELETE CASCADE,
    FOREIGN KEY (variant_id, workspace_id) REFERENCES product_variants (id, workspace_id) ON DELETE CASCADE
);
CREATE INDEX price_checks_variant_idx ON price_checks (workspace_id, variant_id, observed_on);

CREATE TRIGGER grocery_stores_set_updated_at BEFORE UPDATE ON grocery_stores
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
ALTER TABLE grocery_stores ENABLE ROW LEVEL SECURITY;
ALTER TABLE grocery_stores FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_isolation ON grocery_stores
    USING (workspace_id = current_workspace_id()) WITH CHECK (workspace_id = current_workspace_id());

CREATE TRIGGER product_groups_set_updated_at BEFORE UPDATE ON product_groups
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
ALTER TABLE product_groups ENABLE ROW LEVEL SECURITY;
ALTER TABLE product_groups FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_isolation ON product_groups
    USING (workspace_id = current_workspace_id()) WITH CHECK (workspace_id = current_workspace_id());

CREATE TRIGGER products_set_updated_at BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
ALTER TABLE products ENABLE ROW LEVEL SECURITY;
ALTER TABLE products FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_isolation ON products
    USING (workspace_id = current_workspace_id()) WITH CHECK (workspace_id = current_workspace_id());

CREATE TRIGGER product_variants_set_updated_at BEFORE UPDATE ON product_variants
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
ALTER TABLE product_variants ENABLE ROW LEVEL SECURITY;
ALTER TABLE product_variants FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_isolation ON product_variants
    USING (workspace_id = current_workspace_id()) WITH CHECK (workspace_id = current_workspace_id());

CREATE TRIGGER grocery_receipts_set_updated_at BEFORE UPDATE ON grocery_receipts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
ALTER TABLE grocery_receipts ENABLE ROW LEVEL SECURITY;
ALTER TABLE grocery_receipts FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_isolation ON grocery_receipts
    USING (workspace_id = current_workspace_id()) WITH CHECK (workspace_id = current_workspace_id());

CREATE TRIGGER grocery_receipt_lines_set_updated_at BEFORE UPDATE ON grocery_receipt_lines
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
ALTER TABLE grocery_receipt_lines ENABLE ROW LEVEL SECURITY;
ALTER TABLE grocery_receipt_lines FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_isolation ON grocery_receipt_lines
    USING (workspace_id = current_workspace_id()) WITH CHECK (workspace_id = current_workspace_id());

CREATE TRIGGER store_item_aliases_set_updated_at BEFORE UPDATE ON store_item_aliases
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
ALTER TABLE store_item_aliases ENABLE ROW LEVEL SECURITY;
ALTER TABLE store_item_aliases FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_isolation ON store_item_aliases
    USING (workspace_id = current_workspace_id()) WITH CHECK (workspace_id = current_workspace_id());

CREATE TRIGGER price_checks_set_updated_at BEFORE UPDATE ON price_checks
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
ALTER TABLE price_checks ENABLE ROW LEVEL SECURITY;
ALTER TABLE price_checks FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_isolation ON price_checks
    USING (workspace_id = current_workspace_id()) WITH CHECK (workspace_id = current_workspace_id());

-- +goose Down
DROP TABLE price_checks;
DROP TABLE store_item_aliases;
DROP TABLE grocery_receipt_lines;
DROP TABLE grocery_receipts;
DROP TABLE product_variants;
DROP TABLE products;
DROP TABLE product_groups;
DROP TABLE grocery_stores;
ALTER TABLE transactions DROP CONSTRAINT transactions_id_workspace_id_key;
