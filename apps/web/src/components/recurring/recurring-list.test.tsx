import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import type { RecurringRow } from "./recurring-dialog";
import { RecurringList, sortItems } from "./recurring-list";
import type { StatusFilter } from "@/lib/recurring-filters";

const replace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));
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
  monthly_amount: 19900,
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
  row({ id: "i", name: "Rent income", type: "income", amount: 800000, monthly_amount: 1733333, interval_count: 2, interval_unit: "week", category_name: null }),
];
const accounts = [{ id: "mxn", name: "Checking", currency: "MXN", minor_units: 2, archived: false }];

function renderList(list = items, status?: StatusFilter) {
  renderWithIntl(<RecurringList items={list} accounts={accounts} categories={[]} defaultDate="2026-10-05" status={status} />);
  return userEvent.setup();
}

beforeEach(() => {
  setRecurringStatus.mockReset();
  replace.mockReset();
});

const names = () => screen.getAllByTestId("recurring-row").map((r) => within(r).getAllByText(/./)[0].textContent);

describe("sortItems", () => {
  it("lists active first, then paused, then cancelled", () => {
    expect(sortItems(items, "due", "en").map((i) => i.id)).toEqual(["a", "i", "p", "c"]);
  });

  it("sorts by next due date with undated items last", () => {
    const list = [
      row({ id: "x", next_due_on: null }),
      row({ id: "l", next_due_on: "2026-12-01" }),
      row({ id: "e", next_due_on: "2026-11-01" }),
    ];
    expect(sortItems(list, "due", "en").map((i) => i.id)).toEqual(["e", "l", "x"]);
  });

  it("sorts by monthly cost, highest first", () => {
    const list = [
      row({ id: "y", amount: 120000, monthly_amount: 10000, interval_unit: "year" }),
      row({ id: "m", amount: 5000, monthly_amount: 5000 }),
      row({ id: "w", amount: 2000, monthly_amount: 8667, interval_unit: "week" }),
    ];
    expect(sortItems(list, "amount", "en").map((i) => i.id)).toEqual(["y", "w", "m"]);
  });

  it("sorts by name", () => {
    const list = [row({ id: "b", name: "Spotify" }), row({ id: "a", name: "Apple" })];
    expect(sortItems(list, "name", "en").map((i) => i.id)).toEqual(["a", "b"]);
  });
});

describe("RecurringList", () => {
  it("shows amount, frequency, account, category, next due date and status", () => {
    renderList(items, "all");
    const netflix = screen.getAllByTestId("recurring-row")[0];
    expect(within(netflix).getByText("Netflix")).toBeInTheDocument();
    expect(within(netflix).getByText("−MX$199.00")).toBeInTheDocument();
    expect(within(netflix).getByText("Every month · Checking · Fun")).toBeInTheDocument();
    expect(within(netflix).getByText("Next: Nov 5, 2026")).toBeInTheDocument();
    expect(screen.getByText("Every 2 weeks · Checking")).toBeInTheDocument();
    expect(screen.getByText("+MX$8,000.00")).toBeInTheDocument();
    expect(screen.getAllByText("Paused").length).toBeGreaterThan(0);
  });

  it("hides paused and cancelled items by default and shows counts", () => {
    renderList();
    expect(names()).toEqual(["Netflix", "Rent income"]);
    expect(screen.getByRole("button", { name: "Active 2" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "Paused 1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancelled 1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "All 4" })).toBeInTheDocument();
  });

  it("shows a footer with the hidden count and Show all", async () => {
    const user = renderList();
    expect(screen.getByTestId("hidden-note")).toHaveTextContent("2 paused or cancelled hidden");
    await user.click(screen.getByRole("button", { name: "Show all" }));
    expect(replace).toHaveBeenCalledWith("/subscriptions?status=all", { scroll: false });
    expect(screen.getAllByTestId("recurring-row")).toHaveLength(4);
    expect(screen.queryByTestId("hidden-note")).not.toBeInTheDocument();
  });

  it("has no footer when nothing is hidden", () => {
    renderList([row({})]);
    expect(screen.queryByTestId("hidden-note")).not.toBeInTheDocument();
  });

  it("switches status and updates the URL", async () => {
    const user = renderList();
    await user.click(screen.getByRole("button", { name: "Paused 1" }));
    expect(names()).toEqual(["Paused thing"]);
    expect(replace).toHaveBeenLastCalledWith("/subscriptions?status=paused", { scroll: false });
    await user.click(screen.getByRole("button", { name: "Active 2" }));
    expect(replace).toHaveBeenLastCalledWith("/subscriptions", { scroll: false });
    expect(screen.getAllByTestId("recurring-row")).toHaveLength(2);
  });

  it("starts on the status from the URL", () => {
    renderList(items, "cancelled");
    expect(names()).toEqual(["Old gym"]);
  });

  it("filters by type combined with status", async () => {
    const user = renderList();
    await user.selectOptions(screen.getByRole("combobox", { name: "Type" }), "income");
    expect(names()).toEqual(["Rent income"]);
    expect(screen.queryByTestId("hidden-note")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Paused 1" }));
    expect(screen.getByText("Nothing here with this filter.")).toBeInTheDocument();
    await user.selectOptions(screen.getByRole("combobox", { name: "Type" }), "all");
    expect(names()).toEqual(["Paused thing"]);
  });

  it("sorts with the select", async () => {
    const user = renderList([row({ id: "a", name: "Zed", amount: 100, monthly_amount: 100 }), row({ id: "b", name: "Abe", amount: 900, monthly_amount: 900 })]);
    await user.selectOptions(screen.getByRole("combobox", { name: "Sort" }), "name");
    expect(names()).toEqual(["Abe", "Zed"]);
    await user.selectOptions(screen.getByRole("combobox", { name: "Sort" }), "amount");
    expect(names()).toEqual(["Abe", "Zed"]);
    await user.selectOptions(screen.getByRole("combobox", { name: "Sort" }), "due");
    expect(names()).toEqual(["Zed", "Abe"]);
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
    const user = renderList([row({ status: "paused" })], "all");
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
    const user = renderList([row({ status: "cancelled" })], "all");
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
    ], "all");
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
