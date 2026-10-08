import { NextRequest } from "next/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { proxy } from "./proxy";

const nonceOf = (csp: string | null) => /'nonce-([^']+)'/.exec(csp ?? "")?.[1];

describe("proxy", () => {
  afterEach(() => vi.unstubAllEnvs());

  it("adds the security headers to the response", () => {
    vi.stubEnv("NODE_ENV", "production");
    const response = proxy(new NextRequest("https://app.example/login"));
    expect(response.headers.get("content-security-policy")).toContain("frame-ancestors 'none'");
    expect(response.headers.get("x-content-type-options")).toBe("nosniff");
    expect(response.headers.get("strict-transport-security")).toContain("max-age=");
  });

  it("generates a fresh nonce per request and forwards it to the page", () => {
    vi.stubEnv("NODE_ENV", "production");
    const a = proxy(new NextRequest("https://app.example/"));
    const b = proxy(new NextRequest("https://app.example/"));
    const nonce = nonceOf(a.headers.get("content-security-policy"));
    expect(nonce).toBeTruthy();
    expect(nonce).not.toBe(nonceOf(b.headers.get("content-security-policy")));
    // Next.js forwards overridden request headers as x-middleware-request-*.
    expect(a.headers.get("x-middleware-request-x-nonce")).toBe(nonce);
    expect(a.headers.get("x-middleware-request-content-security-policy")).toContain(`'nonce-${nonce}'`);
  });

  it("skips HSTS in development", () => {
    vi.stubEnv("NODE_ENV", "development");
    const response = proxy(new NextRequest("http://localhost:3000/"));
    expect(response.headers.get("strict-transport-security")).toBeNull();
    expect(response.headers.get("content-security-policy")).toContain("'unsafe-eval'");
  });
});
