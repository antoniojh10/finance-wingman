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
	Account     string  `json:"account,omitempty" jsonschema:"Account name or ID. When several members have an account with that name, the user's own is used; name another member's as shown by list_accounts, e.g. BNP (Ana). Optional when only one active account exists"`
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
	InitialBalance *float64 `json:"initial_balance,omitempty" jsonschema:"The balance the account holds on balance_as_of, in the account currency, e.g. 1500.00. May be negative (e.g. credit card debt). Defaults to 0. Transactions dated on or before balance_as_of are already included in it"`
	BalanceAsOf    string   `json:"balance_as_of,omitempty" jsonschema:"Date (YYYY-MM-DD) the initial_balance refers to. Defaults to today. Only transactions dated after it change the balance, so past transactions can be backfilled without altering it"`
	Owner          string   `json:"owner,omitempty" jsonschema:"Who the account belongs to: \"me\" (default, the user you are helping), \"shared\" (joint or household account), or a workspace member's name or email. Members can each have an account with the same name"`
}

type updateAccountArgs struct {
	Account        string   `json:"account" jsonschema:"Name or ID of the account to edit. Archived accounts are accepted too, so they can be unarchived"`
	Name           *string  `json:"name,omitempty" jsonschema:"New account name. Must not clash with another active account of the same owner"`
	Type           *string  `json:"type,omitempty" jsonschema:"New type: one of checking, savings, credit_card, cash, investment, other"`
	InitialBalance *float64 `json:"initial_balance,omitempty" jsonschema:"New balance on balance_as_of (the current anchor date unless balance_as_of is also passed), in the account currency, e.g. 1500.00. May be negative. Transactions are kept"`
	BalanceAsOf    *string  `json:"balance_as_of,omitempty" jsonschema:"New date (YYYY-MM-DD) the initial_balance refers to. To say the balance is X today, pass initial_balance X and balance_as_of today. Editing initial_balance alone keeps the existing date"`
	Archived       *bool    `json:"archived,omitempty" jsonschema:"true to archive the account (hidden from list_accounts and unusable for new transactions, history is kept), false to restore it"`
	Owner          *string  `json:"owner,omitempty" jsonschema:"New owner: \"me\", \"shared\", or a workspace member's name or email"`
}

type listAccountsArgs struct {
	Owner string `json:"owner,omitempty" jsonschema:"Only accounts of this owner: \"me\", \"shared\", or a workspace member's name or email. Omit for all accounts"`
}

type createCategoryArgs struct {
	Name  string `json:"name" jsonschema:"Category name, e.g. Groceries"`
	Kind  string `json:"kind" jsonschema:"expense or income"`
	Color string `json:"color,omitempty" jsonschema:"Optional hex color like #22c55e"`
}

type updateCategoryArgs struct {
	Category string  `json:"category" jsonschema:"Name or ID of the category to edit"`
	Name     *string `json:"name,omitempty" jsonschema:"New category name. Must not clash with another active category of the same kind"`
	Color    *string `json:"color,omitempty" jsonschema:"Hex color like #22c55e, or empty string to clear it"`
	Icon     *string `json:"icon,omitempty" jsonschema:"Icon name, or empty string to clear it"`
	Archived *bool   `json:"archived,omitempty" jsonschema:"true to archive the category (hidden from list_categories, but transactions stay categorized), false to restore it"`
}

type createCategoriesArgs struct {
	Items []createCategoryArgs `json:"items" jsonschema:"Categories to create (1 to 100). All are created or none are"`
}

type createAccountsArgs struct {
	Items []createAccountArgs `json:"items" jsonschema:"Accounts to create (1 to 100). All are created or none are"`
}

type batchTransactionArgs struct {
	Type              string   `json:"type" jsonschema:"expense, income or transfer"`
	Amount            float64  `json:"amount" jsonschema:"Positive amount in the source account currency, e.g. 150.50"`
	Account           string   `json:"account,omitempty" jsonschema:"Account name or ID (the source account for transfers). Optional when only one active account exists"`
	ToAccount         string   `json:"to_account,omitempty" jsonschema:"Destination account name or ID. Required for transfers only"`
	DestinationAmount *float64 `json:"destination_amount,omitempty" jsonschema:"Amount received in the destination currency. Required only for transfers between different currencies"`
	Category          string   `json:"category,omitempty" jsonschema:"Existing category name matching the type (not for transfers). Optional"`
	Description       string   `json:"description,omitempty" jsonschema:"Short note, e.g. the merchant or what was bought"`
	Date              string   `json:"date,omitempty" jsonschema:"Date in YYYY-MM-DD format. Defaults to today"`
}

type addTransactionsArgs struct {
	Items []batchTransactionArgs `json:"items" jsonschema:"Transactions to record (1 to 100). All are recorded or none are"`
}

type summaryArgs struct {
	Period string `json:"period,omitempty" jsonschema:"One of this_month (default), last_month, this_year, last_30_days. Ignored when from/to are given"`
	From   string `json:"from,omitempty" jsonschema:"Start date YYYY-MM-DD (inclusive)"`
	To     string `json:"to,omitempty" jsonschema:"End date YYYY-MM-DD (inclusive)"`
	Owner  string `json:"owner,omitempty" jsonschema:"Only accounts of this owner: \"me\", \"shared\", or a workspace member's name or email. Omit for the whole workspace (the default)"`
}

type listTransactionsArgs struct {
	Limit    int    `json:"limit,omitempty" jsonschema:"Maximum number of transactions (default 10, max 50)"`
	Account  string `json:"account,omitempty" jsonschema:"Account name or ID"`
	Category string `json:"category,omitempty" jsonschema:"Category name"`
	Type     string `json:"type,omitempty" jsonschema:"expense, income or transfer"`
	From     string `json:"from,omitempty" jsonschema:"Start date YYYY-MM-DD"`
	To       string `json:"to,omitempty" jsonschema:"End date YYYY-MM-DD"`
	Search   string `json:"search,omitempty" jsonschema:"Text to search in descriptions"`
	Owner    string `json:"owner,omitempty" jsonschema:"Only transactions on accounts of this owner (either side of a transfer): \"me\", \"shared\", or a workspace member's name or email"`
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
	Balance  string `json:"balance" jsonschema:"Current balance: initial balance plus transactions dated after balance_as_of"`
	Owner    string `json:"owner" jsonschema:"Workspace member the account belongs to, or shared"`
	// BalanceAsOf is the date the account's initial balance refers to.
	BalanceAsOf string `json:"balance_as_of"`
	Archived    bool   `json:"archived,omitempty"`
}

type accountsOut struct {
	Accounts []accountOut `json:"accounts"`
}

type categoryOut struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Archived bool   `json:"archived,omitempty"`
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
	RecurringID         string `json:"recurring_id,omitempty"`
	RecurringName       string `json:"recurring_name,omitempty"`
	RecurringDueOn      string `json:"recurring_due_on,omitempty"`
	// RecurringMatch is set by add_expense and add_income when the new
	// transaction looks like a payment of an active recurring item.
	RecurringMatch *recurringMatchOut `json:"recurring_match,omitempty"`
	// BudgetWarning is set when this expense leaves its category near or
	// over its monthly budget.
	BudgetWarning *budgetWarningOut `json:"budget_warning,omitempty"`
}

type transactionsOut struct {
	Transactions []transactionOut `json:"transactions"`
	Total        int64            `json:"total"`
	// BudgetWarnings lists the categories left near or over budget by the
	// expenses of add_transactions and update_transactions.
	BudgetWarnings []budgetWarningOut `json:"budget_warnings,omitempty"`
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
	addTool(s, &mcp.Tool{
		Name:        "add_expense",
		Title:       "Add expense",
		Description: "Record an expense (money spent) in an account, optionally with a category.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args recordArgs) (*mcp.CallToolResult, transactionOut, error) {
		return s.record(ctx, finance.TypeExpense, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "add_income",
		Title:       "Add income",
		Description: "Record income (money received, e.g. salary) in an account, optionally with a category.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args recordArgs) (*mcp.CallToolResult, transactionOut, error) {
		return s.record(ctx, finance.TypeIncome, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "add_transfer",
		Title:       "Transfer between accounts",
		Description: "Move money between two of the workspace's accounts. Transfers are not counted as income or expenses.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args transferArgs) (*mcp.CallToolResult, transactionOut, error) {
		return s.transfer(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "list_accounts",
		Title:       "List accounts",
		Description: "List active accounts with their owner (a workspace member, or shared), currency and current balance (initial balance plus transactions dated after balance_as_of).",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listAccountsArgs) (*mcp.CallToolResult, accountsOut, error) {
		return s.listAccounts(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "list_categories",
		Title:       "List categories",
		Description: "List active expense and income categories.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listCategoriesArgs) (*mcp.CallToolResult, categoriesOut, error) {
		return s.listCategories(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "create_account",
		Title:       "Create account",
		Description: "Create a new account (bank account, card, cash, ...) with its currency and initial_balance, the balance it holds on balance_as_of (default today): transactions dated on or before that date are already included in it and do not change the balance, so past transactions can be added later for history. Only create accounts the user asked for; check list_accounts first to avoid duplicates.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args createAccountArgs) (*mcp.CallToolResult, accountOut, error) {
		return s.createAccount(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "update_account",
		Title:       "Update account",
		Description: "Edit an existing account in place, keeping its transactions: rename it, change its type, fix its initial_balance and/or the balance_as_of date it refers to, or archive/unarchive it. Only the fields you pass change; the currency can never be changed. Editing initial_balance alone keeps balance_as_of; to set the balance as of today pass both. Archived accounts disappear from list_accounts and cannot receive new transactions. Confirm with the user before changing a balance or archiving.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args updateAccountArgs) (*mcp.CallToolResult, accountOut, error) {
		return s.updateAccount(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "create_category",
		Title:       "Create category",
		Description: "Create a new expense or income category. Only create categories the user asked for; check list_categories first to avoid duplicates.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args createCategoryArgs) (*mcp.CallToolResult, categoryOut, error) {
		return s.createCategory(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "update_category",
		Title:       "Update category",
		Description: "Edit an existing category in place: rename it, change its color or icon, or archive/unarchive it. Only the fields you pass change; the kind (expense or income) cannot be changed. Archived categories disappear from list_categories but their transactions stay categorized. Confirm with the user before archiving.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args updateCategoryArgs) (*mcp.CallToolResult, categoryOut, error) {
		return s.updateCategory(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "create_categories",
		Title:       "Create several categories",
		Description: "Create up to 100 expense or income categories at once. All-or-nothing: if any item is invalid nothing is created. Check list_categories first to avoid duplicates.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args createCategoriesArgs) (*mcp.CallToolResult, categoriesOut, error) {
		return s.createCategories(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "create_accounts",
		Title:       "Create several accounts",
		Description: "Create up to 100 accounts at once. All-or-nothing: if any item is invalid nothing is created. Check list_accounts first to avoid duplicates.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args createAccountsArgs) (*mcp.CallToolResult, accountsOut, error) {
		return s.createAccounts(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "add_transactions",
		Title:       "Add several transactions",
		Description: "Record up to 100 expenses, incomes or transfers at once; each item has its own type. All-or-nothing: if any item is invalid nothing is recorded, and the error names the failing item.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, req *mcp.CallToolRequest, args addTransactionsArgs) (*mcp.CallToolResult, transactionsOut, error) {
		return s.addTransactions(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "get_summary",
		Title:       "Get summary",
		Description: "Income, expenses and net per currency for a period, broken down by category, plus current balances.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args summaryArgs) (*mcp.CallToolResult, summaryOut, error) {
		return s.summary(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "list_transactions",
		Title:       "List transactions",
		Description: "List recent transactions, newest first, with optional filters.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listTransactionsArgs) (*mcp.CallToolResult, transactionsOut, error) {
		return s.listTransactions(ctx, args)
	})

	addTool(s, &mcp.Tool{
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
	msg := fmt.Sprintf("%s of %s %s in %s%s on %s (id %s).", verb, out.Amount, out.Currency, out.Account, categorySuffix(out.Category), out.Date, out.ID)
	if m := s.matchHint(ctx, tx.ID); m != nil {
		out.RecurringMatch = m
		msg += fmt.Sprintf(" This looks like a payment of the recurring item %s (due %s, estimated %s %s) but it is NOT linked. Ask the user whether to link it with link_transaction_to_recurring.", m.Recurring, m.Period, m.EstimatedAmount, out.Currency)
	}
	if w := s.budgetWarnings(ctx, []finance.Transaction{tx}); len(w) > 0 {
		out.BudgetWarning = &w[0]
		msg += "\n" + w[0].message()
	}
	return text(msg), out, nil
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

func (s *Server) listAccounts(ctx context.Context, args listAccountsArgs) (*mcp.CallToolResult, accountsOut, error) {
	owner, err := s.ownerFilter(ctx, args.Owner)
	if err != nil {
		return nil, accountsOut{}, err
	}
	accounts, err := s.finance.ListOwnedAccounts(ctx, false, owner)
	if err != nil {
		return nil, accountsOut{}, err
	}
	out := accountsOut{Accounts: make([]accountOut, len(accounts))}
	lines := make([]string, len(accounts))
	for i, a := range accounts {
		out.Accounts[i] = accountToOut(a)
		lines[i] = fmt.Sprintf("- %s (%s, %s, %s): %s %s", a.Name, ownerLabel(a.Owner), a.Type, a.Currency, out.Accounts[i].Balance, a.Currency)
	}
	if len(lines) == 0 && args.Owner != "" {
		return text("No active accounts belong to that owner."), out, nil
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
	owner, err := s.resolveOwner(ctx, args.Owner)
	if err != nil {
		return nil, accountOut{}, err
	}
	in := finance.CreateAccountInput{Name: args.Name, Type: args.Type, Currency: args.Currency, BalanceAsOf: args.BalanceAsOf, Owner: owner}
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
	return text(fmt.Sprintf("Created %s account %s in %s (owner: %s) with a balance of %s %s (id %s).", out.Type, out.Name, out.Currency, out.Owner, out.Balance, out.Currency, out.ID)), out, nil
}

func (s *Server) updateAccount(ctx context.Context, args updateAccountArgs) (*mcp.CallToolResult, accountOut, error) {
	if args.Name == nil && args.Type == nil && args.InitialBalance == nil && args.BalanceAsOf == nil && args.Archived == nil && args.Owner == nil {
		return nil, accountOut{}, errors.New("nothing to update; pass at least one of name, type, initial_balance, balance_as_of, archived or owner")
	}
	account, err := s.resolveAnyAccount(ctx, args.Account)
	if err != nil {
		return nil, accountOut{}, err
	}
	in := finance.UpdateAccountInput{Name: args.Name, Type: args.Type, BalanceAsOf: args.BalanceAsOf, Archived: args.Archived}
	if args.Owner != nil {
		owner, err := s.resolveOwner(ctx, *args.Owner)
		if err != nil {
			return nil, accountOut{}, err
		}
		if owner == "" {
			return nil, accountOut{}, errors.New("owner must not be empty; pass \"me\", \"shared\" or a member's name or email")
		}
		in.Owner = &owner
	}
	if args.InitialBalance != nil {
		v, err := decimalToMinor(*args.InitialBalance, account.MinorUnits, account.Currency, "initial_balance")
		if err != nil {
			return nil, accountOut{}, err
		}
		in.InitialBalance = &v
	}
	updated, err := s.finance.UpdateAccount(ctx, account.ID, in)
	if err != nil {
		return nil, accountOut{}, friendly(err)
	}
	out := accountToOut(updated)
	state := ""
	if updated.Archived {
		state = " It is archived."
	}
	return text(fmt.Sprintf("Updated %s account %s in %s (owner: %s): balance is now %s %s (id %s).%s", out.Type, out.Name, out.Currency, out.Owner, out.Balance, out.Currency, out.ID, state)), out, nil
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

func (s *Server) updateCategory(ctx context.Context, args updateCategoryArgs) (*mcp.CallToolResult, categoryOut, error) {
	if args.Name == nil && args.Color == nil && args.Icon == nil && args.Archived == nil {
		return nil, categoryOut{}, errors.New("nothing to update; pass at least one of name, color, icon or archived")
	}
	category, err := s.resolveAnyCategory(ctx, args.Category)
	if err != nil {
		return nil, categoryOut{}, err
	}
	in := finance.UpdateCategoryInput{Name: args.Name, Color: args.Color, Icon: args.Icon, Archived: args.Archived}
	updated, err := s.finance.UpdateCategory(ctx, category.ID, in)
	if err != nil {
		return nil, categoryOut{}, friendly(err)
	}
	out := categoryToOut(updated)
	state := ""
	if updated.Archived {
		state = " It is archived."
	}
	return text(fmt.Sprintf("Updated %s category %s (id %s).%s", out.Kind, out.Name, out.ID, state)), out, nil
}

func (s *Server) createCategories(ctx context.Context, args createCategoriesArgs) (*mcp.CallToolResult, categoriesOut, error) {
	if err := checkBatch(len(args.Items)); err != nil {
		return nil, categoriesOut{}, err
	}
	inputs := make([]finance.CreateCategoryInput, len(args.Items))
	for i, item := range args.Items {
		inputs[i] = finance.CreateCategoryInput{Name: item.Name, Kind: strings.ToLower(strings.TrimSpace(item.Kind))}
		if item.Color != "" {
			inputs[i].Color = &args.Items[i].Color
		}
	}
	created, err := s.finance.CreateCategories(ctx, inputs)
	if err != nil {
		return nil, categoriesOut{}, batchFailure(err)
	}
	out := categoriesOut{Categories: make([]categoryOut, len(created))}
	lines := make([]string, len(created))
	for i, c := range created {
		out.Categories[i] = categoryToOut(c)
		lines[i] = fmt.Sprintf("- %s (%s)", c.Name, c.Kind)
	}
	return text(fmt.Sprintf("Created %d categories:\n%s", len(created), strings.Join(lines, "\n"))), out, nil
}

func (s *Server) createAccounts(ctx context.Context, args createAccountsArgs) (*mcp.CallToolResult, accountsOut, error) {
	if err := checkBatch(len(args.Items)); err != nil {
		return nil, accountsOut{}, err
	}
	inputs := make([]finance.CreateAccountInput, len(args.Items))
	for i, item := range args.Items {
		owner, err := s.resolveOwner(ctx, item.Owner)
		if err != nil {
			return nil, accountsOut{}, batchFailure(itemErr(i, "owner", err))
		}
		inputs[i] = finance.CreateAccountInput{Name: item.Name, Type: item.Type, Currency: item.Currency, BalanceAsOf: item.BalanceAsOf, Owner: owner}
		if item.InitialBalance != nil {
			currency, err := s.findCurrency(ctx, item.Currency)
			if err != nil {
				return nil, accountsOut{}, batchFailure(itemErr(i, "currency", err))
			}
			inputs[i].InitialBalance, err = decimalToMinor(*item.InitialBalance, currency.MinorUnits, currency.Code, "initial_balance")
			if err != nil {
				return nil, accountsOut{}, batchFailure(itemErr(i, "", err))
			}
		}
	}
	created, err := s.finance.CreateAccounts(ctx, inputs)
	if err != nil {
		return nil, accountsOut{}, batchFailure(err)
	}
	out := accountsOut{Accounts: make([]accountOut, len(created))}
	lines := make([]string, len(created))
	for i, a := range created {
		out.Accounts[i] = accountToOut(a)
		lines[i] = fmt.Sprintf("- %s (%s, %s, %s): %s %s", a.Name, ownerLabel(a.Owner), a.Type, a.Currency, out.Accounts[i].Balance, a.Currency)
	}
	return text(fmt.Sprintf("Created %d accounts:\n%s", len(created), strings.Join(lines, "\n"))), out, nil
}

func (s *Server) addTransactions(ctx context.Context, args addTransactionsArgs) (*mcp.CallToolResult, transactionsOut, error) {
	if err := checkBatch(len(args.Items)); err != nil {
		return nil, transactionsOut{}, err
	}
	inputs := make([]finance.TransactionInput, len(args.Items))
	for i, item := range args.Items {
		in, err := s.batchTransactionInput(ctx, item)
		if err != nil {
			return nil, transactionsOut{}, batchFailure(itemErr(i, "", err))
		}
		inputs[i] = in
	}
	created, err := s.finance.CreateTransactions(ctx, inputs)
	if err != nil {
		return nil, transactionsOut{}, batchFailure(err)
	}
	out := transactionsOut{Transactions: make([]transactionOut, len(created)), Total: int64(len(created))}
	lines := make([]string, len(created))
	for i, tx := range created {
		t := transactionToOut(tx)
		out.Transactions[i] = t
		target := t.Account
		if t.ToAccount != "" {
			target += " → " + t.ToAccount
		}
		lines[i] = fmt.Sprintf("- %s %s %s %s · %s%s%s [id %s]", t.Date, t.Type, t.Amount, t.Currency, target, categorySuffix(t.Category), descriptionSuffix(t.Description), t.ID)
	}
	out.BudgetWarnings = s.budgetWarnings(ctx, created)
	return text(fmt.Sprintf("Recorded %d transactions:\n%s%s", len(created), strings.Join(lines, "\n"), warningText(out.BudgetWarnings))), out, nil
}

// batchTransactionInput resolves account and category names and converts
// decimal amounts for one item of add_transactions.
func (s *Server) batchTransactionInput(ctx context.Context, item batchTransactionArgs) (finance.TransactionInput, error) {
	typ := strings.ToLower(strings.TrimSpace(item.Type))
	if typ != finance.TypeExpense && typ != finance.TypeIncome && typ != finance.TypeTransfer {
		return finance.TransactionInput{}, errors.New("type must be expense, income or transfer")
	}
	account, err := s.resolveAccount(ctx, item.Account, "account")
	if err != nil {
		return finance.TransactionInput{}, err
	}
	amount, err := toMinor(item.Amount, account.MinorUnits, account.Currency, "amount")
	if err != nil {
		return finance.TransactionInput{}, err
	}
	in := finance.TransactionInput{
		Type:        typ,
		AccountID:   account.ID,
		Amount:      amount,
		Description: item.Description,
		OccurredOn:  item.Date,
	}
	if typ == finance.TypeTransfer {
		if strings.TrimSpace(item.Category) != "" {
			return in, errors.New("transfers cannot have a category; omit category")
		}
		if strings.TrimSpace(item.ToAccount) == "" {
			return in, errors.New("to_account is required for transfers")
		}
		to, err := s.resolveAccount(ctx, item.ToAccount, "to_account")
		if err != nil {
			return in, err
		}
		in.DestinationAccountID = &to.ID
		if item.DestinationAmount != nil {
			dest, err := toMinor(*item.DestinationAmount, to.MinorUnits, to.Currency, "destination_amount")
			if err != nil {
				return in, err
			}
			in.DestinationAmount = &dest
		} else if account.Currency != to.Currency {
			return in, fmt.Errorf("%s uses %s and %s uses %s: ask the user how much %s was received and pass destination_amount", account.Name, account.Currency, to.Name, to.Currency, to.Currency)
		}
		return in, nil
	}
	if strings.TrimSpace(item.ToAccount) != "" || item.DestinationAmount != nil {
		return in, errors.New("to_account and destination_amount are only for transfers; omit them")
	}
	if strings.TrimSpace(item.Category) != "" {
		category, err := s.resolveCategory(ctx, item.Category, typ)
		if err != nil {
			return in, err
		}
		in.CategoryID = &category.ID
	}
	return in, nil
}

func (s *Server) summary(ctx context.Context, args summaryArgs) (*mcp.CallToolResult, summaryOut, error) {
	from, to := args.From, args.To
	if from == "" && to == "" {
		var err error
		if from, to, err = periodRange(s.finance.Today(), args.Period); err != nil {
			return nil, summaryOut{}, err
		}
	}
	owner, err := s.ownerFilter(ctx, args.Owner)
	if err != nil {
		return nil, summaryOut{}, err
	}
	sum, err := s.finance.Summary(ctx, from, to, owner)
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
	owner, err := s.ownerFilter(ctx, args.Owner)
	if err != nil {
		return nil, transactionsOut{}, err
	}
	filter := finance.TransactionFilter{Limit: min(limit, 50), Owner: owner}
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
	var recurringNames map[uuid.UUID]string
	for _, tx := range page.Items {
		if tx.RecurringID != nil {
			if recurringNames, err = s.recurringNames(ctx); err != nil {
				return nil, transactionsOut{}, err
			}
			break
		}
	}
	for i, tx := range page.Items {
		t := transactionToOut(tx)
		if tx.RecurringID != nil {
			t.RecurringName = recurringNames[*tx.RecurringID]
		}
		out.Transactions[i] = t
		target := t.Account
		if t.ToAccount != "" {
			target += " → " + t.ToAccount
		}
		lines[i] = fmt.Sprintf("- %s %s %s %s · %s%s%s [id %s]", t.Date, t.Type, t.Amount, t.Currency, target, categorySuffix(t.Category), descriptionSuffix(t.Description)+recurringSuffix(t), t.ID)
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

// resolveAccount finds an active account by ID, case-insensitive name or
// label ("BNP (Antonio)"). When several members have an account with that
// name, the caller's own wins. An empty reference resolves to the only
// active account, if there is exactly one.
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
	var exact, partial []finance.Account
	for _, a := range accounts {
		switch {
		case a.ID.String() == ref:
			return a, nil
		case strings.EqualFold(a.Name, ref) || strings.EqualFold(accountLabel(a), ref):
			exact = append(exact, a)
		case strings.Contains(strings.ToLower(a.Name), strings.ToLower(ref)):
			partial = append(partial, a)
		}
	}
	if len(exact) > 0 {
		if mine := preferMine(ctx, exact); len(mine) == 1 {
			return mine[0], nil
		}
		return finance.Account{}, ambiguousAccounts(ref, exact)
	}
	if mine := preferMine(ctx, partial); len(mine) == 1 {
		return mine[0], nil
	}
	return finance.Account{}, fmt.Errorf("no account matches %q for %s; available accounts: %s", ref, field, accountNames(accounts))
}

// resolveAnyAccount finds an account by ID, name or label among all
// accounts, archived ones included (so they can be unarchived). Active
// accounts win over archived ones with the same name, and the caller's own
// account over other members' ones.
func (s *Server) resolveAnyAccount(ctx context.Context, ref string) (finance.Account, error) {
	accounts, err := s.finance.ListAccounts(ctx, true)
	if err != nil {
		return finance.Account{}, err
	}
	if len(accounts) == 0 {
		return finance.Account{}, errors.New("there are no accounts; create one with create_account first")
	}
	ref = strings.TrimSpace(ref)
	var exact, partial []finance.Account
	for _, a := range accounts {
		switch {
		case a.ID.String() == ref || strings.EqualFold(a.Name, ref) || strings.EqualFold(accountLabel(a), ref):
			exact = append(exact, a)
		case ref != "" && strings.Contains(strings.ToLower(a.Name), strings.ToLower(ref)):
			partial = append(partial, a)
		}
	}
	var active []finance.Account
	for _, a := range exact {
		if !a.Archived {
			active = append(active, a)
		}
	}
	if len(active) > 0 {
		if mine := preferMine(ctx, active); len(mine) == 1 {
			return mine[0], nil
		}
		return finance.Account{}, ambiguousAccounts(ref, active)
	}
	if len(exact) > 0 {
		return exact[0], nil
	}
	if len(partial) == 1 {
		return partial[0], nil
	}
	return finance.Account{}, fmt.Errorf("no account matches %q for account; available accounts: %s", ref, accountNamesWithState(accounts))
}

func accountNamesWithState(accounts []finance.Account) string {
	names := make([]string, len(accounts))
	for i, a := range accounts {
		names[i] = accountWithDetails(a)
	}
	return strings.Join(names, ", ")
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

// resolveAnyCategory finds a category by ID or name among all categories,
// archived ones included (so they can be unarchived). Active categories win
// over archived ones with the same name.
func (s *Server) resolveAnyCategory(ctx context.Context, ref string) (finance.Category, error) {
	categories, err := s.finance.ListCategories(ctx, nil, true)
	if err != nil {
		return finance.Category{}, err
	}
	if len(categories) == 0 {
		return finance.Category{}, errors.New("there are no categories; create one with create_category first")
	}
	ref = strings.TrimSpace(ref)
	var exact, partial []finance.Category
	for _, c := range categories {
		switch {
		case c.ID.String() == ref || strings.EqualFold(c.Name, ref):
			exact = append(exact, c)
		case ref != "" && strings.Contains(strings.ToLower(c.Name), strings.ToLower(ref)):
			partial = append(partial, c)
		}
	}
	var active []finance.Category
	for _, c := range exact {
		if !c.Archived {
			active = append(active, c)
		}
	}
	if len(active) == 1 {
		return active[0], nil
	}
	if len(active) > 1 {
		matches := make([]string, len(active))
		for i, c := range active {
			matches[i] = fmt.Sprintf("%s (%s, id %s)", c.Name, c.Kind, c.ID)
		}
		return finance.Category{}, fmt.Errorf("several categories are named %q: %s; pass the id instead", ref, strings.Join(matches, ", "))
	}
	if len(exact) > 0 {
		return exact[0], nil
	}
	if len(partial) == 1 {
		return partial[0], nil
	}
	names := make([]string, len(categories))
	for i, c := range categories {
		names[i] = c.Name
		if c.Archived {
			names[i] = names[i] + " (archived)"
		}
	}
	return finance.Category{}, fmt.Errorf("no category matches %q; available categories: %s", ref, strings.Join(names, ", "))
}

func accountNames(accounts []finance.Account) string {
	names := make([]string, len(accounts))
	for i, a := range accounts {
		names[i] = accountWithDetails(a)
	}
	return strings.Join(names, ", ")
}

// accountWithDetails names an account with its currency, owner and state,
// e.g. "BNP (EUR, Antonio)" or "Cash (MXN, archived)".
func accountWithDetails(a finance.Account) string {
	details := []string{a.Currency}
	if a.Owner != nil {
		details = append(details, ownerDisplay(a.Owner))
	}
	if a.Archived {
		details = append(details, "archived")
	}
	return fmt.Sprintf("%s (%s)", a.Name, strings.Join(details, ", "))
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

func checkBatch(n int) error {
	if n == 0 {
		return errors.New("items must contain at least one item")
	}
	if n > finance.MaxBatchSize {
		return fmt.Errorf("items must contain at most %d items, got %d; split the call into several batches", finance.MaxBatchSize, n)
	}
	return nil
}

// itemErr prefixes an error with the index of the failing batch item (and
// the field, when known) so the model knows what to fix.
func itemErr(index int, field string, err error) error {
	if field != "" {
		return fmt.Errorf("items[%d].%s: %w", index, field, err)
	}
	return fmt.Errorf("items[%d]: %w", index, err)
}

// batchFailure explains that a failed batch created nothing.
func batchFailure(err error) error {
	return fmt.Errorf("nothing was created: %s. Fix that item and resend the whole batch", friendly(err).Error())
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
	return accountOut{ID: a.ID.String(), Name: a.Name, Type: a.Type, Currency: a.Currency, Owner: ownerLabel(a.Owner), Balance: money.Format(a.Balance, a.MinorUnits), BalanceAsOf: a.BalanceAsOf, Archived: a.Archived}
}

func categoryToOut(c finance.Category) categoryOut {
	return categoryOut{ID: c.ID.String(), Name: c.Name, Kind: c.Kind, Archived: c.Archived}
}

func transactionToOut(tx finance.Transaction) transactionOut {
	out := transactionOut{
		ID:          tx.ID.String(),
		Type:        tx.Type,
		Date:        tx.OccurredOn,
		Amount:      money.Format(tx.Amount, tx.MinorUnits),
		Currency:    tx.Currency,
		Account:     ownedName(tx.AccountName, tx.AccountOwner),
		Description: tx.Description,
	}
	if tx.CategoryName != nil {
		out.Category = *tx.CategoryName
	}
	if tx.RecurringID != nil {
		out.RecurringID = tx.RecurringID.String()
	}
	if tx.RecurringDueOn != nil {
		out.RecurringDueOn = *tx.RecurringDueOn
	}
	if tx.DestinationAccountName != nil {
		out.ToAccount = ownedName(*tx.DestinationAccountName, tx.DestinationOwner)
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

func recurringSuffix(t transactionOut) string {
	if t.RecurringName == "" {
		return ""
	}
	return " (pays recurring " + t.RecurringName + " due " + t.RecurringDueOn + ")"
}

func text(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}
