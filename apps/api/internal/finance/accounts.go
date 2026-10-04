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
	InitialBalance int64     `json:"initial_balance" doc:"Opening balance in minor units"`
	Balance        int64     `json:"balance" doc:"Current balance in minor units"`
	Archived       bool      `json:"archived"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreateAccountInput struct {
	Name           string `json:"name" minLength:"1" maxLength:"100"`
	Type           string `json:"type" enum:"checking,savings,credit_card,cash,investment,other"`
	Currency       string `json:"currency" minLength:"3" maxLength:"3" example:"MXN" doc:"ISO 4217 currency code"`
	InitialBalance int64  `json:"initial_balance,omitempty" doc:"Opening balance in minor units (may be negative, e.g. credit cards)"`
}

type UpdateAccountInput struct {
	Name           *string `json:"name,omitempty" minLength:"1" maxLength:"100"`
	Type           *string `json:"type,omitempty" enum:"checking,savings,credit_card,cash,investment,other"`
	InitialBalance *int64  `json:"initial_balance,omitempty"`
	Archived       *bool   `json:"archived,omitempty"`
}

func accountFromRow(r store.GetAccountRow) Account {
	return Account{
		ID:             r.ID,
		Name:           r.Name,
		Type:           r.Type,
		Currency:       r.Currency,
		MinorUnits:     int(r.MinorUnits),
		InitialBalance: r.InitialBalance,
		Balance:        r.Balance,
		Archived:       r.ArchivedAt != nil,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

func (s *Service) ListAccounts(ctx context.Context, includeArchived bool) ([]Account, error) {
	rows, err := s.q.ListAccounts(ctx, includeArchived)
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

	id, err := s.q.CreateAccount(ctx, store.CreateAccountParams{
		Name:           name,
		Type:           in.Type,
		Currency:       currency,
		InitialBalance: in.InitialBalance,
	})
	if pgErrorCode(err) == pgUniqueViolation {
		return Account{}, Conflict("an active account with this name already exists")
	}
	if err != nil {
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
	if _, err := s.GetAccount(ctx, id); err != nil {
		return Account{}, err
	}

	err := s.q.UpdateAccount(ctx, store.UpdateAccountParams{
		ID:             id,
		Name:           in.Name,
		Type:           in.Type,
		InitialBalance: in.InitialBalance,
		Archived:       in.Archived,
	})
	if pgErrorCode(err) == pgUniqueViolation {
		return Account{}, Conflict("an active account with this name already exists")
	}
	if err != nil {
		return Account{}, err
	}
	return s.GetAccount(ctx, id)
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
