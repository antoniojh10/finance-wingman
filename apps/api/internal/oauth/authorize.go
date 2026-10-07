package oauth

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

// PKCE verifiers are 43-128 characters (RFC 7636 §4.1); an S256 challenge is
// always the 43-character base64url encoding of a SHA-256 digest.
var codeChallengePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

// handleAuthorize validates an authorization request and shows the sign-in
// page. Errors that make the redirect URI untrustworthy are shown on the page;
// all others are returned to the client through the redirect URI.
func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	locale := pickLocale(q.Get("ui_locales"), r.Header.Get("Accept-Language"))

	client, err := s.q.GetOAuthClient(r.Context(), q.Get("client_id"))
	if errors.Is(err, pgx.ErrNoRows) {
		s.renderError(w, r, locale, http.StatusBadRequest, msgUnknownClient)
		return
	}
	if err != nil {
		s.logger.ErrorContext(r.Context(), "oauth: get client", "error", err)
		s.renderError(w, r, locale, http.StatusInternalServerError, msgServerError)
		return
	}

	redirectURI := q.Get("redirect_uri")
	if redirectURI == "" && len(client.RedirectUris) == 1 {
		redirectURI = client.RedirectUris[0]
	}
	if !matchRedirectURI(client.RedirectUris, redirectURI) {
		s.renderError(w, r, locale, http.StatusBadRequest, msgInvalidRedirect)
		return
	}

	state := q.Get("state")
	fail := func(code, description string) {
		http.Redirect(w, r, s.errorRedirect(redirectURI, state, code, description), http.StatusFound)
	}
	if q.Get("response_type") != "code" {
		fail("unsupported_response_type", "only response_type=code is supported")
		return
	}
	if q.Get("code_challenge_method") != "S256" || !codeChallengePattern.MatchString(q.Get("code_challenge")) {
		fail("invalid_request", "PKCE with code_challenge_method=S256 is required")
		return
	}
	if resource := q.Get("resource"); resource != "" && strings.TrimRight(resource, "/") != s.ResourceURL() {
		fail("invalid_target", "unknown resource")
		return
	}

	req, err := s.q.CreateAuthorizationRequest(r.Context(), store.CreateAuthorizationRequestParams{
		ClientID:      client.ID,
		RedirectUri:   redirectURI,
		CodeChallenge: q.Get("code_challenge"),
		State:         state,
		Scope:         Scope,
		Resource:      s.ResourceURL(),
		ExpiresAt:     s.now().Add(s.cfg.AuthorizationRequestTTL),
	})
	if err != nil {
		s.logger.ErrorContext(r.Context(), "oauth: create authorization request", "error", err)
		s.renderError(w, r, locale, http.StatusInternalServerError, msgServerError)
		return
	}

	s.renderPage(w, r, http.StatusOK, pageData{
		Locale:       locale,
		Step:         stepEmail,
		RequestID:    req.ID.String(),
		ClientName:   displayName(client.Name),
		RedirectHost: hostOf(redirectURI),
	})
}

// handleAuthorizeSubmit processes the sign-in page forms.
func (s *Server) handleAuthorizeSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.renderError(w, r, "en", http.StatusBadRequest, msgExpired)
		return
	}
	locale := pickLocale(r.PostForm.Get("locale"), r.Header.Get("Accept-Language"))

	id, err := uuid.Parse(r.PostForm.Get("request_id"))
	if err != nil {
		s.renderError(w, r, locale, http.StatusBadRequest, msgExpired)
		return
	}
	req, err := s.q.GetAuthorizationRequest(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !s.now().Before(req.ExpiresAt)) {
		s.renderError(w, r, locale, http.StatusBadRequest, msgExpired)
		return
	}
	if err != nil {
		s.logger.ErrorContext(r.Context(), "oauth: get authorization request", "error", err)
		s.renderError(w, r, locale, http.StatusInternalServerError, msgServerError)
		return
	}

	page := pageData{
		Locale:       locale,
		RequestID:    req.ID.String(),
		ClientName:   displayName(req.ClientName),
		RedirectHost: hostOf(req.RedirectUri),
	}

	switch r.PostForm.Get("action") {
	case "send_code":
		email, err := auth.NormalizeEmail(r.PostForm.Get("email"))
		if err != nil {
			page.Step, page.Error, page.Email = stepEmail, msgInvalidEmail, r.PostForm.Get("email")
			s.renderPage(w, r, http.StatusUnprocessableEntity, page)
			return
		}
		if err := s.auth.RequestLoginCode(r.Context(), email, locale, page.ClientName); err != nil {
			s.logger.ErrorContext(r.Context(), "oauth: request login code", "error", err)
			page.Step, page.Error, page.Email = stepEmail, msgSendFailed, email
			s.renderPage(w, r, http.StatusInternalServerError, page)
			return
		}
		if err := s.q.SetAuthorizationRequestEmail(r.Context(), store.SetAuthorizationRequestEmailParams{ID: req.ID, Email: &email}); err != nil {
			s.logger.ErrorContext(r.Context(), "oauth: set request email", "error", err)
			s.renderError(w, r, locale, http.StatusInternalServerError, msgServerError)
			return
		}
		page.Step, page.Email = stepCode, email
		s.renderPage(w, r, http.StatusOK, page)

	case "verify":
		if req.Email == nil {
			page.Step = stepEmail
			s.renderPage(w, r, http.StatusBadRequest, page)
			return
		}
		user, err := s.auth.AuthenticateCode(r.Context(), *req.Email, r.PostForm.Get("code"))
		if errors.Is(err, auth.ErrInvalidCredentials) {
			page.Step, page.Email, page.Error = stepCode, *req.Email, msgInvalidCode
			s.renderPage(w, r, http.StatusUnauthorized, page)
			return
		}
		if err != nil {
			s.logger.ErrorContext(r.Context(), "oauth: authenticate code", "error", err)
			s.renderError(w, r, locale, http.StatusInternalServerError, msgServerError)
			return
		}
		var workspaceID *uuid.UUID
		if id, err := s.q.GetDefaultWorkspaceID(r.Context(), user.ID); err == nil {
			workspaceID = &id
		} else if !errors.Is(err, pgx.ErrNoRows) {
			s.logger.ErrorContext(r.Context(), "oauth: get default workspace", "error", err)
			s.renderError(w, r, locale, http.StatusInternalServerError, msgServerError)
			return
		}
		code, err := s.issueAuthorizationCode(r, req, user.ID, workspaceID)
		if err != nil {
			s.logger.ErrorContext(r.Context(), "oauth: issue authorization code", "error", err)
			s.renderError(w, r, locale, http.StatusInternalServerError, msgServerError)
			return
		}
		http.Redirect(w, r, s.successRedirect(req.RedirectUri, req.State, code), http.StatusFound)

	case "change_email":
		page.Step = stepEmail
		s.renderPage(w, r, http.StatusOK, page)

	case "deny":
		if err := s.q.DeleteAuthorizationRequest(r.Context(), req.ID); err != nil {
			s.logger.ErrorContext(r.Context(), "oauth: delete authorization request", "error", err)
		}
		http.Redirect(w, r, s.errorRedirect(req.RedirectUri, req.State, "access_denied", "the user denied the request"), http.StatusFound)

	default:
		s.renderError(w, r, locale, http.StatusBadRequest, msgExpired)
	}
}

func (s *Server) issueAuthorizationCode(r *http.Request, req store.GetAuthorizationRequestRow, userID uuid.UUID, workspaceID *uuid.UUID) (string, error) {
	code, err := auth.RandomToken()
	if err != nil {
		return "", err
	}
	if err := s.q.CreateAuthorizationCode(r.Context(), store.CreateAuthorizationCodeParams{
		CodeHash:      auth.HashToken(code),
		ClientID:      req.ClientID,
		UserID:        userID,
		RedirectUri:   req.RedirectUri,
		CodeChallenge: req.CodeChallenge,
		Scope:         req.Scope,
		WorkspaceID:   workspaceID,
		ExpiresAt:     s.now().Add(s.cfg.AuthorizationCodeTTL),
	}); err != nil {
		return "", err
	}
	return code, s.q.DeleteAuthorizationRequest(r.Context(), req.ID)
}

func (s *Server) successRedirect(redirectURI, state, code string) string {
	return withQuery(redirectURI, map[string]string{"code": code, "state": state, "iss": s.cfg.Issuer})
}

func (s *Server) errorRedirect(redirectURI, state, code, description string) string {
	return withQuery(redirectURI, map[string]string{"error": code, "error_description": description, "state": state, "iss": s.cfg.Issuer})
}

func withQuery(rawURL string, params map[string]string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	q := u.Query()
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	return u.Host
}

func displayName(name string) string {
	if strings.TrimSpace(name) == "" {
		return "An application"
	}
	return name
}
