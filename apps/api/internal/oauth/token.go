package oauth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"slices"
	"strings"

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

var (
	errInvalidClient = errors.New("invalid client")
	errScopeExceeded = errors.New("requested scope exceeds the grant")
)

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

	// The scope the user approved; codes issued before scopes existed carry
	// the legacy scope, which is read and write.
	scope := auth.FormatScope(auth.ParseScope(record.Scope))
	var resp tokenResponse
	err = pgx.BeginFunc(r.Context(), s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		grant := uuid.New()
		if err := q.CreateOAuthGrant(r.Context(), store.CreateOAuthGrantParams{
			ID:          grant,
			ClientID:    client.ID,
			UserID:      record.UserID,
			WorkspaceID: record.WorkspaceID,
			Scope:       scope,
		}); err != nil {
			return err
		}
		var err error
		resp, err = s.issueTokens(r.Context(), q, client.ID, record.UserID, record.WorkspaceID, grant, scope)
		return err
	})
	if err != nil {
		s.serverError(w, r, "issue tokens", err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// refresh rotates a refresh token. It locks the token's grant first, the
// lock revocation takes too, so a refresh racing a disconnect either
// completes before it (and its new tokens are revoked with the rest) or
// fails.
func (s *Server) refresh(w http.ResponseWriter, r *http.Request, client store.OauthClient) {
	ctx := r.Context()
	token := r.PostForm.Get("refresh_token")
	if token == "" {
		oauthError(w, http.StatusBadRequest, "invalid_request", "refresh_token is required")
		return
	}
	hash := auth.HashToken(token)
	requested := strings.Fields(r.PostForm.Get("scope"))

	old, err := s.q.GetRefreshToken(ctx, hash)
	if errors.Is(err, pgx.ErrNoRows) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "invalid refresh token")
		return
	}
	if err != nil {
		s.serverError(w, r, "get refresh token", err)
		return
	}

	var (
		resp    tokenResponse
		failure string
	)
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		grant, err := q.LockOAuthGrant(ctx, old.FamilyID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && grant.RevokedAt != nil) {
			failure = "invalid refresh token"
			return nil
		}
		if err != nil {
			return err
		}
		record, err := q.RotateRefreshToken(ctx, hash)
		if errors.Is(err, pgx.ErrNoRows) {
			// A revoked token being presented again means it may have leaked:
			// revoke every token of its grant (OAuth 2.1 §4.3.1).
			s.logger.WarnContext(ctx, "oauth: refresh token reuse detected", "client_id", old.ClientID, "user_id", old.UserID)
			failure = "invalid refresh token"
			return revokeGrant(ctx, q, grant.ID)
		}
		if err != nil {
			return err
		}
		if record.ClientID != client.ID {
			failure = "refresh token was issued to another client"
			return revokeGrant(ctx, q, grant.ID)
		}
		if !s.now().Before(record.ExpiresAt) {
			failure = "refresh token expired"
			return nil
		}
		// New tokens always carry the grant's scope (the one the user
		// approved, read under the grant lock), never more. A client may
		// not ask for write access on refresh if it was not granted (RFC
		// 6749 §6); like at authorization, unknown scopes are ignored. The
		// refusal rolls back the rotation, so the refresh token keeps
		// working.
		scopes := auth.ParseScope(grant.Scope)
		asksWrite := slices.Contains(requested, auth.ScopeWrite) || slices.Contains(requested, "finance")
		if asksWrite && !slices.Contains(scopes, auth.ScopeWrite) {
			return errScopeExceeded
		}
		if err := q.TouchOAuthGrant(ctx, grant.ID); err != nil {
			return err
		}
		resp, err = s.issueTokens(ctx, q, client.ID, record.UserID, record.WorkspaceID, grant.ID, auth.FormatScope(scopes))
		return err
	})
	if errors.Is(err, errScopeExceeded) {
		oauthError(w, http.StatusBadRequest, "invalid_scope", "the requested scope exceeds the one granted")
		return
	}
	if err != nil {
		s.serverError(w, r, "refresh tokens", err)
		return
	}
	if failure != "" {
		oauthError(w, http.StatusBadRequest, "invalid_grant", failure)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// issueTokens creates an access token (a session acting on workspaceID) and
// a refresh token of the grant, which keeps the same workspace when rotated.
// Both carry scope, the grant's: an empty scope would mean full access, so
// it is refused.
func (s *Server) issueTokens(ctx context.Context, q *store.Queries, clientID string, userID uuid.UUID, workspaceID *uuid.UUID, grant uuid.UUID, scope string) (tokenResponse, error) {
	if scope == "" {
		return tokenResponse{}, errors.New("oauth: issuing tokens without a scope")
	}
	access, err := auth.RandomToken()
	if err != nil {
		return tokenResponse{}, err
	}
	refresh, err := auth.RandomToken()
	if err != nil {
		return tokenResponse{}, err
	}
	now := s.now()
	if _, err := q.CreateOAuthSession(ctx, store.CreateOAuthSessionParams{
		UserID:        userID,
		TokenHash:     auth.HashToken(access),
		Client:        SessionClient,
		ExpiresAt:     now.Add(s.cfg.AccessTokenTTL),
		OauthClientID: &clientID,
		OauthFamilyID: &grant,
		WorkspaceID:   workspaceID,
		Scope:         scope,
	}); err != nil {
		return tokenResponse{}, err
	}
	if err := q.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
		TokenHash:   auth.HashToken(refresh),
		FamilyID:    grant,
		ClientID:    clientID,
		UserID:      userID,
		Scope:       scope,
		WorkspaceID: workspaceID,
		ExpiresAt:   now.Add(s.cfg.RefreshTokenTTL),
	}); err != nil {
		return tokenResponse{}, err
	}
	return tokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.cfg.AccessTokenTTL.Seconds()),
		RefreshToken: refresh,
		Scope:        scope,
	}, nil
}

// revokeGrant revokes a grant with its refresh tokens and deletes its access
// token sessions, so the client can neither refresh nor call the API. q must
// be bound to a transaction; the grant row is locked first, like refreshes
// do, so concurrent flows cannot deadlock or outlive the revocation.
func revokeGrant(ctx context.Context, q *store.Queries, grant uuid.UUID) error {
	if err := q.RevokeOAuthGrant(ctx, grant); err != nil {
		return err
	}
	if err := q.RevokeRefreshTokenFamily(ctx, grant); err != nil {
		return err
	}
	return q.DeleteFamilySessions(ctx, &grant)
}

// revoke runs revokeGrant in its own transaction.
func (s *Server) revoke(ctx context.Context, grant uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return revokeGrant(ctx, s.q.WithTx(tx), grant)
	})
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
	var grant *uuid.UUID
	if record, err := s.q.GetRefreshToken(r.Context(), hash); err == nil && record.ClientID == client.ID {
		grant = &record.FamilyID
	} else if family, err := s.q.GetSessionFamily(r.Context(), hash); err == nil && family != nil {
		grant = family
	}
	if grant != nil {
		if err := s.revoke(r.Context(), *grant); err != nil {
			s.logger.ErrorContext(r.Context(), "oauth: revoke grant", "error", err)
		}
	}
	w.WriteHeader(http.StatusOK)
}

// PurgeExpired deletes expired authorization requests, codes, and refresh
// tokens, and the grants left without tokens.
func (s *Server) PurgeExpired(ctx context.Context) error {
	if err := s.q.DeleteExpiredOAuthRecords(ctx, s.now()); err != nil {
		return err
	}
	return s.q.DeleteOrphanOAuthGrants(ctx)
}

func verifyPKCE(verifier, challenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(challenge)) == 1
}
