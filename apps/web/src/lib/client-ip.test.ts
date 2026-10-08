import { afterEach, describe, expect, it, vi } from "vitest";

import { clientIpFrom, clientIpHeaders, sessionClientHeaders } from "./client-ip";

const incoming = vi.hoisted(() => ({ forwardedFor: null as string | null, userAgent: null as string | null }));

vi.mock("server-only", () => ({}));
vi.mock("next/headers", () => ({
  headers: async () =>
    new Headers({
      ...(incoming.forwardedFor ? { "x-forwarded-for": incoming.forwardedFor } : {}),
      ...(incoming.userAgent ? { "user-agent": incoming.userAgent } : {}),
    }),
}));

afterEach(() => {
  vi.unstubAllEnvs();
  incoming.forwardedFor = null;
  incoming.userAgent = null;
});

describe("sessionClientHeaders", () => {
  it("forwards the browser's user agent with the client address", async () => {
    incoming.forwardedFor = "198.51.100.7";
    incoming.userAgent = "Mozilla/5.0 Firefox/131.0";
    vi.stubEnv("CLIENT_IP_SECRET", "s3cret");
    vi.stubEnv("TRUSTED_PROXY_HOPS", "1");
    expect(await sessionClientHeaders()).toEqual({
      "X-Client-IP": "198.51.100.7",
      "X-Client-IP-Secret": "s3cret",
      "User-Agent": "Mozilla/5.0 Firefox/131.0",
    });
  });

  it("sends nothing when the browser sent no user agent", async () => {
    expect(await sessionClientHeaders()).toEqual({});
  });
});

describe("clientIpFrom", () => {
  it("takes the entry the trusted proxy appended, ignoring forged ones", () => {
    expect(clientIpFrom("6.6.6.6, 198.51.100.7", 1)).toBe("198.51.100.7");
    expect(clientIpFrom("198.51.100.7, 10.0.0.1", 2)).toBe("198.51.100.7");
  });

  it("returns nothing without a header, hops or enough entries", () => {
    expect(clientIpFrom(null, 1)).toBeUndefined();
    expect(clientIpFrom("198.51.100.7", 0)).toBeUndefined();
    expect(clientIpFrom("198.51.100.7", 2)).toBeUndefined();
  });
});

describe("clientIpHeaders", () => {
  it("sends nothing unless the shared secret is configured", async () => {
    incoming.forwardedFor = "198.51.100.7";
    vi.stubEnv("TRUSTED_PROXY_HOPS", "1");
    expect(await clientIpHeaders()).toEqual({});
  });

  it("forwards the client address with the secret", async () => {
    incoming.forwardedFor = "6.6.6.6, 198.51.100.7";
    vi.stubEnv("CLIENT_IP_SECRET", "s3cret");
    vi.stubEnv("TRUSTED_PROXY_HOPS", "1");
    expect(await clientIpHeaders()).toEqual({ "X-Client-IP": "198.51.100.7", "X-Client-IP-Secret": "s3cret" });
  });

  it("sends nothing when the request carries no forwarded address", async () => {
    vi.stubEnv("CLIENT_IP_SECRET", "s3cret");
    vi.stubEnv("TRUSTED_PROXY_HOPS", "1");
    expect(await clientIpHeaders()).toEqual({});
  });
});
