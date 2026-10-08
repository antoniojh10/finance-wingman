import path from "node:path";

import { defineConfig, devices } from "@playwright/test";

// End-to-end tests run the real API and web app against the docker-compose
// Postgres and Mailpit (`make up`). `pnpm e2e` (scripts/e2e.mjs) gives each
// run its own database, free ports and sign-in email; the defaults below
// only apply when Playwright is started directly. The web app is a
// production build unless E2E_DEV is set.
export const apiPort = Number(process.env.E2E_API_PORT ?? 8081);
const webPort = Number(process.env.E2E_WEB_PORT ?? 3100);
const databaseUrl =
  process.env.E2E_RUN_DATABASE_URL ?? "postgres://finance:finance@localhost:5432/finance_test?sslmode=disable";

// Mailpit is shared, so concurrent runs need distinct addresses.
export const e2eUser = process.env.E2E_RUN_ID ? `e2e-${process.env.E2E_RUN_ID}@example.com` : "e2e@example.com";
// Session saved by e2e/auth.setup.ts, so tests start signed in.
export const authFile = path.join(__dirname, "playwright/.auth/user.json");

export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: `http://localhost:${webPort}`,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    { name: "setup", testMatch: /auth\.setup\.ts/ },
    { name: "desktop", use: { ...devices["Desktop Chrome"], storageState: authFile }, dependencies: ["setup"] },
    // Only the flows tagged @mobile, which cover the mobile navigation (tab
    // bar, header menus, settings via the avatar); the rest only repeat
    // the desktop run.
    { name: "mobile", use: { ...devices["Pixel 7"], storageState: authFile }, dependencies: ["setup"], grep: /@mobile/ },
  ],
  webServer: [
    {
      command: "go run ./cmd/api serve",
      cwd: "../api",
      url: `http://localhost:${apiPort}/healthz`,
      reuseExistingServer: false,
      timeout: 120_000,
      env: {
        PORT: String(apiPort),
        DATABASE_URL: databaseUrl,
        PUBLIC_URL: `http://localhost:${apiPort}`,
        WEB_BASE_URL: `http://localhost:${webPort}`,
        INITIAL_USERS: `${e2eUser}:E2E User`,
        LOGIN_EMAILS_PER_HOUR: "1000",
        // Rate limiting stays on, with room for the whole suite: every test
        // shares one saved session and the web server's address.
        RATE_LIMIT_API_PER_MINUTE: "12000",
        RATE_LIMIT_AUTH_PER_MINUTE: "600",
        // Every run invites people into the same workspace.
        INVITATIONS_PER_HOUR: "1000",
        EMAIL_PROVIDER: "smtp",
        SMTP_HOST: "localhost",
        SMTP_PORT: "1025",
        APP_TIMEZONE: "America/Mexico_City",
      },
    },
    {
      // The standalone production build, as deployed: `next dev` compiles
      // each route on its first visit, which makes every test slower.
      // E2E_DEV=1 skips the build while iterating on a flow.
      command: process.env.E2E_DEV ? `pnpm dev --port ${webPort}` : "pnpm build && pnpm start:standalone",
      url: `http://localhost:${webPort}/login`,
      reuseExistingServer: false,
      timeout: 300_000,
      env: {
        PORT: String(webPort),
        HOSTNAME: "localhost",
        API_URL: `http://localhost:${apiPort}`,
        API_PUBLIC_URL: `http://localhost:${apiPort}`,
        APP_TIMEZONE: "America/Mexico_City",
      },
    },
  ],
});
