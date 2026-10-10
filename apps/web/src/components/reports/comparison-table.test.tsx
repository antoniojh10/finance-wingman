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
    expect(screen.getAllByRole("columnheader", { name: "Δ Sep" })).toHaveLength(2);
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
    expect(cells[2].getAttribute("style")).toContain("color-mix");
    expect(cells[5].getAttribute("style")).toBeNull();
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

  it("orders Average and Δ right after the category on phones and last from md up", () => {
    renderTable();
    const classes = (el: Element) => el.className;
    const mobile = (cls: string) => cls.includes("md:hidden");
    const desktop = (cls: string) => cls.includes("hidden") && cls.includes("md:table-cell");

    const headers = screen.getAllByRole("columnheader").map(classes);
    expect(mobile(headers[1])).toBe(true);
    expect(mobile(headers[2])).toBe(true);
    expect(desktop(headers[headers.length - 2])).toBe(true);
    expect(desktop(headers[headers.length - 1])).toBe(true);

    for (const id of ["comparison-row-c1", "comparison-total"]) {
      const row = screen.getByTestId(id);
      const cells = Array.from(row.children).map(classes);
      expect(row.querySelector('[data-testid="comparison-average-mobile"]')).toBe(row.children[1]);
      expect(row.querySelector('[data-testid="comparison-delta-mobile"]')).toBe(row.children[2]);
      expect(mobile(cells[1]) && mobile(cells[2])).toBe(true);
      expect(row.querySelector('[data-testid="comparison-average"]')).toBe(row.children[row.children.length - 2]);
      expect(row.querySelector('[data-testid="comparison-delta"]')).toBe(row.children[row.children.length - 1]);
      expect(desktop(cells[cells.length - 2]) && desktop(cells[cells.length - 1])).toBe(true);
      expect(row.children).toHaveLength(headers.length);
    }
  });
});
