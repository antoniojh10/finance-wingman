// Runs Playwright against a database created just for this run, on free
// ports, so runs (and worktrees) do not share state or clash. The database
// lives on the Postgres server in E2E_DATABASE_URL (the docker-compose one by
// default) and is dropped afterwards; stale ones from crashed runs too.
//
// Usage: pnpm e2e [playwright test args]
import { spawn } from "node:child_process";
import net from "node:net";

import {
  createDatabase,
  databaseName,
  dropDatabase,
  dropStaleDatabases,
  newRunId,
  withDatabase,
} from "./e2e-database.mjs";

const adminUrl = process.env.E2E_DATABASE_URL ?? "postgres://finance:finance@localhost:5432/postgres?sslmode=disable";

function freePort() {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.once("error", reject);
    server.listen(0, "127.0.0.1", () => {
      const { port } = server.address();
      server.close(() => resolve(port));
    });
  });
}

const runId = newRunId();
const name = databaseName(runId);
let created = false;
let child;

async function cleanup() {
  if (!created) return;
  created = false;
  try {
    await dropDatabase(adminUrl, name);
  } catch (error) {
    console.error(`could not drop ${name}: ${error.message}`);
  }
}

for (const signal of ["SIGINT", "SIGTERM"]) {
  // Playwright shuts its servers down on a signal; wait for it, then drop.
  process.on(signal, () => child?.kill(signal));
}

let exitCode = 1;
try {
  const dropped = await dropStaleDatabases(adminUrl);
  if (dropped.length > 0) console.log(`dropped stale e2e databases: ${dropped.join(", ")}`);
  await createDatabase(adminUrl, name);
  created = true;

  const env = {
    ...process.env,
    E2E_RUN_ID: runId,
    E2E_RUN_DATABASE_URL: withDatabase(adminUrl, name),
    E2E_API_PORT: String(await freePort()),
    E2E_WEB_PORT: String(await freePort()),
  };
  child = spawn("pnpm", ["exec", "playwright", "test", ...process.argv.slice(2)], { env, stdio: "inherit" });
  exitCode = await new Promise((resolve) => child.on("exit", (code, signal) => resolve(code ?? (signal ? 1 : 0))));
} catch (error) {
  console.error(`e2e: ${error.message} (is Postgres running? try \`make up\`)`);
} finally {
  await cleanup();
}
process.exit(exitCode);
