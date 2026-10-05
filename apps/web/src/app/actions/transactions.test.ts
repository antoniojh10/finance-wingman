import "@/test/server-mocks";

import { beforeEach, describe, expect, it } from "vitest";

import { cookieJar, mockApi } from "@/test/server-mocks";

import { findRecurringMatch, linkTransactionRecurring, saveTransaction, unlinkTransactionRecurring } from "./transactions";

beforeEach(() => {
  cookieJar.clear();
  cookieJar.set("fw_session", { value: "tok" });
});

const accounts = { items: [{ id: "mxn", name: "Checking", currency: "MXN", minor_units: 2, archived: false }] };

function form(values: Record<string, string>): FormData {
  const data = new FormData();
  for (const [key, value] of Object.entries(values)) data.set(key, value);
  return data;
}

describe("saveTransaction", () => {
  it("returns the id of a created transaction", async () => {
    mockApi([
      { method: "GET", path: "/api/v1/accounts", status: 200, body: accounts },
      { method: "POST", path: "/api/v1/transactions", status: 201, body: { id: "tx9" } },
    ]);
    const result = await saveTransaction({ ok: false }, form({ type: "expense", account_id: "mxn", amount: "5" }));
    expect(result).toMatchObject({ ok: true, transactionId: "tx9" });
  });

  it("returns no id when editing", async () => {
    mockApi([
      { method: "GET", path: "/api/v1/accounts", status: 200, body: accounts },
      { method: "PUT", path: "/api/v1/transactions/tx1", status: 200, body: { id: "tx1" } },
    ]);
    const result = await saveTransaction({ ok: false }, form({ id: "tx1", type: "expense", account_id: "mxn", amount: "5" }));
    expect(result.ok).toBe(true);
    expect(result.transactionId).toBeUndefined();
  });
});

describe("findRecurringMatch", () => {
  it("returns the matching item and period", async () => {
    const requests = mockApi([
      {
        method: "GET",
        path: "/api/v1/transactions/tx1/recurring-match",
        status: 200,
        body: { item: { id: "r1", name: "Netflix" }, period_due_on: "2026-10-05" },
      },
    ]);
    expect(await findRecurringMatch("tx1")).toEqual({ recurringId: "r1", name: "Netflix", period: "2026-10-05" });
    expect(requests[0].auth).toBe("Bearer tok");
  });

  it("returns null when nothing matches", async () => {
    mockApi([{ method: "GET", path: "/api/v1/transactions/tx1/recurring-match", status: 200, body: { item: null, period_due_on: null } }]);
    expect(await findRecurringMatch("tx1")).toBeNull();
  });

  it("returns null when the lookup fails", async () => {
    mockApi([{ method: "GET", path: "/api/v1/transactions/tx1/recurring-match", status: 500, body: { status: 500 } }]);
    expect(await findRecurringMatch("tx1")).toBeNull();
  });
});

describe("linkTransactionRecurring", () => {
  it("links with the chosen period", async () => {
    const requests = mockApi([{ method: "PUT", path: "/api/v1/transactions/tx1/recurring", status: 200, body: {} }]);
    expect((await linkTransactionRecurring("tx1", "r1", "2026-10-05")).ok).toBe(true);
    expect(requests[0]).toMatchObject({ method: "PUT", body: { recurring_id: "r1", period: "2026-10-05" } });
  });

  it("omits an empty period", async () => {
    const requests = mockApi([{ method: "PUT", path: "/api/v1/transactions/tx1/recurring", status: 200, body: {} }]);
    await linkTransactionRecurring("tx1", "r1", "");
    expect(requests[0].body).toEqual({ recurring_id: "r1" });
  });

  it("shows the API reason when the link is refused", async () => {
    mockApi([
      {
        method: "PUT",
        path: "/api/v1/transactions/tx1/recurring",
        status: 422,
        body: { status: 422, detail: "transaction and recurring item must share the account" },
      },
    ]);
    expect(await linkTransactionRecurring("tx1", "r1")).toEqual({
      ok: false,
      message: "transaction and recurring item must share the account",
    });
  });
});

describe("unlinkTransactionRecurring", () => {
  it("unlinks the transaction", async () => {
    const requests = mockApi([{ method: "DELETE", path: "/api/v1/transactions/tx1/recurring", status: 204 }]);
    expect((await unlinkTransactionRecurring("tx1")).ok).toBe(true);
    expect(requests[0].method).toBe("DELETE");
  });

  it("reports a missing transaction", async () => {
    mockApi([{ method: "DELETE", path: "/api/v1/transactions/tx1/recurring", status: 404, body: { status: 404 } }]);
    expect((await unlinkTransactionRecurring("tx1")).ok).toBe(false);
  });
});
