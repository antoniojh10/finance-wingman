import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { TransactionForm } from "./transaction-dialog";
import type { AccountOption, CategoryOption } from "./types";

const saveTransaction = vi.fn();
vi.mock("@/app/actions/transactions", () => ({
  saveTransaction: (...args: unknown[]) => saveTransaction(...args),
}));

const accounts: AccountOption[] = [
  { id: "mxn", name: "Checking", currency: "MXN", minor_units: 2, archived: false },
  { id: "mxn2", name: "Savings", currency: "MXN", minor_units: 2, archived: false },
  { id: "usd", name: "Dollars", currency: "USD", minor_units: 2, archived: false },
  { id: "old", name: "Closed", currency: "MXN", minor_units: 2, archived: true },
];
const categories: CategoryOption[] = [
  { id: "food", name: "Food", kind: "expense", archived: false },
  { id: "salary", name: "Salary", kind: "income", archived: false },
  { id: "gone", name: "Old category", kind: "expense", archived: true },
];

function renderForm(props: Partial<React.ComponentProps<typeof TransactionForm>> = {}) {
  const onSaved = vi.fn();
  renderWithIntl(
    <TransactionForm accounts={accounts} categories={categories} defaultDate="2026-10-04" onSaved={onSaved} onCancel={vi.fn()} {...props} />,
  );
  return { onSaved, user: userEvent.setup() };
}

function submittedData(): Record<string, string> {
  const data = saveTransaction.mock.calls.at(-1)?.[1] as FormData;
  return Object.fromEntries([...data.entries()].map(([k, v]) => [k, String(v)]));
}

beforeEach(() => {
  saveTransaction.mockReset();
});

describe("TransactionForm", () => {
  it("submits an expense and reports success", async () => {
    saveTransaction.mockResolvedValue({ ok: true, nonce: 1 });
    const { onSaved, user } = renderForm();

    await user.type(screen.getByLabelText("Amount (MXN)"), "125.50");
    await user.selectOptions(screen.getByLabelText("Category"), "Food");
    await user.type(screen.getByLabelText("Description"), "Lunch");
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    expect(submittedData()).toMatchObject({
      type: "expense",
      account_id: "mxn",
      amount: "125.50",
      category_id: "food",
      description: "Lunch",
      occurred_on: "2026-10-04",
    });
  });

  it("hides archived accounts and categories and filters categories by type", async () => {
    const { user } = renderForm();
    expect(screen.queryByRole("option", { name: "Closed (MXN)" })).not.toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Old category" })).not.toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Food" })).toBeInTheDocument();

    await user.click(screen.getByRole("radio", { name: "Income" }));
    expect(screen.queryByRole("option", { name: "Food" })).not.toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Salary" })).toBeInTheDocument();
  });

  it("asks for the received amount only for cross-currency transfers", async () => {
    const { user } = renderForm();
    await user.click(screen.getByRole("radio", { name: "Transfer" }));

    expect(screen.getByLabelText("From account")).toBeInTheDocument();
    expect(screen.queryByLabelText("Category")).not.toBeInTheDocument();
    expect(screen.getByLabelText("To account")).toHaveValue("mxn2");
    expect(screen.queryByLabelText(/Amount received/)).not.toBeInTheDocument();

    await user.selectOptions(screen.getByLabelText("To account"), "usd");
    expect(screen.getByLabelText("Amount received (USD)")).toBeInTheDocument();
  });

  it("keeps the user's input and selection after a validation error", async () => {
    saveTransaction.mockResolvedValue({
      ok: false,
      message: "Please check the highlighted fields.",
      fieldErrors: { amount: "Enter a positive amount with at most 2 decimals." },
    });
    const { onSaved, user } = renderForm();

    await user.selectOptions(screen.getByLabelText("Account"), "usd");
    await user.type(screen.getByLabelText("Amount (USD)"), "abc");
    await user.type(screen.getByLabelText("Description"), "Coffee");
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByText("Enter a positive amount with at most 2 decimals.")).toBeInTheDocument();
    expect(onSaved).not.toHaveBeenCalled();
    // Regression: the form must not be reset, or the account would silently
    // fall back to the first option on the next submission.
    expect(screen.getByLabelText("Account")).toHaveValue("usd");
    expect(screen.getByLabelText("Amount (USD)")).toHaveValue("abc");
    expect(screen.getByLabelText("Description")).toHaveValue("Coffee");

    saveTransaction.mockResolvedValue({ ok: true, nonce: 2 });
    await user.clear(screen.getByLabelText("Amount (USD)"));
    await user.type(screen.getByLabelText("Amount (USD)"), "4.50");
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(submittedData()).toMatchObject({ account_id: "usd", amount: "4.50" });
  });

  it("prefills an existing transaction, including archived references", () => {
    renderForm({
      transaction: {
        id: "tx1",
        type: "expense",
        account_id: "old",
        account_name: "Closed",
        currency: "MXN",
        minor_units: 2,
        amount: 12345,
        destination_account_id: null,
        destination_account_name: null,
        destination_currency: null,
        destination_minor_units: null,
        destination_amount: null,
        category_id: "gone",
        category_name: "Old category",
        description: "Old purchase",
        occurred_on: "2026-01-15",
      },
    });
    expect(screen.getByLabelText("Account")).toHaveValue("old");
    expect(screen.getByLabelText("Amount (MXN)")).toHaveValue("123.45");
    expect(screen.getByLabelText("Category")).toHaveValue("gone");
    expect(screen.getByLabelText("Date")).toHaveValue("2026-01-15");
    expect(screen.getByDisplayValue("tx1")).toHaveAttribute("name", "id");
  });
});
