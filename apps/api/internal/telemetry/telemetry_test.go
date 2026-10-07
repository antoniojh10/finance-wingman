package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func envFrom(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestEnabled(t *testing.T) {
	tests := map[string]struct {
		env  map[string]string
		want bool
	}{
		"no endpoint":         {env: nil, want: false},
		"shared endpoint":     {env: map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "https://otlp.example"}, want: true},
		"traces endpoint":     {env: map[string]string{"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "https://otlp.example/v1/traces"}, want: true},
		"metrics endpoint":    {env: map[string]string{"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT": "https://otlp.example/v1/metrics"}, want: true},
		"explicitly disabled": {env: map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "https://otlp.example", "OTEL_SDK_DISABLED": "TRUE"}, want: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := Enabled(envFrom(tc.env)); got != tc.want {
				t.Fatalf("Enabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

// Setup changes the process-wide providers, so these tests restore them and
// don't run in parallel.
func restoreGlobals(t *testing.T) {
	tp, mp, prop := otel.GetTracerProvider(), otel.GetMeterProvider(), otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(tp)
		otel.SetMeterProvider(mp)
		otel.SetTextMapPropagator(prop)
	})
}

func TestSetupDisabledKeepsNoopProviders(t *testing.T) {
	restoreGlobals(t)
	shutdown, err := Setup(context.Background(), envFrom(nil), "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); ok {
		t.Fatal("tracer provider installed without an endpoint")
	}
	if fields := otel.GetTextMapPropagator().Fields(); len(fields) == 0 {
		t.Fatal("trace context propagator not installed")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSetupExportsToOTLPEndpoint(t *testing.T) {
	restoreGlobals(t)
	var (
		mu    sync.Mutex
		paths = map[string]int{}
	)
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths[r.URL.Path]++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer collector.Close()

	// The SDK exporters read the endpoint from the process environment.
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", collector.URL)
	shutdown, err := Setup(context.Background(), os.Getenv, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); !ok {
		t.Fatalf("expected an SDK tracer provider, got %T", otel.GetTracerProvider())
	}

	_, span := otel.Tracer("test").Start(context.Background(), "work")
	span.End()
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if paths["/v1/traces"] == 0 || paths["/v1/metrics"] == 0 {
		t.Fatalf("expected traces and metrics to be exported, got %v", paths)
	}
}
