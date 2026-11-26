import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import type { RecurringRow } from "./recurring-dialog";
import { RecurringItemRow, RecurringList, groupByCategory, sortItems } from "./recurring-list";
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
  category_color: "#ff8800",
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
  row({ id: "i", name: "Rent income", type: "income", amount: 800000, monthly_amount: 1733333, interval_count: 2, interval_unit: "week", category_name: null, category_color: null }),
];
const accounts = [{ id: "mxn", name: "Checking", currency: "MXN", minor_units: 2, archived: false }];

function renderList(list = items, status?: StatusFilter, extra: { category?: string; group?: boolean } = {}) {
  renderWithIntl(
    <RecurringList items={list} accounts={accounts} categories={[]} defaultDate="2026-10-05" status={status} group={false} {...extra} />,
  );
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
    expect(within(netflix).getByTestId("category-pill")).toHaveTextContent("Fun");
    expect(within(netflix).getByText("Every month · Checking · Next: Nov 5, 2026")).toBeInTheDocument();
    expect(within(netflix).queryByTestId("recurring-monthly")).not.toBeInTheDocument();
    expect(screen.getByText("Every 2 weeks · Checking · Next: Nov 5, 2026")).toBeInTheDocument();
    expect(screen.getByText("+MX$8,000.00")).toBeInTheDocument();
    expect(screen.getAllByText("Paused").length).toBeGreaterThan(0);
  });

  it("shows no pill for an uncategorized item", () => {
    renderList();
    const rent = screen.getAllByTestId("recurring-row")[1];
    expect(within(rent).queryByTestId("category-pill")).not.toBeInTheDocument();
  });

  it("shows the real amount and the monthly equivalent under it", () => {
    renderList([row({ id: "y", name: "Domain", amount: 48000, monthly_amount: 4000, interval_unit: "year" })]);
    const domain = screen.getByTestId("recurring-row");
    expect(within(domain).getByText("−MX$480.00")).toBeInTheDocument();
    expect(within(domain).getByText("Every year", { exact: false })).toBeInTheDocument();
    expect(within(domain).getByTestId("recurring-monthly")).toHaveTextContent("MX$40.00 / mo");
  });

  it("hides the pill when showCategory is false", () => {
    renderWithIntl(
      <ul>
        <RecurringItemRow item={row({})} accounts={accounts} categories={[]} defaultDate="2026-10-05" showCategory={false} />
      </ul>,
    );
    expect(screen.queryByTestId("category-pill")).not.toBeInTheDocument();
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
    expect(replace).toHaveBeenCalledWith("/subscriptions?status=all&group=0", { scroll: false });
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
    expect(replace).toHaveBeenLastCalledWith("/subscriptions?status=paused&group=0", { scroll: false });
    await user.click(screen.getByRole("button", { name: "Active 2" }));
    expect(replace).toHaveBeenLastCalledWith("/subscriptions?group=0", { scroll: false });
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

const home = { category_id: "home", category_name: "Home", category_color: "#ffb020" };
const fun = { category_id: "fun", category_name: "Fun", category_color: "#f05aa6" };
const none = { category_id: null, category_name: null, category_color: null };
const grouped = [
  row({ id: "n", name: "Netflix", ...fun, amount: 1000, monthly_amount: 1000 }),
  row({ id: "r", name: "Rent", ...home, amount: 90000, monthly_amount: 90000 }),
  row({ id: "e", name: "Electricity", ...home, amount: 10000, monthly_amount: 10000 }),
  row({ id: "u", name: "Coffee", ...none, amount: 5000, monthly_amount: 5000 }),
  row({ id: "g", name: "Old gym", status: "paused", category_id: "gym", category_name: "Health", category_color: "#12a150", amount: 3000, monthly_amount: 3000 }),
  row({ id: "d", name: "Domain", ...fun, amount: 12000, monthly_amount: 1000, interval_unit: "year" }),
];

describe("groupByCategory", () => {
  it("orders by active monthly amount, uncategorized after, groups without active items last", () => {
    const groups = groupByCategory(grouped, "due", "en");
    expect(groups.map((g) => g.name)).toEqual(["Home", "Fun", null, "Health"]);
    expect(groups[0].totals).toEqual([{ currency: "MXN", minor_units: 2, monthly: 100000 }]);
    expect(groups[1].totals[0].monthly).toBe(2000);
    expect(groups[3].totals).toEqual([]);
  });

  it("puts income-only groups after the expense groups", () => {
    const salary = row({ id: "s", type: "income", category_id: "sal", category_name: "Salary", category_color: null, monthly_amount: 999999 });
    expect(groupByCategory([salary, ...grouped], "due", "en").map((g) => g.name)).toEqual(["Home", "Fun", null, "Salary", "Health"]);
  });

  it("totals each currency apart", () => {
    const usd = row({ id: "x", ...home, currency: "USD", monthly_amount: 700 });
    const [group] = groupByCategory([row({ id: "y", ...home, monthly_amount: 5000 }), usd], "due", "en");
    expect(group.totals).toEqual([
      { currency: "MXN", minor_units: 2, monthly: 5000 },
      { currency: "USD", minor_units: 2, monthly: 700 },
    ]);
  });
});

describe("RecurringList grouping and category filter", () => {
  it("groups by category with subtotals and no pill in the rows", () => {
    renderList(grouped, "active", { group: true });
    const groups = screen.getAllByTestId("recurring-group");
    expect(groups.map((g) => g.getAttribute("aria-label"))).toEqual(["Home", "Fun", "Uncategorized"]);
    const totals = within(groups[0]).getByTestId("group-total");
    expect(totals).toHaveTextContent("MX$1,000.00 / mo");
    expect(totals).toHaveTextContent("MX$12,000.00 / yr");
    expect(within(groups[0]).getAllByTestId("recurring-row")).toHaveLength(2);
    // The only pill in a group is the one in its header.
    expect(within(groups[0]).getAllByTestId("category-pill")).toHaveLength(1);
    expect(screen.getByRole("button", { name: "Group by category" })).toHaveAttribute("aria-pressed", "true");
  });

  it("puts groups with only paused items last, as not counted", () => {
    renderList(grouped, "all", { group: true });
    const groups = screen.getAllByTestId("recurring-group");
    expect(groups.map((g) => g.getAttribute("aria-label"))).toEqual(["Home", "Fun", "Uncategorized", "Health"]);
    expect(within(groups[3]).getByTestId("group-total")).toHaveTextContent("Not counted");
  });

  it("shows a flat list with pills when grouping is off", () => {
    renderList(grouped, "active");
    expect(screen.queryByTestId("recurring-group")).not.toBeInTheDocument();
    expect(screen.getAllByTestId("category-pill").length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "Group by category" })).toHaveAttribute("aria-pressed", "false");
  });

  it("toggles the grouping in the URL, keeping the other filters", async () => {
    const user = renderList(grouped, "all", { category: "home", group: true });
    await user.click(screen.getByRole("button", { name: "Group by category" }));
    expect(replace).toHaveBeenCalledWith("/subscriptions?status=all&category=home&group=0", { scroll: false });
  });

  it("turns grouping back on", async () => {
    const user = renderList(grouped, undefined, { group: false });
    await user.click(screen.getByRole("button", { name: "Group by category" }));
    expect(replace).toHaveBeenCalledWith("/subscriptions", { scroll: false });
  });

  it("filters by category and shows a chip that clears it", async () => {
    const user = renderList(grouped, "active", { category: "home" });
    expect(screen.getAllByTestId("recurring-row")).toHaveLength(2);
    const chip = screen.getByTestId("category-chip");
    expect(chip).toHaveTextContent("Category:");
    expect(within(chip).getByTestId("category-pill")).toHaveTextContent("Home");
    await user.click(within(chip).getByRole("button", { name: "Clear category filter" }));
    expect(replace).toHaveBeenCalledWith("/subscriptions?group=0", { scroll: false });
  });

  it("filters uncategorized items with none", () => {
    renderList(grouped, "active", { category: "none" });
    expect(screen.getAllByTestId("recurring-row")).toHaveLength(1);
    expect(screen.getByTestId("category-chip")).toHaveTextContent("Uncategorized");
  });

  it("keeps the category when the status changes", async () => {
    const user = renderList(grouped, "active", { category: "fun" });
    await user.click(screen.getByRole("button", { name: /^Paused/ }));
    expect(replace).toHaveBeenCalledWith("/subscriptions?status=paused&category=fun&group=0", { scroll: false });
  });
});
