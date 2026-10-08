import { expect, test } from "./fixtures";
import { navigate, openNewTransaction } from "./helpers";

test("shows a created transaction in the activity feed", { tag: "@mobile" }, async ({ page }, testInfo) => {
  const accountName = `Activity ${testInfo.project.name}-${Date.now()}`;

  await page.goto("/");
  await navigate(page, /accounts/i);
  await page.getByRole("button", { name: "New account" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name").fill(accountName);
  await dialog.getByLabel("Opening balance").fill("100");
  await dialog.getByLabel("Balance as of").fill("2020-01-01");
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Account saved")).toBeVisible();

  await navigate(page, /dashboard/i);
  await openNewTransaction(page);
  const form = page.getByRole("main");
  await form.locator("label", { hasText: accountName }).click();
  await form.getByLabel(/^Amount/).fill("12.50");
  await form.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Transaction saved")).toBeVisible();

  // Settings is reached from the avatar on mobile and the sidebar on desktop.
  await page.goto("/settings");
  await page.getByRole("link", { name: /Activity/ }).click();
  await expect(page.getByRole("heading", { name: "Activity", level: 1 })).toBeVisible();

  const item = page.getByTestId("activity-item").filter({ hasText: accountName }).first();
  await expect(item).toContainText("created a transaction");
  await expect(item).toContainText("Expense");
  await expect(item).toHaveAttribute("data-channel", "web");

  // Filtering by channel keeps web changes and drops MCP ones.
  await page.getByRole("navigation", { name: "Channel" }).getByRole("link", { name: "AI assistants" }).click();
  await expect(page).toHaveURL(/channel=mcp/);
  await expect(page.getByTestId("activity-item").filter({ hasText: accountName })).toHaveCount(0);
  await page.getByRole("navigation", { name: "Channel" }).getByRole("link", { name: "Web app" }).click();
  await expect(page.getByTestId("activity-item").filter({ hasText: accountName }).first()).toBeVisible();
});
