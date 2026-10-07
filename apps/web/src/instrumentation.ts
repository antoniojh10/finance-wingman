import { otelConfig, otlpConfigured } from "@/lib/telemetry";

// Runs once when a Next.js server instance starts (see instrumentation.md).
export async function register() {
  if (process.env.NEXT_RUNTIME !== "nodejs" || !otlpConfigured(process.env)) return;
  const [{ registerOTel }, { spanProcessorsFromEnv }] = await Promise.all([
    import("@vercel/otel"),
    import("@/lib/telemetry-export"),
  ]);
  registerOTel({ ...otelConfig(process.env), spanProcessors: spanProcessorsFromEnv(process.env) });
}
