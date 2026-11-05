import path from "node:path";

import { defineConfig, devices } from "@playwright/test";

import base, { e2eUser } from "./playwright.config";

// README screenshots (`make screenshots`, i.e. `pnpm screenshots`). Reuses the
// e2e servers and per-run database from playwright.config.ts, but runs the
// specs in ./screenshots, which the e2e config never matches, so they cannot
// run in the normal suite or in CI. The project name (<device>-<scheme>) ends
// up in the file names.
export const screenshotsAuthFile = path.join(__dirname, "playwright/.auth/demo.json");

const desktop = { ...devices["Desktop Chrome"], viewport: { width: 1280, height: 900 }, deviceScaleFactor: 1 };
// Taller than a phone, so charts show below the controls.
const mobile = { ...devices["Pixel 7"], viewport: { width: 412, height: 1100 }, deviceScaleFactor: 1 };

const shots = (name: string, device: object, colorScheme: "light" | "dark") => ({
  name,
  testMatch: /pages\.spec\.ts/,
  use: { ...device, colorScheme, storageState: screenshotsAuthFile },
  dependencies: ["setup"],
});

export default defineConfig({
  ...base,
  testDir: "./screenshots",
  retries: 0,
  reporter: "list",
  use: { baseURL: base.use?.baseURL },
  webServer: ([base.webServer ?? []].flat()).map((server) => ({
    ...server,
    env: server.env?.INITIAL_USERS ? { ...server.env, INITIAL_USERS: `${e2eUser}:Alex Morgan` } : server.env,
  })),
  projects: [
    { name: "setup", testMatch: /demo\.setup\.ts/ },
    shots("desktop-light", desktop, "light"),
    shots("desktop-dark", desktop, "dark"),
    shots("mobile-light", mobile, "light"),
    shots("mobile-dark", mobile, "dark"),
  ],
});
