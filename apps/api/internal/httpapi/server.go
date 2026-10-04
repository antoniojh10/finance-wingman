// Package httpapi wires the HTTP router, the OpenAPI definition, and the
// REST endpoints.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/oauth"
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
	// OAuth and MCP are optional; when set, the OAuth endpoints and the MCP
	// transport (at /mcp) are mounted.
	OAuth *oauth.Server
	MCP   http.Handler
}

// NewHandler builds the root HTTP handler with every route registered.
func NewHandler(deps Deps) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(requestLogger(deps.Logger))
	router.Use(middleware.Recoverer)

	if deps.OAuth != nil {
		deps.OAuth.Mount(router)
	}
	if deps.MCP != nil {
		router.Handle("/mcp", deps.MCP)
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
	if deps.Finance != nil {
		registerFinance(api, deps.Finance, deps.Logger)
	}

	return router
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}
