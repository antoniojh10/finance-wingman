import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { renderWithIntl } from "@/test/render";

import { RecurringSummary } from "./recurring-summary";

describe("RecurringSummary", () => {
  it("shows monthly and yearly totals per currency", () => {
    renderWithIntl(
      <RecurringSummary
        currencies={[
          { currency: "MXN", minor_units: 2, expense: 50000, expense_count: 2, income: 20000, income_count: 1, net: -30000 },
          { currency: "USD", minor_units: 2, expense: 1000, expense_count: 1, income: 0, income_count: 0, net: -1000 },
        ]}
      />,
    );
    const mxn = screen.getByRole("group", { name: "MXN" });
    expect(within(mxn).getByText("3 active items")).toBeInTheDocument();
    const expenses = within(mxn).getByTestId("summary-MXN-expenses");
    expect(within(expenses).getByText("MX$500.00")).toBeInTheDocument();
    expect(within(expenses).getByText("MX$6,000.00")).toBeInTheDocument();
    const net = within(mxn).getByTestId("summary-MXN-net");
    expect(within(net).getByText("−MX$300.00")).toBeInTheDocument();
    expect(within(net).getByText("−MX$3,600.00")).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "USD" })).toBeInTheDocument();
  });

  it("renders nothing without active items", () => {
    const { container } = renderWithIntl(<RecurringSummary currencies={[]} />);
    expect(container).toBeEmptyDOMElement();
  });
});
