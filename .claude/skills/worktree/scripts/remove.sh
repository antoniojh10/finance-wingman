#!/usr/bin/env bash
# Removes a linked worktree and drops its database. Refuses when the
# worktree has uncommitted changes unless --force is given. The branch is
# kept; delete it yourself once merged.
#
# Usage: remove.sh <worktree-path> [--force]
set -euo pipefail
source "$(dirname "$0")/lib.sh"

wt=$(cd "${1:?usage: remove.sh <worktree-path> [--force]}" && git rev-parse --show-toplevel)
force=${2:-}
main=$(main_checkout)
branch=$(git -C "$wt" branch --show-current)

if [ "$wt" = "$main" ]; then
  echo "error: refusing to remove the main checkout" >&2
  exit 1
fi
if [ "$force" != "--force" ] && [ -n "$(git -C "$wt" status --porcelain)" ]; then
  echo "error: $wt has uncommitted changes; commit them or pass --force" >&2
  git -C "$wt" status --short >&2
  exit 1
fi

db_url=$(get_env "$wt/.env" DATABASE_URL)
db_name=$(printf '%s' "$db_url" | sed -E 's|.*/([^/?]+)(\?.*)?$|\1|')
cd "$main"
if [[ $db_name == finance_wt_* ]]; then
  shared_psql -c "DROP DATABASE IF EXISTS \"$db_name\" WITH (FORCE)" >/dev/null
  echo "dropped database $db_name"
fi
git worktree remove ${force:+--force} "$wt"
echo "removed worktree $wt (branch $branch kept)"
