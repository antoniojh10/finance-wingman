# Finance Wingman — Web

Next.js 16 (App Router) frontend. It acts as a backend-for-frontend: the
browser only talks to Next.js, which keeps the API bearer token in an
HttpOnly cookie and calls the Go API from the server (Server Components and
Server Actions).

## Development

```bash
cp .env.example .env.local
pnpm install
pnpm dev            # http://localhost:3000 (needs the API on :8080)
```

| Command | Description |
| --- | --- |
| `pnpm test` | Unit and component tests (Vitest + Testing Library) |
| `pnpm e2e` | End-to-end tests (Playwright); builds the app and starts its own API on :8081 and web on :3100, needs `make up`. `E2E_DEV=1 pnpm e2e` uses `next dev` instead of building |
| `pnpm lint` / `pnpm typecheck` | ESLint and TypeScript |
| `pnpm gen:api` | Regenerate API types from `openapi.json` (run `make api-openapi` from the repo root to export it first) |

## Structure

- `src/app/(app)` — authenticated pages; `src/app/login`, `src/app/auth` — sign-in
- `src/app/actions` — Server Actions (all mutations)
- `src/lib/api` — typed API client generated from the OpenAPI document
- `src/components` — UI; `src/components/ui` are shadcn/ui (Base UI) primitives
- `messages/{en,es}.json` — UI copy; the language comes from the user's preference
