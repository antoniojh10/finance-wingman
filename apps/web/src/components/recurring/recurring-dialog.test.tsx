import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AccountOption, CategoryOption } from "@/components/transactions/types";
import { renderWithIntl } from "@/test/render";

import { RecurringForm, type RecurringRow } from "./recurring-dialog";

const saveRecurring = vi.fn();
vi.mock("@/app/actions/recurring", () => ({
  saveRecurring: (...args: unknown[]) => saveRecurring(...args),
}));

const accounts: AccountOption[] = [
  { id: "mxn", name: "Checking", currency: "MXN", minor_units: 2, archived: false },
  { id: "usd", name: "Dollars", currency: "USD", minor_units: 2, archived: false },
  { id: "old", name: "Closed", currency: "MXN", minor_units: 2, archived: true },
];
const categories: CategoryOption[] = [
  { id: "fun", name: "Fun", kind: "expense", archived: false },
  { id: "salary", name: "Salary", kind: "income", archived: false },
];

function form(item: RecurringRow | undefined, onSaved: () => void) {
  return (
    <RecurringForm
      accounts={accounts}
      categories={categories}
      item={item}
      defaultDate="2026-10-05"
      onSaved={onSaved}
      onCancel={vi.fn()}
    />
  );
}

function renderForm(item?: RecurringRow) {
  const onSaved = vi.fn();
  const view = renderWithIntl(form(item, onSaved));
  return { onSaved, user: userEvent.setup(), rerender: (next: RecurringRow) => view.rerender(form(next, onSaved)) };
}

function submittedData(): Record<string, string> {
  const data = saveRecurring.mock.calls.at(-1)?.[1] as FormData;
  return Object.fromEntries([...data.entries()].map(([k, v]) => [k, String(v)]));
}

const item: RecurringRow = {
  id: "r1",
  name: "Netflix",
  type: "expense",
  status: "active",
  account_id: "usd",
  account_name: "Dollars",
  category_id: "fun",
  category_name: "Fun",
  currency: "USD",
  minor_units: 2,
  amount: 1599,
  interval_unit: "year",
  interval_count: 2,
  start_on: "2026-01-15",
  total_payments: 4,
  next_due_on: "2027-01-15",
  notes: "family plan",
};

beforeEach(() => saveRecurring.mockReset());

describe("RecurringForm", () => {
  it("submits a new subscription and reports success", async () => {
    saveRecurring.mockResolvedValue({ ok: true, nonce: 1 });
    const { onSaved, user } = renderForm();

    await user.type(screen.getByLabelText("Name"), "Netflix");
    await user.type(screen.getByLabelText("Amount (MXN)"), "199");
    await user.click(screen.getByRole("radio", { name: "Fun" }));
    await user.selectOptions(screen.getByLabelText("Unit"), "year");
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    expect(submittedData()).toMatchObject({
      type: "expense",
      account_id: "mxn",
      name: "Netflix",
      amount: "199",
      category_id: "fun",
      interval_count: "1",
      interval_unit: "year",
      start_on: "2026-10-05",
    });
  });

  it("hides archived accounts and filters categories by type", async () => {
    const { user } = renderForm();
    expect(screen.queryByRole("radio", { name: /Closed/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("radio", { name: "Salary" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("radio", { name: "Income" }));
    expect(screen.getByRole("radio", { name: "Salary" })).toBeInTheDocument();
    expect(screen.queryByRole("radio", { name: "Fun" })).not.toBeInTheDocument();
  });

  it("shows the currency of the selected account", async () => {
    const { user } = renderForm();
    await user.click(screen.getByRole("radio", { name: /Dollars/ }));
    expect(screen.getByLabelText("Amount (USD)")).toBeInTheDocument();
  });

  it("shows field errors and the API message", async () => {
    saveRecurring.mockResolvedValue({
      ok: false,
      message: "A subscription with this name already exists.",
      fieldErrors: { name: "This field is required." },
    });
    const { user } = renderForm();
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("This field is required.")).toBeInTheDocument();
    expect(screen.getByText("A subscription with this name already exists.")).toBeInTheDocument();
  });

  it("prefills an existing item and locks its type and account", () => {
    renderForm(item);
    expect(screen.getByLabelText("Name")).toHaveValue("Netflix");
    expect(screen.getByLabelText("Amount (USD)")).toHaveValue("15.99");
    expect(screen.getByLabelText("Repeats every")).toHaveValue(2);
    expect(screen.getByLabelText("Unit")).toHaveValue("year");
    expect(screen.getByLabelText(/Number of payments/)).toHaveValue(4);
    expect(screen.getByLabelText("First due date")).toHaveValue("2026-01-15");
    expect(screen.getByRole("radio", { name: "Fun" })).toBeChecked();
    expect(screen.queryByRole("radio", { name: /Dollars/ })).not.toBeInTheDocument();
    expect(screen.getByText(/cannot be changed after creation/)).toBeInTheDocument();
  });

  it("keeps the values it opened with when the item is revalidated", () => {
    const warn = vi.spyOn(console, "error").mockImplementation(() => {});
    const { rerender } = renderForm(item);
    rerender({ ...item, name: "Netflix Premium", amount: 2599 });
    expect(screen.getByLabelText("Name")).toHaveValue("Netflix");
    expect(screen.getByLabelText("Amount (USD)")).toHaveValue("15.99");
    expect(warn).not.toHaveBeenCalled();
    warn.mockRestore();
  });

  it("submits the id when editing", async () => {
    saveRecurring.mockResolvedValue({ ok: true, nonce: 1 });
    const { onSaved, user } = renderForm(item);
    await user.clear(screen.getByLabelText("Amount (USD)"));
    await user.type(screen.getByLabelText("Amount (USD)"), "20");
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(submittedData()).toMatchObject({ id: "r1", amount: "20", type: "expense", total_payments: "4" });
  });
});
