import "@/test/server-mocks";

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { cookieJar, mockApi } from "@/test/server-mocks";

import { cancelAccountDeletion, cancelWorkspaceDeletion, scheduleAccountDeletion, scheduleWorkspaceDeletion } from "./deletion";

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

describe("account deletion actions", () => {
  it("schedules the deletion with the typed email", async () => {
    const requests = mockApi([{ method: "POST", path: "/api/v1/auth/me/deletion", status: 200, body: { scheduled_for: "2026-10-15T00:00:00Z", workspaces: [] } }]);
    expect(await scheduleAccountDeletion({ ok: false }, form({ email: " ana@example.com " }))).toMatchObject({ ok: true });
    expect(requests[0]).toMatchObject({ method: "POST", body: { email: "ana@example.com" }, auth: "Bearer tok_123" });
  });

  it("validates the email", async () => {
    const requests = mockApi([]);
    expect(await scheduleAccountDeletion({ ok: false }, form({ email: "" }))).toMatchObject({ fieldErrors: { email: "This field is required." } });
    expect(requests).toHaveLength(0);
    mockApi([{ method: "POST", path: "/api/v1/auth/me/deletion", status: 422, body: { status: 422 } }]);
    expect(await scheduleAccountDeletion({ ok: false }, form({ email: "bob@example.com" }))).toMatchObject({
      ok: false,
      fieldErrors: { email: "Type your email address exactly." },
    });
  });

  it("explains conflicts", async () => {
    mockApi([{ method: "POST", path: "/api/v1/auth/me/deletion", status: 409, body: { status: 409, detail: 'you are the only owner of "Home", which has other members' } }]);
    expect(await scheduleAccountDeletion({ ok: false }, form({ email: "ana@example.com" }))).toEqual({
      ok: false,
      message: "You're the only owner of a workspace with other members. Make another member an owner, or delete it, first.",
    });
    mockApi([{ method: "POST", path: "/api/v1/auth/me/deletion", status: 409, body: { status: 409, detail: "your account is already scheduled for deletion" } }]);
    expect(await scheduleAccountDeletion({ ok: false }, form({ email: "ana@example.com" }))).toEqual({
      ok: false,
      message: "Your account is already scheduled for deletion.",
    });
  });

  it("cancels the deletion", async () => {
    const requests = mockApi([{ method: "DELETE", path: "/api/v1/auth/me/deletion", status: 204 }]);
    expect(await cancelAccountDeletion()).toMatchObject({ ok: true });
    expect(requests[0]).toMatchObject({ method: "DELETE", path: "/api/v1/auth/me/deletion" });
    mockApi([{ method: "DELETE", path: "/api/v1/auth/me/deletion", status: 404, body: { status: 404 } }]);
    expect(await cancelAccountDeletion()).toEqual({ ok: false, message: "This item no longer exists." });
  });
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
