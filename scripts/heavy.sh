#!/usr/bin/env bash
# Runs a heavy test suite (API tests, web unit tests, e2e) while holding a
# lock shared by every checkout and worktree of this repository, so suites
# started in parallel (by people or agents) run one after another instead
# of competing for CPU and Postgres and timing out.
#
# Skipped in CI, when the lock is already held by a parent (nested calls),
# and where flock is not installed (e.g. macOS without util-linux).
set -euo pipefail

if [[ -n "${CI:-}" || -n "${HEAVY_LOCK_HELD:-}" ]] || ! command -v flock >/dev/null 2>&1; then
  exec "$@"
fi

lock="$(cd "$(git rev-parse --git-common-dir)" && pwd)/heavy-tests.lock"
export HEAVY_LOCK_HELD=1
exec 9>"$lock"
if ! flock -n 9; then
  echo "heavy.sh: another test suite is running in this repository; waiting for it to finish..." >&2
  flock 9
fi
exec "$@"
