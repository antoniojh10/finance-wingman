import "@/test/server-mocks";

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { cookieJar, mockApi, RedirectError } from "@/test/server-mocks";

import { deleteAccount, saveAccount } from "./accounts";
import { logout, requestLogin, verifyCode, verifyLink } from "./auth";
import { saveTransaction } from "./transactions";

function form(values: Record<string, string>): FormData {
  const data = new FormData();
  for (const [key, value] of Object.entries(values)) data.set(key, value);
  return data;
}

const session = {
  token: "tok_123",
  expires_at: "2027-01-01T00:00:00Z",
  user: { id: "u1", email: "ana@example.com", name: "Ana", locale: "es" },
};

const accounts = {
  items: [
    { id: "mxn", name: "Checking", currency: "MXN", minor_units: 2, archived: false },
    { id: "usd", name: "Dollars", currency: "USD", minor_units: 2, archived: false },
  ],
};

beforeEach(() => {
  cookieJar.clear();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("auth actions", () => {
  it("validates the email before calling the API", async () => {
    const requests = mockApi([]);
    expect(await requestLogin({ step: "email" }, form({ email: "nope" }))).toEqual({
      step: "email",
      email: "nope",
      error: "Enter a valid email address.",
    });
    expect(requests).toHaveLength(0);
  });

  it("requests a sign-in email in the current language", async () => {
    const requests = mockApi([{ method: "POST", path: "/api/v1/auth/login", status: 202 }]);
    expect(await requestLogin({ step: "email" }, form({ email: " Ana@Example.com " }))).toEqual({
      step: "code",
      email: "ana@example.com",
    });
    expect(requests[0].body).toEqual({ email: "ana@example.com", locale: "en" });
  });

  it("reports delivery failures", async () => {
    mockApi([{ method: "POST", path: "/api/v1/auth/login", status: 500 }]);
    expect((await requestLogin({ step: "email" }, form({ email: "ana@example.com" }))).error).toBe(
      "We couldn't send the email. Please try again.",
    );
  });

  it("stores the session and the user's language after a valid code", async () => {
    const requests = mockApi([{ method: "POST", path: "/api/v1/auth/verify", status: 200, body: session }]);
    await expect(verifyCode({ step: "code" }, form({ email: "ana@example.com", code: "123 456" }))).rejects.toEqual(
      new RedirectError("/"),
    );
    expect(requests[0].body).toEqual({ email: "ana@example.com", code: "123456" });
    // The API records the browser's user agent with the session.
    expect(requests[0].userAgent).toBe("Mozilla/5.0 (Test Browser)");
    expect(cookieJar.get("fw_session")).toMatchObject({ value: "tok_123", options: { httpOnly: true, sameSite: "lax" } });
    expect(cookieJar.get("NEXT_LOCALE")?.value).toBe("es");
  });

  it("rejects invalid codes", async () => {
    mockApi([{ method: "POST", path: "/api/v1/auth/verify", status: 401 }]);
    expect(await verifyCode({ step: "code" }, form({ email: "ana@example.com", code: "000000" }))).toMatchObject({
      error: "That code is invalid or has expired.",
    });
    expect(await verifyCode({ step: "code" }, form({ email: "ana@example.com", code: "12" }))).toMatchObject({
      error: "That code is invalid or has expired.",
    });
    expect(cookieJar.has("fw_session")).toBe(false);
  });

  it("redeems magic links", async () => {
    mockApi([{ method: "POST", path: "/api/v1/auth/verify", status: 401 }]);
    expect(await verifyLink({}, form({ token: "bad" }))).toEqual({
      error: "This sign-in link is invalid or has expired. Request a new one.",
    });

    const requests = mockApi([{ method: "POST", path: "/api/v1/auth/verify", status: 200, body: session }]);
    await expect(verifyLink({}, form({ token: "good" }))).rejects.toEqual(new RedirectError("/"));
    expect(requests[0].userAgent).toBe("Mozilla/5.0 (Test Browser)");
    expect(cookieJar.get("fw_session")?.value).toBe("tok_123");
  });

  it("revokes the session on logout", async () => {
    cookieJar.set("fw_session", { value: "tok_123" });
    const requests = mockApi([{ method: "POST", path: "/api/v1/auth/logout", status: 204 }]);
    await expect(logout()).rejects.toEqual(new RedirectError("/login"));
    expect(requests[0]).toMatchObject({ path: "/api/v1/auth/logout", auth: "Bearer tok_123" });
    expect(cookieJar.has("fw_session")).toBe(false);
  });
});

describe("saveTransaction", () => {
  beforeEach(() => {
    cookieJar.set("fw_session", { value: "tok_123" });
  });

  it("redirects to sign-in without a session", async () => {
    cookieJar.clear();
    mockApi([]);
    await expect(saveTransaction({ ok: false }, form({}))).rejects.toEqual(new RedirectError("/login"));
  });

  it("converts amounts using the account currency and creates the transaction", async () => {
    const requests = mockApi([
      { method: "GET", path: "/api/v1/accounts", status: 200, body: accounts },
      { method: "POST", path: "/api/v1/transactions", status: 201, body: {} },
    ]);
    const result = await saveTransaction(
      { ok: false },
      form({ type: "transfer", account_id: "mxn", destination_account_id: "usd", amount: "1700", destination_amount: "100", description: "Savings" }),
    );
    expect(result.ok).toBe(true);
    expect(requests[0]).toMatchObject({ path: "/api/v1/accounts?include_archived=true", auth: "Bearer tok_123" });
    expect(requests[1].body).toEqual({
      type: "transfer",
      account_id: "mxn",
      amount: 170000,
      destination_account_id: "usd",
      destination_amount: 10000,
      description: "Savings",
    });
  });

  it("replaces an existing transaction", async () => {
    const requests = mockApi([
      { method: "GET", path: "/api/v1/accounts", status: 200, body: accounts },
      { method: "PUT", path: "/api/v1/transactions/tx1", status: 200, body: {} },
    ]);
    expect((await saveTransaction({ ok: false }, form({ id: "tx1", type: "expense", account_id: "mxn", amount: "5" }))).ok).toBe(true);
    expect(requests[1]).toMatchObject({ method: "PUT", body: { type: "expense", account_id: "mxn", amount: 500 } });
  });

  it("accepts amounts grouped in thousands with spaces", async () => {
    const requests = mockApi([
      { method: "GET", path: "/api/v1/accounts", status: 200, body: accounts },
      { method: "POST", path: "/api/v1/transactions", status: 201, body: { id: "tx1" } },
    ]);
    expect((await saveTransaction({ ok: false }, form({ type: "expense", account_id: "mxn", amount: "1 234 567.50" }))).ok).toBe(true);
    expect(requests[1].body).toMatchObject({ amount: 123456750 });
  });

  it("returns translated field errors for invalid input", async () => {
    const requests = mockApi([{ method: "GET", path: "/api/v1/accounts", status: 200, body: accounts }]);
    expect(await saveTransaction({ ok: false }, form({ type: "expense", account_id: "mxn", amount: "1.234" }))).toEqual({
      ok: false,
      message: "Please check the highlighted fields.",
      fieldErrors: { amount: "Enter a positive amount with at most 2 decimals." },
    });
    expect(requests).toHaveLength(1);
  });

  it("maps API validation errors to fields", async () => {
    mockApi([
      { method: "GET", path: "/api/v1/accounts", status: 200, body: accounts },
      {
        method: "POST",
        path: "/api/v1/transactions",
        status: 422,
        body: { status: 422, detail: "category_id: category is archived", errors: [{ location: "category_id", message: "category is archived" }] },
      },
    ]);
    expect(await saveTransaction({ ok: false }, form({ type: "expense", account_id: "mxn", amount: "5", category_id: "c1" }))).toEqual({
      ok: false,
      message: "Please check the highlighted fields.",
      fieldErrors: { category_id: "category is archived" },
    });
  });

  it("sends expired sessions through sign-out", async () => {
    mockApi([{ method: "GET", path: "/api/v1/accounts", status: 401 }]);
    await expect(saveTransaction({ ok: false }, form({ type: "expense", account_id: "mxn", amount: "5" }))).rejects.toEqual(
      new RedirectError("/auth/signout"),
    );
  });
});

describe("account actions", () => {
  beforeEach(() => {
    cookieJar.set("fw_session", { value: "tok_123" });
  });

  it("creates accounts with the opening balance in minor units", async () => {
    const requests = mockApi([
      { method: "GET", path: "/api/v1/currencies", status: 200, body: { items: [{ code: "JPY", name: "Yen", minor_units: 0 }] } },
      { method: "POST", path: "/api/v1/accounts", status: 201, body: {} },
    ]);
    expect((await saveAccount({ ok: false }, form({ name: "Yen", type: "cash", currency: "JPY", initial_balance: "-1 500", balance_as_of: "2026-09-01" }))).ok).toBe(true);
    expect(requests[1].body).toEqual({ name: "Yen", type: "cash", currency: "JPY", initial_balance: -1500, balance_as_of: "2026-09-01" });
  });

  it("sends the balance date when editing an account", async () => {
    const requests = mockApi([
      { method: "GET", path: "/api/v1/accounts/a1", status: 200, body: { minor_units: 2 } },
      { method: "PATCH", path: "/api/v1/accounts/a1", status: 200, body: {} },
    ]);
    const data = form({ id: "a1", name: "Main", type: "cash", initial_balance: "10.50", balance_as_of: "2026-10-05" });
    expect((await saveAccount({ ok: false }, data)).ok).toBe(true);
    expect(requests[1].body).toEqual({ name: "Main", type: "cash", initial_balance: 1050, balance_as_of: "2026-10-05" });
  });

  it("sends the chosen owner when creating and editing", async () => {
    const requests = mockApi([
      { method: "GET", path: "/api/v1/currencies", status: 200, body: { items: [{ code: "EUR", name: "Euro", minor_units: 2 }] } },
      { method: "POST", path: "/api/v1/accounts", status: 201, body: {} },
      { method: "GET", path: "/api/v1/accounts/a1", status: 200, body: { minor_units: 2 } },
      { method: "PATCH", path: "/api/v1/accounts/a1", status: 200, body: {} },
    ]);
    const base = { name: "BNP", type: "checking", initial_balance: "0", balance_as_of: "2026-10-01" };
    expect((await saveAccount({ ok: false }, form({ ...base, currency: "EUR", owner: "shared" }))).ok).toBe(true);
    expect(requests[1].body).toMatchObject({ name: "BNP", owner: "shared" });
    expect((await saveAccount({ ok: false }, form({ ...base, id: "a1", owner: "u2" }))).ok).toBe(true);
    expect(requests[3].body).toMatchObject({ name: "BNP", owner: "u2" });
  });

  it("requires the balance date", async () => {
    mockApi([{ method: "GET", path: "/api/v1/currencies", status: 200, body: { items: [{ code: "MXN", name: "Peso", minor_units: 2 }] } }]);
    const result = await saveAccount({ ok: false }, form({ name: "Main", type: "cash", currency: "MXN", initial_balance: "1" }));
    expect(result.ok).toBe(false);
    expect(result.fieldErrors).toMatchObject({ balance_as_of: "This field is required." });
  });

  it("explains name conflicts", async () => {
    mockApi([
      { method: "GET", path: "/api/v1/currencies", status: 200, body: { items: [{ code: "MXN", name: "Peso", minor_units: 2 }] } },
      { method: "POST", path: "/api/v1/accounts", status: 409, body: { status: 409, detail: "conflict" } },
    ]);
    expect(await saveAccount({ ok: false }, form({ name: "Main", type: "cash", currency: "MXN", balance_as_of: "2026-09-01" }))).toEqual({
      ok: false,
      message: "An active account with this name already exists for this owner.",
    });
  });

  it("suggests archiving accounts with transactions instead of deleting them", async () => {
    mockApi([{ method: "DELETE", path: "/api/v1/accounts/a1", status: 409, body: { status: 409, detail: "has transactions" } }]);
    expect(await deleteAccount("a1")).toEqual({ ok: false, message: "This account has transactions. Archive it instead." });

    mockApi([{ method: "DELETE", path: "/api/v1/accounts/a1", status: 204 }]);
    expect((await deleteAccount("a1")).ok).toBe(true);
  });
});
