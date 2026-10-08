package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/oauth"
)

// errWebSessionOnly is returned when a connected app's access token tries to
// manage the user's sessions or connected apps.
const errWebSessionOnly = "sessions and connected apps can only be managed from a signed-in browser session"

// webSession returns the request's session, or an error when it is the
// access token of a connected app: apps must not see where the user is
// signed in nor disconnect each other.
func webSession(ctx context.Context) (auth.Session, error) {
	return browserSession(ctx, errWebSessionOnly)
}

// errDeletionWebSessionOnly is returned when a connected app's access token
// tries to schedule or cancel a deletion.
const errDeletionWebSessionOnly = "workspaces and accounts can only be deleted from a signed-in browser session"

// browserSession returns the request's session, or a 403 with the given
// message when it is the access token of a connected app.
func browserSession(ctx context.Context, message string) (auth.Session, error) {
	session, _ := sessionFrom(ctx)
	if session.Client != auth.ClientWeb {
		return auth.Session{}, huma.Error403Forbidden(message)
	}
	return session, nil
}

type revokeOtherSessionsOutput struct {
	Body struct {
		Revoked int64 `json:"revoked" doc:"Number of sessions signed out"`
	}
}

func registerSessions(api huma.API, svc *auth.Service, logger *slog.Logger) {
	tags := []string{"Auth"}

	huma.Register(api, huma.Operation{
		OperationID: "list-sessions",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/auth/sessions",
		Summary:     "List the user's active sessions",
		Description: "Browser sessions where the user is signed in, most recently used first. Connected apps are listed separately.",
		Tags:        tags,
	}, func(ctx context.Context, _ *struct{}) (*listOutput[auth.WebSession], error) {
		session, err := webSession(ctx)
		if err != nil {
			return nil, err
		}
		sessions, err := svc.ListSessions(ctx, session.User.ID, session.ID)
		if err != nil {
			return nil, toHTTPError(ctx, logger, err)
		}
		return newList(sessions), nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "revoke-session",
		Method:        http.MethodDelete,
		Path:          apiPrefix + "/auth/sessions/{id}",
		Summary:       "Sign out a session",
		Description:   "Ends one of the user's sessions. Revoking the current session signs the user out.",
		Tags:          tags,
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *idInput) (*struct{}, error) {
		session, err := webSession(ctx)
		if err != nil {
			return nil, err
		}
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		err = svc.RevokeSession(ctx, session.User.ID, id)
		if errors.Is(err, auth.ErrSessionNotFound) {
			return nil, huma.Error404NotFound("session not found")
		}
		if err != nil {
			return nil, toHTTPError(ctx, logger, err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "revoke-other-sessions",
		Method:      http.MethodPost,
		Path:        apiPrefix + "/auth/sessions/revoke-others",
		Summary:     "Sign out everywhere else",
		Description: "Ends every session of the user except the current one. Connected apps stay connected.",
		Tags:        tags,
	}, func(ctx context.Context, _ *struct{}) (*revokeOtherSessionsOutput, error) {
		session, err := webSession(ctx)
		if err != nil {
			return nil, err
		}
		n, err := svc.RevokeOtherSessions(ctx, session.User.ID, session.ID)
		if err != nil {
			return nil, toHTTPError(ctx, logger, err)
		}
		out := &revokeOtherSessionsOutput{}
		out.Body.Revoked = n
		return out, nil
	})
}

func registerConnections(api huma.API, srv *oauth.Server, logger *slog.Logger) {
	tags := []string{"Auth"}

	huma.Register(api, huma.Operation{
		OperationID: "list-connections",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/auth/connections",
		Summary:     "List connected apps",
		Description: "Apps (MCP clients such as Claude or ChatGPT) the user authorized through OAuth that can still act on their behalf, most recently used first.",
		Tags:        tags,
	}, func(ctx context.Context, _ *struct{}) (*listOutput[oauth.Connection], error) {
		session, err := webSession(ctx)
		if err != nil {
			return nil, err
		}
		connections, err := srv.ListConnections(ctx, session.User.ID)
		if err != nil {
			return nil, toHTTPError(ctx, logger, err)
		}
		return newList(connections), nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "disconnect-connection",
		Method:        http.MethodDelete,
		Path:          apiPrefix + "/auth/connections/{id}",
		Summary:       "Disconnect an app",
		Description:   "Revokes the app's refresh tokens and access tokens at once. The app has to be authorized again to regain access.",
		Tags:          tags,
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *idInput) (*struct{}, error) {
		session, err := webSession(ctx)
		if err != nil {
			return nil, err
		}
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		err = srv.Disconnect(ctx, session.User.ID, id)
		if errors.Is(err, oauth.ErrConnectionNotFound) {
			return nil, huma.Error404NotFound("connection not found")
		}
		if err != nil {
			return nil, toHTTPError(ctx, logger, err)
		}
		return nil, nil
	})
}
