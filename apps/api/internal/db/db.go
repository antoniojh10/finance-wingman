// Package db manages the PostgreSQL connection pool and schema migrations.
package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a connection pool and verifies the database is reachable.
// Queries are traced and pool statistics recorded with the global
// OpenTelemetry providers, which are no-ops unless telemetry is set up.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.ConnConfig.Tracer = otelpgx.NewTracer(
		otelpgx.WithSpanNameFunc(QuerySpanName),
		otelpgx.WithDisableConnectionDetailsInAttributes(),
	)
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
