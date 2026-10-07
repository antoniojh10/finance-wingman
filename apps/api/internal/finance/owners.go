package finance

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// Owner references accepted by account inputs and owner filters, besides a
// member's user id.
const (
	OwnerMe     = "me"
	OwnerShared = "shared"
)

// OwnerFilter narrows accounts, transactions and summaries to the accounts
// of one member or to shared accounts. The zero value matches everything.
type OwnerFilter struct {
	UserID *uuid.UUID
	Shared bool
}

// ParseOwnerFilter reads "me", "shared" or a member's user id. An empty
// value matches every owner.
func ParseOwnerFilter(ctx context.Context, field, value string) (OwnerFilter, error) {
	if strings.TrimSpace(value) == "" {
		return OwnerFilter{}, nil
	}
	owner, err := parseOwner(ctx, field, value)
	if err != nil {
		return OwnerFilter{}, err
	}
	if owner == nil {
		return OwnerFilter{Shared: true}, nil
	}
	return OwnerFilter{UserID: owner}, nil
}

// parseOwner resolves an owner reference to a user id, or nil for shared.
func parseOwner(ctx context.Context, field, value string) (*uuid.UUID, error) {
	switch v := strings.ToLower(strings.TrimSpace(value)); v {
	case OwnerShared:
		return nil, nil
	case OwnerMe:
		actor, ok := ActorFrom(ctx)
		if !ok {
			return nil, Invalid(field, "\"me\" needs an authenticated user")
		}
		return &actor, nil
	default:
		id, err := uuid.Parse(v)
		if err != nil {
			return nil, Invalid(field, "must be \"me\", \"shared\" or a member's user id")
		}
		return &id, nil
	}
}

// defaultOwner is the owner of a new account when none is given: the user
// creating it, or nobody (shared) when there is no authenticated user.
func defaultOwner(ctx context.Context) *uuid.UUID {
	if actor, ok := ActorFrom(ctx); ok {
		return &actor
	}
	return nil
}

// ListMembers lists the members of the current workspace, who can own
// accounts.
func (s *Service) ListMembers(ctx context.Context) ([]UserRef, error) {
	rows, err := s.q.ListAccountOwners(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]UserRef, len(rows))
	for i, r := range rows {
		out[i] = UserRef{ID: r.ID, Name: r.Name, Email: r.Email}
	}
	return out, nil
}
