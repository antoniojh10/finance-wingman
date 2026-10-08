import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { budgetState, type BudgetCurrencyData, type BudgetLineData } from "@/lib/budgets";
import { renderWithIntl } from "@/test/render";

import { BudgetsCard } from "./budgets-card";

function line(name: string, amount: number | null, spent: number, committed = 0): BudgetLineData {
  return {
    category_id: name,
    category_name: name,
    category_color: null,
    amount,
    amount_month: amount === null ? null : "2026-10",
    spent,
    committed,
    remaining: amount === null ? null : amount - spent - committed,
    state: budgetState(amount, spent + committed),
  };
}

function currency(code: string, categories: BudgetLineData[]): BudgetCurrencyData {
  return { currency: code, minor_units: 2, budgeted: 0, spent: 0, committed: 0, remaining: 0, categories, uncategorized: null };
}

const href = "/budgets?month=2026-10";

describe("BudgetsCard", () => {
  it("lists only near and over categories, over budget first", () => {
    renderWithIntl(
      <BudgetsCard
        href={href}
        currencies={[
          currency("MXN", [line("Fun", 10000, 2000), line("Food", 10000, 8500), line("Rent", 10000, 12000), line("Misc", null, 500)]),
          currency("USD", [line("Travel", 10000, 100)]),
        ]}
      />,
    );
    const mxn = screen.getByTestId("budgets-attention-MXN");
    const rows = within(mxn).getAllByTestId("budget-attention-row");
    expect(rows).toHaveLength(2);
    expect(within(rows[0]).getByText("Rent")).toBeInTheDocument();
    expect(within(rows[0]).getByText("Over budget")).toBeInTheDocument();
    expect(within(rows[1]).getByText("Food")).toBeInTheDocument();
    expect(within(rows[1]).getByText("Near limit")).toBeInTheDocument();
    expect(screen.queryByText("Fun")).not.toBeInTheDocument();
    expect(screen.queryByText("Misc")).not.toBeInTheDocument();
    expect(screen.queryByTestId("budgets-attention-USD")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "View all" })).toHaveAttribute("href", href);
  });

  it("counts committed recurring towards the state", () => {
    renderWithIntl(<BudgetsCard href={href} currencies={[currency("MXN", [line("Food", 10000, 5000, 4000)])]} />);
    expect(screen.getByText("Near limit")).toBeInTheDocument();
  });

  it("says everything is on track when no category needs attention", () => {
    renderWithIntl(<BudgetsCard href={href} currencies={[currency("MXN", [line("Food", 10000, 1000)])]} />);
    expect(screen.getByTestId("budgets-on-track")).toBeInTheDocument();
    expect(screen.queryByTestId("budget-attention-row")).not.toBeInTheDocument();
  });

  it("invites to set up budgets when none exist", () => {
    renderWithIntl(<BudgetsCard href={href} currencies={[currency("MXN", [line("Food", null, 1000)])]} />);
    expect(screen.getByRole("button", { name: "Set up budgets" })).toHaveAttribute("href", href);
    expect(screen.queryByTestId("budgets-on-track")).not.toBeInTheDocument();
  });

  it("is translated", () => {
    renderWithIntl(<BudgetsCard href={href} currencies={[]} />, { locale: "es" });
    expect(screen.getByRole("button", { name: "Configurar presupuestos" })).toBeInTheDocument();
  });
});
