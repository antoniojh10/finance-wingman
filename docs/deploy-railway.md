# Deploying to Railway

The repository is a monorepo that deploys as three Railway services in one
project:

| Service | Source | Root directory | Port | Health check |
| --- | --- | --- | --- | --- |
| `postgres` | Railway PostgreSQL template | — | — | — |
| `api` (Go) | this repository, branch `main` | `/apps/api` | 8080 | `/healthz` |
| `web` (Next.js) | this repository, branch `main` | `/apps/web` | 3000 | `/login` |

Each app has a production `Dockerfile` (`apps/api/Dockerfile`,
`apps/web/Dockerfile`) in its root directory, which Railway builds. The API
image runs `serve` by default; the web image runs the Next.js standalone
server. Railway sets `PORT` for each service; both apps listen on it.

The Deploy workflow and the CLI commands below refer to the services by
name, so name them `api` and `web`, and keep the environment named
`production`.

## 1. Database

Add the PostgreSQL template to the project. Use PostgreSQL 15 or newer.

Migrations run when the API starts (`MIGRATE_ON_START` defaults to `true`).
They run as the `DATABASE_URL` user, which must be able to create roles: they
create `wingman_app`, the role the API switches to after connecting so that
row-level security isolates workspaces even when the connection user is a
superuser. Set `MIGRATE_ON_START=false` and run `./api migrate up` yourself
if you prefer to migrate separately.

## 2. API service

Create a service from the GitHub repository and configure it:

- **Root directory:** `/apps/api`
- **Branch:** `main`
- **Watch paths:** `/apps/api/**`, so only changes to the API redeploy it
- **Healthcheck path:** `/healthz`

Variables:

| Variable | Example | Notes |
| --- | --- | --- |
| `APP_ENV` | `production` | Enables production checks (https URLs, real email provider) |
| `DATABASE_URL` | `${{postgres.DATABASE_URL}}` | Required. A Railway reference to the Postgres service (use the service's name) |
| `PUBLIC_URL` | `https://api.example.com` | Public HTTPS origin of the API, without a path; the MCP endpoint is `PUBLIC_URL/mcp` |
| `WEB_BASE_URL` | `https://app.example.com` | Public HTTPS URL of the web app (magic links point here) |
| `APP_TIMEZONE` | `America/Mexico_City` | IANA time zone used for "today" and monthly summaries (default UTC) |
| `INITIAL_USERS` | `ana@example.com:Ana,bob@example.com:Bob` | Users granted access on startup (no public sign-up); they own the first workspace |
| `INITIAL_WORKSPACE_NAME` | `Home` | Optional; name of the first workspace (default `Finance Wingman`) |
| `EMAIL_PROVIDER` | `resend` | `resend` or `smtp` (`log` is rejected in production) |
| `EMAIL_FROM` | `no-reply@example.com` | Must belong to a domain verified with the provider |
| `EMAIL_FROM_NAME` | `Finance Wingman` | Optional |
| `RESEND_API_KEY` | `re_...` | Required with `EMAIL_PROVIDER=resend` |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD` | | Used with `EMAIL_PROVIDER=smtp` |
| `LOGIN_EMAILS_PER_HOUR` | `5` | Optional; sign-in emails allowed per user per hour |
| `MIGRATE_ON_START` | `true` | Optional; see [Database](#1-database) |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `https://otlp.example.com/otlp` | Optional; enables traces, metrics and logs ([observability](observability.md)) |
| `OTEL_EXPORTER_OTLP_HEADERS` | `Authorization=Basic%20...` | Optional; credentials for the OTLP endpoint |
| `OTEL_RESOURCE_ATTRIBUTES` | `deployment.environment.name=production` | Optional; separates production from development data |

## 3. Web service

- **Root directory:** `/apps/web`
- **Branch:** `main`
- **Watch paths:** `/apps/web/**`
- **Healthcheck path:** `/login`

Variables:

| Variable | Example | Notes |
| --- | --- | --- |
| `API_URL` | `http://api.railway.internal:8080` | Address the web server uses to reach the API; the private network address of the `api` service avoids public egress |
| `API_PUBLIC_URL` | `https://api.example.com` | Shown in Settings as the MCP connector URL |
| `APP_TIMEZONE` | `America/Mexico_City` | Same value as the API |
| `DEFAULT_CURRENCY` | `MXN` | Preselected when creating the first account |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | same as the API | Optional; enables traces ([observability](observability.md)) |
| `OTEL_EXPORTER_OTLP_HEADERS` | same as the API | Optional |
| `OTEL_RESOURCE_ATTRIBUTES` | same as the API | Optional |

The web image sets `NODE_ENV=production`, which makes the session cookie
`Secure`.

## 4. Domains and email

1. Add a custom domain to each of the `api` and `web` services and create the
   DNS records Railway shows (a CNAME to the Railway target, plus a TXT
   verification record where requested). Set `PUBLIC_URL`, `WEB_BASE_URL`
   and `API_PUBLIC_URL` to the resulting HTTPS origins; the production
   checks reject http URLs.
2. In Resend, verify the sending domain (the DNS records Resend shows) and
   create an API key for `RESEND_API_KEY`.
3. Redeploy the API after changing variables.

## 5. Verify

- `https://<api>/healthz` returns `{"status":"ok","database":"ok"}`.
- `https://<api>/.well-known/oauth-protected-resource/mcp` lists your public
  API URL.
- Sign in at `https://<web>` with an address from `INITIAL_USERS`.
- Add `https://<api>/mcp` as a custom connector in Claude or ChatGPT
  (Settings in the web app shows the exact URL and steps).

Manage users later with the API binary from a shell on the service:
`/api users list|add|remove`.

## Deploys

Railway deploys each service from `main` when a file under its watch paths
changes. The **Deploy** workflow (`.github/workflows/deploy.yml`) deploys by
hand for cases the automatic deploy skips, such as a commit whose changes did
not match the watch paths:

1. In the Railway project, create a project token for the `production`
   environment.
2. Add it to the GitHub repository as the `RAILWAY_TOKEN` Actions secret.
3. Run **Deploy** from the Actions tab and pick `api`, `web` or `both`.

The workflow only runs on `main`, installs the Railway CLI and runs
`railway up --service <service> --environment production` from the
repository root; each service builds from the root directory set in Railway.

## Other hosts

The same Dockerfiles work on any container host: build both images
(`docker build ./apps/api`, `docker build ./apps/web`), run them with the
variables above and a PostgreSQL database, and expose ports 8080 and 3000.
