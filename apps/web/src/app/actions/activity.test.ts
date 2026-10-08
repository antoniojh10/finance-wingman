import "@/test/server-mocks";

import { beforeEach, describe, expect, it } from "vitest";

import { cookieJar, mockApi } from "@/test/server-mocks";

import { fetchActivityPage, loadMoreActivity } from "./activity";

beforeEach(() => {
  cookieJar.clear();
  cookieJar.set("fw_session", { value: "tok" });
});

describe("fetchActivityPage", () => {
  it("sends the filters and returns the items with the next cursor", async () => {
    const requests = mockApi([{ method: "GET", path: "/api/v1/activity", status: 200, body: { items: [{ id: "e1" }], limit: 25, next_cursor: "c2" } }]);
    const page = await fetchActivityPage({ channel: "mcp", entity_type: "transaction" });
    expect(page).toEqual({ items: [{ id: "e1" }], nextCursor: "c2" });
    expect(requests[0].path).toBe("/api/v1/activity?channel=mcp&entity_type=transaction&limit=25");
    expect(requests[0].auth).toBe("Bearer tok");
  });
});

describe("loadMoreActivity", () => {
  it("passes the cursor and re-validates the filters", async () => {
    const requests = mockApi([{ method: "GET", path: "/api/v1/activity", status: 200, body: { items: [], limit: 25, next_cursor: null } }]);
    const page = await loadMoreActivity({ channel: "mcp", entity_type: "unicorn", actor_id: "nope" }, "c2");
    expect(page).toEqual({ items: [], nextCursor: null });
    expect(requests[0].path).toBe("/api/v1/activity?channel=mcp&cursor=c2&limit=25");
  });

  it("fails when the API rejects the request", async () => {
    mockApi([{ method: "GET", path: "/api/v1/activity", status: 400, body: { status: 400, detail: "invalid cursor" } }]);
    await expect(loadMoreActivity({}, "bad")).rejects.toThrow();
  });
});
