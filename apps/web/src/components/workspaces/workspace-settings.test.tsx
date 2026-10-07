import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { WorkspaceSettings, type WorkspaceSettingsProps } from "./workspace-settings";

const refresh = vi.fn();
const actions = {
  inviteMember: vi.fn(),
  leaveWorkspace: vi.fn(),
  removeMember: vi.fn(),
  renameWorkspace: vi.fn(),
  revokeInvitation: vi.fn(),
  setMemberRole: vi.fn(),
};
vi.mock("next/navigation", () => ({ useRouter: () => ({ refresh }) }));
vi.mock("@/app/actions/workspaces", () => ({
  inviteMember: (...args: unknown[]) => actions.inviteMember(...args),
  leaveWorkspace: (...args: unknown[]) => actions.leaveWorkspace(...args),
  removeMember: (...args: unknown[]) => actions.removeMember(...args),
  renameWorkspace: (...args: unknown[]) => actions.renameWorkspace(...args),
  revokeInvitation: (...args: unknown[]) => actions.revokeInvitation(...args),
  setMemberRole: (...args: unknown[]) => actions.setMemberRole(...args),
}));

const props: WorkspaceSettingsProps = {
  workspace: { id: "w1", name: "Home", role: "owner" },
  userId: "u1",
  members: [
    { user_id: "u1", email: "ana@example.com", name: "Ana", role: "owner" },
    { user_id: "u2", email: "bob@example.com", name: "", role: "member" },
  ],
  invitations: [{ id: "i1", email: "carl@example.com", role: "member", invited_by: "Ana", expires_at: "2026-10-14T12:00:00Z" }],
};

beforeEach(() => {
  vi.clearAllMocks();
  for (const action of Object.values(actions)) action.mockResolvedValue({ ok: true, nonce: 1 });
});

describe("WorkspaceSettings", () => {
  it("lists members and pending invitations for owners", () => {
    renderWithIntl(<WorkspaceSettings {...props} />);
    const members = within(screen.getByRole("heading", { name: "Members" }).closest("[data-slot=card]")! as HTMLElement);
    expect(members.getByText("(You)")).toBeInTheDocument();
    expect(members.getByText("bob@example.com")).toBeInTheDocument();
    expect(members.getAllByRole("button", { name: "Actions" })).toHaveLength(2);
    expect(screen.getByText("carl@example.com")).toBeInTheDocument();
    expect(screen.getByText("Member · Expires Oct 14, 2026")).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toHaveValue("Home");
  });

  it("hides management for members", () => {
    renderWithIntl(<WorkspaceSettings {...props} workspace={{ ...props.workspace, role: "member" }} invitations={[]} />);
    expect(screen.getByText(/Only owners can rename the workspace/)).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toBeDisabled();
    expect(screen.queryByRole("heading", { name: "Invite someone" })).not.toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Pending invitations" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Actions" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Leave workspace" })).toBeInTheDocument();
  });

  it("invites someone and clears the form", async () => {
    const user = userEvent.setup();
    renderWithIntl(<WorkspaceSettings {...props} />);
    await user.type(screen.getByLabelText("Email"), "dana@example.com");
    await user.selectOptions(screen.getByLabelText("Role"), "owner");
    await user.click(screen.getByRole("button", { name: "Invite" }));

    await waitFor(() => expect(screen.getByLabelText("Email")).toHaveValue(""));
    const data = actions.inviteMember.mock.calls[0][1] as FormData;
    expect(Object.fromEntries(data.entries())).toEqual({ workspace_id: "w1", email: "dana@example.com", role: "owner" });
  });

  it("shows invitation errors", async () => {
    actions.inviteMember.mockResolvedValue({ ok: false, message: "That person is already a member." });
    const user = userEvent.setup();
    renderWithIntl(<WorkspaceSettings {...props} />);
    await user.type(screen.getByLabelText("Email"), "bob@example.com");
    await user.click(screen.getByRole("button", { name: "Invite" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("That person is already a member.");
  });

  it("changes roles and removes members", async () => {
    const user = userEvent.setup();
    renderWithIntl(<WorkspaceSettings {...props} />);
    const [, bobActions] = screen.getAllByRole("button", { name: "Actions" });

    await user.click(bobActions);
    await user.click(await screen.findByRole("menuitem", { name: "Make owner" }));
    await waitFor(() => expect(actions.setMemberRole).toHaveBeenCalledWith("w1", "u2", "owner"));

    await user.click(bobActions);
    await user.click(await screen.findByRole("menuitem", { name: "Remove" }));
    const dialog = await screen.findByRole("alertdialog", { name: "Remove bob@example.com?" });
    await user.click(within(dialog).getByRole("button", { name: "Remove" }));
    await waitFor(() => expect(actions.removeMember).toHaveBeenCalledWith("w1", "u2"));
  });

  it("revokes invitations and leaves the workspace after confirming", async () => {
    const user = userEvent.setup();
    renderWithIntl(<WorkspaceSettings {...props} />);

    await user.click(screen.getByRole("button", { name: "Revoke" }));
    const revoke = await screen.findByRole("alertdialog", { name: "Revoke the invitation for carl@example.com?" });
    await user.click(within(revoke).getByRole("button", { name: "Revoke" }));
    await waitFor(() => expect(actions.revokeInvitation).toHaveBeenCalledWith("w1", "i1"));

    await user.click(screen.getByRole("button", { name: "Leave workspace" }));
    const leave = await screen.findByRole("alertdialog", { name: "Leave Home?" });
    await user.click(within(leave).getByRole("button", { name: "Leave workspace" }));
    await waitFor(() => expect(actions.leaveWorkspace).toHaveBeenCalledWith("w1", "u1"));
  });
});
