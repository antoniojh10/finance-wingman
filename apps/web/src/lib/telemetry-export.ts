import { OTLPHttpJsonTraceExporter, OTLPHttpProtoTraceExporter } from "@vercel/otel";
import {
  BatchSpanProcessor,
  type ReadableSpan,
  type SpanExporter,
  type SpanProcessor,
} from "@opentelemetry/sdk-trace-base";

type Env = Record<string, string | undefined>;

const SENSITIVE_PARAM = /([?&](?:q|token|code|state|code_challenge|code_verifier|access_token|refresh_token)=)[^&#\s]*/gi;
const EMAIL = /[A-Za-z0-9._%+-]+(?:@|%40)[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+/g;

/**
 * Replaces the value of sensitive query parameters in any URL-like string:
 * `q` (search terms reveal what someone spends money on), and the `token`
 * of magic links and invitations or OAuth `code`/`state` values (credentials).
 * Email addresses are replaced too. None of them leave the server in telemetry.
 */
export function redactSearch(value: string): string {
  return value.replace(SENSITIVE_PARAM, "$1REDACTED").replace(EMAIL, "REDACTED");
}

function redactValue(value: unknown): unknown {
  if (typeof value === "string") return redactSearch(value);
  if (Array.isArray(value)) return value.map(redactValue);
  return value;
}

function redactAttributes<T extends Record<string, unknown> | undefined>(attributes: T): T {
  if (!attributes) return attributes;
  return Object.fromEntries(Object.entries(attributes).map(([key, value]) => [key, redactValue(value)])) as T;
}

function redactSpan(span: ReadableSpan): ReadableSpan {
  return {
    name: redactSearch(span.name),
    kind: span.kind,
    spanContext: () => span.spanContext(),
    parentSpanContext: span.parentSpanContext,
    startTime: span.startTime,
    endTime: span.endTime,
    status: span.status?.message ? { ...span.status, message: redactSearch(span.status.message) } : span.status,
    attributes: redactAttributes(span.attributes),
    links: span.links,
    events: (span.events ?? []).map((event) => ({ ...event, name: redactSearch(event.name), attributes: redactAttributes(event.attributes) })),
    duration: span.duration,
    ended: span.ended,
    resource: span.resource,
    instrumentationScope: span.instrumentationScope,
    droppedAttributesCount: span.droppedAttributesCount,
    droppedEventsCount: span.droppedEventsCount,
    droppedLinksCount: span.droppedLinksCount,
  };
}

/**
 * Wraps an exporter so span names, status messages, string attributes (fetch
 * URLs, Next.js `http.target`) and event attributes (exception messages) are
 * redacted before they are sent.
 */
export class RedactingSpanExporter implements SpanExporter {
  constructor(private readonly inner: SpanExporter) {}

  export(spans: ReadableSpan[], resultCallback: Parameters<SpanExporter["export"]>[1]): void {
    this.inner.export(spans.map(redactSpan), resultCallback);
  }

  shutdown(): Promise<void> {
    return this.inner.shutdown();
  }

  forceFlush(): Promise<void> {
    return this.inner.forceFlush?.() ?? Promise.resolve();
  }
}

/** Parses OTEL_EXPORTER_OTLP_HEADERS (`k=v,k2=v2`, values URL-encoded). */
export function parseHeaders(value: string | undefined): Record<string, string> {
  const headers: Record<string, string> = {};
  for (const pair of value?.split(",") ?? []) {
    const index = pair.indexOf("=");
    if (index <= 0) continue;
    headers[decodeURIComponent(pair.slice(0, index).trim())] = decodeURIComponent(pair.slice(index + 1).trim());
  }
  return headers;
}

/**
 * The OTLP trace exporter `@vercel/otel` would build from the standard
 * environment variables, wrapped in {@link RedactingSpanExporter}.
 */
export function traceExporterFromEnv(env: Env): SpanExporter {
  const url = env.OTEL_EXPORTER_OTLP_TRACES_ENDPOINT || `${env.OTEL_EXPORTER_OTLP_ENDPOINT}/v1/traces`;
  const headers = {
    ...parseHeaders(env.OTEL_EXPORTER_OTLP_HEADERS),
    ...parseHeaders(env.OTEL_EXPORTER_OTLP_TRACES_HEADERS),
  };
  const protocol = env.OTEL_EXPORTER_OTLP_TRACES_PROTOCOL ?? env.OTEL_EXPORTER_OTLP_PROTOCOL;
  const exporter =
    protocol === "http/json"
      ? new OTLPHttpJsonTraceExporter({ url, headers })
      : new OTLPHttpProtoTraceExporter({ url, headers });
  return new RedactingSpanExporter(exporter);
}

/**
 * Replaces `@vercel/otel`'s default ("auto") span processors, which would
 * export spans unredacted. The Vercel runtime exporter they add is not
 * needed outside Vercel.
 */
export function spanProcessorsFromEnv(env: Env): SpanProcessor[] {
  return [new BatchSpanProcessor(traceExporterFromEnv(env))];
}
