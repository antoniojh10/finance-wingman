// Per-run Postgres databases for the end-to-end tests.
import pg from "pg";

export const prefix = "finance_e2e_";
const staleAfterMs = 24 * 60 * 60 * 1000;

/** Postgres identifiers are quoted, never interpolated raw. */
const quote = (name) => `"${name.replaceAll('"', '""')}"`;

/** A run id that sorts by creation time: `<ms>_<random>`. */
export function newRunId(now = Date.now()) {
  return `${now}_${Math.random().toString(36).slice(2, 8)}`;
}

export function databaseName(runId) {
  return `${prefix}${runId}`;
}

/** The same connection URL pointing at another database. */
export function withDatabase(url, name) {
  const next = new URL(url);
  next.pathname = `/${name}`;
  return next.toString();
}

/** Whether `name` is an e2e database created more than a day before `now`. */
export function isStale(name, now = Date.now()) {
  if (!name.startsWith(prefix)) return false;
  const created = Number(name.slice(prefix.length).split("_")[0]);
  return Number.isFinite(created) && now - created > staleAfterMs;
}

async function withAdmin(adminUrl, fn) {
  const client = new pg.Client({ connectionString: adminUrl });
  await client.connect();
  try {
    return await fn(client);
  } finally {
    await client.end();
  }
}

/** Creates an empty database; the API migrates it when it starts. */
export function createDatabase(adminUrl, name) {
  return withAdmin(adminUrl, (client) => client.query(`CREATE DATABASE ${quote(name)}`));
}

export function dropDatabase(adminUrl, name) {
  return withAdmin(adminUrl, (client) => client.query(`DROP DATABASE IF EXISTS ${quote(name)} WITH (FORCE)`));
}

/** Drops e2e databases left behind by crashed runs. Returns their names. */
export function dropStaleDatabases(adminUrl, now = Date.now()) {
  return withAdmin(adminUrl, async (client) => {
    const { rows } = await client.query("SELECT datname FROM pg_database WHERE datname LIKE $1", [`${prefix}%`]);
    const stale = rows.map((row) => row.datname).filter((name) => isStale(name, now));
    for (const name of stale) {
      await client.query(`DROP DATABASE IF EXISTS ${quote(name)} WITH (FORCE)`);
    }
    return stale;
  });
}
