import { describe, expect, it } from "vitest";

import {
  availableCurrencies,
  basketBars,
  basketSavings,
  breakdownBars,
  categoryGap,
  cellTone,
  formatUnitPrice,
  nonFoodShare,
  overviewHref,
  parseOverviewQuery,
  pickCurrency,
  sortCompareRows,
  type Basket,
  type Breakdown,
  type CompareRow,
  type PriceCell,
} from "./groceries-insights";

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

const row = (cells: Record<string, PriceCell>, overrides: Partial<CompareRow> = {}): CompareRow => ({
  best_store_id: null,
  brand: null,
  cells,
  compare_unit: "l",
  display_multiplier: 1,
  max_diff_pct: null,
  product_id: "p1",
  product_name: "Milk",
  ...overrides,
});

const breakdown: Breakdown = {
  assigned_minor: 9000,
  average_receipt_minor: 5000,
  by_group: [
    { color: null, default_key: null, group_id: null, is_food: null, name: null, share_pct: 10, total_minor: 1000 },
    { color: "#12a150", default_key: null, group_id: "g1", is_food: true, name: "Produce", share_pct: 60, total_minor: 6000 },
    { color: "#2f8cff", default_key: null, group_id: "g2", is_food: false, name: "Cleaning", share_pct: 30, total_minor: 3000 },
  ],
  by_store: [
    { name: "Lidl", receipt_count: 1, store_id: "s2", total_minor: 3000 },
    { name: "Mercadona", receipt_count: 1, store_id: "s1", total_minor: 7000 },
  ],
  coverage_pct: 90,
  currency: "EUR",
  food_minor: 6000,
  groceries_category_minor: 9500,
  lines_total_minor: 10000,
  non_food_minor: 3000,
  receipt_count: 2,
  top_products: [],
  total_minor: 10000,
  unassigned_minor: 1000,
};

describe("parseOverviewQuery / overviewHref", () => {
  it("sanitizes invalid values to the defaults", () => {
    expect(parseOverviewQuery({ month: "2026-13", currency: "euro", by: "x", level: "y", basket: "z" })).toEqual({
      month: undefined,
      currency: undefined,
      by: "group",
      level: "product",
      basket: "comparable",
    });
  });

  it("keeps valid values and round-trips them through the href", () => {
    const query = parseOverviewQuery({ month: "2026-09", currency: "eur", by: "store", level: "brand", basket: "all" });
    expect(query).toEqual({ month: "2026-09", currency: "EUR", by: "store", level: "brand", basket: "all" });
    expect(overviewHref(query)).toBe("/groceries?month=2026-09&currency=EUR&by=store&level=brand&basket=all");
  });

  it("leaves defaults out and applies changes", () => {
    const query = parseOverviewQuery({});
    expect(overviewHref(query)).toBe("/groceries");
    expect(overviewHref(query, { by: "store" })).toBe("/groceries?by=store");
  });
});

describe("currencies", () => {
  it("merges and sorts the currencies of several responses", () => {
    expect(availableCurrencies([{ currency: "USD" }], [{ currency: "EUR" }, { currency: "USD" }])).toEqual(["EUR", "USD"]);
  });

  it("falls back to the first when the requested one has no data", () => {
    expect(pickCurrency(["EUR", "USD"], "USD")).toBe("USD");
    expect(pickCurrency(["EUR", "USD"], "GBP")).toBe("EUR");
    expect(pickCurrency([], "EUR")).toBeUndefined();
  });
});

describe("breakdownBars", () => {
  it("orders groups by spend with the unassigned bucket last and scales to the biggest", () => {
    const bars = breakdownBars(breakdown, "group");
    expect(bars.map((bar) => bar.label)).toEqual(["Produce", "Cleaning", null]);
    expect(bars[0].width).toBe(1);
    expect(bars[1].width).toBeCloseTo(0.5);
    expect(bars[2].unassigned).toBe(true);
    expect(bars[1].isFood).toBe(false);
  });

  it("carries the default key of default groups", () => {
    const bars = breakdownBars({ ...breakdown, by_group: [{ ...breakdown.by_group[1], default_key: "produce" }] }, "group");
    expect(bars[0].defaultKey).toBe("produce");
  });

  it("builds store bars with their share of the receipts total", () => {
    const bars = breakdownBars(breakdown, "store");
    expect(bars.map((bar) => [bar.label, bar.share, bar.receiptCount])).toEqual([
      ["Mercadona", 70, 1],
      ["Lidl", 30, 1],
    ]);
  });
});

describe("breakdown figures", () => {
  it("compares the receipts with the Groceries category", () => {
    expect(categoryGap(breakdown)).toBe(500);
    expect(categoryGap({ ...breakdown, groceries_category_minor: null })).toBeNull();
  });

  it("computes the non-food share of the lines", () => {
    expect(nonFoodShare(breakdown)).toBe(30);
    expect(nonFoodShare({ non_food_minor: 0, lines_total_minor: 0 })).toBe(0);
  });
});

describe("cellTone", () => {
  const r = row({ a: cell("a", 100), b: cell("b", 130), c: cell("c", 115) }, { best_store_id: "a" });

  it("marks the cheapest and the most expensive", () => {
    expect(cellTone(r, "a")).toBe("best");
    expect(cellTone(r, "c")).toBe("normal");
    expect(cellTone(r, "b")).toBe("worst");
  });

  it("does not highlight a lone price, equal prices or a missing store", () => {
    expect(cellTone(row({ a: cell("a", 100) }), "a")).toBe("normal");
    expect(cellTone(row({ a: cell("a", 100), b: cell("b", 100) }), "a")).toBe("normal");
    expect(cellTone(r, "zzz")).toBe("normal");
  });
});

describe("sortCompareRows", () => {
  it("puts the biggest gap first and rows with a single price last", () => {
    const rows = [
      row({}, { product_id: "1", product_name: "Eggs", max_diff_pct: null }),
      row({}, { product_id: "2", product_name: "Rice", max_diff_pct: 12 }),
      row({}, { product_id: "3", product_name: "Milk", max_diff_pct: 40 }),
    ];
    expect(sortCompareRows(rows).map((r) => r.product_name)).toEqual(["Milk", "Rice", "Eggs"]);
  });
});

describe("formatUnitPrice", () => {
  it("formats minor units per compare unit in the currency", () => {
    expect(formatUnitPrice(690, "EUR", 2, "en")).toBe("€6.90");
    expect(formatUnitPrice(115.5, "EUR", 2, "en")).toBe("€1.16");
  });
});

describe("basket", () => {
  const basket: Basket = {
    best_single_store: { cost_minor: 8000, name: "Lidl", store_id: "s2" },
    currency: "EUR",
    paid_minor: 10500,
    products: [
      { compare_unit: "l", display_multiplier: 1, monthly_quantity: 8, name: "Milk", paid_minor: 800, product_id: "p1" },
      { compare_unit: "kg", display_multiplier: 1, monthly_quantity: 2, name: "Saffron", paid_minor: 2500, product_id: "p2" },
      { compare_unit: "kg", display_multiplier: 1, monthly_quantity: 5, name: "Rice", paid_minor: 7200, product_id: "p3" },
    ],
    split: {
      cost_minor: 7500,
      lines: [],
      unpriced_products: [{ name: "Saffron", product_id: "p2" }],
    },
    stores: [
      { complete: false, cost_minor: 3000, missing_products: [{ name: "Rice", product_id: "p3" }], name: "Aldi", priced_count: 2, store_id: "s3" },
      { complete: true, cost_minor: 9000, missing_products: [], name: "Mercadona", priced_count: 3, store_id: "s1" },
      { complete: true, cost_minor: 8000, missing_products: [], name: "Lidl", priced_count: 3, store_id: "s2" },
    ],
  };

  it("shows only comparable stores by default, cheapest first and flagged", () => {
    const bars = basketBars(basket, "comparable");
    expect(bars.map((bar) => [bar.name, bar.best])).toEqual([
      ["Lidl", true],
      ["Mercadona", false],
    ]);
    expect(bars[1].width).toBe(1);
  });

  it("adds the incomplete stores last, with their missing count, when asked for all", () => {
    const bars = basketBars(basket, "all");
    expect(bars.map((bar) => bar.name)).toEqual(["Lidl", "Mercadona", "Aldi"]);
    expect(bars[2]).toMatchObject({ complete: false, missing: 1, best: false });
  });

  it("has no best store when no store is comparable", () => {
    const none = { ...basket, stores: [basket.stores[0]] };
    expect(basketBars(none, "comparable")).toEqual([]);
    expect(basketBars(none, "all")[0].best).toBe(false);
  });

  it("compares what was paid, without unpriced products, against best store and split", () => {
    expect(basketSavings(basket)).toEqual([
      { key: "paid", cost: 8000, saving: null },
      { key: "bestStore", storeName: "Lidl", cost: 8000, saving: 0 },
      { key: "split", cost: 7500, saving: 500 },
    ]);
  });

  it("omits the best store row when none prices every product", () => {
    const rows = basketSavings({ ...basket, best_single_store: undefined });
    expect(rows.map((r) => r.key)).toEqual(["paid", "split"]);
  });
});
