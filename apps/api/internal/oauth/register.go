package oauth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

type registrationRequest struct {
	RedirectURIs            []string `json:"redirect_uris"`
	ClientName              string   `json:"client_name"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
}

const maxClientNameLength = 100

// handleRegister implements dynamic client registration (RFC 7591).
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registrationRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "request body must be a JSON object")
		return
	}

	if len(req.RedirectURIs) == 0 || len(req.RedirectURIs) > 10 {
		oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", "between 1 and 10 redirect_uris are required")
		return
	}
	for _, uri := range req.RedirectURIs {
		if err := validateRedirectURI(uri); err != nil {
			oauthError(w, http.StatusBadRequest, "invalid_redirect_uri", err.Error())
			return
		}
	}

	method := req.TokenEndpointAuthMethod
	if method == "" {
		method = "client_secret_basic" // RFC 7591 §2 default
	}
	if !slices.Contains([]string{"none", "client_secret_post", "client_secret_basic"}, method) {
		oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "unsupported token_endpoint_auth_method")
		return
	}
	for _, g := range req.GrantTypes {
		if g != "authorization_code" && g != "refresh_token" {
			oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "unsupported grant type "+g)
			return
		}
	}
	for _, rt := range req.ResponseTypes {
		if rt != "code" {
			oauthError(w, http.StatusBadRequest, "invalid_client_metadata", "unsupported response type "+rt)
			return
		}
	}
	name := strings.TrimSpace(req.ClientName)
	if len([]rune(name)) > maxClientNameLength {
		name = string([]rune(name)[:maxClientNameLength])
	}

	recent, err := s.q.CountRecentOAuthClients(r.Context(), s.now().Add(-time.Hour))
	if err != nil {
		s.serverError(w, r, "count clients", err)
		return
	}
	if recent >= int64(s.cfg.MaxClientsPerHour) {
		oauthError(w, http.StatusTooManyRequests, "temporarily_unavailable", "too many client registrations; try again later")
		return
	}

	clientID := "fw_" + randomID()
	var secret string
	var secretHash []byte
	if method != "none" {
		if secret, err = auth.RandomToken(); err != nil {
			s.serverError(w, r, "generate secret", err)
			return
		}
		secretHash = auth.HashToken(secret)
	}

	client, err := s.q.CreateOAuthClient(r.Context(), store.CreateOAuthClientParams{
		ID:                      clientID,
		SecretHash:              secretHash,
		Name:                    name,
		RedirectUris:            req.RedirectURIs,
		TokenEndpointAuthMethod: method,
	})
	if err != nil {
		s.serverError(w, r, "create client", err)
		return
	}

	resp := map[string]any{
		"client_id":                  client.ID,
		"client_id_issued_at":        client.CreatedAt.Unix(),
		"client_name":                client.Name,
		"redirect_uris":              client.RedirectUris,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": client.TokenEndpointAuthMethod,
		"scope":                      Scope,
	}
	if secret != "" {
		resp["client_secret"] = secret
		resp["client_secret_expires_at"] = 0
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) serverError(w http.ResponseWriter, r *http.Request, msg string, err error) {
	s.logger.ErrorContext(r.Context(), "oauth: "+msg, "error", err)
	oauthError(w, http.StatusInternalServerError, "server_error", "")
}

func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
