import { execFileSync } from "node:child_process";
import path from "node:path";

import { test as setup } from "@playwright/test";

import { mintSession } from "../e2e/helpers";
import { apiPort, databaseUrl, e2eUser } from "../playwright.config";
import { screenshotsAuthFile } from "../playwright.screenshots.config";

// Fills this run's database with the demo workspace for this run's user (the
// API created the user from INITIAL_USERS when it started), then signs in on
// it. The seed is dated relative to today.
setup("seed the demo workspace and sign in", async ({ context }) => {
  setup.setTimeout(180_000);
  execFileSync("go", ["run", "./cmd/api", "demo", "seed", "--email", e2eUser, "--workspace", "Demo", "--locale", "en", "--reset"], {
    cwd: path.join(__dirname, "../../api"),
    env: { ...process.env, DATABASE_URL: databaseUrl, PORT: String(apiPort) },
    stdio: "inherit",
  });
  await context.addCookies([mintSession(e2eUser, { workspace: "Demo" })]);
  await context.storageState({ path: screenshotsAuthFile });
});
