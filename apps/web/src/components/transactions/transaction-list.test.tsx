import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { groupByDay, TransactionList } from "./transaction-list";
import type { AccountOption, CategoryOption, TransactionRow } from "./types";

vi.mock("@/app/actions/transactions", () => ({ saveTransaction: vi.fn(), deleteTransaction: vi.fn() }));

const accounts: AccountOption[] = [
  { id: "mxn", name: "Checking", currency: "MXN", minor_units: 2, archived: false },
  { id: "sav", name: "Savings", currency: "MXN", minor_units: 2, archived: false },
];
const categories: CategoryOption[] = [{ id: "food", name: "Food", kind: "expense", color: "#ff6b4a", archived: false }];

function tx(overrides: Partial<TransactionRow>): TransactionRow {
  return {
    id: "t1",
    type: "expense",
    account_id: "mxn",
    account_name: "Checking",
    currency: "MXN",
    minor_units: 2,
    amount: 12550,
    destination_account_id: null,
    destination_account_name: null,
    destination_currency: null,
    destination_minor_units: null,
    destination_amount: null,
    category_id: "food",
    category_name: "Food",
    description: "Lunch",
    occurred_on: "2026-10-05",
    ...overrides,
  };
}

const rows = [
  tx({ id: "a", created_by: { name: "Ana", email: "ana@example.com" } }),
  tx({ id: "b", type: "income", category_id: null, category_name: null, description: "Salary", amount: 2125000, occurred_on: "2026-10-04" }),
  tx({
    id: "c",
    type: "transfer",
    category_id: null,
    category_name: null,
    description: "",
    destination_account_id: "sav",
    destination_account_name: "Savings",
    amount: 500000,
    occurred_on: "2026-10-01",
  }),
];

describe("groupByDay", () => {
  it("keeps consecutive transactions of the same day together", () => {
    expect(groupByDay(rows).map((g) => [g.date, g.items.length])).toEqual([
      ["2026-10-05", 1],
      ["2026-10-04", 1],
      ["2026-10-01", 1],
    ]);
  });
});

describe("TransactionList", () => {
  it("shows signed amounts by type and who recorded each transaction", () => {
    renderWithIntl(<TransactionList transactions={rows} accounts={accounts} categories={categories} defaultDate="2026-10-05" />);
    const [expense, income, transfer] = screen.getAllByTestId("transaction-row");
    expect(within(expense).getByText("−MX$125.50")).toBeInTheDocument();
    expect(within(expense).getByText("Recorded by Ana")).toBeInTheDocument();
    expect(within(income).getByText("+MX$21,250.00")).toHaveClass("text-income");
    expect(within(transfer).getByText("Transfer")).toBeInTheDocument();
    expect(within(transfer).getByText("MX$5,000.00")).toBeInTheDocument();
  });

  it("groups by day with relative labels", () => {
    renderWithIntl(
      <TransactionList transactions={rows} accounts={accounts} categories={categories} defaultDate="2026-10-05" groupByDate />,
    );
    expect(screen.getByRole("heading", { name: "Today · Oct 5, 2026" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Yesterday · Oct 4, 2026" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Oct 1, 2026" })).toBeInTheDocument();
  });

  it("opens the edit dialog from the row menu", async () => {
    const user = userEvent.setup();
    renderWithIntl(<TransactionList transactions={rows.slice(0, 1)} accounts={accounts} categories={categories} defaultDate="2026-10-05" />);
    await user.click(screen.getByRole("button", { name: "Actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Edit" }));
    expect(await screen.findByRole("dialog", { name: "Edit transaction" })).toBeInTheDocument();
  });

  it("shows the subscription a transaction pays", () => {
    renderWithIntl(
      <TransactionList
        transactions={[tx({ id: "p", recurring_id: "r1" }), tx({ id: "q", recurring_id: null })]}
        accounts={accounts}
        categories={categories}
        defaultDate="2026-10-05"
        recurringNames={{ r1: "Netflix" }}
      />,
    );
    const [linked, plain] = screen.getAllByTestId("transaction-row");
    expect(within(linked).getByTestId("transaction-subscription")).toHaveTextContent("Subscription: Netflix");
    expect(within(plain).queryByTestId("transaction-subscription")).not.toBeInTheDocument();
  });

  it("shows an empty state", () => {
    renderWithIntl(<TransactionList transactions={[]} accounts={accounts} categories={categories} defaultDate="2026-10-05" />);
    expect(screen.getByText("No transactions found.")).toBeInTheDocument();
  });
});
