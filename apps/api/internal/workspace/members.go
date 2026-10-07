package workspace

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

type Member struct {
	UserID   uuid.UUID `json:"user_id"`
	Email    string    `json:"email" format:"email"`
	Name     string    `json:"name"`
	Role     string    `json:"role" enum:"owner,member"`
	JoinedAt time.Time `json:"joined_at"`
}

// Members lists the members of a workspace the actor belongs to.
func (s *Service) Members(ctx context.Context, actorID, workspaceID uuid.UUID) ([]Member, error) {
	if _, err := s.role(ctx, actorID, workspaceID); err != nil {
		return nil, err
	}
	rows, err := s.q.ListWorkspaceMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]Member, len(rows))
	for i, r := range rows {
		out[i] = Member{UserID: r.ID, Email: r.Email, Name: r.Name, Role: r.Role, JoinedAt: r.CreatedAt}
	}
	return out, nil
}

// SetRole changes a member's role. Only owners can change roles, and the
// last owner can't be demoted.
func (s *Service) SetRole(ctx context.Context, actorID, workspaceID, userID uuid.UUID, role string) error {
	if role != RoleOwner && role != RoleMember {
		return finance.Invalid("role", "must be owner or member")
	}
	return s.changeMembers(ctx, actorID, workspaceID, userID, memberChange{
		dropsOwner: role == RoleMember,
		apply: func(q *store.Queries) error {
			_, err := q.UpdateMemberRole(ctx, store.UpdateMemberRoleParams{WorkspaceID: workspaceID, UserID: userID, Role: role})
			return err
		},
	})
}

// RemoveMember takes a user out of a workspace. Owners can remove anyone;
// any member can remove themselves (leave). The last owner can't leave.
func (s *Service) RemoveMember(ctx context.Context, actorID, workspaceID, userID uuid.UUID) error {
	return s.changeMembers(ctx, actorID, workspaceID, userID, memberChange{
		dropsOwner: true,
		self:       true,
		apply: func(q *store.Queries) error {
			if _, err := q.RemoveWorkspaceMember(ctx, store.RemoveWorkspaceMemberParams{WorkspaceID: workspaceID, UserID: userID}); err != nil {
				return err
			}
			return q.ClearSessionsWorkspace(ctx, store.ClearSessionsWorkspaceParams{WorkspaceID: &workspaceID, UserID: userID})
		},
	})
}

type memberChange struct {
	// dropsOwner tells whether the change takes the owner role away from the
	// target, which is refused for the last owner.
	dropsOwner bool
	// self allows members (not only owners) to apply it to themselves.
	self  bool
	apply func(*store.Queries) error
}

// changeMembers applies a membership change under a lock on the workspace's
// memberships, so concurrent changes can't leave it without an owner.
func (s *Service) changeMembers(ctx context.Context, actorID, workspaceID, userID uuid.UUID, change memberChange) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		members, err := q.LockWorkspaceMembers(ctx, workspaceID)
		if err != nil {
			return err
		}
		roles := make(map[uuid.UUID]string, len(members))
		owners := 0
		for _, m := range members {
			roles[m.UserID] = m.Role
			if m.Role == RoleOwner {
				owners++
			}
		}
		actorRole, ok := roles[actorID]
		if !ok {
			return finance.NotFound("workspace")
		}
		if actorRole != RoleOwner && !(change.self && actorID == userID) {
			return errOwnersOnly
		}
		targetRole, ok := roles[userID]
		if !ok {
			return finance.NotFound("member")
		}
		if change.dropsOwner && targetRole == RoleOwner && owners == 1 {
			return finance.Conflict("a workspace needs at least one owner: make another member an owner first")
		}
		return change.apply(q)
	})
}
