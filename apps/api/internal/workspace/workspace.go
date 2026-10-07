// Package workspace manages workspaces, their members and invitations.
// Finance data is isolated per workspace by row-level security; this package
// decides who belongs to which workspace.
package workspace

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

const (
	RoleOwner  = "owner"
	RoleMember = "member"

	maxNameLength = 100
)

type Workspace struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Service struct {
	pool *pgxpool.Pool
	q    *store.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, q: store.New(pool)}
}

// Create makes a new workspace owned by the user.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, name string) (Workspace, error) {
	name, err := validName(name)
	if err != nil {
		return Workspace{}, err
	}
	var w store.Workspace
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		var err error
		if w, err = q.CreateWorkspace(ctx, name); err != nil {
			return err
		}
		return q.AddWorkspaceMember(ctx, store.AddWorkspaceMemberParams{WorkspaceID: w.ID, UserID: userID, Role: RoleOwner})
	})
	if err != nil {
		return Workspace{}, err
	}
	return fromModel(w), nil
}

// Bootstrap creates the first workspace, owned by the given users, when the
// database has none. It reports whether a workspace was created.
func (s *Service) Bootstrap(ctx context.Context, name string, ownerIDs []uuid.UUID) (bool, error) {
	if len(ownerIDs) == 0 {
		return false, nil
	}
	name, err := validName(name)
	if err != nil {
		return false, err
	}
	created := false
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		// Serializes concurrent starts (e.g. several replicas).
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext('workspace-bootstrap'))"); err != nil {
			return err
		}
		n, err := q.CountWorkspaces(ctx)
		if err != nil || n > 0 {
			return err
		}
		w, err := q.CreateWorkspace(ctx, name)
		if err != nil {
			return err
		}
		for _, id := range ownerIDs {
			if err := q.AddWorkspaceMember(ctx, store.AddWorkspaceMemberParams{WorkspaceID: w.ID, UserID: id, Role: RoleOwner}); err != nil {
				return err
			}
		}
		created = true
		return nil
	})
	return created, err
}

func validName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", finance.Invalid("name", "must not be empty")
	}
	if len([]rune(name)) > maxNameLength {
		return "", finance.Invalid("name", "must be at most 100 characters")
	}
	return name, nil
}

func fromModel(w store.Workspace) Workspace {
	return Workspace{ID: w.ID, Name: w.Name, CreatedAt: w.CreatedAt}
}
