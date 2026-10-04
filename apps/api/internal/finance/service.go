// Package finance implements the business rules for accounts, categories,
// transactions, and summaries. It is shared by the REST API and the MCP
// server so both enforce exactly the same validation.
package finance

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

const dateLayout = time.DateOnly

type Service struct {
	q   *store.Queries
	loc *time.Location
	now func() time.Time
}

// NewService builds a service. loc is the time zone used to resolve "today"
// and default periods.
func NewService(pool *pgxpool.Pool, loc *time.Location) *Service {
	if loc == nil {
		loc = time.UTC
	}
	return &Service{q: store.New(pool), loc: loc, now: time.Now}
}

// Today returns the current date in the service time zone.
func (s *Service) Today() time.Time {
	y, m, d := s.now().In(s.loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

type actorKey struct{}

// WithActor attaches the authenticated user to the context so created
// records can be attributed to them.
func WithActor(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, actorKey{}, userID)
}

// ActorFrom returns the authenticated user stored in the context, if any.
func ActorFrom(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(actorKey{}).(uuid.UUID)
	return id, ok
}

func parseDate(field, value string) (time.Time, error) {
	d, err := time.Parse(dateLayout, value)
	if err != nil {
		return time.Time{}, Invalid(field, "must be a date in YYYY-MM-DD format")
	}
	return d, nil
}

func formatDate(t time.Time) string {
	return t.Format(dateLayout)
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func pgErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)
