import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { MonthlyCurrency } from "@/lib/reports";
import { renderWithIntl } from "@/test/render";

import { ComparisonTable } from "./comparison-table";

const data: MonthlyCurrency = {
  currency: "EUR",
  minor_units: 2,
  months: ["2026-07", "2026-08", "2026-09", "2026-10"],
  partial_month: "2026-10",
  categories: [
    { category_id: "c1", name: "Food", color: "#12a150", archived: false, totals: [4000, 3000, 6000, 1000], budgets: [null, null, null, null] },
    { category_id: null, name: null, color: null, archived: false, totals: [100, 0, 100, 0], budgets: [null, null, null, null] },
  ],
  totals: [4100, 3000, 6100, 1000],
  budgets: [null, null, null, null],
};

function renderTable(props: { includeCurrent?: boolean; owner?: string } = {}) {
  return renderWithIntl(<ComparisonTable data={data} months={6} includeCurrent={props.includeCurrent ?? true} owner={props.owner} />);
}

describe("ComparisonTable", () => {
  it("lists each category by month with average and change", () => {
    renderTable();
    const food = screen.getByTestId("comparison-row-c1");
    expect(within(food).getByRole("link", { name: /40[.,]00/ })).toHaveAttribute(
      "href",
      "/transactions?type=expense&from=2026-07-01&to=2026-07-31&category_id=c1",
    );
    expect(within(food).getByTestId("comparison-average")).toHaveTextContent(/43[.,]33/);
    expect(within(food).getByTestId("comparison-delta")).toHaveTextContent("+38%");
    expect(within(food).getByTestId("comparison-delta")).toHaveAttribute("data-tone", "up");
    expect(screen.getByRole("columnheader", { name: "Δ Sep" })).toBeInTheDocument();
  });

  it("ends with a total row", () => {
    renderTable();
    const total = screen.getByTestId("comparison-total");
    expect(within(total).getByText("Total")).toBeInTheDocument();
    expect(within(total).getByTestId("comparison-average")).toHaveTextContent(/44[.,]00/);
  });

  it("links uncategorized expenses without a category filter", () => {
    renderTable();
    const row = screen.getByTestId("comparison-row-uncategorized");
    expect(within(row).getAllByRole("link")[0]).toHaveAttribute("href", "/transactions?type=expense&from=2026-07-01&to=2026-07-31");
  });

  it("marks the month in progress and does not shade it", () => {
    renderTable();
    expect(screen.getByText("(in progress)")).toBeInTheDocument();
    const cells = within(screen.getByTestId("comparison-row-c1")).getAllByRole("cell");
    expect(cells[0].getAttribute("style")).toContain("color-mix");
    expect(cells[3].getAttribute("style")).toBeNull();
  });

  it("keeps the owner filter in the links and drops the current month when hidden", () => {
    renderTable({ owner: "shared", includeCurrent: false });
    expect(within(screen.getByTestId("comparison-row-c1")).getAllByRole("link")[0]).toHaveAttribute(
      "href",
      expect.stringContaining("owner=shared"),
    );
    expect(screen.queryByText("(in progress)")).not.toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: /Oct/ })).not.toBeInTheDocument();
  });
});
