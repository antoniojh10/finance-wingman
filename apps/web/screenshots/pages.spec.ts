import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";

import { expect, test, type Locator, type Page } from "@playwright/test";

import { optimizePng } from "./optimize";

// One PNG per page and project (device and color scheme) in docs/screenshots.
const outDir = path.join(__dirname, "../../../docs/screenshots");

/** Waits until the page looks settled: data loaded, fonts ready, charts drawn. */
async function settle(page: Page): Promise<void> {
  await page.waitForLoadState("networkidle");
  await page.evaluate(() => document.fonts.ready);
  // Hide toasts and stop animations so charts are fully drawn.
  await page.addStyleTag({
    content: "[data-sonner-toaster]{display:none!important}*,*::before,*::after{animation:none!important;transition:none!important}",
  });
  await page.waitForTimeout(600);
}

async function save(name: string, project: string, image: Buffer): Promise<void> {
  mkdirSync(outDir, { recursive: true });
  writeFileSync(path.join(outDir, `${name}-${project}.png`), await optimizePng(image));
}

/** Captures the first screen of the page, or just `target` when given. */
async function capture(page: Page, name: string, project: string, target?: Locator): Promise<void> {
  await settle(page);
  if (target) {
    // The fixed tab bar would cover the element on mobile.
    await page.addStyleTag({ content: 'nav[aria-label="Main"]{display:none!important}' });
    await target.scrollIntoViewIfNeeded();
    await save(name, project, await target.screenshot({ animations: "disabled" }));
    return;
  }
  // The viewport is the crop: long pages show their top, as a user would see it.
  const image = await page.screenshot({ animations: "disabled" });
  await save(name, project, image);
}

/** The previous month (YYYY-MM): in the first days of a month the current one has little data. */
function lastMonth(): string {
  const now = new Date();
  return new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - 1, 1)).toISOString().slice(0, 7);
}

const pages: { name: string; path: string; mobilePath?: string; ready: (page: Page) => Locator; target?: (page: Page) => Locator }[] = [
  { name: "dashboard", path: `/?month=${lastMonth()}`, ready: (p) => p.getByTestId("reports-chart") },
  { name: "transactions", path: "/transactions", mobilePath: "/transactions?type=expense", ready: (p) => p.getByRole("heading", { name: "Transactions" }).first() },
  { name: "reports", path: "/reports", ready: (p) => p.getByRole("heading", { name: "Reports" }).first() },
  { name: "budgets", path: `/budgets?month=${lastMonth()}`, ready: (p) => p.getByRole("heading", { name: "Budgets" }).first() },
  { name: "recurring", path: "/subscriptions", ready: (p) => p.getByRole("heading", { name: "Subscriptions" }).first() },
  { name: "groceries", path: `/groceries?month=${lastMonth()}`, ready: (p) => p.getByRole("heading", { name: "Groceries" }).first() },
  {
    name: "store-comparison",
    path: "/groceries",
    ready: (p) => p.getByTestId("compare-matrix"),
    target: (p) => p.getByTestId("groceries-compare"),
  },
  {
    name: "assistants",
    path: "/settings",
    ready: (p) => p.getByTestId("mcp-url"),
    target: (p) => p.locator("section").filter({ has: p.getByTestId("mcp-url") }),
  },
];

for (const shot of pages) {
  test(shot.name, async ({ page }, testInfo) => {
    const mobile = testInfo.project.name.startsWith("mobile");
    // Transfer rows overflow on narrow screens, so the mobile list shows expenses.
    await page.goto((mobile && shot.mobilePath) || shot.path);
    await expect(shot.ready(page)).toBeVisible();
    if (shot.name === "assistants") {
      // The URL depends on the run's port: show a stable one.
      await page.getByTestId("mcp-url").evaluate((el) => (el.textContent = "https://wingman.example.com/mcp"));
    }
    await capture(page, shot.name, testInfo.project.name, shot.target?.(page));
  });
}
