import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { MonthlyCurrency } from "@/lib/reports";
import { renderWithIntl } from "@/test/render";

import { SpendingTrendCard } from "./spending-trend-card";

function currency(code: string, totals: number[]): MonthlyCurrency {
  return {
    currency: code,
    minor_units: 2,
    months: ["2026-08", "2026-09", "2026-10"],
    partial_month: "2026-10",
    categories: [{ category_id: "rent", name: "Rent", color: "#6d4aff", archived: false, totals, budgets: totals.map(() => null) }],
    totals,
    budgets: totals.map(() => null),
  };
}

describe("SpendingTrendCard", () => {
  it("draws the compact chart of the first currency with expenses", () => {
    renderWithIntl(<SpendingTrendCard href="/reports" currencies={[currency("USD", [0, 0, 0]), currency("EUR", [100000, 90000, 80000])]} />);
    expect(screen.getByRole("heading", { name: "Spending trend" })).toBeInTheDocument();
    expect(screen.getByTestId("reports-chart")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Rent, August 2026: €1,000.00" })).toBeInTheDocument();
    expect(screen.getByTestId("reports-subtitle")).toHaveTextContent("EUR");
  });

  it("leaves out the stats, reference lines and hint of the full report", () => {
    renderWithIntl(<SpendingTrendCard href="/reports" currencies={[currency("EUR", [100000, 90000, 80000])]} />);
    expect(screen.queryByText("Period total")).not.toBeInTheDocument();
    expect(screen.queryByTestId("reference-lines")).not.toBeInTheDocument();
    expect(screen.queryByTestId("average-line")).not.toBeInTheDocument();
  });

  it("links to the report, carrying the owner filter", () => {
    renderWithIntl(<SpendingTrendCard href="/reports?owner=shared" currencies={[currency("EUR", [1, 2, 3])]} />);
    expect(screen.getByRole("link", { name: "See report" })).toHaveAttribute("href", "/reports?owner=shared");
  });

  it("shows an empty state without expenses", () => {
    renderWithIntl(<SpendingTrendCard href="/reports" currencies={[currency("EUR", [0, 0, 0])]} />);
    expect(screen.getByTestId("spending-trend-empty")).toHaveTextContent("No expenses in the last 3 months.");
    expect(screen.queryByTestId("reports-chart")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "See report" })).toBeInTheDocument();
  });
});
