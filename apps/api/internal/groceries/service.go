// Package groceries implements the groceries module: stores, the product
// catalog (groups, products, variants), receipts, aliases and price checks.
// It is deliberately independent of the finance package: the only link with
// finance data is the nullable receipt transaction id.
package groceries

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/text/unicode/norm"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

const (
	maxNameLength = 100

	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

// Service holds the groceries business rules. Every method acts on the
// workspace carried by the context (see db.WithWorkspace).
type Service struct {
	pool *pgxpool.Pool
	q    *store.Queries
	fin  *finance.Service
	loc  *time.Location
	now  func() time.Time
}

// NewService builds the service. The finance service is only used through
// its exported API, to read and create the expenses receipts belong to.
func NewService(pool *pgxpool.Pool, fin *finance.Service) *Service {
	return &Service{pool: pool, q: store.New(pool), fin: fin, loc: time.UTC, now: time.Now}
}

// WithLocation sets the time zone used to resolve "today" and the current
// month (UTC by default) and returns the service.
func (s *Service) WithLocation(loc *time.Location) *Service {
	if loc != nil {
		s.loc = loc
	}
	return s
}

// WithClock replaces the clock; meant for tests.
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

// insightsToday returns the current date in the service time zone, at
// midnight UTC so it compares with date columns.
func (s *Service) insightsToday() time.Time {
	y, m, d := s.now().In(s.loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ErrorKind classifies a domain error for the transport layers.
type ErrorKind int

const (
	KindInvalid ErrorKind = iota + 1
	KindNotFound
	KindConflict
)

// Error is a domain error that transport layers translate into a response.
type Error struct {
	Kind    ErrorKind
	Field   string
	Message string
}

func (e *Error) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return e.Message
}

func Invalid(field, message string) *Error {
	return &Error{Kind: KindInvalid, Field: field, Message: message}
}

func NotFound(entity string) *Error {
	return &Error{Kind: KindNotFound, Message: entity + " not found"}
}

func Conflict(message string) *Error {
	return &Error{Kind: KindConflict, Message: message}
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

func normalizeName(field, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", Invalid(field, "must not be empty")
	}
	if len([]rune(name)) > maxNameLength {
		return "", Invalid(field, fmt.Sprintf("must be at most %d characters", maxNameLength))
	}
	return name, nil
}

func emptyToNil(s *string) *string {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// NormalizeRawText turns the text of a receipt line into the key used to
// match aliases: upper case, accents removed, whitespace collapsed.
func NormalizeRawText(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(unicode.ToUpper(r))
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// numericFromFloat converts a decimal with at most 3 fractional digits.
func numericFromFloat(v float64) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	err := n.Scan(strconv.FormatFloat(v, 'f', 3, 64))
	return n, err
}

func floatFromNumeric(n pgtype.Numeric) *float64 {
	if !n.Valid {
		return nil
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return nil
	}
	return &f.Float64
}

// withTx runs fn in a database transaction.
func (s *Service) withTx(ctx context.Context, fn func(q *store.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
