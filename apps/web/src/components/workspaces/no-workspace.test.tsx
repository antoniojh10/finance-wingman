import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { NoWorkspace } from "./no-workspace";

const refresh = vi.fn();
const createWorkspace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ refresh }) }));
vi.mock("@/app/actions/workspaces", () => ({ createWorkspace: (...args: unknown[]) => createWorkspace(...args) }));

describe("NoWorkspace", () => {
  it("explains the situation and shows validation errors", async () => {
    createWorkspace.mockResolvedValue({
      ok: false,
      message: "Please check the highlighted fields.",
      fieldErrors: { name: "This field is required." },
    });
    const user = userEvent.setup();
    renderWithIntl(<NoWorkspace />);

    expect(screen.getByRole("heading", { name: "You're not in a workspace" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "New workspace" }));
    expect(await screen.findByText("This field is required.")).toBeInTheDocument();
    expect(refresh).not.toHaveBeenCalled();
  });

  it("creates a workspace", async () => {
    createWorkspace.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderWithIntl(<NoWorkspace />);
    await user.type(screen.getByLabelText("Name"), "Home");
    await user.click(screen.getByRole("button", { name: "New workspace" }));
    await waitFor(() => expect(refresh).toHaveBeenCalled());
  });
});
