# Observability

The API and the web app export OpenTelemetry traces (and the API also
metrics) over OTLP/HTTP. Export is **off unless an endpoint is configured**,
so local development and tests are unaffected.

## What is recorded

| Source | Signal | Details |
| --- | --- | --- |
| Web (Next.js) | traces | Page renders, Server Actions and every `fetch` to the API (`@vercel/otel`, `src/instrumentation.ts`) |
| API HTTP | traces + metrics | One span per request, named after the route (`GET /api/v1/accounts/{id}`); `http.server.request.duration` labelled with `http.route`. `/healthz` is skipped |
| API MCP | traces | One span per MCP request; tool calls are named `tools/call <tool>` and marked as errors when the tool returns one |
| API database | traces + metrics | One span per query, named after the sqlc query (`ListAccounts`); connection pool statistics (`pgxpool.*`) |
| API runtime | metrics | Go heap, GC, goroutines (`go.*`) |
| API logs | — | Request logs include `trace_id` to jump from a log line to its trace |

The web app propagates W3C trace context to `API_URL`, so a page view, the
API requests it triggers, and their SQL queries appear in a single trace.

## Configuration

Both services use the standard OpenTelemetry environment variables:

| Variable | Notes |
| --- | --- |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Base OTLP/HTTP URL; `/v1/traces` and `/v1/metrics` are appended. Setting it enables export |
| `OTEL_EXPORTER_OTLP_HEADERS` | Comma-separated `key=value` pairs, e.g. `Authorization=Basic …` |
| `OTEL_SERVICE_NAME` | Defaults to `finance-api` / `finance-web` |
| `OTEL_SDK_DISABLED` | `true` turns export off even with an endpoint |

The API also accepts `PPROF_ADDR` (e.g. `localhost:6060`) to serve
`net/http/pprof` on a separate listener. Never point it at the public port;
on Railway or Seenode a second port is only reachable on the private network.

## Local trace viewer

```bash
make jaeger      # Jaeger UI at http://localhost:16686, OTLP on :4318
```

Set `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318` in `.env` and
`apps/web/.env.local`, restart `make api-run` and `make web-dev`, and load a
page.

## Grafana Cloud (free tier)

1. Create a stack and open **Connections → OpenTelemetry (OTLP)**.
2. Generate a token; Grafana shows the endpoint and an
   `Authorization=Basic …` header.
3. Set on both services:

   ```bash
   OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp-gateway-<region>.grafana.net/otlp
   OTEL_EXPORTER_OTLP_HEADERS=Authorization=Basic%20<base64 instance:token>
   ```

   (Encode the space after `Basic` as `%20`.) Traces appear in Tempo and
   metrics in Prometheus/Mimir. Honeycomb, Axiom and other OTLP backends work
   the same way with their own endpoint and header.

## Cost

With export off, instrumentation uses the no-op providers. With export on,
expect a few MB of extra memory per service and one batched request every
few seconds to the backend.
