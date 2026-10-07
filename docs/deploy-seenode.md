# Deploying to Seenode

The repository is a monorepo with two services plus a database:

| Seenode resource | Root directory | Port |
| --- | --- | --- |
| PostgreSQL (managed database) | — | — |
| Web service `finance-api` (Go) | `apps/api` | 8080 |
| Web service `finance-web` (Next.js) | `apps/web` | 3000 |

Both services can use Seenode's native runtimes (below) or the provided
Dockerfiles (see [Docker alternative](#docker-alternative)).

## 1. Database

Create a managed PostgreSQL database and copy its connection string. Use it
as `DATABASE_URL` for the API (append `?sslmode=require` if the connection
string does not specify an SSL mode and the database requires TLS).
Migrations run automatically when the API starts.

The database must be PostgreSQL 15 or newer. Migrations run as the
`DATABASE_URL` user, which must be able to create roles: they create
`wingman_app`, the role the API switches to after connecting so that
row-level security isolates workspaces even when the connection user is a
superuser.

## 2. API service (`apps/api`)

- **Runtime:** Go (any available version; the build downloads the Go
  toolchain required by `go.mod`)
- **Root directory:** `apps/api`
- **Build command:** `GOTOOLCHAIN=go1.27.1+auto CGO_ENABLED=0 go build -trimpath -o app ./cmd/api`
- **Start command:** `./app serve`
- **Port:** `8080`
- **Health check path:** `/healthz`

Environment variables:

| Variable | Example | Notes |
| --- | --- | --- |
| `APP_ENV` | `production` | Enables production checks (https URLs, real email provider) |
| `PORT` | `8080` | Must match the port configured in Seenode |
| `DATABASE_URL` | `postgres://…` | From the managed database |
| `PUBLIC_URL` | `https://api.example.com` | Public HTTPS origin of the API; the MCP endpoint is `PUBLIC_URL/mcp` |
| `WEB_BASE_URL` | `https://app.example.com` | Public URL of the web app (magic links point here) |
| `APP_TIMEZONE` | `America/Mexico_City` | Used for "today" and monthly summaries |
| `INITIAL_USERS` | `ana@example.com:Ana,bob@example.com:Bob` | Users granted access on startup (no public sign-up); they own the first workspace |
| `INITIAL_WORKSPACE_NAME` | `Home` | Optional; name of the first workspace, created when the database has none (default `Finance Wingman`) |
| `EMAIL_PROVIDER` | `resend` | `resend` or `smtp` (`log` is rejected in production) |
| `EMAIL_FROM` | `no-reply@example.com` | Must belong to a domain verified in Resend |
| `EMAIL_FROM_NAME` | `Finance Wingman` | Optional |
| `RESEND_API_KEY` | `re_…` | Required with `EMAIL_PROVIDER=resend` |
| `LOGIN_EMAILS_PER_HOUR` | `5` | Optional; sign-in emails allowed per user per hour |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `https://otlp-gateway-….grafana.net/otlp` | Optional; enables traces, metrics and logs ([observability](observability.md)) |
| `OTEL_EXPORTER_OTLP_HEADERS` | `Authorization=Basic%20…` | Optional; credentials for the OTLP endpoint |
| `OTEL_RESOURCE_ATTRIBUTES` | `deployment.environment.name=production` | Optional; separates production from development data |

## 3. Web service (`apps/web`)

- **Runtime:** Node.js 22
- **Root directory:** `apps/web`
- **Build command:** `corepack pnpm install --frozen-lockfile && corepack pnpm build && cp -r public .next/standalone/ && cp -r .next/static .next/standalone/.next/`
- **Start command:** `HOSTNAME=0.0.0.0 PORT=3000 node .next/standalone/server.js`
- **Port:** `3000`

Environment variables:

| Variable | Example | Notes |
| --- | --- | --- |
| `API_URL` | `http://finance-api-service:8080` | Internal address of the API on Seenode's private network |
| `API_PUBLIC_URL` | `https://api.example.com` | Shown in Settings as the MCP connector URL |
| `APP_TIMEZONE` | `America/Mexico_City` | Same value as the API |
| `DEFAULT_CURRENCY` | `MXN` | Preselected when creating the first account |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | same as the API | Optional; enables traces ([observability](observability.md)) |
| `OTEL_EXPORTER_OTLP_HEADERS` | same as the API | Optional |
| `OTEL_RESOURCE_ATTRIBUTES` | same as the API | Optional |

`NODE_ENV=production` is set by `next build`/the standalone server, which
makes the session cookie `Secure`.

## 4. Domains and email

1. Attach custom domains (or use the Seenode-provided URLs) to both services
   and set `PUBLIC_URL`, `WEB_BASE_URL` and `API_PUBLIC_URL` accordingly. Both
   must be HTTPS.
2. In Resend, verify the sending domain and create an API key.
3. Redeploy the API after changing environment variables.

## 5. Verify

- `https://<api>/healthz` returns `{"status":"ok","database":"ok"}`.
- `https://<api>/.well-known/oauth-protected-resource/mcp` lists your public API URL.
- Sign in at `https://<web>` with an address from `INITIAL_USERS`.
- Add `https://<api>/mcp` as a custom connector in Claude or ChatGPT
  (Settings in the web app shows the exact URL and steps).

Manage users later from the API service shell: `./app users list|add|remove`.

## Docker alternative

Both apps include a production `Dockerfile` (`apps/api/Dockerfile`,
`apps/web/Dockerfile`). Build and push the images to a registry and create
Seenode services from those images with the same ports and environment
variables. The API image runs `serve` by default; the web image runs the
Next.js standalone server on port 3000.

```bash
docker build -t <registry>/finance-wingman-api ./apps/api
docker build -t <registry>/finance-wingman-web ./apps/web
```
