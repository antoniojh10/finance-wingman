import type { Page } from "@playwright/test";

import { expect, test } from "./fixtures";
import { groceryApi, openGroceries } from "./helpers";

const tabs = (page: Page) => page.getByRole("navigation", { name: "Groceries sections" });

test("assigns a receipt line and the next receipt of the store matches it", async ({ page }, testInfo) => {
  const suffix = `${testInfo.project.name}-${Date.now()}`;
  const store = `Mercado ${suffix}`;
  const lineText = `LECHE ENT ${suffix}`;
  const product = `Whole milk ${suffix}`;
  await page.goto("/");
  const groceries = groceryApi(page);
  const first = await groceries.createReceipt(store, [
    { text: lineText, amount: 129 },
    { text: `PAN BARRA ${suffix}`, amount: 80 },
  ]);

  // The receipt is in the list, with its lines to assign.
  await page.goto("/groceries/receipts");
  const row = page.getByTestId("receipts-list").getByRole("link", { name: new RegExp(`^Open receipt from ${store},`) });
  await expect(row).toContainText("2 to assign");
  await row.click();
  await expect(page).toHaveURL(new RegExp(`/groceries/receipts/${first.id}`));
  await expect(page.getByRole("heading", { name: `Receipt from ${store}`, level: 2 })).toBeVisible();
  await expect(page.getByText("2 lines to assign")).toBeVisible();

  // Assign one line to a new product.
  const line = page.getByTestId("receipt-line").filter({ hasText: lineText });
  await expect(line).toHaveAttribute("data-status", "unassigned");
  await line.getByRole("button", { name: "Assign" }).click();
  await line.getByRole("button", { name: "New product" }).click();
  await line.getByLabel("Product name").fill(product);
  await line.getByLabel("Brand", { exact: true }).fill("Hacendado");
  await line.getByLabel("Pack size").fill("1");
  await line.getByLabel("Compare by").selectOption("l");
  await line.getByRole("button", { name: "Save assignment" }).click();
  await expect(page.getByText("Line assigned.")).toBeVisible();
  await expect(line).toHaveAttribute("data-status", "assigned");
  await expect(line).toContainText(product);
  await expect(page.getByText("1 line to assign")).toBeVisible();

  // The next receipt of the same store arrives with the line already assigned.
  const second = await groceries.createReceipt(store, [{ text: lineText, amount: 135 }]);
  expect(second.lines[0].product_name).toBe(product);
  await page.goto(`/groceries/receipts/${second.id}`);
  const again = page.getByTestId("receipt-line").filter({ hasText: lineText });
  await expect(again).toHaveAttribute("data-status", "assigned");
  await expect(again).toContainText(product);
  await expect(page.getByText("All lines assigned")).toBeVisible();
});

test("compares the stores on the summary and lists the variants of a product", async ({ page }, testInfo) => {
  const suffix = `${testInfo.project.name}-${Date.now()}`;
  const cheap = `Cheap ${suffix}`;
  const dear = `Dear ${suffix}`;
  const product = `Olive oil ${suffix}`;
  await page.goto("/");
  const groceries = groceryApi(page);
  const first = await groceries.createReceipt(cheap, [{ text: `AOVE 1L ${suffix}`, amount: 500 }]);
  await groceries.assign(first, [{ line: `AOVE 1L ${suffix}`, product, brand: "Hacendado", packSize: 1, compareUnit: "l" }]);
  const second = await groceries.createReceipt(dear, [{ text: `ACEITE OLIVA ${suffix}`, amount: 800 }]);
  await groceries.assign(second, [{ line: `ACEITE OLIVA ${suffix}`, product, brand: "Carbonell", packSize: 1, compareUnit: "l" }]);

  // Summary: the cheapest store of the product is highlighted.
  await page.goto("/groceries?currency=EUR");
  await expect(page.getByRole("heading", { name: "Compare supermarkets" })).toBeVisible();
  const matrix = page.getByTestId("compare-matrix");
  const matrixRow = matrix.getByTestId("compare-row").filter({ hasText: product });
  await expect(matrixRow).toBeVisible();
  const headers = await matrix.getByRole("columnheader").allTextContents();
  const cheapColumn = headers.indexOf(cheap);
  const dearColumn = headers.indexOf(dear);
  expect(cheapColumn).toBeGreaterThan(0);
  expect(dearColumn).toBeGreaterThan(0);
  // Columns: product, one per store, gap. The row header is not a cell.
  const cells = matrixRow.getByRole("cell");
  await expect(cells.nth(cheapColumn - 1)).toHaveAttribute("data-tone", "best");
  await expect(cells.nth(cheapColumn - 1)).toContainText("€5.00");
  await expect(cells.nth(dearColumn - 1)).toHaveAttribute("data-tone", "worst");

  // Products: the product shows its variants, the cheapest first.
  await tabs(page).getByRole("link", { name: "Products" }).click();
  await expect(page).toHaveURL(/\/groceries\/products/);
  await page.getByRole("search").getByLabel("Search products").fill(product);
  await page.getByRole("button", { name: "Search", exact: true }).click();
  await page.getByRole("link", { name: new RegExp(`^${product}`) }).click();
  await expect(page.getByTestId("product-name")).toHaveText(product);
  const variants = page.getByTestId("variant-row");
  await expect(variants).toHaveCount(2);
  await expect(variants.first()).toContainText("Hacendado");
  await expect(variants.first()).toContainText(cheap);
  await expect(variants.first()).toContainText("Best price");
  await expect(variants.nth(1)).toContainText("Carbonell");
  await expect(variants.nth(1)).toContainText(dear);
});

test("opens the receipts from settings on a phone", { tag: "@mobile" }, async ({ page }, testInfo) => {
  const suffix = `${testInfo.project.name}-${Date.now()}`;
  const store = `Tienda ${suffix}`;
  await page.goto("/");
  await groceryApi(page).createReceipt(store, [{ text: `ARROZ ${suffix}`, amount: 199 }]);

  // Groceries has no tab bar entry: it is opened from settings.
  await openGroceries(page);
  await expect(page.getByRole("heading", { name: "Groceries", level: 1 })).toBeVisible();
  await tabs(page).getByRole("link", { name: "Receipts" }).click();
  await expect(page).toHaveURL(/\/groceries\/receipts/);
  await page.getByTestId("receipts-list").getByRole("link", { name: new RegExp(`^Open receipt from ${store},`) }).click();
  await expect(page.getByRole("heading", { name: `Receipt from ${store}`, level: 2 })).toBeVisible();
});
