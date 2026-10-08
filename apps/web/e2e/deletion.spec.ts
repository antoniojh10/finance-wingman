import { expect, test } from "./fixtures";
import { latestEmail, signedInContext } from "./helpers";

// Each test acts as a fresh user so the shared e2e user and workspace are
// never scheduled for deletion. Carrying out a deletion after the 7-day
// grace period is covered by the API tests.

function uniqueEmail(prefix: string): string {
  return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 1e6)}@example.com`;
}

test("an owner schedules and cancels the deletion of a workspace", async ({ browser }) => {
  const email = uniqueEmail("delete-workspace");
  const context = await signedInContext(browser, email, { workspace: "Doomed" });
  const page = await context.newPage();

  await page.goto("/settings/workspace");
  await expect(page.getByRole("heading", { name: "Delete workspace" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Download JSON" }).first()).toBeVisible();
  await page.getByRole("button", { name: "Delete workspace" }).click();

  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText(/stays in our backups/)).toBeVisible();
  const confirm = dialog.getByRole("button", { name: "Delete workspace" });
  await expect(confirm).toBeDisabled();
  await dialog.getByLabel("Type Doomed to confirm").fill("Doomed");
  const since = new Date(Date.now() - 1000);
  await confirm.click();

  await expect(page.getByText("Workspace deletion scheduled")).toBeVisible();
  await expect(page.getByRole("status").filter({ hasText: "Doomed will be deleted on" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "This workspace will be deleted" })).toBeVisible();
  expect(await latestEmail(email, since)).toContain("scheduled the workspace “Doomed” for deletion");

  // The banner follows the user around the app.
  await page.goto("/");
  await expect(page.getByRole("status").filter({ hasText: "Doomed will be deleted on" })).toBeVisible();
  await page.getByRole("status").getByRole("link", { name: "Review" }).click();

  await page.getByRole("button", { name: "Cancel deletion" }).click();
  await expect(page.getByText("Deletion cancelled")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Delete workspace" })).toBeVisible();
  await expect(page.getByRole("status").filter({ hasText: "will be deleted" })).toHaveCount(0);
  await context.close();
});
