-- +goose Up
-- A user can schedule the deletion of their account. They can still sign in
-- and cancel during a grace period; once deletion_scheduled_for passes, the
-- API removes them from their workspaces (deleting the ones where they are
-- the only member) and deletes the user, whose sessions, login challenges
-- and OAuth grants and tokens go with it (ON DELETE CASCADE).
ALTER TABLE users ADD COLUMN deletion_scheduled_for timestamptz;

CREATE INDEX users_deletion_scheduled_for_idx ON users (deletion_scheduled_for)
    WHERE deletion_scheduled_for IS NOT NULL;

-- +goose Down
DROP INDEX users_deletion_scheduled_for_idx;
ALTER TABLE users DROP COLUMN deletion_scheduled_for;
