package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

// ownerDisplay names a workspace member: their name, or their email when
// they have none.
func ownerDisplay(u *finance.UserRef) string {
	if u == nil {
		return ""
	}
	if strings.TrimSpace(u.Name) != "" {
		return u.Name
	}
	return u.Email
}

// accountLabel names an account the way the model can refer to it: owned
// accounts carry their owner, e.g. "BNP (Antonio)", shared ones just a name.
func accountLabel(a finance.Account) string {
	return ownedName(a.Name, a.Owner)
}

func ownedName(name string, owner *finance.UserRef) string {
	if owner == nil {
		return name
	}
	return fmt.Sprintf("%s (%s)", name, ownerDisplay(owner))
}

// ownerLabel describes who an account belongs to for tool outputs.
func ownerLabel(owner *finance.UserRef) string {
	if owner == nil {
		return finance.OwnerShared
	}
	return ownerDisplay(owner)
}

// resolveOwner turns an owner reference from the model ("me", "shared", a
// member's name, email or id) into the value the finance service expects.
// An empty reference stays empty, so the service applies its default.
func (s *Server) resolveOwner(ctx context.Context, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	switch strings.ToLower(ref) {
	case "":
		return "", nil
	case finance.OwnerMe, finance.OwnerShared:
		return strings.ToLower(ref), nil
	}
	members, err := s.finance.ListMembers(ctx)
	if err != nil {
		return "", err
	}
	var partial []finance.UserRef
	for _, m := range members {
		if m.ID.String() == ref || strings.EqualFold(m.Email, ref) || strings.EqualFold(m.Name, ref) {
			return m.ID.String(), nil
		}
		if m.Name != "" && strings.Contains(strings.ToLower(m.Name), strings.ToLower(ref)) {
			partial = append(partial, m)
		}
	}
	if len(partial) == 1 {
		return partial[0].ID.String(), nil
	}
	names := make([]string, len(members))
	for i := range members {
		names[i] = fmt.Sprintf("%s <%s>", ownerDisplay(&members[i]), members[i].Email)
	}
	return "", fmt.Errorf("no workspace member matches owner %q; use \"me\", \"shared\" or one of: %s", ref, strings.Join(names, ", "))
}

// ownerFilter resolves an owner reference into a filter; empty matches
// every owner.
func (s *Server) ownerFilter(ctx context.Context, ref string) (finance.OwnerFilter, error) {
	owner, err := s.resolveOwner(ctx, ref)
	if err != nil {
		return finance.OwnerFilter{}, err
	}
	filter, err := finance.ParseOwnerFilter(ctx, "owner", owner)
	return filter, friendly(err)
}

// preferMine narrows accounts sharing a name to the one owned by the user
// making the call, so "BNP" means their own BNP when each member has one.
func preferMine(ctx context.Context, accounts []finance.Account) []finance.Account {
	actor, ok := finance.ActorFrom(ctx)
	if !ok || len(accounts) < 2 {
		return accounts
	}
	var mine []finance.Account
	for _, a := range accounts {
		if a.Owner != nil && a.Owner.ID == actor {
			mine = append(mine, a)
		}
	}
	if len(mine) == 0 {
		return accounts
	}
	return mine
}

// ambiguousAccounts explains how to pick one of several accounts with the
// same name.
func ambiguousAccounts(ref string, accounts []finance.Account) error {
	names := make([]string, len(accounts))
	for i, a := range accounts {
		names[i] = fmt.Sprintf("%s [%s, id %s]", a.Name, ownerLabel(a.Owner), a.ID)
	}
	return fmt.Errorf("several accounts match %q: %s; ask the user which one and pass its id or name with the owner, e.g. %q", ref, strings.Join(names, ", "), accountLabel(accounts[0]))
}
