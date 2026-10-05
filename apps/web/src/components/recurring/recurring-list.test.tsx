import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import type { RecurringRow } from "./recurring-dialog";
import { RecurringList, sortByStatus } from "./recurring-list";

const setRecurringStatus = vi.fn();
vi.mock("@/app/actions/recurring", () => ({
  saveRecurring: vi.fn(),
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

  it("shows an empty state", () => {
    renderList([]);
    expect(screen.getByText(/No subscriptions yet/)).toBeInTheDocument();
  });
});
