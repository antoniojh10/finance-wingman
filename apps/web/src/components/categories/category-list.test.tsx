import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { CategoryBoard, type CategoryListItem } from "./category-list";

vi.mock("@/app/actions/categories", () => ({ saveCategory: vi.fn(), deleteCategory: vi.fn(), setCategoryArchived: vi.fn() }));

const categories: CategoryListItem[] = [
  { id: "rent", name: "Rent", kind: "expense", color: "#6d4aff", archived: false },
  { id: "food", name: "Food", kind: "expense", color: "#ff6b4a", archived: false },
  { id: "salary", name: "Salary", kind: "income", color: "#14b8a6", archived: false },
];
const totals = { rent: [{ currency: "MXN", minor_units: 2, total: 950000 }] };

describe("CategoryBoard", () => {
  it("shows expense categories with this month's totals first", () => {
    renderWithIntl(<CategoryBoard categories={categories} totals={totals} showArchived={false} />);
    expect(screen.getByRole("tab", { name: "Expense" })).toHaveAttribute("aria-selected", "true");
    const cards = screen.getAllByTestId("category-row");
    expect(cards).toHaveLength(2);
    expect(within(cards[0]).getByText("MX$9,500.00")).toBeInTheDocument();
    expect(within(cards[1]).getByText("—")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Show archived" })).toHaveAttribute("href", "/categories?archived=1");
  });

  it("switches to income categories and creates them as income", async () => {
    const user = userEvent.setup();
    renderWithIntl(<CategoryBoard categories={categories} totals={totals} showArchived={false} />);
    await user.click(screen.getByRole("tab", { name: "Income" }));
    expect(screen.getByRole("heading", { name: "Income categories" })).toBeInTheDocument();
    expect(screen.getAllByTestId("category-row")).toHaveLength(1);

    await user.click(screen.getByRole("button", { name: "New category" }));
    const dialog = await screen.findByRole("dialog", { name: "New category" });
    expect(within(dialog).getByLabelText("Kind")).toHaveValue("income");
  });
});
