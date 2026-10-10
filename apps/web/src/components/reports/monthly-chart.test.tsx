import { fireEvent, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import type { MonthlyCurrency } from "@/lib/reports";
import { renderWithIntl } from "@/test/render";

import { MonthlyChart } from "./monthly-chart";

type Cat = MonthlyCurrency["categories"][number];

function category(id: string | null, name: string | null, color: string | null, totals: number[], budgets?: (number | null)[]): Cat {
  return { category_id: id, name, color, archived: false, totals, budgets: budgets ?? totals.map(() => null) };
}

const data: MonthlyCurrency = {
  currency: "EUR",
  minor_units: 2,
  months: ["2026-08", "2026-09", "2026-10"],
  partial_month: "2026-10",
  categories: [
    category("rent", "Rent", "#6d4aff", [100000, 100000, 100000], [95000, 100000, 100000]),
    category("food", "Food", "#12a150", [40000, 50000, 10000], [45000, 45000, 45000]),
    category(null, null, null, [2000, 1000, 500]),
  ],
  totals: [142000, 151000, 110500],
  budgets: [140000, 145000, 145000],
};

function renderChart(props: Partial<React.ComponentProps<typeof MonthlyChart>> = {}) {
  return renderWithIntl(<MonthlyChart data={data} months={3} includeCurrent scale="amount" {...props} />);
}

describe("MonthlyChart", () => {
  it("draws a segment per category and month, with a label for each", () => {
    renderChart();
    expect(screen.getByRole("img", { name: "Rent, August 2026: €1,000.00" })).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Food, September 2026: €500.00" })).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "Other, August 2026: €20.00" })).toBeInTheDocument();
  });

  it("colors segments by category and gives Other a hatched fill", () => {
    renderChart();
    expect(screen.getByRole("img", { name: /^Rent, August/ })).toHaveAttribute("fill", "#6d4aff");
    expect(screen.getByRole("img", { name: /^Other, August/ }).getAttribute("fill")).toMatch(/^url\(#.*other\)$/);
  });

  it("stacks the largest category at the base", () => {
    renderChart();
    const rent = screen.getByRole("img", { name: /^Rent, August/ });
    const food = screen.getByRole("img", { name: /^Food, August/ });
    expect(Number(rent.getAttribute("y"))).toBeGreaterThan(Number(food.getAttribute("y")));
  });

  it("shows totals, stats, the average line and the budget line", () => {
    renderChart();
    expect(screen.getByText("€1.4K")).toBeInTheDocument();
    expect(screen.getByText("Period total")).toBeInTheDocument();
    expect(screen.getByTestId("average-key")).toHaveTextContent("average €1.5K");
    expect(screen.getByTestId("budget-key")).toHaveTextContent("budget €1.5K");
    expect(screen.getByTestId("reports-subtitle")).toHaveTextContent("August 2026 – October 2026 · expenses only (no transfers) · EUR");
  });

  it("marks the month in progress", () => {
    renderChart();
    expect(screen.getByText("in progress")).toBeInTheDocument();
    expect(screen.getByText("Oct*")).toBeInTheDocument();
  });

  it("leaves the month in progress out when asked", () => {
    renderChart({ includeCurrent: false, months: 3 });
    expect(screen.queryByText("in progress")).not.toBeInTheDocument();
    expect(screen.queryByRole("img", { name: /October/ })).not.toBeInTheDocument();
  });

  it("shows percentages instead of amounts and no reference lines", () => {
    renderChart({ scale: "share" });
    expect(screen.getAllByText("100%").length).toBeGreaterThan(0);
    expect(screen.queryByTestId("average-line")).not.toBeInTheDocument();
    expect(screen.queryByTestId("budget-line")).not.toBeInTheDocument();
    expect(screen.queryByTestId("reference-lines")).not.toBeInTheDocument();
  });

  it("toggles categories from the legend and drops them from the totals", async () => {
    const user = userEvent.setup();
    renderChart();
    const legend = screen.getByRole("group", { name: /Categories/ });
    const rent = within(legend).getByRole("button", { name: "Rent" });
    expect(rent).toHaveAttribute("aria-pressed", "true");

    await user.click(rent);
    expect(rent).toHaveAttribute("aria-pressed", "false");
    expect(screen.queryByRole("img", { name: /^Rent,/ })).not.toBeInTheDocument();
    expect(screen.getByRole("img", { name: /^Food, August/ })).toBeInTheDocument();
    expect(screen.queryByText("€1.4K")).not.toBeInTheDocument();

    await user.click(rent);
    expect(rent).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("img", { name: /^Rent, August/ })).toBeInTheDocument();
  });

  it("shows one category's budget when it is the only one visible", async () => {
    const user = userEvent.setup();
    renderChart();
    const legend = screen.getByRole("group", { name: /Categories/ });
    await user.click(within(legend).getByRole("button", { name: "Food" }));
    await user.click(within(legend).getByRole("button", { name: /^Other/ }));
    expect(screen.getByTestId("budget-key")).toHaveTextContent("Rent budget €1.0K");
  });

  it("lists the categories folded into Other on its chip", () => {
    renderChart();
    const chip = screen.getByRole("button", { name: "Other (1)" });
    expect(chip).toHaveAttribute("title", "Uncategorized");
  });

  it("shows the tooltip on keyboard focus", () => {
    renderChart();
    fireEvent.focus(screen.getByRole("img", { name: /^Food, September/ }));
    const tip = screen.getByRole("tooltip");
    expect(tip).toHaveTextContent("€500.00");
    expect(tip).toHaveTextContent("Food · September 2026");
    expect(tip).toHaveTextContent("Of the month");
    expect(tip).toHaveTextContent("33%");
    expect(tip).toHaveTextContent("Category average");
    expect(tip).toHaveTextContent("€450");
    expect(tip).toHaveTextContent("Vs. its average");
    expect(tip).toHaveTextContent("+11%");
  });

  it("shows the tooltip on hover and hides it on leave", () => {
    renderChart();
    const segment = screen.getByRole("img", { name: /^Rent, August/ });
    fireEvent.pointerEnter(segment, { clientX: 10, clientY: 10 });
    expect(screen.getByRole("tooltip")).toHaveTextContent("Rent · August 2026");
    fireEvent.pointerLeave(segment);
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
  });

  it("flags the month in progress in the tooltip without a comparison", () => {
    renderChart();
    fireEvent.focus(screen.getByRole("img", { name: /^Food, October/ }));
    const tip = screen.getByRole("tooltip");
    expect(tip).toHaveTextContent("(in progress)");
    expect(tip).not.toHaveTextContent("Vs. its average");
  });

  it("lists the folded categories in the Other tooltip", () => {
    renderChart();
    fireEvent.focus(screen.getByRole("img", { name: /^Other, August/ }));
    expect(screen.getByRole("tooltip")).toHaveTextContent("Other · August 2026");
    expect(screen.getByRole("tooltip")).toHaveTextContent("Uncategorized");
  });

  it("renders in Spanish", () => {
    renderWithIntl(<MonthlyChart data={data} months={3} includeCurrent scale="amount" />, { locale: "es" });
    expect(screen.getByText("Total del periodo")).toBeInTheDocument();
    expect(screen.getByText("en curso")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: /^Rent, agosto de 2026/ })).toBeInTheDocument();
  });
});
