import { afterEach, describe, expect, it, vi } from "vitest";

const { registerOTel } = vi.hoisted(() => ({ registerOTel: vi.fn() }));
vi.mock("@vercel/otel", () => ({
  registerOTel,
  OTLPHttpProtoTraceExporter: vi.fn(),
  OTLPHttpJsonTraceExporter: vi.fn(),
}));

import { register } from "./instrumentation";

describe("register", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
    registerOTel.mockClear();
  });

  it("registers OpenTelemetry on the Node.js runtime when an endpoint is set", async () => {
    vi.stubEnv("NEXT_RUNTIME", "nodejs");
    vi.stubEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "https://otlp.example");
    await register();
    expect(registerOTel).toHaveBeenCalledWith(expect.objectContaining({ serviceName: "finance-web" }));
    // The default processors would export spans without redacting search terms.
    expect(registerOTel.mock.calls[0][0].spanProcessors).toHaveLength(1);
  });

  it("does nothing without an endpoint", async () => {
    vi.stubEnv("NEXT_RUNTIME", "nodejs");
    vi.stubEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "");
    await register();
    expect(registerOTel).not.toHaveBeenCalled();
  });

  it("does nothing on the edge runtime", async () => {
    vi.stubEnv("NEXT_RUNTIME", "edge");
    vi.stubEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "https://otlp.example");
    await register();
    expect(registerOTel).not.toHaveBeenCalled();
  });
});
