# Observability

The API and the web app export OpenTelemetry traces (and the API also
metrics and logs) over OTLP/HTTP. Export is **off unless an endpoint is configured**,
so local development and tests are unaffected.

## What is recorded

| Source | Signal | Details |
| --- | --- | --- |
| Web (Next.js) | traces | Page renders, Server Actions and every `fetch` to the API (`@vercel/otel`, `src/instrumentation.ts`) |
| API HTTP | traces + metrics | One span per request, named after the route (`GET /api/v1/accounts/{id}`); `http.server.request.duration` labelled with `http.route`. `/healthz` is skipped |
| API MCP | traces | One span per MCP request; tool calls are named `tools/call <tool>` and marked as errors when the tool returns one |
| API database | traces + metrics | One span per query, named after the sqlc query (`ListAccounts`); connection pool statistics (`pgxpool.*`) |
| API runtime | metrics | Go heap, GC, goroutines (`go.*`) |
| API logs | logs | Every `slog` record at info level or above, with `trace_id`/`span_id` when logged inside a request. Logs always go to stdout as JSON as well; debug records stay on stdout only |

The web app propagates W3C trace context to `API_URL`, so a page view, the
API requests it triggers, and their SQL queries appear in a single trace.

## Configuration

Both services use the standard OpenTelemetry environment variables:

| Variable | Notes |
| --- | --- |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Base OTLP/HTTP URL; `/v1/traces`, `/v1/metrics` and `/v1/logs` are appended. Setting it enables export |
| `OTEL_EXPORTER_OTLP_HEADERS` | Comma-separated `key=value` pairs, e.g. `Authorization=Basic%20…` (values are URL-decoded) |
| `OTEL_RESOURCE_ATTRIBUTES` | `deployment.environment.name=development` locally, `production` in production, so the data can be filtered apart |
| `OTEL_SERVICE_NAME` | Defaults to `finance-api` / `finance-web`; leave unset when one `.env` feeds both services |
| `OTEL_SDK_DISABLED` | `true` turns export off even with an endpoint |

The API also accepts `PPROF_ADDR` (e.g. `localhost:6060`) to serve
`net/http/pprof` on a separate listener. Never point it at the public port;
on Railway or Seenode a second port is only reachable on the private network.

## Local Grafana

```bash
make grafana     # Grafana at http://localhost:3300, OTLP/HTTP on :4318
```

`make grafana` runs [`grafana/otel-lgtm`](https://github.com/grafana/docker-otel-lgtm):
an OpenTelemetry Collector plus Tempo (traces), Loki (logs), Mimir/Prometheus
(metrics) and Grafana in one container, the same backends as Grafana Cloud.
Local development sends here; Grafana Cloud only receives production data.

Set `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318` in `.env` and
`apps/web/.env.local`, restart `make api-run` and `make web-dev`, and load a
page. In Grafana (no login), open **Explore** or **Drilldown** and pick the
Tempo, Loki or Prometheus data source. Data is kept in the `grafana-data`
volume; `make down` stops the container (the volume survives). Worktrees
share it: run `make grafana` from the main checkout. Override the ports with
`GRAFANA_UI_PORT` and `OTLP_HTTP_PORT` in `.env`.

Each data source links to the others: from a span in Tempo, **Logs for this
span** opens its log lines in Loki, and a log line with a `trace_id` links
back to the trace.

## Grafana Cloud (free tier)

1. In **grafana.com → My Account**, select the stack and click
   **Configure** on the **OpenTelemetry** tile.
2. Note the **OTLP endpoint** and **Instance ID**, then generate a token
   (**Password / API Token → Generate now**). The default scopes
   (`metrics:write`, `logs:write`, `traces:write`) are enough.
3. Set on both services, as secrets on the hosting platform:

   ```bash
   OTEL_EXPORTER_OTLP_ENDPOINT=https://otlp-gateway-<region>.grafana.net/otlp
   OTEL_EXPORTER_OTLP_HEADERS=Authorization=Basic%20<base64 instance:token>
   OTEL_RESOURCE_ATTRIBUTES=deployment.environment.name=production
   ```

   Encode the space after `Basic` as `%20` and don't quote the value. The
   endpoint ends in `/otlp`, without `/v1/...`.
4. In the stack's Grafana: traces are in Tempo, logs in Loki and metrics in
   Prometheus/Mimir (**Explore** or **Drilldown**);
   **Application Observability** groups them per service and environment.

Honeycomb, Axiom and other OTLP backends work the same way with their own
endpoint and header.

### Environments in a single stack

The free tier has one stack, so environments are told apart by resource
attributes rather than by stack:

- Local development sends to `make grafana`. Point `.env` at Grafana Cloud
  only to test something against it, with
  `deployment.environment.name=development`.
- Use one token per environment (e.g. `finance-dev-otlp` with an expiry and
  `finance-prod-otlp`), so the development one can be revoked on its own.
- Filter alerts and dashboards by environment: `deployment_environment_name`
  is a Loki label and a Tempo resource attribute; on Prometheus metrics it
  is on `target_info`. Setting `service.namespace` (e.g. `finance` vs
  `finance-dev`) in `OTEL_RESOURCE_ATTRIBUTES` also prefixes the `job` label
  of every metric.
- Development data counts towards the free tier limits (10k active metric
  series, 50 GB of logs and traces per month); check **Usage** in the
  stack.

## Privacy

This is a finance app, so telemetry must not contain amounts, descriptions,
MCP tool arguments or search terms:

- API HTTP spans record the path and route, not the query string; request
  logs record the path only.
- The web app redacts the `q` search parameter in span names and string
  attributes before export (`src/lib/telemetry-export.ts`); other query
  parameters (date ranges, account IDs) are kept.
- SQL spans contain the query text with `$n` placeholders, never parameter
  values. MCP spans record the tool name only.
- Emails printed by `EMAIL_PROVIDER=log` (magic links) are written to
  stdout only, never exported.

## Cost

With export off, instrumentation uses the no-op providers. With export on,
expect a few MB of extra memory per service and one batched request every
few seconds to the backend.
