-- +goose Up
-- An owner can schedule a workspace for deletion. It stays usable during a
-- grace period, while any owner can cancel; once deletion_scheduled_for
-- passes, the API deletes the workspace and every row that belongs to it
-- (finance data, invitations, connected apps) through ON DELETE CASCADE.
ALTER TABLE workspaces
    ADD COLUMN deletion_scheduled_for timestamptz,
    ADD COLUMN deletion_requested_by uuid REFERENCES users (id) ON DELETE SET NULL;

CREATE INDEX workspaces_deletion_scheduled_for_idx ON workspaces (deletion_scheduled_for)
    WHERE deletion_scheduled_for IS NOT NULL;

-- +goose Down
DROP INDEX workspaces_deletion_scheduled_for_idx;
ALTER TABLE workspaces DROP COLUMN deletion_requested_by, DROP COLUMN deletion_scheduled_for;
