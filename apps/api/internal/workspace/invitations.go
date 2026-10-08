package workspace

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	mailer "github.com/antoniojh10/finance-wingman/apps/api/internal/mail"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

// Invitation is an open invitation, as listed to workspace owners.
type Invitation struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email" format:"email"`
	Role      string    `json:"role" enum:"owner,member"`
	InvitedBy string    `json:"invited_by,omitempty" doc:"Name or email of the inviter; omitted when unknown"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// InvitationPreview is what the invitee sees before accepting.
type InvitationPreview struct {
	Email         string    `json:"email" format:"email"`
	WorkspaceName string    `json:"workspace_name"`
	InvitedBy     string    `json:"invited_by,omitempty" doc:"Name or email of the inviter; omitted when unknown"`
	Role          string    `json:"role" enum:"owner,member"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type InviteInput struct {
	Email  string `json:"email" format:"email" maxLength:"254"`
	Role   string `json:"role,omitempty" enum:"owner,member" doc:"Defaults to member"`
	Locale string `json:"locale,omitempty" enum:"en,es" doc:"Language of the email; defaults to the inviter's"`
}

// Accepted identifies the user and workspace of an accepted invitation.
type Accepted struct {
	UserID      uuid.UUID
	WorkspaceID uuid.UUID
}

var errInvalidInvitation = finance.NotFound("invitation (it may have expired, been revoked or already been used)")

// Invite emails an invitation to join the workspace. Inviting an address
// again replaces its open invitation. Only owners can invite.
func (s *Service) Invite(ctx context.Context, actorID, workspaceID uuid.UUID, in InviteInput) (Invitation, error) {
	if err := s.requireOwner(ctx, actorID, workspaceID); err != nil {
		return Invitation{}, err
	}
	email, err := auth.NormalizeEmail(in.Email)
	if err != nil {
		return Invitation{}, finance.Invalid("email", "must be a valid email address")
	}
	role := in.Role
	if role == "" {
		role = RoleMember
	}
	if role != RoleOwner && role != RoleMember {
		return Invitation{}, finance.Invalid("role", "must be owner or member")
	}
	member, err := s.q.IsMemberByEmail(ctx, store.IsMemberByEmailParams{WorkspaceID: workspaceID, Email: email})
	if err != nil {
		return Invitation{}, err
	}
	if member {
		return Invitation{}, finance.Conflict(email + " is already a member of this workspace")
	}
	now := s.now()
	recent, err := s.q.CountRecentInvitations(ctx, store.CountRecentInvitationsParams{WorkspaceID: workspaceID, CreatedAt: now.Add(-time.Hour)})
	if err != nil {
		return Invitation{}, err
	}
	if recent >= int64(s.cfg.MaxInvitationsPerHour) {
		return Invitation{}, finance.Conflict("too many invitations sent in the last hour; try again later")
	}
	inviter, err := s.q.GetUser(ctx, actorID)
	if err != nil {
		return Invitation{}, err
	}
	w, err := s.q.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return Invitation{}, err
	}

	token, err := auth.RandomToken()
	if err != nil {
		return Invitation{}, err
	}
	var row store.WorkspaceInvitation
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.RevokeOpenInvitationsForEmail(ctx, store.RevokeOpenInvitationsForEmailParams{WorkspaceID: workspaceID, Email: email}); err != nil {
			return err
		}
		var err error
		row, err = q.CreateInvitation(ctx, store.CreateInvitationParams{
			WorkspaceID: workspaceID,
			Email:       email,
			Role:        role,
			TokenHash:   auth.HashToken(token),
			InvitedBy:   &actorID,
			ExpiresAt:   now.Add(s.cfg.InvitationTTL),
		})
		if err != nil {
			return err
		}
		// Sent inside the transaction so a failed email leaves no invitation.
		return s.sendInvitation(ctx, email, in.Locale, inviter, w.Name, token)
	})
	if err != nil {
		return Invitation{}, err
	}
	return Invitation{
		ID: row.ID, Email: row.Email, Role: row.Role, InvitedBy: displayName(&inviter.Name, &inviter.Email),
		ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
	}, nil
}

func (s *Service) sendInvitation(ctx context.Context, email, locale string, inviter store.User, workspaceName, token string) error {
	if locale != "en" && locale != "es" {
		locale = inviter.Locale
	}
	msg, err := mailer.RenderInvitation(mailer.InvitationEmail{
		To:            email,
		Locale:        locale,
		WorkspaceName: workspaceName,
		InviterName:   displayName(&inviter.Name, &inviter.Email),
		Link:          strings.TrimRight(s.cfg.WebBaseURL, "/") + "/invite?token=" + url.QueryEscape(token),
		TTLDays:       int(s.cfg.InvitationTTL.Hours() / 24),
	})
	if err != nil {
		return err
	}
	if err := s.mail.Send(ctx, msg); err != nil {
		return fmt.Errorf("send invitation email: %w", err)
	}
	return nil
}

// Invitations lists the open invitations of a workspace. Only owners can
// see them.
func (s *Service) Invitations(ctx context.Context, actorID, workspaceID uuid.UUID) ([]Invitation, error) {
	if err := s.requireOwner(ctx, actorID, workspaceID); err != nil {
		return nil, err
	}
	rows, err := s.q.ListOpenInvitations(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]Invitation, len(rows))
	for i, r := range rows {
		out[i] = Invitation{
			ID: r.ID, Email: r.Email, Role: r.Role, InvitedBy: displayName(r.InvitedByName, r.InvitedByEmail),
			ExpiresAt: r.ExpiresAt, CreatedAt: r.CreatedAt,
		}
	}
	return out, nil
}

// RevokeInvitation cancels an open invitation. Only owners can revoke.
func (s *Service) RevokeInvitation(ctx context.Context, actorID, workspaceID, invitationID uuid.UUID) error {
	if err := s.requireOwner(ctx, actorID, workspaceID); err != nil {
		return err
	}
	n, err := s.q.RevokeInvitation(ctx, store.RevokeInvitationParams{ID: invitationID, WorkspaceID: workspaceID})
	if err != nil {
		return err
	}
	if n == 0 {
		return finance.NotFound("invitation")
	}
	return nil
}

// PreviewInvitation describes an open invitation from its emailed token.
func (s *Service) PreviewInvitation(ctx context.Context, token string) (InvitationPreview, error) {
	if token == "" {
		return InvitationPreview{}, errInvalidInvitation
	}
	inv, err := s.q.GetInvitationByTokenHash(ctx, auth.HashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return InvitationPreview{}, errInvalidInvitation
	}
	if err != nil {
		return InvitationPreview{}, err
	}
	if inv.AcceptedAt != nil || inv.RevokedAt != nil || !s.now().Before(inv.ExpiresAt) {
		return InvitationPreview{}, errInvalidInvitation
	}
	return InvitationPreview{
		Email: inv.Email, WorkspaceName: inv.WorkspaceName, InvitedBy: displayName(inv.InvitedByName, inv.InvitedByEmail),
		Role: inv.Role, ExpiresAt: inv.ExpiresAt,
	}, nil
}

// AcceptInvitation redeems the emailed token: the invitee becomes a member
// (and a user, if the address had no access yet). The token proves the
// invitee owns the address, like a magic link, so the caller may sign them in.
func (s *Service) AcceptInvitation(ctx context.Context, token string) (Accepted, error) {
	if token == "" {
		return Accepted{}, errInvalidInvitation
	}
	var (
		out      Accepted
		joined   store.User
		role     string
		wsName   string
		owners   []store.ListWorkspaceMembersRow
		isNewMem bool
	)
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		inv, err := q.AcceptInvitation(ctx, store.AcceptInvitationParams{TokenHash: auth.HashToken(token), ExpiresAt: s.now()})
		if errors.Is(err, pgx.ErrNoRows) {
			return errInvalidInvitation
		}
		if err != nil {
			return err
		}
		user, err := q.UpsertUser(ctx, store.UpsertUserParams{Email: inv.Email})
		if err != nil {
			return err
		}
		// Someone who already belongs keeps their current role.
		_, err = q.GetMemberRole(ctx, store.GetMemberRoleParams{WorkspaceID: inv.WorkspaceID, UserID: user.ID})
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		isNewMem = errors.Is(err, pgx.ErrNoRows)
		if err := q.AddWorkspaceMember(ctx, store.AddWorkspaceMemberParams{WorkspaceID: inv.WorkspaceID, UserID: user.ID, Role: inv.Role}); err != nil {
			return err
		}
		out = Accepted{UserID: user.ID, WorkspaceID: inv.WorkspaceID}
		if !isNewMem {
			return nil
		}
		joined, role = user, inv.Role
		w, err := q.GetWorkspace(ctx, inv.WorkspaceID)
		if err != nil {
			return err
		}
		wsName = w.Name
		members, err := q.ListWorkspaceMembers(ctx, inv.WorkspaceID)
		if err != nil {
			return err
		}
		for _, m := range members {
			if m.Role == RoleOwner && m.ID != user.ID {
				owners = append(owners, m)
			}
		}
		return nil
	})
	if err != nil {
		return out, err
	}
	// Sent after the commit: the invitee has joined whether or not the
	// notification goes out.
	s.notifyMemberJoined(ctx, owners, joined, role, wsName)
	return out, nil
}

// notifyMemberJoined tells the workspace owners (other than the person who
// joined) about the new member. Failures are logged, never returned.
func (s *Service) notifyMemberJoined(ctx context.Context, owners []store.ListWorkspaceMembersRow, joined store.User, role, workspaceName string) {
	for _, o := range owners {
		msg, err := mailer.RenderMemberJoined(mailer.MemberJoinedEmail{
			To: o.Email, Name: o.Name, Locale: o.Locale,
			Member: displayName(&joined.Name, &joined.Email), MemberEmail: joined.Email,
			Role: role, WorkspaceName: workspaceName,
			Link: s.link("/settings/workspace"),
		})
		if err == nil {
			err = s.mail.Send(ctx, msg)
		}
		if err != nil {
			s.logger.WarnContext(ctx, "workspace: send member joined email", "error", err, "workspace_id", joined.ID)
		}
	}
}

// PurgeExpired deletes invitations that can no longer be accepted.
func (s *Service) PurgeExpired(ctx context.Context) error {
	return s.q.DeleteExpiredInvitations(ctx, s.now())
}

func displayName(name, email *string) string {
	if name != nil && strings.TrimSpace(*name) != "" {
		return *name
	}
	if email != nil {
		return *email
	}
	return ""
}
