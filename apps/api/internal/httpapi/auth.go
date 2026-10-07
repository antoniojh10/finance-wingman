package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

const securityScheme = "bearer"

// publicMetadata marks an operation as not requiring authentication.
var publicMetadata = map[string]any{"public": true}

type sessionKey struct{}

func sessionFrom(ctx context.Context) (auth.Session, bool) {
	s, ok := ctx.Value(sessionKey{}).(auth.Session)
	return s, ok
}

// authMiddleware requires a valid bearer session for every operation that is
// not explicitly public, and exposes the user and the session's workspace to
// the finance service.
func authMiddleware(api huma.API, svc *auth.Service, logger *slog.Logger) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if public, _ := ctx.Operation().Metadata["public"].(bool); public {
			next(ctx)
			return
		}
		token, ok := bearerToken(ctx.Header("Authorization"))
		if !ok {
			unauthorized(api, ctx)
			return
		}
		session, err := svc.Authenticate(ctx.Context(), token)
		if errors.Is(err, auth.ErrUnauthenticated) {
			unauthorized(api, ctx)
			return
		}
		if err != nil {
			logger.ErrorContext(ctx.Context(), "authenticate", "error", err)
			_ = huma.WriteErr(api, ctx, http.StatusInternalServerError, "internal server error")
			return
		}
		ctx = huma.WithValue(ctx, sessionKey{}, session)
		reqCtx := finance.WithActor(ctx.Context(), session.User.ID)
		if session.Workspace != nil {
			reqCtx = db.WithWorkspace(reqCtx, session.Workspace.ID)
		}
		next(huma.WithContext(ctx, reqCtx))
	}
}

// errNoWorkspace is returned by workspace-scoped operations when the session
// has no workspace.
const errNoWorkspace = "no workspace selected: create a workspace or switch to one you belong to"

// requireWorkspace rejects operations on workspace data when the session
// does not act on a workspace. Row-level security would hide the data
// anyway; this turns empty results and failed writes into a clear error.
func requireWorkspace(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if _, ok := db.WorkspaceFrom(ctx.Context()); !ok {
			_ = huma.WriteErr(api, ctx, http.StatusConflict, errNoWorkspace)
			return
		}
		next(ctx)
	}
}

func unauthorized(api huma.API, ctx huma.Context) {
	ctx.SetHeader("WWW-Authenticate", `Bearer realm="finance-wingman"`)
	_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, auth.ErrUnauthenticated.Error())
}

func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}

type loginInput struct {
	Body struct {
		Email  string `json:"email" format:"email" maxLength:"254"`
		Locale string `json:"locale,omitempty" enum:"en,es" doc:"Language of the email; defaults to the user's preference"`
	}
}

type loginOutput struct {
	Body struct {
		Message string `json:"message"`
	}
}

type verifyInput struct {
	Body struct {
		Token string `json:"token,omitempty" doc:"Token from the magic link"`
		Email string `json:"email,omitempty" doc:"Email address, required together with code"`
		Code  string `json:"code,omitempty" pattern:"^[0-9]{6}$" doc:"Six-digit code from the email"`
	}
}

func registerAuth(api huma.API, svc *auth.Service, logger *slog.Logger) {
	tags := []string{"Auth"}
	public := []map[string][]string{}

	huma.Register(api, huma.Operation{
		OperationID:   "request-login",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/auth/login",
		Summary:       "Request a sign-in email",
		Description:   "Sends a magic link and a six-digit code to the address if it belongs to an allowed user. The response is the same whether or not the address is known.",
		Tags:          tags,
		Security:      public,
		Metadata:      publicMetadata,
		DefaultStatus: http.StatusAccepted,
	}, func(ctx context.Context, in *loginInput) (*loginOutput, error) {
		err := svc.RequestLogin(ctx, in.Body.Email, in.Body.Locale)
		if errors.Is(err, auth.ErrInvalidEmail) {
			return nil, huma.Error422UnprocessableEntity("invalid email address")
		}
		if err != nil {
			logger.ErrorContext(ctx, "request login", "error", err)
			return nil, huma.Error500InternalServerError("could not send the sign-in email")
		}
		out := &loginOutput{}
		out.Body.Message = "If the address has access, a sign-in email is on its way."
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "verify-login",
		Method:      http.MethodPost,
		Path:        apiPrefix + "/auth/verify",
		Summary:     "Exchange a magic link token or code for a session",
		Description: "Provide either `token`, or `email` and `code`. Returns a bearer token for subsequent requests.",
		Tags:        tags,
		Security:    public,
		Metadata:    publicMetadata,
	}, func(ctx context.Context, in *verifyInput) (*bodyOutput[auth.Session], error) {
		var (
			session auth.Session
			err     error
		)
		switch {
		case in.Body.Token != "":
			session, err = svc.VerifyToken(ctx, in.Body.Token, auth.ClientWeb)
		case in.Body.Email != "" && in.Body.Code != "":
			session, err = svc.VerifyCode(ctx, in.Body.Email, in.Body.Code, auth.ClientWeb)
		default:
			return nil, huma.Error422UnprocessableEntity("provide token, or email and code")
		}
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return nil, huma.Error401Unauthorized(err.Error())
		}
		if err != nil {
			logger.ErrorContext(ctx, "verify login", "error", err)
			return nil, huma.Error500InternalServerError("internal server error")
		}
		return &bodyOutput[auth.Session]{Body: session}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-current-user",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/auth/me",
		Summary:     "Get the signed-in user",
		Tags:        tags,
	}, func(ctx context.Context, _ *struct{}) (*bodyOutput[auth.User], error) {
		session, _ := sessionFrom(ctx)
		return &bodyOutput[auth.User]{Body: session.User}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-current-user",
		Method:      http.MethodPatch,
		Path:        apiPrefix + "/auth/me",
		Summary:     "Update the signed-in user's profile",
		Tags:        tags,
	}, func(ctx context.Context, in *struct{ Body auth.UpdateProfileInput }) (*bodyOutput[auth.User], error) {
		session, _ := sessionFrom(ctx)
		user, err := svc.UpdateProfile(ctx, session.User.ID, in.Body)
		if err != nil {
			logger.ErrorContext(ctx, "update profile", "error", err)
			return nil, huma.Error500InternalServerError("internal server error")
		}
		return &bodyOutput[auth.User]{Body: user}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "logout",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/auth/logout",
		Summary:       "Revoke the current session",
		Tags:          tags,
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, _ *struct{}) (*struct{}, error) {
		session, _ := sessionFrom(ctx)
		if err := svc.Logout(ctx, session.ID); err != nil {
			logger.ErrorContext(ctx, "logout", "error", err)
			return nil, huma.Error500InternalServerError("internal server error")
		}
		return nil, nil
	})
}
