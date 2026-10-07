import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { WorkspaceSwitcher } from "./workspace-switcher";

const refresh = vi.fn();
const switchWorkspace = vi.fn();
const createWorkspace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ refresh }) }));
vi.mock("@/app/actions/workspaces", () => ({
  switchWorkspace: (...args: unknown[]) => switchWorkspace(...args),
  createWorkspace: (...args: unknown[]) => createWorkspace(...args),
}));

const home = { id: "w1", name: "Home" };
const trip = { id: "w2", name: "Trip" };

beforeEach(() => {
  vi.clearAllMocks();
});

describe("WorkspaceSwitcher", () => {
  it("shows the current workspace and switches to another one", async () => {
    switchWorkspace.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderWithIntl(<WorkspaceSwitcher current={home} workspaces={[home, trip]} />);

    const trigger = screen.getByRole("button", { name: "Switch workspace" });
    expect(trigger).toHaveTextContent("Home");
    await user.click(trigger);
    const menu = await screen.findByRole("menu");
    expect(within(menu).getByRole("menuitem", { name: "Manage workspace" })).toHaveAttribute("href", "/settings/workspace");

    await user.click(within(menu).getByRole("menuitem", { name: "Trip" }));
    await waitFor(() => expect(refresh).toHaveBeenCalled());
    expect(switchWorkspace).toHaveBeenCalledWith("w2");
  });

  it("ignores selecting the current workspace", async () => {
    const user = userEvent.setup();
    renderWithIntl(<WorkspaceSwitcher current={home} workspaces={[home, trip]} />);
    await user.click(screen.getByRole("button", { name: "Switch workspace" }));
    await user.click(await screen.findByRole("menuitem", { name: "Home" }));
    expect(switchWorkspace).not.toHaveBeenCalled();
  });

  it("creates a workspace from the menu", async () => {
    createWorkspace.mockResolvedValue({ ok: true, nonce: 1 });
    const user = userEvent.setup();
    renderWithIntl(<WorkspaceSwitcher current={home} workspaces={[home]} />);

    await user.click(screen.getByRole("button", { name: "Switch workspace" }));
    await user.click(await screen.findByRole("menuitem", { name: "New workspace" }));
    const dialog = await screen.findByRole("dialog", { name: "Create a workspace" });
    await user.type(within(dialog).getByLabelText("Name"), "Trip");
    await user.click(within(dialog).getByRole("button", { name: "New workspace" }));

    await waitFor(() => expect(refresh).toHaveBeenCalled());
    const data = createWorkspace.mock.calls[0][1] as FormData;
    expect(data.get("name")).toBe("Trip");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });
});
