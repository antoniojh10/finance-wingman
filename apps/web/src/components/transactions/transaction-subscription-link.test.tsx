import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { TransactionSubscriptionLink } from "./transaction-subscription-link";
import type { RecurringOption, TransactionRow } from "./types";

const link = vi.fn();
const unlink = vi.fn();
vi.mock("@/app/actions/transactions", () => ({
  linkTransactionRecurring: (...args: unknown[]) => link(...args),
  unlinkTransactionRecurring: (...args: unknown[]) => unlink(...args),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const tx: TransactionRow = {
  id: "tx1",
  type: "expense",
  account_id: "mxn",
  account_name: "Checking",
  currency: "MXN",
  minor_units: 2,
  amount: 19900,
  destination_account_id: null,
  destination_account_name: null,
  destination_currency: null,
  destination_minor_units: null,
  destination_amount: null,
  category_id: null,
  category_name: null,
  description: "Netflix",
  occurred_on: "2026-10-05",
  recurring_id: null,
};

const items: RecurringOption[] = [
  { id: "r1", name: "Netflix", type: "expense", account_id: "mxn", status: "active", current_due_on: "2026-10-05", next_due_on: "2026-11-05" },
  { id: "r2", name: "Spotify", type: "expense", account_id: "mxn", status: "paused", current_due_on: null, next_due_on: null },
  { id: "r3", name: "Other account", type: "expense", account_id: "usd", status: "active", current_due_on: null, next_due_on: null },
  { id: "r4", name: "Salary", type: "income", account_id: "mxn", status: "active", current_due_on: null, next_due_on: null },
  { id: "r5", name: "Cancelled", type: "expense", account_id: "mxn", status: "cancelled", current_due_on: null, next_due_on: null },
];

beforeEach(() => {
  link.mockReset();
  unlink.mockReset();
});

describe("TransactionSubscriptionLink", () => {
  it("offers only subscriptions of the same account and type that are not cancelled", () => {
    renderWithIntl(<TransactionSubscriptionLink transaction={tx} recurringItems={items} />);
    const options = screen.getAllByRole("option").map((o) => o.textContent);
    expect(options).toContain("Netflix");
    expect(options).toContain("Spotify");
    expect(options).not.toContain("Other account");
    expect(options).not.toContain("Salary");
    expect(options).not.toContain("Cancelled");
  });

  it("links to the chosen subscription and period", async () => {
    link.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderWithIntl(<TransactionSubscriptionLink transaction={tx} recurringItems={items} />);

    await user.selectOptions(screen.getByLabelText("Period"), "2026-11-05");
    await user.click(screen.getByRole("button", { name: "Link" }));
    await waitFor(() => expect(link).toHaveBeenCalledWith("tx1", "r1", "2026-11-05"));
  });

  it("defaults to the closest period", async () => {
    link.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderWithIntl(<TransactionSubscriptionLink transaction={tx} recurringItems={items} />);

    await user.click(screen.getByRole("button", { name: "Link" }));
    await waitFor(() => expect(link).toHaveBeenCalledWith("tx1", "r1", undefined));
  });

  it("shows the reason when linking fails", async () => {
    link.mockResolvedValue({ ok: false, message: "period is not a due date" });
    const user = userEvent.setup();
    renderWithIntl(<TransactionSubscriptionLink transaction={tx} recurringItems={items} />);

    await user.click(screen.getByRole("button", { name: "Link" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("period is not a due date");
  });

  it("shows the linked subscription and unlinks it", async () => {
    unlink.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderWithIntl(<TransactionSubscriptionLink transaction={{ ...tx, recurring_id: "r1" }} recurringItems={items} />);

    expect(screen.getByText("Pays Netflix")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Unlink" }));
    await waitFor(() => expect(unlink).toHaveBeenCalledWith("tx1"));
  });

  it("renders nothing for transfers or without candidates", () => {
    const { container, rerender } = renderWithIntl(
      <TransactionSubscriptionLink transaction={{ ...tx, type: "transfer" }} recurringItems={items} />,
    );
    expect(container).toBeEmptyDOMElement();
    rerender(<TransactionSubscriptionLink transaction={tx} recurringItems={[items[2]]} />);
    expect(screen.queryByTestId("subscription-link")).not.toBeInTheDocument();
  });
});
