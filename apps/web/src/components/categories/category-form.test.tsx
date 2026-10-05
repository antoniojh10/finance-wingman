import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { CategoryForm, type EditableCategory } from "./category-dialog";

const saveCategory = vi.fn();
vi.mock("@/app/actions/categories", () => ({
  saveCategory: (...args: unknown[]) => saveCategory(...args),
}));

function renderForm(category?: EditableCategory) {
  const onSaved = vi.fn();
  const view = renderWithIntl(
    <CategoryForm
      category={category}
      defaultKind="expense"
      onSaved={onSaved}
      onCancel={vi.fn()}
    />,
  );
  return { onSaved, user: userEvent.setup(), rerender: (next: EditableCategory) => view.rerender(<CategoryForm category={next} defaultKind="expense" onSaved={onSaved} onCancel={vi.fn()} />) };
}

function submittedData(): Record<string, string> {
  const data = saveCategory.mock.calls.at(-1)?.[1] as FormData;
  return Object.fromEntries([...data.entries()].map(([k, v]) => [k, String(v)]));
}

const category: EditableCategory = {
  id: "c1",
  name: "Food",
  kind: "expense",
  color: "#ff6b4a",
};

beforeEach(() => saveCategory.mockReset());

describe("CategoryForm", () => {
  it("submits a new category and reports success", async () => {
    saveCategory.mockResolvedValue({ ok: true, nonce: 1 });
    const { onSaved, user } = renderForm();

    await user.type(screen.getByLabelText("Name"), "Food");
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    expect(submittedData()).toMatchObject({
      name: "Food",
      kind: "expense",
    });
  });

  it("prefills an existing category", () => {
    renderForm(category);
    expect(screen.getByLabelText("Name")).toHaveValue("Food");
    expect(screen.getByLabelText("Kind")).toHaveValue("expense");
    expect(screen.getByDisplayValue("c1")).toHaveAttribute("name", "id");
  });

  it("keeps the values it opened with when the category is revalidated", () => {
    const warn = vi.spyOn(console, "error").mockImplementation(() => {});
    const { rerender } = renderForm(category);
    rerender({ ...category, name: "Groceries", color: "#2f8cff" });
    expect(screen.getByLabelText("Name")).toHaveValue("Food");
    expect(warn).not.toHaveBeenCalled();
    warn.mockRestore();
  });
});
