package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/money"
)

// --- Tool inputs ---

type recordArgs struct {
	Amount      float64 `json:"amount" jsonschema:"Positive amount in the account currency, e.g. 150.50"`
	Account     string  `json:"account,omitempty" jsonschema:"Account name or ID. Optional when only one active account exists"`
	Category    string  `json:"category,omitempty" jsonschema:"Existing category name (see list_categories). Optional"`
	Description string  `json:"description,omitempty" jsonschema:"Short note, e.g. the merchant or what was bought"`
	Date        string  `json:"date,omitempty" jsonschema:"Date in YYYY-MM-DD format. Defaults to today"`
}

type transferArgs struct {
	Amount            float64  `json:"amount" jsonschema:"Positive amount taken from the source account, in its currency"`
	FromAccount       string   `json:"from_account" jsonschema:"Source account name or ID"`
	ToAccount         string   `json:"to_account" jsonschema:"Destination account name or ID"`
	DestinationAmount *float64 `json:"destination_amount,omitempty" jsonschema:"Amount received in the destination currency. Required only when the accounts use different currencies"`
	Description       string   `json:"description,omitempty"`
	Date              string   `json:"date,omitempty" jsonschema:"Date in YYYY-MM-DD format. Defaults to today"`
}

type listCategoriesArgs struct {
	Kind string `json:"kind,omitempty" jsonschema:"Filter by kind: expense or income"`
}

type createAccountArgs struct {
	Name           string   `json:"name" jsonschema:"Account name, e.g. BBVA Checking"`
	Type           string   `json:"type" jsonschema:"One of checking, savings, credit_card, cash, investment, other"`
	Currency       string   `json:"currency" jsonschema:"ISO 4217 currency code, e.g. MXN or USD. Cannot be changed later"`
	InitialBalance *float64 `json:"initial_balance,omitempty" jsonschema:"Opening balance in the account currency, e.g. 1500.00. May be negative (e.g. credit card debt). Defaults to 0"`
}

type createCategoryArgs struct {
	Name  string `json:"name" jsonschema:"Category name, e.g. Groceries"`
	Kind  string `json:"kind" jsonschema:"expense or income"`
	Color string `json:"color,omitempty" jsonschema:"Optional hex color like #22c55e"`
}

type summaryArgs struct {
	Period string `json:"period,omitempty" jsonschema:"One of this_month (default), last_month, this_year, last_30_days. Ignored when from/to are given"`
	From   string `json:"from,omitempty" jsonschema:"Start date YYYY-MM-DD (inclusive)"`
	To     string `json:"to,omitempty" jsonschema:"End date YYYY-MM-DD (inclusive)"`
}

type listTransactionsArgs struct {
	Limit    int    `json:"limit,omitempty" jsonschema:"Maximum number of transactions (default 10, max 50)"`
	Account  string `json:"account,omitempty" jsonschema:"Account name or ID"`
	Category string `json:"category,omitempty" jsonschema:"Category name"`
	Type     string `json:"type,omitempty" jsonschema:"expense, income or transfer"`
	From     string `json:"from,omitempty" jsonschema:"Start date YYYY-MM-DD"`
	To       string `json:"to,omitempty" jsonschema:"End date YYYY-MM-DD"`
	Search   string `json:"search,omitempty" jsonschema:"Text to search in descriptions"`
}

type deleteArgs struct {
	ID string `json:"id" jsonschema:"Transaction ID (from list_transactions or the tool that created it)"`
}

// --- Tool outputs (amounts are decimal strings in the stated currency) ---

type accountOut struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Currency string `json:"currency"`
	Balance  string `json:"balance"`
}

type accountsOut struct {
	Accounts []accountOut `json:"accounts"`
}

type categoryOut struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type categoriesOut struct {
	Categories []categoryOut `json:"categories"`
}

type transactionOut struct {
	ID                  string `json:"id"`
	Type                string `json:"type"`
	Date                string `json:"date"`
	Amount              string `json:"amount"`
	Currency            string `json:"currency"`
	Account             string `json:"account"`
	ToAccount           string `json:"to_account,omitempty"`
	DestinationAmount   string `json:"destination_amount,omitempty"`
	DestinationCurrency string `json:"destination_currency,omitempty"`
	Category            string `json:"category,omitempty"`
	Description         string `json:"description,omitempty"`
	RecordedBy          string `json:"recorded_by,omitempty"`
}

type transactionsOut struct {
	Transactions []transactionOut `json:"transactions"`
	Total        int64            `json:"total"`
}

type categoryTotalOut struct {
	Category string `json:"category"`
	Total    string `json:"total"`
	Count    int64  `json:"count"`
}

type currencySummaryOut struct {
	Currency string             `json:"currency"`
	Income   string             `json:"income"`
	Expense  string             `json:"expense"`
	Net      string             `json:"net"`
	Balance  string             `json:"balance"`
	Expenses []categoryTotalOut `json:"expenses_by_category"`
	Incomes  []categoryTotalOut `json:"income_by_category"`
}

type summaryOut struct {
	From       string               `json:"from"`
	To         string               `json:"to"`
	Currencies []currencySummaryOut `json:"currencies"`
}

type deleteOut struct {
	Deleted transactionOut `json:"deleted"`
}

func boolPtr(b bool) *bool { return &b }

func (s *Server) registerTools() {
	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "add_expense",
		Title:       "Add expense",
		Description: "Record an expense (money spent) in an account, optionally with a category.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args recordArgs) (*mcp.CallToolResult, transactionOut, error) {
		return s.record(actorContext(ctx, req), finance.TypeExpense, args)
	})

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "add_income",
		Title:       "Add income",
		Description: "Record income (money received, e.g. salary) in an account, optionally with a category.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args recordArgs) (*mcp.CallToolResult, transactionOut, error) {
		return s.record(actorContext(ctx, req), finance.TypeIncome, args)
	})

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "add_transfer",
		Title:       "Transfer between accounts",
		Description: "Move money between two of the workspace's accounts. Transfers are not counted as income or expenses.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args transferArgs) (*mcp.CallToolResult, transactionOut, error) {
		return s.transfer(actorContext(ctx, req), args)
	})

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_accounts",
		Title:       "List accounts",
		Description: "List active accounts with their currency and current balance.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, accountsOut, error) {
		return s.listAccounts(ctx)
	})

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_categories",
		Title:       "List categories",
		Description: "List active expense and income categories.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listCategoriesArgs) (*mcp.CallToolResult, categoriesOut, error) {
		return s.listCategories(ctx, args)
	})

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "create_account",
		Title:       "Create account",
		Description: "Create a new account (bank account, card, cash, ...) with its currency and opening balance. Only create accounts the user asked for; check list_accounts first to avoid duplicates.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args createAccountArgs) (*mcp.CallToolResult, accountOut, error) {
		return s.createAccount(ctx, args)
	})

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "create_category",
		Title:       "Create category",
		Description: "Create a new expense or income category. Only create categories the user asked for; check list_categories first to avoid duplicates.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args createCategoryArgs) (*mcp.CallToolResult, categoryOut, error) {
		return s.createCategory(ctx, args)
	})

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "get_summary",
		Title:       "Get summary",
		Description: "Income, expenses and net per currency for a period, broken down by category, plus current balances.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args summaryArgs) (*mcp.CallToolResult, summaryOut, error) {
		return s.summary(ctx, args)
	})

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "list_transactions",
		Title:       "List transactions",
		Description: "List recent transactions, newest first, with optional filters.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listTransactionsArgs) (*mcp.CallToolResult, transactionsOut, error) {
		return s.listTransactions(ctx, args)
	})

	mcp.AddTool(s.mcp, &mcp.Tool{
		Name:        "delete_transaction",
		Title:       "Delete transaction",
		Description: "Permanently delete a transaction, e.g. to undo a mistake. Confirm with the user before deleting.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(true), IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args deleteArgs) (*mcp.CallToolResult, deleteOut, error) {
		return s.deleteTransaction(ctx, args)
	})
}

// --- Handlers ---

func (s *Server) record(ctx context.Context, typ string, args recordArgs) (*mcp.CallToolResult, transactionOut, error) {
	account, err := s.resolveAccount(ctx, args.Account, "account")
	if err != nil {
		return nil, transactionOut{}, err
	}
	amount, err := toMinor(args.Amount, account.MinorUnits, account.Currency, "amount")
	if err != nil {
		return nil, transactionOut{}, err
	}
	in := finance.TransactionInput{
		Type:        typ,
		AccountID:   account.ID,
		Amount:      amount,
		Description: args.Description,
		OccurredOn:  args.Date,
	}
	if strings.TrimSpace(args.Category) != "" {
		category, err := s.resolveCategory(ctx, args.Category, typ)
		if err != nil {
			return nil, transactionOut{}, err
		}
		in.CategoryID = &category.ID
	}
	tx, err := s.finance.CreateTransaction(ctx, in)
	if err != nil {
		return nil, transactionOut{}, friendly(err)
	}
	out := transactionToOut(tx)
	verb := map[string]string{finance.TypeExpense: "Recorded expense", finance.TypeIncome: "Recorded income"}[typ]
	return text(fmt.Sprintf("%s of %s %s in %s%s on %s (id %s).", verb, out.Amount, out.Currency, out.Account, categorySuffix(out.Category), out.Date, out.ID)), out, nil
}

func (s *Server) transfer(ctx context.Context, args transferArgs) (*mcp.CallToolResult, transactionOut, error) {
	from, err := s.resolveAccount(ctx, args.FromAccount, "from_account")
	if err != nil {
		return nil, transactionOut{}, err
	}
	to, err := s.resolveAccount(ctx, args.ToAccount, "to_account")
	if err != nil {
		return nil, transactionOut{}, err
	}
	amount, err := toMinor(args.Amount, from.MinorUnits, from.Currency, "amount")
	if err != nil {
		return nil, transactionOut{}, err
	}
	in := finance.TransactionInput{
		Type:                 finance.TypeTransfer,
		AccountID:            from.ID,
		Amount:               amount,
		DestinationAccountID: &to.ID,
		Description:          args.Description,
		OccurredOn:           args.Date,
	}
	if args.DestinationAmount != nil {
		dest, err := toMinor(*args.DestinationAmount, to.MinorUnits, to.Currency, "destination_amount")
		if err != nil {
			return nil, transactionOut{}, err
		}
		in.DestinationAmount = &dest
	} else if from.Currency != to.Currency {
		return nil, transactionOut{}, fmt.Errorf("%s uses %s and %s uses %s: ask the user how much %s was received and pass destination_amount", from.Name, from.Currency, to.Name, to.Currency, to.Currency)
	}
	tx, err := s.finance.CreateTransaction(ctx, in)
	if err != nil {
		return nil, transactionOut{}, friendly(err)
	}
	out := transactionToOut(tx)
	received := ""
	if out.DestinationCurrency != out.Currency {
		received = fmt.Sprintf(" (%s %s received)", out.DestinationAmount, out.DestinationCurrency)
	}
	return text(fmt.Sprintf("Transferred %s %s from %s to %s%s on %s (id %s).", out.Amount, out.Currency, out.Account, out.ToAccount, received, out.Date, out.ID)), out, nil
}

func (s *Server) listAccounts(ctx context.Context) (*mcp.CallToolResult, accountsOut, error) {
	accounts, err := s.finance.ListAccounts(ctx, false)
	if err != nil {
		return nil, accountsOut{}, err
	}
	out := accountsOut{Accounts: make([]accountOut, len(accounts))}
	lines := make([]string, len(accounts))
	for i, a := range accounts {
		out.Accounts[i] = accountToOut(a)
		lines[i] = fmt.Sprintf("- %s (%s, %s): %s %s", a.Name, a.Type, a.Currency, out.Accounts[i].Balance, a.Currency)
	}
	if len(lines) == 0 {
		return text("There are no active accounts yet. Use create_account to add one."), out, nil
	}
	return text("Accounts:\n" + strings.Join(lines, "\n")), out, nil
}

func (s *Server) listCategories(ctx context.Context, args listCategoriesArgs) (*mcp.CallToolResult, categoriesOut, error) {
	var kind *string
	if args.Kind != "" {
		kind = &args.Kind
	}
	categories, err := s.finance.ListCategories(ctx, kind, false)
	if err != nil {
		return nil, categoriesOut{}, friendly(err)
	}
	out := categoriesOut{Categories: make([]categoryOut, len(categories))}
	byKind := map[string][]string{}
	for i, c := range categories {
		out.Categories[i] = categoryToOut(c)
		byKind[c.Kind] = append(byKind[c.Kind], c.Name)
	}
	var b strings.Builder
	for _, k := range []string{"expense", "income"} {
		if names := byKind[k]; len(names) > 0 {
			label := map[string]string{"expense": "Expense", "income": "Income"}[k]
			fmt.Fprintf(&b, "%s categories: %s\n", label, strings.Join(names, ", "))
		}
	}
	if b.Len() == 0 {
		b.WriteString("There are no categories yet.")
	}
	return text(strings.TrimSpace(b.String())), out, nil
}

func (s *Server) createAccount(ctx context.Context, args createAccountArgs) (*mcp.CallToolResult, accountOut, error) {
	in := finance.CreateAccountInput{Name: args.Name, Type: args.Type, Currency: args.Currency}
	if args.InitialBalance != nil {
		currency, err := s.findCurrency(ctx, args.Currency)
		if err != nil {
			return nil, accountOut{}, err
		}
		in.InitialBalance, err = decimalToMinor(*args.InitialBalance, currency.MinorUnits, currency.Code, "initial_balance")
		if err != nil {
			return nil, accountOut{}, err
		}
	}
	account, err := s.finance.CreateAccount(ctx, in)
	if err != nil {
		return nil, accountOut{}, friendly(err)
	}
	out := accountToOut(account)
	return text(fmt.Sprintf("Created %s account %s in %s with a balance of %s %s (id %s).", out.Type, out.Name, out.Currency, out.Balance, out.Currency, out.ID)), out, nil
}

func (s *Server) createCategory(ctx context.Context, args createCategoryArgs) (*mcp.CallToolResult, categoryOut, error) {
	in := finance.CreateCategoryInput{Name: args.Name, Kind: strings.ToLower(strings.TrimSpace(args.Kind))}
	if args.Color != "" {
		in.Color = &args.Color
	}
	category, err := s.finance.CreateCategory(ctx, in)
	if err != nil {
		return nil, categoryOut{}, friendly(err)
	}
	out := categoryToOut(category)
	return text(fmt.Sprintf("Created %s category %s (id %s).", out.Kind, out.Name, out.ID)), out, nil
}

func (s *Server) summary(ctx context.Context, args summaryArgs) (*mcp.CallToolResult, summaryOut, error) {
	from, to := args.From, args.To
	if from == "" && to == "" {
		var err error
		if from, to, err = periodRange(s.finance.Today(), args.Period); err != nil {
			return nil, summaryOut{}, err
		}
	}
	sum, err := s.finance.Summary(ctx, from, to)
	if err != nil {
		return nil, summaryOut{}, friendly(err)
	}

	out := summaryOut{From: sum.From, To: sum.To, Currencies: make([]currencySummaryOut, len(sum.Currencies))}
	var b strings.Builder
	fmt.Fprintf(&b, "Summary from %s to %s:\n", sum.From, sum.To)
	for i, c := range sum.Currencies {
		f := func(v int64) string { return money.Format(v, c.MinorUnits) }
		cs := currencySummaryOut{Currency: c.Currency, Income: f(c.Income), Expense: f(c.Expense), Net: f(c.Net), Balance: f(c.Balance)}
		for _, e := range c.Expenses {
			cs.Expenses = append(cs.Expenses, categoryTotalOut{Category: categoryLabel(e.CategoryName), Total: f(e.Total), Count: e.TransactionCount})
		}
		for _, e := range c.Incomes {
			cs.Incomes = append(cs.Incomes, categoryTotalOut{Category: categoryLabel(e.CategoryName), Total: f(e.Total), Count: e.TransactionCount})
		}
		out.Currencies[i] = cs

		fmt.Fprintf(&b, "\n%s — income %s, expenses %s, net %s, current balance %s\n", c.Currency, cs.Income, cs.Expense, cs.Net, cs.Balance)
		for _, e := range cs.Expenses {
			fmt.Fprintf(&b, "  • %s: %s (%d)\n", e.Category, e.Total, e.Count)
		}
	}
	if len(sum.Currencies) == 0 {
		b.WriteString("No accounts or transactions yet.")
	}
	return text(strings.TrimSpace(b.String())), out, nil
}

func (s *Server) listTransactions(ctx context.Context, args listTransactionsArgs) (*mcp.CallToolResult, transactionsOut, error) {
	limit := args.Limit
	if limit <= 0 {
		limit = 10
	}
	filter := finance.TransactionFilter{Limit: min(limit, 50)}
	if args.Account != "" {
		account, err := s.resolveAccount(ctx, args.Account, "account")
		if err != nil {
			return nil, transactionsOut{}, err
		}
		filter.AccountID = &account.ID
	}
	if args.Category != "" {
		category, err := s.resolveCategory(ctx, args.Category, "")
		if err != nil {
			return nil, transactionsOut{}, err
		}
		filter.CategoryID = &category.ID
	}
	for _, opt := range []struct {
		value string
		dest  **string
	}{{args.Type, &filter.Type}, {args.From, &filter.From}, {args.To, &filter.To}, {args.Search, &filter.Search}} {
		if opt.value != "" {
			v := opt.value
			*opt.dest = &v
		}
	}

	page, err := s.finance.ListTransactions(ctx, filter)
	if err != nil {
		return nil, transactionsOut{}, friendly(err)
	}
	out := transactionsOut{Transactions: make([]transactionOut, len(page.Items)), Total: page.Total}
	lines := make([]string, len(page.Items))
	for i, tx := range page.Items {
		t := transactionToOut(tx)
		out.Transactions[i] = t
		target := t.Account
		if t.ToAccount != "" {
			target += " → " + t.ToAccount
		}
		lines[i] = fmt.Sprintf("- %s %s %s %s · %s%s%s [id %s]", t.Date, t.Type, t.Amount, t.Currency, target, categorySuffix(t.Category), descriptionSuffix(t.Description), t.ID)
	}
	if len(lines) == 0 {
		return text("No transactions match."), out, nil
	}
	return text(fmt.Sprintf("Showing %d of %d transactions:\n%s", len(lines), page.Total, strings.Join(lines, "\n"))), out, nil
}

func (s *Server) deleteTransaction(ctx context.Context, args deleteArgs) (*mcp.CallToolResult, deleteOut, error) {
	id, err := uuid.Parse(strings.TrimSpace(args.ID))
	if err != nil {
		return nil, deleteOut{}, errors.New("id must be a transaction UUID")
	}
	tx, err := s.finance.GetTransaction(ctx, id)
	if err != nil {
		return nil, deleteOut{}, friendly(err)
	}
	if err := s.finance.DeleteTransaction(ctx, id); err != nil {
		return nil, deleteOut{}, friendly(err)
	}
	out := deleteOut{Deleted: transactionToOut(tx)}
	return text(fmt.Sprintf("Deleted %s of %s %s from %s on %s.", tx.Type, out.Deleted.Amount, tx.Currency, tx.AccountName, tx.OccurredOn)), out, nil
}

// --- Helpers ---

// resolveAccount finds an active account by ID or case-insensitive name. An
// empty reference resolves to the only active account, if there is exactly one.
func (s *Server) resolveAccount(ctx context.Context, ref, field string) (finance.Account, error) {
	accounts, err := s.finance.ListAccounts(ctx, false)
	if err != nil {
		return finance.Account{}, err
	}
	if len(accounts) == 0 {
		return finance.Account{}, errors.New("there are no active accounts; ask the user for the account details and create one with create_account first")
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		if len(accounts) == 1 {
			return accounts[0], nil
		}
		return finance.Account{}, fmt.Errorf("%s is required because there are several accounts: %s", field, accountNames(accounts))
	}
	var partial []finance.Account
	for _, a := range accounts {
		if a.ID.String() == ref || strings.EqualFold(a.Name, ref) {
			return a, nil
		}
		if strings.Contains(strings.ToLower(a.Name), strings.ToLower(ref)) {
			partial = append(partial, a)
		}
	}
	if len(partial) == 1 {
		return partial[0], nil
	}
	return finance.Account{}, fmt.Errorf("no account matches %q for %s; available accounts: %s", ref, field, accountNames(accounts))
}

// resolveCategory finds an active category by ID or case-insensitive name.
// kind restricts the search to expense or income categories when set.
func (s *Server) resolveCategory(ctx context.Context, ref, kind string) (finance.Category, error) {
	var kindFilter *string
	if kind != "" {
		kindFilter = &kind
	}
	categories, err := s.finance.ListCategories(ctx, kindFilter, false)
	if err != nil {
		return finance.Category{}, err
	}
	ref = strings.TrimSpace(ref)
	for _, c := range categories {
		if c.ID.String() == ref || strings.EqualFold(c.Name, ref) {
			return c, nil
		}
	}
	names := make([]string, len(categories))
	for i, c := range categories {
		names[i] = c.Name
	}
	label := "category"
	if kind != "" {
		label = kind + " category"
	}
	if len(names) == 0 {
		return finance.Category{}, fmt.Errorf("no %s named %q exists and there are none yet; omit the category or, if the user wants it, create it with create_category", label, ref)
	}
	return finance.Category{}, fmt.Errorf("no %s named %q; use one of: %s (or omit the category, or create it with create_category if the user wants a new one)", label, ref, strings.Join(names, ", "))
}

func accountNames(accounts []finance.Account) string {
	names := make([]string, len(accounts))
	for i, a := range accounts {
		names[i] = fmt.Sprintf("%s (%s)", a.Name, a.Currency)
	}
	return strings.Join(names, ", ")
}

// findCurrency looks up a supported currency by its ISO 4217 code.
func (s *Server) findCurrency(ctx context.Context, code string) (finance.Currency, error) {
	currencies, err := s.finance.ListCurrencies(ctx)
	if err != nil {
		return finance.Currency{}, err
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	for _, c := range currencies {
		if c.Code == code {
			return c, nil
		}
	}
	return finance.Currency{}, fmt.Errorf("unsupported currency %q; use an ISO 4217 code such as MXN, USD or EUR", code)
}

func toMinor(amount float64, minorUnits int, currency, field string) (int64, error) {
	if amount <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", field)
	}
	return decimalToMinor(amount, minorUnits, currency, field)
}

// decimalToMinor converts a signed decimal amount into minor units.
func decimalToMinor(amount float64, minorUnits int, currency, field string) (int64, error) {
	v, err := money.FromFloat(amount, minorUnits)
	if errors.Is(err, money.ErrTooPrecise) {
		return 0, fmt.Errorf("%s has too many decimals for %s (max %d)", field, currency, minorUnits)
	}
	if err != nil {
		return 0, fmt.Errorf("%s is not a valid amount", field)
	}
	return v, nil
}

// friendly turns domain errors into messages the model can act on.
func friendly(err error) error {
	var domainErr *finance.Error
	if errors.As(err, &domainErr) {
		return errors.New(domainErr.Error())
	}
	return err
}

func periodRange(today time.Time, period string) (string, string, error) {
	const layout = time.DateOnly
	first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	switch period {
	case "", "this_month":
		return first.Format(layout), first.AddDate(0, 1, -1).Format(layout), nil
	case "last_month":
		start := first.AddDate(0, -1, 0)
		return start.Format(layout), first.AddDate(0, 0, -1).Format(layout), nil
	case "this_year":
		return time.Date(today.Year(), 1, 1, 0, 0, 0, 0, time.UTC).Format(layout), time.Date(today.Year(), 12, 31, 0, 0, 0, 0, time.UTC).Format(layout), nil
	case "last_30_days":
		return today.AddDate(0, 0, -29).Format(layout), today.Format(layout), nil
	default:
		return "", "", fmt.Errorf("unknown period %q; use this_month, last_month, this_year or last_30_days", period)
	}
}

func accountToOut(a finance.Account) accountOut {
	return accountOut{ID: a.ID.String(), Name: a.Name, Type: a.Type, Currency: a.Currency, Balance: money.Format(a.Balance, a.MinorUnits)}
}

func categoryToOut(c finance.Category) categoryOut {
	return categoryOut{ID: c.ID.String(), Name: c.Name, Kind: c.Kind}
}

func transactionToOut(tx finance.Transaction) transactionOut {
	out := transactionOut{
		ID:          tx.ID.String(),
		Type:        tx.Type,
		Date:        tx.OccurredOn,
		Amount:      money.Format(tx.Amount, tx.MinorUnits),
		Currency:    tx.Currency,
		Account:     tx.AccountName,
		Description: tx.Description,
	}
	if tx.CategoryName != nil {
		out.Category = *tx.CategoryName
	}
	if tx.DestinationAccountName != nil {
		out.ToAccount = *tx.DestinationAccountName
	}
	if tx.DestinationAmount != nil && tx.DestinationMinorUnits != nil && tx.DestinationCurrency != nil {
		out.DestinationAmount = money.Format(*tx.DestinationAmount, *tx.DestinationMinorUnits)
		out.DestinationCurrency = *tx.DestinationCurrency
	}
	if tx.CreatedBy != nil {
		out.RecordedBy = tx.CreatedBy.Name
		if out.RecordedBy == "" {
			out.RecordedBy = tx.CreatedBy.Email
		}
	}
	return out
}

func categoryLabel(name *string) string {
	if name == nil {
		return "Uncategorized"
	}
	return *name
}

func categorySuffix(category string) string {
	if category == "" {
		return ""
	}
	return " (" + category + ")"
}

func descriptionSuffix(description string) string {
	if description == "" {
		return ""
	}
	return " — " + description
}

func text(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}
