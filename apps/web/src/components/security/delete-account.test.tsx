import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { DeleteAccount, type AccountDeletionImpact } from "./delete-account";

const refresh = vi.fn();
const actions = {
  scheduleAccountDeletion: vi.fn(),
  cancelAccountDeletion: vi.fn(),
};
vi.mock("next/navigation", () => ({ useRouter: () => ({ refresh }) }));
vi.mock("@/app/actions/deletion", () => ({
  scheduleAccountDeletion: (...args: unknown[]) => actions.scheduleAccountDeletion(...args),
  cancelAccountDeletion: (...args: unknown[]) => actions.cancelAccountDeletion(...args),
}));

const shared: AccountDeletionImpact = { id: "w1", name: "Home", role: "member", members: 2, outcome: "leave" };
const solo: AccountDeletionImpact = { id: "w2", name: "Solo", role: "owner", members: 1, outcome: "delete" };
const blocking: AccountDeletionImpact = { id: "w3", name: "Family", role: "owner", members: 3, outcome: "blocked" };

beforeEach(() => {
  vi.clearAllMocks();
  for (const action of Object.values(actions)) action.mockResolvedValue({ ok: true, nonce: 1 });
});

describe("DeleteAccount", () => {
  it("explains what happens to each workspace and confirms with the email", async () => {
    const user = userEvent.setup();
    renderWithIntl(<DeleteAccount email="ana@example.com" deletion={{ workspaces: [shared, solo] }} />);
    const impacts = screen.getAllByTestId("deletion-impact");
    expect(impacts[0]).toHaveTextContent(/Home.*You'll leave it; your accounts there become shared/);
    expect(impacts[1]).toHaveTextContent(/Solo.*it will be deleted with all its data.*export/);
    expect(screen.getByText(/stays in our backups/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Delete my account" }));
    const dialog = within(await screen.findByRole("dialog"));
    expect(dialog.getByText(/delete your account on/)).toBeInTheDocument();
    expect(dialog.getByText("Solo")).toBeInTheDocument();
    const confirm = dialog.getByRole("button", { name: "Delete my account" });
    expect(confirm).toBeDisabled();
    await user.type(dialog.getByLabelText("Type your email (ana@example.com) to confirm"), "ANA@example.com");
    await user.click(confirm);

    await waitFor(() => expect(actions.scheduleAccountDeletion).toHaveBeenCalled());
    const data = actions.scheduleAccountDeletion.mock.calls[0][1] as FormData;
    expect(data.get("email")).toBe("ANA@example.com");
    await waitFor(() => expect(refresh).toHaveBeenCalled());
  });

  it("is blocked while the user is the only owner of a shared workspace", () => {
    renderWithIntl(<DeleteAccount email="ana@example.com" deletion={{ workspaces: [shared, blocking] }} />);
    expect(screen.getAllByTestId("deletion-impact")[1]).toHaveTextContent(/Family.*Make another member an owner/);
    expect(screen.getByRole("alert")).toHaveTextContent("You can't delete your account");
    expect(screen.getByRole("button", { name: "Delete my account" })).toBeDisabled();
  });

  it("shows server errors in the dialog", async () => {
    actions.scheduleAccountDeletion.mockResolvedValue({ ok: false, message: "Your account is already scheduled for deletion." });
    const user = userEvent.setup();
    renderWithIntl(<DeleteAccount email="ana@example.com" deletion={{ workspaces: [] }} />);
    await user.click(screen.getByRole("button", { name: "Delete my account" }));
    await user.type(await screen.findByLabelText(/Type your email/), "ana@example.com");
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Delete my account" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("already scheduled");
  });

  it("lets the user cancel a scheduled deletion", async () => {
    const user = userEvent.setup();
    renderWithIntl(<DeleteAccount email="ana@example.com" deletion={{ scheduled_for: "2026-10-15T12:00:00Z", workspaces: [solo] }} />);
    expect(screen.getByText("Your account will be deleted on October 15, 2026.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Delete my account" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Cancel deletion" }));
    await waitFor(() => expect(actions.cancelAccountDeletion).toHaveBeenCalled());
    expect(refresh).toHaveBeenCalled();
  });

  it("is translated", () => {
    renderWithIntl(<DeleteAccount email="ana@example.com" deletion={{ workspaces: [solo] }} />, { locale: "es" });
    expect(screen.getByRole("heading", { name: "Eliminar mi cuenta" })).toBeInTheDocument();
    expect(screen.getByTestId("deletion-impact")).toHaveTextContent(/se eliminará con todos sus datos/);
  });
});
