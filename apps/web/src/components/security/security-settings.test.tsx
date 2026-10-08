import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithIntl } from "@/test/render";

import { SecuritySettings, type SecuritySettingsProps } from "./security-settings";

const actions = {
  disconnectApp: vi.fn(),
  revokeOtherSessions: vi.fn(),
  revokeSession: vi.fn(),
};
vi.mock("@/app/actions/security", () => ({
  disconnectApp: (...args: unknown[]) => actions.disconnectApp(...args),
  revokeOtherSessions: (...args: unknown[]) => actions.revokeOtherSessions(...args),
  revokeSession: (...args: unknown[]) => actions.revokeSession(...args),
}));

const firefox = "Mozilla/5.0 (X11; Linux x86_64; rv:131.0) Gecko/20100101 Firefox/131.0";
const iphone =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1";

const props: SecuritySettingsProps = {
  sessions: [
    { id: "s1", user_agent: firefox, created_at: "2026-10-01T09:00:00Z", last_used_at: "2026-10-08T12:30:00Z", current: true },
    { id: "s2", user_agent: iphone, created_at: "2026-09-20T09:00:00Z", last_used_at: "2026-10-05T18:00:00Z", current: false },
    { id: "s3", user_agent: "", created_at: "2026-09-01T09:00:00Z", last_used_at: "2026-09-02T08:00:00Z", current: false },
  ],
  connections: [
    {
      id: "c1",
      client_name: "Claude",
      workspace: { id: "w1", name: "Home" },
      scopes: ["finance:read", "finance:write"],
      connected_at: "2026-09-10T10:00:00Z",
      last_used_at: "2026-10-08T07:15:00Z",
    },
    { id: "c2", client_name: "", scopes: ["finance:read"], connected_at: "2026-08-01T10:00:00Z", last_used_at: "2026-08-02T10:00:00Z" },
  ],
};

function card(heading: string) {
  return within(screen.getByRole("heading", { name: heading }).closest("[data-slot=card]") as HTMLElement);
}

beforeEach(() => {
  vi.clearAllMocks();
  for (const action of Object.values(actions)) action.mockResolvedValue({ ok: true, nonce: 1 });
});

describe("SecuritySettings", () => {
  it("lists sessions, marking this device", () => {
    renderWithIntl(<SecuritySettings {...props} />);
    const rows = card("Where you're signed in").getAllByTestId("session-row");
    expect(rows).toHaveLength(3);
    expect(rows[0]).toHaveTextContent("Firefox on Linux");
    expect(rows[0]).toHaveTextContent("This device");
    expect(rows[0]).toHaveTextContent("Last active Oct 8, 2026, 12:30 PM · Signed in Oct 1, 2026");
    expect(within(rows[0]).queryByRole("button")).not.toBeInTheDocument();
    expect(rows[1]).toHaveTextContent("Safari on iOS");
    expect(rows[2]).toHaveTextContent("Unknown device");
    expect(within(rows[1]).getByRole("button", { name: "Sign out Safari on iOS" })).toBeInTheDocument();
  });

  it("signs out another session after confirming", async () => {
    const user = userEvent.setup();
    renderWithIntl(<SecuritySettings {...props} />);
    await user.click(screen.getByRole("button", { name: "Sign out Safari on iOS" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent("Sign out Safari on iOS?");
    await user.click(within(dialog).getByRole("button", { name: "Sign out" }));
    await waitFor(() => expect(actions.revokeSession).toHaveBeenCalledWith("s2"));
  });

  it("signs out everywhere else", async () => {
    const user = userEvent.setup();
    renderWithIntl(<SecuritySettings {...props} />);
    await user.click(screen.getByRole("button", { name: "Sign out everywhere else" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent("Connected apps stay connected.");
    await user.click(within(dialog).getByRole("button", { name: "Sign out everywhere else" }));
    await waitFor(() => expect(actions.revokeOtherSessions).toHaveBeenCalled());
  });

  it("hides sign out everywhere else when this is the only session", () => {
    renderWithIntl(<SecuritySettings {...props} sessions={props.sessions.slice(0, 1)} />);
    expect(screen.queryByRole("button", { name: "Sign out everywhere else" })).not.toBeInTheDocument();
  });

  it("lists connected apps with their workspace and access", () => {
    renderWithIntl(<SecuritySettings {...props} />);
    const rows = card("Connected apps").getAllByTestId("connection-row");
    expect(rows[0]).toHaveTextContent("Claude");
    expect(rows[0]).toHaveTextContent("Home · Read and write · Connected Sep 10, 2026 · Last active Oct 8, 2026, 7:15 AM");
    expect(rows[1]).toHaveTextContent("Unnamed app");
    expect(rows[1]).toHaveTextContent("No workspace · Read only");
  });

  it("disconnects an app after confirming", async () => {
    const user = userEvent.setup();
    renderWithIntl(<SecuritySettings {...props} />);
    await user.click(screen.getByRole("button", { name: "Disconnect Claude" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent("Claude loses access right away.");
    await user.click(within(dialog).getByRole("button", { name: "Disconnect" }));
    await waitFor(() => expect(actions.disconnectApp).toHaveBeenCalledWith("c1"));
  });

  it("explains when no apps are connected", () => {
    renderWithIntl(<SecuritySettings {...props} connections={[]} />);
    expect(screen.getByText("No apps are connected.")).toBeInTheDocument();
  });

  it("is translated", () => {
    renderWithIntl(<SecuritySettings {...props} />, { locale: "es" });
    expect(screen.getByRole("heading", { name: "Apps conectadas" })).toBeInTheDocument();
    expect(screen.getByText("Firefox en Linux")).toBeInTheDocument();
    const rows = card("Apps conectadas").getAllByTestId("connection-row");
    expect(rows[0]).toHaveTextContent("Lectura y escritura");
    expect(rows[1]).toHaveTextContent("Solo lectura");
    expect(screen.getByRole("button", { name: "Cerrar las demás sesiones" })).toBeInTheDocument();
  });
});
