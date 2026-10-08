// Package httpapi wires the HTTP router, the OpenAPI definition, and the
// REST endpoints.
package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/oauth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

const (
	apiTitle = "Finance Wingman API"
	// Version is reported in the OpenAPI document and to MCP clients.
	Version = "0.1.0"
)

// Pinger reports whether the database is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

type Deps struct {
	Logger  *slog.Logger
	DB      Pinger
	Auth    *auth.Service
	Finance *finance.Service
	// Workspaces is optional; when set, workspace, member and invitation
	// endpoints are registered.
	Workspaces *workspace.Service
	// OAuth and MCP are optional; when set, the OAuth endpoints and the MCP
	// transport (at /mcp) are mounted.
	OAuth *oauth.Server
	MCP   http.Handler
	// TracerProvider and MeterProvider are optional; nil uses the global
	// OpenTelemetry providers.
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
	// HSTS adds Strict-Transport-Security; enable it only when the API is
	// served over HTTPS.
	HSTS bool
}

// NewHandler builds the root HTTP handler with every route registered.
func NewHandler(deps Deps) http.Handler {
	router, _ := build(deps)
	return router
}

// OpenAPI returns the indented JSON OpenAPI document for the REST API.
func OpenAPI(deps Deps) ([]byte, error) {
	_, api := build(deps)
	return json.MarshalIndent(api.OpenAPI(), "", "  ")
}

func init() {
	// Lists are always encoded as [] rather than null.
	huma.DefaultArrayNullable = false
}

func build(deps Deps) (chi.Router, huma.API) {
	router := chi.NewRouter()
	router.Use(instrument(deps.TracerProvider, deps.MeterProvider))
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(requestLogger(deps.Logger))
	router.Use(middleware.Recoverer)
	router.Use(securityHeaders(deps.HSTS))

	if deps.OAuth != nil {
		deps.OAuth.Mount(router)
	}
	if deps.MCP != nil {
		router.Handle("/mcp", deps.MCP)
		if deps.OAuth != nil {
			router.HandleFunc("/", mcpHint(deps.OAuth.ResourceURL()))
		}
	}

	config := huma.DefaultConfig(apiTitle, Version)
	config.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		securityScheme: {Type: "http", Scheme: "bearer", Description: "Session token from POST /api/v1/auth/verify"},
	}
	config.Security = []map[string][]string{{securityScheme: {}}}
	api := humachi.New(router, config)

	registerHealth(api, deps.DB)
	if deps.Auth != nil {
		api.UseMiddleware(authMiddleware(api, deps.Auth, deps.Logger))
		registerAuth(api, deps.Auth, deps.Logger)
	}
	if deps.Workspaces != nil && deps.Auth != nil {
		registerWorkspaces(api, deps.Workspaces, deps.Auth, deps.Logger)
	}
	if deps.Finance != nil {
		scoped := huma.NewGroup(api)
		scoped.UseMiddleware(requireWorkspace(api))
		registerFinance(scoped, deps.Finance, deps.Logger)
	}

	return router, api
}

// mcpHint answers requests to the bare root with a pointer to the MCP
// endpoint, for connectors saved without the /mcp path.
func mcpHint(mcpURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "not_found",
			"message": "No MCP endpoint at this URL. The MCP endpoint is " + mcpURL,
		})
	}
}

// securityHeaders sets the headers every response should carry, including
// the non-HTML ones (JSON, SSE) where nosniff matters.
func securityHeaders(hsts bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			if hsts {
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			level := slog.LevelInfo
			if r.URL.Path == "/" && ww.Status() == http.StatusNotFound {
				// Connectors configured with the bare URL retry constantly.
				level = slog.LevelDebug
			}
			duration := time.Since(start).Milliseconds()
			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration_ms", duration,
				"request_id", middleware.GetReqID(r.Context()),
			}
			if sc := trace.SpanContextFromContext(r.Context()); sc.IsValid() {
				// Links the log line to its trace in the telemetry backend.
				attrs = append(attrs, "trace_id", sc.TraceID().String())
			}
			// The message summarizes the request so log lists are readable in
			// backends that show only the message (Loki); the attributes stay
			// for filtering.
			msg := fmt.Sprintf("%s %s %d %dms", r.Method, r.URL.Path, ww.Status(), duration)
			logger.Log(r.Context(), level, msg, attrs...)
		})
	}
}
