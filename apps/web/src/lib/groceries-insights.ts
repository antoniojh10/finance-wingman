import type { components } from "@/lib/api/schema";

// View models for the groceries overview (breakdown, store comparison and
// basket). Pure functions so the components and tests do not need the API.

export type Breakdown = components["schemas"]["CurrencyBreakdown"];
export type Comparison = components["schemas"]["CurrencyComparison"];
export type CompareRow = components["schemas"]["CompareRow"];
export type PriceCell = components["schemas"]["PriceCell"];
export type Basket = components["schemas"]["CurrencyBasket"];

export type BreakdownBy = "group" | "store";
export type CompareLevel = "product" | "brand";
export type BasketScope = "comparable" | "all";

export type OverviewQuery = {
  /** YYYY-MM; undefined means the current month. */
  month?: string;
  currency?: string;
  by: BreakdownBy;
  level: CompareLevel;
  basket: BasketScope;
};

function first(value: string | string[] | undefined): string | undefined {
  return (Array.isArray(value) ? value[0] : value)?.trim() || undefined;
}

const monthPattern = /^\d{4}-(0[1-9]|1[0-2])$/;

/** Sanitizes the overview search params, dropping invalid values. */
export function parseOverviewQuery(params: Record<string, string | string[] | undefined>): OverviewQuery {
  const month = first(params.month);
  const currency = first(params.currency)?.toUpperCase();
  return {
    month: month && monthPattern.test(month) ? month : undefined,
    currency: currency && /^[A-Z]{3}$/.test(currency) ? currency : undefined,
    by: first(params.by) === "store" ? "store" : "group",
    level: first(params.level) === "brand" ? "brand" : "product",
    basket: first(params.basket) === "all" ? "all" : "comparable",
  };
}

/** Overview URL for a query with some values changed; defaults are left out. */
export function overviewHref(query: OverviewQuery, changes: Partial<OverviewQuery> = {}): string {
  const next = { ...query, ...changes };
  const params = new URLSearchParams();
  if (next.month) {
    params.set("month", next.month);
  }
  if (next.currency) {
    params.set("currency", next.currency);
  }
  if (next.by !== "group") {
    params.set("by", next.by);
  }
  if (next.level !== "product") {
    params.set("level", next.level);
  }
  if (next.basket !== "comparable") {
    params.set("basket", next.basket);
  }
  const qs = params.toString();
  return qs ? `/groceries?${qs}` : "/groceries";
}

/** The distinct currencies of the given responses, sorted. */
export function availableCurrencies(...lists: { currency: string }[][]): string[] {
  return [...new Set(lists.flat().map((entry) => entry.currency))].sort();
}

/** The requested currency when it has data, else the first available. */
export function pickCurrency(available: string[], requested?: string): string | undefined {
  return requested && available.includes(requested) ? requested : available[0];
}

export type BarRow = {
  key: string;
  /** Null for the unassigned bucket (the component supplies the label). */
  label: string | null;
  /** Default key of the group while it keeps its default name (to translate the label). */
  defaultKey: string | null;
  total: number;
  /** Width of the bar, 0..1, relative to the biggest row. */
  width: number;
  /** Percentage of the total, 0..100. */
  share: number;
  isFood: boolean | null;
  color: string | null;
  unassigned: boolean;
  receiptCount?: number;
};

/** Bars of the spend by product group or by store, biggest first, unassigned last. */
export function breakdownBars(data: Breakdown, by: BreakdownBy): BarRow[] {
  const rows: BarRow[] =
    by === "group"
      ? data.by_group.map((group) => ({
          key: group.group_id ?? "unassigned",
          label: group.name,
          defaultKey: group.default_key,
          total: group.total_minor,
          width: 0,
          share: group.share_pct,
          isFood: group.is_food,
          color: group.color,
          unassigned: group.group_id === null,
        }))
      : data.by_store.map((store) => ({
          key: store.store_id,
          label: store.name,
          defaultKey: null,
          total: store.total_minor,
          width: 0,
          share: data.total_minor > 0 ? (store.total_minor / data.total_minor) * 100 : 0,
          isFood: null,
          color: null,
          unassigned: false,
          receiptCount: store.receipt_count,
        }));
  const max = Math.max(0, ...rows.map((row) => row.total));
  return rows
    .map((row) => ({ ...row, width: max > 0 ? row.total / max : 0 }))
    .sort((a, b) => Number(a.unassigned) - Number(b.unassigned) || b.total - a.total);
}

/** Receipts total minus the Groceries category total; null when the category does not exist. */
export function categoryGap(data: Pick<Breakdown, "total_minor" | "groceries_category_minor">): number | null {
  return data.groceries_category_minor === null ? null : data.total_minor - data.groceries_category_minor;
}

/** Share of the lines that is not food, 0..100. */
export function nonFoodShare(data: Pick<Breakdown, "non_food_minor" | "lines_total_minor">): number {
  return data.lines_total_minor > 0 ? (data.non_food_minor / data.lines_total_minor) * 100 : 0;
}

export type CellTone = "best" | "worst" | "normal";

/** Whether a store's cell is the cheapest, the most expensive or in between (needs 2+ different prices). */
export function cellTone(row: CompareRow, storeId: string): CellTone {
  const cell = row.cells[storeId];
  const prices = Object.values(row.cells).map((entry) => entry.unit_price_minor);
  if (!cell || prices.length < 2) {
    return "normal";
  }
  const min = Math.min(...prices);
  const max = Math.max(...prices);
  if (min === max) {
    return "normal";
  }
  return cell.unit_price_minor === min ? "best" : cell.unit_price_minor === max ? "worst" : "normal";
}

/** Rows with the biggest price gap first, those with a single price last. */
export function sortCompareRows(rows: CompareRow[]): CompareRow[] {
  return [...rows].sort((a, b) => (b.max_diff_pct ?? -1) - (a.max_diff_pct ?? -1) || a.product_name.localeCompare(b.product_name));
}

/** Price per compare unit (times the display multiplier) in the currency, keeping sub-cent precision. */
export function formatUnitPrice(displayUnitPrice: number, currency: string, minorUnits: number, locale: string): string {
  return new Intl.NumberFormat(locale, {
    style: "currency",
    currency,
    minimumFractionDigits: minorUnits,
    maximumFractionDigits: minorUnits,
  }).format(displayUnitPrice / 10 ** minorUnits);
}

export type BasketBar = {
  storeId: string;
  name: string;
  cost: number;
  width: number;
  complete: boolean;
  missing: number;
  best: boolean;
};

/**
 * Per-store cost bars. "comparable" keeps only the stores pricing every
 * product; "all" adds the incomplete ones, whose cost undercounts.
 */
export function basketBars(basket: Basket, scope: BasketScope): BasketBar[] {
  const stores = basket.stores.filter((store) => scope === "all" || store.complete);
  const max = Math.max(0, ...stores.map((store) => store.cost_minor));
  const complete = stores.filter((store) => store.complete).map((store) => store.cost_minor);
  const cheapest = complete.length > 0 ? Math.min(...complete) : null;
  return stores
    .map((store) => ({
      storeId: store.store_id,
      name: store.name,
      cost: store.cost_minor,
      width: max > 0 ? store.cost_minor / max : 0,
      complete: store.complete,
      missing: store.missing_products.length,
      best: store.complete && store.cost_minor === cheapest,
    }))
    .sort((a, b) => Number(b.complete) - Number(a.complete) || a.cost - b.cost);
}

export type SavingsRow = { key: "paid" | "bestStore" | "split"; storeName?: string; cost: number; saving: number | null };

/**
 * What was paid versus the best single store and the split basket. The paid
 * figure leaves out products no store prices, so it is comparable with both.
 * `saving` is the difference against what was paid.
 */
export function basketSavings(basket: Basket): SavingsRow[] {
  const unpriced = new Set(basket.split.unpriced_products.map((product) => product.product_id));
  const paid = basket.products.filter((product) => !unpriced.has(product.product_id)).reduce((sum, product) => sum + product.paid_minor, 0);
  const rows: SavingsRow[] = [{ key: "paid", cost: paid, saving: null }];
  if (basket.best_single_store) {
    rows.push({ key: "bestStore", storeName: basket.best_single_store.name, cost: basket.best_single_store.cost_minor, saving: paid - basket.best_single_store.cost_minor });
  }
  rows.push({ key: "split", cost: basket.split.cost_minor, saving: paid - basket.split.cost_minor });
  return rows;
}
