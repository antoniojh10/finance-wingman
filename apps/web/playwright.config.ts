import path from "node:path";

import { defineConfig, devices } from "@playwright/test";

// End-to-end tests run the real API (port 8081) and web app (port 3100)
// against the docker-compose Postgres and Mailpit (`make up`). The web app
// is a production build unless E2E_DEV is set.
const apiPort = 8081;
const webPort = 3100;
const databaseUrl =
  process.env.E2E_DATABASE_URL ?? "postgres://finance:finance@localhost:5432/finance_test?sslmode=disable";

export const e2eUser = "e2e@example.com";
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
    { name: "mobile", use: { ...devices["Pixel 7"], storageState: authFile }, dependencies: ["setup"] },
  ],
  webServer: [
    {
      command: "go run ./cmd/api serve",
      cwd: "../api",
      url: `http://localhost:${apiPort}/healthz`,
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
      env: {
        PORT: String(apiPort),
        DATABASE_URL: databaseUrl,
        PUBLIC_URL: `http://localhost:${apiPort}`,
        WEB_BASE_URL: `http://localhost:${webPort}`,
        INITIAL_USERS: `${e2eUser}:E2E User`,
        LOGIN_EMAILS_PER_HOUR: "1000",
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
      reuseExistingServer: !process.env.CI,
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
