import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { renderWithIntl } from "@/test/render";

import { CategoryBreakdown, toRows, type CategoryTotal } from "./category-breakdown";
import { AccountStrip } from "./account-strip";
import { BalanceOverview, type CurrencySummary } from "./balance-overview";

const total = (id: string | null, name: string | null, amount: number, count = 1): CategoryTotal => ({
  category_id: id,
  category_name: name,
  category_color: id ? "#2a78d6" : null,
  total: amount,
  transaction_count: count,
});

describe("toRows", () => {
  const labels = { uncategorized: "Uncategorized", other: "Other" };

  it("sorts by total and labels uncategorized spending", () => {
    const rows = toRows([total("a", "Food", 100), total(null, null, 300)], labels);
    expect(rows.map((r) => [r.name, r.total])).toEqual([
      ["Uncategorized", 300],
      ["Food", 100],
    ]);
  });

  it("folds categories beyond the sixth into Other", () => {
    const totals = Array.from({ length: 9 }, (_, i) => total(`c${i}`, `Cat ${i}`, 900 - i * 100, 2));
    const rows = toRows(totals, labels);
    expect(rows).toHaveLength(6);
    expect(rows[5]).toMatchObject({ name: "Other", total: 400 + 300 + 200 + 100, count: 8 });
    expect(rows.reduce((sum, r) => sum + r.total, 0)).toBe(totals.reduce((sum, t) => sum + t.total, 0));
  });
});

describe("CategoryBreakdown", () => {
  it("shows an empty state", () => {
    renderWithIntl(<CategoryBreakdown totals={[]} currency="MXN" minorUnits={2} />);
    expect(screen.getByText("No expenses this month.")).toBeInTheDocument();
  });

  it("renders one row per category with its amount", () => {
    renderWithIntl(
      <CategoryBreakdown totals={[total("a", "Rent", 1200000), total("b", "Food", 300000, 4)]} currency="USD" minorUnits={2} />,
    );
    const items = screen.getAllByRole("listitem");
    expect(items).toHaveLength(2);
    expect(within(items[0]).getByText("Rent")).toBeInTheDocument();
    expect(within(items[0]).getByText("$12,000.00")).toBeInTheDocument();
    expect(within(items[1]).getByText(/20% of expenses · 4 transactions/)).toBeInTheDocument();
  });
});

const usd: CurrencySummary = { currency: "USD", minor_units: 2, income: 0, expense: 1299, net: -1299, balance: 138701, expenses: [] };
const mxn: CurrencySummary = {
  currency: "MXN",
  minor_units: 2,
  income: 500000,
  expense: 120000,
  net: 380000,
  balance: 2500000,
  expenses: [total("a", "Rent", 120000)],
};

describe("BalanceOverview", () => {
  it("shows one currency's totals and only highlights a positive net", () => {
    renderWithIntl(<BalanceOverview currencies={[usd]} />);
    const hero = screen.getByRole("region", { name: "Balance" });
    expect(within(hero).getByText("$1,387.01")).toBeInTheDocument();
    expect(within(hero).getByText("$0.00")).toBeInTheDocument();
    expect(within(hero).getByText("$12.99")).toBeInTheDocument();
    expect(within(hero).getByText("−$12.99")).not.toHaveClass("text-lime");
    // A single currency needs no switcher.
    expect(screen.queryByRole("button", { name: "USD" })).not.toBeInTheDocument();
    expect(screen.getByText("No expenses this month.")).toBeInTheDocument();
  });

  it("switches between currencies without combining them", async () => {
    const user = userEvent.setup();
    renderWithIntl(<BalanceOverview currencies={[mxn, usd]} />);
    const hero = screen.getByRole("region", { name: "Balance" });
    expect(screen.getByRole("button", { name: "MXN" })).toHaveAttribute("aria-pressed", "true");
    expect(within(hero).getByText("+MX$3,800.00")).toHaveClass("text-lime");
    expect(screen.getByText("Rent")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "USD" }));
    expect(screen.getByRole("button", { name: "USD" })).toHaveAttribute("aria-pressed", "true");
    expect(within(hero).getByText("$1,387.01")).toBeInTheDocument();
    expect(screen.queryByText("Rent")).not.toBeInTheDocument();
  });
});

describe("AccountStrip", () => {
  it("lists each account with its type and balance", () => {
    renderWithIntl(
      <AccountStrip
        accounts={[
          { id: "1", name: "Wallet", type: "cash", currency: "MXN", minor_units: 2, balance: 192000 },
          { id: "2", name: "Card", type: "credit_card", currency: "MXN", minor_units: 2, balance: -50015 },
        ]}
      />,
    );
    const items = screen.getAllByRole("listitem");
    expect(within(items[0]).getByText("Cash")).toBeInTheDocument();
    expect(within(items[0]).getByText("MX$1,920.00")).toBeInTheDocument();
    expect(within(items[1]).getByText("−MX$500.15")).toHaveClass("text-expense");
  });
});
