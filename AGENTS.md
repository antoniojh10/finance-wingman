# Agent Rules

These rules apply to every agent (and human) contributing to this repository.

## Language
- All code must be written in English: identifiers, comments, commit messages, docs inside the code, test names, and API fields.
- User-facing UI copy may be localized; keep it out of identifiers.

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
- Single shared workspace, accessible by two users with equal permissions.
- Authentication: passwordless magic link by email; no public sign-up (users are allowlisted).
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
- Business rules live in `internal/finance` and are shared by REST and MCP; transport layers only translate input/output and errors.
- Amounts in the REST API are integer minor units; use `internal/money` to convert decimal input (e.g. from MCP tools).
- Every `/api/v1` operation requires a bearer session unless registered with `Metadata: publicMetadata`; keep the public surface minimal.
- Tests use `newTestAPI`, which signs requests as an owner user; use `api.as("")` for anonymous requests and `testutil.MailRecorder` to read sent emails.
- MCP tools live in `internal/mcpserver`; each tool must have tests (in-memory client in `tools_test.go`). Tool errors should tell the model how to fix the call (e.g. list valid account names).
- OAuth lives in `internal/oauth`; flow tests in `internal/httpapi/oauth_test.go` simulate an MCP host end to end.
- Schema changes go in a new goose migration (`make migrate-new name=...`) with a working `Down` section; never edit an applied migration.
