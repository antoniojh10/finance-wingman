package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrator applies the embedded schema migrations.
type Migrator struct {
	sqlDB    *sql.DB
	provider *goose.Provider
}

// NewMigrator builds a migrator for the database behind the pool. Close must
// be called when done.
func NewMigrator(pool *pgxpool.Pool) (*Migrator, error) {
	migrations, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("load migrations: %w", err)
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations)
	if err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("create migration provider: %w", err)
	}
	return &Migrator{sqlDB: sqlDB, provider: provider}, nil
}

// Up applies all pending migrations and returns how many were applied.
func (m *Migrator) Up(ctx context.Context) (int, error) {
	results, err := m.provider.Up(ctx)
	if err != nil {
		return len(results), fmt.Errorf("migrate up: %w", err)
	}
	return len(results), nil
}

// Down rolls back the most recently applied migration.
func (m *Migrator) Down(ctx context.Context) error {
	if _, err := m.provider.Down(ctx); err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

// Reset rolls back every applied migration.
func (m *Migrator) Reset(ctx context.Context) error {
	if _, err := m.provider.DownTo(ctx, 0); err != nil {
		return fmt.Errorf("migrate reset: %w", err)
	}
	return nil
}

// Status reports the state of every known migration.
func (m *Migrator) Status(ctx context.Context) ([]*goose.MigrationStatus, error) {
	status, err := m.provider.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("migration status: %w", err)
	}
	return status, nil
}

func (m *Migrator) Close() error {
	return m.sqlDB.Close()
}

// MigrateUp is a convenience wrapper that applies all pending migrations.
func MigrateUp(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	m, err := NewMigrator(pool)
	if err != nil {
		return 0, err
	}
	defer m.Close()
	return m.Up(ctx)
}
