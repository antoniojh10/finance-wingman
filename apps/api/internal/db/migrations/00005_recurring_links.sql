-- +goose Up
-- Links a transaction to the recurring item it pays and to the due date
-- (period) it settles. Both columns are set together or both are null.
ALTER TABLE transactions
    ADD COLUMN recurring_id uuid REFERENCES recurring_items (id) ON DELETE SET NULL,
    ADD COLUMN recurring_due_on date,
    ADD CONSTRAINT transactions_recurring_link_check
        CHECK ((recurring_id IS NULL) = (recurring_due_on IS NULL));

CREATE INDEX transactions_recurring_idx ON transactions (recurring_id, recurring_due_on);

-- +goose StatementBegin
-- The foreign key action would null recurring_id alone and violate the
-- check above, so clear both columns before the row is deleted.
CREATE FUNCTION recurring_items_unlink_transactions() RETURNS trigger AS $$
BEGIN
    UPDATE transactions SET recurring_id = NULL, recurring_due_on = NULL WHERE recurring_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER recurring_items_unlink_transactions BEFORE DELETE ON recurring_items
    FOR EACH ROW EXECUTE FUNCTION recurring_items_unlink_transactions();

-- +goose Down
DROP TRIGGER recurring_items_unlink_transactions ON recurring_items;
DROP FUNCTION recurring_items_unlink_transactions();
DROP INDEX transactions_recurring_idx;
ALTER TABLE transactions
    DROP CONSTRAINT transactions_recurring_link_check,
    DROP COLUMN recurring_due_on,
    DROP COLUMN recurring_id;
