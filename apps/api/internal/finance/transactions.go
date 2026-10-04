package finance

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

const (
	TypeExpense  = "expense"
	TypeIncome   = "income"
	TypeTransfer = "transfer"

	maxDescriptionLength = 500
	DefaultPageSize      = 50
	MaxPageSize          = 200
)

type UserRef struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email"`
}

type Transaction struct {
	ID                     uuid.UUID  `json:"id"`
	Type                   string     `json:"type" enum:"expense,income,transfer"`
	AccountID              uuid.UUID  `json:"account_id" format:"uuid"`
	AccountName            string     `json:"account_name"`
	Currency               string     `json:"currency" example:"MXN"`
	MinorUnits             int        `json:"minor_units" example:"2"`
	Amount                 int64      `json:"amount" doc:"Positive amount in minor units of the account currency"`
	DestinationAccountID   *uuid.UUID `json:"destination_account_id" format:"uuid" nullable:"true"`
	DestinationAccountName *string    `json:"destination_account_name"`
	DestinationCurrency    *string    `json:"destination_currency"`
	DestinationMinorUnits  *int       `json:"destination_minor_units"`
	DestinationAmount      *int64     `json:"destination_amount" doc:"Amount received by the destination account (transfers only)"`
	CategoryID             *uuid.UUID `json:"category_id" format:"uuid" nullable:"true"`
	CategoryName           *string    `json:"category_name"`
	Description            string     `json:"description"`
	OccurredOn             string     `json:"occurred_on" format:"date" example:"2026-10-04"`
	CreatedBy              *UserRef   `json:"created_by,omitempty" doc:"Omitted when the creator is unknown"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// TransactionInput is used both to create a transaction and to fully replace
// an existing one.
type TransactionInput struct {
	Type                 string     `json:"type" enum:"expense,income,transfer"`
	AccountID            uuid.UUID  `json:"account_id" format:"uuid" doc:"Source account (the account charged for expenses and transfers, credited for income)"`
	Amount               int64      `json:"amount" minimum:"1" doc:"Positive amount in minor units of the account currency"`
	DestinationAccountID *uuid.UUID `json:"destination_account_id,omitempty" format:"uuid" doc:"Required for transfers"`
	DestinationAmount    *int64     `json:"destination_amount,omitempty" minimum:"1" doc:"Amount received by the destination account. Required for transfers between currencies; defaults to amount otherwise"`
	CategoryID           *uuid.UUID `json:"category_id,omitempty" format:"uuid" doc:"Category whose kind matches the type (not allowed for transfers)"`
	Description          string     `json:"description,omitempty" maxLength:"500"`
	OccurredOn           string     `json:"occurred_on,omitempty" format:"date" doc:"Defaults to today"`
}

type TransactionFilter struct {
	AccountID  *uuid.UUID
	CategoryID *uuid.UUID
	Type       *string
	From       *string
	To         *string
	Search     *string
	Limit      int
	Offset     int
}

type TransactionPage struct {
	Items  []Transaction `json:"items"`
	Total  int64         `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

func transactionFromRow(r store.GetTransactionRow) Transaction {
	t := Transaction{
		ID:                     r.ID,
		Type:                   r.Type,
		AccountID:              r.AccountID,
		AccountName:            r.AccountName,
		Currency:               r.Currency,
		MinorUnits:             int(r.MinorUnits),
		Amount:                 r.Amount,
		DestinationAccountID:   r.DestinationAccountID,
		DestinationAccountName: r.DestinationAccountName,
		DestinationCurrency:    r.DestinationCurrency,
		DestinationAmount:      r.DestinationAmount,
		CategoryID:             r.CategoryID,
		CategoryName:           r.CategoryName,
		Description:            r.Description,
		OccurredOn:             formatDate(r.OccurredOn),
		CreatedAt:              r.CreatedAt,
		UpdatedAt:              r.UpdatedAt,
	}
	if r.DestinationMinorUnits != nil {
		units := int(*r.DestinationMinorUnits)
		t.DestinationMinorUnits = &units
	}
	if r.CreatedBy != nil {
		t.CreatedBy = &UserRef{ID: *r.CreatedBy, Name: deref(r.CreatedByName), Email: deref(r.CreatedByEmail)}
	}
	return t
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

func (s *Service) GetTransaction(ctx context.Context, id uuid.UUID) (Transaction, error) {
	row, err := s.q.GetTransaction(ctx, id)
	if isNoRows(err) {
		return Transaction{}, NotFound("transaction")
	}
	if err != nil {
		return Transaction{}, err
	}
	return transactionFromRow(row), nil
}

func (s *Service) ListTransactions(ctx context.Context, f TransactionFilter) (TransactionPage, error) {
	params := store.ListTransactionsParams{
		AccountID:  f.AccountID,
		CategoryID: f.CategoryID,
		Type:       f.Type,
		RowLimit:   int32(DefaultPageSize),
		RowOffset:  int32(f.Offset),
	}
	if f.Limit > 0 {
		params.RowLimit = int32(min(f.Limit, MaxPageSize))
	}
	if f.Offset < 0 {
		return TransactionPage{}, Invalid("offset", "must not be negative")
	}
	if f.Type != nil && !validType(*f.Type) {
		return TransactionPage{}, Invalid("type", "must be expense, income or transfer")
	}
	if f.From != nil {
		d, err := parseDate("from", *f.From)
		if err != nil {
			return TransactionPage{}, err
		}
		params.FromDate = &d
	}
	if f.To != nil {
		d, err := parseDate("to", *f.To)
		if err != nil {
			return TransactionPage{}, err
		}
		params.ToDate = &d
	}
	if f.Search != nil {
		if q := strings.TrimSpace(*f.Search); q != "" {
			// Escape LIKE wildcards so user input is matched literally.
			q = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q)
			params.Search = &q
		}
	}

	rows, err := s.q.ListTransactions(ctx, params)
	if err != nil {
		return TransactionPage{}, err
	}
	page := TransactionPage{Items: make([]Transaction, len(rows)), Limit: int(params.RowLimit), Offset: f.Offset}
	for i, r := range rows {
		page.Total = r.TotalCount
		page.Items[i] = transactionFromRow(store.GetTransactionRow{
			ID: r.ID, Type: r.Type, AccountID: r.AccountID, Amount: r.Amount,
			DestinationAccountID: r.DestinationAccountID, DestinationAmount: r.DestinationAmount,
			CategoryID: r.CategoryID, Description: r.Description, OccurredOn: r.OccurredOn,
			CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			AccountName: r.AccountName, Currency: r.Currency, MinorUnits: r.MinorUnits,
			DestinationAccountName: r.DestinationAccountName, DestinationCurrency: r.DestinationCurrency,
			DestinationMinorUnits: r.DestinationMinorUnits, CategoryName: r.CategoryName,
			CreatedByName: r.CreatedByName, CreatedByEmail: r.CreatedByEmail,
		})
	}
	return page, nil
}

func validType(t string) bool {
	return t == TypeExpense || t == TypeIncome || t == TypeTransfer
}

// resolved holds validated, normalized values ready to be written.
type resolved struct {
	typ                  string
	accountID            uuid.UUID
	amount               int64
	destinationAccountID *uuid.UUID
	destinationAmount    *int64
	categoryID           *uuid.UUID
	description          string
	occurredOn           time.Time
}

// validate checks a transaction input against the business rules. existing
// is the transaction being replaced (nil on create); archived accounts and
// categories already referenced by it remain allowed so old records can be
// edited.
func (s *Service) validate(ctx context.Context, in TransactionInput, existing *store.Transaction) (resolved, error) {
	r := resolved{typ: in.Type, accountID: in.AccountID, amount: in.Amount}

	if !validType(in.Type) {
		return r, Invalid("type", "must be expense, income or transfer")
	}
	if in.Amount <= 0 {
		return r, Invalid("amount", "must be greater than zero")
	}

	r.description = strings.TrimSpace(in.Description)
	if utf8.RuneCountInString(r.description) > maxDescriptionLength {
		return r, Invalid("description", "must be at most 500 characters")
	}

	if in.OccurredOn == "" {
		r.occurredOn = s.Today()
	} else {
		d, err := parseDate("occurred_on", in.OccurredOn)
		if err != nil {
			return r, err
		}
		r.occurredOn = d
	}

	account, err := s.usableAccount(ctx, "account_id", in.AccountID, existing != nil && existing.AccountID == in.AccountID)
	if err != nil {
		return r, err
	}

	if in.Type == TypeTransfer {
		if in.CategoryID != nil {
			return r, Invalid("category_id", "transfers cannot have a category")
		}
		if in.DestinationAccountID == nil {
			return r, Invalid("destination_account_id", "is required for transfers")
		}
		if *in.DestinationAccountID == in.AccountID {
			return r, Invalid("destination_account_id", "must be different from account_id")
		}
		keep := existing != nil && existing.DestinationAccountID != nil && *existing.DestinationAccountID == *in.DestinationAccountID
		dest, err := s.usableAccount(ctx, "destination_account_id", *in.DestinationAccountID, keep)
		if err != nil {
			return r, err
		}
		destAmount := in.Amount
		switch {
		case dest.Currency == account.Currency && in.DestinationAmount != nil && *in.DestinationAmount != in.Amount:
			return r, Invalid("destination_amount", "must equal amount for transfers in the same currency")
		case dest.Currency != account.Currency && in.DestinationAmount == nil:
			return r, Invalid("destination_amount", "is required for transfers between different currencies")
		case in.DestinationAmount != nil:
			if *in.DestinationAmount <= 0 {
				return r, Invalid("destination_amount", "must be greater than zero")
			}
			destAmount = *in.DestinationAmount
		}
		r.destinationAccountID = in.DestinationAccountID
		r.destinationAmount = &destAmount
		return r, nil
	}

	if in.DestinationAccountID != nil || in.DestinationAmount != nil {
		return r, Invalid("destination_account_id", "only transfers can have a destination account")
	}
	if in.CategoryID != nil {
		category, err := s.q.GetCategory(ctx, *in.CategoryID)
		if isNoRows(err) {
			return r, Invalid("category_id", "category not found")
		}
		if err != nil {
			return r, err
		}
		if category.Kind != in.Type {
			return r, Invalid("category_id", "category kind "+category.Kind+" does not match transaction type "+in.Type)
		}
		keep := existing != nil && existing.CategoryID != nil && *existing.CategoryID == category.ID
		if category.ArchivedAt != nil && !keep {
			return r, Invalid("category_id", "category is archived")
		}
		r.categoryID = &category.ID
	}
	return r, nil
}

func (s *Service) usableAccount(ctx context.Context, field string, id uuid.UUID, allowArchived bool) (store.GetAccountRow, error) {
	account, err := s.q.GetAccount(ctx, id)
	if isNoRows(err) {
		return account, Invalid(field, "account not found")
	}
	if err != nil {
		return account, err
	}
	if account.ArchivedAt != nil && !allowArchived {
		return account, Invalid(field, "account is archived")
	}
	return account, nil
}

func (s *Service) CreateTransaction(ctx context.Context, in TransactionInput) (Transaction, error) {
	r, err := s.validate(ctx, in, nil)
	if err != nil {
		return Transaction{}, err
	}
	params := store.CreateTransactionParams{
		Type:                 r.typ,
		AccountID:            r.accountID,
		Amount:               r.amount,
		DestinationAccountID: r.destinationAccountID,
		DestinationAmount:    r.destinationAmount,
		CategoryID:           r.categoryID,
		Description:          r.description,
		OccurredOn:           r.occurredOn,
	}
	if actor, ok := ActorFrom(ctx); ok {
		params.CreatedBy = &actor
	}
	id, err := s.q.CreateTransaction(ctx, params)
	if err != nil {
		return Transaction{}, err
	}
	return s.GetTransaction(ctx, id)
}

func (s *Service) UpdateTransaction(ctx context.Context, id uuid.UUID, in TransactionInput) (Transaction, error) {
	existing, err := s.q.GetTransactionRecord(ctx, id)
	if isNoRows(err) {
		return Transaction{}, NotFound("transaction")
	}
	if err != nil {
		return Transaction{}, err
	}
	r, err := s.validate(ctx, in, &existing)
	if err != nil {
		return Transaction{}, err
	}
	err = s.q.UpdateTransaction(ctx, store.UpdateTransactionParams{
		ID:                   id,
		Type:                 r.typ,
		AccountID:            r.accountID,
		Amount:               r.amount,
		DestinationAccountID: r.destinationAccountID,
		DestinationAmount:    r.destinationAmount,
		CategoryID:           r.categoryID,
		Description:          r.description,
		OccurredOn:           r.occurredOn,
	})
	if err != nil {
		return Transaction{}, err
	}
	return s.GetTransaction(ctx, id)
}

func (s *Service) DeleteTransaction(ctx context.Context, id uuid.UUID) error {
	affected, err := s.q.DeleteTransaction(ctx, id)
	if err != nil {
		return err
	}
	if affected == 0 {
		return NotFound("transaction")
	}
	return nil
}
