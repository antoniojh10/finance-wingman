import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { AcceptInvitationForm } from "./accept-invitation-form";

const acceptInvitation = vi.fn();
vi.mock("@/app/actions/workspaces", () => ({ acceptInvitation: (...args: unknown[]) => acceptInvitation(...args) }));

describe("AcceptInvitationForm", () => {
  it("accepts with the token and shows why it failed", async () => {
    acceptInvitation.mockResolvedValue({ error: "This invitation is invalid." });
    const user = userEvent.setup();
    renderWithIntl(<AcceptInvitationForm token="inv_1" />);

    await user.click(screen.getByRole("button", { name: "Accept invitation" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("This invitation is invalid.");
    expect((acceptInvitation.mock.calls[0][1] as FormData).get("token")).toBe("inv_1");
    expect(screen.getByText("Go to sign in").closest("a")).toHaveAttribute("href", "/login");
  });
});
