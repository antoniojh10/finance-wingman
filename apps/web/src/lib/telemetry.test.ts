import { afterEach, describe, expect, it, vi } from "vitest";

import { DEFAULT_SERVICE_NAME, otelConfig, otlpConfigured } from "./telemetry";

describe("otlpConfigured", () => {
  it("is off without an endpoint", () => {
    expect(otlpConfigured({})).toBe(false);
  });

  it("is on with a shared or traces endpoint", () => {
    expect(otlpConfigured({ OTEL_EXPORTER_OTLP_ENDPOINT: "https://otlp.example" })).toBe(true);
    expect(otlpConfigured({ OTEL_EXPORTER_OTLP_TRACES_ENDPOINT: "https://otlp.example/v1/traces" })).toBe(true);
  });

  it("respects OTEL_SDK_DISABLED", () => {
    expect(otlpConfigured({ OTEL_EXPORTER_OTLP_ENDPOINT: "https://otlp.example", OTEL_SDK_DISABLED: "TRUE" })).toBe(false);
  });
});

describe("otelConfig", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("uses the default service name and propagates context to the API", () => {
    vi.stubEnv("API_URL", "http://finance-api:8080");
    const config = otelConfig({});
    expect(config.serviceName).toBe(DEFAULT_SERVICE_NAME);
    expect(config.instrumentationConfig?.fetch?.propagateContextUrls).toEqual(["http://finance-api:8080"]);
  });

  it("honours OTEL_SERVICE_NAME", () => {
    expect(otelConfig({ OTEL_SERVICE_NAME: "web-staging" }).serviceName).toBe("web-staging");
  });
});
