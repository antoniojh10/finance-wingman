import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { CatalogEntry, Receipt, ReceiptLine } from "@/lib/groceries";
import { renderWithIntl } from "@/test/render";

import { ReceiptDetail } from "./receipt-detail";

const push = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));
const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("sonner", () => ({ toast: { success: (...a: unknown[]) => toastSuccess(...a), error: (...a: unknown[]) => toastError(...a) } }));

const assignReceiptLine = vi.fn();
const unassignReceiptLine = vi.fn();
const deleteReceipt = vi.fn();
vi.mock("@/app/actions/groceries", () => ({
  assignReceiptLine: (...a: unknown[]) => assignReceiptLine(...a),
  unassignReceiptLine: (...a: unknown[]) => unassignReceiptLine(...a),
  deleteReceipt: (...a: unknown[]) => deleteReceipt(...a),
}));

const line = (overrides: Partial<ReceiptLine>): ReceiptLine => ({
  id: "l1",
  amount: 129,
  brand: null,
  discount: 0,
  pack_label: null,
  position: 1,
  product_id: null,
  product_name: null,
  quantity: 1,
  raw_text: "LECHE ENT. HACENDADO 1L",
  variant_id: null,
  ...overrides,
});

const receipt = (overrides: Partial<Receipt> = {}): Receipt => ({
  id: "r1",
  created_at: "2026-10-01T10:00:00Z",
  updated_at: "2026-10-01T10:00:00Z",
  currency: "EUR",
  line_count: 3,
  lines_total: 1000,
  purchased_on: "2026-09-27",
  source: "photo",
  store_id: "s1",
  store_name: "Mercadona",
  total: 1000,
  transaction_id: "t1",
  unassigned_count: 1,
  warnings: [],
  lines: [
    line({ id: "l1", variant_id: "v1", product_id: "p1", product_name: "Whole milk", brand: "Hacendado", pack_label: "1 L" }),
    line({ id: "l2", raw_text: "PLATANO", quantity: 1.25, amount: 250, discount: 50 }),
    line({ id: "l3", raw_text: "PAN BARRA", amount: 621, quantity: 2 }),
  ],
  ...overrides,
});

const catalog: CatalogEntry[] = [{ id: "p1", name: "Whole milk", compareUnit: "l", variants: [{ id: "v1", label: "Hacendado · 1 L" }] }];

function renderDetail(r: Receipt = receipt()) {
  renderWithIntl(<ReceiptDetail receipt={r} catalog={catalog} groups={[]} />);
  return userEvent.setup();
}

beforeEach(() => {
  for (const mock of [push, toastSuccess, toastError, assignReceiptLine, unassignReceiptLine, deleteReceipt]) mock.mockReset();
});

describe("ReceiptDetail", () => {
  it("lists raw lines with amounts, quantity and discount", () => {
    renderDetail();
    const lines = screen.getAllByTestId("receipt-line");
    expect(lines).toHaveLength(3);
    expect(within(lines[0]).getByText("LECHE ENT. HACENDADO 1L")).toBeInTheDocument();
    expect(within(lines[1]).getByText(/1\.25 ×/)).toBeInTheDocument();
    expect(within(lines[1]).getByText("discount €0.50")).toBeInTheDocument();
    expect(within(lines[2]).getByText("€6.21")).toBeInTheDocument();
  });

  it("shows the mapping of each line", () => {
    renderDetail();
    const lines = screen.getAllByTestId("receipt-line");
    expect(lines[0]).toHaveAttribute("data-status", "assigned");
    expect(within(lines[0]).getByText("Whole milk")).toBeInTheDocument();
    expect(within(lines[0]).getByText(/Hacendado · 1 L/)).toBeInTheDocument();
    expect(lines[1]).toHaveAttribute("data-status", "unassigned");
    expect(within(lines[1]).getByText("Unassigned")).toBeInTheDocument();
    expect(screen.getByText("1 line to assign")).toBeInTheDocument();
  });

  it("says when every line is assigned", () => {
    renderDetail(receipt({ unassigned_count: 0 }));
    expect(screen.getByText("All lines assigned")).toBeInTheDocument();
  });

  it("shows warnings with both amounts", () => {
    renderDetail(
      receipt({
        warnings: [
          { code: "total_mismatch", receipt_total: 1000, transaction_amount: 1100 },
          { code: "lines_mismatch", receipt_total: 1000, lines_total: 900 },
        ],
      }),
    );
    expect(screen.getByText("The receipt total (€10.00) differs from the linked expense (€11.00).")).toBeInTheDocument();
    expect(screen.getByText("The lines add up to €9.00, but the receipt total is €10.00.")).toBeInTheDocument();
  });

  it("links the expense, or says there is none", () => {
    renderDetail();
    expect(screen.getByRole("button", { name: "View expense" })).toHaveAttribute(
      "href",
      "/transactions?type=expense&from=2026-09-27&to=2026-09-27",
    );
  });

  it("shows when no expense is linked", () => {
    renderDetail(receipt({ transaction_id: null }));
    expect(screen.queryByRole("button", { name: "View expense" })).not.toBeInTheDocument();
    expect(screen.getByText("Not linked to an expense")).toBeInTheDocument();
  });

  it("opens the assignment form for a line and closes it after saving", async () => {
    assignReceiptLine.mockResolvedValue({ ok: true, nonce: 1 });
    const user = renderDetail();
    const target = screen.getAllByTestId("receipt-line")[1];
    await user.click(within(target).getByRole("button", { name: "Assign" }));
    await user.selectOptions(within(target).getByLabelText("Product"), "p1");
    await user.selectOptions(within(target).getByLabelText("Brand and size"), "v1");
    await user.click(within(target).getByRole("button", { name: "Save assignment" }));
    await waitFor(() => expect(assignReceiptLine).toHaveBeenCalled());
    await waitFor(() => expect(toastSuccess).toHaveBeenCalledWith("Line assigned."));
    expect(within(target).queryByRole("button", { name: "Save assignment" })).not.toBeInTheDocument();
  });

  it("unassigns a line", async () => {
    unassignReceiptLine.mockResolvedValue({ ok: true, nonce: 1 });
    const user = renderDetail();
    await user.click(within(screen.getAllByTestId("receipt-line")[0]).getByRole("button", { name: "Unassign" }));
    await waitFor(() => expect(unassignReceiptLine).toHaveBeenCalledWith("r1", "l1"));
    expect(toastSuccess).toHaveBeenCalledWith("Line unassigned.");
  });

  it("deletes the receipt after confirming", async () => {
    deleteReceipt.mockResolvedValue({ ok: true, nonce: 1 });
    const user = renderDetail();
    await user.click(screen.getByRole("button", { name: "Delete receipt" }));
    expect(await screen.findByText("Delete this receipt?")).toBeInTheDocument();
    expect(deleteReceipt).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(deleteReceipt).toHaveBeenCalledWith("r1"));
    await waitFor(() => expect(push).toHaveBeenCalledWith("/groceries/receipts"));
  });

  it("keeps the receipt when deleting fails", async () => {
    deleteReceipt.mockResolvedValue({ ok: false, message: "Something went wrong. Please try again." });
    const user = renderDetail();
    await user.click(screen.getByRole("button", { name: "Delete receipt" }));
    await user.click(await screen.findByRole("button", { name: "Delete" }));
    await waitFor(() => expect(toastError).toHaveBeenCalledWith("Something went wrong. Please try again."));
    expect(push).not.toHaveBeenCalled();
  });
});
