// Package oauth implements the OAuth 2.1 authorization server used by MCP
// hosts (Claude, ChatGPT, ...) to obtain access tokens on behalf of a user.
//
// Supported: authorization server metadata (RFC 8414), protected resource
// metadata (RFC 9728), dynamic client registration (RFC 7591), authorization
// code grant with mandatory PKCE (S256), rotating refresh tokens, and token
// revocation (RFC 7009). Users authenticate with an emailed one-time code.
package oauth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

const (
	// Scope is the single scope granted to OAuth clients.
	Scope = "finance"
	// SessionClient labels sessions created through OAuth.
	SessionClient = "mcp"
)

type Config struct {
	// Issuer is the public base URL of the API, e.g. https://api.example.com.
	Issuer string
	// ResourcePath is the path of the protected MCP endpoint.
	ResourcePath string

	AccessTokenTTL          time.Duration
	RefreshTokenTTL         time.Duration
	AuthorizationCodeTTL    time.Duration
	AuthorizationRequestTTL time.Duration
	MaxClientsPerHour       int
}

func (c Config) withDefaults() Config {
	c.Issuer = strings.TrimRight(c.Issuer, "/")
	if c.ResourcePath == "" {
		c.ResourcePath = "/mcp"
	}
	if c.AccessTokenTTL == 0 {
		c.AccessTokenTTL = time.Hour
	}
	if c.RefreshTokenTTL == 0 {
		c.RefreshTokenTTL = 90 * 24 * time.Hour
	}
	if c.AuthorizationCodeTTL == 0 {
		c.AuthorizationCodeTTL = 5 * time.Minute
	}
	if c.AuthorizationRequestTTL == 0 {
		c.AuthorizationRequestTTL = 20 * time.Minute
	}
	if c.MaxClientsPerHour == 0 {
		c.MaxClientsPerHour = 50
	}
	return c
}

type Server struct {
	pool   *pgxpool.Pool
	q      *store.Queries
	auth   *auth.Service
	cfg    Config
	logger *slog.Logger
	now    func() time.Time
}

func NewServer(pool *pgxpool.Pool, authSvc *auth.Service, cfg Config, logger *slog.Logger) *Server {
	return &Server{pool: pool, q: store.New(pool), auth: authSvc, cfg: cfg.withDefaults(), logger: logger, now: time.Now}
}

// SetClock overrides the time source; intended for tests.
func (s *Server) SetClock(now func() time.Time) { s.now = now }

// ResourceURL is the canonical URL of the protected MCP endpoint.
func (s *Server) ResourceURL() string { return s.cfg.Issuer + s.cfg.ResourcePath }

// ResourceMetadataURL is advertised in WWW-Authenticate challenges.
func (s *Server) ResourceMetadataURL() string {
	return s.cfg.Issuer + "/.well-known/oauth-protected-resource" + s.cfg.ResourcePath
}

// Mount registers the OAuth endpoints and metadata documents.
func (s *Server) Mount(r chi.Router) {
	resourceMeta := mcpauth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               s.ResourceURL(),
		AuthorizationServers:   []string{s.cfg.Issuer},
		ScopesSupported:        []string{Scope},
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "Finance Wingman",
	})
	// Advertised only at the path-suffixed URL (RFC 9728), so a connector
	// saved with the bare API URL fails at discovery instead of after login.
	r.Handle("/.well-known/oauth-protected-resource"+s.cfg.ResourcePath, resourceMeta)

	r.Group(func(r chi.Router) {
		r.Use(allowCORS)
		r.Get("/.well-known/oauth-authorization-server", s.handleAuthServerMetadata)
		r.Options("/.well-known/oauth-authorization-server", noContent)
		r.Post("/oauth/register", s.handleRegister)
		r.Options("/oauth/register", noContent)
		r.Post("/oauth/token", s.handleToken)
		r.Options("/oauth/token", noContent)
		r.Post("/oauth/revoke", s.handleRevoke)
		r.Options("/oauth/revoke", noContent)
	})

	r.Get("/oauth/authorize", s.handleAuthorize)
	r.Post("/oauth/authorize", s.handleAuthorizeSubmit)
}

func (s *Server) handleAuthServerMetadata(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"issuer":                                         s.cfg.Issuer,
		"authorization_endpoint":                         s.cfg.Issuer + "/oauth/authorize",
		"token_endpoint":                                 s.cfg.Issuer + "/oauth/token",
		"registration_endpoint":                          s.cfg.Issuer + "/oauth/register",
		"revocation_endpoint":                            s.cfg.Issuer + "/oauth/revoke",
		"scopes_supported":                               []string{Scope},
		"response_types_supported":                       []string{"code"},
		"response_modes_supported":                       []string{"query"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"token_endpoint_auth_methods_supported":          []string{"none", "client_secret_post", "client_secret_basic"},
		"revocation_endpoint_auth_methods_supported":     []string{"none", "client_secret_post", "client_secret_basic"},
		"code_challenge_methods_supported":               []string{"S256"},
		"authorization_response_iss_parameter_supported": true,
		"ui_locales_supported":                           []string{"en", "es"},
	})
}

// allowCORS lets browser-based MCP clients reach the token-related
// endpoints. These endpoints never rely on cookies, so a wildcard origin
// does not expose ambient credentials.
func allowCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Mcp-Protocol-Version")
		next.ServeHTTP(w, r)
	})
}

func noContent(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// oauthError writes an RFC 6749 error response.
func oauthError(w http.ResponseWriter, status int, code, description string) {
	body := map[string]string{"error": code}
	if description != "" {
		body["error_description"] = description
	}
	writeJSON(w, status, body)
}
