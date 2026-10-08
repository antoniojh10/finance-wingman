package finance

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/money"
)

// Export rows are flat and self-describing: amounts are decimals in the
// currency named next to them (never minor units), references carry the
// name as well as the id, and people are identified by email.

type ExportAccount struct {
	ID             uuid.UUID   `json:"id"`
	Name           string      `json:"name"`
	Type           string      `json:"type"`
	Currency       string      `json:"currency"`
	InitialBalance json.Number `json:"initial_balance"`
	BalanceAsOf    string      `json:"balance_as_of"`
	Balance        json.Number `json:"balance"`
	OwnerEmail     *string     `json:"owner_email"`
	Archived       bool        `json:"archived"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

type ExportCategory struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	Color     *string   `json:"color"`
	Icon      *string   `json:"icon"`
	Archived  bool      `json:"archived"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ExportTransaction struct {
	ID                     uuid.UUID    `json:"id"`
	Type                   string       `json:"type"`
	OccurredOn             string       `json:"occurred_on"`
	AccountID              uuid.UUID    `json:"account_id"`
	AccountName            string       `json:"account_name"`
	Currency               string       `json:"currency"`
	Amount                 json.Number  `json:"amount"`
	DestinationAccountID   *uuid.UUID   `json:"destination_account_id"`
	DestinationAccountName *string      `json:"destination_account_name"`
	DestinationCurrency    *string      `json:"destination_currency"`
	DestinationAmount      *json.Number `json:"destination_amount"`
	CategoryID             *uuid.UUID   `json:"category_id"`
	CategoryName           *string      `json:"category_name"`
	Description            string       `json:"description"`
	RecurringID            *uuid.UUID   `json:"recurring_id"`
	RecurringDueOn         *string      `json:"recurring_due_on"`
	CreatedByEmail         *string      `json:"created_by_email"`
	CreatedAt              time.Time    `json:"created_at"`
	UpdatedAt              time.Time    `json:"updated_at"`
}

type ExportRecurringItem struct {
	ID            uuid.UUID   `json:"id"`
	Name          string      `json:"name"`
	Type          string      `json:"type"`
	AccountID     uuid.UUID   `json:"account_id"`
	AccountName   string      `json:"account_name"`
	Currency      string      `json:"currency"`
	CategoryID    *uuid.UUID  `json:"category_id"`
	CategoryName  *string     `json:"category_name"`
	Amount        json.Number `json:"amount"`
	Notes         string      `json:"notes"`
	IntervalUnit  string      `json:"interval_unit"`
	IntervalCount int         `json:"interval_count"`
	StartOn       string      `json:"start_on"`
	TotalPayments *int        `json:"total_payments"`
	Status        string      `json:"status"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// ExportBudget is a budget row as stored: the amount applies from Month on
// until a later row for the same category and currency replaces it. A null
// amount means the budget was cleared from that month.
type ExportBudget struct {
	CategoryID   uuid.UUID    `json:"category_id"`
	CategoryName string       `json:"category_name"`
	Currency     string       `json:"currency"`
	Month        string       `json:"month" doc:"YYYY-MM"`
	Amount       *json.Number `json:"amount"`
}

// ExportSnapshot holds every entity of the workspace except transactions,
// which can be large and are streamed with ExportTransactions.
type ExportSnapshot struct {
	Accounts   []ExportAccount       `json:"accounts"`
	Categories []ExportCategory      `json:"categories"`
	Recurring  []ExportRecurringItem `json:"recurring_items"`
	Budgets    []ExportBudget        `json:"budgets"`
}

func decimal(amount int64, minorUnits int) json.Number {
	return json.Number(money.Format(amount, minorUnits))
}

func decimalPtr(amount *int64, minorUnits int) *json.Number {
	if amount == nil {
		return nil
	}
	n := decimal(*amount, minorUnits)
	return &n
}

func emailOf(u *UserRef) *string {
	if u == nil {
		return nil
	}
	return &u.Email
}

// ExportSnapshot reads accounts (archived included), categories, recurring
// items and budgets of the workspace in the context.
func (s *Service) ExportSnapshot(ctx context.Context) (ExportSnapshot, error) {
	accounts, err := s.ListAccounts(ctx, true)
	if err != nil {
		return ExportSnapshot{}, err
	}
	categories, err := s.ListCategories(ctx, nil, true)
	if err != nil {
		return ExportSnapshot{}, err
	}
	recurring, err := s.ListRecurringItems(ctx, RecurringFilter{})
	if err != nil {
		return ExportSnapshot{}, err
	}
	budgets, err := s.q.ListAllBudgets(ctx)
	if err != nil {
		return ExportSnapshot{}, err
	}

	snap := ExportSnapshot{
		Accounts:   make([]ExportAccount, len(accounts)),
		Categories: make([]ExportCategory, len(categories)),
		Recurring:  make([]ExportRecurringItem, len(recurring)),
		Budgets:    make([]ExportBudget, len(budgets)),
	}
	for i, a := range accounts {
		snap.Accounts[i] = ExportAccount{
			ID: a.ID, Name: a.Name, Type: a.Type, Currency: a.Currency,
			InitialBalance: decimal(a.InitialBalance, a.MinorUnits),
			BalanceAsOf:    a.BalanceAsOf,
			Balance:        decimal(a.Balance, a.MinorUnits),
			OwnerEmail:     emailOf(a.Owner),
			Archived:       a.Archived, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
		}
	}
	for i, c := range categories {
		snap.Categories[i] = ExportCategory{
			ID: c.ID, Name: c.Name, Kind: c.Kind, Color: c.Color, Icon: c.Icon,
			Archived: c.Archived, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
		}
	}
	for i, r := range recurring {
		snap.Recurring[i] = ExportRecurringItem{
			ID: r.ID, Name: r.Name, Type: r.Type,
			AccountID: r.AccountID, AccountName: r.AccountName, Currency: r.Currency,
			CategoryID: r.CategoryID, CategoryName: r.CategoryName,
			Amount: decimal(r.Amount, r.MinorUnits), Notes: r.Notes,
			IntervalUnit: r.IntervalUnit, IntervalCount: r.IntervalCount,
			StartOn: r.StartOn, TotalPayments: r.TotalPayments, Status: r.Status,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
	}
	for i, b := range budgets {
		snap.Budgets[i] = ExportBudget{
			CategoryID: b.CategoryID, CategoryName: b.CategoryName, Currency: b.Currency,
			Month:  b.Month.Format(monthLayout),
			Amount: decimalPtr(b.AmountMinor, int(b.MinorUnits)),
		}
	}
	return snap, nil
}

// ExportTransactions calls fn with the workspace's transactions, newest
// first, in pages so the whole history is never held in memory.
func (s *Service) ExportTransactions(ctx context.Context, fn func([]ExportTransaction) error) error {
	for offset := 0; ; {
		page, err := s.ListTransactions(ctx, TransactionFilter{Limit: MaxPageSize, Offset: offset})
		if err != nil {
			return err
		}
		if len(page.Items) == 0 {
			return nil
		}
		batch := make([]ExportTransaction, len(page.Items))
		for i, t := range page.Items {
			e := ExportTransaction{
				ID: t.ID, Type: t.Type, OccurredOn: t.OccurredOn,
				AccountID: t.AccountID, AccountName: t.AccountName, Currency: t.Currency,
				Amount:               decimal(t.Amount, t.MinorUnits),
				DestinationAccountID: t.DestinationAccountID, DestinationAccountName: t.DestinationAccountName,
				DestinationCurrency: t.DestinationCurrency,
				CategoryID:          t.CategoryID, CategoryName: t.CategoryName,
				Description: t.Description,
				RecurringID: t.RecurringID, RecurringDueOn: t.RecurringDueOn,
				CreatedByEmail: emailOf(t.CreatedBy),
				CreatedAt:      t.CreatedAt, UpdatedAt: t.UpdatedAt,
			}
			if t.DestinationAmount != nil && t.DestinationMinorUnits != nil {
				e.DestinationAmount = decimalPtr(t.DestinationAmount, *t.DestinationMinorUnits)
			}
			batch[i] = e
		}
		if err := fn(batch); err != nil {
			return err
		}
		offset += len(page.Items)
		if int64(offset) >= page.Total {
			return nil
		}
	}
}
