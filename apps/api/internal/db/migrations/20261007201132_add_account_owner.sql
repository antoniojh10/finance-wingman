-- +goose Up
-- An account may belong to a workspace member (e.g. each person's own bank
-- account) or to nobody, meaning it is shared. The owner is only a label:
-- every member still manages every account. Referencing the membership keeps
-- the owner inside the workspace, and when they leave the workspace their
-- accounts become shared instead of being deleted.
ALTER TABLE accounts
    ADD COLUMN owner_user_id uuid,
    ADD CONSTRAINT accounts_owner_fkey FOREIGN KEY (workspace_id, owner_user_id)
        REFERENCES workspace_members (workspace_id, user_id) ON DELETE SET NULL (owner_user_id);

CREATE INDEX accounts_owner_idx ON accounts (workspace_id, owner_user_id);

-- Two members can each have an account with the same name ("BNP"); names
-- stay unique per owner, with shared accounts forming their own group.
DROP INDEX accounts_name_active_key;
CREATE UNIQUE INDEX accounts_name_active_key ON accounts (workspace_id, owner_user_id, lower(name))
    NULLS NOT DISTINCT WHERE archived_at IS NULL;

-- +goose Down
-- Fails if active accounts of different owners share a name; rename them first.
DROP INDEX accounts_name_active_key;
CREATE UNIQUE INDEX accounts_name_active_key ON accounts (workspace_id, lower(name)) WHERE archived_at IS NULL;
DROP INDEX accounts_owner_idx;
ALTER TABLE accounts DROP CONSTRAINT accounts_owner_fkey, DROP COLUMN owner_user_id;
