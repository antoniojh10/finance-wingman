package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
)

// testAPI drives the full HTTP stack against an isolated database.
type testAPI struct {
	t       *testing.T
	handler http.Handler
	svc     *finance.Service
	pool    *pgxpool.Pool
}

func newTestAPI(t *testing.T) *testAPI {
	t.Helper()
	pool := testutil.NewDatabase(t, true)
	svc := finance.NewService(pool, time.UTC)
	return &testAPI{
		t:    t,
		svc:  svc,
		pool: pool,
		handler: NewHandler(Deps{
			Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
			DB:      pool,
			Finance: svc,
		}),
	}
}

type response struct {
	t      *testing.T
	Status int
	Body   []byte
}

// do sends a request; body may be nil, a string (raw JSON), or any value
// that is encoded as JSON.
func (a *testAPI) do(method, path string, body any) response {
	a.t.Helper()
	var reader io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		reader = bytes.NewBufferString(b)
	default:
		encoded, err := json.Marshal(b)
		if err != nil {
			a.t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	}
	req := httptest.NewRequest(method, path, reader)
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	return response{t: a.t, Status: rec.Code, Body: rec.Body.Bytes()}
}

// expect asserts the status code and returns the response for decoding.
func (r response) expect(status int) response {
	r.t.Helper()
	if r.Status != status {
		r.t.Fatalf("expected status %d, got %d: %s", status, r.Status, r.Body)
	}
	return r
}

func (r response) decode(dest any) {
	r.t.Helper()
	if err := json.Unmarshal(r.Body, dest); err != nil {
		r.t.Fatalf("decode response: %v\n%s", err, r.Body)
	}
}

type errorBody struct {
	Status int    `json:"status"`
	Detail string `json:"detail"`
	Errors []struct {
		Location string `json:"location"`
		Message  string `json:"message"`
	} `json:"errors"`
}

// expectError asserts an error status and returns the problem details.
func (r response) expectError(status int) errorBody {
	r.t.Helper()
	r.expect(status)
	var body errorBody
	r.decode(&body)
	return body
}

// Fixture helpers.

func (a *testAPI) createAccount(name, currency string, initialBalance int64) finance.Account {
	a.t.Helper()
	var account finance.Account
	a.do(http.MethodPost, "/api/v1/accounts", map[string]any{
		"name": name, "type": "checking", "currency": currency, "initial_balance": initialBalance,
	}).expect(http.StatusCreated).decode(&account)
	return account
}

func (a *testAPI) createCategory(name, kind string) finance.Category {
	a.t.Helper()
	var category finance.Category
	a.do(http.MethodPost, "/api/v1/categories", map[string]any{"name": name, "kind": kind}).
		expect(http.StatusCreated).decode(&category)
	return category
}

func (a *testAPI) createTransaction(body map[string]any) finance.Transaction {
	a.t.Helper()
	var tx finance.Transaction
	a.do(http.MethodPost, "/api/v1/transactions", body).expect(http.StatusCreated).decode(&tx)
	return tx
}

func (a *testAPI) getAccount(id string) finance.Account {
	a.t.Helper()
	var account finance.Account
	a.do(http.MethodGet, "/api/v1/accounts/"+id, nil).expect(http.StatusOK).decode(&account)
	return account
}

const missingID = "00000000-0000-0000-0000-000000000000"
