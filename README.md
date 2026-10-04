# Finance Wingman

Personal finance app for a shared workspace: multiple accounts in different
currencies, categorized expenses/income, transfers, and an MCP server so
transactions can be added from Claude or ChatGPT mobile.

## Structure

```
apps/api   Go API (REST + MCP), PostgreSQL migrations
apps/web   Next.js frontend (coming soon)
infra      Local infrastructure (Postgres init scripts)
```

## Requirements

- Go 1.27+
- Docker (with Compose)

## Getting started

```bash
cp .env.example .env
make up          # Postgres on :5432, Mailpit on :8025
make api-run     # API on :8080 (applies migrations on start)
```

Grant access to a user (no public sign-up), then sign in with the link or
code that arrives in Mailpit:

```bash
cd apps/api && go run ./cmd/api users add you@example.com "Your Name"
# or set INITIAL_USERS="you@example.com:You,partner@example.com:Partner"
```

- Health check: http://localhost:8080/healthz
- API docs: http://localhost:8080/docs
- OpenAPI spec: http://localhost:8080/openapi.json
- Mailpit inbox: http://localhost:8025

## Common tasks

| Command | Description |
| --- | --- |
| `make api-test` | Run all API tests (requires `make up`) |
| `make api-generate` | Regenerate sqlc code after editing SQL |
| `make api-lint` | `go vet` + `gofmt` check |
| `make migrate-up` / `migrate-down` / `migrate-status` | Manage migrations |
| `make migrate-new name=add_x` | Create a new SQL migration |
| `make psql` | Open a psql shell |
| `go run ./cmd/api users list\|add\|remove` | Manage who has access (from `apps/api`) |

## Authentication

- `POST /api/v1/auth/login` emails a magic link and a 6-digit code (15 min, single use).
- `POST /api/v1/auth/verify` exchanges the link token or email + code for a bearer token.
- Every other `/api/v1` endpoint requires `Authorization: Bearer <token>`.
- The web app keeps the token server-side (Next.js acts as a backend-for-frontend).

See [AGENTS.md](AGENTS.md) for contribution rules.
