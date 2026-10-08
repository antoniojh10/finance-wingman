import { describe, expect, it } from "vitest";

import { amountField, barSegments, budgetState, buildBudgetItems, buildEditCurrencies, initialField, parseBudgetInput, type BudgetCurrencyData } from "./budgets";

describe("budgetState", () => {
  it("matches the API thresholds", () => {
    expect(budgetState(null, 500)).toBe("none");
    expect(budgetState(1000, 799)).toBe("ok");
    expect(budgetState(1000, 800)).toBe("near");
    expect(budgetState(1000, 1000)).toBe("near");
    expect(budgetState(1000, 1001)).toBe("over");
  });

  it("treats a zero budget as over once anything is used", () => {
    expect(budgetState(0, 0)).toBe("ok");
    expect(budgetState(0, 1)).toBe("over");
  });
});

describe("barSegments", () => {
  it("splits the budget between spent and committed", () => {
    expect(barSegments(1000, 250, 500)).toEqual({ spent: 25, committed: 50 });
  });

  it("scales to the usage when over budget", () => {
    expect(barSegments(1000, 1500, 500)).toEqual({ spent: 75, committed: 25 });
  });

  it("is empty without a scale", () => {
    expect(barSegments(0, 0, 0)).toEqual({ spent: 0, committed: 0 });
  });
});

describe("parseBudgetInput", () => {
  it("returns minor units, accepting zero and grouping spaces", () => {
    expect(parseBudgetInput("1 250.50", 2)).toBe(125050);
    expect(parseBudgetInput("0", 2)).toBe(0);
    expect(parseBudgetInput("0,00", 2)).toBe(0);
  });

  it("returns null when empty and undefined when invalid", () => {
    expect(parseBudgetInput("  ", 2)).toBeNull();
    expect(parseBudgetInput("abc", 2)).toBeUndefined();
    expect(parseBudgetInput("1.005", 2)).toBeUndefined();
    expect(parseBudgetInput("-5", 2)).toBeUndefined();
  });
});

function form(values: Record<string, string>): FormData {
  const data = new FormData();
  for (const [key, value] of Object.entries(values)) data.set(key, value);
  return data;
}

describe("buildBudgetItems", () => {
  const units = (currency: string) => (currency === "MXN" ? 2 : currency === "JPY" ? 0 : undefined);

  it("only sends the fields that changed", () => {
    const { items, errors } = buildBudgetItems(
      form({
        [amountField("MXN", "a")]: "300",
        [initialField("MXN", "a")]: "",
        [amountField("MXN", "b")]: "500.00",
        [initialField("MXN", "b")]: "500.00",
        [amountField("MXN", "c")]: "",
        [initialField("MXN", "c")]: "",
        [amountField("JPY", "d")]: "0",
        [initialField("JPY", "d")]: "1000",
      }),
      units,
    );
    expect(errors).toEqual({});
    expect(items).toEqual([
      { category_id: "a", currency: "MXN", amount: 30000 },
      { category_id: "d", currency: "JPY", amount: 0 },
    ]);
  });

  it("clears a budget whose field was emptied", () => {
    const { items } = buildBudgetItems(
      form({ [amountField("MXN", "a")]: "", [initialField("MXN", "a")]: "100" }),
      units,
    );
    expect(items).toEqual([{ category_id: "a", currency: "MXN", clear: true }]);
  });

  it("reports invalid amounts and unknown currencies", () => {
    const { items, errors } = buildBudgetItems(
      form({ [amountField("MXN", "a")]: "abc", [amountField("XXX", "b")]: "5" }),
      units,
    );
    expect(items).toEqual([]);
    expect(Object.keys(errors)).toEqual([amountField("MXN", "a"), amountField("XXX", "b")]);
  });
});

describe("buildEditCurrencies", () => {
  const line = (id: string, amount: number | null, amountMonth: string | null) => ({
    category_id: id,
    category_name: id,
    category_color: null,
    amount,
    amount_month: amountMonth,
    spent: 0,
    committed: 0,
    remaining: amount,
    state: "ok" as const,
  });
  const status: BudgetCurrencyData[] = [
    { currency: "MXN", minor_units: 2, budgeted: 100, spent: 0, committed: 0, remaining: 100, categories: [line("food", 100, "2026-03"), line("old", 50, "2026-03")], uncategorized: null },
  ];

  it("lists every active expense category per account currency", () => {
    const result = buildEditCurrencies({
      status,
      accounts: [
        { currency: "MXN", minor_units: 2, archived: false },
        { currency: "USD", minor_units: 2, archived: false },
        { currency: "EUR", minor_units: 2, archived: true },
      ],
      categories: [
        { id: "food", name: "Food", kind: "expense", archived: false },
        { id: "fun", name: "Fun", kind: "expense", archived: false },
        { id: "salary", name: "Salary", kind: "income", archived: false },
        { id: "old", name: "Old", kind: "expense", archived: true },
        { id: "gone", name: "Gone", kind: "expense", archived: true },
      ],
      suggestions: [{ currency: "MXN", suggestions: [{ category_id: "fun", suggested: 4000 }] }],
    });
    expect(result.map((c) => c.currency)).toEqual(["MXN", "USD"]);
    // An archived category stays only while it has a budget to clear.
    expect(result[0].rows.map((r) => r.name)).toEqual(["Food", "Fun", "Old"]);
    expect(result[0].rows[0]).toMatchObject({ amount: 100, amountMonth: "2026-03", suggested: null });
    expect(result[0].rows[1]).toMatchObject({ amount: null, suggested: 4000 });
    expect(result[1].rows.map((r) => r.name)).toEqual(["Food", "Fun"]);
  });
});
