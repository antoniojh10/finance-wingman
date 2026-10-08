import "@/test/server-mocks";

import { revalidatePath } from "next/cache";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { cookieJar, mockApi, RedirectError } from "@/test/server-mocks";

import { disconnectApp, revokeOtherSessions, revokeSession } from "./security";

beforeEach(() => {
  cookieJar.clear();
  cookieJar.set("fw_session", { value: "tok_123" });
  vi.mocked(revalidatePath).mockClear();
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("security actions", () => {
  it("signs out a session", async () => {
    const requests = mockApi([{ method: "DELETE", path: "/api/v1/auth/sessions/s2", status: 204 }]);
    expect(await revokeSession("s2")).toMatchObject({ ok: true });
    expect(requests[0]).toMatchObject({ method: "DELETE", path: "/api/v1/auth/sessions/s2", auth: "Bearer tok_123" });
    expect(revalidatePath).toHaveBeenCalledWith("/settings/security");
  });

  it("reports sessions that no longer exist", async () => {
    mockApi([{ method: "DELETE", path: "/api/v1/auth/sessions/s9", status: 404, body: { status: 404 } }]);
    expect(await revokeSession("s9")).toEqual({ ok: false, message: "This item no longer exists." });
    expect(revalidatePath).not.toHaveBeenCalled();
  });

  it("signs out everywhere else", async () => {
    const requests = mockApi([{ method: "POST", path: "/api/v1/auth/sessions/revoke-others", status: 200, body: { revoked: 2 } }]);
    expect(await revokeOtherSessions()).toMatchObject({ ok: true });
    expect(requests[0]).toMatchObject({ method: "POST", auth: "Bearer tok_123" });
    expect(revalidatePath).toHaveBeenCalledWith("/settings/security");
  });

  it("disconnects an app", async () => {
    const requests = mockApi([{ method: "DELETE", path: "/api/v1/auth/connections/c1", status: 204 }]);
    expect(await disconnectApp("c1")).toMatchObject({ ok: true });
    expect(requests[0]).toMatchObject({ method: "DELETE", path: "/api/v1/auth/connections/c1", auth: "Bearer tok_123" });
  });

  it("reports failures and expired sessions", async () => {
    mockApi([{ method: "DELETE", path: "/api/v1/auth/connections/c1", status: 403, body: { status: 403 } }]);
    expect(await disconnectApp("c1")).toMatchObject({ ok: false });
    mockApi([{ method: "POST", path: "/api/v1/auth/sessions/revoke-others", status: 401, body: { status: 401 } }]);
    await expect(revokeOtherSessions()).rejects.toEqual(new RedirectError("/auth/signout"));
  });
});
