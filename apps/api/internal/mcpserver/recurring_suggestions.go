package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/money"
)

// --- Tool inputs and outputs ---

type suggestionOut struct {
	Key          string  `json:"key"`
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Amount       string  `json:"amount"`
	Currency     string  `json:"currency"`
	Account      string  `json:"account"`
	Category     string  `json:"category,omitempty"`
	Frequency    string  `json:"frequency"`
	NextDue      string  `json:"next_due,omitempty"`
	Transactions int     `json:"transactions"`
	Confidence   float64 `json:"confidence"`
}

type listSuggestionsArgs struct{}

type listSuggestionsOut struct {
	Suggestions []suggestionOut `json:"suggestions"`
}

type acceptSuggestionArgs struct {
	Key      string   `json:"key" jsonschema:"Key of a suggestion returned by list_recurring_suggestions"`
	Name     string   `json:"name,omitempty" jsonschema:"Name for the recurring item. Defaults to the suggested name"`
	Amount   *float64 `json:"amount,omitempty" jsonschema:"Estimated amount of each occurrence in the account currency. Defaults to the suggested amount"`
	Category string   `json:"category,omitempty" jsonschema:"Existing category name matching the type (see list_categories). Defaults to the suggested category"`
}

type acceptSuggestionOut struct {
	Recurring          recurringOut `json:"recurring"`
	LinkedTransactions int          `json:"linked_transactions"`
}

type dismissSuggestionArgs struct {
	Key string `json:"key" jsonschema:"Key of a suggestion returned by list_recurring_suggestions"`
}

type dismissSuggestionOut struct {
	Dismissed string `json:"dismissed"`
}

type linkRecurringArgs struct {
	Transaction string `json:"transaction" jsonschema:"ID of the transaction (from add_expense, add_income or list_transactions)"`
	Recurring   string `json:"recurring,omitempty" jsonschema:"Name or ID of the recurring item to link the transaction to (see list_recurring). Omit it when unlinking"`
	Period      string `json:"period,omitempty" jsonschema:"Due date being paid, YYYY-MM-DD; must be one of the item's due dates. Defaults to the due date closest to the transaction date"`
	Unlink      bool   `json:"unlink,omitempty" jsonschema:"true to remove the transaction's link to its recurring item instead of linking"`
}

type linkRecurringOut struct {
	Transaction transactionOut `json:"transaction"`
}

// recurringMatchOut tells that a new transaction looks like a payment of an
// active recurring item. The transaction is not linked.
type recurringMatchOut struct {
	Recurring       string `json:"recurring"`
	RecurringID     string `json:"recurring_id"`
	Period          string `json:"period"`
	EstimatedAmount string `json:"estimated_amount"`
}

func (s *Server) registerRecurringSuggestionTools() {
	addTool(s, &mcp.Tool{
		Name:        "list_recurring_suggestions",
		Title:       "List recurring suggestions",
		Description: "List recurring patterns detected among unlinked expenses and income (e.g. a Spotify charge every month) that could be tracked as recurring items. Suggestions only: show them to the user and call accept_recurring_suggestion only after the user agrees.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ listSuggestionsArgs) (*mcp.CallToolResult, listSuggestionsOut, error) {
		return s.listRecurringSuggestions(ctx)
	})

	addTool(s, &mcp.Tool{
		Name:        "accept_recurring_suggestion",
		Title:       "Accept recurring suggestion",
		Description: "Turn a suggestion into a recurring item and link its matching past transactions to it. Only call it after the user agreed to track it. Optional name, amount and category overrides. Fails if the name clashes with another non-cancelled item.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args acceptSuggestionArgs) (*mcp.CallToolResult, acceptSuggestionOut, error) {
		return s.acceptRecurringSuggestion(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "dismiss_recurring_suggestion",
		Title:       "Dismiss recurring suggestion",
		Description: "Hide a suggestion for good because the user does not want to track it. It does not touch any transaction. Ask the user first.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args dismissSuggestionArgs) (*mcp.CallToolResult, dismissSuggestionOut, error) {
		return s.dismissRecurringSuggestion(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "link_transaction_to_recurring",
		Title:       "Link transaction to recurring item",
		Description: "Mark an existing transaction as a payment of a recurring item (use it when add_expense or add_income reports a recurring_match and the user confirms), or remove that link with unlink=true. The transaction must have the item's type and account. It does not change the transaction amount or the item's estimate.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args linkRecurringArgs) (*mcp.CallToolResult, linkRecurringOut, error) {
		return s.linkTransactionToRecurring(ctx, args)
	})
}

func suggestionToOut(sg finance.RecurringSuggestion) suggestionOut {
	out := suggestionOut{
		Key:          sg.Key,
		Name:         sg.Name,
		Type:         sg.Type,
		Amount:       money.Format(sg.Amount, sg.MinorUnits),
		Currency:     sg.Currency,
		Account:      sg.AccountName,
		Frequency:    sg.Frequency,
		Transactions: len(sg.TransactionIDs),
		Confidence:   sg.Confidence,
	}
	if sg.CategoryName != nil {
		out.Category = *sg.CategoryName
	}
	if sg.NextDueOn != nil {
		out.NextDue = *sg.NextDueOn
	}
	return out
}

func (s *Server) listRecurringSuggestions(ctx context.Context) (*mcp.CallToolResult, listSuggestionsOut, error) {
	items, err := s.finance.ListRecurringSuggestions(ctx)
	if err != nil {
		return nil, listSuggestionsOut{}, friendly(err)
	}
	out := listSuggestionsOut{Suggestions: make([]suggestionOut, len(items))}
	lines := make([]string, len(items))
	for i, sg := range items {
		o := suggestionToOut(sg)
		out.Suggestions[i] = o
		lines[i] = fmt.Sprintf("- %s (%s, %s): ~%s %s in %s%s, seen %d times, confidence %.2f [key %s]",
			o.Name, o.Type, o.Frequency, o.Amount, o.Currency, o.Account, categorySuffix(o.Category), o.Transactions, o.Confidence, o.Key)
	}
	if len(lines) == 0 {
		return text("No recurring patterns detected among unlinked transactions."), out, nil
	}
	return text("Detected recurring patterns (suggestions only, amounts are estimates). Ask the user before accepting any:\n" + strings.Join(lines, "\n")), out, nil
}

func (s *Server) acceptRecurringSuggestion(ctx context.Context, args acceptSuggestionArgs) (*mcp.CallToolResult, acceptSuggestionOut, error) {
	key := strings.TrimSpace(args.Key)
	suggestions, err := s.finance.ListRecurringSuggestions(ctx)
	if err != nil {
		return nil, acceptSuggestionOut{}, friendly(err)
	}
	var sg *finance.RecurringSuggestion
	for i := range suggestions {
		if suggestions[i].Key == key {
			sg = &suggestions[i]
			break
		}
	}
	if sg == nil {
		return nil, acceptSuggestionOut{}, errors.New("no suggestion has that key; it may have been accepted, dismissed or no longer detected. Call list_recurring_suggestions and use a listed key")
	}
	in := finance.AcceptSuggestionInput{Key: sg.Key, Name: strings.TrimSpace(args.Name)}
	if args.Amount != nil {
		v, err := toMinor(*args.Amount, sg.MinorUnits, sg.Currency, "amount")
		if err != nil {
			return nil, acceptSuggestionOut{}, err
		}
		in.Amount = &v
	}
	if strings.TrimSpace(args.Category) != "" {
		category, err := s.resolveCategory(ctx, args.Category, sg.Type)
		if err != nil {
			return nil, acceptSuggestionOut{}, err
		}
		in.CategoryID = &category.ID
	}
	item, err := s.finance.AcceptRecurringSuggestion(ctx, in)
	if err != nil {
		return nil, acceptSuggestionOut{}, friendly(err)
	}
	out := recurringToOut(item)
	linked := len(sg.TransactionIDs)
	return text(fmt.Sprintf("Accepted suggestion: created recurring %s %s: %s %s %s from %s%s (id %s) and linked %d existing transactions to it. The amount is an estimate in the account currency.",
			out.Type, out.Name, out.Amount, out.Currency, everyLabel(out.Frequency, out.IntervalCount), out.StartDate, nextDueSuffix(out), out.ID, linked)),
		acceptSuggestionOut{Recurring: out, LinkedTransactions: linked}, nil
}

func (s *Server) dismissRecurringSuggestion(ctx context.Context, args dismissSuggestionArgs) (*mcp.CallToolResult, dismissSuggestionOut, error) {
	key := strings.TrimSpace(args.Key)
	if err := s.finance.DismissRecurringSuggestion(ctx, finance.SuggestionKeyInput{Key: key}); err != nil {
		return nil, dismissSuggestionOut{}, friendly(err)
	}
	return text("Dismissed the suggestion; it will not be suggested again."), dismissSuggestionOut{Dismissed: key}, nil
}

func (s *Server) linkTransactionToRecurring(ctx context.Context, args linkRecurringArgs) (*mcp.CallToolResult, linkRecurringOut, error) {
	id, err := uuid.Parse(strings.TrimSpace(args.Transaction))
	if err != nil {
		return nil, linkRecurringOut{}, errors.New("transaction must be the ID of a transaction (see list_transactions)")
	}
	if args.Unlink {
		if strings.TrimSpace(args.Recurring) != "" || strings.TrimSpace(args.Period) != "" {
			return nil, linkRecurringOut{}, errors.New("with unlink=true do not pass recurring or period")
		}
		tx, err := s.finance.UnlinkTransactionFromRecurring(ctx, id)
		if err != nil {
			return nil, linkRecurringOut{}, friendly(err)
		}
		return text(fmt.Sprintf("Transaction %s is no longer linked to a recurring item.", tx.ID)), linkRecurringOut{Transaction: transactionToOut(tx)}, nil
	}
	if strings.TrimSpace(args.Recurring) == "" {
		return nil, linkRecurringOut{}, errors.New("pass recurring (name or ID, see list_recurring) to link the transaction, or unlink=true to remove its link")
	}
	item, err := s.resolveRecurring(ctx, args.Recurring)
	if err != nil {
		return nil, linkRecurringOut{}, err
	}
	tx, err := s.finance.LinkTransactionToRecurring(ctx, id, finance.LinkRecurringInput{RecurringID: item.ID, Period: strings.TrimSpace(args.Period)})
	if err != nil {
		return nil, linkRecurringOut{}, friendly(err)
	}
	out := transactionToOut(tx)
	out.RecurringName = item.Name
	return text(fmt.Sprintf("Linked transaction %s to %s for period %s.", out.ID, item.Name, out.RecurringDueOn)), linkRecurringOut{Transaction: out}, nil
}

// matchHint looks for an active recurring item the new transaction could be
// paying. It never fails the tool: the hint is best effort and nothing is
// linked.
func (s *Server) matchHint(ctx context.Context, txID uuid.UUID) *recurringMatchOut {
	m, err := s.finance.MatchRecurringItem(ctx, txID)
	if err != nil || m.Item == nil || m.PeriodDueOn == nil {
		return nil
	}
	return &recurringMatchOut{
		Recurring:       m.Item.Name,
		RecurringID:     m.Item.ID.String(),
		Period:          *m.PeriodDueOn,
		EstimatedAmount: money.Format(m.Item.Amount, m.Item.MinorUnits),
	}
}
