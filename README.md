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

- Health check: http://localhost:8080/healthz
- API docs: http://localhost:8080/docs
- OpenAPI spec: http://localhost:8080/openapi.json
- Mailpit inbox: http://localhost:8025

## Common tasks

| Command | Description |
| --- | --- |
| `make api-test` | Run all API tests (requires `make up`) |
| `make api-lint` | `go vet` + `gofmt` check |
| `make migrate-up` / `migrate-down` / `migrate-status` | Manage migrations |
| `make migrate-new name=add_x` | Create a new SQL migration |
| `make psql` | Open a psql shell |

See [AGENTS.md](AGENTS.md) for contribution rules.
