package groceries

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

const (
	dateLayout       = "2006-01-02"
	maxReceiptLines  = 300
	maxRawTextLength = 200
	maxQuantity      = 9999999.0
	defaultSource    = "manual"
	defaultBrand     = "Generic"
	groceriesCatName = "groceries"

	// How a receipt line got its variant.
	MatchAlias = "alias"
	MatchGiven = "given"
	MatchNone  = "none"

	WarningTotalMismatch = "total_mismatch"
	WarningLinesMismatch = "lines_mismatch"
)

// ReceiptSources are the ways a receipt can be entered.
var ReceiptSources = []string{"photo", "digital", "manual"}

// Warning flags a receipt that does not add up. Mismatches are allowed.
type Warning struct {
	Code              string `json:"code" enum:"total_mismatch,lines_mismatch"`
	ReceiptTotal      *int64 `json:"receipt_total,omitempty" doc:"Receipt total in minor units"`
	TransactionAmount *int64 `json:"transaction_amount,omitempty" doc:"Amount of the linked transaction in minor units (total_mismatch)"`
	LinesTotal        *int64 `json:"lines_total,omitempty" doc:"Sum of the line amounts in minor units (lines_mismatch)"`
}

// ReceiptLine is one line of a receipt. Amounts are minor units.
type ReceiptLine struct {
	ID          uuid.UUID  `json:"id"`
	Position    int32      `json:"position"`
	RawText     string     `json:"raw_text" example:"LECHE ENT. HACENDADO 1L"`
	Quantity    float64    `json:"quantity"`
	Amount      int64      `json:"amount" doc:"Paid for the line after the discount, in minor units"`
	Discount    int64      `json:"discount" doc:"Discount already applied to amount, in minor units"`
	VariantID   *uuid.UUID `json:"variant_id" nullable:"true" format:"uuid"`
	ProductID   *uuid.UUID `json:"product_id" nullable:"true" format:"uuid"`
	ProductName *string    `json:"product_name" nullable:"true"`
	Brand       *string    `json:"brand" nullable:"true"`
	PackLabel   *string    `json:"pack_label" nullable:"true"`
	Match       string     `json:"match,omitempty" enum:"alias,given,none" doc:"How the variant was found. Only present in the response to creating a receipt"`
}

// ReceiptSummary is a receipt without its lines.
type ReceiptSummary struct {
	ID              uuid.UUID  `json:"id"`
	StoreID         uuid.UUID  `json:"store_id"`
	StoreName       string     `json:"store_name"`
	TransactionID   *uuid.UUID `json:"transaction_id" nullable:"true" format:"uuid"`
	PurchasedOn     string     `json:"purchased_on" format:"date"`
	Currency        string     `json:"currency" example:"EUR"`
	Total           int64      `json:"total" doc:"Receipt total in minor units"`
	Source          string     `json:"source" enum:"photo,digital,manual"`
	LineCount       int64      `json:"line_count"`
	UnassignedCount int64      `json:"unassigned_count" doc:"Lines without a product variant yet"`
	LinesTotal      int64      `json:"lines_total" doc:"Sum of the line amounts in minor units"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Receipt is a receipt with its lines and warnings.
type Receipt struct {
	ReceiptSummary
	Lines    []ReceiptLine `json:"lines"`
	Warnings []Warning     `json:"warnings" doc:"Mismatches between the receipt, its lines and its transaction"`
}

type ReceiptLineInput struct {
	RawText   string   `json:"raw_text" minLength:"1" maxLength:"200"`
	Quantity  *float64 `json:"quantity,omitempty" exclusiveMinimum:"0" maximum:"9999999" doc:"Up to 3 decimals; defaults to 1"`
	Amount    int64    `json:"amount" minimum:"0" doc:"Paid for the line after the discount, in minor units"`
	Discount  *int64   `json:"discount,omitempty" minimum:"0" doc:"Discount already applied to amount, in minor units"`
	VariantID *string  `json:"variant_id,omitempty" format:"uuid" doc:"Skips the alias lookup"`
}

type CreateExpenseInput struct {
	AccountID  string  `json:"account_id" format:"uuid"`
	CategoryID *string `json:"category_id,omitempty" format:"uuid" doc:"Defaults to the expense category named Groceries, when it exists"`
}

type CreateReceiptInput struct {
	StoreID              *string             `json:"store_id,omitempty" format:"uuid" doc:"Either store_id or store_name"`
	StoreName            *string             `json:"store_name,omitempty" maxLength:"100"`
	CreateStoreIfMissing *bool               `json:"create_store_if_missing,omitempty" doc:"With store_name: create the store when it does not exist. Defaults to true"`
	PurchasedOn          string              `json:"purchased_on" format:"date"`
	Currency             string              `json:"currency" minLength:"3" maxLength:"3" example:"EUR"`
	Total                int64               `json:"total" minimum:"1" doc:"Receipt total in minor units"`
	Source               string              `json:"source,omitempty" enum:"photo,digital,manual" doc:"Defaults to manual"`
	Lines                []ReceiptLineInput  `json:"lines,omitempty" maxItems:"300"`
	TransactionID        *string             `json:"transaction_id,omitempty" format:"uuid" doc:"Existing expense to link. Not combinable with create_expense"`
	CreateExpense        *CreateExpenseInput `json:"create_expense,omitempty" doc:"Create the expense (receipt total, purchase date, store name) and link it"`
}

type UpdateReceiptInput struct {
	StoreID       *string `json:"store_id,omitempty"`
	PurchasedOn   *string `json:"purchased_on,omitempty" format:"date"`
	Total         *int64  `json:"total,omitempty" minimum:"1" doc:"Receipt total in minor units"`
	TransactionID *string `json:"transaction_id,omitempty" doc:"Expense to link; empty string unlinks"`
}

type ListReceiptsParams struct {
	Month          string // YYYY-MM
	StoreID        *uuid.UUID
	UnassignedOnly bool
}

// AssignLineInput maps one receipt line to a product variant: either an
// existing variant_id, or a product (and brand/pack) that is found or
// created by name. With neither the line is unassigned (the alias is kept).
type AssignLineInput struct {
	LineID      string   `json:"line_id" format:"uuid"`
	VariantID   *string  `json:"variant_id,omitempty" nullable:"true" doc:"Existing variant. Omit it and product (or send null) to unassign the line"`
	Product     *string  `json:"product,omitempty" maxLength:"100" doc:"Product name (case-insensitive); created when new"`
	Brand       *string  `json:"brand,omitempty" maxLength:"100" doc:"Defaults to Generic"`
	PackSize    *float64 `json:"pack_size,omitempty" exclusiveMinimum:"0" maximum:"999999999" doc:"In the product's compare unit; omit when sold loose"`
	PackLabel   *string  `json:"pack_label,omitempty" maxLength:"50"`
	CompareUnit *string  `json:"compare_unit,omitempty" enum:"kg,l,unit" doc:"Required when the product is new"`
	Group       *string  `json:"group,omitempty" doc:"Product group name, used when the product is new"`
}

type AssignLinesInput struct {
	Assignments []AssignLineInput `json:"assignments" minItems:"1" maxItems:"300"`
}

// fromFinance translates a finance domain error into a groceries one.
func fromFinance(field string, err error) error {
	var fe *finance.Error
	if !errors.As(err, &fe) {
		return err
	}
	switch fe.Kind {
	case finance.KindConflict:
		return Conflict(fe.Message)
	case finance.KindInvalid:
		if fe.Field != "" {
			field = field + "." + fe.Field
		}
		return Invalid(field, fe.Message)
	case finance.KindNotFound:
		return Invalid(field, fe.Message)
	}
	return err
}

func isFinanceNotFound(err error) bool {
	var fe *finance.Error
	return errors.As(err, &fe) && fe.Kind == finance.KindNotFound
}

func parseRequiredID(field, raw string) (uuid.UUID, error) {
	id, err := optionalID(field, &raw)
	if err != nil {
		return uuid.Nil, err
	}
	if id == nil {
		return uuid.Nil, Invalid(field, "is required")
	}
	return *id, nil
}

func parseDay(field, value string) (time.Time, error) {
	d, err := time.Parse(dateLayout, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, Invalid(field, "must be a date in YYYY-MM-DD format")
	}
	return d, nil
}

func quantityNumeric(field string, q *float64) (pgtype.Numeric, error) {
	v := 1.0
	if q != nil {
		v = *q
	}
	if v <= 0 || v > maxQuantity {
		return pgtype.Numeric{}, Invalid(field, "must be greater than 0")
	}
	return numericFromFloat(v)
}

func lineFromDetailed(r store.ListGroceryReceiptLinesDetailedRow) ReceiptLine {
	qty := 0.0
	if f := floatFromNumeric(r.Quantity); f != nil {
		qty = *f
	}
	return ReceiptLine{
		ID: r.ID, Position: r.Position, RawText: r.RawText, Quantity: qty, Amount: r.AmountMinor,
		Discount: r.DiscountMinor, VariantID: r.VariantID, ProductID: r.ProductID,
		ProductName: r.ProductName, Brand: r.Brand, PackLabel: r.PackLabel,
	}
}

func summaryFromRow(r store.ListGroceryReceiptSummariesRow) ReceiptSummary {
	return ReceiptSummary{
		ID: r.ID, StoreID: r.StoreID, StoreName: r.StoreName, TransactionID: r.TransactionID,
		PurchasedOn: r.PurchasedOn.Format(dateLayout), Currency: r.Currency, Total: r.TotalMinor, Source: r.Source,
		LineCount: r.LineCount, UnassignedCount: r.UnassignedCount, LinesTotal: r.LinesTotal,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func (s *Service) loadReceipt(ctx context.Context, id uuid.UUID, matches map[uuid.UUID]string) (Receipt, error) {
	rows, err := s.q.ListGroceryReceiptSummaries(ctx, store.ListGroceryReceiptSummariesParams{ID: &id})
	if err != nil {
		return Receipt{}, err
	}
	if len(rows) == 0 {
		return Receipt{}, NotFound("receipt")
	}
	lineRows, err := s.q.ListGroceryReceiptLinesDetailed(ctx, id)
	if err != nil {
		return Receipt{}, err
	}
	receipt := Receipt{ReceiptSummary: summaryFromRow(rows[0]), Lines: make([]ReceiptLine, len(lineRows)), Warnings: []Warning{}}
	for i, r := range lineRows {
		receipt.Lines[i] = lineFromDetailed(r)
		receipt.Lines[i].Match = matches[r.ID]
	}
	if receipt.TransactionID != nil {
		tx, err := s.fin.GetTransaction(ctx, *receipt.TransactionID)
		switch {
		case err == nil:
			if tx.Amount != receipt.Total {
				total, amount := receipt.Total, tx.Amount
				receipt.Warnings = append(receipt.Warnings, Warning{Code: WarningTotalMismatch, ReceiptTotal: &total, TransactionAmount: &amount})
			}
		case !isFinanceNotFound(err):
			return Receipt{}, err
		}
	}
	if receipt.LineCount > 0 && receipt.LinesTotal != receipt.Total {
		total, lines := receipt.Total, receipt.LinesTotal
		receipt.Warnings = append(receipt.Warnings, Warning{Code: WarningLinesMismatch, LinesTotal: &lines, ReceiptTotal: &total})
	}
	return receipt, nil
}

func (s *Service) GetReceipt(ctx context.Context, id uuid.UUID) (Receipt, error) {
	return s.loadReceipt(ctx, id, nil)
}

func (s *Service) ListReceipts(ctx context.Context, p ListReceiptsParams) ([]ReceiptSummary, error) {
	params := store.ListGroceryReceiptSummariesParams{StoreID: p.StoreID, UnassignedOnly: p.UnassignedOnly}
	if p.Month != "" {
		first, err := time.Parse("2006-01", p.Month)
		if err != nil {
			return nil, Invalid("month", "must be a month in YYYY-MM format")
		}
		last := first.AddDate(0, 1, -1)
		params.FromDate, params.ToDate = &first, &last
	}
	rows, err := s.q.ListGroceryReceiptSummaries(ctx, params)
	if err != nil {
		return nil, err
	}
	out := make([]ReceiptSummary, len(rows))
	for i, r := range rows {
		out[i] = summaryFromRow(r)
	}
	return out, nil
}

// DeleteReceipt removes a receipt and its lines; its transaction stays.
func (s *Service) DeleteReceipt(ctx context.Context, id uuid.UUID) error {
	n, err := s.q.DeleteGroceryReceipt(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return NotFound("receipt")
	}
	return nil
}

// activeStore returns a store that exists and is not archived.
func (s *Service) activeStore(ctx context.Context, field string, id uuid.UUID) (store.GroceryStore, error) {
	st, err := s.q.GetGroceryStore(ctx, id)
	if isNoRows(err) {
		return st, Invalid(field, "unknown store")
	}
	if err != nil {
		return st, err
	}
	if st.ArchivedAt != nil {
		return st, Invalid(field, "store is archived")
	}
	return st, nil
}

// checkLinkable makes sure a transaction can be linked to a receipt of the
// given currency: an expense in that currency, not linked to another receipt.
func (s *Service) checkLinkable(ctx context.Context, transactionID uuid.UUID, currency string, receiptID *uuid.UUID) error {
	tx, err := s.fin.GetTransaction(ctx, transactionID)
	if err != nil {
		return fromFinance("transaction_id", err)
	}
	if tx.Type != "expense" {
		return Invalid("transaction_id", "must be an expense, not "+tx.Type)
	}
	if tx.Currency != currency {
		return Invalid("transaction_id", fmt.Sprintf("is in %s but the receipt is in %s", tx.Currency, currency))
	}
	other, err := s.q.GetGroceryReceiptByTransaction(ctx, &transactionID)
	if err == nil && (receiptID == nil || other.ID != *receiptID) {
		return Conflict("this transaction is already linked to another receipt")
	}
	if err != nil && !isNoRows(err) {
		return err
	}
	return nil
}

// defaultExpenseCategory returns the expense category named Groceries.
func (s *Service) defaultExpenseCategory(ctx context.Context) (*uuid.UUID, error) {
	kind := "expense"
	categories, err := s.fin.ListCategories(ctx, &kind, false)
	if err != nil {
		return nil, err
	}
	for _, c := range categories {
		if strings.ToLower(strings.TrimSpace(c.Name)) == groceriesCatName {
			id := c.ID
			return &id, nil
		}
	}
	return nil, nil
}

type preparedLine struct {
	rawText   string
	key       string
	quantity  pgtype.Numeric
	amount    int64
	discount  int64
	variantID *uuid.UUID
}

func (s *Service) prepareLines(ctx context.Context, in []ReceiptLineInput) ([]preparedLine, error) {
	if len(in) > maxReceiptLines {
		return nil, Invalid("lines", fmt.Sprintf("must contain at most %d lines", maxReceiptLines))
	}
	out := make([]preparedLine, len(in))
	for i, l := range in {
		field := func(name string) string { return fmt.Sprintf("lines[%d].%s", i, name) }
		raw := strings.TrimSpace(l.RawText)
		if raw == "" || len([]rune(raw)) > maxRawTextLength {
			return nil, Invalid(field("raw_text"), fmt.Sprintf("must be between 1 and %d characters", maxRawTextLength))
		}
		qty, err := quantityNumeric(field("quantity"), l.Quantity)
		if err != nil {
			return nil, err
		}
		if l.Amount < 0 {
			return nil, Invalid(field("amount"), "must not be negative")
		}
		var discount int64
		if l.Discount != nil {
			discount = *l.Discount
		}
		if discount < 0 {
			return nil, Invalid(field("discount"), "must not be negative")
		}
		variantID, err := optionalID(field("variant_id"), l.VariantID)
		if err != nil {
			return nil, err
		}
		if variantID != nil {
			if _, err := s.q.GetProductVariant(ctx, *variantID); err != nil {
				if isNoRows(err) {
					return nil, Invalid(field("variant_id"), "unknown variant")
				}
				return nil, err
			}
		}
		out[i] = preparedLine{rawText: raw, key: NormalizeRawText(raw), quantity: qty, amount: l.Amount, discount: discount, variantID: variantID}
	}
	return out, nil
}

// CreateReceipt records a receipt. Lines without a variant are matched
// through the store's aliases. The expense, when created here, is deleted
// again if the receipt cannot be saved.
func (s *Service) CreateReceipt(ctx context.Context, in CreateReceiptInput) (Receipt, error) {
	purchased, err := parseDay("purchased_on", in.PurchasedOn)
	if err != nil {
		return Receipt{}, err
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if len(currency) != 3 {
		return Receipt{}, Invalid("currency", "must be a 3-letter ISO 4217 code")
	}
	if in.Total < 1 {
		return Receipt{}, Invalid("total", "must be greater than 0")
	}
	source := in.Source
	if source == "" {
		source = defaultSource
	}
	if !slices.Contains(ReceiptSources, source) {
		return Receipt{}, Invalid("source", "must be one of "+strings.Join(ReceiptSources, ", "))
	}

	storeID, err := optionalID("store_id", in.StoreID)
	if err != nil {
		return Receipt{}, err
	}
	storeName := ""
	if in.StoreName != nil {
		storeName = strings.TrimSpace(*in.StoreName)
	}
	if (storeID == nil) == (storeName == "") {
		return Receipt{}, Invalid("store_id", "provide exactly one of store_id or store_name")
	}
	if storeID != nil {
		st, err := s.activeStore(ctx, "store_id", *storeID)
		if err != nil {
			return Receipt{}, err
		}
		storeName = st.Name
	} else {
		if storeName, err = normalizeName("store_name", storeName); err != nil {
			return Receipt{}, err
		}
		if _, err := s.q.FindGroceryStoreByName(ctx, storeName); err != nil {
			if !isNoRows(err) {
				return Receipt{}, err
			}
			if in.CreateStoreIfMissing != nil && !*in.CreateStoreIfMissing {
				return Receipt{}, Invalid("store_name", "unknown store; create it first or set create_store_if_missing")
			}
		}
	}

	lines, err := s.prepareLines(ctx, in.Lines)
	if err != nil {
		return Receipt{}, err
	}

	if in.TransactionID != nil && in.CreateExpense != nil {
		return Receipt{}, Invalid("create_expense", "cannot be combined with transaction_id")
	}
	transactionID, err := optionalID("transaction_id", in.TransactionID)
	if err != nil {
		return Receipt{}, err
	}
	if transactionID != nil {
		if err := s.checkLinkable(ctx, *transactionID, currency, nil); err != nil {
			return Receipt{}, err
		}
	}

	var createdExpense *uuid.UUID
	if in.CreateExpense != nil {
		id, err := s.createExpense(ctx, in.CreateExpense, currency, in.Total, purchased, storeName)
		if err != nil {
			return Receipt{}, err
		}
		createdExpense, transactionID = &id, &id
	}

	var receiptID uuid.UUID
	matches := map[uuid.UUID]string{}
	err = s.withTx(ctx, func(q *store.Queries) error {
		if storeID == nil {
			st, err := q.FindGroceryStoreByName(ctx, storeName)
			if isNoRows(err) {
				st, err = q.CreateGroceryStore(ctx, storeName)
				if pgErrorCode(err) == pgUniqueViolation {
					return Conflict("an active store with this name already exists")
				}
			}
			if err != nil {
				return err
			}
			storeID = &st.ID
		}
		params := store.InsertGroceryReceiptParams{
			StoreID: *storeID, TransactionID: transactionID, PurchasedOn: purchased,
			Currency: currency, TotalMinor: in.Total, Source: source,
		}
		if actor, ok := finance.ActorFrom(ctx); ok {
			params.CreatedBy = &actor
		}
		receipt, err := q.InsertGroceryReceipt(ctx, params)
		if err != nil {
			return mapReceiptWriteError(err)
		}
		receiptID = receipt.ID
		for i, l := range lines {
			variantID, match := l.variantID, MatchGiven
			if variantID == nil {
				match = MatchNone
				alias, err := q.GetStoreItemAlias(ctx, store.GetStoreItemAliasParams{StoreID: *storeID, RawTextKey: l.key})
				if err == nil {
					variantID, match = &alias.VariantID, MatchAlias
				} else if !isNoRows(err) {
					return err
				}
			}
			row, err := q.InsertGroceryReceiptLine(ctx, store.InsertGroceryReceiptLineParams{
				ReceiptID: receipt.ID, Position: int32(i + 1), RawText: l.rawText, Quantity: l.quantity,
				AmountMinor: l.amount, DiscountMinor: l.discount, VariantID: variantID,
			})
			if err != nil {
				return err
			}
			matches[row.ID] = match
		}
		return nil
	})
	if err != nil {
		if createdExpense != nil {
			// The expense was created by this call: do not leave it behind.
			_ = s.fin.DeleteTransaction(context.WithoutCancel(ctx), *createdExpense)
		}
		return Receipt{}, err
	}
	return s.loadReceipt(ctx, receiptID, matches)
}

func (s *Service) createExpense(ctx context.Context, in *CreateExpenseInput, currency string, total int64, day time.Time, storeName string) (uuid.UUID, error) {
	accountID, err := parseRequiredID("create_expense.account_id", in.AccountID)
	if err != nil {
		return uuid.Nil, err
	}
	account, err := s.fin.GetAccount(ctx, accountID)
	if err != nil {
		return uuid.Nil, fromFinance("create_expense.account_id", err)
	}
	if account.Currency != currency {
		return uuid.Nil, Invalid("currency", fmt.Sprintf("is %s but account %q is in %s", currency, account.Name, account.Currency))
	}
	categoryID, err := optionalID("create_expense.category_id", in.CategoryID)
	if err != nil {
		return uuid.Nil, err
	}
	if categoryID == nil {
		if categoryID, err = s.defaultExpenseCategory(ctx); err != nil {
			return uuid.Nil, err
		}
	}
	tx, err := s.fin.CreateTransaction(ctx, finance.TransactionInput{
		Type: "expense", AccountID: accountID, Amount: total, CategoryID: categoryID,
		Description: storeName, OccurredOn: day.Format(dateLayout),
	})
	if err != nil {
		return uuid.Nil, fromFinance("create_expense", err)
	}
	return tx.ID, nil
}

func mapReceiptWriteError(err error) error {
	switch pgErrorCode(err) {
	case pgUniqueViolation:
		return Conflict("this transaction is already linked to another receipt")
	case pgForeignKeyViolation:
		if strings.Contains(err.Error(), "currency") {
			return Invalid("currency", "unknown currency")
		}
	}
	return err
}

// UpdateReceipt changes the store, date, total or transaction link. Lines
// are not edited here.
func (s *Service) UpdateReceipt(ctx context.Context, id uuid.UUID, in UpdateReceiptInput) (Receipt, error) {
	current, err := s.q.GetGroceryReceipt(ctx, id)
	if isNoRows(err) {
		return Receipt{}, NotFound("receipt")
	}
	if err != nil {
		return Receipt{}, err
	}
	params := store.UpdateGroceryReceiptParams{ID: id, TotalMinor: in.Total}
	if in.Total != nil && *in.Total < 1 {
		return Receipt{}, Invalid("total", "must be greater than 0")
	}
	if in.PurchasedOn != nil {
		d, err := parseDay("purchased_on", *in.PurchasedOn)
		if err != nil {
			return Receipt{}, err
		}
		params.PurchasedOn = &d
	}
	if in.StoreID != nil {
		storeID, err := parseRequiredID("store_id", *in.StoreID)
		if err != nil {
			return Receipt{}, err
		}
		if _, err := s.activeStore(ctx, "store_id", storeID); err != nil {
			return Receipt{}, err
		}
		params.StoreID = &storeID
	}
	if in.TransactionID != nil {
		params.SetTransaction = true
		transactionID, err := optionalID("transaction_id", in.TransactionID)
		if err != nil {
			return Receipt{}, err
		}
		if transactionID != nil {
			if err := s.checkLinkable(ctx, *transactionID, current.Currency, &id); err != nil {
				return Receipt{}, err
			}
		}
		params.TransactionID = transactionID
	}
	if _, err := s.q.UpdateGroceryReceipt(ctx, params); err != nil {
		return Receipt{}, mapReceiptWriteError(err)
	}
	return s.loadReceipt(ctx, id, nil)
}

// AssignLines maps receipt lines to product variants, creating products and
// variants by name when needed, and remembers each mapping as an alias of the
// receipt's store so the next receipt matches automatically. All assignments
// apply or none does.
func (s *Service) AssignLines(ctx context.Context, receiptID uuid.UUID, in AssignLinesInput) (Receipt, error) {
	receipt, err := s.q.GetGroceryReceipt(ctx, receiptID)
	if isNoRows(err) {
		return Receipt{}, NotFound("receipt")
	}
	if err != nil {
		return Receipt{}, err
	}
	if len(in.Assignments) == 0 {
		return Receipt{}, Invalid("assignments", "must contain at least one assignment")
	}
	var groups []ProductGroup
	for _, a := range in.Assignments {
		if a.Group != nil && strings.TrimSpace(*a.Group) != "" {
			if groups, err = s.ListGroups(ctx); err != nil {
				return Receipt{}, err
			}
			break
		}
	}

	err = s.withTx(ctx, func(q *store.Queries) error {
		for i, a := range in.Assignments {
			field := func(name string) string { return fmt.Sprintf("assignments[%d].%s", i, name) }
			lineID, err := parseRequiredID(field("line_id"), a.LineID)
			if err != nil {
				return err
			}
			line, err := q.GetGroceryReceiptLine(ctx, store.GetGroceryReceiptLineParams{ID: lineID, ReceiptID: receiptID})
			if isNoRows(err) {
				return Invalid(field("line_id"), "unknown line in this receipt")
			}
			if err != nil {
				return err
			}
			variantID, err := resolveVariant(ctx, q, a, groups, field)
			if err != nil {
				return err
			}
			if _, err := q.SetGroceryReceiptLineVariant(ctx, store.SetGroceryReceiptLineVariantParams{ID: line.ID, VariantID: variantID}); err != nil {
				return err
			}
			if variantID != nil {
				if _, err := q.UpsertStoreItemAlias(ctx, store.UpsertStoreItemAliasParams{
					StoreID: receipt.StoreID, RawTextKey: NormalizeRawText(line.RawText), VariantID: *variantID,
				}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return Receipt{}, err
	}
	return s.loadReceipt(ctx, receiptID, nil)
}

// resolveVariant returns the variant an assignment points at, creating the
// product and variant when they are new. Nil means "unassign".
func resolveVariant(ctx context.Context, q *store.Queries, a AssignLineInput, groups []ProductGroup, field func(string) string) (*uuid.UUID, error) {
	variantID, err := optionalID(field("variant_id"), a.VariantID)
	if err != nil {
		return nil, err
	}
	productName := ""
	if a.Product != nil {
		productName = strings.TrimSpace(*a.Product)
	}
	if variantID != nil && productName != "" {
		return nil, Invalid(field("product"), "cannot be combined with variant_id")
	}
	if variantID != nil {
		if _, err := q.GetProductVariant(ctx, *variantID); err != nil {
			if isNoRows(err) {
				return nil, Invalid(field("variant_id"), "unknown variant")
			}
			return nil, err
		}
		return variantID, nil
	}
	if productName == "" {
		return nil, nil
	}
	productName, err = normalizeName(field("product"), productName)
	if err != nil {
		return nil, err
	}

	var productID uuid.UUID
	product, err := q.FindGroceryProductByName(ctx, productName)
	switch {
	case err == nil:
		productID = product.ID
	case !isNoRows(err):
		return nil, err
	default:
		if productID, err = createProduct(ctx, q, a, productName, groups, field); err != nil {
			return nil, err
		}
	}

	brand := defaultBrand
	if a.Brand != nil && strings.TrimSpace(*a.Brand) != "" {
		if brand, err = normalizeName(field("brand"), *a.Brand); err != nil {
			return nil, err
		}
	}
	size, err := validatePackSize(a.PackSize)
	if err != nil {
		return nil, Invalid(field("pack_size"), "must be greater than 0")
	}
	variant, err := q.FindProductVariant(ctx, store.FindProductVariantParams{ProductID: productID, Brand: brand, PackSize: size})
	if isNoRows(err) {
		variant, err = q.CreateProductVariant(ctx, store.CreateProductVariantParams{
			ProductID: productID, Brand: brand, PackSize: size, PackLabel: emptyToNil(a.PackLabel),
		})
		if pgErrorCode(err) == pgUniqueViolation {
			return nil, Conflict("this product already has a variant with this brand and pack size")
		}
	}
	if err != nil {
		return nil, err
	}
	return &variant.ID, nil
}

func createProduct(ctx context.Context, q *store.Queries, a AssignLineInput, name string, groups []ProductGroup, field func(string) string) (uuid.UUID, error) {
	if a.CompareUnit == nil {
		return uuid.Nil, Invalid(field("compare_unit"), fmt.Sprintf("is required to create the new product %q (one of %s)", name, strings.Join(CompareUnits, ", ")))
	}
	if err := validateCompareUnit(*a.CompareUnit); err != nil {
		return uuid.Nil, Invalid(field("compare_unit"), "must be one of "+strings.Join(CompareUnits, ", "))
	}
	var groupID *uuid.UUID
	if a.Group != nil && strings.TrimSpace(*a.Group) != "" {
		wanted := strings.TrimSpace(*a.Group)
		names := make([]string, len(groups))
		for i, g := range groups {
			names[i] = g.Name
			if strings.EqualFold(g.Name, wanted) {
				id := g.ID
				groupID = &id
			}
		}
		if groupID == nil {
			return uuid.Nil, Invalid(field("group"), fmt.Sprintf("unknown product group %q; valid groups: %s", wanted, strings.Join(names, ", ")))
		}
	}
	id, err := q.CreateGroceryProduct(ctx, store.CreateGroceryProductParams{
		Name: name, GroupID: groupID, CompareUnit: *a.CompareUnit, DisplayMultiplier: 1,
	})
	if pgErrorCode(err) == pgUniqueViolation {
		return uuid.Nil, Conflict("an active product with this name already exists")
	}
	return id, err
}
