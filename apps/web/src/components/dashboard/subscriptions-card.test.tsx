import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { SubscriptionsCard, type UpcomingPayment } from "./subscriptions-card";

const registerRecurringPayment = vi.fn();
vi.mock("@/app/actions/recurring", () => ({
  registerRecurringPayment: (...args: unknown[]) => registerRecurringPayment(...args),
}));
const toastSuccess = vi.fn();
vi.mock("sonner", () => ({ toast: { success: (...args: unknown[]) => toastSuccess(...args), error: vi.fn() } }));

const item = (id: string, name: string, overrides: Partial<UpcomingPayment["item"]> = {}): UpcomingPayment["item"] => ({
  id,
  name,
  type: "expense",
  amount: 19900,
  currency: "MXN",
  minor_units: 2,
  account_name: "Checking",
  ...overrides,
});

const upcoming: UpcomingPayment[] = [
  { due_on: "2026-10-01", status: "overdue", item: item("a", "Rent") },
  { due_on: "2026-10-06", status: "pending", item: item("b", "Netflix") },
  { due_on: "2026-10-07", status: "paid", item: item("c", "Gym") },
];
const committed = [
  { currency: "MXN", minor_units: 2, expense: 150000 },
  { currency: "USD", minor_units: 2, expense: 4500 },
  { currency: "EUR", minor_units: 2, expense: 0 },
];

function renderCard(props: Partial<React.ComponentProps<typeof SubscriptionsCard>> = {}) {
  renderWithIntl(<SubscriptionsCard upcoming={upcoming} committed={committed} defaultDate="2026-10-05" {...props} />);
  return userEvent.setup();
}

describe("SubscriptionsCard", () => {
  it("lists overdue and upcoming payments with their status", () => {
    renderCard();
    const [rent, netflix, gym] = screen.getAllByTestId("upcoming-row");
    expect(within(rent).getByText("Overdue")).toBeInTheDocument();
    expect(within(rent).getByText("Due Oct 1, 2026 · Checking")).toBeInTheDocument();
    expect(within(netflix).getByText("Pending")).toBeInTheDocument();
    expect(within(netflix).getByText("−MX$199.00")).toBeInTheDocument();
    expect(within(gym).getByText("Paid")).toBeInTheDocument();
  });

  it("offers Register only for unpaid periods", () => {
    renderCard();
    const [rent, netflix, gym] = screen.getAllByTestId("upcoming-row");
    expect(within(rent).getByRole("button", { name: "Register payment" })).toBeInTheDocument();
    expect(within(netflix).getByRole("button", { name: "Register payment" })).toBeInTheDocument();
    expect(within(gym).queryByRole("button", { name: "Register payment" })).not.toBeInTheDocument();
  });

  it("opens the prefilled confirm dialog for the row's period", async () => {
    const user = renderCard();
    await user.click(within(screen.getAllByTestId("upcoming-row")[0]).getByRole("button", { name: "Register payment" }));
    expect(await screen.findByRole("dialog", { name: "Register payment: Rent" })).toBeInTheDocument();
    expect(screen.getByLabelText("Amount (MXN)")).toHaveValue("199.00");
    expect(screen.getByText("Paying the period due on Oct 1, 2026.")).toBeInTheDocument();
  });

  it("keeps the dialog mounted when the revalidation marks the period paid", async () => {
    const card = (list: UpcomingPayment[]) => (
      <SubscriptionsCard upcoming={list} committed={committed} defaultDate="2026-10-05" />
    );
    const view = renderWithIntl(card(upcoming));
    // The revalidated data (status paid) arrives together with the action result.
    registerRecurringPayment.mockImplementation(async () => {
      view.rerender(card(upcoming.map((u, i) => (i === 0 ? { ...u, status: "paid" as const } : u))));
      return { ok: true, nonce: 1 };
    });
    const user = userEvent.setup();
    await user.click(within(screen.getAllByTestId("upcoming-row")[0]).getByRole("button", { name: "Register payment" }));
    await user.click(within(await screen.findByRole("dialog")).getByRole("button", { name: "Register payment" }));
    await waitFor(() => expect(toastSuccess).toHaveBeenCalledWith("Payment registered"));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(within(screen.getAllByTestId("upcoming-row")[0]).getByText("Paid")).toBeInTheDocument();
  });

  it("shows the committed monthly cost per currency and a link to the page", () => {
    renderCard();
    const costs = screen.getByTestId("committed-costs");
    expect(within(costs).getByText("MX$1,500.00")).toBeInTheDocument();
    expect(within(costs).getByText("$45.00")).toBeInTheDocument();
    expect(within(costs).queryByText(/€/)).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "View all" })).toHaveAttribute("href", "/subscriptions");
  });

  it("says when nothing is due", () => {
    renderCard({ upcoming: [] });
    expect(screen.getByText("Nothing due in the next 7 days.")).toBeInTheDocument();
    expect(screen.queryByTestId("upcoming-row")).not.toBeInTheDocument();
  });
});
