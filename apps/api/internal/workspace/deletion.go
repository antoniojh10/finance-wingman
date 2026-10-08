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

// ExecuteScheduledDeletions carries out the deletions whose grace period is
// over. It is meant to run periodically; several API instances can run it
// at once. Each deletion commits on its own, so a failure leaves the others
// done; email failures are reported after every deletion has run.
func (s *Service) ExecuteScheduledDeletions(ctx context.Context) error {
	var errs []error
	for {
		done, err := s.deleteNextDueWorkspace(ctx)
		if err != nil {
			errs = append(errs, err)
		}
		if !done {
			break
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
