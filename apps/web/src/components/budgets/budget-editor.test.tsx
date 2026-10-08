import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { EditCurrency } from "@/lib/budgets";
import { renderWithIntl } from "@/test/render";

import { BudgetEditor } from "./budget-editor";

const saveBudgets = vi.fn();
vi.mock("@/app/actions/budgets", () => ({ saveBudgets: (...args: unknown[]) => saveBudgets(...args) }));
const push = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const currencies: EditCurrency[] = [
  {
    currency: "MXN",
    minorUnits: 2,
    rows: [
      { categoryId: "food", name: "Food", color: null, amount: 100000, amountMonth: "2026-03", suggested: 120000 },
      { categoryId: "fun", name: "Fun", color: null, amount: null, amountMonth: null, suggested: 4500 },
      { categoryId: "misc", name: "Misc", color: null, amount: null, amountMonth: null, suggested: null },
    ],
  },
];

function submitted(): Record<string, string> {
  const data = saveBudgets.mock.calls.at(-1)![1] as FormData;
  return Object.fromEntries([...data.entries()].map(([k, v]) => [k, String(v)]));
}

beforeEach(() => {
  saveBudgets.mockReset();
  push.mockReset();
  saveBudgets.mockResolvedValue({ ok: true, nonce: 1 });
});

describe("BudgetEditor", () => {
  it("starts with the amounts in force and marks inherited ones", () => {
    renderWithIntl(<BudgetEditor month="2026-10" currencies={currencies} doneHref="/budgets?month=2026-10" />);
    expect(screen.getByLabelText("Food")).toHaveValue("1 000.00");
    expect(screen.getByLabelText("Fun")).toHaveValue("");
    expect(screen.getByText("from March 2026")).toBeInTheDocument();
  });

  it("fills a field with its suggestion without saving", async () => {
    const user = userEvent.setup();
    renderWithIntl(<BudgetEditor month="2026-10" currencies={currencies} doneHref="/budgets" />);
    await user.click(screen.getByRole("button", { name: "Use the suggestion for Fun" }));
    expect(screen.getByLabelText("Fun")).toHaveValue("45.00");
    expect(saveBudgets).not.toHaveBeenCalled();
  });

  it("applies every suggestion and saves them for the month", async () => {
    const user = userEvent.setup();
    renderWithIntl(<BudgetEditor month="2026-10" currencies={currencies} doneHref="/budgets?month=2026-10" />);
    await user.click(screen.getByRole("button", { name: "Apply all suggestions" }));
    expect(screen.getByLabelText("Food")).toHaveValue("1 200.00");
    expect(screen.getByLabelText("Fun")).toHaveValue("45.00");
    expect(screen.getByLabelText("Misc")).toHaveValue("");

    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(saveBudgets).toHaveBeenCalled());
    expect(submitted()).toMatchObject({
      month: "2026-10",
      "amount:MXN:food": "1 200.00",
      "initial:MXN:food": "1 000.00",
      "amount:MXN:fun": "45.00",
      "initial:MXN:fun": "",
    });
    await waitFor(() => expect(push).toHaveBeenCalledWith("/budgets?month=2026-10"));
  });

  it("submits a typed amount grouped in thousands", async () => {
    const user = userEvent.setup();
    renderWithIntl(<BudgetEditor month="2026-10" currencies={currencies} doneHref="/budgets" />);
    await user.type(screen.getByLabelText("Misc"), "2500");
    expect(screen.getByLabelText("Misc")).toHaveValue("2 500");
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(saveBudgets).toHaveBeenCalled());
    expect(submitted()["amount:MXN:misc"]).toBe("2 500");
  });

  it("shows field errors and stays in edit mode", async () => {
    saveBudgets.mockResolvedValue({ ok: false, message: "Please check the highlighted fields.", fieldErrors: { "amount:MXN:fun": "Bad amount" } });
    const user = userEvent.setup();
    renderWithIntl(<BudgetEditor month="2026-10" currencies={currencies} doneHref="/budgets" />);
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("Bad amount")).toBeInTheDocument();
    expect(screen.getByLabelText("Fun")).toHaveAttribute("aria-invalid", "true");
    expect(push).not.toHaveBeenCalled();
  });

  it("cancels back to the month view", () => {
    renderWithIntl(<BudgetEditor month="2026-10" currencies={currencies} doneHref="/budgets?month=2026-10" />);
    expect(screen.getByRole("button", { name: "Cancel" })).toHaveAttribute("href", "/budgets?month=2026-10");
  });
});
