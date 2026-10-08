import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { DeleteWorkspace } from "./delete-workspace";

const refresh = vi.fn();
const actions = {
  scheduleWorkspaceDeletion: vi.fn(),
  cancelWorkspaceDeletion: vi.fn(),
};
vi.mock("next/navigation", () => ({ useRouter: () => ({ refresh }) }));
vi.mock("@/app/actions/deletion", () => ({
  scheduleWorkspaceDeletion: (...args: unknown[]) => actions.scheduleWorkspaceDeletion(...args),
  cancelWorkspaceDeletion: (...args: unknown[]) => actions.cancelWorkspaceDeletion(...args),
}));

const owner = { id: "w1", name: "Home", role: "owner" as const };

beforeEach(() => {
  vi.clearAllMocks();
  for (const action of Object.values(actions)) action.mockResolvedValue({ ok: true, nonce: 1 });
});

describe("DeleteWorkspace", () => {
  it("offers owners an export and a confirmation by name", async () => {
    const user = userEvent.setup();
    renderWithIntl(<DeleteWorkspace workspace={owner} />);
    expect(screen.getByRole("link", { name: "Download JSON" })).toHaveAttribute("href", "/export?format=json");
    expect(screen.getByRole("link", { name: "Download CSV (zip)" })).toHaveAttribute("href", "/export?format=csv");
    expect(screen.getByText(/stays in our backups/)).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Delete workspace" }));
    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveTextContent("Delete Home?");
    expect(dialog).toHaveTextContent(/will be deleted on/);
    const confirm = screen.getAllByRole("button", { name: "Delete workspace" }).at(-1)!;
    expect(confirm).toBeDisabled();

    await user.type(screen.getByLabelText("Type Home to confirm"), "home");
    expect(confirm).toBeDisabled();
    await user.clear(screen.getByLabelText("Type Home to confirm"));
    await user.type(screen.getByLabelText("Type Home to confirm"), "Home");
    await user.click(confirm);

    await waitFor(() => expect(actions.scheduleWorkspaceDeletion).toHaveBeenCalled());
    const data = actions.scheduleWorkspaceDeletion.mock.calls[0][1] as FormData;
    expect(Object.fromEntries(data)).toEqual({ id: "w1", name: "Home" });
    await waitFor(() => expect(refresh).toHaveBeenCalled());
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("shows errors from the server", async () => {
    actions.scheduleWorkspaceDeletion.mockResolvedValue({ ok: false, message: "This workspace is already scheduled for deletion." });
    const user = userEvent.setup();
    renderWithIntl(<DeleteWorkspace workspace={owner} />);
    await user.click(screen.getByRole("button", { name: "Delete workspace" }));
    await user.type(await screen.findByLabelText("Type Home to confirm"), "Home");
    await user.click(screen.getAllByRole("button", { name: "Delete workspace" }).at(-1)!);
    expect(await screen.findByRole("alert")).toHaveTextContent("already scheduled");
  });

  it("lets owners cancel a scheduled deletion", async () => {
    const user = userEvent.setup();
    renderWithIntl(<DeleteWorkspace workspace={{ ...owner, deletion_scheduled_for: "2026-10-15T12:00:00Z" }} />);
    expect(screen.getByText(/Home and all its data will be deleted on October 15, 2026\. Any owner can cancel/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Cancel deletion" }));
    await waitFor(() => expect(actions.cancelWorkspaceDeletion).toHaveBeenCalledWith("w1"));
    expect(refresh).toHaveBeenCalled();
  });

  it("only informs members", () => {
    const { container, rerender } = renderWithIntl(<DeleteWorkspace workspace={{ ...owner, role: "member" }} />);
    expect(container).toBeEmptyDOMElement();
    rerender(<DeleteWorkspace workspace={{ ...owner, role: "member", deletion_scheduled_for: "2026-10-15T12:00:00Z" }} />);
    expect(screen.getByText(/ask an owner to cancel it/)).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("is translated", () => {
    renderWithIntl(<DeleteWorkspace workspace={{ ...owner, deletion_scheduled_for: "2026-10-15T12:00:00Z" }} />, { locale: "es" });
    expect(screen.getByText(/Home y todos sus datos se eliminarán el 15 de octubre de 2026/)).toBeInTheDocument();
  });
});
