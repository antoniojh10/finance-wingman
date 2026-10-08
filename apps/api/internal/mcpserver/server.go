// Package mcpserver exposes the finance service as MCP tools so assistants
// such as Claude or ChatGPT can record transactions and read summaries.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/ratelimit"
)

const instructions = `Finance Wingman tracks a shared household workspace: accounts in different currencies, categorized expenses and income, and transfers between accounts.

Guidelines:
- Amounts are decimal numbers in the account's currency (e.g. 150.50). Never convert currencies yourself.
- Call list_accounts and list_categories when unsure which account or category the user means. Prefer existing categories; do not invent names.
- Create accounts or categories (create_account, create_category) only when the user asks for them or confirms a new one is needed.
- Each account belongs to a workspace member or is shared (joint or household accounts). New accounts belong to the user you are helping unless they say otherwise (owner "shared" or another member's name). Members can each have an account with the same name, e.g. two "BNP" accounts: list_accounts shows them as BNP (Ana) and BNP (Luis); a bare "BNP" means the user's own, and another member's is named with the owner in parentheses. list_accounts, list_transactions and get_summary take an optional owner ("me", "shared" or a member) when the user asks about one person's money; otherwise cover the whole workspace.
- An account's initial_balance is the balance it holds on balance_as_of (default today), not a balance before all history: transactions dated on or before balance_as_of are already part of it, only later ones change the balance. So when the user states today's balance, create the account with it and then past transactions can be backfilled without altering the balance; for a balance on another date pass balance_as_of. Income and expense summaries still count backdated transactions.
- To fix or change an existing account use update_account (rename, change type, correct initial_balance or its balance_as_of date, archive or unarchive). The currency cannot be changed. Confirm with the user before changing a balance or archiving. Archived accounts are hidden from list_accounts and cannot receive new transactions, but update_account still finds them so they can be restored. There is no tool to delete accounts.
- To fix or change an existing category use update_category (rename, change color or icon, archive or unarchive). The kind (expense or income) cannot be changed. Confirm with the user before archiving. Archived categories are hidden from list_categories but their transactions stay categorized, and update_category still finds them so they can be restored.
- Recurring items (users may call them subscriptions: Netflix, rent, a phone installment plan, salary) are tracked with list_recurring, create_recurring, update_recurring, list_upcoming_recurring and mark_recurring_paid. list_recurring shows next due dates, the current period status, the last payment and the committed monthly cost per currency; amounts are estimates in the account's currency and create_recurring does not create transactions. Create one only when the user asks to track it (check list_recurring for duplicates; names are unique among non-cancelled items). Frequency is week, month or year plus interval_count (quarterly = month, 3). If the user does not name the account and several exist, ask which one. Ask for any missing amount or schedule instead of guessing. To pause, resume or cancel one use update_recurring with status (cancelled items are kept for history; ask before cancelling).
- When the user says they paid (or received) something that matches a recurring item ("I paid Netflix", "rent is paid"), use mark_recurring_paid instead of add_expense or add_income; pass amount only if it differs from the estimate. If several items could match, or none clearly does, ask the user which one. If mark_recurring_paid reports the period is already paid, ask the user whether to pay the next period in advance or record a second charge, then retry with an explicit period. For 'what is due this week?' or 'what is still pending?' use list_upcoming_recurring (days=7, 30, ...); pending and overdue mean unpaid. list_transactions shows which recurring item a transaction pays (recurring_name).
- Detected recurring patterns are available with list_recurring_suggestions. Mention them in moderation: only when the user asks about recurring items or subscriptions (e.g. alongside list_recurring), never interrupting other tasks. Suggest, never accept without the user's OK: call accept_recurring_suggestion (optional name, amount, category overrides; it links the matching past transactions) or dismiss_recurring_suggestion only after the user agrees.
- add_expense and add_income may return a recurring_match when the new transaction looks like a payment of an active recurring item. It is not linked automatically: offer it to the user and, if they agree, call link_transaction_to_recurring (optional period; unlink=true removes a link).
- For several records at once use the batch tools (create_accounts, create_categories, add_transactions): they are all-or-nothing, accept up to 100 items, and an error names the failing item so you can fix it and resend the whole batch.
- To fix or re-categorize existing transactions use update_transaction (one) or update_transactions (several, preferred for re-categorizing many: all-or-nothing, up to 100). Get the transaction ids from list_transactions first; pass only the fields to change, and clear_category=true to remove a category. The type cannot be changed (delete and re-add instead, after confirming with the user). Editing keeps the link to a recurring item (but a linked transaction cannot change account).
- If the user does not name an account and several exist, ask which one to use.
- There is no tool to export or download all the data. If the user wants a copy of their data, tell them to use "Export your data" (JSON or CSV) in the Settings page of the Finance Wingman web app.
- Dates use YYYY-MM-DD and default to today.
- Summaries are per currency; never add amounts from different currencies together.`

type Server struct {
	finance *finance.Service
	mcp     *mcp.Server
	// workspaceOf returns the workspace a tool call acts on; tests replace it
	// because in-memory transports carry no bearer token.
	workspaceOf func(*mcp.CallToolRequest) (uuid.UUID, bool)
	// actorOf returns the user behind a tool call, replaced by tests for the
	// same reason.
	actorOf func(*mcp.CallToolRequest) (uuid.UUID, bool)
	// limiter, when set, caps tool calls per user (see SetLimiter).
	limiter *ratelimit.Limiter
}

// tokenWorkspaceKey is the TokenInfo.Extra key holding the workspace of the
// bearer session.
const tokenWorkspaceKey = "workspace_id"

// TokenInfo.Extra keys holding the OAuth client behind the bearer session,
// recorded with the changes it makes.
const (
	tokenClientIDKey   = "oauth_client_id"
	tokenClientNameKey = "oauth_client_name"
)

const noWorkspaceMessage = "This connection is not linked to a workspace (the user may have left it). " +
	"Ask the user to disconnect and reconnect the Finance Wingman connector, and to pick a workspace when signing in."

func New(fin *finance.Service, version string) *Server {
	s := &Server{
		finance:     fin,
		workspaceOf: tokenWorkspace,
		actorOf:     tokenUser,
		mcp: mcp.NewServer(&mcp.Implementation{
			Name:    "finance-wingman",
			Title:   "Finance Wingman",
			Version: version,
		}, &mcp.ServerOptions{Instructions: instructions}),
	}
	// The global provider forwards to the SDK once telemetry is set up.
	s.mcp.AddReceivingMiddleware(tracing(otel.GetTracerProvider()), s.rateLimit, s.workspaceScope)
	s.registerTools()
	s.registerUpdateTransactionTools()
	s.registerRecurringTools()
	s.registerRecurringSuggestionTools()
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
		info := &mcpauth.TokenInfo{
			UserID:     session.User.ID.String(),
			Expiration: session.ExpiresAt,
			Scopes:     []string{"finance"},
		}
		info.Extra = map[string]any{}
		if session.Workspace != nil {
			info.Extra[tokenWorkspaceKey] = session.Workspace.ID.String()
		}
		if session.OAuthClient != nil {
			info.Extra[tokenClientIDKey] = session.OAuthClient.ID
			info.Extra[tokenClientNameKey] = session.OAuthClient.Name
		}
		return info, nil
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

// SetLimiter caps tool calls per user. Set it before serving requests.
func (s *Server) SetLimiter(l *ratelimit.Limiter) { s.limiter = l }

// rateLimit answers tool calls over the user's budget with a tool error that
// tells the model to wait, so it can relay the delay instead of retrying.
func (s *Server) rateLimit(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		call, ok := req.(*mcp.CallToolRequest)
		if !ok || s.limiter == nil {
			return next(ctx, method, req)
		}
		user, ok := s.actorOf(call)
		if !ok {
			return next(ctx, method, req)
		}
		if allowed, wait := s.limiter.Allow(user.String()); !allowed {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(
				"Rate limit reached: too many tool calls in a short time. Do not retry immediately; wait about %d seconds, "+
					"and tell the user if the task cannot continue. Prefer batch tools (add_transactions, create_accounts, "+
					"create_categories, update_transactions) over many single calls.", ratelimit.RetryAfterSeconds(wait))}}}, nil
		}
		return next(ctx, method, req)
	}
}

// workspaceScope makes every tool call act on the workspace of its bearer
// session, as its user, and refuses calls from sessions without one.
func (s *Server) workspaceScope(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		call, ok := req.(*mcp.CallToolRequest)
		if !ok {
			return next(ctx, method, req)
		}
		id, ok := s.workspaceOf(call)
		if !ok {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: noWorkspaceMessage}}}, nil
		}
		ctx = db.WithWorkspace(ctx, id)
		ctx = finance.WithChannel(ctx, tokenChannel(call))
		if user, ok := s.actorOf(call); ok {
			ctx = finance.WithActor(ctx, user)
		}
		return next(ctx, method, req)
	}
}

// tokenChannel is the channel of a tool call: always MCP, with the OAuth
// client when the token carries one.
func tokenChannel(req *mcp.CallToolRequest) finance.Channel {
	ch := finance.Channel{Kind: finance.ChannelMCP}
	if req.Extra != nil && req.Extra.TokenInfo != nil {
		ch.ClientID, _ = req.Extra.TokenInfo.Extra[tokenClientIDKey].(string)
		ch.ClientName, _ = req.Extra.TokenInfo.Extra[tokenClientNameKey].(string)
	}
	return ch
}

func tokenWorkspace(req *mcp.CallToolRequest) (uuid.UUID, bool) {
	if req.Extra == nil || req.Extra.TokenInfo == nil {
		return uuid.Nil, false
	}
	raw, _ := req.Extra.TokenInfo.Extra[tokenWorkspaceKey].(string)
	id, err := uuid.Parse(raw)
	return id, err == nil
}

// tokenUser returns the user behind the bearer token, so writes are
// attributed to them and "my" accounts can be told apart.
func tokenUser(req *mcp.CallToolRequest) (uuid.UUID, bool) {
	if req.Extra == nil || req.Extra.TokenInfo == nil {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(req.Extra.TokenInfo.UserID)
	return id, err == nil
}
