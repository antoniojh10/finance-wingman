package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

type budgetStatusInput struct {
	Month string `query:"month" pattern:"^\\d{4}-\\d{2}$" example:"2026-10" doc:"Month as YYYY-MM; defaults to the current month"`
	Owner string `query:"owner" doc:"\"me\", \"shared\" or a member's user id. Only narrows spent and committed to those accounts; budgets are workspace-wide"`
}

type budgetSuggestionsInput struct {
	Month string `query:"month" pattern:"^\\d{4}-\\d{2}$" example:"2026-10" doc:"Month to suggest budgets for, as YYYY-MM; defaults to the current month"`
}

type budgetImpactInput struct {
	CategoryID string `query:"category_id" format:"uuid" required:"true" doc:"Expense category of the expense"`
	Currency   string `query:"currency" required:"true" minLength:"3" maxLength:"3" example:"MXN" doc:"ISO 4217 code of the expense's account"`
	Date       string `query:"date" required:"true" pattern:"^\\d{4}-\\d{2}-\\d{2}$" example:"2026-10-08" doc:"Date of the expense as YYYY-MM-DD; its month decides the budget"`
	Amount     int64  `query:"amount" required:"true" minimum:"0" doc:"Amount of the expense in minor units"`
}

type setBudgetsInput struct {
	Month string `path:"month" pattern:"^\\d{4}-\\d{2}$" example:"2026-10" doc:"Month as YYYY-MM"`
	Body  struct {
		Items []finance.BudgetItemInput `json:"items" minItems:"1" maxItems:"100"`
	}
}

func (h *financeHandlers) registerBudgets(api huma.API) {
	tags := []string{"Budgets"}

	huma.Register(api, huma.Operation{
		OperationID: "get-budget-status",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/budgets",
		Summary:     "Budget status of a month",
		Description: "Per currency and expense category: the budget in force (set in the month or inherited from the latest earlier one), spent, committed (unpaid recurring expenses due in the month, not counted as spent), remaining and state. Categories with spending but no budget and an uncategorized bucket are included. No currency conversion.",
		Tags:        tags,
	}, func(ctx context.Context, in *budgetStatusInput) (*bodyOutput[finance.BudgetStatus], error) {
		owner, err := finance.ParseOwnerFilter(ctx, "query.owner", in.Owner)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		status, err := h.svc.BudgetStatus(ctx, in.Month, owner)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[finance.BudgetStatus]{Body: status}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "suggest-budgets",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/budgets/suggestions",
		Summary:     "Suggest budget amounts from past spending",
		Description: "Per currency and expense category with spending in the 3 complete months before the month: the median monthly spending (months before the category's first expense are skipped, later months without spending count as zero) plus the monthly amount of active recurring expenses with no payment in those months, rounded up to a whole currency unit. Includes the months used and the budget currently in force. Transfers, income and uncategorized expenses are ignored. No currency conversion.",
		Tags:        tags,
	}, func(ctx context.Context, in *budgetSuggestionsInput) (*bodyOutput[finance.BudgetSuggestions], error) {
		suggestions, err := h.svc.SuggestBudgets(ctx, in.Month)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[finance.BudgetSuggestions]{Body: suggestions}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-budget-impact",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/budgets/impact",
		Summary:     "Budget impact of an expense not recorded yet",
		Description: "Tells whether adding an expense to a category would leave its budget near or over the limit in the month of the date, counting spent and committed recurring expenses of the whole workspace. Never pass an expense that is already recorded: it would be counted twice. Without a budget in force for the category, currency and month, has_budget is false and there is no warning.",
		Tags:        tags,
	}, func(ctx context.Context, in *budgetImpactInput) (*bodyOutput[finance.BudgetImpactResult], error) {
		categoryID, err := parseOptionalID("query.category_id", in.CategoryID)
		if err != nil || categoryID == nil {
			return nil, h.fail(ctx, finance.Invalid("query.category_id", "is required"))
		}
		date, err := time.Parse("2006-01-02", in.Date)
		if err != nil {
			return nil, h.fail(ctx, finance.Invalid("query.date", "must be a date as YYYY-MM-DD"))
		}
		impact, err := h.svc.BudgetImpact(ctx, *categoryID, in.Currency, date, in.Amount)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[finance.BudgetImpactResult]{Body: impact}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "set-budgets",
		Method:      http.MethodPut,
		Path:        apiPrefix + "/budgets/{month}",
		Summary:     "Set or clear budgets from a month",
		Description: "Atomically sets (amount) or clears (clear: true) the budget of expense categories per currency, effective from the month. Later months inherit it until they set their own. Returns the status of the month.",
		Tags:        tags,
	}, func(ctx context.Context, in *setBudgetsInput) (*bodyOutput[finance.BudgetStatus], error) {
		status, err := h.svc.SetBudgets(ctx, in.Month, in.Body.Items)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[finance.BudgetStatus]{Body: status}, nil
	})
}
