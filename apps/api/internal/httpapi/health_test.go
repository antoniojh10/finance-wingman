package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func newTestHandler(db Pinger) http.Handler {
	return NewHandler(Deps{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		DB:     db,
	})
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHealthOK(t *testing.T) {
	rec := get(t, newTestHandler(fakePinger{}), "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body)
	}
	var body struct{ Status, Database string }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "ok" || body.Database != "ok" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestHealthDatabaseDown(t *testing.T) {
	rec := get(t, newTestHandler(fakePinger{err: errors.New("connection refused")}), "/healthz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body)
	}
}

func TestSecurityHeaders(t *testing.T) {
	rec := get(t, newTestHandler(fakePinger{}), "/healthz")
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected nosniff, got %q", got)
	}
	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("HSTS must be off by default, got %q", got)
	}

	h := NewHandler(Deps{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), DB: fakePinger{}, HSTS: true})
	rec = get(t, h, "/healthz")
	if got := rec.Header().Get("Strict-Transport-Security"); got != "max-age=63072000; includeSubDomains" {
		t.Fatalf("unexpected HSTS header %q", got)
	}
}

func TestHealthWithRealDatabase(t *testing.T) {
	pool := testutil.NewDatabase(t, false)
	rec := get(t, newTestHandler(pool), "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body)
	}
}

func TestOpenAPISpecIsServed(t *testing.T) {
	rec := get(t, newTestHandler(fakePinger{}), "/openapi.json")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var spec struct {
		Paths map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	if _, ok := spec.Paths["/healthz"]; !ok {
		t.Fatal("expected /healthz in the OpenAPI spec")
	}
}

func TestOpenAPIDocumentWithoutDatabase(t *testing.T) {
	spec, err := OpenAPI(Deps{
		Logger:  slog.New(slog.DiscardHandler),
		Auth:    &auth.Service{},
		Finance: &finance.Service{},
	})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(spec, &doc); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/transactions", "/api/v1/auth/verify", "/api/v1/summary"} {
		if _, ok := doc.Paths[path]; !ok {
			t.Errorf("missing %s in the OpenAPI document", path)
		}
	}
}
