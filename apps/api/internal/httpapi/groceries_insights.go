package httpapi

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/groceries"
)

type listPriceChecksInput struct {
	VariantID string `query:"variant_id" format:"uuid" doc:"Only checks of this variant"`
	ProductID string `query:"product_id" format:"uuid" doc:"Only checks of this product"`
	StoreID   string `query:"store_id" format:"uuid" doc:"Only checks at this store"`
}

type createPriceCheckInput struct {
	Body groceries.CreatePriceCheckInput
}

type breakdownInput struct {
	Month    string `query:"month" pattern:"^\\d{4}-(0[1-9]|1[0-2])$" example:"2026-09" doc:"Calendar month YYYY-MM; defaults to the current month"`
	Currency string `query:"currency" minLength:"3" maxLength:"3" doc:"Only this ISO 4217 currency"`
}

type compareInput struct {
	Level    string `query:"level" enum:"product,brand" default:"product" doc:"product: cheapest variant per store; brand: manufacturer brands sold in at least two stores"`
	Currency string `query:"currency" minLength:"3" maxLength:"3" doc:"Only this ISO 4217 currency"`
}

type basketInput struct {
	Months   int    `query:"months" default:"3" minimum:"1" maximum:"12" doc:"Full calendar months, the current one excluded, averaged into the basket"`
	Currency string `query:"currency" minLength:"3" maxLength:"3" doc:"Only this ISO 4217 currency"`
}

func (h *groceriesHandlers) registerInsights(api huma.API) {
	tags := []string{"Groceries"}

	huma.Register(api, huma.Operation{
		OperationID: "list-price-checks",
		Method:      http.MethodGet,
		Path:        groceriesPrefix + "/price-checks",
		Summary:     "List price checks",
		Description: "Prices seen in a store without buying, newest first (at most 200).",
		Tags:        tags,
	}, func(ctx context.Context, in *listPriceChecksInput) (*listOutput[groceries.PriceCheck], error) {
		variantID, err := parseOptionalID("query.variant_id", in.VariantID)
		if err != nil {
			return nil, err
		}
		productID, err := parseOptionalID("query.product_id", in.ProductID)
		if err != nil {
			return nil, err
		}
		storeID, err := parseOptionalID("query.store_id", in.StoreID)
		if err != nil {
			return nil, err
		}
		items, err := h.svc.ListPriceChecks(ctx, groceries.ListPriceChecksParams{VariantID: variantID, ProductID: productID, StoreID: storeID})
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return newList(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-price-check",
		Method:        http.MethodPost,
		Path:          groceriesPrefix + "/price-checks",
		Summary:       "Record a price check",
		Description:   "Notes the pack price of a variant at a store. The unit price is the pack price divided by the pack size (loose variants: the price per compare unit).",
		Tags:          tags,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createPriceCheckInput) (*bodyOutput[groceries.PriceCheck], error) {
		item, err := h.svc.CreatePriceCheck(ctx, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[groceries.PriceCheck]{Body: item}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-price-check",
		Method:        http.MethodDelete,
		Path:          groceriesPrefix + "/price-checks/{id}",
		Summary:       "Delete a price check",
		Tags:          tags,
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *idInput) (*struct{}, error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		if err := h.svc.DeletePriceCheck(ctx, id); err != nil {
			return nil, h.fail(ctx, err)
		}
		return nil, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-groceries-breakdown",
		Method:      http.MethodGet,
		Path:        groceriesPrefix + "/breakdown",
		Summary:     "Grocery spending of a month",
		Description: "Receipts of the month per currency: total, average receipt, spend by product group and store, top 10 products, how much of the lines is mapped to products, and the Groceries category expenses to compare with.",
		Tags:        tags,
	}, func(ctx context.Context, in *breakdownInput) (*bodyOutput[groceries.Breakdown], error) {
		out, err := h.svc.Breakdown(ctx, in.Month, in.Currency)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[groceries.Breakdown]{Body: out}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "compare-grocery-stores",
		Method:      http.MethodGet,
		Path:        groceriesPrefix + "/compare",
		Summary:     "Compare supermarket prices",
		Description: "Matrix of products (or brands) by store with the cheapest latest unit price of the last 180 days, per currency. Observations older than 60 days are flagged stale.",
		Tags:        tags,
	}, func(ctx context.Context, in *compareInput) (*bodyOutput[groceries.Comparison], error) {
		out, err := h.svc.Compare(ctx, in.Level, in.Currency)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[groceries.Comparison]{Body: out}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-groceries-basket",
		Method:      http.MethodGet,
		Path:        groceriesPrefix + "/basket",
		Summary:     "Monthly basket cost per store",
		Description: "The basket is the average monthly quantity of each product over the last full months. Each store is valued with its cheapest variant of every product; stores missing prices are flagged. Includes the best single store and the split basket (each product at its cheapest store).",
		Tags:        tags,
	}, func(ctx context.Context, in *basketInput) (*bodyOutput[groceries.Basket], error) {
		out, err := h.svc.Basket(ctx, in.Months, in.Currency)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[groceries.Basket]{Body: out}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-product-prices",
		Method:      http.MethodGet,
		Path:        groceriesPrefix + "/products/{id}/prices",
		Summary:     "Prices of a product",
		Description: "Latest unit price of each variant at each store and the monthly average unit price of the last 12 months.",
		Tags:        tags,
	}, func(ctx context.Context, in *idInput) (*bodyOutput[groceries.ProductPrices], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		out, err := h.svc.ProductPrices(ctx, id)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[groceries.ProductPrices]{Body: out}, nil
	})
}
