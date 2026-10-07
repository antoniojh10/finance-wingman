package db

import "testing"

func TestQuerySpanName(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"-- name: ListAccounts :many\nSELECT * FROM accounts":      "ListAccounts",
		"  -- name: DeleteAccount :execrows\nDELETE FROM accounts": "DeleteAccount",
		"select 1": "SELECT",
		"-- goose comment\n\n  insert into t values (1)": "INSERT",
		"":                  "query",
		"-- only a comment": "query",
	}
	for stmt, want := range cases {
		if got := QuerySpanName(stmt); got != want {
			t.Errorf("QuerySpanName(%q) = %q, want %q", stmt, got, want)
		}
	}
}
