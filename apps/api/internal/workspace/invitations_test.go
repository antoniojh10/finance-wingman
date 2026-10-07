package workspace_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

func TestInviteAndAccept(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()

	inv, err := f.svc.Invite(ctx, f.ana, f.home, workspace.InviteInput{Email: " Carl@Example.com ", Locale: "es"})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Email != "carl@example.com" || inv.Role != workspace.RoleMember || inv.InvitedBy != "ana@example.com" {
		t.Fatalf("unexpected invitation: %+v", inv)
	}
	msgs := f.mail.Messages()
	if len(msgs) != 1 || msgs[0].To != "carl@example.com" || !strings.Contains(msgs[0].Subject, "Home") || !strings.Contains(msgs[0].Text, "te invitó") {
		t.Fatalf("unexpected email: %+v", msgs)
	}
	token := f.mail.LastInvitationToken(t)

	list, err := f.svc.Invitations(ctx, f.ana, f.home)
	if err != nil || len(list) != 1 || list[0].ID != inv.ID {
		t.Fatalf("expected the open invitation to be listed: %+v %v", list, err)
	}
	preview, err := f.svc.PreviewInvitation(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if preview.WorkspaceName != "Home" || preview.Email != "carl@example.com" || preview.InvitedBy != "ana@example.com" {
		t.Fatalf("unexpected preview: %+v", preview)
	}

	accepted, err := f.svc.AcceptInvitation(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.WorkspaceID != f.home || role(t, f.pool, f.home, accepted.UserID) != workspace.RoleMember {
		t.Fatalf("the invitee should be a member: %+v", accepted)
	}

	// Single use.
	_, err = f.svc.AcceptInvitation(ctx, token)
	expectKind(t, err, finance.KindNotFound)
	_, err = f.svc.PreviewInvitation(ctx, token)
	expectKind(t, err, finance.KindNotFound)
	if list, _ := f.svc.Invitations(ctx, f.ana, f.home); len(list) != 0 {
		t.Fatalf("accepted invitations are no longer open: %+v", list)
	}
}

func TestInvitingAgainReplacesTheOpenInvitation(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.svc.Invite(ctx, f.ana, f.home, workspace.InviteInput{Email: "carl@example.com"}); err != nil {
		t.Fatal(err)
	}
	first := f.mail.LastInvitationToken(t)
	if _, err := f.svc.Invite(ctx, f.ana, f.home, workspace.InviteInput{Email: "carl@example.com", Role: workspace.RoleOwner}); err != nil {
		t.Fatal(err)
	}
	second := f.mail.LastInvitationToken(t)

	_, err := f.svc.AcceptInvitation(ctx, first)
	expectKind(t, err, finance.KindNotFound)
	accepted, err := f.svc.AcceptInvitation(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if role(t, f.pool, f.home, accepted.UserID) != workspace.RoleOwner {
		t.Fatal("the latest invitation's role should apply")
	}
}

func TestInviteErrors(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()

	_, err := f.svc.Invite(ctx, f.bob, f.home, workspace.InviteInput{Email: "carl@example.com"})
	expectKind(t, err, finance.KindForbidden)
	_, err = f.svc.Invite(ctx, f.outsider, f.home, workspace.InviteInput{Email: "carl@example.com"})
	expectKind(t, err, finance.KindNotFound)
	_, err = f.svc.Invite(ctx, f.ana, f.home, workspace.InviteInput{Email: "not-an-email"})
	expectKind(t, err, finance.KindInvalid)
	_, err = f.svc.Invite(ctx, f.ana, f.home, workspace.InviteInput{Email: "carl@example.com", Role: "admin"})
	expectKind(t, err, finance.KindInvalid)
	_, err = f.svc.Invite(ctx, f.ana, f.home, workspace.InviteInput{Email: "BOB@example.com"})
	expectKind(t, err, finance.KindConflict)
	if len(f.mail.Messages()) != 0 {
		t.Fatal("no email should be sent for rejected invitations")
	}
	_, err = f.svc.Invitations(ctx, f.bob, f.home)
	expectKind(t, err, finance.KindForbidden)
}

func TestInvitationsAreRateLimited(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	svc := workspace.NewService(f.pool, f.mail, workspace.Config{WebBaseURL: "http://web.test", MaxInvitationsPerHour: 2})
	ctx := context.Background()

	for _, email := range []string{"a@example.com", "b@example.com"} {
		if _, err := svc.Invite(ctx, f.ana, f.home, workspace.InviteInput{Email: email}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := svc.Invite(ctx, f.ana, f.home, workspace.InviteInput{Email: "c@example.com"})
	expectKind(t, err, finance.KindConflict)
}

func TestRevokedAndExpiredInvitationsCantBeAccepted(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	now := time.Now()
	f.svc.SetClock(func() time.Time { return now })

	inv, err := f.svc.Invite(ctx, f.ana, f.home, workspace.InviteInput{Email: "carl@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	revoked := f.mail.LastInvitationToken(t)
	expectKind(t, f.svc.RevokeInvitation(ctx, f.bob, f.home, inv.ID), finance.KindForbidden)
	if err := f.svc.RevokeInvitation(ctx, f.ana, f.home, inv.ID); err != nil {
		t.Fatal(err)
	}
	expectKind(t, f.svc.RevokeInvitation(ctx, f.ana, f.home, inv.ID), finance.KindNotFound)
	_, err = f.svc.AcceptInvitation(ctx, revoked)
	expectKind(t, err, finance.KindNotFound)

	if _, err := f.svc.Invite(ctx, f.ana, f.home, workspace.InviteInput{Email: "dana@example.com"}); err != nil {
		t.Fatal(err)
	}
	expiring := f.mail.LastInvitationToken(t)
	now = now.Add(8 * 24 * time.Hour)
	_, err = f.svc.PreviewInvitation(ctx, expiring)
	expectKind(t, err, finance.KindNotFound)
	_, err = f.svc.AcceptInvitation(ctx, expiring)
	expectKind(t, err, finance.KindNotFound)

	if err := f.svc.PurgeExpired(ctx); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM workspace_invitations WHERE email = 'dana@example.com'`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatal("expired invitations should be purged")
	}
}

func TestAcceptingKeepsAnExistingMembersRole(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.svc.Invite(ctx, f.ana, f.home, workspace.InviteInput{Email: "eve@example.com", Role: workspace.RoleOwner}); err != nil {
		t.Fatal(err)
	}
	token := f.mail.LastInvitationToken(t)
	// Eve joins some other way before accepting.
	if _, err := f.pool.Exec(ctx, `INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'member')`, f.home, f.outsider); err != nil {
		t.Fatal(err)
	}
	accepted, err := f.svc.AcceptInvitation(ctx, token)
	if err != nil || accepted.UserID != f.outsider {
		t.Fatalf("accept: %+v %v", accepted, err)
	}
	if role(t, f.pool, f.home, f.outsider) != workspace.RoleMember {
		t.Fatal("an existing member keeps their role")
	}
}
