-- +goose Up
-- Append-only log of who changed what in a workspace. It stores metadata
-- only: ids, the kind of change and the names of the fields it touched,
-- never amounts, descriptions, names, emails, IPs or user agents, so it
-- reveals no more than the records it mentions and survives their deletion.
-- actor_id is cleared when the user is erased; the OAuth client is kept as a
-- snapshot (id and name) because clients can be removed independently.
-- created_at uses clock_timestamp() so entries written by one database
-- transaction (a batch) keep their order.
CREATE TABLE activity_log (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL DEFAULT current_workspace_id() REFERENCES workspaces (id) ON DELETE CASCADE,
    actor_id     uuid REFERENCES users (id) ON DELETE SET NULL,
    channel      text NOT NULL CHECK (channel IN ('web', 'mcp')),
    client_id    text,
    client_name  text,
    action       text NOT NULL,
    entity_type  text NOT NULL,
    entity_id    uuid NOT NULL,
    details      jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(details) = 'object'),
    created_at   timestamptz NOT NULL DEFAULT clock_timestamp()
);

-- Newest-first listing with keyset pagination, optionally by entity or actor.
CREATE INDEX activity_log_workspace_created_idx ON activity_log (workspace_id, created_at DESC, id DESC);
CREATE INDEX activity_log_entity_idx ON activity_log (workspace_id, entity_type, entity_id);
CREATE INDEX activity_log_actor_idx ON activity_log (actor_id);

ALTER TABLE activity_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE activity_log FORCE ROW LEVEL SECURITY;
CREATE POLICY workspace_isolation ON activity_log
    USING (workspace_id = current_workspace_id()) WITH CHECK (workspace_id = current_workspace_id());

-- Append-only for the API: it can add and read entries, never change or
-- remove them. Erasing a user or a workspace still clears the column or the
-- rows through the foreign keys, which run with the table owner's rights.
-- A future retention job runs as the owner too.
REVOKE UPDATE, DELETE ON activity_log FROM wingman_app;

-- +goose Down
DROP TABLE activity_log;
