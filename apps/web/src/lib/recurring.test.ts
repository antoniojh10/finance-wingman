import { describe, expect, it } from "vitest";

import { buildRecurringFields, yearlyFromMonthly } from "./recurring";

function form(values: Record<string, string>): FormData {
  const data = new FormData();
  for (const [key, value] of Object.entries(values)) data.set(key, value);
  return data;
}

const account = { id: "a", minor_units: 2 };
const valid = { name: " Gym ", amount: "30", interval_unit: "week", interval_count: "2", start_on: "2026-10-05" };

describe("yearlyFromMonthly", () => {
  it("multiplies by twelve", () => {
    expect(yearlyFromMonthly(19950)).toBe(239400);
    expect(yearlyFromMonthly(-100)).toBe(-1200);
  });
});

describe("buildRecurringFields", () => {
  it("builds the fields in minor units", () => {
    expect(buildRecurringFields(form(valid), account)).toEqual({
      fields: {
        name: "Gym",
        amount: 3000,
        category_id: undefined,
        interval_unit: "week",
        interval_count: 2,
        start_on: "2026-10-05",
        total_payments: undefined,
        notes: "",
      },
    });
  });

  it("falls back to monthly for an unknown unit", () => {
    const result = buildRecurringFields(form({ ...valid, interval_unit: "day" }), account);
    expect("fields" in result && result.fields.interval_unit).toBe("month");
  });

  it("collects every invalid field", () => {
    const result = buildRecurringFields(
      form({ name: "", amount: "1.234", interval_count: "1.5", start_on: "", total_payments: "0" }),
      account,
    );
    expect("errors" in result && result.errors.map((e) => e.field)).toEqual([
      "name",
      "amount",
      "interval_count",
      "start_on",
      "total_payments",
    ]);
  });
});
