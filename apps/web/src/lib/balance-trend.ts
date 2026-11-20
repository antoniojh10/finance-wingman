import type { components } from "@/lib/api/schema";

export type DailyCurrency = components["schemas"]["DailyCurrency"];

/** One day of the chart: a real end-of-day balance or a projected one. */
export type TrendPoint = {
  date: string;
  balance: number;
  projected: boolean;
  /** Income minus expense of the day; null for projected days. */
  net: number | null;
};

/** A pending recurring occurrence, placed on the day the projection applies it. */
export type TrendRecurring = {
  recurringId: string;
  name: string;
  /** Signed amount in minor units. */
  amount: number;
  date: string;
  /** Index of the point of that day in `points`. */
  index: number;
};

export type Trend = {
  points: TrendPoint[];
  /** Index of the last real day (today); null when the range has no real days. */
  todayIndex: number | null;
  /** Index of the first projected day; null without a projection. */
  projectionStart: number | null;
  recurring: TrendRecurring[];
  /** Projected balance on the last day; null without a projection. */
  endBalance: number | null;
  /** Lowest projected balance when it is below today's; null otherwise. */
  low: { index: number; balance: number } | null;
};

/** Merges the real days and the projection of one currency into a single series. */
export function buildTrend(data: DailyCurrency): Trend {
  const real: TrendPoint[] = data.days.map((d) => ({ date: d.date, balance: d.balance, projected: false, net: d.income - d.expense }));
  const projected: TrendPoint[] = data.projection.map((p) => ({ date: p.date, balance: p.balance, projected: true, net: null }));
  const points = [...real, ...projected];
  const todayIndex = real.length > 0 ? real.length - 1 : null;
  const projectionStart = projected.length > 0 ? real.length : null;

  const indexByDate = new Map(points.map((p, i) => [p.date, i]));
  const recurring = data.projection_basis.recurring.flatMap((r) => {
    const index = indexByDate.get(r.applied_on);
    return index === undefined ? [] : [{ recurringId: r.recurring_id, name: r.name, amount: r.amount, date: r.applied_on, index }];
  });

  let low: Trend["low"] = null;
  if (projectionStart !== null && todayIndex !== null) {
    const todayBalance = points[todayIndex].balance;
    for (let i = projectionStart; i < points.length; i++) {
      if (points[i].balance < todayBalance && (low === null || points[i].balance < low.balance)) {
        low = { index: i, balance: points[i].balance };
      }
    }
  }

  return { points, todayIndex, projectionStart, recurring, endBalance: projected.length > 0 ? projected[projected.length - 1].balance : null, low };
}

/** Whether there is anything to draw. */
export function hasTrend(data: DailyCurrency): boolean {
  return data.days.length + data.projection.length > 0;
}

/** Y axis: a rounded range containing every value (and 0 when the values cross it) with evenly spaced ticks. */
export function yScale(values: number[]): { min: number; max: number; ticks: number[] } {
  const lo = Math.min(...values);
  const hi = Math.max(...values);
  const span = hi - lo || Math.max(Math.abs(hi), 1);
  const step = niceStep(span / 4);
  let min = Math.floor((lo - span * 0.05) / step) * step;
  let max = Math.ceil((hi + span * 0.05) / step) * step;
  // A range that touches 0 keeps it on the axis.
  if (lo >= 0 && min < 0) {
    min = 0;
  }
  if (hi <= 0 && max > 0) {
    max = 0;
  }
  if (max === min) {
    max = min + step;
  }
  const ticks: number[] = [];
  for (let v = min; v <= max + step / 2; v += step) {
    ticks.push(v);
  }
  return { min, max, ticks };
}

/** A round step (1, 2, 2.5 or 5 times a power of ten) at least `raw` wide. */
export function niceStep(raw: number): number {
  if (raw <= 0) {
    return 1;
  }
  const power = 10 ** Math.floor(Math.log10(raw));
  const factor = [1, 2, 2.5, 5, 10].find((f) => f * power >= raw) ?? 10;
  return factor * power;
}

/** Whether the values go both sides of 0, which is where the zero line is drawn. */
export function crossesZero(values: number[]): boolean {
  return Math.min(...values) < 0 && Math.max(...values) > 0;
}

/** Evenly spread indexes for the x axis, always including the first and the last day. */
export function xTickIndexes(count: number, wanted = 5): number[] {
  if (count <= wanted) {
    return Array.from({ length: count }, (_, i) => i);
  }
  const indexes = new Set<number>();
  for (let k = 0; k < wanted; k++) {
    indexes.add(Math.round((k * (count - 1)) / (wanted - 1)));
  }
  return [...indexes];
}

/** Index of the point nearest to an x position, for a chart whose points are evenly spaced between `left` and `right`. */
export function nearestIndex(x: number, left: number, right: number, count: number): number {
  if (count <= 1 || right <= left) {
    return 0;
  }
  const ratio = (x - left) / (right - left);
  return Math.min(count - 1, Math.max(0, Math.round(ratio * (count - 1))));
}

/** SVG path through points, as a line. */
export function linePath(coords: { x: number; y: number }[]): string {
  return coords.map((c, i) => `${i === 0 ? "M" : "L"}${c.x.toFixed(1)},${c.y.toFixed(1)}`).join("");
}

/** SVG path closing a line down to a baseline, for the area fill. */
export function areaPath(coords: { x: number; y: number }[], baseline: number): string {
  if (coords.length === 0) {
    return "";
  }
  const last = coords[coords.length - 1];
  const first = coords[0];
  return `${linePath(coords)}L${last.x.toFixed(1)},${baseline.toFixed(1)}L${first.x.toFixed(1)},${baseline.toFixed(1)}Z`;
}
