import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { RegisterPaymentDialog, type PaymentTarget } from "./register-payment-dialog";

const registerRecurringPayment = vi.fn();
vi.mock("@/app/actions/recurring", () => ({
  registerRecurringPayment: (...args: unknown[]) => registerRecurringPayment(...args),
}));

const target: PaymentTarget = {
  id: "r1",
  name: "Netflix",
  amount: 19950,
  currency: "MXN",
  minor_units: 2,
  period: "2026-10-01",
};

function renderDialog() {
  renderWithIntl(
    <RegisterPaymentDialog target={target} defaultDate="2026-10-05" trigger={<button type="button">Open</button>} />,
  );
  return userEvent.setup();
}

function submittedData(): Record<string, string> {
  const data = registerRecurringPayment.mock.calls.at(-1)?.[1] as FormData;
  return Object.fromEntries([...data.entries()].map(([k, v]) => [k, String(v)]));
}

beforeEach(() => registerRecurringPayment.mockReset());

describe("RegisterPaymentDialog", () => {
  it("is prefilled with the estimate, today and the period", async () => {
    const user = renderDialog();
    await user.click(screen.getByRole("button", { name: "Open" }));
    expect(await screen.findByRole("dialog", { name: "Register payment: Netflix" })).toBeInTheDocument();
    expect(screen.getByLabelText("Amount (MXN)")).toHaveValue("199.50");
    expect(screen.getByLabelText("Payment date")).toHaveValue("2026-10-05");
    expect(screen.getByText("Paying the period due on Oct 1, 2026.")).toBeInTheDocument();
  });

  it("submits the edited amount and date with the period", async () => {
    registerRecurringPayment.mockResolvedValue({ ok: true, nonce: 1 });
    const user = renderDialog();
    await user.click(screen.getByRole("button", { name: "Open" }));
    const amount = await screen.findByLabelText("Amount (MXN)");
    await user.clear(amount);
    await user.type(amount, "210");
    await user.click(screen.getByRole("button", { name: "Register payment" }));
    await waitFor(() => expect(registerRecurringPayment).toHaveBeenCalled());
    expect(submittedData()).toEqual({ id: "r1", period: "2026-10-01", amount: "210", date: "2026-10-05" });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("shows field errors and keeps the dialog open", async () => {
    registerRecurringPayment.mockResolvedValue({
      ok: false,
      message: "Please check the highlighted fields.",
      fieldErrors: { amount: "Enter a positive amount with at most 2 decimals." },
    });
    const user = renderDialog();
    await user.click(screen.getByRole("button", { name: "Open" }));
    await user.click(await screen.findByRole("button", { name: "Register payment" }));
    expect(await screen.findByText("Enter a positive amount with at most 2 decimals.")).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("shows an API error such as a paused item", async () => {
    registerRecurringPayment.mockResolvedValue({ ok: false, message: "item is not active" });
    const user = renderDialog();
    await user.click(screen.getByRole("button", { name: "Open" }));
    await user.click(await screen.findByRole("button", { name: "Register payment" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("item is not active");
  });
});
