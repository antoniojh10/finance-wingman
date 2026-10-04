package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

type HealthOutput struct {
	Body struct {
		Status   string `json:"status" example:"ok" doc:"Overall service status"`
		Database string `json:"database" example:"ok" doc:"Database connectivity status"`
	}
}

func registerHealth(api huma.API, db Pinger) {
	huma.Register(api, huma.Operation{
		OperationID: "get-health",
		Method:      http.MethodGet,
		Path:        "/healthz",
		Summary:     "Health check",
		Description: "Reports whether the API and its database are reachable.",
		Tags:        []string{"System"},
		Security:    []map[string][]string{},
		Metadata:    publicMetadata,
	}, func(ctx context.Context, _ *struct{}) (*HealthOutput, error) {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			return nil, huma.Error503ServiceUnavailable("database unavailable")
		}
		out := &HealthOutput{}
		out.Body.Status = "ok"
		out.Body.Database = "ok"
		return out, nil
	})
}
