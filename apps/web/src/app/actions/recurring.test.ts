import "@/test/server-mocks";

import { beforeEach, describe, expect, it } from "vitest";

import { cookieJar, mockApi } from "@/test/server-mocks";

import {
  acceptSuggestion,
  dismissSuggestion,
  registerRecurringPayment,
  saveRecurring,
  setRecurringStatus,
} from "./recurring";

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

describe("registerRecurringPayment", () => {
  const item = { id: "r1", minor_units: 2 };
  const payment = { id: "r1", period: "2026-10-01", amount: "210.5", date: "2026-10-05" };

  it("posts the payment converting the amount to minor units", async () => {
    const requests = mockApi([
      { method: "GET", path: "/api/v1/recurring/r1", status: 200, body: item },
      { method: "POST", path: "/api/v1/recurring/r1/payments", status: 201, body: {} },
    ]);
    expect((await registerRecurringPayment({ ok: false }, form(payment))).ok).toBe(true);
    expect(requests[1].body).toEqual({ amount: 21050, date: "2026-10-05", period: "2026-10-01" });
  });

  it("omits the period when none is given", async () => {
    const requests = mockApi([
      { method: "GET", path: "/api/v1/recurring/r1", status: 200, body: item },
      { method: "POST", path: "/api/v1/recurring/r1/payments", status: 201, body: {} },
    ]);
    await registerRecurringPayment({ ok: false }, form({ ...payment, period: "" }));
    expect(requests[1].body).not.toHaveProperty("period");
  });

  it("reports invalid amount and date without posting", async () => {
    const requests = mockApi([{ method: "GET", path: "/api/v1/recurring/r1", status: 200, body: item }]);
    const result = await registerRecurringPayment({ ok: false }, form({ ...payment, amount: "0", date: "" }));
    expect(result.ok).toBe(false);
    expect(Object.keys(result.fieldErrors ?? {}).sort()).toEqual(["amount", "date"]);
    expect(requests.map((r) => r.method)).toEqual(["GET"]);
  });

  it("shows the API message when the item is not active", async () => {
    mockApi([
      { method: "GET", path: "/api/v1/recurring/r1", status: 200, body: item },
      { method: "POST", path: "/api/v1/recurring/r1/payments", status: 409, body: { status: 409, detail: "item is not active" } },
    ]);
    const result = await registerRecurringPayment({ ok: false }, form(payment));
    expect(result.ok).toBe(false);
    expect(result.message).toContain("item is not active");
  });

  it("maps a missing item to a friendly message", async () => {
    mockApi([{ method: "GET", path: "/api/v1/recurring/r1", status: 404, body: { status: 404 } }]);
    expect(await registerRecurringPayment({ ok: false }, form(payment))).toEqual({
      ok: false,
      message: "This item no longer exists.",
    });
  });
});

describe("acceptSuggestion", () => {
  const suggestions = { items: [{ key: "k1", name: "Spotify", minor_units: 2 }] };
  const input = { key: "k1", name: "Spotify Premium", amount: "10.5" };

  it("accepts with the edited name and amount in minor units", async () => {
    const requests = mockApi([
      { method: "GET", path: "/api/v1/recurring/suggestions", status: 200, body: suggestions },
      { method: "POST", path: "/api/v1/recurring/suggestions/accept", status: 201, body: {} },
    ]);
    expect((await acceptSuggestion({ ok: false }, form(input))).ok).toBe(true);
    expect(requests[1].body).toEqual({ key: "k1", name: "Spotify Premium", amount: 1050 });
  });

  it("reports an invalid name and amount without posting", async () => {
    const requests = mockApi([{ method: "GET", path: "/api/v1/recurring/suggestions", status: 200, body: suggestions }]);
    const result = await acceptSuggestion({ ok: false }, form({ key: "k1", name: "", amount: "abc" }));
    expect(result.ok).toBe(false);
    expect(Object.keys(result.fieldErrors ?? {}).sort()).toEqual(["amount", "name"]);
    expect(requests.map((r) => r.method)).toEqual(["GET"]);
  });

  it("reports a stale suggestion as not found", async () => {
    const requests = mockApi([{ method: "GET", path: "/api/v1/recurring/suggestions", status: 200, body: { items: [] } }]);
    expect(await acceptSuggestion({ ok: false }, form(input))).toEqual({ ok: false, message: "This item no longer exists." });
    expect(requests).toHaveLength(1);
  });

  it("shows the API message on a name conflict", async () => {
    mockApi([
      { method: "GET", path: "/api/v1/recurring/suggestions", status: 200, body: suggestions },
      { method: "POST", path: "/api/v1/recurring/suggestions/accept", status: 409, body: { status: 409, detail: "name already in use" } },
    ]);
    const result = await acceptSuggestion({ ok: false }, form(input));
    expect(result.ok).toBe(false);
    expect(result.message).toContain("name already in use");
  });
});

describe("dismissSuggestion", () => {
  it("posts the key", async () => {
    const requests = mockApi([{ method: "POST", path: "/api/v1/recurring/suggestions/dismiss", status: 204 }]);
    expect((await dismissSuggestion("k1")).ok).toBe(true);
    expect(requests[0].body).toEqual({ key: "k1" });
  });

  it("reports a stale key", async () => {
    mockApi([{ method: "POST", path: "/api/v1/recurring/suggestions/dismiss", status: 404, body: { status: 404 } }]);
    expect(await dismissSuggestion("k1")).toEqual({ ok: false, message: "This item no longer exists." });
  });
});
