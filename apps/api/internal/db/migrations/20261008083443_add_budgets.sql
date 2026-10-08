-- +goose Up
-- Monthly budgets per expense category and currency. A row sets the amount
-- from its month on: months without a row inherit the latest earlier row
-- (see the effective budget query), so editing a month only affects that
-- month and the ones inheriting from it. A null amount_minor means "no
-- budget from this month on", which removes a budget without deleting
-- history. Currencies are never converted, hence one budget per currency.
-- The category kind (expense only) is enforced by the service.
CREATE TABLE budgets (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL DEFAULT current_workspace_id() REFERENCES workspaces (id) ON DELETE CASCADE,
    category_id  uuid NOT NULL,
    currency     text NOT NULL REFERENCES currencies (code),
    month        date NOT NULL CHECK (extract(day FROM month) = 1),
    amount_minor bigint CHECK (amount_minor >= 0),
    created_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),

    UNIQUE (workspace_id, category_id, currency, month),
    FOREIGN KEY (category_id, workspace_id) REFERENCES categories (id, workspace_id) ON DELETE CASCADE
);

CREATE TRIGGER budgets_set_updated_at BEFORE UPDATE ON budgets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE budgets ENABLE ROW LEVEL SECURITY;
ALTER TABLE budgets FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_isolation ON budgets
    USING (workspace_id = current_workspace_id()) WITH CHECK (workspace_id = current_workspace_id());

-- +goose Down
DROP TABLE budgets;
