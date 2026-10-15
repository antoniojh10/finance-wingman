import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { CatalogEntry } from "@/lib/groceries";
import { renderWithIntl } from "@/test/render";

import { AssignLineForm } from "./assign-line-form";

const assignReceiptLine = vi.fn();
vi.mock("@/app/actions/groceries", () => ({
  assignReceiptLine: (...args: unknown[]) => assignReceiptLine(...args),
}));

const catalog: CatalogEntry[] = [
  { id: "p1", name: "Milk", compareUnit: "l", variants: [{ id: "v1", label: "Hacendado · 1 L" }, { id: "v2", label: "Generic" }] },
];
const groups = [{ id: "g1", name: "Dairy & eggs" }];

function renderForm(props: { catalog?: CatalogEntry[]; onDone?: () => void } = {}) {
  const onDone = props.onDone ?? vi.fn();
  renderWithIntl(
    <AssignLineForm receiptId="r1" lineId="l1" catalog={props.catalog ?? catalog} groups={groups} onDone={onDone} onCancel={vi.fn()} />,
  );
  return { user: userEvent.setup(), onDone };
}

function submitted(): Record<string, string> {
  const data = assignReceiptLine.mock.calls.at(-1)?.[1] as FormData;
  return Object.fromEntries([...data.entries()].map(([k, v]) => [k, String(v)]));
}

beforeEach(() => assignReceiptLine.mockReset());

describe("AssignLineForm", () => {
  it("assigns an existing product variant", async () => {
    assignReceiptLine.mockResolvedValue({ ok: true, nonce: 1 });
    const { user, onDone } = renderForm();
    expect(screen.getByLabelText("Brand and size")).toBeDisabled();
    await user.selectOptions(screen.getByLabelText("Product"), "p1");
    await user.selectOptions(screen.getByLabelText("Brand and size"), "v2");
    await user.click(screen.getByRole("button", { name: "Save assignment" }));
    await waitFor(() => expect(assignReceiptLine).toHaveBeenCalled());
    expect(submitted()).toEqual({ receipt_id: "r1", line_id: "l1", mode: "existing", variant_id: "v2" });
    await waitFor(() => expect(onDone).toHaveBeenCalled());
  });

  it("creates a new product with brand and pack", async () => {
    assignReceiptLine.mockResolvedValue({ ok: true, nonce: 2 });
    const { user } = renderForm();
    await user.click(screen.getByRole("button", { name: "New product" }));
    await user.type(screen.getByLabelText("Product name"), "Semi-skimmed milk");
    await user.type(screen.getByLabelText("Brand"), "Pascual");
    await user.type(screen.getByLabelText("Pack label"), "6×1 L");
    await user.type(screen.getByLabelText("Pack size"), "6");
    await user.selectOptions(screen.getByLabelText("Compare by"), "l");
    await user.selectOptions(screen.getByLabelText("Group"), "Dairy & eggs");
    await user.click(screen.getByRole("button", { name: "Save assignment" }));
    await waitFor(() => expect(assignReceiptLine).toHaveBeenCalled());
    expect(submitted()).toEqual({
      receipt_id: "r1",
      line_id: "l1",
      mode: "new",
      product: "Semi-skimmed milk",
      brand: "Pascual",
      pack_label: "6×1 L",
      pack_size: "6",
      compare_unit: "l",
      group: "Dairy & eggs",
    });
  });

  it("starts in new product mode when the catalog is empty", () => {
    renderForm({ catalog: [] });
    expect(screen.getByRole("button", { name: "Existing product" })).toBeDisabled();
    expect(screen.getByLabelText("Product name")).toBeInTheDocument();
  });

  it("shows field errors and stays open", async () => {
    assignReceiptLine.mockResolvedValue({
      ok: false,
      message: "Please check the highlighted fields.",
      fieldErrors: { variant_id: "This field is required." },
    });
    const { user, onDone } = renderForm();
    await user.click(screen.getByRole("button", { name: "Save assignment" }));
    expect(await screen.findByText("This field is required.")).toBeInTheDocument();
    expect(screen.getByText("Please check the highlighted fields.")).toBeInTheDocument();
    expect(onDone).not.toHaveBeenCalled();
  });
});
