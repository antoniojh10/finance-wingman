package workspace

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	mailer "github.com/antoniojh10/finance-wingman/apps/api/internal/mail"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

// DefaultDeletionGracePeriod is how long a scheduled deletion can be
// cancelled before it is carried out.
const DefaultDeletionGracePeriod = 7 * 24 * time.Hour

// ScheduleDeletion schedules the workspace for deletion after the grace
// period. Only owners can do it, confirming with the workspace's exact
// name. Every member is emailed. The workspace stays usable until then.
func (s *Service) ScheduleDeletion(ctx context.Context, actorID, workspaceID uuid.UUID, confirmName string) (Workspace, error) {
	if err := s.requireOwner(ctx, actorID, workspaceID); err != nil {
		return Workspace{}, err
	}
	current, err := s.q.GetWorkspace(ctx, workspaceID)
	if err != nil {
		return Workspace{}, err
	}
	if strings.TrimSpace(confirmName) != current.Name {
		return Workspace{}, finance.Invalid("name", "must match the workspace name exactly")
	}
	actor, err := s.q.GetUser(ctx, actorID)
	if err != nil {
		return Workspace{}, err
	}
	when := s.now().Add(s.cfg.DeletionGracePeriod)
	var w store.Workspace
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		var err error
		w, err = q.ScheduleWorkspaceDeletion(ctx, store.ScheduleWorkspaceDeletionParams{
			ID: workspaceID, DeletionScheduledFor: &when, DeletionRequestedBy: &actorID,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return finance.Conflict("the workspace is already scheduled for deletion")
		}
		if err != nil {
			return err
		}
		members, err := q.ListWorkspaceMembers(ctx, workspaceID)
		if err != nil {
			return err
		}
		// Sent inside the transaction so a failed email schedules nothing.
		for _, m := range members {
			msg, err := mailer.RenderWorkspaceDeletionScheduled(mailer.WorkspaceDeletionEmail{
				To: m.Email, Name: m.Name, Locale: m.Locale, WorkspaceName: w.Name,
				RequestedBy: displayName(&actor.Name, &actor.Email),
				Date:        when.In(s.cfg.Location),
				Link:        s.link("/settings/workspace"),
			})
			if err != nil {
				return err
			}
			if err := s.mail.Send(ctx, msg); err != nil {
				return fmt.Errorf("send workspace deletion email: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return Workspace{}, err
	}
	return fromModel(w), nil
}

// CreateDemo makes a workspace owned by the user and marks it as created by
// the demo seed. Only such workspaces can be deleted by DeleteDemoNow.
func (s *Service) CreateDemo(ctx context.Context, userID uuid.UUID, name string) (Workspace, error) {
	w, err := s.Create(ctx, userID, name)
	if err != nil {
		return Workspace{}, err
	}
	if err := s.q.MarkWorkspaceDemo(ctx, w.ID); err != nil {
		_ = pgx.BeginFunc(context.WithoutCancel(ctx), s.pool, func(tx pgx.Tx) error {
			return deleteWorkspace(ctx, s.q.WithTx(tx), w.ID)
		})
		return Workspace{}, err
	}
	return w, nil
}

// FindOwned returns the workspace with that name owned by the user and
// whether the demo seed created it. found is false when there is none.
func (s *Service) FindOwned(ctx context.Context, userID uuid.UUID, name string) (w Workspace, isDemo, found bool, err error) {
	row, err := s.q.FindOwnedWorkspaceByName(ctx, store.FindOwnedWorkspaceByNameParams{UserID: userID, Name: strings.TrimSpace(name)})
	if errors.Is(err, pgx.ErrNoRows) {
		return Workspace{}, false, false, nil
	}
	if err != nil {
		return Workspace{}, false, false, err
	}
	return fromModel(row), row.IsDemo, true, nil
}

// DeleteDemoNow deletes a workspace created by the demo seed with everything
// in it right away: no grace period and no emails. Only its owner can do it,
// and any other workspace is refused, so a real workspace can never be
// hard-deleted through it. It is for maintenance commands, not the HTTP API,
// which schedules deletions instead.
func (s *Service) DeleteDemoNow(ctx context.Context, actorID, workspaceID uuid.UUID) error {
	if err := s.requireOwner(ctx, actorID, workspaceID); err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		w, err := q.GetWorkspace(ctx, workspaceID)
		if err != nil {
			return err
		}
		if !w.IsDemo {
			return finance.Forbidden("only workspaces created by the demo seed can be deleted this way")
		}
		return deleteWorkspace(ctx, q, workspaceID)
	})
}

// CancelDeletion cancels the workspace's scheduled deletion. Only owners
// can cancel.
func (s *Service) CancelDeletion(ctx context.Context, actorID, workspaceID uuid.UUID) error {
	if err := s.requireOwner(ctx, actorID, workspaceID); err != nil {
		return err
	}
	n, err := s.q.CancelWorkspaceDeletion(ctx, workspaceID)
	if err != nil {
		return err
	}
	if n == 0 {
		return finance.NotFound("scheduled deletion")
	}
	return nil
}

// ExecuteScheduledDeletions carries out the workspace and account deletions
// whose grace period is over. It is meant to run periodically; several API
// instances can run it at once. Each deletion commits on its own, so a
// failure leaves the others done; email failures are reported after every
// deletion has run.
func (s *Service) ExecuteScheduledDeletions(ctx context.Context) error {
	var errs []error
	for _, next := range []func(context.Context) (bool, error){s.deleteNextDueWorkspace, s.deleteNextDueAccount} {
		for {
			done, err := next(ctx)
			if err != nil {
				errs = append(errs, err)
			}
			if !done {
				break
			}
		}
	}
	return errors.Join(errs...)
}

// deleteNextDueWorkspace deletes one workspace whose grace period is over
// and emails its members. It reports whether a workspace was deleted.
func (s *Service) deleteNextDueWorkspace(ctx context.Context) (bool, error) {
	var (
		w       store.Workspace
		members []store.ListWorkspaceMembersRow
		found   bool
	)
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		var err error
		w, err = q.ClaimDueWorkspaceDeletion(ctx, ptr(s.now()))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		if members, err = q.ListWorkspaceMembers(ctx, w.ID); err != nil {
			return err
		}
		return deleteWorkspace(ctx, q, w.ID)
	})
	if err != nil || !found {
		return false, err
	}
	var errs []error
	for _, m := range members {
		msg, err := mailer.RenderWorkspaceDeleted(mailer.WorkspaceDeletionEmail{
			To: m.Email, Name: m.Name, Locale: m.Locale, WorkspaceName: w.Name,
		})
		if err == nil {
			err = s.mail.Send(ctx, msg)
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("send workspace deleted email: %w", err))
		}
	}
	return true, errors.Join(errs...)
}

// deleteWorkspace deletes a workspace with everything in it: finance data,
// members, invitations and connected apps (through ON DELETE CASCADE), and
// the access tokens of those apps. Browser sessions acting on it stop acting
// on any workspace.
func deleteWorkspace(ctx context.Context, q *store.Queries, workspaceID uuid.UUID) error {
	if err := q.DeleteWorkspaceAppSessions(ctx, &workspaceID); err != nil {
		return err
	}
	_, err := q.DeleteWorkspace(ctx, workspaceID)
	return err
}

func (s *Service) link(path string) string {
	return strings.TrimRight(s.cfg.WebBaseURL, "/") + path
}

func ptr[T any](v T) *T { return &v }
