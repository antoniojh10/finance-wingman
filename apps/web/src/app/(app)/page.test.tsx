import "@/test/server-mocks";

import { beforeEach, describe, expect, it, vi } from "vitest";

import { BalanceTrendCard } from "@/components/dashboard/balance-trend-card";

const calls: { path: string; query: unknown }[] = [];
const daily = { currencies: [{ currency: "EUR", minor_units: 2, days: [], projection: [], projection_basis: { average_daily_expense: 0, recurring: [] } }] };

const bodies: Record<string, unknown> = {
  "/api/v1/summary": { currencies: [] },
  "/api/v1/accounts": { items: [{ id: "a1", name: "Main", owner: null, type: "checking", currency: "EUR", minor_units: 2, balance: 0 }] },
  "/api/v1/categories": { items: [] },
  "/api/v1/transactions": { items: [], total: 0 },
  "/api/v1/recurring/upcoming": { items: [] },
  "/api/v1/recurring/summary": { currencies: [] },
  "/api/v1/recurring": { items: [] },
  "/api/v1/recurring/suggestions": { items: [] },
  "/api/v1/budgets": { currencies: [] },
  "/api/v1/summary/monthly": { currencies: [] },
  "/api/v1/summary/daily": daily,
};

vi.mock("@/lib/session", () => ({
  authedApi: async () => ({
    GET: async (path: string, init?: { params?: { query?: unknown } }) => {
      calls.push({ path, query: init?.params?.query });
      return { data: bodies[path], response: new Response() };
    },
  }),
  expectData: <T,>(result: { data?: T }) => result.data as T,
  getAccountOwners: async () => [],
  getCurrentUser: async () => ({ id: "u1" }),
}));

import DashboardPage from "./page";

type Element = { type: unknown; props: Record<string, unknown> & { children?: unknown } };

function find(node: unknown, type: unknown): Element | undefined {
  if (!node || typeof node !== "object") {
    return undefined;
  }
  const element = node as Element;
  if (element.type === type) {
    return element;
  }
  const children = element.props?.children;
  for (const child of Array.isArray(children) ? children : [children]) {
    const found = find(child, type);
    if (found) {
      return found;
    }
  }
  return undefined;
}

beforeEach(() => {
  calls.length = 0;
});

describe("dashboard page", () => {
  it("requests the daily balance for the month in view and the owner filter", async () => {
    const page = await DashboardPage({ searchParams: Promise.resolve({ month: "2026-09", owner: "shared" }) });
    const daily = calls.find((c) => c.path === "/api/v1/summary/daily");
    expect(daily?.query).toEqual({ from: "2026-09-01", to: "2026-09-30", owner: "shared" });
    expect(find(page, BalanceTrendCard)?.props.currencies).toHaveLength(1);
  });

  it("requests the whole workspace without an owner", async () => {
    await DashboardPage({ searchParams: Promise.resolve({ month: "2026-02" }) });
    expect(calls.find((c) => c.path === "/api/v1/summary/daily")?.query).toEqual({ from: "2026-02-01", to: "2026-02-28", owner: undefined });
  });
});
