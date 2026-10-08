import { describe, expect, it } from "vitest";

import { contentSecurityPolicy, securityHeaders } from "./security-headers";

describe("contentSecurityPolicy", () => {
  it("locks down framing, plugins, base URI and forms in production", () => {
    const csp = contentSecurityPolicy({ nonce: "abc", isDev: false });
    expect(csp).toContain("default-src 'self'");
    expect(csp).toContain("script-src 'self' 'nonce-abc' 'strict-dynamic'");
    expect(csp).toContain("style-src 'self' 'unsafe-inline'");
    expect(csp).toContain("frame-ancestors 'none'");
    expect(csp).toContain("object-src 'none'");
    expect(csp).toContain("base-uri 'self'");
    expect(csp).toContain("form-action 'self'");
    expect(csp).not.toContain("unsafe-eval");
    expect(csp).not.toContain("ws:");
  });

  it("allows eval and websockets in development only", () => {
    const csp = contentSecurityPolicy({ nonce: "abc", isDev: true });
    expect(csp).toContain("'unsafe-eval'");
    expect(csp).toContain("connect-src 'self' ws: wss:");
  });
});

describe("securityHeaders", () => {
  it("sends the full set with HSTS in production", () => {
    const headers = securityHeaders({ nonce: "abc", isDev: false });
    expect(headers["X-Content-Type-Options"]).toBe("nosniff");
    expect(headers["Referrer-Policy"]).toBe("strict-origin-when-cross-origin");
    expect(headers["Permissions-Policy"]).toBe("camera=(), microphone=(), geolocation=()");
    expect(headers["Strict-Transport-Security"]).toBe("max-age=63072000; includeSubDomains");
    expect(headers["Content-Security-Policy"]).toContain("'nonce-abc'");
  });

  it("omits HSTS in development", () => {
    expect(securityHeaders({ nonce: "abc", isDev: true })).not.toHaveProperty("Strict-Transport-Security");
  });
});
