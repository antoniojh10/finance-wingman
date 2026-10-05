package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

type listRecurringInput struct {
	Status string `query:"status" enum:"active,paused,cancelled" doc:"Only items with this status"`
	Type   string `query:"type" enum:"expense,income" doc:"Only items of this type"`
}

type createRecurringInput struct {
	Body finance.CreateRecurringItemInput
}

type updateRecurringInput struct {
	ID   string `path:"id" format:"uuid"`
	Body finance.UpdateRecurringItemInput
}

func (h *financeHandlers) registerRecurring(api huma.API) {
	tags := []string{"Recurring"}

	huma.Register(api, huma.Operation{
		OperationID: "list-recurring-items",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/recurring",
		Summary:     "List recurring items",
		Description: "Subscriptions, bills, installments and recurring income, with their next due date.",
		Tags:        tags,
	}, func(ctx context.Context, in *listRecurringInput) (*listOutput[finance.RecurringItem], error) {
		items, err := h.svc.ListRecurringItems(ctx, finance.RecurringFilter{
			Status: optionalString(in.Status),
			Type:   optionalString(in.Type),
		})
		if err != nil {
			return nil, h.fail(err)
		}
		return newList(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-recurring-summary",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/recurring/summary",
		Summary:     "Committed monthly cost",
		Description: "Active recurring items normalized to an average month and grouped per currency (no conversion), split into expenses and income.",
		Tags:        tags,
	}, func(ctx context.Context, _ *struct{}) (*bodyOutput[finance.RecurringSummary], error) {
		summary, err := h.svc.RecurringSummary(ctx)
		if err != nil {
			return nil, h.fail(err)
		}
		return &bodyOutput[finance.RecurringSummary]{Body: summary}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-recurring-item",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/recurring",
		Summary:       "Create a recurring item",
		Tags:          tags,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createRecurringInput) (*bodyOutput[finance.RecurringItem], error) {
		item, err := h.svc.CreateRecurringItem(ctx, in.Body)
		if err != nil {
			return nil, h.fail(err)
		}
		return &bodyOutput[finance.RecurringItem]{Body: item}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-recurring-item",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/recurring/{id}",
		Summary:     "Get a recurring item",
		Tags:        tags,
	}, func(ctx context.Context, in *idInput) (*bodyOutput[finance.RecurringItem], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		item, err := h.svc.GetRecurringItem(ctx, id)
		if err != nil {
			return nil, h.fail(err)
		}
		return &bodyOutput[finance.RecurringItem]{Body: item}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-recurring-item",
		Method:      http.MethodPatch,
		Path:        apiPrefix + "/recurring/{id}",
		Summary:     "Update a recurring item",
		Description: "Partially updates an item. The type and account cannot be changed. There is no delete: set status to cancelled to retire an item.",
		Tags:        tags,
	}, func(ctx context.Context, in *updateRecurringInput) (*bodyOutput[finance.RecurringItem], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		item, err := h.svc.UpdateRecurringItem(ctx, id, in.Body)
		if err != nil {
			return nil, h.fail(err)
		}
		return &bodyOutput[finance.RecurringItem]{Body: item}, nil
	})
}
