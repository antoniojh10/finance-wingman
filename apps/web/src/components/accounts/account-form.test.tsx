import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { AccountForm, type EditableAccount } from "./account-dialog";

const saveAccount = vi.fn();
vi.mock("@/app/actions/accounts", () => ({
  saveAccount: (...args: unknown[]) => saveAccount(...args),
}));

const currencies = [
  { code: "MXN", name: "Mexican Peso" },
  { code: "USD", name: "US Dollar" },
];

function renderForm(account?: EditableAccount) {
  const onSaved = vi.fn();
  const view = renderWithIntl(
    <AccountForm
      account={account}
      currencies={currencies}
      defaultCurrency="MXN"
      onSaved={onSaved}
      onCancel={vi.fn()}
    />,
  );
  return { onSaved, user: userEvent.setup(), rerender: (next: EditableAccount) => view.rerender(<AccountForm account={next} currencies={currencies} defaultCurrency="MXN" onSaved={onSaved} onCancel={vi.fn()} />) };
}

function submittedData(): Record<string, string> {
  const data = saveAccount.mock.calls.at(-1)?.[1] as FormData;
  return Object.fromEntries([...data.entries()].map(([k, v]) => [k, String(v)]));
}

const account: EditableAccount = {
  id: "a1",
  name: "Checking",
  type: "checking",
  currency: "MXN",
  minor_units: 2,
  initial_balance: 100000,
};

beforeEach(() => saveAccount.mockReset());

describe("AccountForm", () => {
  it("submits a new account and reports success", async () => {
    saveAccount.mockResolvedValue({ ok: true, nonce: 1 });
    const { onSaved, user } = renderForm();

    await user.type(screen.getByLabelText("Name"), "Checking");
    await user.type(screen.getByLabelText("Opening balance"), "500.00");
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    expect(submittedData()).toMatchObject({
      name: "Checking",
      type: "checking",
      currency: "MXN",
      initial_balance: "500.00",
    });
  });

  it("prefills an existing account", () => {
    renderForm(account);
    expect(screen.getByLabelText("Name")).toHaveValue("Checking");
    expect(screen.getByLabelText("Type")).toHaveValue("checking");
    expect(screen.getByLabelText("Currency")).toHaveValue("MXN");
    expect(screen.getByLabelText("Opening balance")).toHaveValue("1000.00");
    expect(screen.getByDisplayValue("a1")).toHaveAttribute("name", "id");
  });

  it("keeps the values it opened with when the account is revalidated", () => {
    const warn = vi.spyOn(console, "error").mockImplementation(() => {});
    const { rerender } = renderForm(account);
    rerender({ ...account, name: "Savings", initial_balance: 200000 });
    expect(screen.getByLabelText("Name")).toHaveValue("Checking");
    expect(screen.getByLabelText("Opening balance")).toHaveValue("1000.00");
    expect(warn).not.toHaveBeenCalled();
    warn.mockRestore();
  });
});
