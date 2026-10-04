import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { renderWithIntl } from "@/test/render";

import { CategoryBreakdown, toRows, type CategoryTotal } from "./category-breakdown";
import { SummaryCards } from "./summary-cards";

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

describe("SummaryCards", () => {
  it("shows per-currency totals and only highlights positive amounts", () => {
    renderWithIntl(
      <SummaryCards summary={{ currency: "USD", minor_units: 2, income: 0, expense: 1299, net: -1299, balance: 138701 }} />,
    );
    expect(screen.getByText("Income · USD")).toBeInTheDocument();
    expect(screen.getByText("$0.00")).not.toHaveClass("text-emerald-700");
    expect(screen.getByText("$12.99")).toBeInTheDocument();
    expect(screen.getByText("−$12.99")).toBeInTheDocument();
    expect(screen.getByText("$1,387.01")).toBeInTheDocument();
  });
});
