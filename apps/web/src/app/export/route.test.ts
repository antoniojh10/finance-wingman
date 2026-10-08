import "@/test/server-mocks";

import { NextRequest } from "next/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { cookieJar } from "@/test/server-mocks";

import { GET } from "./route";

function call(query = "") {
  return GET(new NextRequest(`http://localhost:3000/export${query}`));
}

function stubApi(response: Response) {
  const fetchMock = vi.fn(async () => response);
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

beforeEach(() => {
  cookieJar.clear();
  cookieJar.set("fw_session", { value: "tok_123" });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("export route", () => {
  it("streams the API export with the session token", async () => {
    const fetchMock = stubApi(
      new Response('{"schema_version":1}', {
        headers: {
          "Content-Type": "application/json",
          "Content-Disposition": 'attachment; filename="finance-wingman-export-2026-10-08.json"',
        },
      }),
    );

    const res = await call("?format=json");

    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Type")).toBe("application/json");
    expect(res.headers.get("Content-Disposition")).toContain("finance-wingman-export-2026-10-08.json");
    expect(res.headers.get("Cache-Control")).toBe("no-store");
    expect(await res.text()).toBe('{"schema_version":1}');
    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("http://localhost:8080/api/v1/export?format=json");
    expect(init.headers).toEqual({ Authorization: "Bearer tok_123" });
  });

  it("requests CSV as a zip", async () => {
    const fetchMock = stubApi(new Response("zip", { headers: { "Content-Type": "application/zip" } }));
    const res = await call("?format=csv");
    expect(res.headers.get("Content-Type")).toBe("application/zip");
    expect((fetchMock.mock.calls[0] as unknown as [string])[0]).toContain("format=csv");
  });

  it("defaults to JSON", async () => {
    const fetchMock = stubApi(new Response("{}"));
    await call();
    expect((fetchMock.mock.calls[0] as unknown as [string])[0]).toContain("format=json");
  });

  it("rejects signed-out requests without calling the API", async () => {
    cookieJar.clear();
    const fetchMock = stubApi(new Response("{}"));
    expect((await call()).status).toBe(401);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("rejects unknown formats without calling the API", async () => {
    const fetchMock = stubApi(new Response("{}"));
    expect((await call("?format=xml")).status).toBe(400);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("passes an expired session through and hides other API errors", async () => {
    stubApi(new Response("{}", { status: 401 }));
    expect((await call()).status).toBe(401);
    stubApi(new Response("{}", { status: 500 }));
    expect((await call()).status).toBe(502);
  });

  it("reports an unreachable API", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("connection refused")));
    expect((await call()).status).toBe(502);
  });
});
