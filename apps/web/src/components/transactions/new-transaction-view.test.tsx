import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { NewTransactionView } from "./new-transaction-view";
import type { AccountOption } from "./types";

const push = vi.fn();
const saveTransaction = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));
vi.mock("@/app/actions/transactions", () => ({
  saveTransaction: (...args: unknown[]) => saveTransaction(...args),
}));

const accounts: AccountOption[] = [{ id: "mxn", name: "Checking", currency: "MXN", minor_units: 2, archived: false }];

beforeEach(() => {
  push.mockReset();
  saveTransaction.mockReset();
});

describe("NewTransactionView", () => {
  it("returns to the previous page after saving", async () => {
    saveTransaction.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderWithIntl(<NewTransactionView accounts={accounts} categories={[]} defaultDate="2026-10-05" returnTo="/transactions" />);

    expect(screen.getByRole("heading", { name: "New transaction" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Close" })).toHaveAttribute("href", "/transactions");
    await user.type(screen.getByLabelText("Amount (MXN)"), "99");
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(push).toHaveBeenCalledWith("/transactions"));
  });

  it("asks for an account first when there are none", () => {
    renderWithIntl(<NewTransactionView accounts={[]} categories={[]} defaultDate="2026-10-05" returnTo="/" />);
    expect(screen.getByText(/Create an account before adding transactions/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Save" })).not.toBeInTheDocument();
  });
});
