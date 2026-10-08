package httpapi

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/mail"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/mcpserver"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/oauth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/telemetry"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

// logCollector keeps everything the OTLP log exporter would receive.
type logCollector struct {
	mu   sync.Mutex
	dump strings.Builder
}

func (c *logCollector) Export(_ context.Context, records []sdklog.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, r := range records {
		fmt.Fprintln(&c.dump, r.Body().String())
		r.WalkAttributes(func(kv attribute.KeyValue) bool {
			fmt.Fprintln(&c.dump, kv.Key, kv.Value.Emit())
			return true
		})
	}
	return nil
}

func (c *logCollector) Shutdown(context.Context) error   { return nil }
func (c *logCollector) ForceFlush(context.Context) error { return nil }

// flakyMail records messages and, when failing, returns an error that echoes
// the recipient, as SMTP servers and mail APIs do.
type flakyMail struct {
	*testutil.MailRecorder
	fail atomic.Bool
}

func (m *flakyMail) Send(ctx context.Context, msg mail.Message) error {
	_ = m.MailRecorder.Send(ctx, msg)
	if m.fail.Load() {
		return fmt.Errorf("smtp: 550 5.1.1 <%s>: recipient rejected", msg.To)
	}
	return nil
}

// TestTelemetryCarriesNoPersonalData drives REST, OAuth and MCP requests that
// contain known sensitive values and asserts none of them reaches the spans
// or the logs that are exported over OTLP.
func TestTelemetryCarriesNoPersonalData(t *testing.T) {
	// Not parallel: it swaps the global providers, which the database pool
	// and the MCP server read when they are built.
	spans := tracetest.NewSpanRecorder()
	logs := &logCollector{}
	logProvider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(logs)))
	previousTracer, previousLogger := otel.GetTracerProvider(), global.GetLoggerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)))
	global.SetLoggerProvider(logProvider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previousTracer)
		global.SetLoggerProvider(previousLogger)
		_ = logProvider.Shutdown(context.Background())
	})

	api := newTestAPI(t)
	logger := slog.New(telemetry.LogHandler(slog.NewJSONHandler(io.Discard, nil), slog.LevelInfo))
	sender := &flakyMail{MailRecorder: api.mail}
	authSvc := auth.NewService(api.pool, sender, auth.Config{WebBaseURL: "http://web.test"}, logger)
	svc := finance.NewService(api.pool, time.UTC)
	oauthSrv := oauth.NewServer(api.pool, authSvc, &testutil.MailRecorder{}, oauth.Config{Issuer: testIssuer}, logger)
	workspaces := workspace.NewService(api.pool, sender, workspace.Config{WebBaseURL: "http://web.test"})
	mcpHandler := mcpserver.New(svc, Version).Handler(authSvc, testIssuer, oauthSrv.ResourceMetadataURL(), logger)
	api.handler = NewHandler(Deps{Logger: logger, DB: api.pool, Auth: authSvc, Finance: svc, Workspaces: workspaces, OAuth: oauthSrv, MCP: mcpHandler})
	api.mail = sender.MailRecorder

	const (
		accountName   = "ZZ-secret-account"
		categoryName  = "ZZ-secret-category"
		description   = "ZZ-secret-description"
		searchTerm    = "ZZ-secret-search"
		amountDecimal = "87654.32"
		amountMinor   = "8765432"
		state         = "ZZ-secret-state"
		email         = "owner@example.com"
		unknownEmail  = "zz-stranger@example.org"
	)
	sensitive := []string{accountName, categoryName, description, searchTerm, amountDecimal, amountMinor, state, email, unknownEmail, api.token}

	// REST: finance data in bodies, search terms in the query string, a
	// conflict, and a validation error.
	account := api.createAccount(accountName, "MXN", 0)
	category := api.createCategory(categoryName, "expense")
	api.createTransaction(map[string]any{
		"type": "expense", "account_id": account.ID, "category_id": category.ID,
		"amount": 8765432, "description": description, "occurred_on": "2026-01-15",
	})
	api.do(http.MethodGet, "/api/v1/transactions?q="+searchTerm, nil).expect(http.StatusOK)
	api.do(http.MethodPost, "/api/v1/accounts", map[string]any{"name": accountName, "type": "checking", "currency": "MXN"}).expect(http.StatusConflict)
	api.do(http.MethodPost, "/api/v1/accounts", `{"name": 5}`).expect(http.StatusUnprocessableEntity)

	// Login: a sender failure that echoes the recipient is logged as an error.
	sender.fail.Store(true)
	api.as("").do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": email}).expect(http.StatusInternalServerError)
	api.as("").do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": unknownEmail}).expect(http.StatusAccepted)
	sender.fail.Store(false)
	api.do(http.MethodPost, "/api/v1/auth/login", map[string]any{"email": email})
	magicCode := api.mail.LastCode(api.t)
	token, _ := api.mail.LastLogin(api.t)
	api.as("").do(http.MethodPost, "/api/v1/auth/verify", map[string]any{"email": email, "code": magicCode}).expect(http.StatusOK)
	sensitive = append(sensitive, magicCode, token)

	// OAuth with secrets in the query string, then MCP tool calls.
	client := newOAuthClient(t, api, "none")
	api.raw(httptest.NewRequest(http.MethodGet, client.authorizeURL(map[string]string{"state": state}), nil))
	code := client.authorize(t, email)
	tokens := client.exchange(code)
	sensitive = append(sensitive, client.challenge(), client.verifier, code, tokens.AccessToken, tokens.RefreshToken)
	client.refresh(tokens.RefreshToken)

	server := httptest.NewServer(api.handler)
	defer server.Close()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "claude-test"}, nil).Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint:   server.URL + "/mcp",
		HTTPClient: &http.Client{Transport: bearerTransport{token: tokens.AccessToken, base: http.DefaultTransport}},
	}, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()
	for _, call := range []mcp.CallToolParams{
		{Name: "add_expense", Arguments: map[string]any{"amount": 87654.32, "account": accountName, "category": categoryName, "description": description}},
		{Name: "add_expense", Arguments: map[string]any{"amount": 87654.32, "account": "ZZ-unknown-account", "description": description}},
		{Name: "list_transactions", Arguments: map[string]any{"query": searchTerm}},
		{Name: "add_expense", Arguments: map[string]any{"amount": "ZZ-not-a-number", "description": description}},
	} {
		if _, err := session.CallTool(context.Background(), &call); err != nil {
			t.Logf("tool %s: protocol error: %v", call.Name, err)
		}
	}
	sensitive = append(sensitive, "ZZ-unknown-account", "ZZ-not-a-number")

	var exported strings.Builder
	for _, s := range spans.Ended() {
		fmt.Fprintln(&exported, s.Name(), s.Status(), s.Attributes())
		for _, e := range s.Events() {
			fmt.Fprintln(&exported, e.Name, e.Attributes)
		}
	}
	if exported.Len() == 0 {
		t.Fatal("no spans were recorded")
	}
	spanDump := exported.String()
	for _, name := range []string{"POST /api/v1/accounts", "tools/call add_expense", "CreateAccount"} {
		if !strings.Contains(spanDump, name) {
			t.Fatalf("expected a %q span, got:\n%s", name, spanDump)
		}
	}
	if err := logProvider.ForceFlush(context.Background()); err != nil {
		t.Fatal(err)
	}
	logs.mu.Lock()
	logDump := logs.dump.String()
	logs.mu.Unlock()
	if !strings.Contains(logDump, "request login") {
		t.Fatalf("expected the failed login to be logged, got:\n%s", logDump)
	}

	for _, value := range sensitive {
		if value == "" {
			continue
		}
		if strings.Contains(spanDump, value) {
			t.Errorf("spans leak %q:\n%s", value, spanDump)
		}
		if strings.Contains(logDump, value) {
			t.Errorf("logs leak %q:\n%s", value, logDump)
		}
	}
}
