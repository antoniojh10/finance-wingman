import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { AccountList, groupByCurrency, type AccountListItem } from "./account-list";

const setAccountArchived = vi.fn();
vi.mock("@/app/actions/accounts", () => ({
  saveAccount: vi.fn(),
  deleteAccount: vi.fn(),
  setAccountArchived: (...args: unknown[]) => setAccountArchived(...args),
}));

const account = (overrides: Partial<AccountListItem>): AccountListItem => ({
  id: "a",
  name: "Checking",
  type: "checking",
  currency: "MXN",
  minor_units: 2,
  initial_balance: 0,
  balance_as_of: "2026-09-01",
  balance: 100000,
  archived: false,
  ...overrides,
});

const accounts = [
  account({ id: "a" }),
  account({ id: "b", name: "Card", type: "credit_card", balance: -25000 }),
  account({ id: "c", name: "Old", archived: true, balance: 999900 }),
  account({ id: "d", name: "Wise", currency: "USD", balance: 321000 }),
];
const currencies = [{ code: "MXN", name: "Mexican Peso" }];

beforeEach(() => setAccountArchived.mockReset());

describe("groupByCurrency", () => {
  it("totals only active accounts per currency", () => {
    expect(groupByCurrency(accounts).map((g) => [g.currency, g.total, g.active, g.accounts.length])).toEqual([
      ["MXN", 75000, 2, 3],
      ["USD", 321000, 1, 1],
    ]);
  });
});

describe("AccountList", () => {
  it("shows a section per currency with its total", () => {
    renderWithIntl(<AccountList accounts={accounts} currencies={currencies} defaultCurrency="MXN" defaultDate="2026-10-05" />);
    const mxn = screen.getByRole("region", { name: "MXN" });
    expect(within(mxn).getByText("2 active accounts")).toBeInTheDocument();
    expect(within(mxn).getByText("MX$750.00")).toBeInTheDocument();
    expect(within(mxn).getByText("−MX$250.00")).toHaveClass("text-expense");
    expect(within(mxn).getByText("Archived")).toBeInTheDocument();
    expect(within(screen.getByRole("region", { name: "USD" })).getAllByText("$3,210.00")).toHaveLength(2);
  });

  it("archives an account from its menu", async () => {
    setAccountArchived.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderWithIntl(<AccountList accounts={accounts.slice(0, 1)} currencies={currencies} defaultCurrency="MXN" defaultDate="2026-10-05" />);
    await user.click(screen.getByRole("button", { name: "Actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Archive" }));
    await waitFor(() => expect(setAccountArchived).toHaveBeenCalledWith("a", true));
  });

  it("shows an empty state", () => {
    renderWithIntl(<AccountList accounts={[]} currencies={currencies} defaultCurrency="MXN" defaultDate="2026-10-05" />);
    expect(screen.getByText("No accounts yet.")).toBeInTheDocument();
  });

  it("shows each account's owner when the workspace has several members", () => {
    const ownership = { userId: "u1", owners: [{ id: "u1", name: "Luis" }, { id: "u2", name: "Ana" }] };
    renderWithIntl(
      <AccountList
        accounts={[account({ id: "a", owner: { id: "u2", name: "Ana", email: "ana@example.com" } }), account({ id: "b", name: "Joint" })]}
        currencies={currencies}
        defaultCurrency="MXN"
        defaultDate="2026-10-05"
        ownership={ownership}
      />,
    );
    expect(screen.getByText("Checking · Ana")).toBeInTheDocument();
    expect(screen.getByText("Checking · Shared")).toBeInTheDocument();
  });
});
