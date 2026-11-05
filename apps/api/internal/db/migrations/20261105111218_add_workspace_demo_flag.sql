-- +goose Up
-- Workspaces created by `api demo seed`. Only those can be hard-deleted by
-- the seed's --reset; real workspaces never are.
ALTER TABLE workspaces ADD COLUMN is_demo boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE workspaces DROP COLUMN is_demo;
