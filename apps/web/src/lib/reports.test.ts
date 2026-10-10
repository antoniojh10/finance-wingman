import { describe, expect, it } from "vitest";

import {
  buildReport,
  computeView,
  describePoint,
  formatCompact,
  monthlyRequest,
  niceMax,
  parseReportsQuery,
  reportsHref,
  type MonthlyCurrency,
} from "./reports";

const labels = { uncategorized: "Uncategorized", other: "Other" };

type Cat = MonthlyCurrency["categories"][number];

function category(id: string | null, name: string | null, totals: number[], budgets?: (number | null)[]): Cat {
  return { category_id: id, name, color: id ? `#${id}` : null, archived: false, totals, budgets: budgets ?? totals.map(() => null) };
}

/** Four months ending in the partial 2026-10. */
function fixture(categories: Cat[], budgets: (number | null)[] = [null, null, null, null]): MonthlyCurrency {
  const months = ["2026-07", "2026-08", "2026-09", "2026-10"];
  return {
    currency: "EUR",
    minor_units: 2,
    months,
    partial_month: "2026-10",
    categories,
    totals: months.map((_, i) => categories.reduce((s, c) => s + c.totals[i], 0)),
    budgets,
  };
}

const sample = fixture(
  [
    category("rent", "Rent", [1000, 1000, 1000, 1000], [900, 900, 1000, 1000]),
    category("food", "Food", [400, 300, 500, 100], [null, 450, 450, 450]),
    category("fun", "Fun", [50, 100, 30, 10]),
    category(null, null, [20, 0, 10, 5]),
  ],
  [900, 1350, 1450, 1450],
);

describe("parseReportsQuery", () => {
  it("defaults to six months of amounts including the current month", () => {
    expect(parseReportsQuery({})).toEqual({ months: 6, currency: undefined, scale: "amount", includeCurrent: true, owner: undefined });
  });

  it("reads valid params and drops invalid ones", () => {
    const owner = "0b9e7a52-3b6c-4f3e-9c1a-1d2e3f4a5b6c";
    expect(parseReportsQuery({ months: "12", currency: "usd", scale: "share", current: "0", owner })).toEqual({
      months: 12,
      currency: "USD",
      scale: "share",
      includeCurrent: false,
      owner,
    });
    expect(parseReportsQuery({ months: "5", currency: "dollars", scale: "x", owner: "nope" })).toMatchObject({
      months: 6,
      currency: undefined,
      scale: "amount",
      owner: undefined,
    });
  });
});

describe("reportsHref", () => {
  const base = parseReportsQuery({});

  it("omits defaults", () => {
    expect(reportsHref(base)).toBe("/reports");
  });

  it("keeps the other filters when one changes", () => {
    const query = parseReportsQuery({ months: "3", currency: "USD" });
    expect(reportsHref(query, { scale: "share" })).toBe("/reports?months=3&currency=USD&scale=share");
    expect(reportsHref(query, { includeCurrent: false, months: 6 })).toBe("/reports?currency=USD&current=0");
    expect(reportsHref(query, { owner: "shared" })).toBe("/reports?months=3&currency=USD&owner=shared");
  });
});

describe("monthlyRequest", () => {
  it("maps filters to API params", () => {
    expect(monthlyRequest(parseReportsQuery({ months: "3", owner: "shared" }))).toEqual({ months: 3, owner: "shared" });
  });

  it("asks for the next window when the current month is left out", () => {
    expect(monthlyRequest(parseReportsQuery({ months: "3", current: "0" })).months).toBe(6);
    expect(monthlyRequest(parseReportsQuery({ months: "6", current: "0" })).months).toBe(12);
    expect(monthlyRequest(parseReportsQuery({ months: "12", current: "0" })).months).toBe(12);
  });
});

describe("buildReport", () => {
  it("orders categories by period total with Other last", () => {
    const report = buildReport(sample, { months: 4, includeCurrent: true, labels, topN: 2 });
    expect(report.series.map((s) => s.name)).toEqual(["Rent", "Food", "Other"]);
    const other = report.series[2];
    expect(other.other).toBe(true);
    expect(other.members).toEqual(["Fun", "Uncategorized"]);
    expect(other.values).toEqual([70, 100, 40, 15]);
    expect(other.color).toBeNull();
  });

  it("folds uncategorized into Other even with room for it", () => {
    const report = buildReport(sample, { months: 4, includeCurrent: true, labels });
    expect(report.series.map((s) => s.name)).toEqual(["Rent", "Food", "Fun", "Other"]);
    expect(report.series[3].members).toEqual(["Uncategorized"]);
  });

  it("keeps a category's color from the API", () => {
    const report = buildReport(sample, { months: 4, includeCurrent: true, labels });
    expect(report.series[1].color).toBe("#food");
  });

  it("flags the month in progress and can leave it out", () => {
    const withCurrent = buildReport(sample, { months: 4, includeCurrent: true, labels });
    expect(withCurrent.partialIndex).toBe(3);
    const without = buildReport(sample, { months: 4, includeCurrent: false, labels });
    expect(without.months).toEqual(["2026-07", "2026-08", "2026-09"]);
    expect(without.partialIndex).toBe(-1);
  });

  it("keeps only the last months of a longer window", () => {
    const report = buildReport(sample, { months: 2, includeCurrent: true, labels });
    expect(report.months).toEqual(["2026-09", "2026-10"]);
    expect(report.series[0].values).toEqual([1000, 1000]);
    expect(report.budgets).toEqual([1450, 1450]);
  });

  it("drops categories without spending in the window", () => {
    const data = fixture([category("rent", "Rent", [10, 10, 10, 10]), category("old", "Old", [50, 0, 0, 0])]);
    const report = buildReport(data, { months: 2, includeCurrent: true, labels });
    expect(report.series.map((s) => s.name)).toEqual(["Rent"]);
  });

  it("is empty without expenses", () => {
    const report = buildReport(fixture([]), { months: 4, includeCurrent: true, labels });
    expect(report.series).toEqual([]);
  });
});

describe("computeView", () => {
  const shape = buildReport(sample, { months: 4, includeCurrent: true, labels });

  it("totals the visible series per month", () => {
    const view = computeView(shape, new Set());
    expect(view.totals).toEqual([1470, 1400, 1540, 1115]);
    expect(view.periodTotal).toBe(5525);
  });

  it("averages the closed months only", () => {
    const view = computeView(shape, new Set());
    expect(view.average).toBeCloseTo((1470 + 1400 + 1540) / 3);
    expect(view.highest).toEqual({ index: 2, total: 1540 });
  });

  it("averages every month when the current one is not shown", () => {
    const closed = buildReport(sample, { months: 4, includeCurrent: false, labels });
    expect(computeView(closed, new Set()).average).toBeCloseTo((1470 + 1400 + 1540) / 3);
  });

  it("leaves hidden categories out of totals and average", () => {
    const view = computeView(shape, new Set(["rent"]));
    expect(view.totals).toEqual([470, 400, 540, 115]);
    expect(view.average).toBeCloseTo((470 + 400 + 540) / 3);
    expect(view.visible.map((s) => s.key)).toEqual(["food", "fun", "other"]);
  });

  it("shows the workspace budget total while nothing is hidden", () => {
    const view = computeView(shape, new Set());
    expect(view.budgetLine).toEqual([900, 1350, 1450, 1450]);
    expect(view.budgetSeries).toBeNull();
  });

  it("sums the budgets of the visible categories once some are hidden", () => {
    const view = computeView(shape, new Set(["fun", "other"]));
    expect(view.budgetLine).toEqual([900, 1350, 1450, 1450]);
    expect(computeView(shape, new Set(["rent", "fun"])).budgetLine).toEqual([null, 450, 450, 450]);
  });

  it("follows the budget of the only visible category", () => {
    const view = computeView(shape, new Set(["food", "fun", "other"]));
    expect(view.budgetLine).toEqual([900, 900, 1000, 1000]);
    expect(view.budgetSeries?.name).toBe("Rent");
  });

  it("has no budget line when no budget is set (owner filter)", () => {
    const noBudgets = fixture([category("rent", "Rent", [1, 1, 1, 1])], [null, null, null, null]);
    const view = computeView(buildReport(noBudgets, { months: 4, includeCurrent: true, labels }), new Set());
    expect(view.budgetLine).toBeNull();
  });

  it("handles everything hidden", () => {
    const view = computeView(shape, new Set(shape.series.map((s) => s.key)));
    expect(view.periodTotal).toBe(0);
    expect(view.highest).toBeNull();
    expect(view.budgetLine).toBeNull();
  });
});

describe("describePoint", () => {
  const shape = buildReport(sample, { months: 4, includeCurrent: true, labels });
  const view = computeView(shape, new Set());

  it("gives share of the month and difference against the category average", () => {
    const food = shape.series[1];
    const point = describePoint(shape, view, food, 2);
    expect(point.amount).toBe(500);
    expect(point.share).toBeCloseTo(500 / 1540);
    expect(point.average).toBeCloseTo(400);
    expect(point.delta).toBeCloseTo(0.25);
  });

  it("has no difference for the month in progress", () => {
    const point = describePoint(shape, view, shape.series[1], 3);
    expect(point.partial).toBe(true);
    expect(point.delta).toBeNull();
  });
});

describe("axis helpers", () => {
  it("rounds the axis maximum up", () => {
    expect(niceMax(0)).toBe(1);
    expect(niceMax(1470)).toBe(1500);
    expect(niceMax(1600)).toBe(2000);
  });

  it("formats compact amounts in minor units", () => {
    expect(formatCompact(123456, "EUR", 2, "en-US")).toBe("€1.2K");
  });
});
