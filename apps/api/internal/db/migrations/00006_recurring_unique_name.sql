-- +goose Up
-- Names are unique (case-insensitive) among non-cancelled recurring items,
-- so a cancelled name can be reused for a new item.
CREATE UNIQUE INDEX recurring_items_active_name_key
    ON recurring_items (lower(name)) WHERE status <> 'cancelled';

-- +goose Down
DROP INDEX recurring_items_active_name_key;
