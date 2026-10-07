import { OTLPHttpJsonTraceExporter, OTLPHttpProtoTraceExporter } from "@vercel/otel";
import {
  BatchSpanProcessor,
  type ReadableSpan,
  type SpanExporter,
  type SpanProcessor,
} from "@opentelemetry/sdk-trace-base";

type Env = Record<string, string | undefined>;

const SEARCH_PARAM = /([?&]q=)[^&#\s]*/g;

/**
 * Replaces the value of the `q` search parameter in any URL-like string.
 * Search terms can reveal what someone spends money on, so they never leave
 * the server in telemetry.
 */
export function redactSearch(value: string): string {
  return value.replace(SEARCH_PARAM, "$1REDACTED");
}

function redactSpan(span: ReadableSpan): ReadableSpan {
  const attributes = Object.fromEntries(
    Object.entries(span.attributes).map(([key, value]) => [key, typeof value === "string" ? redactSearch(value) : value]),
  );
  return {
    name: redactSearch(span.name),
    kind: span.kind,
    spanContext: () => span.spanContext(),
    parentSpanContext: span.parentSpanContext,
    startTime: span.startTime,
    endTime: span.endTime,
    status: span.status,
    attributes,
    links: span.links,
    events: span.events,
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
 * Wraps an exporter so span names and string attributes (fetch URLs, Next.js
 * `http.target`) are redacted before they are sent.
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
