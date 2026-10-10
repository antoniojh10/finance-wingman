import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { parseReportsQuery, type MonthlyCurrency } from "@/lib/reports";
import { renderWithIntl } from "@/test/render";

import { ReportsView } from "./reports-view";

function currency(code: string, spend: number): MonthlyCurrency {
  return {
    currency: code,
    minor_units: 2,
    months: ["2026-08", "2026-09", "2026-10"],
    partial_month: "2026-10",
    categories: spend
      ? [{ category_id: "c1", name: "Food", color: "#12a150", archived: false, totals: [spend, spend, spend], budgets: [null, null, null] }]
      : [],
    totals: [spend, spend, spend],
    budgets: [null, null, null],
  };
}

const ana = "0b9e7a52-3b6c-4f3e-9c1a-1d2e3f4a5b6c";
const luis = "1c9e7a52-3b6c-4f3e-9c1a-1d2e3f4a5b6d";
const owners = [
  { id: ana, name: "Ana" },
  { id: luis, name: "Luis" },
];

function renderView(currencies: MonthlyCurrency[], params: Record<string, string> = {}) {
  return renderWithIntl(<ReportsView currencies={currencies} query={parseReportsQuery(params)} owners={owners} userId={ana} />);
}

describe("ReportsView", () => {
  it("shows the chart for the first currency by default", () => {
    renderView([currency("EUR", 1000), currency("USD", 500)]);
    expect(screen.getByRole("heading", { name: "Monthly spending by category" })).toBeInTheDocument();
    expect(screen.getByTestId("reports-chart")).toBeInTheDocument();
    expect(screen.getByTestId("reports-subtitle")).toHaveTextContent("EUR");
  });

  it("shows the requested currency and links the selector with the other filters", () => {
    renderView([currency("EUR", 1000), currency("USD", 500)], { currency: "USD", months: "3" });
    expect(screen.getByTestId("reports-subtitle")).toHaveTextContent("USD");
    const nav = screen.getByRole("navigation", { name: "Currency" });
    expect(within(nav).getByRole("link", { name: "USD" })).toHaveAttribute("aria-current", "page");
    expect(within(nav).getByRole("link", { name: "EUR" })).toHaveAttribute("href", "/reports?months=3&currency=EUR");
  });

  it("hides the currency selector when there is only one", () => {
    renderView([currency("EUR", 1000)]);
    expect(screen.queryByRole("navigation", { name: "Currency" })).not.toBeInTheDocument();
  });

  it("falls back to the first currency when the requested one has no data", () => {
    renderView([currency("EUR", 1000)], { currency: "JPY" });
    expect(screen.getByTestId("reports-subtitle")).toHaveTextContent("EUR");
  });

  it("links the period, scale and current-month filters", () => {
    renderView([currency("EUR", 1000)], { months: "12", scale: "share" });
    const period = screen.getByRole("navigation", { name: "Period" });
    expect(within(period).getByRole("link", { name: "12 months" })).toHaveAttribute("aria-current", "page");
    expect(within(period).getByRole("link", { name: "3 months" })).toHaveAttribute("href", "/reports?months=3&scale=share");
    const scale = screen.getByRole("navigation", { name: "Scale" });
    expect(within(scale).getByRole("link", { name: "% of month" })).toHaveAttribute("aria-current", "page");
    expect(within(scale).getByRole("link", { name: "Amount" })).toHaveAttribute("href", "/reports?months=12");
    const toggle = screen.getByRole("switch", { name: "Include the current month" });
    expect(toggle).toHaveAttribute("aria-checked", "true");
    expect(toggle).toHaveAttribute("href", "/reports?months=12&scale=share&current=0");
  });

  it("reflects a hidden current month", () => {
    renderView([currency("EUR", 1000)], { current: "0" });
    const toggle = screen.getByRole("switch", { name: "Include the current month" });
    expect(toggle).toHaveAttribute("aria-checked", "false");
    expect(toggle).toHaveAttribute("href", "/reports");
  });

  it("keeps the other filters in the owner links and explains the missing budget line", () => {
    renderView([currency("EUR", 1000)], { months: "3", owner: "shared" });
    const owner = screen.getByRole("navigation", { name: "Whose accounts" });
    expect(within(owner).getByRole("link", { name: "Shared" })).toHaveAttribute("aria-current", "page");
    expect(within(owner).getByRole("link", { name: "Mine" })).toHaveAttribute("href", `/reports?months=3&owner=${ana}`);
    expect(within(owner).getByRole("link", { name: "Everyone" })).toHaveAttribute("href", "/reports?months=3");
    expect(screen.getByText(/budget line is hidden/)).toBeInTheDocument();
  });

  it("shows an empty state when there are no expenses", () => {
    renderView([currency("EUR", 0)]);
    expect(screen.getByTestId("reports-empty")).toHaveTextContent("No expenses in this period");
    expect(screen.queryByTestId("reports-chart")).not.toBeInTheDocument();
  });

  it("shows an empty state without any currency", () => {
    renderView([]);
    expect(screen.getByTestId("reports-empty")).toBeInTheDocument();
    expect(screen.queryByRole("navigation", { name: "Currency" })).not.toBeInTheDocument();
  });
});
