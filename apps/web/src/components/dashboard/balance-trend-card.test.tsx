import { fireEvent, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import type { DailyCurrency } from "@/lib/balance-trend";
import { renderWithIntl } from "@/test/render";

import { BalanceTrendCard } from "./balance-trend-card";

const day = (date: string, balance: number, income = 0, expense = 0) => ({ date, balance, income, expense, savings_balance: 0, investment_balance: 0 });

function currency(code: string, overrides: Partial<DailyCurrency> = {}): DailyCurrency {
  return {
    currency: code,
    minor_units: 2,
    days: [day("2026-11-01", 100000), day("2026-11-02", 90000, 0, 10000)],
    projection: [
      { date: "2026-11-03", balance: 85000 },
      { date: "2026-11-04", balance: 40000 },
      { date: "2026-11-05", balance: 60000 },
    ],
    projection_basis: {
      average_daily_expense: 5000,
      recurring: [{ recurring_id: "r1", name: "Rent", amount: -45000, applied_on: "2026-11-04", due_on: "2026-11-04" }],
    },
    ...overrides,
  };
}

const past = { projection: [], projection_basis: { average_daily_expense: 0, recurring: [] } };

describe("BalanceTrendCard", () => {
  it("draws the real balance, the projection and its caption", () => {
    renderWithIntl(<BalanceTrendCard currencies={[currency("EUR")]} />);
    expect(screen.getByRole("heading", { name: "Balance trend" })).toBeInTheDocument();
    expect(screen.getByTestId("balance-trend-caption")).toHaveTextContent("Projected from pending recurring items and your average daily spending (€50.00/day)");
    expect(screen.getByTestId("balance-line")).toBeInTheDocument();
    expect(screen.getByTestId("projection-line")).toHaveAttribute("stroke-dasharray");
    expect(screen.getByTestId("today-marker")).toHaveTextContent("Today");
    expect(screen.getByTestId("end-label")).toHaveTextContent("€600");
    expect(screen.getByTestId("low-label")).toHaveTextContent("Low €400");
    // A single currency needs no switcher.
    expect(screen.queryByRole("button", { name: "EUR" })).not.toBeInTheDocument();
  });

  it("switches between currencies", async () => {
    const user = userEvent.setup();
    renderWithIntl(<BalanceTrendCard currencies={[currency("EUR"), currency("USD", { days: [day("2026-11-01", 5000), day("2026-11-02", 7000)] })]} />);
    expect(screen.getByRole("button", { name: "EUR" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByTestId("balance-trend-caption")).toHaveTextContent("€50.00/day");
    await user.click(screen.getByRole("button", { name: "USD" }));
    expect(screen.getByRole("button", { name: "USD" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByTestId("balance-trend-caption")).toHaveTextContent("$50.00/day");
  });

  it("shows an empty state when no account has days in range", () => {
    renderWithIntl(<BalanceTrendCard currencies={[currency("EUR", { days: [], projection: [] })]} />);
    expect(screen.getByTestId("balance-trend-empty")).toHaveTextContent("No accounts with a balance in this period.");
    expect(screen.queryByTestId("balance-trend-chart")).not.toBeInTheDocument();
    renderWithIntl(<BalanceTrendCard currencies={[]} />);
    expect(screen.getAllByTestId("balance-trend-empty")).toHaveLength(2);
  });

  it("shows only real balances for a past month", () => {
    renderWithIntl(<BalanceTrendCard currencies={[currency("EUR", past)]} />);
    expect(screen.getByTestId("balance-line")).toBeInTheDocument();
    expect(screen.queryByTestId("projection-line")).not.toBeInTheDocument();
    expect(screen.queryByTestId("today-marker")).not.toBeInTheDocument();
    expect(screen.queryByTestId("end-label")).not.toBeInTheDocument();
    expect(screen.queryByTestId("recurring-dot")).not.toBeInTheDocument();
    expect(screen.queryByTestId("balance-trend-caption")).not.toBeInTheDocument();
  });

  it("shows only the projection for a future month", () => {
    renderWithIntl(<BalanceTrendCard currencies={[currency("EUR", { days: [] })]} />);
    expect(screen.queryByTestId("balance-line")).not.toBeInTheDocument();
    expect(screen.getByTestId("projection-line")).toBeInTheDocument();
    expect(screen.queryByTestId("today-marker")).not.toBeInTheDocument();
  });

  it("draws a zero line when the balance crosses 0", () => {
    const negative = currency("EUR", { projection: [{ date: "2026-11-03", balance: -20000 }], projection_basis: { average_daily_expense: 0, recurring: [] } });
    renderWithIntl(<BalanceTrendCard currencies={[negative]} />);
    expect(screen.getByTestId("zero-line")).toBeInTheDocument();
  });
});

describe("BalanceTrendChart interactions", () => {
  it("marks recurring occurrences and shows their name and amount on hover", () => {
    renderWithIntl(<BalanceTrendCard currencies={[currency("EUR")]} />);
    const dot = screen.getByRole("img", { name: /Rent, .*: -€450\.00/ });
    fireEvent.pointerEnter(dot);
    const tip = screen.getByRole("tooltip");
    expect(tip).toHaveTextContent("Rent");
    expect(tip).toHaveTextContent("-€450.00");
    fireEvent.pointerLeave(dot);
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
  });

  it("shows the day net on real days and a projected label on projected ones", () => {
    renderWithIntl(<BalanceTrendCard currencies={[currency("EUR")]} />);
    const overlay = screen.getByTestId("balance-trend-overlay");
    // The chart is 640px wide in jsdom: left 58, right 624, five days.
    fireEvent.pointerMove(overlay, { clientX: 58 + 566 / 4 });
    let tip = screen.getByRole("tooltip");
    expect(tip).toHaveTextContent("€900.00");
    expect(tip).toHaveTextContent("Balance");
    expect(tip).toHaveTextContent("Day net");
    expect(tip).toHaveTextContent("-€100.00");
    expect(screen.getByTestId("crosshair")).toBeInTheDocument();

    fireEvent.pointerMove(overlay, { clientX: 624 });
    tip = screen.getByRole("tooltip");
    expect(tip).toHaveTextContent("€600.00");
    expect(tip).toHaveTextContent("Projected balance");
    expect(tip).not.toHaveTextContent("Day net");
  });

  it("moves between days with the arrow keys", () => {
    renderWithIntl(<BalanceTrendCard currencies={[currency("EUR")]} />);
    const overlay = screen.getByTestId("balance-trend-overlay");
    fireEvent.keyDown(overlay, { key: "ArrowRight" });
    expect(screen.getByRole("tooltip")).toHaveTextContent("€1,000.00");
    fireEvent.keyDown(overlay, { key: "ArrowRight" });
    expect(screen.getByRole("tooltip")).toHaveTextContent("€900.00");
    fireEvent.blur(overlay);
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
  });
});
