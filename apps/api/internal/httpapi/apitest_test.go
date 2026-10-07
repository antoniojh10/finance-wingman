package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/mcpserver"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/oauth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

// testAPI drives the full HTTP stack against an isolated database. Requests
// are authenticated as owner, acting on the owner's workspace, unless token
// is changed.
type testAPI struct {
	t          *testing.T
	handler    http.Handler
	svc        *finance.Service
	auth       *auth.Service
	oauth      *oauth.Server
	workspaces *workspace.Service
	mail       *testutil.MailRecorder
	pool       *pgxpool.Pool
	owner      auth.User
	token      string
	// workspace is the owner's workspace; ctx acts on it, for calling the
	// services directly.
	workspace workspace.Workspace
	ctx       context.Context
}

func newTestAPI(t *testing.T) *testAPI {
	t.Helper()
	pool := testutil.NewDatabase(t, true)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	recorder := &testutil.MailRecorder{}
	authSvc := auth.NewService(pool, recorder, auth.Config{WebBaseURL: "http://web.test"}, logger)
	svc := finance.NewService(pool, time.UTC)
	oauthSrv := oauth.NewServer(pool, authSvc, oauth.Config{Issuer: testIssuer}, logger)
	mcpHandler := mcpserver.New(svc, Version).Handler(authSvc, testIssuer, oauthSrv.ResourceMetadataURL(), logger)

	ctx := context.Background()
	owner, err := authSvc.AddUser(ctx, "owner@example.com", "Owner")
	if err != nil {
		t.Fatal(err)
	}
	workspaces := workspace.NewService(pool, recorder, workspace.Config{WebBaseURL: "http://web.test"})
	home, err := workspaces.Create(ctx, owner.ID, "Home")
	if err != nil {
		t.Fatal(err)
	}
	session, err := authSvc.CreateSession(ctx, owner.ID, auth.ClientWeb, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	return &testAPI{
		t:          t,
		svc:        svc,
		auth:       authSvc,
		oauth:      oauthSrv,
		workspaces: workspaces,
		mail:       recorder,
		pool:       pool,
		owner:      owner,
		token:      session.Token,
		workspace:  home,
		ctx:        db.WithWorkspace(ctx, home.ID),
		handler: NewHandler(Deps{
			Logger:     logger,
			DB:         pool,
			Auth:       authSvc,
			Finance:    svc,
			Workspaces: workspaces,
			OAuth:      oauthSrv,
			MCP:        mcpHandler,
		}),
	}
}

// as returns a copy of the API client that sends the given bearer token
// (empty for anonymous requests).
func (a *testAPI) as(token string) *testAPI {
	c := *a
	c.token = token
	return &c
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
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
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
		// Anchored far in the past so fixture transactions with fixed dates all count.
		"balance_as_of": "2000-01-01",
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

const testIssuer = "http://api.test"

const missingID = "00000000-0000-0000-0000-000000000000"

// newUser adds a user with their own workspace and returns a client signed
// in as them, acting on that workspace.
func (a *testAPI) newUser(email, workspaceName string) *testAPI {
	a.t.Helper()
	ctx := context.Background()
	user, err := a.auth.AddUser(ctx, email, "")
	if err != nil {
		a.t.Fatal(err)
	}
	w, err := a.workspaces.Create(ctx, user.ID, workspaceName)
	if err != nil {
		a.t.Fatal(err)
	}
	session, err := a.auth.CreateSession(ctx, user.ID, auth.ClientWeb, time.Hour)
	if err != nil {
		a.t.Fatal(err)
	}
	c := a.as(session.Token)
	c.workspace = w
	c.ctx = db.WithWorkspace(ctx, w.ID)
	return c
}
