package db_test

import (
	"context"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
)

// Rolling back OAuth scopes disconnects read-only connections, since the
// previous version would refresh them into full access, and keeps the
// others.
func TestOAuthScopesDown(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testutil.NewDatabase(t, false)
	m, err := db.NewMigrator(pool)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.UpTo(ctx, 20261008140330); err != nil {
		t.Fatal(err)
	}

	var userID string
	mustScan(t, ctx, pool, &userID, `INSERT INTO users (email) VALUES ('ana@example.com') RETURNING id`)
	const readOnly, full = "11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222"
	if _, err := pool.Exec(ctx, `INSERT INTO oauth_clients (id, name, redirect_uris, token_endpoint_auth_method)
		VALUES ('fw_claude', 'Claude', '{https://claude.ai/cb}', 'none')`); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO oauth_grants (id, client_id, user_id, scope) VALUES
			($1, 'fw_claude', $3, 'finance:read'),
			($2, 'fw_claude', $3, 'finance:read finance:write')`,
		`INSERT INTO oauth_refresh_tokens (token_hash, family_id, client_id, user_id, scope, expires_at) VALUES
			('r', $1, 'fw_claude', $3, 'finance:read', now() + interval '1 day'),
			('f', $2, 'fw_claude', $3, 'finance:read finance:write', now() + interval '1 day')`,
		`INSERT INTO sessions (user_id, token_hash, client, expires_at, oauth_client_id, oauth_family_id, scope) VALUES
			($3, 'sr', 'mcp', now() + interval '1 hour', 'fw_claude', $1, 'finance:read'),
			($3, 'sf', 'mcp', now() + interval '1 hour', 'fw_claude', $2, 'finance:read finance:write'),
			($3, 'sw', 'web', now() + interval '1 hour', NULL, NULL, '')`,
	} {
		if _, err := pool.Exec(ctx, stmt, readOnly, full, userID); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}

	if err := m.DownTo(ctx, 20261008125641); err != nil {
		t.Fatal(err)
	}
	var readOnlyRevoked, fullActive bool
	var sessions int
	mustScan(t, ctx, pool, &readOnlyRevoked, `SELECT g.revoked_at IS NOT NULL AND t.revoked_at IS NOT NULL
		FROM oauth_grants g JOIN oauth_refresh_tokens t ON t.family_id = g.id WHERE g.id = $1`, readOnly)
	mustScan(t, ctx, pool, &fullActive, `SELECT g.revoked_at IS NULL AND t.revoked_at IS NULL
		FROM oauth_grants g JOIN oauth_refresh_tokens t ON t.family_id = g.id WHERE g.id = $1`, full)
	mustScan(t, ctx, pool, &sessions, `SELECT count(*) FROM sessions`)
	if !readOnlyRevoked || !fullActive || sessions != 2 {
		t.Fatalf("unexpected rollback: readOnlyRevoked=%v fullActive=%v sessions=%d (the read-only one should be deleted)", readOnlyRevoked, fullActive, sessions)
	}

	// Up again works on the rolled back data.
	if _, err := m.Up(ctx); err != nil {
		t.Fatal(err)
	}
}
