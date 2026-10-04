// Package auth implements passwordless login (magic link + one-time code)
// and opaque bearer sessions.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	mailer "github.com/antoniojh10/finance-wingman/apps/api/internal/mail"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

var (
	// ErrInvalidCredentials is returned for any failed login attempt. The
	// cause is intentionally not disclosed.
	ErrInvalidCredentials = errors.New("invalid or expired sign-in code")
	ErrUnauthenticated    = errors.New("missing, invalid or expired session")
	ErrInvalidEmail       = errors.New("invalid email address")
	ErrUserNotFound       = errors.New("user not found")
)

const (
	ClientWeb  = "web"
	codeDigits = 6
)

var Locales = []string{"en", "es"}

type Config struct {
	// WebBaseURL is where magic links point to (the Next.js app).
	WebBaseURL           string
	ChallengeTTL         time.Duration
	SessionTTL           time.Duration
	MaxChallengesPerHour int
	MaxCodeAttempts      int
}

func (c Config) withDefaults() Config {
	if c.ChallengeTTL == 0 {
		c.ChallengeTTL = 15 * time.Minute
	}
	if c.SessionTTL == 0 {
		c.SessionTTL = 60 * 24 * time.Hour
	}
	if c.MaxChallengesPerHour == 0 {
		c.MaxChallengesPerHour = 5
	}
	if c.MaxCodeAttempts == 0 {
		c.MaxCodeAttempts = 5
	}
	return c
}

type Service struct {
	q      *store.Queries
	mail   mailer.Sender
	cfg    Config
	logger *slog.Logger
	now    func() time.Time
}

func NewService(pool *pgxpool.Pool, sender mailer.Sender, cfg Config, logger *slog.Logger) *Service {
	return &Service{q: store.New(pool), mail: sender, cfg: cfg.withDefaults(), logger: logger, now: time.Now}
}

// SetClock overrides the time source; intended for tests.
func (s *Service) SetClock(now func() time.Time) { s.now = now }

type User struct {
	ID     uuid.UUID `json:"id"`
	Email  string    `json:"email" format:"email"`
	Name   string    `json:"name"`
	Locale string    `json:"locale" enum:"en,es"`
}

type Session struct {
	ID        uuid.UUID `json:"-"`
	Token     string    `json:"token,omitempty" doc:"Bearer token; only returned when the session is created"`
	Client    string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	User      User      `json:"user"`
}

func userFromModel(u store.User) User {
	return User{ID: u.ID, Email: u.Email, Name: u.Name, Locale: u.Locale}
}

// NormalizeEmail lowercases and validates an email address.
func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", ErrInvalidEmail
	}
	return email, nil
}

// RequestLogin emails a sign-in link and code to a known user. Unknown
// emails and rate-limited requests succeed silently so callers cannot
// discover which addresses have access.
func (s *Service) RequestLogin(ctx context.Context, rawEmail, locale string) error {
	email, err := NormalizeEmail(rawEmail)
	if err != nil {
		return err
	}
	user, err := s.q.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		s.logger.Info("login requested for unknown email")
		return nil
	}
	if err != nil {
		return err
	}

	now := s.now()
	recent, err := s.q.CountRecentChallenges(ctx, store.CountRecentChallengesParams{UserID: user.ID, CreatedAt: now.Add(-time.Hour)})
	if err != nil {
		return err
	}
	if recent >= int64(s.cfg.MaxChallengesPerHour) {
		s.logger.Warn("login rate limit reached", "user_id", user.ID)
		return nil
	}

	token, err := randomToken()
	if err != nil {
		return err
	}
	code, err := randomCode()
	if err != nil {
		return err
	}
	tokenHash := hashToken(token)
	if _, err := s.q.CreateChallenge(ctx, store.CreateChallengeParams{
		UserID:    user.ID,
		TokenHash: tokenHash,
		CodeHash:  hashCode(tokenHash, code),
		ExpiresAt: now.Add(s.cfg.ChallengeTTL),
	}); err != nil {
		return err
	}

	if !isLocale(locale) {
		locale = user.Locale
	}
	msg, err := mailer.RenderLogin(mailer.LoginEmail{
		To:         user.Email,
		Name:       user.Name,
		Locale:     locale,
		Link:       strings.TrimRight(s.cfg.WebBaseURL, "/") + "/auth/verify?token=" + url.QueryEscape(token),
		Code:       code,
		TTLMinutes: int(s.cfg.ChallengeTTL.Minutes()),
	})
	if err != nil {
		return err
	}
	if err := s.mail.Send(ctx, msg); err != nil {
		return fmt.Errorf("send login email: %w", err)
	}
	return nil
}

// VerifyToken redeems the token from a magic link and opens a session.
func (s *Service) VerifyToken(ctx context.Context, token, client string) (Session, error) {
	if token == "" {
		return Session{}, ErrInvalidCredentials
	}
	challenge, err := s.q.GetChallengeByTokenHash(ctx, hashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}
	if challenge.ConsumedAt != nil || !s.now().Before(challenge.ExpiresAt) {
		return Session{}, ErrInvalidCredentials
	}
	return s.redeem(ctx, challenge, client)
}

// VerifyCode redeems the numeric code from the most recent sign-in email.
func (s *Service) VerifyCode(ctx context.Context, rawEmail, code, client string) (Session, error) {
	email, err := NormalizeEmail(rawEmail)
	if err != nil {
		return Session{}, ErrInvalidCredentials
	}
	code = strings.TrimSpace(code)
	if len(code) != codeDigits {
		return Session{}, ErrInvalidCredentials
	}
	user, err := s.q.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}
	challenge, err := s.q.GetLatestOpenChallenge(ctx, store.GetLatestOpenChallengeParams{UserID: user.ID, ExpiresAt: s.now()})
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}

	attempts, err := s.q.IncrementChallengeAttempts(ctx, challenge.ID)
	if err != nil {
		return Session{}, err
	}
	if int(attempts) > s.cfg.MaxCodeAttempts {
		// Too many guesses: burn the challenge so it cannot be brute-forced.
		if _, err := s.q.ConsumeChallenge(ctx, challenge.ID); err != nil {
			return Session{}, err
		}
		return Session{}, ErrInvalidCredentials
	}
	if subtle.ConstantTimeCompare(hashCode(challenge.TokenHash, code), challenge.CodeHash) != 1 {
		return Session{}, ErrInvalidCredentials
	}
	return s.redeem(ctx, challenge, client)
}

func (s *Service) redeem(ctx context.Context, challenge store.LoginChallenge, client string) (Session, error) {
	consumed, err := s.q.ConsumeChallenge(ctx, challenge.ID)
	if err != nil {
		return Session{}, err
	}
	if consumed == 0 {
		return Session{}, ErrInvalidCredentials
	}
	if err := s.q.ConsumeOpenChallenges(ctx, challenge.UserID); err != nil {
		return Session{}, err
	}
	return s.CreateSession(ctx, challenge.UserID, client, s.cfg.SessionTTL)
}

// CreateSession issues a new bearer token for a user.
func (s *Service) CreateSession(ctx context.Context, userID uuid.UUID, client string, ttl time.Duration) (Session, error) {
	if client == "" {
		client = ClientWeb
	}
	user, err := s.q.GetUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrUserNotFound
	}
	if err != nil {
		return Session{}, err
	}
	token, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	row, err := s.q.CreateSession(ctx, store.CreateSessionParams{
		UserID:    userID,
		TokenHash: hashToken(token),
		Client:    client,
		ExpiresAt: s.now().Add(ttl),
	})
	if err != nil {
		return Session{}, err
	}
	return Session{ID: row.ID, Token: token, Client: row.Client, ExpiresAt: row.ExpiresAt, User: userFromModel(user)}, nil
}

// Authenticate resolves a bearer token into its session.
func (s *Service) Authenticate(ctx context.Context, token string) (Session, error) {
	if token == "" {
		return Session{}, ErrUnauthenticated
	}
	row, err := s.q.GetSessionByTokenHash(ctx, hashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrUnauthenticated
	}
	if err != nil {
		return Session{}, err
	}
	now := s.now()
	if !now.Before(row.ExpiresAt) {
		return Session{}, ErrUnauthenticated
	}
	// Avoid a write on every request; hourly precision is enough.
	if now.Sub(row.LastUsedAt) > time.Hour {
		if err := s.q.TouchSession(ctx, row.ID); err != nil {
			s.logger.Warn("touch session", "error", err)
		}
	}
	return Session{
		ID:        row.ID,
		Client:    row.Client,
		ExpiresAt: row.ExpiresAt,
		User:      User{ID: row.UserID, Email: row.Email, Name: row.Name, Locale: row.Locale},
	}, nil
}

func (s *Service) Logout(ctx context.Context, sessionID uuid.UUID) error {
	return s.q.DeleteSession(ctx, sessionID)
}

type UpdateProfileInput struct {
	Name   *string `json:"name,omitempty" maxLength:"100"`
	Locale *string `json:"locale,omitempty" enum:"en,es"`
}

func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, in UpdateProfileInput) (User, error) {
	if in.Locale != nil && !isLocale(*in.Locale) {
		return User{}, fmt.Errorf("unsupported locale %q", *in.Locale)
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		in.Name = &name
	}
	u, err := s.q.UpdateUser(ctx, store.UpdateUserParams{ID: userID, Name: in.Name, Locale: in.Locale})
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, err
	}
	return userFromModel(u), nil
}

// AddUser grants access to an email address (or updates its name).
func (s *Service) AddUser(ctx context.Context, rawEmail, name string) (User, error) {
	email, err := NormalizeEmail(rawEmail)
	if err != nil {
		return User{}, err
	}
	u, err := s.q.UpsertUser(ctx, store.UpsertUserParams{Email: email, Name: strings.TrimSpace(name)})
	if err != nil {
		return User{}, err
	}
	return userFromModel(u), nil
}

func (s *Service) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.q.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	users := make([]User, len(rows))
	for i, u := range rows {
		users[i] = userFromModel(u)
	}
	return users, nil
}

// RemoveUser revokes access; the user's sessions are deleted and their
// transactions are kept without attribution.
func (s *Service) RemoveUser(ctx context.Context, rawEmail string) error {
	email, err := NormalizeEmail(rawEmail)
	if err != nil {
		return err
	}
	n, err := s.q.DeleteUserByEmail(ctx, email)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUserNotFound
	}
	return nil
}

// PurgeExpired deletes expired sessions and login challenges.
func (s *Service) PurgeExpired(ctx context.Context) error {
	return s.q.DeleteExpiredAuthRecords(ctx, s.now())
}

func isLocale(l string) bool {
	for _, known := range Locales {
		if l == known {
			return true
		}
	}
	return false
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func randomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// hashToken returns the SHA-256 digest used to store opaque tokens.
func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// hashCode binds the code to its challenge so equal codes hash differently.
func hashCode(tokenHash []byte, code string) []byte {
	h := sha256.New()
	h.Write(tokenHash)
	h.Write([]byte(code))
	return h.Sum(nil)
}
