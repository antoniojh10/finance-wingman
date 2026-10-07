package telemetry

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// memoryExporter keeps exported log records for assertions.
type memoryExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *memoryExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, r := range records {
		e.records = append(e.records, r.Clone())
	}
	return nil
}

func (e *memoryExporter) Shutdown(context.Context) error   { return nil }
func (e *memoryExporter) ForceFlush(context.Context) error { return nil }

func (e *memoryExporter) bodies() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, len(e.records))
	for i, r := range e.records {
		out[i] = r.Body().AsString()
	}
	return out
}

func newTestLogger(t *testing.T, level slog.Level) (*slog.Logger, *bytes.Buffer, *memoryExporter) {
	t.Helper()
	exporter := &memoryExporter{}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	stdout := &bytes.Buffer{}
	local := slog.NewJSONHandler(stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(logHandlerWithProvider(local, level, provider)), stdout, exporter
}

func TestLogHandlerWritesLocallyAndExports(t *testing.T) {
	logger, stdout, exporter := newTestLogger(t, slog.LevelInfo)

	logger.Info("request", "status", 200)

	if !strings.Contains(stdout.String(), `"msg":"request"`) {
		t.Fatalf("record not written locally: %s", stdout)
	}
	if got := exporter.bodies(); len(got) != 1 || got[0] != "request" {
		t.Fatalf("exported records = %v, want [request]", got)
	}
	if got := exporter.records[0].Severity(); got != log.SeverityInfo {
		t.Fatalf("severity = %v, want info", got)
	}
}

func TestLogHandlerKeepsDebugLocal(t *testing.T) {
	logger, stdout, exporter := newTestLogger(t, slog.LevelInfo)

	logger.Debug("retry noise")

	if !strings.Contains(stdout.String(), "retry noise") {
		t.Fatalf("debug record should still reach the local handler: %s", stdout)
	}
	if got := exporter.bodies(); len(got) != 0 {
		t.Fatalf("debug record exported: %v", got)
	}
}

func TestLogHandlerPropagatesAttrsAndGroups(t *testing.T) {
	logger, stdout, exporter := newTestLogger(t, slog.LevelInfo)

	logger.With("request_id", "abc").WithGroup("db").Info("query", "rows", 3)

	if !strings.Contains(stdout.String(), `"request_id":"abc","db":{"rows":3}`) {
		t.Fatalf("local record missing attrs or group: %s", stdout)
	}
	attrs := map[string]string{}
	exporter.records[0].WalkAttributes(func(kv attribute.KeyValue) bool {
		attrs[string(kv.Key)] = kv.Value.Emit()
		return true
	})
	if attrs["request_id"] != "abc" {
		t.Fatalf("exported record missing request_id: %v", attrs)
	}
	if _, ok := attrs["db"]; !ok {
		t.Fatalf("exported record missing db group: %v", attrs)
	}
}

func TestLogHandlerCarriesTraceContext(t *testing.T) {
	logger, _, exporter := newTestLogger(t, slog.LevelInfo)
	tracer := sdktrace.NewTracerProvider().Tracer("test")
	ctx, span := tracer.Start(context.Background(), "request")
	defer span.End()

	logger.InfoContext(ctx, "inside span")

	r := exporter.records[0]
	if r.TraceID() != span.SpanContext().TraceID() || r.SpanID() != span.SpanContext().SpanID() {
		t.Fatalf("record trace/span = %s/%s, want %s/%s", r.TraceID(), r.SpanID(), span.SpanContext().TraceID(), span.SpanContext().SpanID())
	}
}
