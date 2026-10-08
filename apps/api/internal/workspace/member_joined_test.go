package workspace_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/mail"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

func memberJoinedMails(f fixture) []mail.Message {
	var out []mail.Message
	for _, m := range f.mail.Messages() {
		if strings.Contains(m.Subject, " joined ") || strings.Contains(m.Subject, " se unió a ") {
			out = append(out, m)
		}
	}
	return out
}

func TestAcceptingNotifiesTheOwnersExceptTheJoiner(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	// A second owner who prefers Spanish; bob stays a plain member.
	dana := addUser(t, f.pool, "dana@example.com")
	if _, err := f.pool.Exec(ctx, `INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'owner')`, f.home, dana); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE users SET locale = 'es' WHERE id = $1`, dana); err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.Invite(ctx, f.ana, f.home, workspace.InviteInput{Email: "carl@example.com", Role: workspace.RoleOwner}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AcceptInvitation(ctx, f.mail.LastInvitationToken(t)); err != nil {
		t.Fatal(err)
	}

	got := map[string]mail.Message{}
	for _, m := range memberJoinedMails(f) {
		got[m.To] = m
	}
	if len(got) != 2 || got["ana@example.com"].Subject == "" || got["dana@example.com"].Subject == "" {
		t.Fatalf("expected one email to each owner other than the joiner: %+v", got)
	}
	en, es := got["ana@example.com"], got["dana@example.com"]
	if en.Subject != "carl@example.com joined Home" || !strings.Contains(en.Text, "Role: Owner.") || !strings.Contains(en.Text, "http://web.test/settings/workspace") {
		t.Fatalf("unexpected English email: %+v", en)
	}
	if es.Subject != "carl@example.com se unió a Home" || !strings.Contains(es.Text, "Rol: Propietario.") {
		t.Fatalf("unexpected Spanish email: %+v", es)
	}
}

func TestNoMemberJoinedEmailForAnExistingMemberOrReusedToken(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()

	// An existing member accepting is not a new member.
	if _, err := f.svc.Invite(ctx, f.ana, f.home, workspace.InviteInput{Email: "eve@example.com"}); err != nil {
		t.Fatal(err)
	}
	token := f.mail.LastInvitationToken(t)
	if _, err := f.pool.Exec(ctx, `INSERT INTO workspace_members (workspace_id, user_id, role) VALUES ($1, $2, 'member')`, f.home, f.outsider); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.AcceptInvitation(ctx, token); err != nil {
		t.Fatal(err)
	}
	// A reused token fails and sends nothing.
	if _, err := f.svc.AcceptInvitation(ctx, token); err == nil {
		t.Fatal("expected the reused token to fail")
	}
	if got := memberJoinedMails(f); len(got) != 0 {
		t.Fatalf("expected no emails: %+v", got)
	}
}

func TestAcceptingSucceedsWhenTheNotificationFails(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()
	if _, err := f.svc.Invite(ctx, f.ana, f.home, workspace.InviteInput{Email: "carl@example.com"}); err != nil {
		t.Fatal(err)
	}
	token := f.mail.LastInvitationToken(t)
	f.mail.Fail = func(mail.Message) error { return errors.New("mail server down") }

	accepted, err := f.svc.AcceptInvitation(ctx, token)
	if err != nil {
		t.Fatalf("a failed notification must not fail the acceptance: %v", err)
	}
	if role(t, f.pool, f.home, accepted.UserID) != workspace.RoleMember {
		t.Fatal("the invitee should be a member")
	}
}
