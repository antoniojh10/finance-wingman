package auth_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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

func TestSessionLifetime(t *testing.T) {
	t.Parallel()
	svc, mail, c := newService(t)
	ctx := auth.WithUserAgent(context.Background(), "  Firefox  ")

	if err := svc.RequestLogin(ctx, "ana@example.com", ""); err != nil {
		t.Fatal(err)
	}
	token, _ := mail.LastLogin(t)
	session, err := svc.VerifyToken(ctx, token, auth.ClientWeb)
	if err != nil {
		t.Fatal(err)
	}
	if want := c.Now().Add(auth.DefaultSessionTTL); session.ExpiresAt.Sub(want).Abs() > time.Millisecond {
		t.Fatalf("expires at %v, want %v", session.ExpiresAt, want)
	}
	listed, err := svc.ListSessions(ctx, session.User.ID, session.ID)
	if err != nil || len(listed) != 1 || listed[0].UserAgent != "Firefox" || !listed[0].Current {
		t.Fatalf("ListSessions() = %+v, %v", listed, err)
	}

	// Using the session keeps it alive past the idle timeout...
	for i := 0; i < 2; i++ {
		c.Advance(auth.DefaultSessionIdleTimeout - time.Hour)
		if _, err := svc.Authenticate(ctx, session.Token); err != nil {
			t.Fatalf("active session after %d idle periods: %v", i+1, err)
		}
	}
	// ...but not past its absolute lifetime.
	c.Advance(auth.DefaultSessionTTL - 2*auth.DefaultSessionIdleTimeout + 3*time.Hour)
	if _, err := svc.Authenticate(ctx, session.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("expected the session to expire, got %v", err)
	}

	// An unused session ends after the idle timeout, and is purged.
	idle, err := svc.CreateSession(ctx, session.User.ID, auth.ClientWeb, 0)
	if err != nil {
		t.Fatal(err)
	}
	c.Advance(auth.DefaultSessionIdleTimeout + time.Minute)
	if _, err := svc.Authenticate(ctx, idle.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("expected the idle session to end, got %v", err)
	}
	if listed, err := svc.ListSessions(ctx, session.User.ID, idle.ID); err != nil || len(listed) != 0 {
		t.Fatalf("idle sessions should not be listed: %+v, %v", listed, err)
	}
	if err := svc.PurgeExpired(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeSession(ctx, session.User.ID, idle.ID); !errors.Is(err, auth.ErrSessionNotFound) {
		t.Fatalf("idle session should have been purged, got %v", err)
	}
}

func TestUserAgentIsBounded(t *testing.T) {
	t.Parallel()
	svc, _, _ := newService(t)
	users, _ := svc.ListUsers(context.Background())
	long := strings.Repeat("é", 400) // 800 bytes
	ctx := auth.WithUserAgent(context.Background(), long)
	session, err := svc.CreateSession(ctx, users[0].ID, auth.ClientWeb, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := svc.ListSessions(ctx, users[0].ID, session.ID)
	if err != nil || len(listed) != 1 {
		t.Fatalf("ListSessions() = %+v, %v", listed, err)
	}
	if ua := listed[0].UserAgent; len(ua) > 512 || !utf8.ValidString(ua) || !strings.HasPrefix(long, ua) {
		t.Fatalf("user agent should be truncated to valid UTF-8, got %d bytes", len(ua))
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
