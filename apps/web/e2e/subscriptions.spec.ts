import { expect, test } from "./fixtures";
import { apiPort } from "../playwright.config";

test("filters subscriptions from the category breakdown", { tag: "@mobile" }, async ({ page, context }, testInfo) => {
  const suffix = `${testInfo.project.name}-${Date.now()}`;
  const homeName = `Home ${suffix}`;
  const funName = `Fun ${suffix}`;
  const token = (await context.cookies()).find((c) => c.name === "fw_session")?.value;
  if (!token) throw new Error("no session cookie");
  const api = `http://localhost:${apiPort}/api/v1`;
  const call = async (path: string, data: unknown) => {
    const res = await page.request.post(`${api}${path}`, { headers: { Authorization: `Bearer ${token}` }, data });
    expect(res.ok(), `${path}: ${await res.text()}`).toBeTruthy();
    return res.json();
  };

  const account = await call("/accounts", { name: `Subs ${suffix}`, type: "checking", currency: "MXN", initial_balance: 0 });
  const { items: cats } = await call("/categories/batch", {
    items: [
      { name: homeName, kind: "expense", color: "#ffb020" },
      { name: funName, kind: "expense", color: "#f05aa6" },
    ],
  });
  const id = (name: string) => cats.find((c: { id: string; name: string }) => c.name === name).id as string;
  const start = new Date().toISOString().slice(0, 10);
  for (const [name, category, amount] of [
    [`Rent ${suffix}`, homeName, 900000],
    [`Netflix ${suffix}`, funName, 19900],
  ] as const) {
    await call("/recurring", {
      name,
      type: "expense",
      account_id: account.id,
      category_id: id(category),
      amount,
      interval_unit: "month",
      interval_count: 1,
      start_on: start,
    });
  }

  await page.goto("/subscriptions");
  // Grouped by category by default: one card per category.
  await expect(page.getByTestId("recurring-group").filter({ hasText: homeName })).toContainText(`Rent ${suffix}`);
  await expect(page.getByTestId("recurring-group").filter({ hasText: funName })).toContainText(`Netflix ${suffix}`);

  // A breakdown row filters the list and shows the chip.
  const breakdown = page.getByTestId("category-breakdown");
  await breakdown.getByTestId("breakdown-row").filter({ hasText: funName }).click();
  await expect(page).toHaveURL(new RegExp(`category=${id(funName)}`));
  const chip = page.getByTestId("category-chip");
  await expect(chip).toContainText(funName);
  await expect(page.getByTestId("recurring-row").filter({ hasText: `Netflix ${suffix}` })).toBeVisible();
  await expect(page.getByTestId("recurring-row").filter({ hasText: `Rent ${suffix}` })).toHaveCount(0);

  // The chip clears it.
  await chip.getByRole("button", { name: "Clear category filter" }).click();
  await expect(page).not.toHaveURL(/category=/);
  await expect(chip).toBeHidden();
  await expect(page.getByTestId("recurring-row").filter({ hasText: `Rent ${suffix}` })).toBeVisible();

  // Pressing the same row again clears it too, and grouping can be turned off.
  const row = breakdown.getByTestId("breakdown-row").filter({ hasText: homeName });
  await row.click();
  await expect(chip).toContainText(homeName);
  await row.click();
  await expect(chip).toBeHidden();
  await page.getByRole("button", { name: "Group by category" }).click();
  await expect(page).toHaveURL(/group=0/);
  await expect(page.getByTestId("recurring-group")).toHaveCount(0);
});
