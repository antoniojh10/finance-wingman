# Evidence mapping

Every factual sentence in a published page gets a row here. Keep the table
in the review notes (or the ticket), not in the page. A row without a
source means the sentence is deleted or rewritten until it is true.

| # | Claim (page, section) | Evidence (file:line, migration, doc, vendor URL + date) | Status |
|---|---|---|---|

Status: `shipped` (verified in code at a commit), `config` (depends on a
production setting the user confirmed), `vendor` (vendor page cited),
`user` (user supplied, cannot be checked), `dropped`.

## Where to look in this repo

| Claim topic | Look at |
|---|---|
| Workspace isolation (RLS) | `apps/api/internal/db/migrations` (policies, `FORCE ROW LEVEL SECURITY`), `db.WithWorkspace`, isolation tests |
| No passwords, magic link | `apps/api/internal/auth`, `apps/web/src/app/actions/auth.ts` |
| Token hashing and expiry | `internal/auth`, `internal/oauth`: what is stored (hash or plaintext) and the TTLs |
| Sessions, cookies | `apps/web/src/lib/session.ts`, `apps/web/src/i18n/request.ts`: names, flags (httpOnly, secure, sameSite), lifetime |
| OAuth / MCP scopes | `internal/oauth`, `internal/mcpserver` (read-only scope only if shipped) |
| Rate limits | API middleware and config (for example `LOGIN_EMAILS_PER_HOUR`); do not claim global limits unless present |
| Security headers | `apps/web/next.config.*`, API middleware; quote the real headers |
| Transport encryption | production checks in `internal/config` (https enforced); TLS terminates at the platform |
| Encryption at rest | a property of the database host's disks; claim only with a vendor source. Field-level encryption only if shipped |
| Backups | `docs/` and the hosting dashboard; frequency, retention, restore drill only if done |
| Telemetry content | `apps/api/internal/telemetry`, `apps/web/src/lib/telemetry*.ts`, `docs/observability.md`; list attribute names; confirm no emails, amounts, descriptions or tokens |
| Email content | `apps/api/internal/mail`: what is sent, to whom, through which provider |
| Data categories | migrations and `internal/db/queries` (columns), REST schemas |
| Export / deletion | endpoints and tests; what remains afterwards (logs, backups) |
| Region | `docs/deploy-railway.md` plus dashboard values from the user; never infer from a vendor default |
| Subprocessors | env vars and outbound clients in the code, `docs/` |

## Rules

- Quote the control, not the intention: "sign-in links expire after N
  minutes" needs the N from code.
- Absence claims ("we use no analytics cookies") need a search for cookies,
  scripts and third-party hosts in `apps/web` (layout, fonts, CSP).
- Time-bound facts (vendor regions, vendor retention) carry the date
  checked and are re-verified at each update.
- If code and a ticket disagree, code wins; flag the ticket.
- Re-run the table whenever the pages change or a related migration lands.
