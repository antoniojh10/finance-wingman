-- +goose Up
-- Detected recurring-item suggestions the user dismissed, so they do not
-- reappear. A suggestion is identified by its account, type and normalized
-- description, whatever frequency or amount it is detected with.
CREATE TABLE recurring_dismissed_suggestions (
    account_id   uuid NOT NULL REFERENCES accounts (id) ON DELETE CASCADE,
    type         text NOT NULL CHECK (type IN ('expense', 'income')),
    description  text NOT NULL CHECK (description <> ''),
    dismissed_by uuid REFERENCES users (id) ON DELETE SET NULL,
    dismissed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (account_id, type, description)
);

-- +goose Down
DROP TABLE recurring_dismissed_suggestions;
