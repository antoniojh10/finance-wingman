// Package workspace manages workspaces, their members and invitations.
// Finance data is isolated per workspace by row-level security; this package
// decides who belongs to which workspace.
package workspace

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	mailer "github.com/antoniojh10/finance-wingman/apps/api/internal/mail"
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

// Membership is a workspace seen by one of its members.
type Membership struct {
	Workspace
	Role string `json:"role" enum:"owner,member"`
}

type Config struct {
	// WebBaseURL is where invitation links point to (the Next.js app).
	WebBaseURL string
	// InvitationTTL is how long an invitation can be accepted.
	InvitationTTL time.Duration
	// MaxInvitationsPerHour caps invitation emails per workspace.
	MaxInvitationsPerHour int
}

func (c Config) withDefaults() Config {
	if c.InvitationTTL == 0 {
		c.InvitationTTL = 7 * 24 * time.Hour
	}
	if c.MaxInvitationsPerHour == 0 {
		c.MaxInvitationsPerHour = 20
	}
	return c
}

type Service struct {
	pool *pgxpool.Pool
	q    *store.Queries
	mail mailer.Sender
	cfg  Config
	now  func() time.Time
}

func NewService(pool *pgxpool.Pool, sender mailer.Sender, cfg Config) *Service {
	return &Service{pool: pool, q: store.New(pool), mail: sender, cfg: cfg.withDefaults(), now: time.Now}
}

// SetClock overrides the time source; intended for tests.
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// List returns the workspaces the user belongs to.
func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Membership, error) {
	rows, err := s.q.ListUserWorkspaces(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]Membership, len(rows))
	for i, r := range rows {
		out[i] = Membership{Workspace: Workspace{ID: r.ID, Name: r.Name, CreatedAt: r.CreatedAt}, Role: r.Role}
	}
	return out, nil
}

// Rename changes the workspace name. Only owners can rename.
func (s *Service) Rename(ctx context.Context, actorID, workspaceID uuid.UUID, name string) (Workspace, error) {
	if err := s.requireOwner(ctx, actorID, workspaceID); err != nil {
		return Workspace{}, err
	}
	name, err := validName(name)
	if err != nil {
		return Workspace{}, err
	}
	w, err := s.q.RenameWorkspace(ctx, store.RenameWorkspaceParams{ID: workspaceID, Name: name})
	if err != nil {
		return Workspace{}, err
	}
	return fromModel(w), nil
}

// role returns the actor's role in the workspace. Workspaces the actor does
// not belong to are reported as not found, so their existence isn't revealed.
func (s *Service) role(ctx context.Context, actorID, workspaceID uuid.UUID) (string, error) {
	role, err := s.q.GetMemberRole(ctx, store.GetMemberRoleParams{WorkspaceID: workspaceID, UserID: actorID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", finance.NotFound("workspace")
	}
	return role, err
}

func (s *Service) requireOwner(ctx context.Context, actorID, workspaceID uuid.UUID) error {
	role, err := s.role(ctx, actorID, workspaceID)
	if err != nil {
		return err
	}
	if role != RoleOwner {
		return errOwnersOnly
	}
	return nil
}

var errOwnersOnly = finance.Forbidden("only workspace owners can do this")

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
