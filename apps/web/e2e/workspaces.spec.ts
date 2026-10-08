import { expect, test, type Page } from "@playwright/test";

import { expectWorkspace, invitationLinkFrom, latestEmail, navigate, openWorkspaceMenu } from "./helpers";

async function createAccount(page: Page, name: string) {
  await navigate(page, /accounts/i);
  await page.getByRole("button", { name: "New account" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name").fill(name);
  await dialog.getByLabel("Currency").selectOption("MXN");
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Account saved")).toBeVisible();
}

async function expectAccount(page: Page, name: string, visible: boolean) {
  await navigate(page, /accounts/i);
  await expect(page.getByRole("heading", { name: "Accounts" })).toBeVisible();
  const row = page.getByTestId("account-row").filter({ hasText: name });
  if (visible) {
    await expect(row).toBeVisible();
  } else {
    await expect(row).toHaveCount(0);
  }
}

test("invites someone to a workspace and keeps workspaces apart", { tag: "@mobile" }, async ({ page, browser }, testInfo) => {
  const suffix = `${testInfo.project.name}-${Date.now()}`;
  const invitee = `invitee-${suffix}@example.com`;
  const sharedAccount = `Shared ${suffix}`;
  const personalAccount = `Personal ${suffix}`;

  // The owner shares an account and invites someone.
  await page.goto("/");
  await createAccount(page, sharedAccount);
  await openWorkspaceMenu(page);
  await page.getByRole("menuitem", { name: "Manage workspace" }).click();
  await expect(page.getByRole("heading", { name: "Invite someone" })).toBeVisible();
  const ownerWorkspace = (await page.getByLabel("Name").inputValue()).trim();

  const since = new Date(Date.now() - 1000);
  await page.getByLabel("Email").fill(invitee);
  await page.getByRole("button", { name: "Invite" }).click();
  await expect(page.getByText("Invitation sent")).toBeVisible();
  await expect(page.getByText(invitee)).toBeVisible();
  const link = invitationLinkFrom(await latestEmail(invitee, since));

  // The invitee accepts in their own browser and lands in the workspace.
  // Without the owner's saved session.
  const context = await browser.newContext({ ...testInfo.project.use, storageState: undefined });
  const guest = await context.newPage();
  await guest.goto(link.replace(/^https?:\/\/[^/]+/, ""));
  await expect(guest.getByRole("heading", { name: "Join a workspace" })).toBeVisible();
  await expect(guest.getByText(/invited .* to the workspace/)).toContainText(ownerWorkspace);
  await guest.getByRole("button", { name: "Accept invitation" }).click();
  await expect(guest.getByRole("heading", { name: "Dashboard" })).toBeVisible();
  await expectWorkspace(guest, ownerWorkspace);
  await expectAccount(guest, sharedAccount, true);

  // The link works once.
  await guest.goto(link.replace(/^https?:\/\/[^/]+/, ""));
  await expect(guest.getByText(/invalid, has expired or was already used/)).toBeVisible();

  // A new workspace starts empty and doesn't show the shared data.
  await guest.goto("/");
  await openWorkspaceMenu(guest);
  await guest.getByRole("menuitem", { name: "New workspace" }).click();
  const dialog = guest.getByRole("dialog", { name: "Create a workspace" });
  await dialog.getByLabel("Name").fill(`Personal ${suffix}`);
  await dialog.getByRole("button", { name: "New workspace" }).click();
  await expect(guest.getByText("Workspace created")).toBeVisible();
  await expectWorkspace(guest, `Personal ${suffix}`);
  await expectAccount(guest, sharedAccount, false);

  // The empty dashboard's call to action opens the new account form directly.
  await guest.goto("/");
  await guest.getByRole("link", { name: "Create account" }).click();
  await expect(guest).toHaveURL(/\/accounts\?new=1/);
  const accountDialog = guest.getByRole("dialog");
  await accountDialog.getByLabel("Name").fill(personalAccount);
  await accountDialog.getByLabel("Currency").selectOption("MXN");
  await accountDialog.getByRole("button", { name: "Save" }).click();
  await expect(guest.getByText("Account saved")).toBeVisible();
  await expect(guest).toHaveURL(/\/accounts$/);

  // Switching back shows the shared data again, and never the personal one.
  await openWorkspaceMenu(guest);
  await guest.getByRole("menuitem", { name: ownerWorkspace }).click();
  await expectWorkspace(guest, ownerWorkspace);
  await expectAccount(guest, sharedAccount, true);
  await expectAccount(guest, personalAccount, false);
  await context.close();

  // The owner sees the new member, no longer pending.
  await page.reload();
  const card = (heading: string) => page.locator("[data-slot=card]").filter({ has: page.getByRole("heading", { name: heading }) });
  await expect(card("Members")).toContainText(invitee);
  await expect(card("Pending invitations")).not.toContainText(invitee);
});
