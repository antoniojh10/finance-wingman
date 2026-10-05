import { describe, expect, it } from "vitest";
import { filterHref, newTransactionHref, pageHref, parseTransactionQuery, safeReturnPath } from "./query";

const id = "3f2b8c1e-5a4d-4c3b-9e8f-1a2b3c4d5e6f";

describe("parseTransactionQuery", () => {
  it("keeps valid filters", () => {
    expect(parseTransactionQuery({ account_id: id, type: "income", from: "2026-09-01", to: "2026-09-30", q: " tacos ", page: "3" })).toEqual({
      account_id: id,
      category_id: undefined,
      type: "income",
      from: "2026-09-01",
      to: "2026-09-30",
      q: "tacos",
      page: 3,
    });
  });

  it("drops invalid values", () => {
    expect(parseTransactionQuery({ account_id: "nope", type: "refund", from: "yesterday", page: "-2" })).toEqual({
      account_id: undefined,
      category_id: undefined,
      type: undefined,
      from: undefined,
      to: undefined,
      q: undefined,
      page: 1,
    });
  });

  it("uses the first value of repeated params", () => {
    expect(parseTransactionQuery({ type: ["expense", "income"] }).type).toBe("expense");
  });
});

describe("pageHref", () => {
  it("keeps filters and sets the page", () => {
    const query = parseTransactionQuery({ type: "expense", q: "a b" });
    expect(pageHref(query, 2)).toBe("?type=expense&q=a+b&page=2");
    expect(pageHref(query, 1)).toBe("?type=expense&q=a+b");
    expect(pageHref(parseTransactionQuery({}), 1)).toBe("?");
  });
});

describe("safeReturnPath", () => {
  it("keeps same-origin paths", () => {
    expect(safeReturnPath("/transactions?type=income")).toBe("/transactions?type=income");
    expect(safeReturnPath(["/accounts", "/other"])).toBe("/accounts");
  });

  it("falls back for missing or external targets", () => {
    expect(safeReturnPath(undefined)).toBe("/");
    expect(safeReturnPath("https://evil.example")).toBe("/");
    expect(safeReturnPath("//evil.example")).toBe("/");
    expect(safeReturnPath("/\\evil.example", "/transactions")).toBe("/transactions");
  });
});

describe("newTransactionHref", () => {
  it("only adds a return path when leaving a page other than the dashboard", () => {
    expect(newTransactionHref("/")).toBe("/transactions/new");
    expect(newTransactionHref("/transactions?type=income")).toBe("/transactions/new?return=%2Ftransactions%3Ftype%3Dincome");
  });
});

describe("filterHref", () => {
  it("changes one filter, keeps the rest and resets the page", () => {
    const query = parseTransactionQuery({ q: "tacos", type: "income", page: "4" });
    expect(filterHref(query, { type: "expense" })).toBe("?type=expense&q=tacos");
    expect(filterHref(query, { type: undefined })).toBe("?q=tacos");
  });
});
