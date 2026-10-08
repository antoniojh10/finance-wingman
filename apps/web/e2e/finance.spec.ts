import { expect, test, type Page } from "@playwright/test";

import { navigate, openCategories, openNewTransaction } from "./helpers";

test.describe.configure({ mode: "serial" });

test("manages accounts, categories and transactions", { tag: "@mobile" }, async ({ page }, testInfo) => {
  const suffix = `${testInfo.project.name}-${Date.now()}`;
  const accountName = `Checking ${suffix}`;
  const categoryName = `Food ${suffix}`;
  const description = `Groceries ${suffix}`;

  await page.goto("/");

  // Account
  await navigate(page, /accounts/i);
  await page.getByRole("button", { name: "New account" }).click();
  const accountDialog = page.getByRole("dialog");
  await accountDialog.getByLabel("Name").fill(accountName);
  await accountDialog.getByLabel("Currency").selectOption("MXN");
  await accountDialog.getByLabel("Opening balance").fill("1000");
  await expect(accountDialog.getByLabel("Opening balance")).toHaveValue("1 000");
  // Anchor the opening balance in the past: transactions on or before that
  // day are already part of it, so today's expense must come after it.
  await accountDialog.getByLabel("Balance as of").fill("2020-01-01");
  await accountDialog.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Account saved")).toBeVisible();
  const accountRow = page.getByTestId("account-row").filter({ hasText: accountName });
  await expect(accountRow).toContainText("MX$1,000.00");

  // Category
  await openCategories(page);
  await page.getByRole("button", { name: "New category" }).first().click();
  await page.getByRole("dialog").getByLabel("Name").fill(categoryName);
  await page.getByRole("dialog").getByRole("button", { name: "Save" }).click();
  await expect(page.getByTestId("category-row").filter({ hasText: categoryName })).toBeVisible();

  // Transaction from the dashboard
  await navigate(page, /dashboard/i);
  await expect(page).toHaveURL(/\/$/);
  await openNewTransaction(page);
  const txForm = page.getByRole("main");
  await txForm.locator("label", { hasText: accountName }).click();
  await txForm.getByLabel(/^Amount/).fill("0");
  await txForm.getByRole("button", { name: "Save" }).click();
  await expect(txForm.getByText(/positive amount/i)).toBeVisible();
  await txForm.getByLabel(/^Amount/).fill("250.50");
  await txForm.locator("label", { hasText: categoryName }).click();
  await txForm.getByLabel("Description").fill(description);
  await txForm.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Transaction saved")).toBeVisible();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByTestId("transaction-row").filter({ hasText: description })).toContainText("−MX$250.50");

  // The balance reflects the expense.
  await navigate(page, /accounts/i);
  await expect(page.getByTestId("account-row").filter({ hasText: accountName })).toContainText("MX$749.50");

  // Edit and delete from the transactions page.
  await navigate(page, /transactions/i);
  await page.getByRole("searchbox", { name: "Search" }).fill(description);
  await page.getByRole("button", { name: "Filter" }).click();
  const row = page.getByTestId("transaction-row").filter({ hasText: description });
  await expect(row).toHaveCount(1);
  await row.getByRole("button", { name: "Actions" }).click();
  await page.getByRole("menuitem", { name: "Edit" }).click();
  await page.getByRole("dialog").getByLabel(/^Amount/).fill("300");
  await page.getByRole("dialog").getByRole("button", { name: "Save" }).click();
  await expect(row).toContainText("−MX$300.00");

  await row.getByRole("button", { name: "Actions" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
  await expect(page.getByText("Transaction deleted")).toBeVisible();
  await expect(row).toHaveCount(0);

  // Clean up: the account has no transactions left, so it can be deleted.
  await navigate(page, /accounts/i);
  await page.getByTestId("account-row").filter({ hasText: accountName }).getByRole("button", { name: "Actions" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
  await expect(page.getByText("Account deleted")).toBeVisible();
});

test("creates, edits and pauses a subscription", async ({ page }, testInfo) => {
  const suffix = `${testInfo.project.name}-${Date.now()}`;
  const accountName = `Cards ${suffix}`;
  const name = `Netflix ${suffix}`;

  await page.goto("/");

  await navigate(page, /accounts/i);
  await page.getByRole("button", { name: "New account" }).click();
  await page.getByRole("dialog").getByLabel("Name").fill(accountName);
  await page.getByRole("dialog").getByLabel("Currency").selectOption("MXN");
  await page.getByRole("dialog").getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Account saved")).toBeVisible();

  // Create
  await navigate(page, /subscriptions/i);
  await expect(page.getByRole("heading", { name: "Subscriptions" })).toBeVisible();
  await page.getByRole("button", { name: "New subscription" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(dialog.getByText("This field is required.").first()).toBeVisible();
  await dialog.getByLabel("Name").fill(name);
  await dialog.locator("label", { hasText: accountName }).click();
  await dialog.getByLabel(/^Amount/).fill("199");
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Subscription saved")).toBeVisible();
  const row = page.getByTestId("recurring-row").filter({ hasText: name });
  await expect(row).toContainText("−MX$199.00");
  await expect(row).toContainText("Every month");
  await expect(page.getByTestId("summary-MXN-expenses")).toBeVisible();

  // Edit
  await row.getByRole("button", { name: "Actions" }).click();
  await page.getByRole("menuitem", { name: "Edit" }).click();
  await page.getByRole("dialog").getByLabel(/^Amount/).fill("249");
  await page.getByRole("dialog").getByRole("button", { name: "Save" }).click();
  await expect(row).toContainText("−MX$249.00");

  // Pause
  await row.getByRole("button", { name: "Actions" }).click();
  await page.getByRole("menuitem", { name: "Pause" }).click();
  await expect(page.getByText("Subscription paused")).toBeVisible();
  await expect(row).toContainText("Paused");

  // Clean up: cancel it so it stops counting.
  await row.getByRole("button", { name: "Actions" }).click();
  await page.getByRole("menuitem", { name: "Cancel subscription" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Cancel subscription" }).click();
  await expect(row).toContainText("Cancelled");
});

test("registers a subscription payment from the dashboard", async ({ page }, testInfo) => {
  const suffix = `${testInfo.project.name}-${Date.now()}`;
  const accountName = `Pay ${suffix}`;
  const name = `Gym ${suffix}`;

  await page.goto("/");

  await navigate(page, /accounts/i);
  await page.getByRole("button", { name: "New account" }).click();
  await page.getByRole("dialog").getByLabel("Name").fill(accountName);
  await page.getByRole("dialog").getByLabel("Currency").selectOption("MXN");
  await page.getByRole("dialog").getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Account saved")).toBeVisible();

  // The first due date defaults to today, so the subscription is pending.
  await navigate(page, /subscriptions/i);
  await page.getByRole("button", { name: "New subscription" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name").fill(name);
  await dialog.locator("label", { hasText: accountName }).click();
  await dialog.getByLabel(/^Amount/).fill("300");
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Subscription saved")).toBeVisible();
  const listRow = page.getByTestId("recurring-row").filter({ hasText: name });
  await expect(listRow).toContainText("Pending");

  // Register from the dashboard card, adjusting the amount.
  await navigate(page, /dashboard/i);
  const card = page.getByTestId("subscriptions-card");
  const upcoming = card.getByTestId("upcoming-row").filter({ hasText: name });
  await expect(upcoming).toContainText("Pending");
  await upcoming.getByRole("button", { name: "Register payment" }).click();
  const confirm = page.getByRole("dialog");
  await expect(confirm.getByLabel(/^Amount/)).toHaveValue("300.00");
  await confirm.getByLabel(/^Amount/).fill("310");
  await confirm.getByRole("button", { name: "Register payment" }).click();
  await expect(page.getByText("Payment registered")).toBeVisible();
  await expect(upcoming).toContainText("Paid");
  await expect(upcoming.getByRole("button", { name: "Register payment" })).toHaveCount(0);

  // The status changed on the subscriptions page and the transaction shows its subscription.
  await navigate(page, /subscriptions/i);
  await expect(listRow).toContainText("Paid");
  await navigate(page, /transactions/i);
  await expect(page.getByTestId("transaction-row").filter({ hasText: `Subscription: ${name}` })).toContainText("−MX$310.00");

  // Clean up: cancel it so it stops counting.
  await navigate(page, /subscriptions/i);
  await listRow.getByRole("button", { name: "Actions" }).click();
  await page.getByRole("menuitem", { name: "Cancel subscription" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Cancel subscription" }).click();
  await expect(listRow).toContainText("Cancelled");
});

/** Waits for a toast and then for it to go away: stacked toasts break strict locators and cover header links. */
async function expectToast(page: Page, message: string) {
  const toast = page.getByText(message);
  await expect(toast).toBeVisible();
  await page.mouse.move(0, 0);
  await expect(toast).toHaveCount(0, { timeout: 15_000 });
}

test("accepts a detected subscription and sees its linked transactions", async ({ page }, testInfo) => {
  const suffix = `${testInfo.project.name}-${Date.now()}`;
  const accountName = `Detect ${suffix}`;
  const description = `Streaming ${suffix}`;
  const name = `Streaming plan ${suffix}`;

  await page.goto("/");

  await navigate(page, /accounts/i);
  await page.getByRole("button", { name: "New account" }).click();
  await page.getByRole("dialog").getByLabel("Name").fill(accountName);
  await page.getByRole("dialog").getByLabel("Currency").selectOption("MXN");
  await page.getByRole("dialog").getByRole("button", { name: "Save" }).click();
  await expectToast(page, "Account saved");

  // Three monthly payments of the same amount on the 15th, 1 to 3 months ago.
  const now = new Date();
  for (const monthsAgo of [3, 2, 1]) {
    const date = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - monthsAgo, 15)).toISOString().slice(0, 10);
    await navigate(page, /dashboard/i);
    await openNewTransaction(page);
    const form = page.getByRole("main");
    await form.locator("label", { hasText: accountName }).click();
    await form.getByLabel(/^Amount/).fill("120");
    await form.getByLabel("Date", { exact: true }).fill(date);
    await form.getByLabel("Description").fill(description);
    await form.getByRole("button", { name: "Save" }).click();
    await expectToast(page, "Transaction saved");
  }

  // The dashboard card points at the suggestions.
  await navigate(page, /dashboard/i);
  await expect(page.getByTestId("subscriptions-card").getByTestId("suggestions-link")).toContainText(/suggestion/);

  // Accept the suggestion with a clean name.
  await navigate(page, /subscriptions/i);
  const suggestion = page.getByTestId("suggestion-row").filter({ hasText: accountName });
  await expect(suggestion).toContainText("3 matching transactions");
  await suggestion.getByRole("button", { name: "Add" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByLabel("Amount (MXN)")).toHaveValue("120.00");
  await dialog.getByLabel("Name").fill(name);
  await dialog.getByRole("button", { name: "Add subscription" }).click();
  await expectToast(page, "Subscription added");
  await expect(suggestion).toHaveCount(0);
  const listRow = page.getByTestId("recurring-row").filter({ hasText: name });
  await expect(listRow).toContainText("−MX$120.00");

  // The matching transactions are linked to the new subscription.
  await navigate(page, /transactions/i);
  await page.getByRole("searchbox", { name: "Search" }).fill(description);
  await page.getByRole("button", { name: "Filter" }).click();
  await expect(page.getByTestId("transaction-row").filter({ hasText: `Subscription: ${name}` })).toHaveCount(3);

  // Clean up: cancel it so it stops counting.
  await navigate(page, /subscriptions/i);
  await listRow.getByRole("button", { name: "Actions" }).click();
  await page.getByRole("menuitem", { name: "Cancel subscription" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Cancel subscription" }).click();
  await expect(listRow).toContainText("Cancelled");
});

test("links and unlinks a transaction to a subscription", async ({ page }, testInfo) => {
  const suffix = `${testInfo.project.name}-${Date.now()}`;
  const accountName = `Link ${suffix}`;
  const name = `Hulu ${suffix}`;

  await page.goto("/");

  await navigate(page, /accounts/i);
  await page.getByRole("button", { name: "New account" }).click();
  await page.getByRole("dialog").getByLabel("Name").fill(accountName);
  await page.getByRole("dialog").getByLabel("Currency").selectOption("MXN");
  await page.getByRole("dialog").getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Account saved")).toBeVisible();

  await navigate(page, /subscriptions/i);
  await page.getByRole("button", { name: "New subscription" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Name").fill(name);
  await dialog.locator("label", { hasText: accountName }).click();
  await dialog.getByLabel(/^Amount/).fill("150");
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Subscription saved")).toBeVisible();

  // A new transaction that looks like the subscription offers to link it.
  await navigate(page, /dashboard/i);
  await openNewTransaction(page);
  const form = page.getByRole("main");
  await form.locator("label", { hasText: accountName }).click();
  await form.getByLabel(/^Amount/).fill("150");
  await form.getByLabel("Description").fill(name);
  await form.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Transaction saved")).toBeVisible();
  await page.getByRole("button", { name: `Link to ${name}?` }).click();
  await expect(page.getByText(`Linked to ${name}`)).toBeVisible();

  await navigate(page, /transactions/i);
  await page.getByRole("searchbox", { name: "Search" }).fill(name);
  await page.getByRole("button", { name: "Filter" }).click();
  const row = page.getByTestId("transaction-row").filter({ hasText: name });
  await expect(row).toContainText(`Subscription: ${name}`);

  // Unlink from the edit dialog.
  await row.getByRole("button", { name: "Actions" }).click();
  await page.getByRole("menuitem", { name: "Edit" }).click();
  const edit = page.getByRole("dialog");
  await expect(edit.getByText(`Pays ${name}`)).toBeVisible();
  await edit.getByRole("button", { name: "Unlink" }).click();
  await expect(page.getByText("Unlinked from the subscription")).toBeVisible();
  await expect(edit.getByRole("button", { name: "Link", exact: true })).toBeVisible();
  await edit.getByRole("button", { name: "Cancel" }).click();
  await expect(row).not.toContainText("Subscription:");

  // Clean up: cancel the subscription so it stops counting.
  await navigate(page, /subscriptions/i);
  const listRow = page.getByTestId("recurring-row").filter({ hasText: name });
  await listRow.getByRole("button", { name: "Actions" }).click();
  await page.getByRole("menuitem", { name: "Cancel subscription" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Cancel subscription" }).click();
  await expect(listRow).toContainText("Cancelled");
});
