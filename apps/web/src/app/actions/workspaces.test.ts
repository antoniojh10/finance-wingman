import "@/test/server-mocks";

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { cookieJar, mockApi, RedirectError } from "@/test/server-mocks";

import {
  acceptInvitation,
  createWorkspace,
  inviteMember,
  leaveWorkspace,
  removeMember,
  renameWorkspace,
  revokeInvitation,
  setMemberRole,
  switchWorkspace,
} from "./workspaces";

function form(values: Record<string, string>): FormData {
  const data = new FormData();
  for (const [key, value] of Object.entries(values)) data.set(key, value);
  return data;
}

const session = {
  token: "tok_new",
  expires_at: "2027-01-01T00:00:00Z",
  user: { id: "u2", email: "bob@example.com", name: "", locale: "en" },
  workspace: { id: "w1", name: "Home", role: "member" },
};

beforeEach(() => {
  cookieJar.clear();
  cookieJar.set("fw_session", { value: "tok_123" });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("workspace actions", () => {
  it("switches the session's workspace", async () => {
    const requests = mockApi([{ method: "PUT", path: "/api/v1/auth/session/workspace", status: 200, body: session }]);
    expect(await switchWorkspace("w2")).toMatchObject({ ok: true });
    expect(requests[0]).toMatchObject({ body: { workspace_id: "w2" }, auth: "Bearer tok_123" });
  });

  it("reports workspaces the user doesn't belong to", async () => {
    mockApi([{ method: "PUT", path: "/api/v1/auth/session/workspace", status: 404, body: { status: 404 } }]);
    expect(await switchWorkspace("w9")).toEqual({ ok: false, message: "This item no longer exists." });
  });

  it("creates a workspace and switches to it", async () => {
    const requests = mockApi([
      { method: "POST", path: "/api/v1/workspaces", status: 201, body: { id: "w2", name: "Trip", role: "owner" } },
      { method: "PUT", path: "/api/v1/auth/session/workspace", status: 200, body: session },
    ]);
    expect(await createWorkspace({ ok: false }, form({ name: " Trip " }))).toMatchObject({ ok: true });
    expect(requests.map((r) => [r.method, r.path, r.body])).toEqual([
      ["POST", "/api/v1/workspaces", { name: "Trip" }],
      ["PUT", "/api/v1/auth/session/workspace", { workspace_id: "w2" }],
    ]);
  });

  it("requires a workspace name", async () => {
    const requests = mockApi([]);
    expect(await createWorkspace({ ok: false }, form({ name: " " }))).toMatchObject({
      ok: false,
      fieldErrors: { name: "This field is required." },
    });
    expect(await renameWorkspace({ ok: false }, form({ id: "w1", name: "" }))).toMatchObject({ ok: false });
    expect(requests).toHaveLength(0);
  });

  it("renames a workspace", async () => {
    const requests = mockApi([{ method: "PATCH", path: "/api/v1/workspaces/w1", status: 200, body: { id: "w1", name: "Casa" } }]);
    expect(await renameWorkspace({ ok: false }, form({ id: "w1", name: "Casa" }))).toMatchObject({ ok: true });
    expect(requests[0].body).toEqual({ name: "Casa" });
  });

  it("reports owner-only actions", async () => {
    mockApi([{ method: "PATCH", path: "/api/v1/workspaces/w1", status: 403, body: { status: 403 } }]);
    expect(await renameWorkspace({ ok: false }, form({ id: "w1", name: "Casa" }))).toEqual({
      ok: false,
      message: "You don't have permission to do that.",
    });
  });

  it("invites someone in the current language", async () => {
    const requests = mockApi([{ method: "POST", path: "/api/v1/workspaces/w1/invitations", status: 201, body: {} }]);
    expect(await inviteMember({ ok: false }, form({ workspace_id: "w1", email: " Bob@Example.com ", role: "owner" }))).toMatchObject({
      ok: true,
    });
    expect(requests[0].body).toEqual({ email: "bob@example.com", role: "owner", locale: "en" });
  });

  it("validates invitations and explains conflicts", async () => {
    const requests = mockApi([{ method: "POST", path: "/api/v1/workspaces/w1/invitations", status: 409, body: { status: 409 } }]);
    expect(await inviteMember({ ok: false }, form({ workspace_id: "w1", email: "nope" }))).toMatchObject({
      fieldErrors: { email: "Enter a valid email address." },
    });
    expect(requests).toHaveLength(0);
    expect(await inviteMember({ ok: false }, form({ workspace_id: "w1", email: "ana@example.com" }))).toEqual({
      ok: false,
      message: "That person is already a member, or too many invitations were sent recently.",
    });
    expect(requests[0].body).toMatchObject({ role: "member" });
  });

  it("revokes invitations and manages members", async () => {
    const requests = mockApi([
      { method: "DELETE", path: "/api/v1/workspaces/w1/invitations/i1", status: 204 },
      { method: "PATCH", path: "/api/v1/workspaces/w1/members/u2", status: 204 },
      { method: "DELETE", path: "/api/v1/workspaces/w1/members/u2", status: 204 },
    ]);
    expect(await revokeInvitation("w1", "i1")).toMatchObject({ ok: true });
    expect(await setMemberRole("w1", "u2", "owner")).toMatchObject({ ok: true });
    expect(await removeMember("w1", "u2")).toMatchObject({ ok: true });
    expect(requests[1].body).toEqual({ role: "owner" });
  });

  it("explains that a workspace needs an owner", async () => {
    mockApi([
      { method: "PATCH", path: "/api/v1/workspaces/w1/members/u1", status: 409, body: { status: 409 } },
      { method: "DELETE", path: "/api/v1/workspaces/w1/members/u1", status: 409, body: { status: 409 } },
    ]);
    const lastOwner = "A workspace needs at least one owner. Make another member an owner first.";
    expect(await setMemberRole("w1", "u1", "member")).toEqual({ ok: false, message: lastOwner });
    expect(await removeMember("w1", "u1")).toEqual({ ok: false, message: lastOwner });
  });

  it("leaves a workspace and moves to another one", async () => {
    const requests = mockApi([
      { method: "DELETE", path: "/api/v1/workspaces/w1/members/u1", status: 204 },
      { method: "GET", path: "/api/v1/workspaces", status: 200, body: { items: [{ id: "w2", name: "Trip", role: "owner" }] } },
      { method: "PUT", path: "/api/v1/auth/session/workspace", status: 200, body: session },
    ]);
    await expect(leaveWorkspace("w1", "u1")).rejects.toEqual(new RedirectError("/"));
    expect(requests.at(-1)).toMatchObject({ method: "PUT", body: { workspace_id: "w2" } });
  });

  it("accepts an invitation and signs in as the invitee", async () => {
    const requests = mockApi([{ method: "POST", path: "/api/v1/invitations/accept", status: 200, body: session }]);
    await expect(acceptInvitation({}, form({ token: "inv_1" }))).rejects.toEqual(new RedirectError("/"));
    expect(requests[0]).toMatchObject({ body: { token: "inv_1" }, auth: null, userAgent: "Mozilla/5.0 (Test Browser)" });
    expect(cookieJar.get("fw_session")?.value).toBe("tok_new");
  });

  it("reports invalid invitations", async () => {
    mockApi([{ method: "POST", path: "/api/v1/invitations/accept", status: 404, body: { status: 404 } }]);
    const invalid = { error: "This invitation is invalid, has expired or was already used. Ask for a new one." };
    expect(await acceptInvitation({}, form({ token: "used" }))).toEqual(invalid);
    expect(await acceptInvitation({}, form({}))).toEqual(invalid);
    expect(cookieJar.get("fw_session")?.value).toBe("tok_123");
  });
});
