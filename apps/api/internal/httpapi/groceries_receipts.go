package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/groceries"
)

type listGroceryReceiptsInput struct {
	Month          string `query:"month" pattern:"^\\d{4}-\\d{2}$" doc:"Only receipts purchased in this month (YYYY-MM)"`
	StoreID        string `query:"store_id" format:"uuid" doc:"Only receipts of this store"`
	UnassignedOnly bool   `query:"unassigned_only" doc:"Only receipts with lines that are not mapped to a product yet"`
}

type createGroceryReceiptInput struct {
	Body groceries.CreateReceiptInput
}

type updateGroceryReceiptInput struct {
	ID   string `path:"id" format:"uuid"`
	Body groceries.UpdateReceiptInput
}

type assignReceiptLinesInput struct {
	ID   string `path:"id" format:"uuid"`
	Body groceries.AssignLinesInput
}

func (h *groceriesHandlers) registerReceipts(api huma.API) {
	tags := []string{"Groceries"}

	huma.Register(api, huma.Operation{
		OperationID: "list-grocery-receipts",
		Method:      http.MethodGet,
		Path:        groceriesPrefix + "/receipts",
		Summary:     "List grocery receipts",
		Description: "Newest first, with totals and the number of lines not mapped to a product yet. Amounts are minor units.",
		Tags:        tags,
	}, func(ctx context.Context, in *listGroceryReceiptsInput) (*listOutput[groceries.ReceiptSummary], error) {
		params := groceries.ListReceiptsParams{Month: in.Month, UnassignedOnly: in.UnassignedOnly}
		if in.StoreID != "" {
			id, err := parseID(in.StoreID)
			if err != nil {
				return nil, err
			}
			params.StoreID = &id
		}
		items, err := h.svc.ListReceipts(ctx, params)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return newList(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "create-grocery-receipt",
		Method:      http.MethodPost,
		Path:        groceriesPrefix + "/receipts",
		Summary:     "Create a grocery receipt",
		Description: "Lines without variant_id are matched through the store's aliases (match: alias | given | none). " +
			"Link an existing expense with transaction_id, or create one with create_expense. " +
			"A receipt that does not add up is accepted and reported in warnings.",
		Tags:          tags,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createGroceryReceiptInput) (*bodyOutput[groceries.Receipt], error) {
		item, err := h.svc.CreateReceipt(ctx, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[groceries.Receipt]{Body: item}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-grocery-receipt",
		Method:      http.MethodGet,
		Path:        groceriesPrefix + "/receipts/{id}",
		Summary:     "Get a grocery receipt",
		Tags:        tags,
	}, func(ctx context.Context, in *idInput) (*bodyOutput[groceries.Receipt], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		item, err := h.svc.GetReceipt(ctx, id)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[groceries.Receipt]{Body: item}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-grocery-receipt",
		Method:      http.MethodPatch,
		Path:        groceriesPrefix + "/receipts/{id}",
		Summary:     "Update a grocery receipt",
		Description: "Changes the store, purchase date, total or the linked expense (empty transaction_id unlinks).",
		Tags:        tags,
	}, func(ctx context.Context, in *updateGroceryReceiptInput) (*bodyOutput[groceries.Receipt], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		item, err := h.svc.UpdateReceipt(ctx, id, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[groceries.Receipt]{Body: item}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-grocery-receipt",
		Method:        http.MethodDelete,
		Path:          groceriesPrefix + "/receipts/{id}",
		Summary:       "Delete a grocery receipt",
		Description:   "The linked transaction is not deleted.",
		Tags:          tags,
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *idInput) (*struct{}, error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		if err := h.svc.DeleteReceipt(ctx, id); err != nil {
			return nil, h.fail(ctx, err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "assign-grocery-receipt-lines",
		Method:      http.MethodPost,
		Path:        groceriesPrefix + "/receipts/{id}/assign",
		Summary:     "Assign receipt lines to products",
		Description: "Each assignment gives an existing variant_id, or a product name with brand and pack that is found or " +
			"created. The mapping is remembered for the receipt's store, so the next receipt with the same text matches " +
			"automatically. An assignment with neither variant_id nor product unassigns the line (the alias is kept). " +
			"All assignments apply or none does.",
		Tags: tags,
	}, func(ctx context.Context, in *assignReceiptLinesInput) (*bodyOutput[groceries.Receipt], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		item, err := h.svc.AssignLines(ctx, id, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[groceries.Receipt]{Body: item}, nil
	})
}
