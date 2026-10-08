package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

type budgetStatusInput struct {
	Month string `query:"month" pattern:"^\\d{4}-\\d{2}$" example:"2026-10" doc:"Month as YYYY-MM; defaults to the current month"`
	Owner string `query:"owner" doc:"\"me\", \"shared\" or a member's user id. Only narrows spent and committed to those accounts; budgets are workspace-wide"`
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
