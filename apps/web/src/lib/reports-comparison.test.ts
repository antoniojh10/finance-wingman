import { describe, expect, it } from "vitest";

import { buildComparison, deltaTone, heatShare, monthTransactionsHref, type MonthlyCurrency } from "./reports";

const labels = { uncategorized: "Uncategorized", other: "Other" };
const options = { months: 4, includeCurrent: true, labels };

type Cat = MonthlyCurrency["categories"][number];

function category(id: string | null, name: string | null, totals: number[]): Cat {
  return { category_id: id, name, color: id ? `#${id}` : null, archived: false, totals, budgets: totals.map(() => null) };
}

/** Months ending in the partial 2026-10. */
function fixture(categories: Cat[], months = ["2026-07", "2026-08", "2026-09", "2026-10"]): MonthlyCurrency {
  return {
    currency: "EUR",
    minor_units: 2,
    months,
    partial_month: "2026-10",
    categories,
    totals: months.map((_, i) => categories.reduce((s, c) => s + c.totals[i], 0)),
    budgets: months.map(() => null),
  };
}

const sample = fixture([
  category("rent", "Rent", [1000, 1000, 1000, 1000]),
  category("food", "Food", [400, 300, 500, 100]),
  category("fun", "Fun", [50, 100, 30, 10]),
  category(null, null, [20, 0, 10, 5]),
]);

describe("buildComparison", () => {
  it("keeps every category, uncategorized included, instead of folding into Other", () => {
    const many = fixture(Array.from({ length: 9 }, (_, i) => category(`c${i}`, `Cat ${i}`, [10 + i, 10, 10, 10])));
    const grid = buildComparison(many, options);
    expect(grid.rows).toHaveLength(9);
    expect(grid.rows.some((r) => r.key === "other")).toBe(false);

    const withUncategorized = buildComparison(sample, options);
    expect(withUncategorized.rows.map((r) => r.key)).toEqual(["rent", "food", "fun", "uncategorized"]);
    expect(withUncategorized.rows[3]).toMatchObject({ name: "Uncategorized", categoryId: null });
  });

  it("averages the closed months and compares the last closed month with that average", () => {
    const grid = buildComparison(sample, options);
    const food = grid.rows.find((r) => r.key === "food")!;
    expect(grid.partialIndex).toBe(3);
    expect(grid.lastClosedIndex).toBe(2);
    expect(food.average).toBe(400);
    expect(food.delta).toBeCloseTo(0.25);
    expect(grid.rows.find((r) => r.key === "rent")!.delta).toBe(0);
  });

  it("sums the total row and leaves the month in progress out of the averages", () => {
    const grid = buildComparison(sample, options);
    expect(grid.total.values).toEqual([1470, 1400, 1540, 1115]);
    expect(grid.total.average).toBeCloseTo((1470 + 1400 + 1540) / 3);
    expect(grid.total.max).toBe(1540);
  });

  it("treats the last month as closed when the current one is hidden", () => {
    const grid = buildComparison(sample, { ...options, includeCurrent: false });
    expect(grid.months).toEqual(["2026-07", "2026-08", "2026-09"]);
    expect(grid.partialIndex).toBe(-1);
    expect(grid.lastClosedIndex).toBe(2);
  });

  it("has no delta without a closed month", () => {
    const grid = buildComparison(fixture([category("a", "A", [40])], ["2026-10"]), { ...options, months: 1 });
    expect(grid.lastClosedIndex).toBe(-1);
    expect(grid.rows[0].delta).toBeNull();
    expect(grid.rows[0].average).toBe(0);
  });
});

describe("heatShare and deltaTone", () => {
  const row = buildComparison(sample, options).rows.find((r) => r.key === "food")!;

  it("scales cells to the row's highest closed month and skips the month in progress", () => {
    expect(heatShare(row, 2, 3)).toBe(1);
    expect(heatShare(row, 0, 3)).toBeCloseTo(0.8);
    expect(heatShare(row, 3, 3)).toBe(0);
  });

  it("only colours changes beyond 10%", () => {
    expect(deltaTone(0.25)).toBe("up");
    expect(deltaTone(-0.25)).toBe("down");
    expect(deltaTone(0.1)).toBeNull();
    expect(deltaTone(-0.05)).toBeNull();
    expect(deltaTone(null)).toBeNull();
  });
});

describe("monthTransactionsHref", () => {
  it("filters the expenses of one category and month", () => {
    expect(monthTransactionsHref("2026-02", "abc")).toBe("/transactions?type=expense&from=2026-02-01&to=2026-02-28&category_id=abc");
  });

  it("omits the category for uncategorized and keeps the owner", () => {
    expect(monthTransactionsHref("2026-09", null, "shared")).toBe("/transactions?type=expense&from=2026-09-01&to=2026-09-30&owner=shared");
  });
});
