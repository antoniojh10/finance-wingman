import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { BudgetCurrencyData, BudgetLineData } from "@/lib/budgets";
import { renderWithIntl } from "@/test/render";

import { BudgetBar } from "./budget-bar";
import { BudgetList } from "./budget-list";

const line = (overrides: Partial<BudgetLineData>): BudgetLineData => ({
  category_id: "c",
  category_name: "Food",
  category_color: null,
  amount: 100000,
  amount_month: "2026-10",
  spent: 0,
  committed: 0,
  remaining: 100000,
  state: "ok",
  ...overrides,
});

const mxn: BudgetCurrencyData = {
  currency: "MXN",
  minor_units: 2,
  budgeted: 300000,
  spent: 120000,
  committed: 50000,
  remaining: 130000,
  categories: [
    line({ category_id: "food", category_name: "Food", spent: 90000, committed: 0, remaining: 10000, amount: 100000, state: "near" }),
    line({ category_id: "rent", category_name: "Rent", amount: 200000, amount_month: "2026-03", spent: 30000, committed: 50000, remaining: 120000, state: "ok" }),
    line({ category_id: "fun", category_name: "Fun", amount: null, amount_month: null, spent: 5000, remaining: null, state: "none" }),
    line({ category_id: "gym", category_name: "Gym", amount: 10000, spent: 15000, remaining: -5000, state: "over" }),
  ],
  uncategorized: line({ category_id: null, category_name: null, amount: null, amount_month: null, spent: 2500, remaining: null, state: "none" }),
};

function row(name: string): HTMLElement {
  return screen.getAllByTestId("budget-row").find((r) => within(r).queryByText(name))!;
}

describe("BudgetList", () => {
  it("shows the totals of each currency", () => {
    renderWithIntl(<BudgetList currencies={[mxn]} month="2026-10" editHref="/budgets?edit=1" />);
    const summary = within(screen.getByTestId("budget-summary"));
    expect(summary.getByText("Budgeted").nextSibling).toHaveTextContent("$3,000.00");
    expect(summary.getByText("Committed").nextSibling).toHaveTextContent("$500.00");
    expect(summary.getByText("Remaining").nextSibling).toHaveTextContent("$1,300.00");
  });

  it("shows the state, the committed segment and the figures of a line", () => {
    renderWithIntl(<BudgetList currencies={[mxn]} month="2026-10" editHref="/budgets?edit=1" />);
    const rent = within(row("Rent"));
    expect(rent.getByText("On track")).toBeInTheDocument();
    expect(rent.getByText("Committed").nextSibling).toHaveTextContent("$500.00");
    expect(rent.getByText("Remaining").nextSibling).toHaveTextContent("$1,200.00");
    expect(within(row("Food")).getByText("Near limit")).toBeInTheDocument();
    const gym = within(row("Gym"));
    expect(gym.getByText("Over budget")).toBeInTheDocument();
    expect(gym.getByText("Over by").nextSibling).toHaveTextContent("$50.00");
  });

  it("marks an inherited amount with the month it was set in", () => {
    renderWithIntl(<BudgetList currencies={[mxn]} month="2026-10" editHref="/budgets?edit=1" />);
    expect(within(row("Rent")).getByTestId("inherited-hint")).toHaveTextContent("from March 2026");
    expect(within(row("Food")).queryByTestId("inherited-hint")).not.toBeInTheDocument();
  });

  it("lists spending without a budget and uncategorized spending without a bar", () => {
    renderWithIntl(<BudgetList currencies={[mxn]} month="2026-10" editHref="/budgets?edit=1" />);
    const fun = within(row("Fun"));
    expect(fun.getByText("No budget")).toBeInTheDocument();
    expect(fun.queryByRole("progressbar")).not.toBeInTheDocument();
    const uncategorized = within(row("Uncategorized"));
    expect(uncategorized.getByText("Spent").nextSibling).toHaveTextContent("$25.00");
    expect(uncategorized.queryByRole("progressbar")).not.toBeInTheDocument();
  });

  it("offers to set budgets when there is nothing to show", () => {
    renderWithIntl(<BudgetList currencies={[]} month="2026-10" editHref="/budgets?month=2026-10&edit=1" />);
    expect(screen.getByRole("button", { name: "Set budgets" })).toHaveAttribute("href", "/budgets?month=2026-10&edit=1");
  });
});

describe("BudgetBar", () => {
  it("draws the spent and committed segments apart", () => {
    const { container } = renderWithIntl(<BudgetBar amount={1000} spent={250} committed={500} state="near" />);
    const bar = screen.getByRole("progressbar", { name: "Budget used" });
    expect(bar).toHaveAttribute("aria-valuenow", "75");
    expect(bar).toHaveAttribute("data-state", "near");
    expect(container.querySelector("[data-segment=spent]")).toHaveStyle({ width: "25%" });
    expect(container.querySelector("[data-segment=committed]")).toHaveStyle({ width: "50%" });
  });

  it("caps the value at 100 while reporting the real usage", () => {
    renderWithIntl(<BudgetBar amount={1000} spent={1500} committed={0} state="over" />);
    const bar = screen.getByRole("progressbar");
    expect(bar).toHaveAttribute("aria-valuenow", "100");
    expect(bar).toHaveAttribute("aria-valuetext", "150% used");
  });
});
