package oauth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

// ErrConnectionNotFound is returned when a connection does not exist, is
// already disconnected, or belongs to another user.
var ErrConnectionNotFound = errors.New("connection not found")

// Connection is an app (OAuth client) the user authorized to act on one of
// their workspaces, as long as it can still refresh its tokens.
type Connection struct {
	ID          uuid.UUID            `json:"id"`
	ClientName  string               `json:"client_name" doc:"Name the app registered with; empty when it gave none"`
	Workspace   *ConnectionWorkspace `json:"workspace,omitempty" doc:"The workspace the app acts on; omitted when it acts on none"`
	ConnectedAt time.Time            `json:"connected_at"`
	LastUsedAt  time.Time            `json:"last_used_at" doc:"Latest token refresh or request; request times are updated at most once an hour"`
}

type ConnectionWorkspace struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// ListConnections returns the user's connected apps, most recently used
// first.
func (s *Server) ListConnections(ctx context.Context, userID uuid.UUID) ([]Connection, error) {
	rows, err := s.q.ListUserOAuthGrants(ctx, store.ListUserOAuthGrantsParams{UserID: userID, Now: s.now()})
	if err != nil {
		return nil, err
	}
	connections := make([]Connection, len(rows))
	for i, r := range rows {
		connections[i] = Connection{
			ID:          r.ID,
			ClientName:  r.ClientName,
			ConnectedAt: r.CreatedAt,
			LastUsedAt:  r.LastUsedAt,
		}
		if r.WorkspaceID != nil && r.WorkspaceName != nil {
			connections[i].Workspace = &ConnectionWorkspace{ID: *r.WorkspaceID, Name: *r.WorkspaceName}
		}
	}
	return connections, nil
}

// Disconnect revokes one of the user's connections: its refresh tokens stop
// working and its access tokens are deleted, so the app has to be
// authorized again.
func (s *Server) Disconnect(ctx context.Context, userID, connectionID uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		grant, err := q.LockOAuthGrant(ctx, connectionID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrConnectionNotFound
		}
		if err != nil {
			return err
		}
		if grant.UserID != userID || grant.RevokedAt != nil {
			return ErrConnectionNotFound
		}
		return revokeGrant(ctx, q, grant.ID)
	})
}
