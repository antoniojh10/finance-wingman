package oauth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

var errInvalidClient = errors.New("invalid client")

// authenticateClient identifies the client and verifies its secret when it
// was registered as confidential.
func (s *Server) authenticateClient(r *http.Request) (store.OauthClient, error) {
	clientID, secret, hasBasic := r.BasicAuth()
	if !hasBasic {
		clientID = r.PostForm.Get("client_id")
		secret = r.PostForm.Get("client_secret")
	}
	if clientID == "" {
		return store.OauthClient{}, errInvalidClient
	}
	client, err := s.q.GetOAuthClient(r.Context(), clientID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.OauthClient{}, errInvalidClient
	}
	if err != nil {
		return store.OauthClient{}, err
	}
	if client.TokenEndpointAuthMethod == "none" {
		return client, nil
	}
	if secret == "" || subtle.ConstantTimeCompare(auth.HashToken(secret), client.SecretHash) != 1 {
		return store.OauthClient{}, errInvalidClient
	}
	return client, nil
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request", "body must be application/x-www-form-urlencoded")
		return
	}
	client, err := s.authenticateClient(r)
	if errors.Is(err, errInvalidClient) {
		w.Header().Set("WWW-Authenticate", `Basic realm="oauth"`)
		oauthError(w, http.StatusUnauthorized, "invalid_client", "client authentication failed")
		return
	}
	if err != nil {
		s.serverError(w, r, "authenticate client", err)
		return
	}

	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		s.exchangeCode(w, r, client)
	case "refresh_token":
		s.refresh(w, r, client)
	default:
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type", "")
	}
}

func (s *Server) exchangeCode(w http.ResponseWriter, r *http.Request, client store.OauthClient) {
	code, verifier := r.PostForm.Get("code"), r.PostForm.Get("code_verifier")
	if code == "" || verifier == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "code and code_verifier are required")
		return
	}

	record, err := s.q.UseAuthorizationCode(r.Context(), auth.HashToken(code))
	if errors.Is(err, pgx.ErrNoRows) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "invalid or already used authorization code")
		return
	}
	if err != nil {
		s.serverError(w, r, "use authorization code", err)
		return
	}
	switch {
	case !s.now().Before(record.ExpiresAt):
		oauthError(w, http.StatusBadRequest, "invalid_grant", "authorization code expired")
		return
	case record.ClientID != client.ID:
		oauthError(w, http.StatusBadRequest, "invalid_grant", "authorization code was issued to another client")
		return
	case r.PostForm.Get("redirect_uri") != "" && r.PostForm.Get("redirect_uri") != record.RedirectUri:
		oauthError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri does not match")
		return
	case !verifyPKCE(verifier, record.CodeChallenge):
		oauthError(w, http.StatusBadRequest, "invalid_grant", "PKCE verification failed")
		return
	}

	resp, err := s.issueTokens(r.Context(), client.ID, record.UserID, uuid.New())
	if err != nil {
		s.serverError(w, r, "issue tokens", err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request, client store.OauthClient) {
	token := r.PostForm.Get("refresh_token")
	if token == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "refresh_token is required")
		return
	}
	hash := auth.HashToken(token)

	record, err := s.q.RotateRefreshToken(r.Context(), hash)
	if errors.Is(err, pgx.ErrNoRows) {
		// A revoked token being presented again means it may have leaked:
		// revoke every token in its family (OAuth 2.1 §4.3.1).
		if old, err := s.q.GetRefreshToken(r.Context(), hash); err == nil && old.RevokedAt != nil {
			s.logger.WarnContext(r.Context(), "oauth: refresh token reuse detected", "client_id", old.ClientID, "user_id", old.UserID)
			s.revokeFamily(r.Context(), old.FamilyID)
		}
		oauthError(w, http.StatusBadRequest, "invalid_grant", "invalid refresh token")
		return
	}
	if err != nil {
		s.serverError(w, r, "rotate refresh token", err)
		return
	}
	if record.ClientID != client.ID {
		s.revokeFamily(r.Context(), record.FamilyID)
		oauthError(w, http.StatusBadRequest, "invalid_grant", "refresh token was issued to another client")
		return
	}
	if !s.now().Before(record.ExpiresAt) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "refresh token expired")
		return
	}

	resp, err := s.issueTokens(r.Context(), client.ID, record.UserID, record.FamilyID)
	if err != nil {
		s.serverError(w, r, "issue tokens", err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) issueTokens(ctx context.Context, clientID string, userID, family uuid.UUID) (tokenResponse, error) {
	access, err := auth.RandomToken()
	if err != nil {
		return tokenResponse{}, err
	}
	refresh, err := auth.RandomToken()
	if err != nil {
		return tokenResponse{}, err
	}
	now := s.now()
	if _, err := s.q.CreateOAuthSession(ctx, store.CreateOAuthSessionParams{
		UserID:        userID,
		TokenHash:     auth.HashToken(access),
		Client:        SessionClient,
		ExpiresAt:     now.Add(s.cfg.AccessTokenTTL),
		OauthClientID: &clientID,
		OauthFamilyID: &family,
	}); err != nil {
		return tokenResponse{}, err
	}
	if err := s.q.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
		TokenHash: auth.HashToken(refresh),
		FamilyID:  family,
		ClientID:  clientID,
		UserID:    userID,
		Scope:     Scope,
		ExpiresAt: now.Add(s.cfg.RefreshTokenTTL),
	}); err != nil {
		return tokenResponse{}, err
	}
	return tokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.cfg.AccessTokenTTL.Seconds()),
		RefreshToken: refresh,
		Scope:        Scope,
	}, nil
}

func (s *Server) revokeFamily(ctx context.Context, family uuid.UUID) {
	if err := s.q.RevokeRefreshTokenFamily(ctx, family); err != nil {
		s.logger.ErrorContext(ctx, "oauth: revoke refresh tokens", "error", err)
	}
	if err := s.q.DeleteFamilySessions(ctx, &family); err != nil {
		s.logger.ErrorContext(ctx, "oauth: delete family sessions", "error", err)
	}
}

// handleRevoke implements RFC 7009. It always answers 200 so clients cannot
// probe which tokens exist.
func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request", "")
		return
	}
	client, err := s.authenticateClient(r)
	if errors.Is(err, errInvalidClient) {
		oauthError(w, http.StatusUnauthorized, "invalid_client", "")
		return
	}
	if err != nil {
		s.serverError(w, r, "authenticate client", err)
		return
	}

	hash := auth.HashToken(r.PostForm.Get("token"))
	if record, err := s.q.GetRefreshToken(r.Context(), hash); err == nil && record.ClientID == client.ID {
		s.revokeFamily(r.Context(), record.FamilyID)
	} else if family, err := s.q.GetSessionFamily(r.Context(), hash); err == nil && family != nil {
		s.revokeFamily(r.Context(), *family)
	}
	w.WriteHeader(http.StatusOK)
}

// PurgeExpired deletes expired authorization requests, codes, and refresh
// tokens.
func (s *Server) PurgeExpired(ctx context.Context) error {
	return s.q.DeleteExpiredOAuthRecords(ctx, s.now())
}

func verifyPKCE(verifier, challenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(challenge)) == 1
}
