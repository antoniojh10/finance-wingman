import { readFile } from "node:fs/promises";

import type { Page } from "@playwright/test";

import { expect, test } from "./fixtures";

import { e2eUser } from "../playwright.config";
import { navigate } from "./helpers";

async function createAccount(page: Page, name: string) {
  await navigate(page, /accounts/i);
  await page.getByRole("button", { name: "New account" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name").fill(name);
  await dialog.getByLabel("Currency").selectOption("MXN");
  await dialog.getByLabel("Opening balance").fill("1500.50");
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Account saved")).toBeVisible();
}

async function download(page: Page, name: string): Promise<{ file: Buffer; filename: string }> {
  const [downloaded] = await Promise.all([page.waitForEvent("download"), page.getByRole("link", { name }).click()]);
  return { file: await readFile(await downloaded.path()), filename: downloaded.suggestedFilename() };
}

test("exports the workspace data as JSON and CSV", { tag: "@mobile" }, async ({ page }, testInfo) => {
  const accountName = `Export ${testInfo.project.name}-${Date.now()}`;
  await page.goto("/");
  await createAccount(page, accountName);

  await navigate(page, /settings/i);
  await expect(page.getByRole("heading", { name: "Export your data" })).toBeVisible();

  const json = await download(page, "Download JSON");
  expect(json.filename).toMatch(/^finance-wingman-export-\d{4}-\d{2}-\d{2}\.json$/);
  const doc = JSON.parse(json.file.toString("utf8"));
  expect(doc.schema_version).toBe(1);
  expect(doc.members.map((m: { email: string }) => m.email)).toContain(e2eUser);
  const account = doc.accounts.find((a: { name: string }) => a.name === accountName);
  expect(account).toMatchObject({ currency: "MXN", initial_balance: 1500.5 });
  expect(Array.isArray(doc.transactions)).toBe(true);

  const csv = await download(page, "Download CSV (zip)");
  expect(csv.filename).toMatch(/\.zip$/);
  // A zip starts with "PK" and stores file names uncompressed.
  expect(csv.file.subarray(0, 2).toString("latin1")).toBe("PK");
  const listing = csv.file.toString("latin1");
  for (const entity of ["members", "accounts", "categories", "transactions", "recurring_items", "budgets"]) {
    expect(listing).toContain(`${entity}.csv`);
  }
});
