import { expect, test } from "@playwright/test";

import { e2eUser } from "../playwright.config";
import { codeFrom, linkFrom, navigate, openNewTransaction, requestLogin, signIn } from "./helpers";

test.describe.configure({ mode: "serial" });

test("protected pages redirect to sign-in", async ({ page }) => {
  await page.goto("/transactions");
  await expect(page).toHaveURL(/\/login/);
});

test("rejects a wrong code", async ({ page }) => {
  const text = await requestLogin(page, e2eUser);
  const wrong = codeFrom(text) === "000000" ? "111111" : "000000";
  await page.getByLabel(/6-digit code/i).fill(wrong);
  await page.getByRole("button", { name: /^sign in$/i }).click();
  await expect(page.getByText(/that code is invalid or has expired/i)).toBeVisible();
});

test("signs in with the magic link", async ({ page }) => {
  const text = await requestLogin(page, e2eUser);
  await page.goto(linkFrom(text).replace(/^https?:\/\/[^/]+/, ""));
  await page.getByRole("button", { name: /continue/i }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();
});

test("manages accounts, categories and transactions", async ({ page }, testInfo) => {
  const suffix = `${testInfo.project.name}-${Date.now()}`;
  const accountName = `Checking ${suffix}`;
  const categoryName = `Food ${suffix}`;
  const description = `Groceries ${suffix}`;

  await signIn(page, e2eUser);

  // Account
  await navigate(page, /accounts/i);
  await page.getByRole("button", { name: "New account" }).click();
  const accountDialog = page.getByRole("dialog");
  await accountDialog.getByLabel("Name").fill(accountName);
  await accountDialog.getByLabel("Currency").selectOption("MXN");
  await accountDialog.getByLabel("Opening balance").fill("1000");
  await accountDialog.getByRole("button", { name: "Save" }).click();
  await expect(page.getByText("Account saved")).toBeVisible();
  const accountRow = page.getByTestId("account-row").filter({ hasText: accountName });
  await expect(accountRow).toContainText("MX$1,000.00");

  // Category
  await navigate(page, /categories/i);
  await page.getByRole("button", { name: "New category" }).first().click();
  await page.getByRole("dialog").getByLabel("Name").fill(categoryName);
  await page.getByRole("dialog").getByRole("button", { name: "Save" }).click();
  await expect(page.getByTestId("category-row").filter({ hasText: categoryName })).toBeVisible();

  // Transaction from the dashboard
  await navigate(page, /dashboard/i);
  await openNewTransaction(page);
  const txForm = page.getByRole("main");
  await txForm.getByLabel("Account").selectOption({ label: `${accountName} (MXN)` });
  await txForm.getByLabel(/^Amount/).fill("abc");
  await txForm.getByRole("button", { name: "Save" }).click();
  await expect(txForm.getByText(/positive amount/i)).toBeVisible();
  await txForm.getByLabel(/^Amount/).fill("250.50");
  await txForm.getByLabel("Category").selectOption({ label: categoryName });
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
  await page.getByTestId("account-row").filter({ hasText: accountName }).getByRole("button", { name: "Delete" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
  await expect(page.getByText("Account deleted")).toBeVisible();
});

test("switches language and signs out", async ({ page }) => {
  await signIn(page, e2eUser);
  await navigate(page, /settings/i);
  await expect(page.getByTestId("mcp-url")).toHaveText("http://localhost:8081/mcp");

  await page.getByLabel("Language").selectOption("es");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("heading", { name: "Ajustes" })).toBeVisible();
  await expect(page.getByRole("navigation", { name: "Main" }).first()).toContainText("Movimientos");

  // Restore English for other tests.
  await page.getByLabel("Idioma").selectOption("en");
  await page.getByRole("button", { name: "Guardar" }).click();
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();

  await page.getByRole("main").getByRole("button", { name: "Sign out" }).click();
  await expect(page).toHaveURL(/\/login/);
  await page.goto("/");
  await expect(page).toHaveURL(/\/login/);
});
