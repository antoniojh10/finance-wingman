import { screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { ReceiptSummary } from "@/lib/groceries";
import { renderWithIntl } from "@/test/render";

import { ReceiptsList } from "./receipts-list";

const receipt = (overrides: Partial<ReceiptSummary>): ReceiptSummary => ({
  id: "r1",
  created_at: "2026-10-01T10:00:00Z",
  currency: "EUR",
  line_count: 12,
  lines_total: 6804,
  purchased_on: "2026-10-02",
  source: "photo",
  store_id: "s1",
  store_name: "Mercadona",
  total: 6804,
  transaction_id: "t1",
  unassigned_count: 0,
  updated_at: "2026-10-01T10:00:00Z",
  ...overrides,
});

const stores = [
  { id: "s1", name: "Mercadona" },
  { id: "s2", name: "Lidl" },
];

function renderList(receipts: ReceiptSummary[], props: Partial<React.ComponentProps<typeof ReceiptsList>> = {}) {
  return renderWithIntl(
    <ReceiptsList receipts={receipts} stores={stores} query={{}} month="2026-10" currentMonth="2026-10" {...props} />,
  );
}

describe("ReceiptsList", () => {
  it("shows store, date, total and line count linking to the receipt", () => {
    renderList([receipt({})]);
    const link = screen.getByRole("link", { name: "Open receipt from Mercadona, Oct 2, 2026" });
    expect(link).toHaveAttribute("href", "/groceries/receipts/r1");
    expect(within(link).getByText("€68.04")).toBeInTheDocument();
    expect(within(link).getByText("Oct 2, 2026 · 12 lines")).toBeInTheDocument();
    expect(within(link).getByText("Linked expense")).toBeInTheDocument();
  });

  it("flags unassigned lines, mismatching totals and a missing expense", () => {
    renderList([receipt({ unassigned_count: 3, lines_total: 6000, transaction_id: null })]);
    expect(screen.getByText("3 to assign")).toBeInTheDocument();
    expect(screen.getByText("Lines don't add up")).toBeInTheDocument();
    expect(screen.getByText("No expense linked")).toBeInTheDocument();
  });

  it("shows no badges for a clean receipt", () => {
    renderList([receipt({})]);
    expect(screen.queryByText(/to assign/)).not.toBeInTheDocument();
    expect(screen.queryByText("Lines don't add up")).not.toBeInTheDocument();
  });

  it("navigates months keeping the store filter", () => {
    renderList([receipt({})], { query: { storeId: "6f1c2a3e-1b2c-4d5e-8f90-123456789abc" } });
    expect(screen.getByRole("link", { name: "Previous month" })).toHaveAttribute(
      "href",
      "/groceries/receipts?month=2026-09&store=6f1c2a3e-1b2c-4d5e-8f90-123456789abc",
    );
    expect(screen.getByRole("link", { name: "All months" })).toHaveAttribute(
      "href",
      "/groceries/receipts?month=all&store=6f1c2a3e-1b2c-4d5e-8f90-123456789abc",
    );
  });

  it("filters by store", () => {
    renderList([receipt({})]);
    expect(screen.getByRole("link", { name: "Lidl" })).toHaveAttribute("href", "/groceries/receipts?month=2026-10&store=s2");
    expect(screen.getByRole("link", { name: "All stores" })).toHaveAttribute("aria-current", "page");
  });

  it("hides the store filter when there is a single store", () => {
    renderList([receipt({})], { stores: [stores[0]] });
    expect(screen.queryByRole("link", { name: "All stores" })).not.toBeInTheDocument();
  });

  it("hides the month picker when showing every month", () => {
    renderList([receipt({})], { month: "all" });
    expect(screen.queryByRole("link", { name: "Previous month" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "By month" })).toHaveAttribute("href", "/groceries/receipts?month=2026-10");
  });

  it("explains how to add receipts when there are none at all", () => {
    renderList([], { month: "all" });
    expect(screen.getByTestId("receipts-empty")).toHaveTextContent("Send a photo of a receipt to Claude");
    expect(screen.queryByRole("link", { name: "Show all months" })).not.toBeInTheDocument();
  });

  it("offers every month when a filter finds nothing", () => {
    renderList([]);
    expect(screen.getByText("No receipts match this filter.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Show all months" })).toHaveAttribute("href", "/groceries/receipts?month=all");
  });
});
