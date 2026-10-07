// Package db manages the PostgreSQL connection pool and schema migrations.
package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/exaring/otelpgx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AppRole is the role the API acts as. The schema migration creates it;
// row-level security applies to it even when the connection user is a
// superuser.
const AppRole = "wingman_app"

// Connect opens the API's connection pool and verifies the database is
// reachable. Every connection switches to AppRole, and every acquire sets
// app.workspace_id from the context (see WithWorkspace), so finance tables
// only expose the current workspace's rows. The schema must already be
// migrated: use ConnectOwner to run migrations.
//
// Queries are traced and pool statistics recorded with the global
// OpenTelemetry providers, which are no-ops unless telemetry is set up.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return connect(ctx, databaseURL, func(cfg *pgxpool.Config) {
		cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
			if _, err := conn.Exec(ctx, "SET ROLE "+pgx.Identifier{AppRole}.Sanitize()); err != nil {
				return fmt.Errorf("switch to role %s (are migrations applied?): %w", AppRole, err)
			}
			return nil
		}
		cfg.PrepareConn = func(ctx context.Context, conn *pgx.Conn) (bool, error) {
			// Always overwrite the setting so a pooled connection never keeps
			// the workspace of a previous request.
			workspace := ""
			if id, ok := WorkspaceFrom(ctx); ok {
				workspace = id.String()
			}
			if _, err := conn.Exec(ctx, "SELECT set_config('app.workspace_id', $1, false)", workspace); err != nil {
				return false, fmt.Errorf("set workspace: %w", err)
			}
			return true, nil
		}
	})
}

// ConnectOwner opens a pool as the connection user itself, without switching
// to AppRole. It is meant for schema migrations.
func ConnectOwner(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return connect(ctx, databaseURL, func(*pgxpool.Config) {})
}

func connect(ctx context.Context, databaseURL string, configure func(*pgxpool.Config)) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.ConnConfig.Tracer = otelpgx.NewTracer(
		otelpgx.WithSpanNameFunc(QuerySpanName),
		otelpgx.WithDisableConnectionDetailsInAttributes(),
	)
	configure(cfg)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	if err := otelpgx.RecordStats(pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("record pool stats: %w", err)
	}
	return pool, nil
}

type workspaceKey struct{}

// WithWorkspace makes queries run with ctx act on the given workspace.
func WithWorkspace(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, workspaceKey{}, id)
}

// WorkspaceFrom returns the workspace set with WithWorkspace, if any.
func WorkspaceFrom(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(workspaceKey{}).(uuid.UUID)
	return id, ok && id != uuid.Nil
}

// QuerySpanName names a query span after its sqlc query ("-- name: ListAccounts
// :many" becomes "ListAccounts"), falling back to the SQL operation keyword
// ("SELECT") for queries written by hand.
func QuerySpanName(stmt string) string {
	stmt = strings.TrimSpace(stmt)
	if rest, ok := strings.CutPrefix(stmt, "-- name:"); ok {
		if fields := strings.Fields(rest); len(fields) > 0 {
			return fields[0]
		}
	}
	for _, line := range strings.Split(stmt, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		if fields := strings.Fields(line); len(fields) > 0 {
			return strings.ToUpper(fields[0])
		}
	}
	return "query"
}
