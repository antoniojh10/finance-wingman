import { describe, expect, it } from "vitest";
import { formatMoney, parseAmount, toDecimalString } from "./money";

describe("parseAmount", () => {
  it.each([
    ["12", 2, 1200],
    ["12.5", 2, 1250],
    ["12.50", 2, 1250],
    ["12.500", 2, 1250],
    ["0.01", 2, 1],
    [".5", 2, 50],
    ["12,75", 2, 1275],
    [" 42.10 ", 2, 4210],
    ["1500", 0, 1500],
    ["1.2345", 4, 12345],
  ])("parses %s with %i decimals", (input, units, expected) => {
    expect(parseAmount(input, units)).toBe(expected);
  });

  it.each([
    ["", 2],
    ["abc", 2],
    ["-5", 2],
    ["0", 2],
    ["0.00", 2],
    ["1,000.50", 2],
    ["1.2.3", 2],
    ["12.345", 2],
    ["12.5", 0],
    ["1e5", 2],
    [".", 2],
    ["99999999999999999999", 2],
  ])("rejects %s with %i decimals", (input, units) => {
    expect(parseAmount(input, units)).toBeNull();
  });
});

describe("toDecimalString", () => {
  it.each([
    [0, 2, "0.00"],
    [1, 2, "0.01"],
    [1250, 2, "12.50"],
    [-325, 2, "-3.25"],
    [1500, 0, "1500"],
    [12345, 4, "1.2345"],
  ])("renders %i with %i decimals as %s", (amount, units, expected) => {
    expect(toDecimalString(amount, units)).toBe(expected);
  });

  it("round-trips with parseAmount", () => {
    for (const amount of [1, 99, 100, 123456789]) {
      expect(parseAmount(toDecimalString(amount, 2), 2)).toBe(amount);
    }
  });
});

describe("formatMoney", () => {
  it("formats using the locale and currency decimals", () => {
    expect(formatMoney(123456, "USD", 2, "en-US")).toBe("$1,234.56");
    expect(formatMoney(1500, "JPY", 0, "en-US")).toBe("¥1,500");
    expect(formatMoney(-2500, "MXN", 2, "es-MX")).toBe("-$25.00");
  });
});
