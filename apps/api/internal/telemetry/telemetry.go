// Package telemetry configures OpenTelemetry traces, metrics and logs.
//
// Export is opt-in: it is enabled only when an OTLP endpoint is configured
// through the standard environment variables (OTEL_EXPORTER_OTLP_ENDPOINT or
// the per-signal OTEL_EXPORTER_OTLP_{TRACES,METRICS,LOGS}_ENDPOINT). Headers,
// protocol details and the service name (OTEL_SERVICE_NAME) are read by the
// SDK from the environment as well. Without an endpoint the global providers
// stay no-ops, so instrumented code costs almost nothing.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// DefaultServiceName is reported when OTEL_SERVICE_NAME is not set.
const DefaultServiceName = "finance-api"

// Shutdown flushes pending telemetry and releases exporters.
type Shutdown func(context.Context) error

// Enabled reports whether the environment configures an OTLP endpoint and the
// SDK is not explicitly disabled.
func Enabled(getenv func(string) string) bool {
	if strings.EqualFold(getenv("OTEL_SDK_DISABLED"), "true") {
		return false
	}
	for _, key := range []string{"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"} {
		if getenv(key) != "" {
			return true
		}
	}
	return false
}

// Setup installs the global tracer, meter and logger providers, the W3C
// trace context propagator, and Go runtime metrics. Logs reach the logger
// provider only through a handler built with [LogHandler]. When telemetry is not enabled
// it only installs the propagator and returns a no-op Shutdown.
func Setup(ctx context.Context, getenv func(string) string, version string) (Shutdown, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	if !Enabled(getenv) {
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		semconv.ServiceVersion(version),
	))
	if err != nil {
		return nil, fmt.Errorf("telemetry resource: %w", err)
	}
	if getenv("OTEL_SERVICE_NAME") == "" {
		res, err = resource.Merge(res, resource.NewSchemaless(semconv.ServiceName(DefaultServiceName)))
		if err != nil {
			return nil, fmt.Errorf("telemetry resource: %w", err)
		}
	}

	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("trace exporter: %w", err)
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("metric exporter: %w", err), tracerProvider.Shutdown(ctx))
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)

	logExporter, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("log exporter: %w", err), tracerProvider.Shutdown(ctx), meterProvider.Shutdown(ctx))
	}
	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	global.SetLoggerProvider(loggerProvider)

	shutdown := func(ctx context.Context) error {
		return errors.Join(tracerProvider.Shutdown(ctx), meterProvider.Shutdown(ctx), loggerProvider.Shutdown(ctx))
	}
	if err := runtime.Start(runtime.WithMeterProvider(meterProvider)); err != nil {
		return nil, errors.Join(fmt.Errorf("runtime metrics: %w", err), shutdown(ctx))
	}
	return shutdown, nil
}
