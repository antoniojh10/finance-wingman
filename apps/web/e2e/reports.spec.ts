import { expect, test } from "./fixtures";
import { navigate, seedExpenseHistory } from "./helpers";

const CATEGORIES = [
  { name: "Groceries", base: 400 },
  { name: "Dining", base: 200 },
  { name: "Transport", base: 120 },
];

const legendLabel = "Categories (press to hide or show)";

test("reports the spending by month, from periods to the transactions behind a cell", async ({ page }, testInfo) => {
  const suffix = `${testInfo.project.name}-${Date.now()}`;
  await page.goto("/");
  const { categories } = await seedExpenseHistory(page, {
    suffix,
    currency: "EUR",
    months: 12,
    categories: CATEGORIES.map((c) => ({ name: `${c.name} ${suffix}`, base: c.base })),
  });
  const [groceries, dining] = categories.map((c) => c.name);

  await page.goto("/reports?currency=EUR&months=12");
  const periods = page.getByRole("navigation", { name: "Period" });
  await expect(page.getByTestId("reports-chart")).toBeVisible();
  await expect(page.getByTestId("comparison-total")).toBeVisible();
  await expect(periods.getByRole("link", { name: "12 months" })).toHaveAttribute("aria-current", "page");

  // Three months: the period link and the table's month columns follow.
  await periods.getByRole("link", { name: "3 months" }).click();
  await expect(page).toHaveURL(/months=3/);
  await expect(periods.getByRole("link", { name: "3 months" })).toHaveAttribute("aria-current", "page");
  await expect(page.getByTestId("reports-table").getByRole("row", { name: groceries })).toBeVisible();

  // Hiding a category removes its segments from the chart; showing it again brings them back.
  const legend = page.getByRole("group", { name: legendLabel });
  const segments = page.getByTestId("reports-chart").locator("[data-segment]");
  const segmentsBefore = await segments.count();
  const diningButton = legend.getByRole("button", { name: dining });
  await expect(diningButton).toHaveAttribute("aria-pressed", "true");
  await diningButton.click();
  await expect(diningButton).toHaveAttribute("aria-pressed", "false");
  await expect.poll(() => segments.count()).toBeLessThan(segmentsBefore);
  await diningButton.click();
  await expect(diningButton).toHaveAttribute("aria-pressed", "true");
  await expect.poll(() => segments.count()).toBe(segmentsBefore);

  // Share of the month instead of amounts.
  const scale = page.getByRole("navigation", { name: "Scale" });
  await scale.getByRole("link", { name: "% of month" }).click();
  await expect(page).toHaveURL(/scale=share/);
  await expect(scale.getByRole("link", { name: "% of month" })).toHaveAttribute("aria-current", "page");
  await expect(page.getByTestId("reports-chart")).toBeVisible();

  // A table cell leads to the expenses of that category and month.
  await page.getByTestId("reports-table").getByRole("row", { name: groceries }).getByRole("link").first().click();
  await expect(page).toHaveURL(/\/transactions\?.*category_id=.*type=expense|\/transactions\?.*type=expense.*category_id=/);
  // Seeded descriptions are "<category> <months ago>": the list shows only this category's.
  await expect(page.getByText(new RegExp(`^${groceries} \\d+$`)).first()).toBeVisible();
  await expect(page.getByText(new RegExp(`^${dining} \\d+$`))).toHaveCount(0);

  // The dashboard card shows the same chart and links back to the report.
  await navigate(page, /dashboard/i);
  const trend = page.getByTestId("spending-trend-card");
  await expect(trend.getByTestId("reports-chart")).toBeVisible();
  await trend.getByRole("link", { name: "See report" }).click();
  await expect(page).toHaveURL(/\/reports/);
});

test("reviews the reports on a phone, reached from settings", { tag: "@mobile" }, async ({ page }, testInfo) => {
  const suffix = `${testInfo.project.name}-${Date.now()}`;
  await page.goto("/");
  await seedExpenseHistory(page, {
    suffix,
    currency: "EUR",
    months: 6,
    categories: CATEGORIES.map((c) => ({ name: `${c.name} ${suffix}`, base: c.base })),
  });

  // Reports has no tab bar entry: it is opened from settings.
  await navigate(page, /settings/i);
  await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
  await navigate(page, /reports/i);
  await expect(page.getByRole("heading", { name: "Reports", level: 1 })).toBeVisible();

  const currencies = page.getByRole("navigation", { name: "Currency" });
  if ((await currencies.count()) > 0) {
    await currencies.getByRole("link", { name: "EUR" }).click();
  }
  await expect(page.getByTestId("reports-chart")).toBeVisible();

  const periods = page.getByRole("navigation", { name: "Period" });
  await periods.getByRole("link", { name: "3 months" }).click();
  await expect(page).toHaveURL(/months=3/);
  await expect(periods.getByRole("link", { name: "3 months" })).toHaveAttribute("aria-current", "page");
  // Both layouts render an average cell; exactly one of them is visible at this width.
  await expect(page.getByTestId("comparison-total").getByTestId(/^comparison-average/).filter({ visible: true })).toHaveCount(1);
});
