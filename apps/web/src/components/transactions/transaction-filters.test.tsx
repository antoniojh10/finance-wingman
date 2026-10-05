import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { parseTransactionQuery } from "@/lib/query";
import { renderWithIntl } from "@/test/render";

import { TransactionFilters } from "./transaction-filters";

const accountId = "3f2b8c1e-5a4d-4c3b-9e8f-1a2b3c4d5e6f";
const accounts = [{ id: accountId, name: "Checking", currency: "MXN", minor_units: 2, archived: false }];

describe("TransactionFilters", () => {
  it("links each type chip to the filtered list, keeping the search", () => {
    renderWithIntl(<TransactionFilters values={parseTransactionQuery({ q: "tacos", type: "income" })} accounts={accounts} categories={[]} />);
    expect(screen.getByRole("link", { name: "Income" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "All" })).toHaveAttribute("href", "?q=tacos");
    expect(screen.getByRole("link", { name: "Expense" })).toHaveAttribute("href", "?type=expense&q=tacos");
    expect(screen.getByRole("searchbox", { name: "Search" })).toHaveValue("tacos");
    // The selected type survives a new search.
    expect(screen.getByDisplayValue("income")).toHaveAttribute("name", "type");
  });

  it("folds the advanced filters and counts the active ones", () => {
    const { container } = renderWithIntl(
      <TransactionFilters values={parseTransactionQuery({ account_id: accountId, from: "2026-10-01" })} accounts={accounts} categories={[]} />,
    );
    expect(container.querySelector("details")).toHaveAttribute("open");
    expect(screen.getByText("More filters").parentElement).toHaveTextContent("2");
    expect(screen.getByLabelText("Account")).toHaveValue(accountId);
  });
});
