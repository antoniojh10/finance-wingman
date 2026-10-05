// Package mcpserver exposes the finance service as MCP tools so assistants
// such as Claude or ChatGPT can record transactions and read summaries.
package mcpserver

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

const instructions = `Finance Wingman tracks a shared household workspace: accounts in different currencies, categorized expenses and income, and transfers between accounts.

Guidelines:
- Amounts are decimal numbers in the account's currency (e.g. 150.50). Never convert currencies yourself.
- Call list_accounts and list_categories when unsure which account or category the user means. Prefer existing categories; do not invent names.
- If the user does not name an account and several exist, ask which one to use.
- Dates use YYYY-MM-DD and default to today.
- Summaries are per currency; never add amounts from different currencies together.`

type Server struct {
	finance *finance.Service
	mcp     *mcp.Server
}

func New(fin *finance.Service, version string) *Server {
	s := &Server{
		finance: fin,
		mcp: mcp.NewServer(&mcp.Implementation{
			Name:    "finance-wingman",
			Title:   "Finance Wingman",
			Version: version,
		}, &mcp.ServerOptions{Instructions: instructions}),
	}
	s.registerTools()
	return s
}

// Handler serves the MCP Streamable HTTP transport, protected by bearer
// tokens issued by the OAuth server (or regular API sessions). publicURL is
// the API's public origin (PUBLIC_URL), accepted as Host by the DNS
// rebinding check.
func (s *Server) Handler(authSvc *auth.Service, publicURL, resourceMetadataURL string, logger *slog.Logger) http.Handler {
	streamable := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s.mcp }, &mcp.StreamableHTTPOptions{
		Stateless: true,
		Logger:    logger,
		// Replaced by hostGuard, which also accepts the public host.
		DisableLocalhostProtection: true,
	})
	verifier := func(ctx context.Context, token string, _ *http.Request) (*mcpauth.TokenInfo, error) {
		session, err := authSvc.Authenticate(ctx, token)
		if errors.Is(err, auth.ErrUnauthenticated) {
			return nil, mcpauth.ErrInvalidToken
		}
		if err != nil {
			return nil, err
		}
		return &mcpauth.TokenInfo{
			UserID:     session.User.ID.String(),
			Expiration: session.ExpiresAt,
			Scopes:     []string{"finance"},
		}, nil
	}
	return hostGuard(publicURL, mcpauth.RequireBearerToken(verifier, &mcpauth.RequireBearerTokenOptions{
		ResourceMetadataURL: resourceMetadataURL,
	})(streamable))
}

// hostGuard is the SDK's DNS rebinding protection (on a loopback listener,
// reject Host headers that are not loopback) extended to accept the public
// host. Tunnels such as Tailscale Funnel forward public traffic to the
// loopback listener with that Host, which the SDK check would reject.
func hostGuard(publicURL string, next http.Handler) http.Handler {
	publicHost := ""
	if u, err := url.Parse(publicURL); err == nil {
		publicHost = u.Host
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		localAddr, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
		if ok && localAddr != nil && isLoopback(localAddr.String()) &&
			!isLoopback(r.Host) && !strings.EqualFold(r.Host, publicHost) {
			http.Error(w, "Forbidden: invalid Host header", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isLoopback reports whether a host or host:port names the loopback
// interface.
func isLoopback(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// MCP exposes the underlying server, e.g. for in-memory tests.
func (s *Server) MCP() *mcp.Server { return s.mcp }

// actorContext attributes writes to the user behind the bearer token.
func actorContext(ctx context.Context, req *mcp.CallToolRequest) context.Context {
	if req == nil || req.Extra == nil || req.Extra.TokenInfo == nil {
		return ctx
	}
	if id, err := uuid.Parse(req.Extra.TokenInfo.UserID); err == nil {
		return finance.WithActor(ctx, id)
	}
	return ctx
}
