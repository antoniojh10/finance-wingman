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

type upcomingRecurringInput struct {
	Days int `query:"days" minimum:"1" maximum:"366" default:"30" doc:"Look this many days ahead (today included)"`
}

type registerPaymentInput struct {
	ID   string `path:"id" format:"uuid"`
	Body finance.RecurringPaymentInput
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
		OperationID: "list-upcoming-recurring",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/recurring/upcoming",
		Summary:     "Upcoming recurring payments",
		Description: "Due dates of active items in the next days (today included) with their status, plus the current period of items that are overdue. Ordered by due date.",
		Tags:        tags,
	}, func(ctx context.Context, in *upcomingRecurringInput) (*listOutput[finance.UpcomingRecurring], error) {
		days := in.Days
		if days == 0 {
			days = 30
		}
		items, err := h.svc.UpcomingRecurring(ctx, days)
		if err != nil {
			return nil, h.fail(err)
		}
		return newList(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "register-recurring-payment",
		Method:        http.MethodPost,
		Path:          apiPrefix + "/recurring/{id}/payments",
		Summary:       "Register a payment",
		Description:   "Creates the transaction for one period of an active item (type, account, category and description come from the item) and links it. The item's estimate is not changed by the amount paid.",
		Tags:          tags,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *registerPaymentInput) (*bodyOutput[finance.Transaction], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		tx, err := h.svc.RegisterRecurringPayment(ctx, id, in.Body)
		if err != nil {
			return nil, h.fail(err)
		}
		return &bodyOutput[finance.Transaction]{Body: tx}, nil
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

type linkRecurringInput struct {
	ID   string `path:"id" format:"uuid"`
	Body finance.LinkRecurringInput
}

func (h *financeHandlers) registerTransactionLinks(api huma.API) {
	tags := []string{"Recurring"}

	huma.Register(api, huma.Operation{
		OperationID: "link-transaction-recurring",
		Method:      http.MethodPut,
		Path:        apiPrefix + "/transactions/{id}/recurring",
		Summary:     "Link a transaction to a recurring item",
		Description: "The transaction must have the item's type and account. An already linked transaction is re-linked. Several transactions may settle the same period.",
		Tags:        tags,
	}, func(ctx context.Context, in *linkRecurringInput) (*bodyOutput[finance.Transaction], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		tx, err := h.svc.LinkTransactionToRecurring(ctx, id, in.Body)
		if err != nil {
			return nil, h.fail(err)
		}
		return &bodyOutput[finance.Transaction]{Body: tx}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "unlink-transaction-recurring",
		Method:      http.MethodDelete,
		Path:        apiPrefix + "/transactions/{id}/recurring",
		Summary:     "Unlink a transaction from its recurring item",
		Description: "Does nothing when the transaction is not linked.",
		Tags:        tags,
	}, func(ctx context.Context, in *idInput) (*bodyOutput[finance.Transaction], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		tx, err := h.svc.UnlinkTransactionFromRecurring(ctx, id)
		if err != nil {
			return nil, h.fail(err)
		}
		return &bodyOutput[finance.Transaction]{Body: tx}, nil
	})
}
