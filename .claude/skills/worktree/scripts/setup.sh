#!/usr/bin/env bash
# Prepares a linked worktree so it can run alongside the main checkout and
# other worktrees: copies the gitignored env files, assigns free ports, and
# creates a dedicated database. Idempotent; safe to re-run.
#
# Usage: setup.sh [worktree-path]   (defaults to the current directory)
set -euo pipefail
source "$(dirname "$0")/lib.sh"

cd "${1:-.}"
wt=$(git rev-parse --show-toplevel)
cd "$wt"
main=$(main_checkout)

if [ "$wt" = "$main" ]; then
  echo "error: $wt is the main checkout; run this inside a linked worktree" >&2
  exit 1
fi
for f in .env apps/web/.env.local; do
  if [ ! -f "$main/$f" ]; then
    echo "error: $main/$f is missing; create it from the .env.example first" >&2
    exit 1
  fi
done

# Env files are gitignored, so a fresh worktree has none.
[ -f .env ] || cp "$main/.env" .env
[ -f apps/web/.env.local ] || cp "$main/apps/web/.env.local" apps/web/.env.local

# Slot N gives API port 8090+N and web port 3000+N (main uses 8080/3000;
# e2e picks free ports per run). Keep the slot already assigned to this worktree.
slot=$(get_env .env WORKTREE_SLOT)
if [ -z "$slot" ]; then
  used=" "
  while IFS= read -r other; do
    [ "$other" = "$wt" ] && continue
    used+="$(get_env "$other/.env" WORKTREE_SLOT) "
  done < <(linked_worktrees)
  for n in 1 2 3 4 5 6 7 8 9; do
    if [[ $used != *" $n "* ]]; then slot=$n; break; fi
  done
  if [ -z "$slot" ]; then
    echo "error: all 9 worktree slots are taken; remove a worktree first" >&2
    exit 1
  fi
fi
api_port=$((8090 + slot))
web_port=$((3000 + slot))

db_name="finance_wt_$(slugify "$(basename "$wt")")"
main_db_url=$(get_env "$main/.env" DATABASE_URL)
db_url=$(printf '%s' "$main_db_url" | sed -E "s|/[^/?]+(\?.*)?$|/${db_name}\1|")

set_env .env WORKTREE_SLOT "$slot"
set_env .env PORT "$api_port"
set_env .env WEB_PORT "$web_port"
set_env .env DATABASE_URL "$db_url"
set_env .env MIGRATE_ON_START true
set_env .env PUBLIC_URL "http://localhost:$api_port"
set_env .env WEB_BASE_URL "http://localhost:$web_port"
set_env apps/web/.env.local API_URL "http://localhost:$api_port"
set_env apps/web/.env.local API_PUBLIC_URL "http://localhost:$api_port"

# The worktree shares the main checkout's Postgres; it only gets its own DB.
if ! shared_psql -c 'SELECT 1' >/dev/null 2>&1; then
  echo "error: shared Postgres is not running; run 'make up' in $main" >&2
  exit 1
fi
if [ "$(shared_psql -tAc "SELECT 1 FROM pg_database WHERE datname = '$db_name'")" != "1" ]; then
  shared_psql -c "CREATE DATABASE \"$db_name\" OWNER finance" >/dev/null
fi

if [ ! -d apps/web/node_modules ]; then
  (cd apps/web && pnpm install --frozen-lockfile --silent)
fi

cat <<EOF
Worktree ready: $wt
  branch:   $(git branch --show-current)
  slot:     $slot
  API:      http://localhost:$api_port   (make api-run)
  web:      http://localhost:$web_port   (make web-dev)
  database: $db_name (migrated on first api-run)
EOF
