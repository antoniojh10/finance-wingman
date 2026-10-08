package httpapi

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

const apiPrefix = "/api/v1"

type financeHandlers struct {
	svc    *finance.Service
	logger *slog.Logger
}

type idInput struct {
	ID string `path:"id" format:"uuid"`
}

type bodyOutput[T any] struct {
	Body T
}

type listOutput[T any] struct {
	Body struct {
		Items []T `json:"items"`
	}
}

func newList[T any](items []T) *listOutput[T] {
	out := &listOutput[T]{}
	out.Body.Items = items
	return out
}

// batchInput is the request body of the batch create endpoints.
type batchInput[T any] struct {
	Body struct {
		Items []T `json:"items" minItems:"1" maxItems:"100" doc:"Items to create (1 to 100). All are created or none are"`
	}
}

func registerFinance(api huma.API, svc *finance.Service, logger *slog.Logger) {
	h := &financeHandlers{svc: svc, logger: logger}
	h.registerCurrencies(api)
	h.registerAccounts(api)
	h.registerCategories(api)
	h.registerTransactions(api)
	h.registerRecurring(api)
	h.registerTransactionLinks(api)
	h.registerSummary(api)
	h.registerBudgets(api)
	h.registerActivity(api)
}

func (h *financeHandlers) fail(ctx context.Context, err error) error {
	return toHTTPError(ctx, h.logger, err)
}

// --- Currencies ---

func (h *financeHandlers) registerCurrencies(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "list-currencies",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/currencies",
		Summary:     "List supported currencies",
		Tags:        []string{"Currencies"},
	}, func(ctx context.Context, _ *struct{}) (*listOutput[finance.Currency], error) {
		items, err := h.svc.ListCurrencies(ctx)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return newList(items), nil
	})
}

// --- Accounts ---

type listAccountsInput struct {
	IncludeArchived bool   `query:"include_archived" doc:"Include archived accounts"`
	Owner           string `query:"owner" doc:"\"me\", \"shared\" or a member's user id; omit for every owner"`
}

type createAccountInput struct {
	Body finance.CreateAccountInput
}

type updateAccountInput struct {
	ID   string `path:"id" format:"uuid"`
	Body finance.UpdateAccountInput
}

func (h *financeHandlers) registerAccounts(api huma.API) {
	tags := []string{"Accounts"}

	huma.Register(api, huma.Operation{
		OperationID: "list-accounts",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/accounts",
		Summary:     "List accounts with their current balance",
		Tags:        tags,
	}, func(ctx context.Context, in *listAccountsInput) (*listOutput[finance.Account], error) {
		owner, err := finance.ParseOwnerFilter(ctx, "query.owner", in.Owner)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		items, err := h.svc.ListOwnedAccounts(ctx, in.IncludeArchived, owner)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return newList(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-account",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/accounts",
		Summary:       "Create an account",
		Tags:          tags,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createAccountInput) (*bodyOutput[finance.Account], error) {
		account, err := h.svc.CreateAccount(ctx, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[finance.Account]{Body: account}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-account",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/accounts/{id}",
		Summary:     "Get an account",
		Tags:        tags,
	}, func(ctx context.Context, in *idInput) (*bodyOutput[finance.Account], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		account, err := h.svc.GetAccount(ctx, id)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[finance.Account]{Body: account}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-account",
		Method:      http.MethodPatch,
		Path:        apiPrefix + "/accounts/{id}",
		Summary:     "Update an account",
		Description: "Partially updates an account. The currency cannot be changed. Set archived to true to hide the account.",
		Tags:        tags,
	}, func(ctx context.Context, in *updateAccountInput) (*bodyOutput[finance.Account], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		account, err := h.svc.UpdateAccount(ctx, id, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[finance.Account]{Body: account}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-account",
		Method:        http.MethodDelete,
		Path:          apiPrefix + "/accounts/{id}",
		Summary:       "Delete an account",
		Description:   "Permanently deletes an account without transactions. Accounts with transactions must be archived instead.",
		Tags:          tags,
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *idInput) (*struct{}, error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		if err := h.svc.DeleteAccount(ctx, id); err != nil {
			return nil, h.fail(ctx, err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-accounts-batch",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/accounts/batch",
		Summary:       "Create several accounts",
		Description:   "Creates up to 100 accounts in a single transaction: if any item is invalid, nothing is created. Errors point at the failing item, e.g. items[3].name.",
		Tags:          tags,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *batchInput[finance.CreateAccountInput]) (*listOutput[finance.Account], error) {
		items, err := h.svc.CreateAccounts(ctx, in.Body.Items)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return newList(items), nil
	})
}

// --- Categories ---

type listCategoriesInput struct {
	Kind            string `query:"kind" enum:"expense,income" doc:"Filter by kind"`
	IncludeArchived bool   `query:"include_archived" doc:"Include archived categories"`
}

type createCategoryInput struct {
	Body finance.CreateCategoryInput
}

type updateCategoryInput struct {
	ID   string `path:"id" format:"uuid"`
	Body finance.UpdateCategoryInput
}

func (h *financeHandlers) registerCategories(api huma.API) {
	tags := []string{"Categories"}

	huma.Register(api, huma.Operation{
		OperationID: "list-categories",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/categories",
		Summary:     "List categories",
		Tags:        tags,
	}, func(ctx context.Context, in *listCategoriesInput) (*listOutput[finance.Category], error) {
		items, err := h.svc.ListCategories(ctx, optionalString(in.Kind), in.IncludeArchived)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return newList(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-category",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/categories",
		Summary:       "Create a category",
		Tags:          tags,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createCategoryInput) (*bodyOutput[finance.Category], error) {
		category, err := h.svc.CreateCategory(ctx, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[finance.Category]{Body: category}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-category",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/categories/{id}",
		Summary:     "Get a category",
		Tags:        tags,
	}, func(ctx context.Context, in *idInput) (*bodyOutput[finance.Category], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		category, err := h.svc.GetCategory(ctx, id)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[finance.Category]{Body: category}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-category",
		Method:      http.MethodPatch,
		Path:        apiPrefix + "/categories/{id}",
		Summary:     "Update a category",
		Description: "Partially updates a category. The kind cannot be changed.",
		Tags:        tags,
	}, func(ctx context.Context, in *updateCategoryInput) (*bodyOutput[finance.Category], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		category, err := h.svc.UpdateCategory(ctx, id, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[finance.Category]{Body: category}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-category",
		Method:        http.MethodDelete,
		Path:          apiPrefix + "/categories/{id}",
		Summary:       "Delete a category",
		Description:   "Deletes a category. Its transactions are kept and become uncategorized.",
		Tags:          tags,
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *idInput) (*struct{}, error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		if err := h.svc.DeleteCategory(ctx, id); err != nil {
			return nil, h.fail(ctx, err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-categories-batch",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/categories/batch",
		Summary:       "Create several categories",
		Description:   "Creates up to 100 categories in a single transaction: if any item is invalid, nothing is created. Errors point at the failing item, e.g. items[3].name.",
		Tags:          tags,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *batchInput[finance.CreateCategoryInput]) (*listOutput[finance.Category], error) {
		items, err := h.svc.CreateCategories(ctx, in.Body.Items)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return newList(items), nil
	})
}

// --- Transactions ---

type listTransactionsInput struct {
	AccountID  string `query:"account_id" format:"uuid" doc:"Transactions where the account is the source or the destination"`
	CategoryID string `query:"category_id" format:"uuid"`
	Type       string `query:"type" enum:"expense,income,transfer"`
	From       string `query:"from" format:"date" doc:"Inclusive start date"`
	To         string `query:"to" format:"date" doc:"Inclusive end date"`
	Search     string `query:"q" maxLength:"100" doc:"Search in the description"`
	Owner      string `query:"owner" doc:"Transactions on accounts of this owner (either side of a transfer): \"me\", \"shared\" or a member's user id"`
	Limit      int    `query:"limit" minimum:"1" maximum:"200" default:"50"`
	Offset     int    `query:"offset" minimum:"0"`
}

type transactionBodyInput struct {
	Body finance.TransactionInput
}

type replaceTransactionInput struct {
	ID   string `path:"id" format:"uuid"`
	Body finance.TransactionInput
}

func (h *financeHandlers) registerTransactions(api huma.API) {
	tags := []string{"Transactions"}

	huma.Register(api, huma.Operation{
		OperationID: "list-transactions",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/transactions",
		Summary:     "List transactions",
		Description: "Returns transactions, newest first, with optional filters.",
		Tags:        tags,
	}, func(ctx context.Context, in *listTransactionsInput) (*bodyOutput[finance.TransactionPage], error) {
		accountID, err := parseOptionalID("query.account_id", in.AccountID)
		if err != nil {
			return nil, err
		}
		categoryID, err := parseOptionalID("query.category_id", in.CategoryID)
		if err != nil {
			return nil, err
		}
		owner, err := finance.ParseOwnerFilter(ctx, "query.owner", in.Owner)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		page, err := h.svc.ListTransactions(ctx, finance.TransactionFilter{
			AccountID:  accountID,
			CategoryID: categoryID,
			Type:       optionalString(in.Type),
			From:       optionalString(in.From),
			To:         optionalString(in.To),
			Search:     optionalString(in.Search),
			Owner:      owner,
			Limit:      in.Limit,
			Offset:     in.Offset,
		})
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[finance.TransactionPage]{Body: page}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-transaction",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/transactions",
		Summary:       "Create a transaction",
		Description:   "Records an expense, an income, or a transfer between two accounts.",
		Tags:          tags,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *transactionBodyInput) (*bodyOutput[finance.Transaction], error) {
		tx, err := h.svc.CreateTransaction(ctx, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[finance.Transaction]{Body: tx}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-transaction",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/transactions/{id}",
		Summary:     "Get a transaction",
		Tags:        tags,
	}, func(ctx context.Context, in *idInput) (*bodyOutput[finance.Transaction], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		tx, err := h.svc.GetTransaction(ctx, id)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[finance.Transaction]{Body: tx}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "replace-transaction",
		Method:      http.MethodPut,
		Path:        apiPrefix + "/transactions/{id}",
		Summary:     "Replace a transaction",
		Description: "Replaces every field of a transaction.",
		Tags:        tags,
	}, func(ctx context.Context, in *replaceTransactionInput) (*bodyOutput[finance.Transaction], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		tx, err := h.svc.UpdateTransaction(ctx, id, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[finance.Transaction]{Body: tx}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-transaction",
		Method:        http.MethodDelete,
		Path:          apiPrefix + "/transactions/{id}",
		Summary:       "Delete a transaction",
		Tags:          tags,
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *idInput) (*struct{}, error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		if err := h.svc.DeleteTransaction(ctx, id); err != nil {
			return nil, h.fail(ctx, err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-transactions-batch",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/transactions/batch",
		Summary:       "Create several transactions",
		Description:   "Creates up to 100 transactions in a single transaction: if any item is invalid, nothing is created. Errors point at the failing item, e.g. items[3].name.",
		Tags:          tags,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *batchInput[finance.TransactionInput]) (*listOutput[finance.Transaction], error) {
		items, err := h.svc.CreateTransactions(ctx, in.Body.Items)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return newList(items), nil
	})
}

// --- Summary ---

type summaryInput struct {
	From  string `query:"from" format:"date" doc:"Inclusive start date; defaults to the first day of the current month"`
	To    string `query:"to" format:"date" doc:"Inclusive end date; defaults to the last day of the current month"`
	Owner string `query:"owner" doc:"Limit to accounts of this owner: \"me\", \"shared\" or a member's user id; omit for the whole workspace"`
}

func (h *financeHandlers) registerSummary(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-summary",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/summary",
		Summary:     "Income and expense summary",
		Description: "Totals per currency for a period, broken down by category, plus current balances. Transfers are excluded from income and expenses.",
		Tags:        []string{"Summary"},
	}, func(ctx context.Context, in *summaryInput) (*bodyOutput[finance.Summary], error) {
		owner, err := finance.ParseOwnerFilter(ctx, "query.owner", in.Owner)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		summary, err := h.svc.Summary(ctx, in.From, in.To, owner)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[finance.Summary]{Body: summary}, nil
	})
}
