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

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

const (
	apiTitle   = "Finance Wingman API"
	apiVersion = "0.1.0"
)

// Pinger reports whether the database is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

type Deps struct {
	Logger  *slog.Logger
	DB      Pinger
	Finance *finance.Service
}

// NewHandler builds the root HTTP handler with every route registered.
func NewHandler(deps Deps) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(requestLogger(deps.Logger))
	router.Use(middleware.Recoverer)

	config := huma.DefaultConfig(apiTitle, apiVersion)
	api := humachi.New(router, config)

	registerHealth(api, deps.DB)
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
