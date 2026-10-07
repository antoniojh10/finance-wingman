package httpapi

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

type instrumentedHandler struct {
	handler http.Handler
	spans   *tracetest.SpanRecorder
	metrics *sdkmetric.ManualReader
	logs    *bytes.Buffer
}

func newInstrumentedHandler(t *testing.T) instrumentedHandler {
	t.Helper()
	spans := tracetest.NewSpanRecorder()
	metrics := sdkmetric.NewManualReader()
	logs := &bytes.Buffer{}
	handler := NewHandler(Deps{
		Logger:         slog.New(slog.NewJSONHandler(logs, nil)),
		DB:             fakePinger{},
		Auth:           &auth.Service{},
		Finance:        &finance.Service{},
		TracerProvider: sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)),
		MeterProvider:  sdkmetric.NewMeterProvider(sdkmetric.WithReader(metrics)),
	})
	return instrumentedHandler{handler: handler, spans: spans, metrics: metrics, logs: logs}
}

func TestTelemetryNamesSpansAfterRoute(t *testing.T) {
	h := newInstrumentedHandler(t)

	// Unauthenticated, but the route still matches.
	rec := get(t, h.handler, "/api/v1/accounts/0b5d6c1e-8a4f-4a52-9a43-0d2f1f3a8c11")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	ended := h.spans.Ended()
	if len(ended) != 1 {
		t.Fatalf("expected 1 span, got %d", len(ended))
	}
	if got, want := ended[0].Name(), "GET /api/v1/accounts/{id}"; got != want {
		t.Fatalf("span name = %q, want %q", got, want)
	}
	if !hasAttribute(ended[0].Attributes(), "http.route", "/api/v1/accounts/{id}") {
		t.Fatalf("missing http.route attribute: %v", ended[0].Attributes())
	}

	traceID := ended[0].SpanContext().TraceID().String()
	if !strings.Contains(h.logs.String(), `"trace_id":"`+traceID+`"`) {
		t.Fatalf("request log does not include trace_id %s: %s", traceID, h.logs)
	}
}

func TestTelemetryLabelsMetricsWithRoute(t *testing.T) {
	h := newInstrumentedHandler(t)
	get(t, h.handler, "/api/v1/accounts/0b5d6c1e-8a4f-4a52-9a43-0d2f1f3a8c11")

	var data metricdata.ResourceMetrics
	if err := h.metrics.Collect(context.Background(), &data); err != nil {
		t.Fatal(err)
	}
	for _, scope := range data.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != "http.server.request.duration" {
				continue
			}
			hist, ok := m.Data.(metricdata.Histogram[float64])
			if !ok || len(hist.DataPoints) == 0 {
				t.Fatalf("unexpected duration data: %#v", m.Data)
			}
			if v, ok := hist.DataPoints[0].Attributes.Value("http.route"); !ok || v.AsString() != "/api/v1/accounts/{id}" {
				t.Fatalf("duration metric not labelled with route: %v", hist.DataPoints[0].Attributes)
			}
			return
		}
	}
	t.Fatal("http.server.request.duration was not recorded")
}

func TestTelemetrySkipsHealthAndLabelsUnmatched(t *testing.T) {
	h := newInstrumentedHandler(t)
	get(t, h.handler, "/healthz")
	get(t, h.handler, "/no-such-path")

	ended := h.spans.Ended()
	if len(ended) != 1 {
		t.Fatalf("expected only the unmatched request to be traced, got %d spans", len(ended))
	}
	if got := ended[0].Name(); got != "GET unmatched" {
		t.Fatalf("span name = %q, want %q", got, "GET unmatched")
	}
}

// Search terms can reveal spending, so the query string must stay out of
// spans and request logs (both are exported to the telemetry backend).
func TestTelemetryOmitsQueryString(t *testing.T) {
	h := newInstrumentedHandler(t)
	const secret = "pharmacy-secret"
	get(t, h.handler, "/api/v1/transactions?q="+secret+"&from=2026-01-01")

	ended := h.spans.Ended()
	if len(ended) != 1 {
		t.Fatalf("expected 1 span, got %d", len(ended))
	}
	if strings.Contains(ended[0].Name(), secret) {
		t.Fatalf("span name contains the query: %q", ended[0].Name())
	}
	for _, a := range ended[0].Attributes() {
		if strings.Contains(a.Value.Emit(), secret) {
			t.Fatalf("span attribute %s contains the query: %q", a.Key, a.Value.Emit())
		}
	}
	if strings.Contains(h.logs.String(), secret) {
		t.Fatalf("request log contains the query: %s", h.logs)
	}
}

func TestTelemetryContinuesIncomingTrace(t *testing.T) {
	h := newInstrumentedHandler(t)
	const parentTraceID = "4bf92f3577b34da6a3ce929d0e0e4736"

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	req.Header.Set("traceparent", "00-"+parentTraceID+"-00f067aa0ba902b7-01")
	h.handler.ServeHTTP(httptest.NewRecorder(), req)

	ended := h.spans.Ended()
	if len(ended) != 1 {
		t.Fatalf("expected 1 span, got %d", len(ended))
	}
	if got := ended[0].SpanContext().TraceID().String(); got != parentTraceID {
		t.Fatalf("trace id = %s, want the caller's %s", got, parentTraceID)
	}
}

func hasAttribute(attrs []attribute.KeyValue, key, value string) bool {
	for _, a := range attrs {
		if string(a.Key) == key && a.Value.AsString() == value {
			return true
		}
	}
	return false
}
