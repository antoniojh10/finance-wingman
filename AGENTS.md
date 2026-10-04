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

## Workflow
- Start local services with `make up`; integration tests need Postgres running.
- Run `make api-lint api-test` before considering backend work done.
- Integration tests use `testutil.NewDatabase`, which creates an isolated database per test; never share state between tests.
- SQL lives in `apps/api/internal/db/queries`; run `make api-generate` after editing queries or migrations. Never edit `internal/store` by hand.
- Business rules live in `internal/finance` and are shared by REST and MCP; transport layers only translate input/output and errors.
- Amounts in the REST API are integer minor units; use `internal/money` to convert decimal input (e.g. from MCP tools).
- Every `/api/v1` operation requires a bearer session unless registered with `Metadata: publicMetadata`; keep the public surface minimal.
- Tests use `newTestAPI`, which signs requests as an owner user; use `api.as("")` for anonymous requests and `testutil.MailRecorder` to read sent emails.
- Schema changes go in a new goose migration (`make migrate-new name=...`) with a working `Down` section; never edit an applied migration.
