package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	mailer "github.com/antoniojh10/finance-wingman/apps/api/internal/mail"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

// What deleting their account means for each of the user's workspaces.
const (
	// OutcomeLeave: the user leaves; their accounts become shared.
	OutcomeLeave = "leave"
	// OutcomeDelete: the user is its only member, so it is deleted too.
	OutcomeDelete = "delete"
	// OutcomeBlocked: the user is its only owner and it has other members,
	// so the account can't be deleted until ownership is transferred or the
	// workspace is deleted.
	OutcomeBlocked = "blocked"
)

// AccountDeletion describes the deletion of a user's account: whether it is
// scheduled, and what happens to each of their workspaces.
type AccountDeletion struct {
	ScheduledFor *time.Time              `json:"scheduled_for,omitempty" doc:"When the account will be deleted; omitted unless a deletion is scheduled"`
	Workspaces   []AccountDeletionImpact `json:"workspaces"`
}

// AccountDeletionImpact is what deleting the account does to one workspace.
type AccountDeletionImpact struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Role    string    `json:"role" enum:"owner,member"`
	Members int       `json:"members" doc:"Number of members, the user included"`
	Outcome string    `json:"outcome" enum:"leave,delete,blocked" doc:"leave: the user leaves and their accounts become shared; delete: the user is the only member, so the workspace and all its data are deleted; blocked: the user is the only owner of a workspace with other members, which prevents the deletion"`
}

// Blocking returns the names of the workspaces that prevent the deletion.
func (d AccountDeletion) Blocking() []string {
	var names []string
	for _, w := range d.Workspaces {
		if w.Outcome == OutcomeBlocked {
			names = append(names, w.Name)
		}
	}
	return names
}

func (d AccountDeletion) deleted() []string {
	var names []string
	for _, w := range d.Workspaces {
		if w.Outcome == OutcomeDelete {
			names = append(names, w.Name)
		}
	}
	return names
}

func accountDeletion(scheduledFor *time.Time, rows []store.ListUserWorkspacesForDeletionRow) AccountDeletion {
	d := AccountDeletion{ScheduledFor: scheduledFor, Workspaces: make([]AccountDeletionImpact, len(rows))}
	for i, r := range rows {
		outcome := OutcomeLeave
		switch {
		case r.Members == 1:
			outcome = OutcomeDelete
		case r.Role == RoleOwner && r.Owners == 1:
			outcome = OutcomeBlocked
		}
		d.Workspaces[i] = AccountDeletionImpact{ID: r.ID, Name: r.Name, Role: r.Role, Members: int(r.Members), Outcome: outcome}
	}
	return d
}

func blockedError(names []string) error {
	return finance.Conflict(fmt.Sprintf(
		"you are the only owner of %s, which has other members: make another member an owner, or delete the workspace, first",
		strings.Join(quoted(names), ", ")))
}

func quoted(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = fmt.Sprintf("%q", n)
	}
	return out
}

// AccountDeletion describes what deleting the user's account would do, and
// when it is scheduled, if it is.
func (s *Service) AccountDeletion(ctx context.Context, userID uuid.UUID) (AccountDeletion, error) {
	user, err := s.q.GetUser(ctx, userID)
	if err != nil {
		return AccountDeletion{}, err
	}
	rows, err := s.q.ListUserWorkspacesForDeletion(ctx, userID)
	if err != nil {
		return AccountDeletion{}, err
	}
	return accountDeletion(user.DeletionScheduledFor, rows), nil
}

// ScheduleAccountDeletion schedules the user's account for deletion after
// the grace period, confirming with their email address. It is refused
// while the user is the only owner of a workspace with other members. The
// user is emailed; they can keep signing in and cancel until then.
func (s *Service) ScheduleAccountDeletion(ctx context.Context, userID uuid.UUID, confirmEmail string) (AccountDeletion, error) {
	user, err := s.q.GetUser(ctx, userID)
	if err != nil {
		return AccountDeletion{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(confirmEmail), user.Email) {
		return AccountDeletion{}, finance.Invalid("email", "must match your email address")
	}
	if user.DeletionScheduledFor != nil {
		return AccountDeletion{}, finance.Conflict("your account is already scheduled for deletion")
	}
	rows, err := s.q.ListUserWorkspacesForDeletion(ctx, userID)
	if err != nil {
		return AccountDeletion{}, err
	}
	plan := accountDeletion(nil, rows)
	if blocking := plan.Blocking(); len(blocking) > 0 {
		return AccountDeletion{}, blockedError(blocking)
	}
	when := s.now().Add(s.cfg.DeletionGracePeriod)
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		scheduled, err := q.ScheduleUserDeletion(ctx, store.ScheduleUserDeletionParams{ID: userID, DeletionScheduledFor: &when})
		if errors.Is(err, pgx.ErrNoRows) {
			return finance.Conflict("your account is already scheduled for deletion")
		}
		if err != nil {
			return err
		}
		plan.ScheduledFor = scheduled.DeletionScheduledFor
		// Sent inside the transaction so a failed email schedules nothing.
		msg, err := mailer.RenderAccountDeletionScheduled(mailer.AccountDeletionEmail{
			To: user.Email, Name: user.Name, Locale: user.Locale,
			Date:       when.In(s.cfg.Location),
			Workspaces: plan.deleted(),
			Link:       s.link("/settings/security"),
		})
		if err != nil {
			return err
		}
		if err := s.mail.Send(ctx, msg); err != nil {
			return fmt.Errorf("send account deletion email: %w", err)
		}
		return nil
	})
	if err != nil {
		return AccountDeletion{}, err
	}
	return plan, nil
}

// CancelAccountDeletion cancels the scheduled deletion of the user's
// account.
func (s *Service) CancelAccountDeletion(ctx context.Context, userID uuid.UUID) error {
	n, err := s.q.CancelUserDeletion(ctx, userID)
	if err != nil {
		return err
	}
	if n == 0 {
		return finance.NotFound("scheduled deletion")
	}
	return nil
}

// deleteNextDueAccount deletes one account whose grace period is over and
// emails its user. It reports whether an account was processed.
//
// Under locks on the user's workspaces it removes the user from each of
// them: workspaces where they are the only member are deleted, and in the
// others their accounts become shared. Then the user is deleted, which ends
// their sessions and connected apps. If meanwhile they became the only
// owner of a workspace with other members, nothing is deleted: the
// scheduled deletion is cancelled and the user is told why.
func (s *Service) deleteNextDueAccount(ctx context.Context) (bool, error) {
	var (
		user  store.User
		plan  AccountDeletion
		found bool
	)
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		var err error
		user, err = q.ClaimDueUserDeletion(ctx, ptr(s.now()))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		if err := q.LockUserWorkspaces(ctx, user.ID); err != nil {
			return err
		}
		rows, err := q.ListUserWorkspacesForDeletion(ctx, user.ID)
		if err != nil {
			return err
		}
		plan = accountDeletion(user.DeletionScheduledFor, rows)
		if len(plan.Blocking()) > 0 {
			_, err := q.CancelUserDeletion(ctx, user.ID)
			return err
		}
		for _, w := range plan.Workspaces {
			if w.Outcome == OutcomeDelete {
				if err := deleteWorkspace(ctx, q, w.ID); err != nil {
					return err
				}
				continue
			}
			// Accounts are isolated per workspace: act on this one to
			// rename the user's accounts that would clash once shared.
			if err := db.ActOnWorkspace(ctx, tx, w.ID); err != nil {
				return err
			}
			if err := q.RenameClashingMemberAccounts(ctx, &user.ID); err != nil {
				return err
			}
			if _, err := q.RemoveWorkspaceMember(ctx, store.RemoveWorkspaceMemberParams{WorkspaceID: w.ID, UserID: user.ID}); err != nil {
				return err
			}
		}
		_, err = q.DeleteUser(ctx, user.ID)
		return err
	})
	if err != nil || !found {
		return false, err
	}

	email := mailer.AccountDeletionEmail{To: user.Email, Name: user.Name, Locale: user.Locale}
	var msg mailer.Message
	if blocking := plan.Blocking(); len(blocking) > 0 {
		email.Workspaces = blocking
		email.Link = s.link("/settings/workspace")
		msg, err = mailer.RenderAccountDeletionBlocked(email)
	} else {
		email.Workspaces = plan.deleted()
		msg, err = mailer.RenderAccountDeleted(email)
	}
	if err == nil {
		err = s.mail.Send(ctx, msg)
	}
	if err != nil {
		return true, fmt.Errorf("send account deletion email: %w", err)
	}
	return true, nil
}
