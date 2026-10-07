import type { ReadableSpan, SpanExporter } from "@opentelemetry/sdk-trace-base";
import { describe, expect, it, vi } from "vitest";

import { parseHeaders, RedactingSpanExporter, redactSearch } from "./telemetry-export";

describe("redactSearch", () => {
  it("redacts the q parameter and keeps the rest of the URL", () => {
    expect(redactSearch("http://api:8080/api/v1/transactions?q=pharmacy&from=2026-01-01")).toBe(
      "http://api:8080/api/v1/transactions?q=REDACTED&from=2026-01-01",
    );
    expect(redactSearch("/transactions?from=2026-01-01&q=rent%20flat")).toBe("/transactions?from=2026-01-01&q=REDACTED");
    expect(redactSearch("fetch GET http://api/x?q=a b")).toBe("fetch GET http://api/x?q=REDACTED b");
  });

  it("leaves other parameters and plain text untouched", () => {
    expect(redactSearch("/transactions?account=abc&qty=2")).toBe("/transactions?account=abc&qty=2");
    expect(redactSearch("GET /transactions")).toBe("GET /transactions");
  });
});

describe("RedactingSpanExporter", () => {
  it("redacts span names and string attributes before exporting", () => {
    const exported: ReadableSpan[] = [];
    const inner: SpanExporter = {
      export: (spans, done) => {
        exported.push(...spans);
        done({ code: 0 });
      },
      shutdown: vi.fn(async () => {}),
    };
    const spanContext = { traceId: "t", spanId: "s", traceFlags: 1 };
    const span = {
      name: "fetch GET http://api/api/v1/transactions?q=pharmacy",
      attributes: { "http.url": "http://api/api/v1/transactions?q=pharmacy", "http.status_code": 200 },
      spanContext: () => spanContext,
    } as unknown as ReadableSpan;
    const done = vi.fn();

    new RedactingSpanExporter(inner).export([span], done);

    expect(done).toHaveBeenCalledWith({ code: 0 });
    expect(exported[0].name).toBe("fetch GET http://api/api/v1/transactions?q=REDACTED");
    expect(exported[0].attributes).toEqual({
      "http.url": "http://api/api/v1/transactions?q=REDACTED",
      "http.status_code": 200,
    });
    expect(exported[0].spanContext()).toBe(spanContext);
  });

  it("delegates shutdown and flush", async () => {
    const inner: SpanExporter = { export: vi.fn(), shutdown: vi.fn(async () => {}), forceFlush: vi.fn(async () => {}) };
    const exporter = new RedactingSpanExporter(inner);
    await exporter.forceFlush();
    await exporter.shutdown();
    expect(inner.forceFlush).toHaveBeenCalled();
    expect(inner.shutdown).toHaveBeenCalled();
  });
});

describe("parseHeaders", () => {
  it("decodes URL-encoded values like Grafana's Basic auth header", () => {
    expect(parseHeaders("Authorization=Basic%20abc==,X-Scope=1")).toEqual({ Authorization: "Basic abc==", "X-Scope": "1" });
  });

  it("returns no headers when unset", () => {
    expect(parseHeaders(undefined)).toEqual({});
  });
});
