package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

type updateTransactionArgs struct {
	ID                string   `json:"id" jsonschema:"Transaction ID (from list_transactions or the tool that created it)"`
	Amount            *float64 `json:"amount,omitempty" jsonschema:"New positive amount in the source account currency, e.g. 150.50. Omit to keep the current one"`
	Date              *string  `json:"date,omitempty" jsonschema:"New date in YYYY-MM-DD format. Omit to keep the current one"`
	Description       *string  `json:"description,omitempty" jsonschema:"New note, e.g. the merchant or what was bought. Pass an empty string to clear it. Omit to keep the current one"`
	Category          *string  `json:"category,omitempty" jsonschema:"New category name or ID; its kind must match the transaction type (see list_categories). Omit to keep the current one; to remove the category use clear_category instead. Not for transfers"`
	ClearCategory     bool     `json:"clear_category,omitempty" jsonschema:"true to remove the category so the transaction becomes uncategorized. Do not combine with category"`
	Account           *string  `json:"account,omitempty" jsonschema:"Move the transaction to this account (name or ID; the source account for transfers). Omit to keep the current one"`
	ToAccount         *string  `json:"to_account,omitempty" jsonschema:"New destination account name or ID. Transfers only"`
	DestinationAmount *float64 `json:"destination_amount,omitempty" jsonschema:"New amount received in the destination currency. Transfers only; required when changing the amount or accounts of a transfer between different currencies"`
}

type updateTransactionsArgs struct {
	Items []updateTransactionArgs `json:"items" jsonschema:"Transactions to update (1 to 100), each with its id and only the fields to change. All are updated or none are. Each id may appear once"`
}

func (s *Server) registerUpdateTransactionTools() {
	annotations := &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(false)}

	addTool(s, &mcp.Tool{
		Name:  "update_transaction",
		Title: "Update transaction",
		Description: "Edit an existing transaction: only the fields you pass change (amount, date, description, category, account, and for transfers to_account and destination_amount). " +
			"Use it to fix mistakes and to re-categorize; the type (expense, income, transfer) cannot be changed. Get the id from list_transactions. " +
			"To remove the category pass clear_category=true. To update several transactions, prefer update_transactions.",
		Annotations: annotations,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args updateTransactionArgs) (*mcp.CallToolResult, transactionOut, error) {
		return s.updateTransaction(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:  "update_transactions",
		Title: "Update several transactions",
		Description: "Edit up to 100 existing transactions in one call, e.g. to re-categorize many of them. Each item has an id (from list_transactions) and only the fields to change, as in update_transaction. " +
			"All items are updated or none are; an error names the failing item so you can fix it and resend the whole batch.",
		Annotations: annotations,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args updateTransactionsArgs) (*mcp.CallToolResult, transactionsOut, error) {
		return s.updateTransactions(ctx, args)
	})
}

func (s *Server) updateTransaction(ctx context.Context, args updateTransactionArgs) (*mcp.CallToolResult, transactionOut, error) {
	id, in, err := s.mergeTransactionUpdate(ctx, args)
	if err != nil {
		return nil, transactionOut{}, err
	}
	tx, err := s.finance.UpdateTransaction(ctx, id, in)
	if err != nil {
		return nil, transactionOut{}, friendly(err)
	}
	out := transactionToOut(tx)
	msg := fmt.Sprintf("Updated %s", describeTransaction(out))
	// Only edits that can move spending between budgets are checked.
	if args.Amount != nil || args.Date != nil || args.Category != nil || args.Account != nil {
		if w := s.budgetWarnings(ctx, []finance.Transaction{tx}); len(w) > 0 {
			out.BudgetWarning = &w[0]
			msg += "\n" + w[0].message()
		}
	}
	return text(msg), out, nil
}

func (s *Server) updateTransactions(ctx context.Context, args updateTransactionsArgs) (*mcp.CallToolResult, transactionsOut, error) {
	if err := checkBatch(len(args.Items)); err != nil {
		return nil, transactionsOut{}, err
	}
	updates := make([]finance.TransactionUpdate, len(args.Items))
	for i, item := range args.Items {
		id, in, err := s.mergeTransactionUpdate(ctx, item)
		if err != nil {
			return nil, transactionsOut{}, updateBatchFailure(itemErr(i, "", err))
		}
		updates[i] = finance.TransactionUpdate{ID: id, Input: in}
	}
	updated, err := s.finance.UpdateTransactions(ctx, updates)
	if err != nil {
		return nil, transactionsOut{}, updateBatchFailure(err)
	}
	out := transactionsOut{Transactions: make([]transactionOut, len(updated)), Total: int64(len(updated))}
	lines := make([]string, len(updated))
	for i, tx := range updated {
		out.Transactions[i] = transactionToOut(tx)
		lines[i] = "- " + describeTransaction(out.Transactions[i])
	}
	out.BudgetWarnings = s.budgetWarnings(ctx, updated)
	return text(fmt.Sprintf("Updated %d transactions:\n%s%s", len(updated), strings.Join(lines, "\n"), warningText(out.BudgetWarnings))), out, nil
}

// updateBatchFailure explains that a failed batch changed nothing.
func updateBatchFailure(err error) error {
	return fmt.Errorf("nothing was updated: %s. Fix that item and resend the whole batch", friendly(err).Error())
}

func describeTransaction(t transactionOut) string {
	target := t.Account
	if t.ToAccount != "" {
		target += " → " + t.ToAccount
	}
	return fmt.Sprintf("%s %s %s %s · %s%s%s [id %s]", t.Date, t.Type, t.Amount, t.Currency, target, categorySuffix(t.Category), descriptionSuffix(t.Description), t.ID)
}

// mergeTransactionUpdate loads the transaction and overlays the fields given
// in args, returning the full input that UpdateTransaction expects.
func (s *Server) mergeTransactionUpdate(ctx context.Context, args updateTransactionArgs) (uuid.UUID, finance.TransactionInput, error) {
	var in finance.TransactionInput
	id, err := uuid.Parse(strings.TrimSpace(args.ID))
	if err != nil {
		return uuid.Nil, in, errors.New("id must be a transaction UUID (get it from list_transactions)")
	}
	if args.ClearCategory && args.Category != nil && strings.TrimSpace(*args.Category) != "" {
		return uuid.Nil, in, errors.New("category and clear_category cannot be combined: pass one of them")
	}
	existing, err := s.finance.GetTransaction(ctx, id)
	if err != nil {
		return uuid.Nil, in, friendly(err)
	}

	in = finance.TransactionInput{
		Type:                 existing.Type,
		AccountID:            existing.AccountID,
		Amount:               existing.Amount,
		DestinationAccountID: existing.DestinationAccountID,
		DestinationAmount:    existing.DestinationAmount,
		CategoryID:           existing.CategoryID,
		Description:          existing.Description,
		OccurredOn:           existing.OccurredOn,
	}
	if args.Date != nil {
		in.OccurredOn = *args.Date
	}
	if args.Description != nil {
		in.Description = *args.Description
	}

	// Source account and amount. Amounts convert with the minor units of the
	// account that ends up holding them.
	minorUnits, currency := existing.MinorUnits, existing.Currency
	accountChanged := false
	if args.Account != nil && strings.TrimSpace(*args.Account) != "" {
		account, err := s.resolveAccount(ctx, *args.Account, "account")
		if err != nil {
			return uuid.Nil, in, err
		}
		accountChanged = account.ID != existing.AccountID
		in.AccountID = account.ID
		minorUnits, currency = account.MinorUnits, account.Currency
	}
	amountChanged := false
	if args.Amount != nil {
		amount, err := toMinor(*args.Amount, minorUnits, currency, "amount")
		if err != nil {
			return uuid.Nil, in, err
		}
		amountChanged = amount != existing.Amount
		in.Amount = amount
	}

	if existing.Type == finance.TypeTransfer {
		return id, in, s.mergeTransferUpdate(ctx, args, existing, &in, currency, accountChanged || amountChanged)
	}

	if args.ToAccount != nil || args.DestinationAmount != nil {
		return uuid.Nil, in, errors.New("to_account and destination_amount are only for transfers; omit them")
	}
	switch {
	case args.ClearCategory || (args.Category != nil && strings.TrimSpace(*args.Category) == ""):
		in.CategoryID = nil
	case args.Category != nil:
		category, err := s.resolveCategory(ctx, *args.Category, existing.Type)
		if err != nil {
			return uuid.Nil, in, err
		}
		in.CategoryID = &category.ID
	}
	return id, in, nil
}

// mergeTransferUpdate applies the transfer-only fields. sourceChanged is set
// when the amount or source account differ from the stored ones.
func (s *Server) mergeTransferUpdate(ctx context.Context, args updateTransactionArgs, existing finance.Transaction, in *finance.TransactionInput, sourceCurrency string, sourceChanged bool) error {
	if args.Category != nil && strings.TrimSpace(*args.Category) != "" {
		return errors.New("transfers cannot have a category; omit category")
	}
	in.CategoryID = nil

	destChanged := false
	if args.ToAccount != nil && strings.TrimSpace(*args.ToAccount) != "" {
		to, err := s.resolveAccount(ctx, *args.ToAccount, "to_account")
		if err != nil {
			return err
		}
		destChanged = existing.DestinationAccountID == nil || to.ID != *existing.DestinationAccountID
		in.DestinationAccountID = &to.ID
	}
	destinationID := in.DestinationAccountID
	if destinationID == nil {
		return errors.New("the transfer has no destination account; pass to_account")
	}
	dest, err := s.resolveAnyAccount(ctx, destinationID.String())
	if err != nil {
		return err
	}

	switch {
	case args.DestinationAmount != nil:
		amount, err := toMinor(*args.DestinationAmount, dest.MinorUnits, dest.Currency, "destination_amount")
		if err != nil {
			return err
		}
		in.DestinationAmount = &amount
	case dest.Currency == sourceCurrency:
		// Same currency: the destination receives the amount.
		in.DestinationAmount = nil
	case sourceChanged || destChanged:
		return fmt.Errorf("the transfer is between %s and %s: ask the user how much %s was received and pass destination_amount", sourceCurrency, dest.Currency, dest.Currency)
	}
	return nil
}
