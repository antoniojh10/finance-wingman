import type { Configuration } from "@vercel/otel";

import { apiBaseUrl } from "@/lib/api/client";

type Env = Record<string, string | undefined>;

export const DEFAULT_SERVICE_NAME = "finance-web";

/**
 * Whether OpenTelemetry export is configured. Like the API, the web app only
 * exports when an OTLP endpoint is set, so local runs and tests are unchanged.
 */
export function otlpConfigured(env: Env): boolean {
  if (env.OTEL_SDK_DISABLED?.toLowerCase() === "true") return false;
  return Boolean(env.OTEL_EXPORTER_OTLP_ENDPOINT || env.OTEL_EXPORTER_OTLP_TRACES_ENDPOINT);
}

/**
 * `@vercel/otel` configuration. The exporter itself reads the standard
 * OTEL_EXPORTER_OTLP_* variables. Trace context is propagated to the Go API
 * so a page render and the API requests and SQL queries it causes share one
 * trace.
 */
export function otelConfig(env: Env): Configuration {
  return {
    serviceName: env.OTEL_SERVICE_NAME || DEFAULT_SERVICE_NAME,
    instrumentationConfig: {
      fetch: { propagateContextUrls: [apiBaseUrl()] },
    },
  };
}
