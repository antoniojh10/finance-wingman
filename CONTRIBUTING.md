# Contributing

The rules for every contributor, human or agent, live in
[AGENTS.md](AGENTS.md); read it first. This page covers the basics.

## Setup

Requirements: Go 1.27+, Node.js 22+ and pnpm 10+, Docker with Compose.

```bash
cp .env.example .env
make up                            # Postgres on :5432, Mailpit on :8025
make api-run                       # API on :8080 (applies migrations on start)
make web-install && make web-dev   # Web on :3000
```

Set `INITIAL_USERS` in `.env` to grant yourself access, then sign in with
the link or code that arrives in Mailpit. See the [README](README.md) for
details.

## Tests and lint

Run these before opening a pull request (`make up` must be running for
tests that use Postgres):

```bash
make api-lint api-test             # backend
make web-lint web-test             # frontend
make web-e2e                       # when a user flow changed
```

A change is not done until its tests are written and passing. After
editing SQL queries or migrations run `make api-generate`; after changing
API endpoints or schemas run `make api-openapi`. Schema changes go in a new
migration (`make migrate-new name=add_x`); never edit an applied one.

## Commits

Messages follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/),
for example `feat(mcp): add update_transaction tool`. Types, scopes and
the header limit are listed in [AGENTS.md](AGENTS.md#commits). Keep one
logical change per commit, with its tests. Code, comments and commit
messages are written in English.

## Pull requests

1. Branch from `main` (`feat/...`, `fix/...`, `docs/...`).
2. Open a pull request against `main` and fill in the template.
3. CI must pass. To bring in changes from `main`, merge `main` into your
   branch; do not rebase or force-push.

## Reporting problems

Use the issue templates for bugs and feature requests. Report security
problems privately as described in [SECURITY.md](SECURITY.md).
