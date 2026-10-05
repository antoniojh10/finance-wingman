#!/usr/bin/env bash
# Creates a worktree on a new branch under .claude/worktrees/ and sets it up.
#
# Usage: new.sh <branch> [base]   (base defaults to main)
#   new.sh feat/budgets
#   new.sh fix/login-rate-limit origin/main
set -euo pipefail
source "$(dirname "$0")/lib.sh"

branch=${1:?usage: new.sh <branch> [base]}
base=${2:-main}
main=$(main_checkout)
path="$main/.claude/worktrees/$(slugify "$branch" | tr '_' '-')"

if [ -e "$path" ]; then
  echo "error: $path already exists" >&2
  exit 1
fi
if git show-ref --verify --quiet "refs/heads/$branch"; then
  git worktree add "$path" "$branch"
else
  git worktree add -b "$branch" "$path" "$base"
fi
"$(dirname "$0")/setup.sh" "$path"
