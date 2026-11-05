# Backups and restores

Production PostgreSQL runs as a Railway service. Railway Hobby allows no
volume backups (`maxBackupsCount: 0`) and no schedules are configured, so
backups are manual dumps taken with `make prod-backup`.

## When to back up

Take a backup before merging or deploying a change that includes any of:

- a database migration;
- a data change or data fix;
- heavy calculation logic in the API (summaries, budgets, recurring,
  groceries insights, money handling).

The pull request template has a checklist item for it.

## Taking a backup

Requirements: Node (for `npx`), `docker` with the local compose Postgres
running (`make up`) for the verification step, and `gpg` if encrypting.
Log in to Railway once with `npx -y @railway/cli login` and link the
project with `npx -y @railway/cli link`.

```sh
make prod-backup                  # dump + verify
make prod-backup encrypt=1        # also encrypt with gpg (AES256), removes the plaintext
make prod-backup verify=0         # skip the local restore check
scripts/prod-backup.sh --dry-run  # print the steps without running them
```

The script:

1. runs `pg_dump -Fc` inside the Postgres service over `railway ssh`,
   transferred as base64 so the SSH session does not alter the binary data;
2. decodes it to `~/backups/finance-wingman/prod-<UTC timestamp>.dump`
   (`umask 077`) and checks the `PGDMP` header;
3. restores it with `--no-owner --no-acl` into a temporary database
   `verify_prod_<timestamp>` in the local compose Postgres, prints the live
   row estimate per table and the latest goose migration version, and drops
   the temporary database (if compose Postgres is not running the step is
   skipped and the dump is kept);
4. with `encrypt=1`, runs `gpg --symmetric --cipher-algo AES256`.

The script uses the Railway project linked in this directory (run
`npx -y @railway/cli link` once) and the `Postgres` service in the
`production` environment. Override them with `RAILWAY_PROJECT_ID`,
`RAILWAY_ENVIRONMENT` and `RAILWAY_POSTGRES_SERVICE` (names or ids), for
example in your `.env`, and the output directory with `BACKUP_DIR`. Dumps contain user data: never commit them
or store them in the repository.

## Restoring

Restore into a new Postgres service, never over the live one.

1. In the Railway project, create a new PostgreSQL service.
2. Restore the dump into it (decrypt first with `gpg -d` if needed). Use the
   new service's public connection URL, or `railway ssh` into it:

   ```sh
   pg_restore --no-owner --no-acl -d "<new service connection URL>" prod-<timestamp>.dump
   ```

   Use `--no-owner` so objects are owned by the new service's user. The
   `wingman_app` role and its row-level security policies are created by
   the migrations; if the restored database lacks them, run the API
   migrations (`MIGRATE_ON_START=true`) against it.
3. Check the data: row counts per table (`pg_stat_user_tables` or
   `select count(*)`), and `select max(version_id) from goose_db_version`
   against the last migration in `apps/api/internal/db/migrations`.
4. Point the api service's `DATABASE_URL` at the new service and redeploy.
5. Keep the old service and its volume until the restored data is confirmed.

## Incident steps

1. Scale the api service to 0 replicas so nothing writes to the database.
2. Restore the latest dump into a new Postgres service (above).
3. Verify the restored data.
4. Switch the api's `DATABASE_URL` to the new service and scale the api
   back up.
5. Notify affected users of the data window that was lost (changes made
   after the dump).
6. Keep the old service until everything is confirmed.

## Figures

| Date       | Size   | Tables | Transactions | Last migration   |
| ---------- | ------ | ------ | ------------ | ---------------- |
| 2026-11-05 | 231 KB | 28     | 1,367        | 20261015120623   |

First dump taken 2026-11-05 at 11:29 UTC before merging #97, verified by
restoring into a temporary local database.
