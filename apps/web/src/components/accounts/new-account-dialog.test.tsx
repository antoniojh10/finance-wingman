import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { NewAccountDialog } from "./new-account-dialog";

let query = "";
const replace = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
  usePathname: () => "/accounts",
  useSearchParams: () => new URLSearchParams(query),
}));
vi.mock("@/app/actions/accounts", () => ({ saveAccount: vi.fn() }));

const renderDialog = () =>
  renderWithIntl(
    <NewAccountDialog
      currencies={[{ code: "MXN", name: "Mexican Peso" }]}
      defaultCurrency="MXN"
      defaultDate="2026-10-08"
      trigger={<button>New account</button>}
    />,
  );

beforeEach(() => {
  query = "";
  replace.mockReset();
});

describe("NewAccountDialog", () => {
  it("stays closed without the flag", () => {
    renderDialog();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("opens on load with ?new=1", async () => {
    query = "new=1";
    renderDialog();
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
  });

  it("removes the flag but keeps other params when closed", async () => {
    query = "owner=shared&new=1";
    renderDialog();
    await userEvent.click(await screen.findByRole("button", { name: /cancel/i }));
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/accounts?owner=shared"));
  });

  it("replaces with the bare path when the flag was the only param", async () => {
    query = "new=1";
    renderDialog();
    await userEvent.click(await screen.findByRole("button", { name: /cancel/i }));
    await waitFor(() => expect(replace).toHaveBeenCalledWith("/accounts"));
  });

  it("does not touch the URL when opened with the trigger", async () => {
    renderDialog();
    await userEvent.click(screen.getByRole("button", { name: "New account" }));
    await userEvent.click(await screen.findByRole("button", { name: /cancel/i }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(replace).not.toHaveBeenCalled();
  });
});
