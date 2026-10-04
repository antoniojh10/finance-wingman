// Package testutil provides helpers shared by integration tests.
package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
)

const defaultTestDatabaseURL = "postgres://finance:finance@localhost:5432/finance_test?sslmode=disable"

// NewDatabase creates an isolated, empty database for a single test and drops
// it when the test finishes. Pass migrate=true to apply the schema.
//
// The admin connection comes from TEST_DATABASE_URL (defaults to the
// docker-compose database). Run `make up` before running integration tests.
func NewDatabase(t *testing.T, migrate bool) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		adminURL = defaultTestDatabaseURL
	}

	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect to test database (is `make up` running?): %v", err)
	}
	defer admin.Close(ctx)

	name := "test_" + randomSuffix(t)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatalf("create database %s: %v", name, err)
	}

	pool, err := db.Connect(ctx, withDatabase(t, adminURL, name))
	if err != nil {
		t.Fatalf("connect to %s: %v", name, err)
	}

	t.Cleanup(func() {
		pool.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		admin, err := pgx.Connect(ctx, adminURL)
		if err != nil {
			t.Errorf("connect to drop %s: %v", name, err)
			return
		}
		defer admin.Close(ctx)
		if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Errorf("drop database %s: %v", name, err)
		}
	})

	if migrate {
		if _, err := db.MigrateUp(ctx, pool); err != nil {
			t.Fatalf("migrate %s: %v", name, err)
		}
	}
	return pool
}

func randomSuffix(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("random suffix: %v", err)
	}
	return hex.EncodeToString(b)
}

func withDatabase(t *testing.T, rawURL, name string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	u.Path = "/" + name
	return u.String()
}
