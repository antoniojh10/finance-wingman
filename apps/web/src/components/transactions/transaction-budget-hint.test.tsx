import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { TransactionForm } from "./transaction-dialog";
import type { AccountOption, CategoryOption, TransactionRow } from "./types";

const saveTransaction = vi.fn();
vi.mock("@/app/actions/transactions", () => ({
  saveTransaction: (...args: unknown[]) => saveTransaction(...args),
}));
const getBudgetImpact = vi.fn();
vi.mock("@/app/actions/budgets", () => ({
  getBudgetImpact: (...args: unknown[]) => getBudgetImpact(...args),
}));

const accounts: AccountOption[] = [{ id: "mxn", name: "Checking", currency: "MXN", minor_units: 2, archived: false }];
const categories: CategoryOption[] = [
  { id: "food", name: "Food", kind: "expense", archived: false },
  { id: "fun", name: "Fun", kind: "expense", archived: false },
  { id: "salary", name: "Salary", kind: "income", archived: false },
];

function renderForm(props: Partial<React.ComponentProps<typeof TransactionForm>> = {}) {
  const onSaved = vi.fn();
  renderWithIntl(
    <TransactionForm accounts={accounts} categories={categories} defaultDate="2026-10-04" onSaved={onSaved} onCancel={vi.fn()} {...props} />,
  );
  return { onSaved, user: userEvent.setup() };
}

const near = { hasBudget: true, warning: true, state: "near", remaining: 2000 };
const over = { hasBudget: true, warning: true, state: "over", remaining: -1500 };
const fine = { hasBudget: true, warning: false, state: "ok", remaining: 8000 };

beforeEach(() => {
  saveTransaction.mockReset();
  getBudgetImpact.mockReset();
});

describe("budget hint in the transaction form", () => {
  it("warns when the expense leaves the category near or over budget", async () => {
    getBudgetImpact.mockResolvedValue(near);
    const { user } = renderForm();

    await user.type(screen.getByLabelText("Amount (MXN)"), "30");
    expect(screen.queryByTestId("budget-hint")).not.toBeInTheDocument();
    await user.click(screen.getByRole("radio", { name: "Food" }));

    expect(await screen.findByTestId("budget-hint")).toHaveTextContent("Food: this leaves MX$20.00 of the budget.");
    expect(getBudgetImpact).toHaveBeenLastCalledWith({ categoryId: "food", currency: "MXN", date: "2026-10-04", amount: 3000 });
  });

  it("follows the amount and category, and disappears when they no longer warn", async () => {
    getBudgetImpact.mockImplementation(async ({ categoryId, amount }: { categoryId: string; amount: number }) =>
      categoryId === "fun" ? fine : amount > 5000 ? over : near,
    );
    const { user } = renderForm();

    await user.type(screen.getByLabelText("Amount (MXN)"), "30");
    await user.click(screen.getByRole("radio", { name: "Food" }));
    expect(await screen.findByText(/this leaves/)).toBeInTheDocument();

    await user.clear(screen.getByLabelText("Amount (MXN)"));
    await user.type(screen.getByLabelText("Amount (MXN)"), "60");
    expect(await screen.findByText("Food: this goes over budget by MX$15.00.")).toBeInTheDocument();
    expect(screen.getByTestId("budget-hint")).toHaveAttribute("data-state", "over");

    await user.click(screen.getByRole("radio", { name: "Fun" }));
    await waitFor(() => expect(screen.queryByTestId("budget-hint")).not.toBeInTheDocument());

    await user.click(screen.getByRole("radio", { name: "Food" }));
    expect(await screen.findByTestId("budget-hint")).toBeInTheDocument();
    await user.clear(screen.getByLabelText("Amount (MXN)"));
    expect(screen.queryByTestId("budget-hint")).not.toBeInTheDocument();
  });

  it("uses the date typed to pick the month", async () => {
    getBudgetImpact.mockResolvedValue(near);
    const { user } = renderForm();
    await user.type(screen.getByLabelText("Amount (MXN)"), "30");
    await user.click(screen.getByRole("radio", { name: "Food" }));
    await screen.findByTestId("budget-hint");

    getBudgetImpact.mockResolvedValue(fine);
    await user.clear(screen.getByLabelText("Date"));
    await user.type(screen.getByLabelText("Date"), "2026-09-15");
    await waitFor(() => expect(getBudgetImpact).toHaveBeenLastCalledWith(expect.objectContaining({ date: "2026-09-15" })));
    await waitFor(() => expect(screen.queryByTestId("budget-hint")).not.toBeInTheDocument());
  });

  it("does not look anything up for income", async () => {
    getBudgetImpact.mockResolvedValue(over);
    const { user } = renderForm();
    await user.type(screen.getByLabelText("Amount (MXN)"), "30");
    await user.click(screen.getByRole("radio", { name: "Income" }));
    await user.click(screen.getByRole("radio", { name: "Salary" }));
    await new Promise((resolve) => setTimeout(resolve, 450));
    expect(getBudgetImpact).not.toHaveBeenCalled();
    expect(screen.queryByTestId("budget-hint")).not.toBeInTheDocument();
  });

  it("does not warn when editing a saved expense", async () => {
    getBudgetImpact.mockResolvedValue(over);
    const transaction: TransactionRow = {
      id: "t1",
      type: "expense",
      account_id: "mxn",
      account_name: "Checking",
      currency: "MXN",
      minor_units: 2,
      amount: 3000,
      destination_account_id: null,
      destination_account_name: null,
      destination_currency: null,
      destination_minor_units: null,
      destination_amount: null,
      category_id: "food",
      category_name: "Food",
      description: "",
      occurred_on: "2026-10-04",
    };
    renderForm({ transaction });
    await new Promise((resolve) => setTimeout(resolve, 450));
    expect(getBudgetImpact).not.toHaveBeenCalled();
    expect(screen.queryByTestId("budget-hint")).not.toBeInTheDocument();
  });

  it("never blocks saving and hands the warning over for the toast", async () => {
    getBudgetImpact.mockResolvedValue(over);
    saveTransaction.mockResolvedValue({ ok: true, nonce: 1, transactionId: "tx1" });
    const { onSaved, user } = renderForm();

    await user.type(screen.getByLabelText("Amount (MXN)"), "60");
    await user.click(screen.getByRole("radio", { name: "Food" }));
    await screen.findByTestId("budget-hint");
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    expect(onSaved).toHaveBeenCalledWith({ transactionId: "tx1", budgetWarning: "Food: this goes over budget by MX$15.00." });
  });

  it("saves normally when the lookup fails", async () => {
    getBudgetImpact.mockResolvedValue(null);
    saveTransaction.mockResolvedValue({ ok: true, nonce: 1, transactionId: "tx1" });
    const { onSaved, user } = renderForm();

    await user.type(screen.getByLabelText("Amount (MXN)"), "60");
    await user.click(screen.getByRole("radio", { name: "Food" }));
    await waitFor(() => expect(getBudgetImpact).toHaveBeenCalled());
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSaved).toHaveBeenCalledWith({ transactionId: "tx1", budgetWarning: undefined }));
    expect(screen.queryByTestId("budget-hint")).not.toBeInTheDocument();
  });
});
