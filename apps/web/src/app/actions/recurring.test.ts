import "@/test/server-mocks";

import { beforeEach, describe, expect, it } from "vitest";

import { cookieJar, mockApi } from "@/test/server-mocks";

import { saveRecurring, setRecurringStatus } from "./recurring";

function form(values: Record<string, string>): FormData {
  const data = new FormData();
  for (const [key, value] of Object.entries(values)) data.set(key, value);
  return data;
}

const accounts = { items: [{ id: "mxn", name: "Checking", currency: "MXN", minor_units: 2, archived: false }] };

const valid = {
  type: "expense",
  account_id: "mxn",
  name: "Netflix",
  amount: "199.50",
  category_id: "cat1",
  interval_unit: "month",
  interval_count: "1",
  start_on: "2026-10-05",
  total_payments: "",
  notes: "",
};

beforeEach(() => {
  cookieJar.clear();
  cookieJar.set("fw_session", { value: "tok" });
});

describe("saveRecurring", () => {
  it("creates an item converting the amount to minor units", async () => {
    const requests = mockApi([
      { method: "GET", path: "/api/v1/accounts", status: 200, body: accounts },
      { method: "POST", path: "/api/v1/recurring", status: 201, body: {} },
    ]);
    expect((await saveRecurring({ ok: false }, form(valid))).ok).toBe(true);
    expect(requests[1].body).toEqual({
      type: "expense",
      account_id: "mxn",
      name: "Netflix",
      amount: 19950,
      category_id: "cat1",
      interval_unit: "month",
      interval_count: 1,
      start_on: "2026-10-05",
      notes: "",
    });
  });

  it("sends the installment count when provided", async () => {
    const requests = mockApi([
      { method: "GET", path: "/api/v1/accounts", status: 200, body: accounts },
      { method: "POST", path: "/api/v1/recurring", status: 201, body: {} },
    ]);
    await saveRecurring({ ok: false }, form({ ...valid, type: "income", total_payments: "12" }));
    expect(requests[1].body).toMatchObject({ type: "income", total_payments: 12 });
  });

  it("reports invalid fields without creating anything", async () => {
    const requests = mockApi([{ method: "GET", path: "/api/v1/accounts", status: 200, body: accounts }]);
    const result = await saveRecurring(
      { ok: false },
      form({ ...valid, name: "", amount: "abc", interval_count: "0", total_payments: "x" }),
    );
    expect(result.ok).toBe(false);
    expect(Object.keys(result.fieldErrors ?? {}).sort()).toEqual(["amount", "interval_count", "name", "total_payments"]);
    expect(requests.map((r) => r.method)).toEqual(["GET"]);
  });

  it("requires an account", async () => {
    mockApi([{ method: "GET", path: "/api/v1/accounts", status: 200, body: accounts }]);
    const result = await saveRecurring({ ok: false }, form({ ...valid, account_id: "" }));
    expect(result.fieldErrors).toMatchObject({ account_id: "This field is required." });
  });

  it("updates an item and clears the optional fields left empty", async () => {
    const requests = mockApi([
      { method: "GET", path: "/api/v1/recurring/r1", status: 200, body: { id: "r1", account_id: "mxn", minor_units: 2 } },
      { method: "PATCH", path: "/api/v1/recurring/r1", status: 200, body: {} },
    ]);
    expect((await saveRecurring({ ok: false }, form({ ...valid, id: "r1", category_id: "" }))).ok).toBe(true);
    expect(requests[1].body).toMatchObject({ amount: 19950, clear_category: true, clear_total_payments: true });
    expect(requests[1].body).not.toHaveProperty("type");
    expect(requests[1].body).not.toHaveProperty("account_id");
  });

  it("shows the API message on a name conflict", async () => {
    mockApi([
      { method: "GET", path: "/api/v1/accounts", status: 200, body: accounts },
      { method: "POST", path: "/api/v1/recurring", status: 409, body: { status: 409, detail: "name already in use" } },
    ]);
    const result = await saveRecurring({ ok: false }, form(valid));
    expect(result.ok).toBe(false);
    expect(result.message).toContain("name already in use");
  });

  it("maps not found to a friendly message", async () => {
    mockApi([{ method: "GET", path: "/api/v1/recurring/r1", status: 404, body: { status: 404 } }]);
    expect(await saveRecurring({ ok: false }, form({ ...valid, id: "r1" }))).toEqual({
      ok: false,
      message: "This item no longer exists.",
    });
  });
});

describe("setRecurringStatus", () => {
  it("patches the status", async () => {
    const requests = mockApi([{ method: "PATCH", path: "/api/v1/recurring/r1", status: 200, body: {} }]);
    expect((await setRecurringStatus("r1", "paused")).ok).toBe(true);
    expect(requests[0].body).toEqual({ status: "paused" });
  });

  it("reports failures", async () => {
    mockApi([{ method: "PATCH", path: "/api/v1/recurring/r1", status: 404, body: { status: 404 } }]);
    expect((await setRecurringStatus("r1", "cancelled")).ok).toBe(false);
  });
});
