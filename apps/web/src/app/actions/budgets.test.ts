import "@/test/server-mocks";

import { beforeEach, describe, expect, it } from "vitest";

import { cookieJar, mockApi } from "@/test/server-mocks";

import { saveBudgets } from "./budgets";

function form(values: Record<string, string>): FormData {
  const data = new FormData();
  for (const [key, value] of Object.entries(values)) data.set(key, value);
  return data;
}

const accounts = {
  items: [
    { id: "a1", name: "Checking", currency: "MXN", minor_units: 2, archived: false },
    { id: "a2", name: "Cash", currency: "JPY", minor_units: 0, archived: false },
  ],
};

beforeEach(() => {
  cookieJar.clear();
  cookieJar.set("fw_session", { value: "tok" });
});

describe("saveBudgets", () => {
  it("sends the changed amounts for the month in minor units", async () => {
    const requests = mockApi([
      { method: "GET", path: "/api/v1/accounts", status: 200, body: accounts },
      { method: "PUT", path: "/api/v1/budgets/2026-10", status: 200, body: {} },
    ]);
    const result = await saveBudgets(
      { ok: false },
      form({
        month: "2026-10",
        "amount:MXN:food": "1 500.50",
        "initial:MXN:food": "",
        "amount:MXN:fun": "200",
        "initial:MXN:fun": "200",
        "amount:JPY:rent": "",
        "initial:JPY:rent": "90000",
      }),
    );
    expect(result.ok).toBe(true);
    expect(requests[1]).toMatchObject({
      method: "PUT",
      path: "/api/v1/budgets/2026-10",
      body: {
        items: [
          { category_id: "food", currency: "MXN", amount: 150050 },
          { category_id: "rent", currency: "JPY", clear: true },
        ],
      },
    });
  });

  it("does not call the API when nothing changed", async () => {
    const requests = mockApi([{ method: "GET", path: "/api/v1/accounts", status: 200, body: accounts }]);
    const result = await saveBudgets({ ok: false }, form({ month: "2026-10", "amount:MXN:food": "10", "initial:MXN:food": "10" }));
    expect(result.ok).toBe(true);
    expect(requests.map((r) => r.method)).toEqual(["GET"]);
  });

  it("reports invalid amounts per field without saving", async () => {
    const requests = mockApi([{ method: "GET", path: "/api/v1/accounts", status: 200, body: accounts }]);
    const result = await saveBudgets({ ok: false }, form({ month: "2026-10", "amount:MXN:food": "abc", "amount:JPY:rent": "1.5" }));
    expect(result.ok).toBe(false);
    expect(Object.keys(result.fieldErrors ?? {})).toEqual(["amount:MXN:food", "amount:JPY:rent"]);
    expect(requests.map((r) => r.method)).toEqual(["GET"]);
  });

  it("rejects an invalid month", async () => {
    const requests = mockApi([]);
    const result = await saveBudgets({ ok: false }, form({ month: "october", "amount:MXN:food": "10" }));
    expect(result.ok).toBe(false);
    expect(requests).toHaveLength(0);
  });

  it("shows API validation errors", async () => {
    mockApi([
      { method: "GET", path: "/api/v1/accounts", status: 200, body: accounts },
      { method: "PUT", path: "/api/v1/budgets/2026-10", status: 422, body: { status: 422, detail: "invalid" } },
    ]);
    const result = await saveBudgets({ ok: false }, form({ month: "2026-10", "amount:MXN:food": "10" }));
    expect(result).toMatchObject({ ok: false, message: "Please check the highlighted fields." });
  });
});
