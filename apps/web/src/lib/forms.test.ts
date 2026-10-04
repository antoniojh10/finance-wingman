import { describe, expect, it } from "vitest";
import { buildTransactionBody, parseSignedAmount } from "./forms";

const accounts = [
  { id: "mxn", currency: "MXN", minor_units: 2 },
  { id: "mxn2", currency: "MXN", minor_units: 2 },
  { id: "usd", currency: "USD", minor_units: 2 },
  { id: "jpy", currency: "JPY", minor_units: 0 },
];

function form(values: Record<string, string>): FormData {
  const data = new FormData();
  for (const [key, value] of Object.entries(values)) {
    data.set(key, value);
  }
  return data;
}

describe("parseSignedAmount", () => {
  it.each([
    ["", 0],
    ["0", 0],
    ["-0.00", 0],
    ["150.5", 15050],
    ["-2,500.00".replace(",", ""), -250000],
    ["+3", 300],
  ])("parses %s", (input, expected) => {
    expect(parseSignedAmount(input, 2)).toBe(expected);
  });

  it("rejects invalid input", () => {
    expect(parseSignedAmount("abc", 2)).toBeNull();
    expect(parseSignedAmount("1.234", 2)).toBeNull();
  });
});

describe("buildTransactionBody", () => {
  it("builds an expense with category and description", () => {
    const result = buildTransactionBody(
      form({ type: "expense", account_id: "mxn", amount: "250.50", category_id: "cat-1", description: " Lunch ", occurred_on: "2026-10-01" }),
      accounts,
    );
    expect(result).toEqual({
      body: {
        type: "expense",
        account_id: "mxn",
        amount: 25050,
        category_id: "cat-1",
        description: "Lunch",
        occurred_on: "2026-10-01",
      },
    });
  });

  it("uses the account currency decimals", () => {
    expect(buildTransactionBody(form({ type: "income", account_id: "jpy", amount: "1500" }), accounts)).toMatchObject({
      body: { amount: 1500 },
    });
    expect(buildTransactionBody(form({ type: "income", account_id: "jpy", amount: "15.5" }), accounts)).toEqual({
      errors: [{ field: "amount", error: "invalidAmount", decimals: 0 }],
    });
  });

  it("requires an account", () => {
    expect(buildTransactionBody(form({ type: "expense", amount: "10" }), accounts)).toEqual({
      errors: [{ field: "account_id", error: "required" }],
    });
  });

  it("builds same-currency transfers without destination amount or category", () => {
    const result = buildTransactionBody(
      form({ type: "transfer", account_id: "mxn", destination_account_id: "mxn2", amount: "100", category_id: "ignored" }),
      accounts,
    );
    expect(result).toEqual({
      body: { type: "transfer", account_id: "mxn", amount: 10000, destination_account_id: "mxn2" },
    });
  });

  it("requires the received amount for cross-currency transfers", () => {
    expect(
      buildTransactionBody(form({ type: "transfer", account_id: "mxn", destination_account_id: "usd", amount: "1700" }), accounts),
    ).toEqual({ errors: [{ field: "destination_amount", error: "invalidAmount", decimals: 2 }] });

    expect(
      buildTransactionBody(
        form({ type: "transfer", account_id: "mxn", destination_account_id: "usd", amount: "1700", destination_amount: "100" }),
        accounts,
      ),
    ).toMatchObject({ body: { destination_amount: 10000 } });
  });

  it("requires a destination for transfers", () => {
    expect(buildTransactionBody(form({ type: "transfer", account_id: "mxn", amount: "1" }), accounts)).toEqual({
      errors: [{ field: "destination_account_id", error: "required" }],
    });
  });
});
