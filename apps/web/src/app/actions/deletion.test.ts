import "@/test/server-mocks";

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { cookieJar, mockApi } from "@/test/server-mocks";

import { cancelWorkspaceDeletion, scheduleWorkspaceDeletion } from "./deletion";

function form(values: Record<string, string>): FormData {
  const data = new FormData();
  for (const [key, value] of Object.entries(values)) data.set(key, value);
  return data;
}

beforeEach(() => {
  cookieJar.clear();
  cookieJar.set("fw_session", { value: "tok_123" });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("workspace deletion actions", () => {
  it("schedules the deletion with the typed name", async () => {
    const requests = mockApi([
      { method: "POST", path: "/api/v1/workspaces/w1/deletion", status: 200, body: { id: "w1", name: "Home", deletion_scheduled_for: "2026-10-15T00:00:00Z" } },
    ]);
    expect(await scheduleWorkspaceDeletion({ ok: false }, form({ id: "w1", name: " Home " }))).toMatchObject({ ok: true });
    expect(requests[0]).toMatchObject({ method: "POST", body: { name: "Home" }, auth: "Bearer tok_123" });
  });

  it("requires the name before calling the API", async () => {
    const requests = mockApi([]);
    expect(await scheduleWorkspaceDeletion({ ok: false }, form({ id: "w1", name: "" }))).toMatchObject({
      ok: false,
      fieldErrors: { name: "This field is required." },
    });
    expect(requests).toHaveLength(0);
  });

  it("reports a name that doesn't match", async () => {
    mockApi([{ method: "POST", path: "/api/v1/workspaces/w1/deletion", status: 422, body: { status: 422, errors: [{ location: "name" }] } }]);
    expect(await scheduleWorkspaceDeletion({ ok: false }, form({ id: "w1", name: "home" }))).toMatchObject({
      ok: false,
      fieldErrors: { name: "Type the workspace name exactly as shown." },
    });
  });

  it("reports an already scheduled deletion and members trying it", async () => {
    mockApi([{ method: "POST", path: "/api/v1/workspaces/w1/deletion", status: 409, body: { status: 409 } }]);
    expect(await scheduleWorkspaceDeletion({ ok: false }, form({ id: "w1", name: "Home" }))).toEqual({
      ok: false,
      message: "This workspace is already scheduled for deletion.",
    });
    mockApi([{ method: "POST", path: "/api/v1/workspaces/w1/deletion", status: 403, body: { status: 403 } }]);
    expect(await scheduleWorkspaceDeletion({ ok: false }, form({ id: "w1", name: "Home" }))).toEqual({
      ok: false,
      message: "You don't have permission to do that.",
    });
  });

  it("cancels the deletion", async () => {
    const requests = mockApi([{ method: "DELETE", path: "/api/v1/workspaces/w1/deletion", status: 204 }]);
    expect(await cancelWorkspaceDeletion("w1")).toMatchObject({ ok: true });
    expect(requests[0]).toMatchObject({ method: "DELETE", path: "/api/v1/workspaces/w1/deletion" });

    mockApi([{ method: "DELETE", path: "/api/v1/workspaces/w1/deletion", status: 404, body: { status: 404 } }]);
    expect(await cancelWorkspaceDeletion("w1")).toEqual({ ok: false, message: "This item no longer exists." });
  });
});
