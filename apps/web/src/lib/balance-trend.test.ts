import { describe, expect, it } from "vitest";

import { areaPath, buildTrend, crossesZero, hasTrend, linePath, nearestIndex, niceStep, xTickIndexes, yScale, type DailyCurrency } from "./balance-trend";

const day = (date: string, balance: number, income = 0, expense = 0) => ({ date, balance, income, expense, savings_balance: 0, investment_balance: 0 });

function currency(overrides: Partial<DailyCurrency> = {}): DailyCurrency {
  return {
    currency: "EUR",
    minor_units: 2,
    days: [day("2026-11-01", 100000), day("2026-11-02", 90000, 0, 10000)],
    projection: [
      { date: "2026-11-03", balance: 85000 },
      { date: "2026-11-04", balance: 40000 },
      { date: "2026-11-05", balance: 60000 },
    ],
    projection_basis: {
      average_daily_expense: 5000,
      recurring: [{ recurring_id: "r1", name: "Rent", amount: -45000, applied_on: "2026-11-04", due_on: "2026-11-04" }],
    },
    ...overrides,
  };
}

describe("buildTrend", () => {
  it("joins real days and projection, with the day net only for real days", () => {
    const trend = buildTrend(currency());
    expect(trend.points.map((p) => [p.date, p.projected, p.net])).toEqual([
      ["2026-11-01", false, 0],
      ["2026-11-02", false, -10000],
      ["2026-11-03", true, null],
      ["2026-11-04", true, null],
      ["2026-11-05", true, null],
    ]);
    expect(trend.todayIndex).toBe(1);
    expect(trend.projectionStart).toBe(2);
    expect(trend.endBalance).toBe(60000);
  });

  it("places recurring occurrences on the day the projection applies them", () => {
    const trend = buildTrend(currency());
    expect(trend.recurring).toEqual([{ recurringId: "r1", name: "Rent", amount: -45000, date: "2026-11-04", index: 3 }]);
  });

  it("finds the projected low point only when it is below today's balance", () => {
    expect(buildTrend(currency()).low).toEqual({ index: 3, balance: 40000 });
    const rising = currency({ projection: [{ date: "2026-11-03", balance: 95000 }, { date: "2026-11-04", balance: 99000 }] });
    expect(buildTrend(rising).low).toBeNull();
  });

  it("has no projection for a past month", () => {
    const trend = buildTrend(currency({ projection: [], projection_basis: { average_daily_expense: 0, recurring: [] } }));
    expect(trend.projectionStart).toBeNull();
    expect(trend.endBalance).toBeNull();
    expect(trend.low).toBeNull();
    expect(trend.todayIndex).toBe(1);
  });

  it("has only the projection for a future month", () => {
    const trend = buildTrend(currency({ days: [] }));
    expect(trend.todayIndex).toBeNull();
    expect(trend.projectionStart).toBe(0);
    expect(trend.low).toBeNull();
    expect(trend.points).toHaveLength(3);
  });
});

describe("hasTrend", () => {
  it("is false without days and projection", () => {
    expect(hasTrend(currency({ days: [], projection: [] }))).toBe(false);
    expect(hasTrend(currency())).toBe(true);
  });
});

describe("yScale", () => {
  it("covers every value with round ticks", () => {
    const { min, max, ticks } = yScale([100000, 90000, 40000, 60000]);
    expect(min).toBeLessThanOrEqual(40000);
    expect(max).toBeGreaterThanOrEqual(100000);
    expect(ticks[0]).toBe(min);
    expect(ticks[ticks.length - 1]).toBe(max);
    expect(ticks.length).toBeGreaterThanOrEqual(3);
    expect(ticks.length).toBeLessThanOrEqual(8);
  });

  it("keeps 0 on the axis when values cross it", () => {
    const { min, max } = yScale([-5000, 12000]);
    expect(min).toBeLessThan(0);
    expect(max).toBeGreaterThan(0);
  });

  it("does not go below 0 for non-negative values", () => {
    expect(yScale([100, 2000]).min).toBeGreaterThanOrEqual(0);
  });

  it("handles a flat series", () => {
    const { min, max } = yScale([5000, 5000]);
    expect(min).toBeLessThan(5000);
    expect(max).toBeGreaterThan(5000);
    expect(yScale([0, 0]).max).toBeGreaterThan(yScale([0, 0]).min);
  });
});

describe("niceStep and crossesZero", () => {
  it("rounds steps up to 1, 2, 2.5 or 5 times a power of ten", () => {
    expect(niceStep(0.9)).toBe(1);
    expect(niceStep(1500)).toBe(2000);
    expect(niceStep(2200)).toBe(2500);
    expect(niceStep(4100)).toBe(5000);
    expect(niceStep(0)).toBe(1);
  });

  it("detects values on both sides of 0", () => {
    expect(crossesZero([-1, 1])).toBe(true);
    expect(crossesZero([0, 1])).toBe(false);
    expect(crossesZero([-3, -1])).toBe(false);
  });
});

describe("xTickIndexes", () => {
  it("returns about five ticks including both ends", () => {
    expect(xTickIndexes(30)).toEqual([0, 7, 15, 22, 29]);
  });

  it("returns every day when there are few", () => {
    expect(xTickIndexes(3)).toEqual([0, 1, 2]);
    expect(xTickIndexes(1)).toEqual([0]);
  });
});

describe("nearestIndex", () => {
  it("snaps an x position to the closest point and clamps outside", () => {
    expect(nearestIndex(0, 0, 100, 11)).toBe(0);
    expect(nearestIndex(52, 0, 100, 11)).toBe(5);
    expect(nearestIndex(500, 0, 100, 11)).toBe(10);
    expect(nearestIndex(-5, 0, 100, 11)).toBe(0);
    expect(nearestIndex(10, 0, 100, 1)).toBe(0);
  });
});

describe("paths", () => {
  it("draws a line and closes an area down to the baseline", () => {
    const coords = [{ x: 0, y: 10 }, { x: 5, y: 4 }];
    expect(linePath(coords)).toBe("M0.0,10.0L5.0,4.0");
    expect(areaPath(coords, 20)).toBe("M0.0,10.0L5.0,4.0L5.0,20.0L0.0,20.0Z");
    expect(areaPath([], 20)).toBe("");
  });
});
