import { expect, test } from "./fixtures";
import { navigate, openCategories, openNewTransaction } from "./helpers";

test("sets a budget and sees an expense consume it", { tag: "@mobile" }, async ({ page }, testInfo) => {
  const suffix = `${testInfo.project.name}-${Date.now()}`;
  const accountName = `Budget account ${suffix}`;
  const categoryName = `Dining ${suffix}`;

  await page.goto("/");

  await navigate(page, /accounts/i);
  await page.getByRole("button", { name: "New account" }).click();
  const accountDialog = page.getByRole("dialog");
  await accountDialog.getByLabel("Name").fill(accountName);
  await accountDialog.getByLabel("Currency").selectOption("MXN");
  await accountDialog.getByLabel("Opening balance").fill("1000");
  await accountDialog.getByLabel("Balance as of").fill("2020-01-01");
  await accountDialog.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Account saved")).toBeVisible();

  await openCategories(page);
  await page.getByRole("button", { name: "New category" }).first().click();
  await page.getByRole("dialog").getByLabel("Name").fill(categoryName);
  await page.getByRole("dialog").getByRole("button", { name: "Save" }).click();
  await expect(page.getByTestId("category-row").filter({ hasText: categoryName })).toBeVisible();

  // Set the budget for the month.
  await navigate(page, /budgets/i);
  await expect(page.getByRole("heading", { name: "Budgets", level: 1 })).toBeVisible();
  await page.getByRole("button", { name: "Edit budgets" }).click();
  await page.getByTestId("budget-edit-MXN").getByLabel(categoryName, { exact: true }).fill("500");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Budgets saved")).toBeVisible();
  const row = page.getByTestId("budget-MXN").getByTestId("budget-row").filter({ hasText: categoryName });
  await expect(row).toContainText("MX$500.00");
  await expect(row).toContainText("On track");

  // An expense in the category consumes it.
  await openNewTransaction(page);
  const txForm = page.getByRole("main");
  await txForm.locator("label", { hasText: accountName }).click();
  await txForm.getByLabel(/^Amount/).fill("450");
  await txForm.locator("label", { hasText: categoryName }).click();

  // The form warns, without blocking, that the expense leaves the budget nearly spent...
  const hint = txForm.getByTestId("budget-hint");
  await expect(hint).toContainText("this leaves MX$50.00 of the budget");
  // ...or over it, and the warning follows the amount.
  await txForm.getByLabel(/^Amount/).fill("600");
  await expect(hint).toContainText("this goes over budget by MX$100.00");
  await txForm.getByLabel(/^Amount/).fill("450");
  await expect(hint).toContainText("this leaves MX$50.00 of the budget");

  await txForm.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Transaction saved")).toBeVisible();
  await expect(page.getByText(/this leaves MX\$50\.00 of the budget/)).toBeVisible();

  await navigate(page, /budgets/i);
  const consumed = page.getByTestId("budget-MXN").getByTestId("budget-row").filter({ hasText: categoryName });
  await expect(consumed.getByText("Spent").locator("xpath=following-sibling::dd")).toHaveText("MX$450.00");
  await expect(consumed.getByText("Remaining").locator("xpath=following-sibling::dd")).toHaveText("MX$50.00");
  await expect(consumed).toContainText("Near limit");

  // The dashboard card points at the category that is near its budget.
  await navigate(page, /dashboard/i);
  const attention = page.getByTestId("budgets-card").getByTestId("budget-attention-row").filter({ hasText: categoryName });
  await expect(attention).toContainText("Near limit");
});
