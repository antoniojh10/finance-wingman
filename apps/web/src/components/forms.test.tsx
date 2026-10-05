import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { AccountForm } from "./accounts/account-dialog";
import { CategoryForm } from "./categories/category-dialog";

const saveAccount = vi.fn();
const saveCategory = vi.fn();
vi.mock("@/app/actions/accounts", () => ({ saveAccount: (...args: unknown[]) => saveAccount(...args) }));
vi.mock("@/app/actions/categories", () => ({ saveCategory: (...args: unknown[]) => saveCategory(...args) }));

const currencies = [
  { code: "MXN", name: "Mexican Peso" },
  { code: "USD", name: "US Dollar" },
];

beforeEach(() => {
  saveAccount.mockReset();
  saveCategory.mockReset();
});

function lastData(mock: ReturnType<typeof vi.fn>): Record<string, string> {
  const data = mock.mock.calls.at(-1)?.[1] as FormData;
  return Object.fromEntries([...data.entries()].map(([k, v]) => [k, String(v)]));
}

describe("AccountForm", () => {
  it("creates an account with the chosen currency", async () => {
    saveAccount.mockResolvedValue({ ok: true, nonce: 1 });
    const onSaved = vi.fn();
    const user = userEvent.setup();
    renderWithIntl(<AccountForm currencies={currencies} defaultCurrency="MXN" defaultDate="2026-10-05" onSaved={onSaved} onCancel={vi.fn()} />);

    await user.type(screen.getByLabelText("Name"), "Wallet");
    await user.selectOptions(screen.getByLabelText("Type"), "cash");
    await user.selectOptions(screen.getByLabelText("Currency"), "USD");
    await user.type(screen.getByLabelText("Opening balance"), "50");
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(lastData(saveAccount)).toEqual({ name: "Wallet", type: "cash", currency: "USD", initial_balance: "50", balance_as_of: "2026-10-05" });
  });

  it("locks the currency when editing and shows server errors", async () => {
    saveAccount.mockResolvedValue({ ok: false, message: "An active account with this name already exists." });
    const user = userEvent.setup();
    renderWithIntl(
      <AccountForm
        account={{ id: "a1", name: "Card", type: "credit_card", currency: "MXN", minor_units: 2, initial_balance: -150000, balance_as_of: "2026-09-01" }}
        currencies={currencies}
        defaultCurrency="MXN"
        defaultDate="2026-10-05"
        onSaved={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    expect(screen.getByLabelText("Currency")).toBeDisabled();
    expect(screen.getByText("The currency cannot be changed after creation.")).toBeInTheDocument();
    expect(screen.getByLabelText("Opening balance")).toHaveValue("-1500.00");

    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("An active account with this name already exists.")).toBeInTheDocument();
    expect(lastData(saveAccount)).toMatchObject({ id: "a1", name: "Card" });
  });
});

describe("CategoryForm", () => {
  it("submits the selected color and kind", async () => {
    saveCategory.mockResolvedValue({ ok: true, nonce: 1 });
    const onSaved = vi.fn();
    const user = userEvent.setup();
    renderWithIntl(<CategoryForm defaultKind="income" onSaved={onSaved} onCancel={vi.fn()} />);

    expect(screen.getByLabelText("Kind")).toHaveValue("income");
    await user.type(screen.getByLabelText("Name"), "Bonus");
    await user.click(screen.getByRole("radio", { name: "#ff6b4a" }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(lastData(saveCategory)).toEqual({ color: "#ff6b4a", name: "Bonus", kind: "income" });
  });

  it("shows field errors from the server", async () => {
    saveCategory.mockResolvedValue({ ok: false, message: "Please check the highlighted fields.", fieldErrors: { name: "This field is required." } });
    const user = userEvent.setup();
    renderWithIntl(<CategoryForm defaultKind="expense" onSaved={vi.fn()} onCancel={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("This field is required.")).toBeInTheDocument();
  });
});
