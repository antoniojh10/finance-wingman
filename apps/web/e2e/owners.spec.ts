import type { Page } from "@playwright/test";

import { expect, test } from "./fixtures";
import { invitationLinkFrom, latestEmail, navigate, openWorkspaceMenu } from "./helpers";

async function createAccount(page: Page, name: string, owner?: string) {
  await navigate(page, /accounts/i);
  await page.getByRole("button", { name: "New account" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name").fill(name);
  await dialog.getByLabel("Currency").selectOption("EUR");
  if (owner) {
    await dialog.getByLabel("Owner").selectOption({ label: owner });
  }
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Account saved")).toBeVisible();
}

test("members each have their own account with the same name", async ({ page, browser }, testInfo) => {
  const suffix = `${testInfo.project.name}-${Date.now()}`;
  const invitee = `owner-${suffix}@example.com`;
  const bank = `BNP ${suffix}`;

  // The owner invites someone, who joins the workspace.
  await page.goto("/");
  await openWorkspaceMenu(page);
  await page.getByRole("menuitem", { name: "Manage workspace" }).click();
  const since = new Date(Date.now() - 1000);
  await page.getByLabel("Email").fill(invitee);
  await page.getByRole("button", { name: "Invite" }).click();
  await expect(page.getByText("Invitation sent")).toBeVisible();
  const link = invitationLinkFrom(await latestEmail(invitee, since));

  // Without the owner's saved session.
  const context = await browser.newContext({ ...testInfo.project.use, storageState: undefined });
  const guest = await context.newPage();
  await guest.goto(link.replace(/^https?:\/\/[^/]+/, ""));
  await guest.getByRole("button", { name: "Accept invitation" }).click();
  await expect(guest.getByRole("heading", { name: "Dashboard" })).toBeVisible();

  // Both add a bank account with the same name; a third one is shared.
  await createAccount(page, bank);
  await createAccount(guest, bank);
  await createAccount(page, `Joint ${suffix}`, "Shared (joint account)");
  await context.close();

  // The owner tells them apart and can look at one person's accounts.
  await navigate(page, /accounts/i);
  const rows = page.getByTestId("account-row").filter({ hasText: bank });
  await expect(rows).toHaveCount(2);
  await expect(rows.filter({ hasText: "Checking · E2E User" })).toHaveCount(1);
  await expect(rows.filter({ hasText: `Checking · ${invitee}` })).toHaveCount(1);

  await page.getByRole("link", { name: "Mine", exact: true }).click();
  await expect(page).toHaveURL(/owner=/);
  await expect(rows).toHaveCount(1);
  await expect(rows).toContainText("E2E User");
  await expect(page.getByTestId("account-row").filter({ hasText: `Joint ${suffix}` })).toHaveCount(0);

  await page.getByRole("link", { name: "Shared", exact: true }).click();
  await expect(page).toHaveURL(/owner=shared/);
  await expect(page.getByTestId("account-row").filter({ hasText: `Joint ${suffix}` })).toHaveCount(1);
  await expect(rows).toHaveCount(0);
});
