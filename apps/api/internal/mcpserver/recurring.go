package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/money"
)

var (
	recurringFrequencies = []string{finance.UnitWeek, finance.UnitMonth, finance.UnitYear}
	recurringStatuses    = []string{finance.StatusActive, finance.StatusPaused, finance.StatusCancelled}
)

// --- Tool inputs ---

type listRecurringArgs struct {
	Status string `json:"status,omitempty" jsonschema:"Filter by status: active, paused, cancelled or all. Defaults to active and paused items (cancelled ones are hidden)"`
	Type   string `json:"type,omitempty" jsonschema:"Filter by type: expense or income"`
}

type createRecurringArgs struct {
	Name          string  `json:"name" jsonschema:"Name of the subscription or recurring item, e.g. Netflix. Must be unique among non-cancelled items"`
	Type          string  `json:"type" jsonschema:"expense (subscription, bill, installment) or income (e.g. salary)"`
	Amount        float64 `json:"amount" jsonschema:"Estimated positive amount of each occurrence in the account currency, e.g. 199.00"`
	Frequency     string  `json:"frequency" jsonschema:"How often it repeats: week, month or year"`
	IntervalCount *int    `json:"interval_count,omitempty" jsonschema:"Repeats every this many frequency units (default 1). E.g. frequency month with interval_count 3 is quarterly"`
	StartDate     string  `json:"start_date,omitempty" jsonschema:"First due date in YYYY-MM-DD format. Defaults to today. For 'on the 5th of each month' use the next 5th"`
	Account       string  `json:"account,omitempty" jsonschema:"Account name or ID that is charged or credited. Optional when only one active account exists"`
	Category      string  `json:"category,omitempty" jsonschema:"Existing category name matching the type (see list_categories). Optional"`
	TotalPayments *int    `json:"total_payments,omitempty" jsonschema:"Number of payments for an installment plan (e.g. 12 for 12 months). Omit for open-ended items"`
	Notes         string  `json:"notes,omitempty" jsonschema:"Free-form note"`
}

type updateRecurringArgs struct {
	Recurring          string   `json:"recurring" jsonschema:"Name or ID of the recurring item to edit (see list_recurring)"`
	Name               *string  `json:"name,omitempty" jsonschema:"New name. Must not clash with another non-cancelled item"`
	Amount             *float64 `json:"amount,omitempty" jsonschema:"New estimated amount of each occurrence in the account currency"`
	Frequency          *string  `json:"frequency,omitempty" jsonschema:"New frequency: week, month or year"`
	IntervalCount      *int     `json:"interval_count,omitempty" jsonschema:"Repeat every this many frequency units"`
	StartDate          *string  `json:"start_date,omitempty" jsonschema:"New first due date, YYYY-MM-DD. The schedule is computed from it"`
	Category           *string  `json:"category,omitempty" jsonschema:"Existing category name matching the item type"`
	ClearCategory      bool     `json:"clear_category,omitempty" jsonschema:"true to remove the category"`
	TotalPayments      *int     `json:"total_payments,omitempty" jsonschema:"Number of payments for an installment plan"`
	ClearTotalPayments bool     `json:"clear_total_payments,omitempty" jsonschema:"true to make an installment plan open-ended"`
	Notes              *string  `json:"notes,omitempty" jsonschema:"New note (empty string clears it)"`
	Status             *string  `json:"status,omitempty" jsonschema:"active, paused or cancelled. Use cancelled when the user cancels a subscription (history is kept), paused to suspend it temporarily, active to resume"`
}

// --- Tool outputs ---

type recurringOut struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	Amount        string `json:"amount"`
	MonthlyAmount string `json:"monthly_amount"`
	Currency      string `json:"currency"`
	Account       string `json:"account"`
	Category      string `json:"category,omitempty"`
	Frequency     string `json:"frequency"`
	IntervalCount int    `json:"interval_count"`
	StartDate     string `json:"start_date"`
	NextDue       string `json:"next_due,omitempty"`
	TotalPayments int    `json:"total_payments,omitempty"`
	LastDue       string `json:"last_due,omitempty"`
	Notes         string `json:"notes,omitempty"`
	// CurrentPeriod is the due date the item is in now and whether it is
	// paid, pending or overdue (active items only).
	CurrentPeriod *periodOut  `json:"current_period,omitempty"`
	LastPayment   *paymentOut `json:"last_payment,omitempty"`
}

type periodOut struct {
	DueOn  string `json:"due_on"`
	Status string `json:"status"`
}

type paymentOut struct {
	TransactionID string `json:"transaction_id"`
	Date          string `json:"date"`
	Amount        string `json:"amount"`
	DueOn         string `json:"due_on"`
}

type upcomingRecurringArgs struct {
	Days int `json:"days,omitempty" jsonschema:"Look this many days ahead, today included (1-366, default 30). Overdue unpaid items are always included"`
}

type upcomingOut struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	DueOn    string `json:"due_on"`
	Status   string `json:"status"`
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
	Account  string `json:"account"`
}

type upcomingRecurringOut struct {
	Items []upcomingOut `json:"items"`
}

type markRecurringPaidArgs struct {
	Recurring string   `json:"recurring" jsonschema:"Name or ID of the recurring item that was paid (see list_recurring)"`
	Amount    *float64 `json:"amount,omitempty" jsonschema:"Amount actually paid in the account currency. Defaults to the item's estimated amount, which is not changed"`
	Date      string   `json:"date,omitempty" jsonschema:"Date of the payment, YYYY-MM-DD. Defaults to today"`
	Period    string   `json:"period,omitempty" jsonschema:"Due date being paid, YYYY-MM-DD; must be one of the item's due dates (see list_upcoming_recurring). Omit it to pay the period closest to date. Pass it explicitly to pay in advance or to record a second payment for an already paid period"`
}

type markRecurringPaidOut struct {
	Transaction transactionOut `json:"transaction"`
	Recurring   string         `json:"recurring"`
	Period      string         `json:"period"`
}

type committedOut struct {
	Currency     string `json:"currency"`
	Expense      string `json:"expense"`
	Income       string `json:"income"`
	Net          string `json:"net"`
	ExpenseCount int    `json:"expense_count"`
	IncomeCount  int    `json:"income_count"`
}

type listRecurringOut struct {
	Items            []recurringOut `json:"items"`
	CommittedMonthly []committedOut `json:"committed_monthly"`
}

func (s *Server) registerRecurringTools() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_recurring",
		Title:       "List recurring items",
		Description: "List subscriptions, bills, installments and recurring income (users may call them subscriptions) with their next due date and status, plus the committed monthly cost per currency (active items only, normalized to an average month, estimates). Use it to answer what subscriptions exist or how much they cost per month.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listRecurringArgs) (*mcp.CallToolResult, listRecurringOut, error) {
		return s.listRecurring(ctx, args)
	})

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_upcoming_recurring",
		Title:       "List upcoming recurring payments",
		Description: "List what is due in the next days (default 30, today included) plus overdue unpaid items, ordered by due date, with status paid, pending or overdue. Use it for 'what is due this week?' or 'what is still pending this month?' (pending and overdue are unpaid).",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args upcomingRecurringArgs) (*mcp.CallToolResult, upcomingRecurringOut, error) {
		return s.listUpcomingRecurring(ctx, args)
	})

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "mark_recurring_paid",
		Title:       "Mark recurring item as paid",
		Description: "Record that a recurring item (subscription, bill, installment, salary) was paid or received: creates the linked expense or income transaction using the item's account, category and name, and marks the period paid. Use it instead of add_expense or add_income when the user paid something that matches a recurring item. Optional amount, date and period overrides. If the period is already paid it fails and explains how to proceed; ask the user before retrying with an explicit period.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args markRecurringPaidArgs) (*mcp.CallToolResult, markRecurringPaidOut, error) {
		return s.markRecurringPaid(actorContext(ctx, req), args)
	})

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "create_recurring",
		Title:       "Create recurring item",
		Description: "Track a new subscription, bill, installment plan or recurring income, e.g. 'Spotify, 99 a month starting on the 5th'. It only records the schedule and an estimated amount; it does not create transactions. Check list_recurring first to avoid duplicates.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args createRecurringArgs) (*mcp.CallToolResult, recurringOut, error) {
		return s.createRecurring(actorContext(ctx, req), args)
	})

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "update_recurring",
		Title:       "Update recurring item",
		Description: "Edit a recurring item found by name: change its amount, schedule, name, category or notes, or pause, resume or cancel it (status). Only the fields you pass change; the type and account cannot be changed (cancel it and create a new one instead). Cancelled items are kept for history and excluded from the committed monthly cost.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args updateRecurringArgs) (*mcp.CallToolResult, recurringOut, error) {
		return s.updateRecurring(ctx, args)
	})
}

// --- Handlers ---

func (s *Server) listRecurring(ctx context.Context, args listRecurringArgs) (*mcp.CallToolResult, listRecurringOut, error) {
	filter := finance.RecurringFilter{}
	status := strings.ToLower(strings.TrimSpace(args.Status))
	hideCancelled := false
	switch status {
	case "":
		hideCancelled = true
	case "all":
	default:
		if !slices.Contains(recurringStatuses, status) {
			return nil, listRecurringOut{}, fmt.Errorf("unknown status %q; use one of: active, paused, cancelled, all", args.Status)
		}
		filter.Status = &status
	}
	if t := strings.ToLower(strings.TrimSpace(args.Type)); t != "" {
		if t != finance.TypeExpense && t != finance.TypeIncome {
			return nil, listRecurringOut{}, fmt.Errorf("unknown type %q; use expense or income", args.Type)
		}
		filter.Type = &t
	}
	items, err := s.finance.ListRecurringItems(ctx, filter)
	if err != nil {
		return nil, listRecurringOut{}, friendly(err)
	}
	summary, err := s.finance.RecurringSummary(ctx)
	if err != nil {
		return nil, listRecurringOut{}, friendly(err)
	}

	out := listRecurringOut{Items: []recurringOut{}, CommittedMonthly: []committedOut{}}
	for _, it := range items {
		if hideCancelled && it.Status == finance.StatusCancelled {
			continue
		}
		out.Items = append(out.Items, recurringToOut(it))
	}
	var lines []string
	for _, it := range out.Items {
		line := fmt.Sprintf("- %s (%s, %s): %s %s %s", it.Name, it.Type, it.Status, it.Amount, it.Currency, everyLabel(it.Frequency, it.IntervalCount))
		if it.NextDue != "" {
			line += ", next due " + it.NextDue
		}
		if it.CurrentPeriod != nil {
			line += fmt.Sprintf("; current period %s is %s", it.CurrentPeriod.DueOn, it.CurrentPeriod.Status)
		}
		if it.LastPayment != nil {
			line += fmt.Sprintf("; last payment %s %s on %s for period %s", it.LastPayment.Amount, it.Currency, it.LastPayment.Date, it.LastPayment.DueOn)
		}
		lines = append(lines, line)
	}
	for _, c := range summary.Currencies {
		out.CommittedMonthly = append(out.CommittedMonthly, committedOut{
			Currency:     c.Currency,
			Expense:      money.Format(c.Expense, c.MinorUnits),
			Income:       money.Format(c.Income, c.MinorUnits),
			Net:          money.Format(c.Net, c.MinorUnits),
			ExpenseCount: c.ExpenseCount,
			IncomeCount:  c.IncomeCount,
		})
	}
	if len(lines) == 0 {
		lines = []string{"No recurring items match."}
	}
	summaryLines := make([]string, len(out.CommittedMonthly))
	for i, c := range out.CommittedMonthly {
		summaryLines[i] = fmt.Sprintf("%s: expenses %s, income %s, net %s per month", c.Currency, c.Expense, c.Income, c.Net)
	}
	body := strings.Join(lines, "\n")
	if len(summaryLines) > 0 {
		body += "\nCommitted monthly cost (active items, estimates): " + strings.Join(summaryLines, "; ")
	}
	return text(body), out, nil
}

func (s *Server) listUpcomingRecurring(ctx context.Context, args upcomingRecurringArgs) (*mcp.CallToolResult, upcomingRecurringOut, error) {
	days := args.Days
	if days == 0 {
		days = 30
	}
	if days < 1 || days > 366 {
		return nil, upcomingRecurringOut{}, errors.New("days must be between 1 and 366")
	}
	items, err := s.finance.UpcomingRecurring(ctx, days)
	if err != nil {
		return nil, upcomingRecurringOut{}, friendly(err)
	}
	out := upcomingRecurringOut{Items: make([]upcomingOut, len(items))}
	lines := make([]string, len(items))
	for i, u := range items {
		it := u.Item
		out.Items[i] = upcomingOut{
			Name: it.Name, Type: it.Type, DueOn: u.DueOn, Status: u.Status,
			Amount: money.Format(it.Amount, it.MinorUnits), Currency: it.Currency, Account: it.AccountName,
		}
		o := out.Items[i]
		lines[i] = fmt.Sprintf("- %s %s: %s (%s) ~%s %s in %s", o.DueOn, o.Status, o.Name, o.Type, o.Amount, o.Currency, o.Account)
	}
	if len(lines) == 0 {
		return text(fmt.Sprintf("Nothing is due in the next %d days.", days)), out, nil
	}
	return text(fmt.Sprintf("Due in the next %d days (and overdue):\n%s\nAmounts are estimates.", days, strings.Join(lines, "\n"))), out, nil
}

func (s *Server) markRecurringPaid(ctx context.Context, args markRecurringPaidArgs) (*mcp.CallToolResult, markRecurringPaidOut, error) {
	item, err := s.resolveRecurring(ctx, args.Recurring)
	if err != nil {
		return nil, markRecurringPaidOut{}, err
	}
	in := finance.RecurringPaymentInput{Period: strings.TrimSpace(args.Period), Date: strings.TrimSpace(args.Date)}
	if args.Amount != nil {
		v, err := toMinor(*args.Amount, item.MinorUnits, item.Currency, "amount")
		if err != nil {
			return nil, markRecurringPaidOut{}, err
		}
		in.Amount = &v
	}
	// An explicit period is always honoured (advance or second payments);
	// otherwise refuse to pay a period that is already paid.
	var opts []finance.PaymentOption
	if in.Period == "" {
		opts = append(opts, finance.RejectIfPaid())
	}
	tx, err := s.finance.RegisterRecurringPayment(ctx, item.ID, in, opts...)
	var paid *finance.PeriodPaidError
	if errors.As(err, &paid) {
		advance := "there are no further due dates to pay in advance"
		if paid.NextDueOn != "" {
			advance = "pass period=" + paid.NextDueOn + " to pay the next period in advance"
		}
		return nil, markRecurringPaidOut{}, fmt.Errorf("%s is already paid for %s. Ask the user what they want: %s, or pass period=%s explicitly to record a second charge for that same period", item.Name, paid.DueOn, advance, paid.DueOn)
	}
	if err != nil {
		return nil, markRecurringPaidOut{}, friendly(err)
	}
	t := transactionToOut(tx)
	t.RecurringName = item.Name
	period := t.RecurringDueOn
	return text(fmt.Sprintf("Marked %s as paid for %s: recorded %s of %s %s in %s on %s (id %s).",
			item.Name, period, t.Type, t.Amount, t.Currency, t.Account, t.Date, t.ID)),
		markRecurringPaidOut{Transaction: t, Recurring: item.Name, Period: period}, nil
}

func (s *Server) createRecurring(ctx context.Context, args createRecurringArgs) (*mcp.CallToolResult, recurringOut, error) {
	typ := strings.ToLower(strings.TrimSpace(args.Type))
	if typ != finance.TypeExpense && typ != finance.TypeIncome {
		return nil, recurringOut{}, fmt.Errorf("type must be expense or income, got %q", args.Type)
	}
	frequency, err := parseFrequency(args.Frequency)
	if err != nil {
		return nil, recurringOut{}, err
	}
	account, err := s.resolveAccount(ctx, args.Account, "account")
	if err != nil {
		return nil, recurringOut{}, err
	}
	amount, err := toMinor(args.Amount, account.MinorUnits, account.Currency, "amount")
	if err != nil {
		return nil, recurringOut{}, err
	}
	in := finance.CreateRecurringItemInput{
		Name:          args.Name,
		Type:          typ,
		AccountID:     account.ID,
		Amount:        amount,
		Notes:         args.Notes,
		IntervalUnit:  frequency,
		StartOn:       strings.TrimSpace(args.StartDate),
		TotalPayments: args.TotalPayments,
	}
	if args.IntervalCount != nil {
		if *args.IntervalCount < 1 {
			return nil, recurringOut{}, errors.New("interval_count must be at least 1; omit it for every single week, month or year")
		}
		in.IntervalCount = *args.IntervalCount
	}
	if strings.TrimSpace(args.Category) != "" {
		category, err := s.resolveCategory(ctx, args.Category, typ)
		if err != nil {
			return nil, recurringOut{}, err
		}
		in.CategoryID = &category.ID
	}
	item, err := s.finance.CreateRecurringItem(ctx, in)
	if err != nil {
		return nil, recurringOut{}, friendly(err)
	}
	out := recurringToOut(item)
	return text(fmt.Sprintf("Created recurring %s %s: %s %s %s from %s%s (id %s).%s",
		out.Type, out.Name, out.Amount, out.Currency, everyLabel(out.Frequency, out.IntervalCount), out.StartDate, nextDueSuffix(out), out.ID, estimateNote())), out, nil
}

func (s *Server) updateRecurring(ctx context.Context, args updateRecurringArgs) (*mcp.CallToolResult, recurringOut, error) {
	if args.Name == nil && args.Amount == nil && args.Frequency == nil && args.IntervalCount == nil &&
		args.StartDate == nil && args.Category == nil && !args.ClearCategory && args.TotalPayments == nil &&
		!args.ClearTotalPayments && args.Notes == nil && args.Status == nil {
		return nil, recurringOut{}, errors.New("nothing to update; pass at least one of name, amount, frequency, interval_count, start_date, category, clear_category, total_payments, clear_total_payments, notes or status")
	}
	item, err := s.resolveRecurring(ctx, args.Recurring)
	if err != nil {
		return nil, recurringOut{}, err
	}
	in := finance.UpdateRecurringItemInput{
		Name:               args.Name,
		IntervalCount:      args.IntervalCount,
		StartOn:            args.StartDate,
		TotalPayments:      args.TotalPayments,
		ClearTotalPayments: args.ClearTotalPayments,
		ClearCategory:      args.ClearCategory,
		Notes:              args.Notes,
	}
	if args.Amount != nil {
		v, err := toMinor(*args.Amount, item.MinorUnits, item.Currency, "amount")
		if err != nil {
			return nil, recurringOut{}, err
		}
		in.Amount = &v
	}
	if args.Frequency != nil {
		f, err := parseFrequency(*args.Frequency)
		if err != nil {
			return nil, recurringOut{}, err
		}
		in.IntervalUnit = &f
	}
	if args.Status != nil {
		st := strings.ToLower(strings.TrimSpace(*args.Status))
		if !slices.Contains(recurringStatuses, st) {
			return nil, recurringOut{}, fmt.Errorf("unknown status %q; use one of: active, paused, cancelled", *args.Status)
		}
		in.Status = &st
	}
	if args.Category != nil && strings.TrimSpace(*args.Category) != "" {
		category, err := s.resolveCategory(ctx, *args.Category, item.Type)
		if err != nil {
			return nil, recurringOut{}, err
		}
		in.CategoryID = &category.ID
	}
	updated, err := s.finance.UpdateRecurringItem(ctx, item.ID, in)
	if err != nil {
		return nil, recurringOut{}, friendly(err)
	}
	out := recurringToOut(updated)
	return text(fmt.Sprintf("Updated recurring %s %s (%s): %s %s %s%s (id %s).%s",
		out.Type, out.Name, out.Status, out.Amount, out.Currency, everyLabel(out.Frequency, out.IntervalCount), nextDueSuffix(out), out.ID, estimateNote())), out, nil
}

// --- Helpers ---

func parseFrequency(v string) (string, error) {
	f := strings.ToLower(strings.TrimSpace(v))
	if !slices.Contains(recurringFrequencies, f) {
		return "", fmt.Errorf("frequency %q is not valid; use one of: %s (combine with interval_count, e.g. month + 3 for quarterly)", v, strings.Join(recurringFrequencies, ", "))
	}
	return f, nil
}

// resolveRecurring finds a recurring item by ID or case-insensitive name.
// Non-cancelled items win; a cancelled item is used only when it is the one
// match left.
func (s *Server) resolveRecurring(ctx context.Context, ref string) (finance.RecurringItem, error) {
	items, err := s.finance.ListRecurringItems(ctx, finance.RecurringFilter{})
	if err != nil {
		return finance.RecurringItem{}, friendly(err)
	}
	if len(items) == 0 {
		return finance.RecurringItem{}, errors.New("there are no recurring items; create one with create_recurring first")
	}
	ref = strings.TrimSpace(ref)
	var open, cancelled, partial []finance.RecurringItem
	for _, it := range items {
		switch {
		case it.ID.String() == ref || strings.EqualFold(it.Name, ref):
			if it.Status == finance.StatusCancelled {
				cancelled = append(cancelled, it)
			} else {
				open = append(open, it)
			}
		case ref != "" && it.Status != finance.StatusCancelled && strings.Contains(strings.ToLower(it.Name), strings.ToLower(ref)):
			partial = append(partial, it)
		}
	}
	switch {
	case len(open) > 0:
		return open[0], nil
	case len(cancelled) == 1:
		return cancelled[0], nil
	case len(cancelled) > 1:
		return finance.RecurringItem{}, fmt.Errorf("several cancelled recurring items are named %q; pass the id of one of them: %s", ref, recurringLabels(cancelled, true))
	case len(partial) == 1:
		return partial[0], nil
	}
	return finance.RecurringItem{}, fmt.Errorf("no recurring item matches %q; available items: %s", ref, recurringLabels(items, false))
}

// recurringNames maps every recurring item ID (cancelled included) to its name.
func (s *Server) recurringNames(ctx context.Context) (map[uuid.UUID]string, error) {
	items, err := s.finance.ListRecurringItems(ctx, finance.RecurringFilter{})
	if err != nil {
		return nil, friendly(err)
	}
	names := make(map[uuid.UUID]string, len(items))
	for _, it := range items {
		names[it.ID] = it.Name
	}
	return names, nil
}

func recurringLabels(items []finance.RecurringItem, withID bool) string {
	labels := make([]string, len(items))
	for i, it := range items {
		labels[i] = fmt.Sprintf("%s (%s)", it.Name, it.Status)
		if withID {
			labels[i] = fmt.Sprintf("%s (%s, id %s)", it.Name, it.Status, it.ID)
		}
	}
	return strings.Join(labels, ", ")
}

func recurringToOut(it finance.RecurringItem) recurringOut {
	out := recurringOut{
		ID:            it.ID.String(),
		Name:          it.Name,
		Type:          it.Type,
		Status:        it.Status,
		Amount:        money.Format(it.Amount, it.MinorUnits),
		MonthlyAmount: money.Format(it.MonthlyAmount, it.MinorUnits),
		Currency:      it.Currency,
		Account:       it.AccountName,
		Frequency:     it.IntervalUnit,
		IntervalCount: it.IntervalCount,
		StartDate:     it.StartOn,
		Notes:         it.Notes,
	}
	if it.CategoryName != nil {
		out.Category = *it.CategoryName
	}
	if it.NextDueOn != nil {
		out.NextDue = *it.NextDueOn
	}
	if it.TotalPayments != nil {
		out.TotalPayments = *it.TotalPayments
	}
	if it.LastDueOn != nil {
		out.LastDue = *it.LastDueOn
	}
	if it.CurrentPeriod != nil {
		out.CurrentPeriod = &periodOut{DueOn: it.CurrentPeriod.DueOn, Status: it.CurrentPeriod.Status}
	}
	if it.LastPayment != nil {
		out.LastPayment = &paymentOut{
			TransactionID: it.LastPayment.TransactionID.String(),
			Date:          it.LastPayment.Date,
			Amount:        money.Format(it.LastPayment.Amount, it.MinorUnits),
			DueOn:         it.LastPayment.DueOn,
		}
	}
	return out
}

func everyLabel(frequency string, count int) string {
	if count <= 1 {
		return "every " + frequency
	}
	return fmt.Sprintf("every %d %ss", count, frequency)
}

func nextDueSuffix(out recurringOut) string {
	if out.NextDue == "" {
		return ""
	}
	return ", next due " + out.NextDue
}

func estimateNote() string {
	return " The amount is an estimate in the account currency; no transaction was created."
}
