# Agent Rules

These rules apply to every agent (and human) contributing to this repository.

## Language
- All code must be written in English: identifiers, comments, commit messages, docs inside the code, test names, and API fields.
- User-facing UI copy may be localized; keep it out of identifiers.

## Commits
Commit messages follow [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/):

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

- **Types:** `feat` (new feature), `fix` (bug fix), `refactor` (no behavior change), `perf`, `test`, `docs`, `style` (formatting only), `build` (dependencies, Docker, tooling), `ci`, `chore` (anything else that doesn't touch app behavior), `revert`.
- **Scopes** (optional, use the area changed): `api`, `web`, `mcp`, `oauth`, `auth`, `db`, `i18n`, `infra`, `deps`. Omit the scope when a change spans several areas.
- **Description:** imperative mood, lowercase, no trailing period, at most 72 characters for the whole header (e.g. `feat(mcp): add update_transaction tool`).
- **Body:** explain *why* when it isn't obvious; wrap at 72 characters.
- **Breaking changes:** add `!` after the type/scope (`feat(api)!: rename amount to amount_minor`) and a `BREAKING CHANGE: <details>` footer. Use this for incompatible REST/MCP contract or schema changes.
- One logical change per commit; don't mix unrelated `feat` and `fix` work.
- Keep tests in the same commit as the change they cover (use `test` only for test-only changes).

## Testing
- Every backend endpoint (REST and MCP tools) must have tests covering success and main error paths.
- The frontend must have tests for components, pages, and data-fetching logic.
- A change is not done until its tests are written and passing.

## Stack
- Backend: Go (`apps/api`) — REST API + MCP server (Streamable HTTP) in a single binary.
- Frontend: Next.js (`apps/web`).
- Database: PostgreSQL (Docker for local development).
- Monorepo: single repository, each app deployed as its own service.

## Domain
- Data lives in workspaces. A user can belong to several and switch between them; a session (and each MCP connection) acts on one.
- Workspace roles: owners manage the workspace, members and invitations; members manage its data. A workspace always keeps an owner.
- Authentication: passwordless magic link by email; no public sign-up (users come from `INITIAL_USERS` or accepted invitations).
- MCP clients (Claude / ChatGPT mobile) authenticate via OAuth, reusing the magic-link login.
- Accounts each have one ISO 4217 currency. Money is stored as int64 minor units.
- Summaries are grouped per currency; no currency conversion.
- Transaction types: expense, income, and transfer between accounts (transfers are not income/expense).
- Transactions can have a category.
- UI is internationalized (English and Spanish).

## Frontend
- Read `apps/web/AGENTS.md` first: Next.js 16 differs from older versions; check `node_modules/next/dist/docs/` before using an API.
- The browser never calls the Go API directly. Pages fetch in Server Components; mutations are Server Actions in `src/app/actions` returning `FormState`.
- Use `useFormAction` (onSubmit + transition) for forms with Server Actions; passing actions to `<form action>` resets the form after submission.
- Keep async Server Components thin; put UI in synchronous components so it can be unit tested. Cover full flows with Playwright (`e2e/`).
- All UI copy lives in `messages/en.json` and `messages/es.json` (keys must match; a test enforces it).
- After changing API endpoints or schemas run `make api-openapi`; never edit `src/lib/api/schema.d.ts` by hand.
- UI primitives are shadcn/ui on Base UI: use `render` (not `asChild`) and `nativeButton={false}` when a Button renders a link.

## Workflow
- Start local services with `make up`; integration tests need Postgres running.
- Run `make api-lint api-test` before considering backend work done, and `make web-lint web-test` (plus `make web-e2e` for flow changes) for frontend work.
- Integration tests use `testutil.NewDatabase`, which creates an isolated database per test; never share state between tests.
- SQL lives in `apps/api/internal/db/queries`; run `make api-generate` after editing queries or migrations. Never edit `internal/store` by hand.
- Business rules live in `internal/finance` and are shared by REST and MCP; transport layers only translate input/output and errors. Workspaces, members and invitations live in `internal/workspace`.
- Finance tables are isolated per workspace with row-level security: the API runs as `wingman_app` and every query acts on the workspace in its context (`db.WithWorkspace`). A new finance table needs `workspace_id` (defaulting to `current_workspace_id()`), composite foreign keys to other finance tables, and a `workspace_isolation` policy with `FORCE ROW LEVEL SECURITY`. Tests calling services directly need a workspace context (`testutil.NewWorkspace`, `api.ctx`).
- Amounts in the REST API are integer minor units; use `internal/money` to convert decimal input (e.g. from MCP tools).
- Every `/api/v1` operation requires a bearer session unless registered with `Metadata: publicMetadata`; keep the public surface minimal.
- Tests use `newTestAPI`, which signs requests as an owner user; use `api.as("")` for anonymous requests and `testutil.MailRecorder` to read sent emails.
- MCP tools live in `internal/mcpserver`; each tool must have tests (in-memory client in `tools_test.go`). Tool errors should tell the model how to fix the call (e.g. list valid account names).
- OAuth lives in `internal/oauth`; flow tests in `internal/httpapi/oauth_test.go` simulate an MCP host end to end.
- Schema changes go in a new goose migration (`make migrate-new name=...`) with a working `Down` section; never edit an applied migration.
