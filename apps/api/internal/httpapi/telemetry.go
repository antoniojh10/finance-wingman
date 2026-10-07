package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.opentelemetry.io/otel/trace"
)

// instrument is a chi middleware that records OpenTelemetry server spans and
// metrics. Spans are named after the matched chi route
// ("GET /api/v1/accounts/{id}") so IDs in the path don't create one series
// per resource. Nil providers fall back to the global ones, which are no-ops
// unless telemetry is set up.
func instrument(tp trace.TracerProvider, mp metric.MeterProvider) func(http.Handler) http.Handler {
	opts := []otelhttp.Option{
		// Continue traces started by the web app (W3C traceparent).
		otelhttp.WithPropagators(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})),
		otelhttp.WithFilter(func(r *http.Request) bool { return r.URL.Path != "/healthz" }),
	}
	if tp != nil {
		opts = append(opts, otelhttp.WithTracerProvider(tp))
	}
	if mp != nil {
		opts = append(opts, otelhttp.WithMeterProvider(mp))
	}
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(routeNamer(next), "http.server", opts...)
	}
}

// routeNamer renames the active span and labels the request metrics with the
// chi route pattern once routing has resolved it.
func routeNamer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)

		route := ""
		if rctx := chi.RouteContext(r.Context()); rctx != nil {
			route = rctx.RoutePattern()
		}
		if route == "" {
			route = "unmatched"
		}
		span := trace.SpanFromContext(r.Context())
		span.SetName(r.Method + " " + route)
		span.SetAttributes(semconv.HTTPRoute(route))
		if labeler, ok := otelhttp.LabelerFromContext(r.Context()); ok {
			labeler.Add(attribute.String(string(semconv.HTTPRouteKey), route))
		}
	})
}
