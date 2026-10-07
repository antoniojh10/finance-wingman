package finance

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

var AccountTypes = []string{"checking", "savings", "credit_card", "cash", "investment", "other"}

const maxNameLength = 100

type Account struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name" example:"BBVA Checking"`
	Type           string    `json:"type" enum:"checking,savings,credit_card,cash,investment,other"`
	Currency       string    `json:"currency" example:"MXN"`
	MinorUnits     int       `json:"minor_units" example:"2"`
	InitialBalance int64     `json:"initial_balance" doc:"Balance on balance_as_of, in minor units"`
	BalanceAsOf    string    `json:"balance_as_of" format:"date" doc:"Date the initial balance refers to. Only transactions after this day change the balance"`
	Balance        int64     `json:"balance" doc:"Current balance in minor units: initial balance plus transactions dated after balance_as_of"`
	Owner          *UserRef  `json:"owner,omitempty" doc:"Workspace member the account belongs to. Omitted when the account is shared"`
	Archived       bool      `json:"archived"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreateAccountInput struct {
	Name           string `json:"name" minLength:"1" maxLength:"100"`
	Type           string `json:"type" enum:"checking,savings,credit_card,cash,investment,other"`
	Currency       string `json:"currency" minLength:"3" maxLength:"3" example:"MXN" doc:"ISO 4217 currency code"`
	InitialBalance int64  `json:"initial_balance,omitempty" doc:"Balance on balance_as_of in minor units (may be negative, e.g. credit cards)"`
	BalanceAsOf    string `json:"balance_as_of,omitempty" format:"date" doc:"Date the initial balance refers to (YYYY-MM-DD). Defaults to today. Transactions on or before it are already included in the initial balance"`
	Owner          string `json:"owner,omitempty" example:"me" doc:"\"me\", \"shared\" or the user id of a workspace member. Defaults to the user creating the account"`
}

type UpdateAccountInput struct {
	Name           *string `json:"name,omitempty" minLength:"1" maxLength:"100"`
	Type           *string `json:"type,omitempty" enum:"checking,savings,credit_card,cash,investment,other"`
	InitialBalance *int64  `json:"initial_balance,omitempty" doc:"Balance on balance_as_of, in minor units"`
	BalanceAsOf    *string `json:"balance_as_of,omitempty" format:"date" doc:"New anchor date (YYYY-MM-DD). Editing initial_balance alone keeps the current anchor"`
	Owner          *string `json:"owner,omitempty" example:"shared" doc:"\"me\", \"shared\" or the user id of a workspace member"`
	Archived       *bool   `json:"archived,omitempty"`
}

func accountFromRow(r store.GetAccountRow) Account {
	a := Account{
		ID:             r.ID,
		Name:           r.Name,
		Type:           r.Type,
		Currency:       r.Currency,
		MinorUnits:     int(r.MinorUnits),
		InitialBalance: r.InitialBalance,
		BalanceAsOf:    formatDate(r.BalanceAsOf),
		Balance:        r.Balance,
		Archived:       r.ArchivedAt != nil,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
	a.Owner = userRef(r.OwnerUserID, r.OwnerName, r.OwnerEmail)
	return a
}

func (s *Service) ListAccounts(ctx context.Context, includeArchived bool) ([]Account, error) {
	return s.ListOwnedAccounts(ctx, includeArchived, OwnerFilter{})
}

// ListOwnedAccounts lists the accounts matching an owner filter.
func (s *Service) ListOwnedAccounts(ctx context.Context, includeArchived bool, owner OwnerFilter) ([]Account, error) {
	rows, err := s.q.ListAccounts(ctx, store.ListAccountsParams{
		IncludeArchived: includeArchived,
		OwnerID:         owner.UserID,
		SharedOnly:      owner.Shared,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Account, len(rows))
	for i, r := range rows {
		out[i] = accountFromRow(store.GetAccountRow(r))
	}
	return out, nil
}

func (s *Service) GetAccount(ctx context.Context, id uuid.UUID) (Account, error) {
	row, err := s.q.GetAccount(ctx, id)
	if isNoRows(err) {
		return Account{}, NotFound("account")
	}
	if err != nil {
		return Account{}, err
	}
	return accountFromRow(row), nil
}

func normalizeName(field, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", Invalid(field, "must not be empty")
	}
	if len([]rune(name)) > maxNameLength {
		return "", Invalid(field, "must be at most 100 characters")
	}
	return name, nil
}

func validateAccountType(t string) error {
	if !slices.Contains(AccountTypes, t) {
		return Invalid("type", "must be one of "+strings.Join(AccountTypes, ", "))
	}
	return nil
}

func (s *Service) CreateAccount(ctx context.Context, in CreateAccountInput) (Account, error) {
	name, err := normalizeName("name", in.Name)
	if err != nil {
		return Account{}, err
	}
	if err := validateAccountType(in.Type); err != nil {
		return Account{}, err
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if _, err := s.q.GetCurrency(ctx, currency); isNoRows(err) {
		return Account{}, Invalid("currency", "unsupported currency "+in.Currency)
	} else if err != nil {
		return Account{}, err
	}

	owner := defaultOwner(ctx)
	if in.Owner != "" {
		if owner, err = parseOwner(ctx, "owner", in.Owner); err != nil {
			return Account{}, err
		}
	}

	balanceAsOf := s.Today()
	if in.BalanceAsOf != "" {
		if balanceAsOf, err = parseDate("balance_as_of", in.BalanceAsOf); err != nil {
			return Account{}, err
		}
	}

	id, err := s.q.CreateAccount(ctx, store.CreateAccountParams{
		Name:           name,
		Type:           in.Type,
		Currency:       currency,
		InitialBalance: in.InitialBalance,
		BalanceAsOf:    balanceAsOf,
		OwnerUserID:    owner,
	})
	if err := accountWriteError(err); err != nil {
		return Account{}, err
	}
	return s.GetAccount(ctx, id)
}

func (s *Service) UpdateAccount(ctx context.Context, id uuid.UUID, in UpdateAccountInput) (Account, error) {
	if in.Name != nil {
		name, err := normalizeName("name", *in.Name)
		if err != nil {
			return Account{}, err
		}
		in.Name = &name
	}
	if in.Type != nil {
		if err := validateAccountType(*in.Type); err != nil {
			return Account{}, err
		}
	}
	var balanceAsOf *time.Time
	if in.BalanceAsOf != nil {
		d, err := parseDate("balance_as_of", *in.BalanceAsOf)
		if err != nil {
			return Account{}, err
		}
		balanceAsOf = &d
	}
	var owner *uuid.UUID
	if in.Owner != nil {
		var err error
		if owner, err = parseOwner(ctx, "owner", *in.Owner); err != nil {
			return Account{}, err
		}
	}
	if _, err := s.GetAccount(ctx, id); err != nil {
		return Account{}, err
	}

	err := s.q.UpdateAccount(ctx, store.UpdateAccountParams{
		ID:             id,
		Name:           in.Name,
		Type:           in.Type,
		InitialBalance: in.InitialBalance,
		BalanceAsOf:    balanceAsOf,
		SetOwner:       in.Owner != nil,
		OwnerUserID:    owner,
		Archived:       in.Archived,
	})
	if err := accountWriteError(err); err != nil {
		return Account{}, err
	}
	return s.GetAccount(ctx, id)
}

// accountWriteError translates constraint violations of an account insert
// or update into domain errors.
func accountWriteError(err error) error {
	switch pgErrorCode(err) {
	case "":
		return err
	case pgUniqueViolation:
		return Conflict("an active account with this name already exists for this owner")
	case pgForeignKeyViolation:
		return Invalid("owner", "must be a member of the workspace")
	}
	return err
}

// DeleteAccount permanently removes an account. Accounts with transactions
// cannot be deleted; archive them instead.
func (s *Service) DeleteAccount(ctx context.Context, id uuid.UUID) error {
	count, err := s.q.CountAccountTransactions(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return Conflict("account has transactions; archive it instead")
	}
	affected, err := s.q.DeleteAccount(ctx, id)
	if pgErrorCode(err) == pgForeignKeyViolation {
		return Conflict("account has transactions; archive it instead")
	}
	if err != nil {
		return err
	}
	if affected == 0 {
		return NotFound("account")
	}
	return nil
}
