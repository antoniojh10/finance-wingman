import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import type { RecurringRow } from "./recurring-dialog";
import { RecurringList, sortByStatus } from "./recurring-list";

const setRecurringStatus = vi.fn();
const registerRecurringPayment = vi.fn();
const toastSuccess = vi.fn();
vi.mock("sonner", () => ({ toast: { success: (...args: unknown[]) => toastSuccess(...args), error: vi.fn() } }));
vi.mock("@/app/actions/recurring", () => ({
  saveRecurring: vi.fn(),
  registerRecurringPayment: (...args: unknown[]) => registerRecurringPayment(...args),
  setRecurringStatus: (...args: unknown[]) => setRecurringStatus(...args),
}));

const row = (overrides: Partial<RecurringRow>): RecurringRow => ({
  id: "a",
  name: "Netflix",
  type: "expense",
  status: "active",
  account_id: "mxn",
  account_name: "Checking",
  category_id: "ent",
  category_name: "Fun",
  currency: "MXN",
  minor_units: 2,
  amount: 19900,
  interval_unit: "month",
  interval_count: 1,
  start_on: "2026-10-05",
  total_payments: null,
  next_due_on: "2026-11-05",
  notes: "",
  ...overrides,
});

const items = [
  row({ id: "c", name: "Old gym", status: "cancelled", next_due_on: null }),
  row({ id: "p", name: "Paused thing", status: "paused" }),
  row({ id: "a", name: "Netflix" }),
  row({ id: "i", name: "Rent income", type: "income", amount: 800000, interval_count: 2, interval_unit: "week", category_name: null }),
];
const accounts = [{ id: "mxn", name: "Checking", currency: "MXN", minor_units: 2, archived: false }];

function renderList(list = items) {
  renderWithIntl(<RecurringList items={list} accounts={accounts} categories={[]} defaultDate="2026-10-05" />);
  return userEvent.setup();
}

beforeEach(() => setRecurringStatus.mockReset());

describe("sortByStatus", () => {
  it("lists active first, then paused, then cancelled", () => {
    expect(sortByStatus(items).map((i) => i.id)).toEqual(["a", "i", "p", "c"]);
  });
});

describe("RecurringList", () => {
  it("shows amount, frequency, account, category, next due date and status", () => {
    renderList();
    const netflix = screen.getAllByTestId("recurring-row")[0];
    expect(within(netflix).getByText("Netflix")).toBeInTheDocument();
    expect(within(netflix).getByText("−MX$199.00")).toBeInTheDocument();
    expect(within(netflix).getByText("Every month · Checking · Fun")).toBeInTheDocument();
    expect(within(netflix).getByText("Next: Nov 5, 2026")).toBeInTheDocument();
    expect(screen.getByText("Every 2 weeks · Checking")).toBeInTheDocument();
    expect(screen.getByText("+MX$8,000.00")).toBeInTheDocument();
    expect(screen.getByText("Paused")).toBeInTheDocument();
    expect(screen.getByText("Cancelled")).toBeInTheDocument();
  });

  it("filters by type", async () => {
    const user = renderList();
    await user.click(screen.getByRole("button", { name: "Income" }));
    expect(screen.getAllByTestId("recurring-row")).toHaveLength(1);
    expect(screen.getByText("Rent income")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "All" }));
    expect(screen.getAllByTestId("recurring-row")).toHaveLength(4);
  });

  it("pauses an active item from its menu", async () => {
    setRecurringStatus.mockResolvedValue({ ok: true, nonce: 1 });
    const user = renderList([row({})]);
    await user.click(screen.getByRole("button", { name: "Actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Pause" }));
    await waitFor(() => expect(setRecurringStatus).toHaveBeenCalledWith("a", "paused"));
  });

  it("resumes a paused item", async () => {
    setRecurringStatus.mockResolvedValue({ ok: true, nonce: 1 });
    const user = renderList([row({ status: "paused" })]);
    await user.click(screen.getByRole("button", { name: "Actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Resume" }));
    await waitFor(() => expect(setRecurringStatus).toHaveBeenCalledWith("a", "active"));
  });

  it("asks for confirmation before cancelling", async () => {
    setRecurringStatus.mockResolvedValue({ ok: true, nonce: 1 });
    const user = renderList([row({})]);
    await user.click(screen.getByRole("button", { name: "Actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Cancel subscription" }));
    expect(setRecurringStatus).not.toHaveBeenCalled();
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Cancel subscription" }));
    await waitFor(() => expect(setRecurringStatus).toHaveBeenCalledWith("a", "cancelled"));
  });

  it("reactivates a cancelled item", async () => {
    setRecurringStatus.mockResolvedValue({ ok: true, nonce: 1 });
    const user = renderList([row({ status: "cancelled" })]);
    await user.click(screen.getByRole("button", { name: "Actions" }));
    expect(screen.queryByRole("menuitem", { name: "Cancel subscription" })).not.toBeInTheDocument();
    await user.click(await screen.findByRole("menuitem", { name: "Reactivate" }));
    await waitFor(() => expect(setRecurringStatus).toHaveBeenCalledWith("a", "active"));
  });

  it("shows the payment status of the current period", () => {
    renderList([
      row({ id: "o", name: "Rent", current_period: { due_on: "2026-10-01", status: "overdue" } }),
      row({ id: "n", name: "Gym", current_period: { due_on: "2026-10-05", status: "pending" } }),
      row({ id: "d", name: "Phone", current_period: { due_on: "2026-10-03", status: "paid" } }),
    ]);
    const [rent, gym, phone] = screen.getAllByTestId("recurring-row");
    expect(within(rent).getByText("Overdue")).toBeInTheDocument();
    expect(within(gym).getByText("Pending")).toBeInTheDocument();
    expect(within(phone).getByText("Paid")).toBeInTheDocument();
  });

  it("offers Register on unpaid active items and opens the confirm dialog", async () => {
    const user = renderList([
      row({ id: "o", name: "Rent", current_period: { due_on: "2026-10-01", status: "overdue" } }),
      row({ id: "d", name: "Phone", current_period: { due_on: "2026-10-03", status: "paid" } }),
      row({ id: "p", name: "Old", status: "paused", current_period: null }),
    ]);
    const [rent, phone, paused] = screen.getAllByTestId("recurring-row");
    expect(within(phone).queryByRole("button", { name: "Register payment" })).not.toBeInTheDocument();
    expect(within(paused).queryByRole("button", { name: "Register payment" })).not.toBeInTheDocument();
    await user.click(within(rent).getByRole("button", { name: "Register payment" }));
    expect(await screen.findByRole("dialog", { name: "Register payment: Rent" })).toBeInTheDocument();
    expect(screen.getByLabelText("Amount (MXN)")).toHaveValue("199.00");
  });

  it("keeps the dialog mounted when the revalidation marks the period paid", async () => {
    const list = (status: "overdue" | "paid") => (
      <RecurringList
        items={[row({ id: "o", name: "Rent", current_period: { due_on: "2026-10-01", status } })]}
        accounts={accounts}
        categories={[]}
        defaultDate="2026-10-05"
      />
    );
    const view = renderWithIntl(list("overdue"));
    registerRecurringPayment.mockImplementation(async () => {
      view.rerender(list("paid"));
      return { ok: true, nonce: 1 };
    });
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Register payment" }));
    await user.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Register payment" }));
    await waitFor(() => expect(toastSuccess).toHaveBeenCalledWith("Payment registered"));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(screen.getByText("Paid")).toBeInTheDocument();
  });

  it("shows the last payment date", () => {
    renderList([
      row({ last_payment: { amount: 19900, date: "2026-10-03", due_on: "2026-10-01", transaction_id: "t" } }),
    ]);
    expect(screen.getByText(/Last paid: Oct 3, 2026/)).toBeInTheDocument();
  });

  it("shows an empty state", () => {
    renderList([]);
    expect(screen.getByText(/No subscriptions yet/)).toBeInTheDocument();
  });
});
