package auth_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
)

type clock struct{ now time.Time }

func (c *clock) Now() time.Time          { return c.now }
func (c *clock) Advance(d time.Duration) { c.now = c.now.Add(d) }

func newService(t *testing.T) (*auth.Service, *testutil.MailRecorder, *clock) {
	t.Helper()
	pool := testutil.NewDatabase(t, true)
	recorder := &testutil.MailRecorder{}
	svc := auth.NewService(pool, recorder, auth.Config{WebBaseURL: "http://web.test"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	c := &clock{now: time.Now()}
	svc.SetClock(c.Now)
	if _, err := svc.AddUser(context.Background(), "ana@example.com", "Ana"); err != nil {
		t.Fatal(err)
	}
	return svc, recorder, c
}

func TestChallengeExpires(t *testing.T) {
	t.Parallel()
	svc, mail, c := newService(t)
	ctx := context.Background()

	if err := svc.RequestLogin(ctx, "ana@example.com", ""); err != nil {
		t.Fatal(err)
	}
	token, code := mail.LastLogin(t)
	c.Advance(16 * time.Minute)

	if _, err := svc.VerifyToken(ctx, token, auth.ClientWeb); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expired token: got %v", err)
	}
	if _, err := svc.VerifyCode(ctx, "ana@example.com", code, auth.ClientWeb); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expired code: got %v", err)
	}
}

func TestCodeAttemptsAreLimited(t *testing.T) {
	t.Parallel()
	svc, mail, _ := newService(t)
	ctx := context.Background()

	if err := svc.RequestLogin(ctx, "ana@example.com", ""); err != nil {
		t.Fatal(err)
	}
	_, code := mail.LastLogin(t)
	wrong := "000000"
	if code == wrong {
		wrong = "999999"
	}
	for i := 0; i < 5; i++ {
		if _, err := svc.VerifyCode(ctx, "ana@example.com", wrong, auth.ClientWeb); !errors.Is(err, auth.ErrInvalidCredentials) {
			t.Fatalf("attempt %d: got %v", i, err)
		}
	}
	// After the limit, even the correct code is rejected.
	if _, err := svc.VerifyCode(ctx, "ana@example.com", code, auth.ClientWeb); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected challenge to be burned, got %v", err)
	}
}

func TestOnlyLatestCodeIsValid(t *testing.T) {
	t.Parallel()
	svc, mail, _ := newService(t)
	ctx := context.Background()

	_ = svc.RequestLogin(ctx, "ana@example.com", "")
	_, first := mail.LastLogin(t)
	_ = svc.RequestLogin(ctx, "ana@example.com", "")
	_, second := mail.LastLogin(t)
	if first == second {
		t.Skip("codes collided by chance")
	}
	if _, err := svc.VerifyCode(ctx, "ana@example.com", first, auth.ClientWeb); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("older code should be rejected, got %v", err)
	}
	if _, err := svc.VerifyCode(ctx, "ana@example.com", second, auth.ClientWeb); err != nil {
		t.Fatalf("latest code should work: %v", err)
	}
}

func TestLoginRequestsAreRateLimited(t *testing.T) {
	t.Parallel()
	svc, mail, c := newService(t)
	ctx := context.Background()

	for i := 0; i < 7; i++ {
		if err := svc.RequestLogin(ctx, "ana@example.com", ""); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(mail.Messages()); n != 5 {
		t.Fatalf("expected 5 emails within an hour, got %d", n)
	}
	c.Advance(61 * time.Minute)
	if err := svc.RequestLogin(ctx, "ana@example.com", ""); err != nil {
		t.Fatal(err)
	}
	if n := len(mail.Messages()); n != 6 {
		t.Fatalf("expected limit to reset after an hour, got %d emails", n)
	}
}

func TestSessionExpires(t *testing.T) {
	t.Parallel()
	svc, _, c := newService(t)
	ctx := context.Background()
	users, _ := svc.ListUsers(ctx)

	session, err := svc.CreateSession(ctx, users[0].ID, auth.ClientWeb, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Authenticate(ctx, session.Token)
	if err != nil || got.User.Email != "ana@example.com" || got.Client != auth.ClientWeb {
		t.Fatalf("Authenticate() = %+v, %v", got, err)
	}
	c.Advance(2 * time.Hour)
	if _, err := svc.Authenticate(ctx, session.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("expected expired session, got %v", err)
	}
	if err := svc.PurgeExpired(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestUserManagement(t *testing.T) {
	t.Parallel()
	svc, mail, _ := newService(t)
	ctx := context.Background()

	if _, err := svc.AddUser(ctx, "BOB@example.com", "Bob"); err != nil {
		t.Fatal(err)
	}
	// Re-adding updates the name but keeps a single user.
	if _, err := svc.AddUser(ctx, "bob@example.com", "Robert"); err != nil {
		t.Fatal(err)
	}
	// An empty name does not erase the existing one.
	if _, err := svc.AddUser(ctx, "bob@example.com", ""); err != nil {
		t.Fatal(err)
	}
	users, err := svc.ListUsers(ctx)
	if err != nil || len(users) != 2 || users[1].Email != "bob@example.com" || users[1].Name != "Robert" {
		t.Fatalf("unexpected users: %+v (%v)", users, err)
	}

	if _, err := svc.AddUser(ctx, "nope", ""); !errors.Is(err, auth.ErrInvalidEmail) {
		t.Fatalf("expected invalid email, got %v", err)
	}

	if err := svc.RemoveUser(ctx, "bob@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := svc.RemoveUser(ctx, "bob@example.com"); !errors.Is(err, auth.ErrUserNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	// Removed users cannot request sign-in emails.
	before := len(mail.Messages())
	_ = svc.RequestLogin(ctx, "bob@example.com", "")
	if len(mail.Messages()) != before {
		t.Fatal("removed user should not receive sign-in emails")
	}
}

func TestNormalizeEmail(t *testing.T) {
	valid := map[string]string{" Ana@Example.COM ": "ana@example.com", "a.b+c@d.io": "a.b+c@d.io"}
	for in, want := range valid {
		if got, err := auth.NormalizeEmail(in); err != nil || got != want {
			t.Errorf("NormalizeEmail(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "ana", "Ana <ana@example.com>", "ana@", "@example.com"} {
		if _, err := auth.NormalizeEmail(in); err == nil {
			t.Errorf("NormalizeEmail(%q) should fail", in)
		}
	}
}
