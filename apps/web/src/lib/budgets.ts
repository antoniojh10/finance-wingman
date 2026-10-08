import { parseAmount } from "@/lib/money";

export type BudgetState = "ok" | "near" | "over" | "none";

/** A line of the budget status API (a category in one currency and month). */
export type BudgetLineData = {
  category_id: string | null;
  category_name: string | null;
  category_color: string | null;
  amount: number | null;
  amount_month: string | null;
  spent: number;
  committed: number;
  remaining: number | null;
  state: BudgetState;
};

export type BudgetCurrencyData = {
  currency: string;
  minor_units: number;
  budgeted: number;
  spent: number;
  committed: number;
  remaining: number;
  categories: BudgetLineData[];
  uncategorized: BudgetLineData | null;
};

/** Share of the budget used (spent + committed) from which a line is "near". */
const NEAR_PERCENT = 80;

/** Same thresholds as the API: near from 80% used, over once it exceeds the amount. */
export function budgetState(amount: number | null, used: number): BudgetState {
  if (amount === null) {
    return "none";
  }
  if (used > amount) {
    return "over";
  }
  if (amount > 0 && used * 100 >= amount * NEAR_PERCENT) {
    return "near";
  }
  return "ok";
}

/**
 * Widths (0 to 100, as a share of the bar) of the spent and committed
 * segments. The bar covers the whole budget, or the whole usage when that
 * exceeds the budget, so an overspent line shows a full bar.
 */
export function barSegments(amount: number, spent: number, committed: number): { spent: number; committed: number } {
  const scale = Math.max(amount, spent + committed);
  if (scale <= 0) {
    return { spent: 0, committed: 0 };
  }
  return { spent: (spent / scale) * 100, committed: (committed / scale) * 100 };
}

/** Name of the form field holding the budget of a category in a currency. */
export function amountField(currency: string, categoryId: string): string {
  return `amount:${currency}:${categoryId}`;
}

/** Name of the hidden field holding the value the budget field started with. */
export function initialField(currency: string, categoryId: string): string {
  return `initial:${currency}:${categoryId}`;
}

/**
 * Parses a budget field. Empty means no budget (null); zero is a valid
 * budget ("spend nothing"). Returns undefined when invalid.
 */
export function parseBudgetInput(input: string, minorUnits: number): number | null | undefined {
  const value = input.trim();
  if (value === "") {
    return null;
  }
  if (/^0*([.,]0*)?$/.test(value.replace(/\s/g, ""))) {
    return 0;
  }
  return parseAmount(value, minorUnits) ?? undefined;
}

export type BudgetItem = { category_id: string; currency: string; amount?: number; clear?: boolean };

export type BudgetFormResult = { items: BudgetItem[]; errors: Record<string, "invalidAmount"> };

/**
 * Collects the budgets changed in the edit form: the fields whose value
 * differs from the one they started with. A field emptied clears the budget
 * from the month on; untouched fields (including inherited amounts) are not
 * sent, so they keep their current source month.
 */
export function buildBudgetItems(formData: FormData, minorUnitsOf: (currency: string) => number | undefined): BudgetFormResult {
  const items: BudgetItem[] = [];
  const errors: Record<string, "invalidAmount"> = {};
  for (const [name, raw] of formData.entries()) {
    const [kind, currency, categoryId] = name.split(":");
    if (kind !== "amount" || !currency || !categoryId || typeof raw !== "string") {
      continue;
    }
    const minorUnits = minorUnitsOf(currency);
    const value = minorUnits === undefined ? undefined : parseBudgetInput(raw, minorUnits);
    if (minorUnits === undefined || value === undefined) {
      errors[name] = "invalidAmount";
      continue;
    }
    const initialRaw = formData.get(initialField(currency, categoryId));
    const initial = typeof initialRaw === "string" ? parseBudgetInput(initialRaw, minorUnits) : null;
    if (value === (initial ?? null)) {
      continue;
    }
    items.push(value === null ? { category_id: categoryId, currency, clear: true } : { category_id: categoryId, currency, amount: value });
  }
  return { items, errors };
}

export type EditRow = {
  categoryId: string;
  name: string;
  color: string | null;
  /** Budget in force, in minor units. */
  amount: number | null;
  amountMonth: string | null;
  /** Suggested budget, in minor units. */
  suggested: number | null;
};

export type EditCurrency = { currency: string; minorUnits: number; rows: EditRow[] };

type CategoryRef = { id: string; name: string; color?: string | null; kind: string; archived: boolean };

/**
 * Rows of the edit form: every active expense category in every currency of
 * the workspace (accounts and budget lines), plus archived categories that
 * still have a budget so it can be cleared.
 */
export function buildEditCurrencies(input: {
  status: BudgetCurrencyData[];
  accounts: { currency: string; minor_units: number; archived: boolean }[];
  categories: CategoryRef[];
  suggestions: { currency: string; suggestions: { category_id: string; suggested: number }[] }[];
}): EditCurrency[] {
  const units = new Map<string, number>();
  for (const a of input.accounts) {
    if (!a.archived) {
      units.set(a.currency, a.minor_units);
    }
  }
  for (const c of input.status) {
    units.set(c.currency, c.minor_units);
  }

  return [...units.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([currency, minorUnits]) => {
      const lines = new Map<string, BudgetLineData>();
      for (const line of input.status.find((c) => c.currency === currency)?.categories ?? []) {
        if (line.category_id) {
          lines.set(line.category_id, line);
        }
      }
      const suggested = new Map<string, number>();
      for (const s of input.suggestions.find((c) => c.currency === currency)?.suggestions ?? []) {
        suggested.set(s.category_id, s.suggested);
      }
      const rows: EditRow[] = input.categories
        .filter((c) => c.kind === "expense" && (!c.archived || lines.get(c.id)?.amount != null))
        .map((c) => ({
          categoryId: c.id,
          name: c.name,
          color: c.color ?? null,
          amount: lines.get(c.id)?.amount ?? null,
          amountMonth: lines.get(c.id)?.amount_month ?? null,
          suggested: suggested.get(c.id) ?? null,
        }))
        .sort((a, b) => a.name.localeCompare(b.name));
      return { currency, minorUnits, rows };
    });
}
