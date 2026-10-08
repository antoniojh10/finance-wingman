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
	ErrNotMember          = errors.New("not a member of this workspace")
	ErrSessionNotFound    = errors.New("session not found")
)

const (
	ClientWeb  = "web"
	codeDigits = 6
	// maxUserAgentLength bounds the stored User-Agent; real ones are far
	// shorter.
	maxUserAgentLength = 512
)

// Session lifetime defaults. Signing in is a round trip to the inbox, so a
// session lasts long enough not to be a chore for a weekly user, while one
// left on an abandoned device stops working after two idle weeks and none
// lasts more than a month. Sessions keep the expiry they were issued with.
const (
	DefaultSessionTTL         = 30 * 24 * time.Hour
	DefaultSessionIdleTimeout = 14 * 24 * time.Hour
)

var Locales = []string{"en", "es"}

type Config struct {
	// WebBaseURL is where magic links point to (the Next.js app).
	WebBaseURL   string
	ChallengeTTL time.Duration
	// SessionTTL is the absolute lifetime of a session: it ends then even
	// while in use.
	SessionTTL time.Duration
	// SessionIdleTimeout ends a session unused for this long, before its
	// absolute expiry.
	SessionIdleTimeout   time.Duration
	MaxChallengesPerHour int
	MaxCodeAttempts      int
}

func (c Config) withDefaults() Config {
	if c.ChallengeTTL == 0 {
		c.ChallengeTTL = 15 * time.Minute
	}
	if c.SessionTTL == 0 {
		c.SessionTTL = DefaultSessionTTL
	}
	if c.SessionIdleTimeout == 0 {
		c.SessionIdleTimeout = DefaultSessionIdleTimeout
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
	ID     uuid.UUID `json:"-"`
	Token  string    `json:"token,omitempty" doc:"Bearer token; only returned when the session is created"`
	Client string    `json:"-"`
	// OAuthClient is the MCP host behind an OAuth session; nil for web
	// sessions.
	OAuthClient *OAuthClient      `json:"-"`
	ExpiresAt   time.Time         `json:"expires_at"`
	User        User              `json:"user"`
	Workspace   *SessionWorkspace `json:"workspace,omitempty" doc:"The workspace the session acts on; omitted until the user joins or picks one"`
}

// OAuthClient identifies the OAuth client (an MCP host such as Claude) a
// session was issued to. Name is the one the client registered with.
type OAuthClient struct {
	ID   string
	Name string
}

// SessionWorkspace is the workspace a session acts on, with the user's role
// in it.
type SessionWorkspace struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Role string    `json:"role" enum:"owner,member"`
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
	return s.requestLogin(ctx, rawEmail, locale, "")
}

// RequestLoginCode emails only a one-time code (no link), used when a user
// connects an OAuth client such as Claude or ChatGPT. clientName is shown in
// the email so the user knows what they are authorizing.
func (s *Service) RequestLoginCode(ctx context.Context, rawEmail, locale, clientName string) error {
	if clientName == "" {
		clientName = "an app"
	}
	return s.requestLogin(ctx, rawEmail, locale, clientName)
}

func (s *Service) requestLogin(ctx context.Context, rawEmail, locale, oauthClient string) error {
	email, err := NormalizeEmail(rawEmail)
	if err != nil {
		return err
	}
	user, err := s.q.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		s.logger.InfoContext(ctx, "login requested for unknown email")
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
		s.logger.WarnContext(ctx, "login rate limit reached", "user_id", user.ID)
		return nil
	}

	token, err := RandomToken()
	if err != nil {
		return err
	}
	code, err := randomCode()
	if err != nil {
		return err
	}
	tokenHash := HashToken(token)
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
	content := mailer.LoginEmail{
		To:         user.Email,
		Name:       user.Name,
		Locale:     locale,
		Code:       code,
		TTLMinutes: int(s.cfg.ChallengeTTL.Minutes()),
		ClientName: oauthClient,
	}
	if oauthClient == "" {
		content.Link = strings.TrimRight(s.cfg.WebBaseURL, "/") + "/auth/verify?token=" + url.QueryEscape(token)
	}
	msg, err := mailer.RenderLogin(content)
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
	challenge, err := s.q.GetChallengeByTokenHash(ctx, HashToken(token))
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

// VerifyCode redeems the numeric code from the most recent sign-in email
// and opens a session.
func (s *Service) VerifyCode(ctx context.Context, rawEmail, code, client string) (Session, error) {
	challenge, err := s.checkCode(ctx, rawEmail, code)
	if err != nil {
		return Session{}, err
	}
	return s.redeem(ctx, challenge, client)
}

// AuthenticateCode redeems the numeric code and returns the user without
// creating a session (the caller issues its own credentials).
func (s *Service) AuthenticateCode(ctx context.Context, rawEmail, code string) (User, error) {
	challenge, err := s.checkCode(ctx, rawEmail, code)
	if err != nil {
		return User{}, err
	}
	if err := s.consume(ctx, challenge); err != nil {
		return User{}, err
	}
	user, err := s.q.GetUser(ctx, challenge.UserID)
	if err != nil {
		return User{}, err
	}
	return userFromModel(user), nil
}

func (s *Service) checkCode(ctx context.Context, rawEmail, code string) (store.LoginChallenge, error) {
	var none store.LoginChallenge
	email, err := NormalizeEmail(rawEmail)
	if err != nil {
		return none, ErrInvalidCredentials
	}
	code = strings.TrimSpace(code)
	if len(code) != codeDigits {
		return none, ErrInvalidCredentials
	}
	user, err := s.q.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return none, ErrInvalidCredentials
	}
	if err != nil {
		return none, err
	}
	challenge, err := s.q.GetLatestOpenChallenge(ctx, store.GetLatestOpenChallengeParams{UserID: user.ID, ExpiresAt: s.now()})
	if errors.Is(err, pgx.ErrNoRows) {
		return none, ErrInvalidCredentials
	}
	if err != nil {
		return none, err
	}

	attempts, err := s.q.IncrementChallengeAttempts(ctx, challenge.ID)
	if err != nil {
		return none, err
	}
	if int(attempts) > s.cfg.MaxCodeAttempts {
		// Too many guesses: burn the challenge so it cannot be brute-forced.
		if _, err := s.q.ConsumeChallenge(ctx, challenge.ID); err != nil {
			return none, err
		}
		return none, ErrInvalidCredentials
	}
	if subtle.ConstantTimeCompare(hashCode(challenge.TokenHash, code), challenge.CodeHash) != 1 {
		return none, ErrInvalidCredentials
	}
	return challenge, nil
}

func (s *Service) redeem(ctx context.Context, challenge store.LoginChallenge, client string) (Session, error) {
	if err := s.consume(ctx, challenge); err != nil {
		return Session{}, err
	}
	return s.CreateSession(ctx, challenge.UserID, client, s.cfg.SessionTTL)
}

// consume marks the challenge (and any other open challenge of the user) as
// used. It fails if a concurrent request consumed it first.
func (s *Service) consume(ctx context.Context, challenge store.LoginChallenge) error {
	consumed, err := s.q.ConsumeChallenge(ctx, challenge.ID)
	if err != nil {
		return err
	}
	if consumed == 0 {
		return ErrInvalidCredentials
	}
	return s.q.ConsumeOpenChallenges(ctx, challenge.UserID)
}

// CreateSession issues a new bearer token for a user, acting on the
// workspace they used last (or none if they belong to no workspace).
func (s *Service) CreateSession(ctx context.Context, userID uuid.UUID, client string, ttl time.Duration) (Session, error) {
	workspaceID, err := s.q.GetDefaultWorkspaceID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.CreateWorkspaceSession(ctx, userID, nil, client, ttl)
	}
	if err != nil {
		return Session{}, err
	}
	return s.CreateWorkspaceSession(ctx, userID, &workspaceID, client, ttl)
}

// CreateWorkspaceSession issues a new bearer token acting on the given
// workspace, which the user must belong to. A zero ttl uses the configured
// session lifetime.
func (s *Service) CreateWorkspaceSession(ctx context.Context, userID uuid.UUID, workspaceID *uuid.UUID, client string, ttl time.Duration) (Session, error) {
	if client == "" {
		client = ClientWeb
	}
	if ttl == 0 {
		ttl = s.cfg.SessionTTL
	}
	var workspace *SessionWorkspace
	if workspaceID != nil {
		w, err := s.membership(ctx, *workspaceID, userID)
		if err != nil {
			return Session{}, err
		}
		workspace = &w
	}
	user, err := s.q.GetUser(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrUserNotFound
	}
	if err != nil {
		return Session{}, err
	}
	token, err := RandomToken()
	if err != nil {
		return Session{}, err
	}
	now := s.now()
	row, err := s.q.CreateSession(ctx, store.CreateSessionParams{
		UserID:      userID,
		TokenHash:   HashToken(token),
		Client:      client,
		ExpiresAt:   now.Add(ttl),
		WorkspaceID: workspaceID,
		UserAgent:   userAgentFrom(ctx),
		LastUsedAt:  now,
	})
	if err != nil {
		return Session{}, err
	}
	return Session{ID: row.ID, Token: token, Client: row.Client, ExpiresAt: row.ExpiresAt, User: userFromModel(user), Workspace: workspace}, nil
}

// SwitchWorkspace makes the session act on another workspace the user
// belongs to.
func (s *Service) SwitchWorkspace(ctx context.Context, session Session, workspaceID uuid.UUID) (Session, error) {
	w, err := s.membership(ctx, workspaceID, session.User.ID)
	if err != nil {
		return Session{}, err
	}
	if err := s.q.SetSessionWorkspace(ctx, store.SetSessionWorkspaceParams{ID: session.ID, WorkspaceID: &workspaceID}); err != nil {
		return Session{}, err
	}
	session.Workspace = &w
	return session, nil
}

func (s *Service) membership(ctx context.Context, workspaceID, userID uuid.UUID) (SessionWorkspace, error) {
	m, err := s.q.GetMembership(ctx, store.GetMembershipParams{WorkspaceID: workspaceID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return SessionWorkspace{}, ErrNotMember
	}
	if err != nil {
		return SessionWorkspace{}, err
	}
	return SessionWorkspace{ID: m.ID, Name: m.Name, Role: m.Role}, nil
}

// Authenticate resolves a bearer token into its session.
func (s *Service) Authenticate(ctx context.Context, token string) (Session, error) {
	if token == "" {
		return Session{}, ErrUnauthenticated
	}
	row, err := s.q.GetSessionByTokenHash(ctx, HashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrUnauthenticated
	}
	if err != nil {
		return Session{}, err
	}
	now := s.now()
	if !now.Before(row.ExpiresAt) || now.Sub(row.LastUsedAt) > s.cfg.SessionIdleTimeout {
		return Session{}, ErrUnauthenticated
	}
	// Avoid a write on every request; hourly precision is enough.
	if now.Sub(row.LastUsedAt) > time.Hour {
		if err := s.q.TouchSession(ctx, store.TouchSessionParams{ID: row.ID, LastUsedAt: now}); err != nil {
			s.logger.WarnContext(ctx, "touch session", "error", err)
		}
	}
	session := Session{
		ID:        row.ID,
		Client:    row.Client,
		ExpiresAt: row.ExpiresAt,
		User:      User{ID: row.UserID, Email: row.Email, Name: row.Name, Locale: row.Locale},
	}
	if row.ActiveWorkspaceID != nil {
		session.Workspace = &SessionWorkspace{ID: *row.ActiveWorkspaceID, Name: *row.WorkspaceName, Role: *row.WorkspaceRole}
	}
	if row.OauthClientID != nil {
		session.OAuthClient = &OAuthClient{ID: *row.OauthClientID}
		if row.OauthClientName != nil {
			session.OAuthClient.Name = *row.OauthClientName
		}
	}
	return session, nil
}

func (s *Service) Logout(ctx context.Context, sessionID uuid.UUID) error {
	return s.q.DeleteSession(ctx, sessionID)
}

type userAgentKey struct{}

// WithUserAgent records the User-Agent of the request opening a session, so
// the user can recognize it in their list of sessions.
func WithUserAgent(ctx context.Context, userAgent string) context.Context {
	return context.WithValue(ctx, userAgentKey{}, userAgent)
}

func userAgentFrom(ctx context.Context) string {
	ua, _ := ctx.Value(userAgentKey{}).(string)
	ua = strings.ToValidUTF8(strings.TrimSpace(ua), "")
	if len(ua) > maxUserAgentLength {
		ua = strings.ToValidUTF8(ua[:maxUserAgentLength], "")
	}
	return ua
}

// WebSession is one of the user's active sign-ins, as shown in their
// security settings.
type WebSession struct {
	ID         uuid.UUID `json:"id"`
	UserAgent  string    `json:"user_agent" doc:"User-Agent of the browser that signed in; empty when unknown"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at" doc:"Updated at most once an hour"`
	ExpiresAt  time.Time `json:"expires_at"`
	Current    bool      `json:"current" doc:"Whether this is the session making the request"`
}

// ListSessions returns the user's active sign-ins, most recently used
// first. Access tokens of connected apps are not included: those are
// managed as OAuth grants.
func (s *Service) ListSessions(ctx context.Context, userID, currentID uuid.UUID) ([]WebSession, error) {
	now := s.now()
	rows, err := s.q.ListUserSessions(ctx, store.ListUserSessionsParams{
		UserID:    userID,
		Now:       now,
		IdleSince: now.Add(-s.cfg.SessionIdleTimeout),
	})
	if err != nil {
		return nil, err
	}
	sessions := make([]WebSession, len(rows))
	for i, r := range rows {
		sessions[i] = WebSession{
			ID:         r.ID,
			UserAgent:  r.UserAgent,
			CreatedAt:  r.CreatedAt,
			LastUsedAt: r.LastUsedAt,
			ExpiresAt:  r.ExpiresAt,
			Current:    r.ID == currentID,
		}
	}
	return sessions, nil
}

// RevokeSession signs the user out of one of their sessions. Sessions of
// other users, and access tokens of connected apps, are reported as not
// found.
func (s *Service) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	n, err := s.q.DeleteUserSession(ctx, store.DeleteUserSessionParams{ID: sessionID, UserID: userID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// RevokeOtherSessions signs the user out everywhere except the current
// session and returns how many sessions ended. Connected apps stay
// connected.
func (s *Service) RevokeOtherSessions(ctx context.Context, userID, currentID uuid.UUID) (int64, error) {
	return s.q.DeleteOtherUserSessions(ctx, store.DeleteOtherUserSessionsParams{UserID: userID, ID: currentID})
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

// PurgeExpired deletes expired or idle sessions and expired login
// challenges.
func (s *Service) PurgeExpired(ctx context.Context) error {
	now := s.now()
	return s.q.DeleteExpiredAuthRecords(ctx, store.DeleteExpiredAuthRecordsParams{
		Now:       now,
		IdleSince: now.Add(-s.cfg.SessionIdleTimeout),
	})
}

func isLocale(l string) bool {
	for _, known := range Locales {
		if l == known {
			return true
		}
	}
	return false
}

// RandomToken returns a URL-safe random token with 256 bits of entropy.
func RandomToken() (string, error) {
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

// HashToken returns the SHA-256 digest used to store opaque tokens.
func HashToken(token string) []byte {
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
