# Finance Wingman

Personal finance app organized in workspaces shared by invitation: multiple
accounts in different currencies, categorized expenses/income, transfers,
and an MCP server so transactions can be added from Claude or ChatGPT
mobile.

## Structure

```
apps/api   Go API (REST + MCP), PostgreSQL migrations
apps/web   Next.js frontend (see apps/web/README.md)
infra      Local infrastructure (Postgres init scripts)
```

## Requirements

- Go 1.27+
- Node.js 22+ and pnpm 10+
- Docker (with Compose)

## Getting started

```bash
cp .env.example .env
make up          # Postgres on :5432, Mailpit on :8025
make api-run     # API on :8080 (applies migrations on start)
make web-install && make web-dev   # Web on :3000
```

Grant access to the first users (no public sign-up), then sign in with the
link or code that arrives in Mailpit:

```bash
# in .env; on startup they own a first workspace (INITIAL_WORKSPACE_NAME)
INITIAL_USERS="you@example.com:You,partner@example.com:Partner"
```

Everyone else joins by invitation from a workspace's settings page. Users
added with `go run ./cmd/api users add` start without a workspace and are
asked to create one.

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
| `make web-test` / `make web-lint` | Web unit tests / lint + typecheck |
| `make web-e2e` | Playwright end-to-end tests (needs `make up`) |
| `make api-openapi` | Export the OpenAPI document and regenerate web API types |
| `make migrate-up` / `migrate-down` / `migrate-status` | Manage migrations |
| `make migrate-new name=add_x` | Create a new SQL migration |
| `make psql` | Open a psql shell |
| `make measure` | Measure RAM/CPU of the production images and estimate hosting cost ([docs](docs/measure.md)) |
| `go run ./cmd/api users list\|add\|remove` | Manage who has access (from `apps/api`) |

## Authentication

- `POST /api/v1/auth/login` emails a magic link and a 6-digit code (15 min, single use).
- `POST /api/v1/auth/verify` exchanges the link token or email + code for a bearer token.
- Every other `/api/v1` endpoint requires `Authorization: Bearer <token>`.
- The web app keeps the token server-side (Next.js acts as a backend-for-frontend).

## MCP (Claude / ChatGPT)

The API serves a remote MCP server at `PUBLIC_URL/mcp` (Streamable HTTP,
stateless) protected by OAuth 2.1:

- Discovery: `/.well-known/oauth-protected-resource` and `/.well-known/oauth-authorization-server`
- Dynamic client registration (`/oauth/register`), authorization code + PKCE (S256), rotating refresh tokens, revocation
- During authorization the user receives a 6-digit code by email and types it on the consent page, which works inside mobile in-app browsers

Tools: `add_expense`, `add_income`, `add_transfer`, `list_accounts`,
`list_categories`, `create_account`, `create_category`, `get_summary`,
`list_transactions`, `delete_transaction`.
Amounts are decimals in the account currency; summaries never mix currencies.
Accounts belong to a workspace member or are shared: a bare account name
means the caller's own, and another member's is written as `BNP (Ana)`.

To connect, add a custom connector with the URL `https://<your-api>/mcp`:

- **Claude**: Settings → Connectors → Add custom connector (available in the mobile app once added).
- **ChatGPT**: Settings → Apps & Connectors → Advanced → Developer mode → Create connector.

The MCP endpoint must be reachable over public HTTPS; for local testing use a
tunnel (e.g. `cloudflared tunnel --url http://localhost:8080`) and set
`PUBLIC_URL` to the tunnel URL.

## Deployment

Production runs on Railway, which deploys each app from `main` when its
folder changes. To deploy a service by hand (e.g. after a commit that
skipped it), run the **Deploy** workflow from the Actions tab and pick
`api`, `web` or `both`; it only deploys `main`. It needs the
`RAILWAY_TOKEN` repository secret: a Railway project token for the
`production` environment.

See [docs/deploy-seenode.md](docs/deploy-seenode.md) for deploying the API,
web app and PostgreSQL on Seenode (native runtimes or Docker images).

See [docs/observability.md](docs/observability.md) to send traces,
metrics and logs to an OpenTelemetry backend.

See [AGENTS.md](AGENTS.md) for contribution rules.
