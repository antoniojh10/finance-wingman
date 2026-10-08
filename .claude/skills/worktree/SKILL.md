---
name: worktree
description: Create, set up, and remove git worktrees of finance-wingman so several tasks (or agents) can run in parallel without clashing on ports, env files, or the database. Use when the user wants to work on tasks in parallel, start a task "in a worktree", run the app for two branches at once, or clean up a finished worktree. Also use right after `claude -w`, EnterWorktree, or an `isolation: worktree` subagent lands in a fresh worktree of this repo.
---

# Worktrees for parallel tasks

A plain `git worktree add` is not enough here: `.env` and
`apps/web/.env.local` are gitignored (missing in the new tree), the API and
web ports are fixed (8080/3000), and every checkout would share the
`finance` database. The scripts in `scripts/` fix that.

Each worktree gets:

| | main checkout | worktree slot N (1–9) |
|---|---|---|
| API (`make api-run`) | 8080 | 8090 + N |
| Web (`make web-dev`) | 3000 | 3000 + N |
| Database | `finance` | `finance_wt_<dir>` (same Postgres) |

Postgres and Mailpit are shared: they run once from the main checkout.

## Create a worktree for a task

```bash
.claude/skills/worktree/scripts/new.sh feat/budgets          # from main
.claude/skills/worktree/scripts/new.sh fix/foo origin/main    # other base
```

This creates `.claude/worktrees/<branch-slug>` on a new branch (or checks out
an existing one), then runs `setup.sh`. Then work from that directory (`cd`
there, or open a new `claude` session in it) and report the URLs it prints.

## Worktree created another way

After `claude -w`, `EnterWorktree`, an `isolation: worktree` subagent, or a
manual `git worktree add`, run setup from inside the worktree before
running the app or tests:

```bash
"$(git worktree list --porcelain | awk '/^worktree /{print substr($0,10); exit}')/.claude/skills/worktree/scripts/setup.sh"
```

(In a worktree created from a commit that already has this skill,
`.claude/skills/worktree/scripts/setup.sh` works too.) Setup is idempotent:
it keeps the slot and database it assigned before.

## Rules inside a worktree

- **Never run `make up` / `make down` / `make psql` in a worktree.** They
  start or stop a second compose project with clashing ports. Run them from
  the main checkout. To query the worktree DB:
  `docker compose --project-directory <main> exec postgres psql -U finance -d finance_wt_<dir>`.
- The worktree DB starts empty; migrations run on the first `make api-run`
  (`MIGRATE_ON_START=true`) and `INITIAL_USERS` is copied from the main
  `.env`, so magic-link login works. Emails land in the shared Mailpit
  (http://localhost:8025); the link points to the worktree's web port.
- `make api-generate`, `make api-openapi`, lint and unit tests are local to
  the worktree and safe to run in parallel.
- **`make api-test` in parallel:** fine while worktrees have the same
  migrations. If a branch adds or changes a migration, don't run it at the
  same time as another worktree's `api-test`: `testutil` drops test
  templates built from other migration sets, so the other run may fail
  mid-way. Re-run it if that happens.
- **`make web-e2e` can run in several worktrees at once:** each run gets its
  own `finance_e2e_*` database and free ports, and is dropped afterwards.
- Commit on the worktree's branch as usual (Conventional Commits, see
  AGENTS.md). Merging back to `main` is the user's call.

## Remove a worktree

```bash
.claude/skills/worktree/scripts/remove.sh .claude/worktrees/<slug>          # refuses if dirty
.claude/skills/worktree/scripts/remove.sh .claude/worktrees/<slug> --force  # discard changes
```

It drops `finance_wt_<dir>` and runs `git worktree remove`; the branch is
kept. Stop that worktree's `api-run` / `web-dev` first. Confirm with the user
before `--force` or before deleting the branch (`git branch -d <branch>`).

`git worktree list` shows every worktree; each one's slot is
`WORKTREE_SLOT` in its `.env`.
