import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { Basket, Breakdown, CompareRow, Comparison, OverviewQuery, PriceCell } from "@/lib/groceries-insights";
import { renderWithIntl } from "@/test/render";

import { GroceriesOverview } from "./overview";

const query: OverviewQuery = { by: "group", level: "product", basket: "comparable" };

const breakdown: Breakdown = {
  assigned_minor: 9000,
  average_receipt_minor: 5000,
  by_group: [
    { color: "#12a150", default_key: null, group_id: "g1", is_food: true, name: "Produce", share_pct: 60, total_minor: 6000 },
    { color: "#2f8cff", default_key: null, group_id: "g2", is_food: false, name: "Cleaning", share_pct: 30, total_minor: 3000 },
    { color: null, default_key: null, group_id: null, is_food: null, name: null, share_pct: 10, total_minor: 1000 },
  ],
  by_store: [{ name: "Mercadona", receipt_count: 2, store_id: "s1", total_minor: 10000 }],
  coverage_pct: 90,
  currency: "EUR",
  food_minor: 6000,
  groceries_category_minor: 9500,
  lines_total_minor: 10000,
  non_food_minor: 3000,
  receipt_count: 2,
  top_products: [{ compare_unit: "kg", name: "Chicken", product_id: "p1", quantity: 2.5, spend_minor: 1725 }],
  total_minor: 10000,
  unassigned_minor: 1000,
};

const cell = (storeId: string, price: number, overrides: Partial<PriceCell> = {}): PriceCell => ({
  brand: "Hacendado",
  display_unit_price: price,
  observed_on: "2026-10-01",
  pack_label: "1 L",
  pack_size: 1,
  source: "receipt",
  stale: false,
  store_id: storeId,
  unit_price_minor: price,
  variant_id: "v1",
  ...overrides,
});

const row = (name: string, cells: Record<string, PriceCell>, overrides: Partial<CompareRow> = {}): CompareRow => ({
  best_store_id: null,
  brand: null,
  cells,
  compare_unit: "l",
  display_multiplier: 1,
  max_diff_pct: null,
  product_id: name,
  product_name: name,
  ...overrides,
});

const comparison: Comparison = {
  currency: "EUR",
  stores: [
    { id: "s1", name: "Mercadona" },
    { id: "s2", name: "Lidl" },
  ],
  rows: [
    row("Milk", { s1: cell("s1", 95), s2: cell("s2", 80, { stale: true, source: "price_check" }) }, { best_store_id: "s2", max_diff_pct: 18.75 }),
    row("Eggs", { s1: cell("s1", 270) }, { compare_unit: "unit", display_multiplier: 12 }),
  ],
};

const basket: Basket = {
  best_single_store: { cost_minor: 8000, name: "Lidl", store_id: "s2" },
  currency: "EUR",
  paid_minor: 9000,
  products: [{ compare_unit: "l", display_multiplier: 1, monthly_quantity: 8, name: "Milk", paid_minor: 9000, product_id: "Milk" }],
  split: { cost_minor: 7500, lines: [], unpriced_products: [] },
  stores: [
    { complete: true, cost_minor: 8000, missing_products: [], name: "Lidl", priced_count: 1, store_id: "s2" },
    { complete: true, cost_minor: 9500, missing_products: [], name: "Mercadona", priced_count: 1, store_id: "s1" },
    { complete: false, cost_minor: 2000, missing_products: [{ name: "Rice", product_id: "r" }], name: "Aldi", priced_count: 0, store_id: "s3" },
  ],
};

function renderOverview(props: Partial<React.ComponentProps<typeof GroceriesOverview>> = {}, locale: "en" | "es" = "en") {
  return renderWithIntl(
    <GroceriesOverview
      query={query}
      month="2026-10"
      currencies={["EUR"]}
      currency="EUR"
      breakdown={breakdown}
      comparison={comparison}
      basket={basket}
      basketMonths={3}
      {...props}
    />,
    { locale },
  );
}

describe("GroceriesOverview", () => {
  it("shows the KPIs: receipts vs category, count, non-food and coverage", () => {
    renderOverview();
    const receipts = within(screen.getByTestId("kpi-receipts"));
    expect(receipts.getByText("€100.00")).toBeInTheDocument();
    expect(receipts.getByText(/Groceries category/)).toHaveTextContent("€95.00");
    expect(receipts.getByText(/\+/)).toHaveTextContent("+€5.00");
    expect(screen.getByTestId("kpi-count")).toHaveTextContent("2");
    expect(screen.getByTestId("kpi-count")).toHaveTextContent("€50.00");
    expect(screen.getByTestId("kpi-non-food")).toHaveTextContent("€30.00");
    expect(screen.getByTestId("kpi-non-food")).toHaveTextContent("30% of the lines");
    const coverage = within(screen.getByTestId("kpi-coverage"));
    expect(coverage.getByText("90%")).toBeInTheDocument();
    expect(coverage.getByRole("link", { name: "Review receipts" })).toHaveAttribute("href", "/groceries/receipts?month=2026-10");
  });

  it("tells when there is no Groceries category to compare with", () => {
    renderOverview({ breakdown: { ...breakdown, groceries_category_minor: null, unassigned_minor: 0 } });
    expect(screen.getByText("No Groceries category to compare with")).toBeInTheDocument();
    expect(screen.getByText("Every line is assigned")).toBeInTheDocument();
  });

  it("draws the breakdown by group with non-food tagged and unassigned last", () => {
    renderOverview();
    const rows = screen.getAllByTestId("breakdown-row");
    expect(rows.map((r) => r.textContent)).toEqual([
      expect.stringContaining("Produce"),
      expect.stringContaining("Cleaning"),
      expect.stringContaining("Unassigned"),
    ]);
    expect(within(rows[1]).getByText("Non-food")).toBeInTheDocument();
    expect(within(rows[0]).queryByText("Non-food")).not.toBeInTheDocument();
  });

  it("translates default groups and keeps custom ones as stored", () => {
    renderOverview(
      { breakdown: { ...breakdown, by_group: [{ ...breakdown.by_group[0], default_key: "produce" }, breakdown.by_group[1], breakdown.by_group[2]] } },
      "es",
    );
    const rows = screen.getAllByTestId("breakdown-row");
    expect(rows[0]).toHaveTextContent("Fruta y verdura");
    expect(rows[1]).toHaveTextContent("Cleaning");
  });

  it("toggles the breakdown to stores through a link", () => {
    renderOverview({ query: { ...query, by: "store" } });
    expect(screen.getByRole("heading", { name: "By store" })).toBeInTheDocument();
    expect(screen.getByText("2 receipts")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Group" })).toHaveAttribute("href", "/groceries?month=2026-10&currency=EUR");
    expect(screen.getByRole("link", { name: "Store" })).toHaveAttribute("aria-current", "page");
  });

  it("lists the top products with quantity and spend", () => {
    renderOverview();
    const table = within(screen.getByTestId("groceries-top-products"));
    expect(table.getByText("Chicken")).toBeInTheDocument();
    expect(table.getByText("2.5 kg")).toBeInTheDocument();
    expect(table.getByText("€17.25")).toBeInTheDocument();
  });

  it("renders the matrix: best highlighted, worst muted, gaps and stale tagged", () => {
    renderOverview();
    const rows = screen.getAllByTestId("compare-row");
    expect(rows).toHaveLength(2);
    const milk = rows[0].querySelectorAll("td");
    expect(milk[0]).toHaveAttribute("data-tone", "worst");
    expect(milk[0]).toHaveTextContent("€0.95");
    expect(milk[1]).toHaveAttribute("data-tone", "best");
    expect(milk[1]).toHaveAttribute("data-stale", "true");
    expect(milk[1]).toHaveTextContent("Old");
    expect(milk[1]).toHaveAttribute("title", "Hacendado · 1 L · Oct 1, 2026 · Price check");
    expect(milk[2]).toHaveTextContent("+19%");
    const eggs = rows[1].querySelectorAll("td");
    expect(eggs[1]).toHaveTextContent("No data");
    expect(rows[1]).toHaveTextContent("per 12 units");
    expect(rows[0]).toHaveTextContent("per L");
  });

  it("switches the matrix level through links and explains an empty brand level", () => {
    renderOverview({ query: { ...query, level: "brand" }, comparison: { ...comparison, rows: [] } });
    expect(screen.getByRole("link", { name: "Cheapest of the product" })).toHaveAttribute("href", "/groceries?month=2026-10&currency=EUR");
    expect(screen.getByRole("link", { name: "Same brand" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByTestId("compare-empty")).toHaveTextContent("No brand is priced at two or more stores yet.");
  });

  it("shows the basket by store with the missing stores hidden by default", () => {
    renderOverview();
    const rows = screen.getAllByTestId("basket-row");
    expect(rows.map((r) => r.textContent)).toEqual([expect.stringContaining("Lidl"), expect.stringContaining("Mercadona")]);
    expect(rows[0]).toHaveTextContent("Cheapest");
    expect(screen.getByText("1 store is hidden because it lacks prices.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "All" })).toHaveAttribute("href", "/groceries?month=2026-10&currency=EUR&basket=all");
  });

  it("includes incomplete stores with their missing count when showing all", () => {
    renderOverview({ query: { ...query, basket: "all" } });
    const rows = screen.getAllByTestId("basket-row");
    expect(rows).toHaveLength(3);
    expect(rows[2]).toHaveTextContent("Aldi");
    expect(rows[2]).toHaveTextContent("1 price missing");
  });

  it("compares what was paid with the best store and the split basket", () => {
    renderOverview();
    expect(screen.getByTestId("savings-paid")).toHaveTextContent("€90.00");
    expect(screen.getByTestId("savings-bestStore")).toHaveTextContent("All at Lidl");
    expect(screen.getByTestId("savings-bestStore")).toHaveTextContent("€80.00");
    expect(screen.getByTestId("savings-bestStore")).toHaveTextContent("€10.00");
    expect(screen.getByTestId("savings-split")).toHaveTextContent("€75.00");
    expect(screen.getByTestId("savings-split")).toHaveTextContent("€15.00");
  });

  it("shows the currency switch only with several currencies", () => {
    const { unmount } = renderOverview();
    expect(screen.queryByRole("navigation", { name: "Currency" })).not.toBeInTheDocument();
    unmount();
    renderOverview({ currencies: ["EUR", "USD"] });
    const nav = within(screen.getByRole("navigation", { name: "Currency" }));
    expect(nav.getByRole("link", { name: "USD" })).toHaveAttribute("href", "/groceries?month=2026-10&currency=USD");
    expect(nav.getByRole("link", { name: "EUR" })).toHaveAttribute("aria-current", "page");
  });

  it("navigates months keeping the currency", () => {
    renderOverview({ currencies: ["EUR", "USD"], currency: "USD", breakdown: { ...breakdown, currency: "USD" } });
    expect(screen.getByRole("link", { name: "Previous month" })).toHaveAttribute("href", "/groceries?month=2026-09&currency=USD");
  });

  it("shows empty states per section when the month, matrix and basket have no data", () => {
    renderOverview({ breakdown: undefined, comparison: undefined, basket: undefined });
    expect(screen.getByTestId("spend-empty")).toHaveTextContent("No receipts in this month for this currency.");
    expect(screen.getByTestId("compare-empty")).toBeInTheDocument();
    expect(screen.getByTestId("basket-empty")).toBeInTheDocument();
  });

  it("shows a single empty state, in Spanish too, when there is no data at all", () => {
    renderOverview({ currencies: [], currency: undefined, breakdown: undefined, comparison: undefined, basket: undefined });
    expect(screen.getByTestId("overview-empty")).toHaveTextContent("No grocery data yet.");
    expect(screen.getByRole("link", { name: "Go to receipts" })).toHaveAttribute("href", "/groceries/receipts");
  });

  it("renders in Spanish", () => {
    renderWithIntl(
      <GroceriesOverview query={query} month="2026-10" currencies={["EUR"]} currency="EUR" breakdown={breakdown} comparison={comparison} basket={basket} basketMonths={3} />,
      { locale: "es" },
    );
    expect(screen.getByRole("heading", { name: "Comparar súpers" })).toBeInTheDocument();
    expect(screen.getAllByText("Sin dato").length).toBeGreaterThan(0);
  });
});
