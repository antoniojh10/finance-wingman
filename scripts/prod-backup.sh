#!/usr/bin/env bash
# Manual production backup: dumps the Railway Postgres service over
# `railway ssh`, saves it locally and verifies it in the local compose
# Postgres. See docs/backups.md.
#
# Usage: scripts/prod-backup.sh [--dry-run] [--encrypt] [--no-verify]
#   (or via make: make prod-backup [encrypt=1] [verify=0])
#
# Environment:
#   BACKUP_DIR                    output directory (default ~/backups/finance-wingman)
#   RAILWAY_PROJECT_ID            Railway project id (default: the project linked
#                                 with `npx -y @railway/cli link`)
#   RAILWAY_ENVIRONMENT           environment name or id (default production)
#   RAILWAY_POSTGRES_SERVICE      Postgres service name or id (default Postgres)
# Put them in .env (gitignored) if you don't want to link the project.
set -euo pipefail
umask 077

DRY_RUN=0
ENCRYPT=0
VERIFY=1
for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=1 ;;
    --encrypt) ENCRYPT=1 ;;
    --no-verify) VERIFY=0 ;;
    -h | --help)
      sed -n '2,15p' "$0"
      exit 0
      ;;
    *)
      echo "unknown argument: $arg" >&2
      exit 2
      ;;
  esac
done

ENVIRONMENT="${RAILWAY_ENVIRONMENT:-production}"
POSTGRES_SERVICE="${RAILWAY_POSTGRES_SERVICE:-Postgres}"
SSH_ARGS=(--environment "$ENVIRONMENT" --service "$POSTGRES_SERVICE")
if [ -n "${RAILWAY_PROJECT_ID:-}" ]; then
  SSH_ARGS=(--project "$RAILWAY_PROJECT_ID" "${SSH_ARGS[@]}")
fi
BACKUP_DIR="${BACKUP_DIR:-$HOME/backups/finance-wingman}"
TS="$(date -u +%Y%m%dT%H%MZ)"
B64="$BACKUP_DIR/prod-$TS.b64"
DUMP="$BACKUP_DIR/prod-$TS.dump"
VERIFY_DB="verify_prod_${TS,,}"

# Runs inside the Postgres service; base64 keeps the binary dump intact over SSH.
REMOTE_CMD='pg_dump -Fc -h localhost -U "$PGUSER" -d "$PGDATABASE" | base64 -w0'

if [ "$DRY_RUN" = 1 ]; then
  cat <<DRY
[dry-run] mkdir -p $BACKUP_DIR
[dry-run] npx -y @railway/cli ssh ${SSH_ARGS[*]} -- sh -c '$REMOTE_CMD' > $B64
[dry-run] tr -d '\r\n ' < $B64 | base64 -d > $DUMP
[dry-run] check first 5 bytes of $DUMP are PGDMP; rm $B64
DRY
  if [ "$VERIFY" = 1 ]; then
    cat <<DRY
[dry-run] docker compose exec -T postgres createdb -U finance $VERIFY_DB
[dry-run] docker compose exec -T postgres pg_restore -U finance -d $VERIFY_DB --no-owner --no-acl < $DUMP
[dry-run] query pg_stat_user_tables and goose_db_version in $VERIFY_DB
[dry-run] docker compose exec -T postgres dropdb -U finance --if-exists $VERIFY_DB (always)
DRY
  fi
  if [ "$ENCRYPT" = 1 ]; then
    echo "[dry-run] gpg --symmetric --cipher-algo AES256 $DUMP; rm $DUMP if gpg succeeded"
  fi
  exit 0
fi

mkdir -p "$BACKUP_DIR"

verify() {
  local psql_db=(docker compose exec -T postgres psql -U finance -d "$VERIFY_DB" -At)
  docker compose exec -T postgres createdb -U finance "$VERIFY_DB"
  # Always drop the temporary database, even if the restore fails.
  trap 'docker compose exec -T postgres dropdb -U finance --if-exists "$VERIFY_DB" >/dev/null 2>&1 || true' EXIT RETURN
  docker compose exec -T postgres pg_restore -U finance -d "$VERIFY_DB" --no-owner --no-acl <"$DUMP"
  echo "--- relname|n_live_tup"
  "${psql_db[@]}" -c "select relname, n_live_tup from pg_stat_user_tables order by relname"
  echo "--- latest goose version"
  "${psql_db[@]}" -c "select max(version_id) from goose_db_version"
}

# BACKUP_SKIP_DUMP lets tests feed an existing dump to the steps after the download.
if [ -z "${BACKUP_SKIP_DUMP:-}" ]; then
  trap 'rm -f "$B64"' EXIT
  echo "Dumping the $POSTGRES_SERVICE service ($ENVIRONMENT)..."
  if ! npx -y @railway/cli ssh "${SSH_ARGS[@]}" -- sh -c "$REMOTE_CMD" >"$B64"; then
    echo "error: railway ssh failed. Log in with 'npx -y @railway/cli login' and link the project with 'npx -y @railway/cli link' (or set RAILWAY_PROJECT_ID)." >&2
    exit 1
  fi
  tr -d '\r\n ' <"$B64" | base64 -d >"$DUMP"
  rm -f "$B64"
else
  cp "$BACKUP_SKIP_DUMP" "$DUMP"
fi

if [ "$(head -c 5 "$DUMP")" != "PGDMP" ]; then
  echo "error: $DUMP does not start with PGDMP; the dump is not valid (kept for inspection)" >&2
  exit 1
fi

if [ "$VERIFY" = 1 ]; then
  if docker compose exec -T postgres pg_isready -U finance >/dev/null 2>&1; then
    echo "Verifying by restoring into temporary database $VERIFY_DB..."
    verify
    echo "Temporary database $VERIFY_DB dropped."
  else
    echo "warning: compose Postgres is not running (make up); skipping verification. The dump is kept." >&2
  fi
fi

FINAL="$DUMP"
if [ "$ENCRYPT" = 1 ]; then
  if gpg --symmetric --cipher-algo AES256 "$DUMP"; then
    rm -f "$DUMP"
    FINAL="$DUMP.gpg"
  else
    echo "error: gpg failed; the plaintext dump was kept at $DUMP" >&2
    exit 1
  fi
fi

echo "Backup saved: $FINAL ($(du -h "$FINAL" | cut -f1))"
