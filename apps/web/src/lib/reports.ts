import type { components } from "@/lib/api/schema";
import { monthRange } from "@/lib/dates";
import { parseOwner } from "@/lib/owners";

// Data shaping for the monthly spending report. Everything here is pure so
// the reports page, the dashboard card and the comparison table can share it.

export type MonthlyCurrency = components["schemas"]["MonthlyCurrency"];

export const PERIODS = [3, 6, 12] as const;
export type Period = (typeof PERIODS)[number];
export const DEFAULT_PERIOD: Period = 6;

/** "amount" stacks absolute figures; "share" normalizes each month to 100%. */
export type ReportsScale = "amount" | "share";

/** The category count that keeps its own segment; the rest folds into "Other". */
export const TOP_CATEGORIES = 7;

export type ReportsQuery = {
  months: Period;
  currency?: string;
  scale: ReportsScale;
  /** Whether the month in progress is shown (it is never part of averages). */
  includeCurrent: boolean;
  /** Accounts' owner: a user id or "shared". */
  owner?: string;
};

function first(value: string | string[] | undefined): string | undefined {
  return (Array.isArray(value) ? value[0] : value)?.trim() || undefined;
}

/** Sanitizes the reports search params, dropping invalid values. */
export function parseReportsQuery(params: Record<string, string | string[] | undefined>): ReportsQuery {
  const months = Number.parseInt(first(params.months) ?? "", 10);
  const currency = first(params.currency)?.toUpperCase();
  return {
    months: (PERIODS as readonly number[]).includes(months) ? (months as Period) : DEFAULT_PERIOD,
    currency: currency && /^[A-Z]{3}$/.test(currency) ? currency : undefined,
    scale: first(params.scale) === "share" ? "share" : "amount",
    includeCurrent: first(params.current) !== "0",
    owner: parseOwner(params.owner),
  };
}

/** Reports URL for a query with some values changed; defaults are left out. */
export function reportsHref(query: ReportsQuery, changes: Partial<ReportsQuery> = {}): string {
  const next = { ...query, ...changes };
  const params = new URLSearchParams();
  if (next.months !== DEFAULT_PERIOD) {
    params.set("months", String(next.months));
  }
  if (next.currency) {
    params.set("currency", next.currency);
  }
  if (next.scale !== "amount") {
    params.set("scale", next.scale);
  }
  if (!next.includeCurrent) {
    params.set("current", "0");
  }
  if (next.owner) {
    params.set("owner", next.owner);
  }
  const qs = params.toString();
  return qs ? `/reports?${qs}` : "/reports";
}

/**
 * Parameters for GET /api/v1/summary/monthly. The API window always ends
 * with the current month and only supports 3, 6 or 12 months, so leaving the
 * current month out asks for the next size up to still show `months` closed
 * months (11 for a 12-month period).
 */
export function monthlyRequest(query: ReportsQuery): { months: Period; owner?: string } {
  const larger: Record<Period, Period> = { 3: 6, 6: 12, 12: 12 };
  return { months: query.includeCurrent ? query.months : larger[query.months], owner: query.owner };
}

export type Series = {
  /** The category id, "uncategorized" or "other"; stable across toggles. */
  key: string;
  name: string;
  color: string | null;
  /** The folded "Other" series. */
  other: boolean;
  /** Names of the categories folded into "Other". */
  members: string[];
  values: number[];
  /** Budget in force per month; null where there is none. */
  budgets: (number | null)[];
  total: number;
};

export type ReportShape = {
  currency: string;
  minorUnits: number;
  months: string[];
  /** Index of the month in progress, or -1 when it is not shown. */
  partialIndex: number;
  /** Largest first, with "Other" last. */
  series: Series[];
  /** Workspace budget total per month; null where none is set. */
  budgets: (number | null)[];
};

export type ShapeLabels = { uncategorized: string; other: string };

function sumBudgets(lists: (number | null)[][], length: number): (number | null)[] {
  return Array.from({ length }, (_, i) => {
    const values = lists.map((list) => list[i]).filter((v): v is number => v !== null && v !== undefined);
    return values.length === 0 ? null : values.reduce((a, b) => a + b, 0);
  });
}

/**
 * Cuts the API window to the requested months, keeps the top categories by
 * period total as their own series and folds the rest (and uncategorized
 * expenses) into "Other".
 */
export function buildReport(
  data: MonthlyCurrency,
  options: { months: number; includeCurrent: boolean; labels: ShapeLabels; topN?: number; keepUncategorized?: boolean },
): ReportShape {
  const topN = options.topN ?? TOP_CATEGORIES;
  const end = options.includeCurrent ? data.months.length : data.months.length - 1;
  const start = Math.max(0, end - options.months);
  const slice = <T,>(list: T[]) => list.slice(start, end);
  const months = slice(data.months);
  const partialIndex = months.indexOf(data.partial_month);

  const rows = data.categories
    .map((c) => {
      const values = slice(c.totals);
      return {
        key: c.category_id ?? "uncategorized",
        id: c.category_id,
        name: c.name ?? options.labels.uncategorized,
        color: c.color,
        values,
        budgets: slice(c.budgets),
        total: values.reduce((a, b) => a + b, 0),
      };
    })
    .filter((row) => row.total > 0)
    .sort((a, b) => b.total - a.total);

  const named = rows.filter((r) => r.id !== null);
  const top = options.keepUncategorized ? rows.filter((r) => r.id === null || named.slice(0, topN).includes(r)) : named.slice(0, topN);
  const rest = rows.filter((r) => !top.includes(r));
  const series: Series[] = top.map((r) => ({
    key: r.key,
    name: r.name,
    color: r.color,
    other: false,
    members: [],
    values: r.values,
    budgets: r.budgets,
    total: r.total,
  }));
  if (rest.length > 0) {
    series.push({
      key: "other",
      name: options.labels.other,
      color: null,
      other: true,
      members: rest.map((r) => r.name),
      values: months.map((_, i) => rest.reduce((sum, r) => sum + r.values[i], 0)),
      budgets: sumBudgets(
        rest.map((r) => r.budgets),
        months.length,
      ),
      total: rest.reduce((sum, r) => sum + r.total, 0),
    });
  }

  return { currency: data.currency, minorUnits: data.minor_units, months, partialIndex, series, budgets: slice(data.budgets) };
}

export type ReportView = {
  visible: Series[];
  /** Total of the visible series per month. */
  totals: number[];
  periodTotal: number;
  /** Mean monthly total over the closed months (the month in progress is left out). */
  average: number;
  /** The closed month with the highest total. */
  highest: { index: number; total: number } | null;
  /** The budget reference per month: the workspace total, or the sum for the visible series once some are hidden. */
  budgetLine: (number | null)[] | null;
  /** Set when the budget line follows a single visible category. */
  budgetSeries: Series | null;
};

/** Totals, average and budget line for the series that are not hidden. */
export function computeView(shape: ReportShape, hidden: ReadonlySet<string>): ReportView {
  const visible = shape.series.filter((s) => !hidden.has(s.key));
  const totals = shape.months.map((_, i) => visible.reduce((sum, s) => sum + s.values[i], 0));
  const closed = totals.map((total, i) => ({ total, i })).filter(({ i }) => i !== shape.partialIndex);
  const average = closed.length === 0 ? 0 : closed.reduce((sum, c) => sum + c.total, 0) / closed.length;
  const best = closed.reduce<{ index: number; total: number } | null>(
    (acc, { total, i }) => (total > 0 && (!acc || total > acc.total) ? { index: i, total } : acc),
    null,
  );

  const line =
    hidden.size === 0
      ? shape.budgets
      : sumBudgets(
          visible.map((s) => s.budgets),
          shape.months.length,
        );
  const budgetLine = line.some((v) => v !== null) ? line : null;
  const only = visible.length === 1 && !visible[0].other ? visible[0] : null;

  return {
    visible,
    totals,
    periodTotal: totals.reduce((a, b) => a + b, 0),
    average,
    highest: best,
    budgetLine,
    budgetSeries: budgetLine && only ? only : null,
  };
}

/** Average of a series over the closed months. */
export function seriesAverage(shape: ReportShape, series: Series): number {
  const closed = series.values.filter((_, i) => i !== shape.partialIndex);
  return closed.length === 0 ? 0 : closed.reduce((a, b) => a + b, 0) / closed.length;
}

export type PointDetail = {
  series: Series;
  month: string;
  partial: boolean;
  amount: number;
  /** Fraction (0-1) of the month's visible total. */
  share: number;
  average: number;
  /** Fraction above (+) or below (-) the category average; null for the month in progress or without an average. */
  delta: number | null;
};

/** Everything the tooltip shows for one segment. */
export function describePoint(shape: ReportShape, view: ReportView, series: Series, index: number): PointDetail {
  const amount = series.values[index];
  const average = seriesAverage(shape, series);
  const partial = index === shape.partialIndex;
  return {
    series,
    month: shape.months[index],
    partial,
    amount,
    share: view.totals[index] > 0 ? amount / view.totals[index] : 0,
    average,
    delta: partial || average <= 0 ? null : amount / average - 1,
  };
}

/** A round upper bound for the y axis at or above `value`. */
export function niceMax(value: number): number {
  if (value <= 0) {
    return 1;
  }
  const power = 10 ** Math.floor(Math.log10(value));
  const step = [1, 1.2, 1.5, 2, 2.5, 3, 4, 5, 6, 8, 10].find((m) => m * power >= value) ?? 10;
  return step * power;
}

/** Short amount for axes and bar totals, e.g. "€1.2K". */
export function formatCompact(amount: number, currency: string, minorUnits: number, locale: string): string {
  return new Intl.NumberFormat(locale, { style: "currency", currency, notation: "compact", maximumFractionDigits: 1 }).format(
    amount / 10 ** minorUnits,
  );
}

export type ComparisonRow = {
  /** The category id or "uncategorized"; null for the total row. */
  key: string | null;
  /** The category id; null for uncategorized and the total row. */
  categoryId: string | null;
  name: string;
  color: string | null;
  values: number[];
  /** Mean over the closed months. */
  average: number;
  /** Fraction above (+) or below (-) the average for the last closed month; null without one or without an average. */
  delta: number | null;
  /** Highest closed-month value of the row, the reference for the cell shading. */
  max: number;
};

export type Comparison = {
  months: string[];
  partialIndex: number;
  /** Index of the last closed month, or -1 when there is none. */
  lastClosedIndex: number;
  rows: ComparisonRow[];
  total: ComparisonRow;
};

function comparisonRow(
  base: Pick<ComparisonRow, "key" | "categoryId" | "name" | "color">,
  values: number[],
  partialIndex: number,
  lastClosedIndex: number,
): ComparisonRow {
  const closed = values.filter((_, i) => i !== partialIndex);
  const average = closed.length === 0 ? 0 : closed.reduce((a, b) => a + b, 0) / closed.length;
  return {
    ...base,
    values,
    average,
    delta: lastClosedIndex < 0 || average <= 0 ? null : values[lastClosedIndex] / average - 1,
    max: Math.max(0, ...closed),
  };
}

/** The category x month grid of the comparison table: every category, none folded into "Other". */
export function buildComparison(data: MonthlyCurrency, options: { months: number; includeCurrent: boolean; labels: ShapeLabels }): Comparison {
  const shape = buildReport(data, { ...options, topN: Number.POSITIVE_INFINITY, keepUncategorized: true });
  const lastClosedIndex = shape.months.map((_, i) => i).findLast((i) => i !== shape.partialIndex) ?? -1;
  const rows = shape.series.map((s) =>
    comparisonRow(
      { key: s.key, categoryId: s.key === "uncategorized" ? null : s.key, name: s.name, color: s.color },
      s.values,
      shape.partialIndex,
      lastClosedIndex,
    ),
  );
  const totals = shape.months.map((_, i) => rows.reduce((sum, r) => sum + r.values[i], 0));
  const total = comparisonRow({ key: null, categoryId: null, name: "", color: null }, totals, shape.partialIndex, lastClosedIndex);
  return { months: shape.months, partialIndex: shape.partialIndex, lastClosedIndex, rows, total };
}

/** Fraction (0-1) of the row's highest closed month that a value represents; the month in progress is never shaded. */
export function heatShare(row: ComparisonRow, index: number, partialIndex: number): number {
  return index === partialIndex || row.max <= 0 ? 0 : Math.min(1, row.values[index] / row.max);
}

/** A delta beyond +-10% is worth colouring. */
export function deltaTone(delta: number | null): "up" | "down" | null {
  if (delta === null) {
    return null;
  }
  return delta > 0.1 ? "up" : delta < -0.1 ? "down" : null;
}

/**
 * Transactions list for the expenses of one month, optionally of one
 * category. Uncategorized expenses cannot be filtered on their own, so that
 * row and the total row list every expense of the month.
 */
export function monthTransactionsHref(month: string, categoryId: string | null, owner?: string): string {
  const { from, to } = monthRange(month);
  const params = new URLSearchParams({ type: "expense", from, to });
  if (categoryId) {
    params.set("category_id", categoryId);
  }
  if (owner) {
    params.set("owner", owner);
  }
  return `/transactions?${params.toString()}`;
}

/** Short month name for the x axis, e.g. "Oct". */
export function formatShortMonth(month: string, locale: string): string {
  const [year, m] = month.split("-").map(Number);
  return new Intl.DateTimeFormat(locale, { month: "short", timeZone: "UTC" })
    .format(new Date(Date.UTC(year, m - 1, 1)))
    .replace(".", "");
}
