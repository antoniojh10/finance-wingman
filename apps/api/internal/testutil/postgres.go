// Package testutil provides helpers shared by integration tests.
package testutil

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
)

const defaultTestDatabaseURL = "postgres://finance:finance@localhost:5432/finance_test?sslmode=disable"

var (
	templateOnce sync.Once
	templateName string
	templateErr  error
)

// NewDatabase creates an isolated database for a single test and drops it
// when the test finishes. With migrate=true the database is cloned from a
// cached, fully migrated template and the pool acts as db.AppRole (like the
// API, so row-level security applies); otherwise it is empty and the pool
// acts as the admin user.
//
// The admin connection comes from TEST_DATABASE_URL (defaults to the
// docker-compose database). Run `make up` before running integration tests.
func NewDatabase(t *testing.T, migrate bool) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	adminURL := adminURL()
	source := "template0"
	if migrate {
		templateOnce.Do(func() { templateName, templateErr = ensureTemplate(adminURL) })
		if templateErr != nil {
			t.Fatalf("prepare template database (is `make up` running?): %v", templateErr)
		}
		source = templateName
	}

	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect to test database (is `make up` running?): %v", err)
	}
	defer admin.Close(ctx)

	name := "test_" + randomSuffix(t)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()+
		" TEMPLATE "+pgx.Identifier{source}.Sanitize()+" STRATEGY FILE_COPY"); err != nil {
		t.Fatalf("create database %s: %v", name, err)
	}

	// A migrated database is used like the API uses it: as db.AppRole, under
	// row-level security. An empty one has no such role yet.
	connect := db.ConnectOwner
	if migrate {
		connect = db.Connect
	}
	pool, err := connect(ctx, withDatabase(adminURL, name))
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
	return pool
}

// ensureTemplate creates (once per migration set) a migrated database used
// as a template. An advisory lock serializes test binaries running in
// parallel.
func ensureTemplate(adminURL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fingerprint, err := db.MigrationsFingerprint()
	if err != nil {
		return "", err
	}
	name := "template_" + fingerprint

	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return "", err
	}
	defer admin.Close(ctx)

	if _, err := admin.Exec(ctx, "SELECT pg_advisory_lock(727274)"); err != nil {
		return "", err
	}
	defer admin.Exec(context.Background(), "SELECT pg_advisory_unlock(727274)") //nolint:errcheck

	var exists bool
	if err := admin.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", name).Scan(&exists); err != nil {
		return "", err
	}
	if exists {
		return name, nil
	}

	// Drop templates built from older migration sets.
	rows, err := admin.Query(ctx, "SELECT datname FROM pg_database WHERE datname LIKE 'template\\_%' AND datname <> $1", name)
	if err != nil {
		return "", err
	}
	stale, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return "", err
	}
	for _, old := range stale {
		if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{old}.Sanitize()+" WITH (FORCE)"); err != nil {
			return "", err
		}
	}

	building := name + "_building"
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{building}.Sanitize()+" WITH (FORCE)"); err != nil {
		return "", err
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{building}.Sanitize()+" TEMPLATE template0"); err != nil {
		return "", err
	}
	pool, err := db.ConnectOwner(ctx, withDatabase(adminURL, building))
	if err != nil {
		return "", err
	}
	_, err = db.MigrateUp(ctx, pool)
	pool.Close()
	if err != nil {
		return "", err
	}
	// Rename only after a successful migration so a crash never leaves a
	// half-migrated template behind.
	if _, err := admin.Exec(ctx, "ALTER DATABASE "+pgx.Identifier{building}.Sanitize()+" RENAME TO "+pgx.Identifier{name}.Sanitize()); err != nil {
		return "", err
	}
	return name, nil
}

func adminURL() string {
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		return u
	}
	return defaultTestDatabaseURL
}

func randomSuffix(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("random suffix: %v", err)
	}
	return hex.EncodeToString(b)
}

func withDatabase(rawURL, name string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic("invalid TEST_DATABASE_URL: " + err.Error())
	}
	u.Path = "/" + name
	return u.String()
}
