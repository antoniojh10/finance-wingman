package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/groceries"
)

const groceriesPrefix = apiPrefix + "/groceries"

type groceriesHandlers struct {
	svc    *groceries.Service
	logger *slog.Logger
}

func registerGroceries(api huma.API, svc *groceries.Service, logger *slog.Logger) {
	h := &groceriesHandlers{svc: svc, logger: logger}
	h.registerStores(api)
	h.registerGroups(api)
	h.registerProducts(api)
	h.registerVariants(api)
}

// fail converts groceries domain errors into Huma errors; anything else is
// handled like in the finance endpoints.
func (h *groceriesHandlers) fail(ctx context.Context, err error) error {
	var domainErr *groceries.Error
	if errors.As(err, &domainErr) {
		switch domainErr.Kind {
		case groceries.KindInvalid:
			return huma.Error422UnprocessableEntity(domainErr.Error(), &huma.ErrorDetail{
				Location: domainErr.Field,
				Message:  domainErr.Message,
			})
		case groceries.KindNotFound:
			return huma.Error404NotFound(domainErr.Message)
		case groceries.KindConflict:
			return huma.Error409Conflict(domainErr.Message)
		}
	}
	return toHTTPError(ctx, h.logger, err)
}

// --- Stores ---

type listGroceryStoresInput struct {
	IncludeArchived bool `query:"include_archived" doc:"Include archived stores"`
}

type createGroceryStoreInput struct {
	Body groceries.CreateStoreInput
}

type updateGroceryStoreInput struct {
	ID   string `path:"id" format:"uuid"`
	Body groceries.UpdateStoreInput
}

func (h *groceriesHandlers) registerStores(api huma.API) {
	tags := []string{"Groceries"}

	huma.Register(api, huma.Operation{
		OperationID: "list-grocery-stores",
		Method:      http.MethodGet,
		Path:        groceriesPrefix + "/stores",
		Summary:     "List grocery stores",
		Description: "Stores are supermarket chains, without branches.",
		Tags:        tags,
	}, func(ctx context.Context, in *listGroceryStoresInput) (*listOutput[groceries.Store], error) {
		items, err := h.svc.ListStores(ctx, in.IncludeArchived)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return newList(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-grocery-store",
		Method:        http.MethodPost,
		Path:          groceriesPrefix + "/stores",
		Summary:       "Create a grocery store",
		Tags:          tags,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createGroceryStoreInput) (*bodyOutput[groceries.Store], error) {
		item, err := h.svc.CreateStore(ctx, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[groceries.Store]{Body: item}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-grocery-store",
		Method:      http.MethodPatch,
		Path:        groceriesPrefix + "/stores/{id}",
		Summary:     "Update a grocery store",
		Description: "Renames a store or archives/unarchives it.",
		Tags:        tags,
	}, func(ctx context.Context, in *updateGroceryStoreInput) (*bodyOutput[groceries.Store], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		item, err := h.svc.UpdateStore(ctx, id, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[groceries.Store]{Body: item}, nil
	})
}

// --- Product groups ---

type createProductGroupInput struct {
	Body groceries.CreateProductGroupInput
}

type updateProductGroupInput struct {
	ID   string `path:"id" format:"uuid"`
	Body groceries.UpdateProductGroupInput
}

func (h *groceriesHandlers) registerGroups(api huma.API) {
	tags := []string{"Groceries"}

	huma.Register(api, huma.Operation{
		OperationID: "list-product-groups",
		Method:      http.MethodGet,
		Path:        groceriesPrefix + "/groups",
		Summary:     "List product groups",
		Description: "Default groups (Produce, Meat & fish, ...) are created the first time a workspace lists them.",
		Tags:        tags,
	}, func(ctx context.Context, _ *struct{}) (*listOutput[groceries.ProductGroup], error) {
		items, err := h.svc.ListGroups(ctx)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return newList(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-product-group",
		Method:        http.MethodPost,
		Path:          groceriesPrefix + "/groups",
		Summary:       "Create a product group",
		Tags:          tags,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createProductGroupInput) (*bodyOutput[groceries.ProductGroup], error) {
		item, err := h.svc.CreateGroup(ctx, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[groceries.ProductGroup]{Body: item}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-product-group",
		Method:      http.MethodPatch,
		Path:        groceriesPrefix + "/groups/{id}",
		Summary:     "Update a product group",
		Tags:        tags,
	}, func(ctx context.Context, in *updateProductGroupInput) (*bodyOutput[groceries.ProductGroup], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		item, err := h.svc.UpdateGroup(ctx, id, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[groceries.ProductGroup]{Body: item}, nil
	})
}

// --- Products ---

type listGroceryProductsInput struct {
	IncludeArchived bool   `query:"include_archived" doc:"Include archived products"`
	GroupID         string `query:"group_id" format:"uuid" doc:"Only products of this group"`
}

type createGroceryProductInput struct {
	Body groceries.CreateProductInput
}

type updateGroceryProductInput struct {
	ID   string `path:"id" format:"uuid"`
	Body groceries.UpdateProductInput
}

func (h *groceriesHandlers) registerProducts(api huma.API) {
	tags := []string{"Groceries"}

	huma.Register(api, huma.Operation{
		OperationID: "list-grocery-products",
		Method:      http.MethodGet,
		Path:        groceriesPrefix + "/products",
		Summary:     "List grocery products",
		Description: "Products with their group and number of variants.",
		Tags:        tags,
	}, func(ctx context.Context, in *listGroceryProductsInput) (*listOutput[groceries.Product], error) {
		groupID, err := parseOptionalID("query.group_id", in.GroupID)
		if err != nil {
			return nil, err
		}
		items, err := h.svc.ListProducts(ctx, groceries.ListProductsParams{IncludeArchived: in.IncludeArchived, GroupID: groupID})
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return newList(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-grocery-product",
		Method:        http.MethodPost,
		Path:          groceriesPrefix + "/products",
		Summary:       "Create a grocery product",
		Description:   "A product is what is bought regardless of brand and pack. Whole and semi-skimmed milk are different products.",
		Tags:          tags,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createGroceryProductInput) (*bodyOutput[groceries.Product], error) {
		item, err := h.svc.CreateProduct(ctx, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[groceries.Product]{Body: item}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-grocery-product",
		Method:      http.MethodPatch,
		Path:        groceriesPrefix + "/products/{id}",
		Summary:     "Update a grocery product",
		Description: "Renames it, changes its group, compare unit or display multiplier, or archives/unarchives it.",
		Tags:        tags,
	}, func(ctx context.Context, in *updateGroceryProductInput) (*bodyOutput[groceries.Product], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		item, err := h.svc.UpdateProduct(ctx, id, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[groceries.Product]{Body: item}, nil
	})
}

// --- Variants ---

type createVariantInput struct {
	ID   string `path:"id" format:"uuid" doc:"Product id"`
	Body groceries.CreateVariantInput
}

type updateVariantInput struct {
	ID   string `path:"id" format:"uuid"`
	Body groceries.UpdateVariantInput
}

func (h *groceriesHandlers) registerVariants(api huma.API) {
	tags := []string{"Groceries"}

	huma.Register(api, huma.Operation{
		OperationID: "list-product-variants",
		Method:      http.MethodGet,
		Path:        groceriesPrefix + "/products/{id}/variants",
		Summary:     "List the variants of a product",
		Tags:        tags,
	}, func(ctx context.Context, in *idInput) (*listOutput[groceries.Variant], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		items, err := h.svc.ListVariants(ctx, id)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return newList(items), nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "create-product-variant",
		Method:        http.MethodPost,
		Path:          groceriesPrefix + "/products/{id}/variants",
		Summary:       "Create a variant of a product",
		Description:   "A variant is a brand and pack size (use brand \"Generic\" for unbranded). Omit pack_size for products sold loose.",
		Tags:          tags,
		DefaultStatus: http.StatusCreated,
	}, func(ctx context.Context, in *createVariantInput) (*bodyOutput[groceries.Variant], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		item, err := h.svc.CreateVariant(ctx, id, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[groceries.Variant]{Body: item}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "update-product-variant",
		Method:      http.MethodPatch,
		Path:        groceriesPrefix + "/variants/{id}",
		Summary:     "Update a product variant",
		Tags:        tags,
	}, func(ctx context.Context, in *updateVariantInput) (*bodyOutput[groceries.Variant], error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		item, err := h.svc.UpdateVariant(ctx, id, in.Body)
		if err != nil {
			return nil, h.fail(ctx, err)
		}
		return &bodyOutput[groceries.Variant]{Body: item}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID:   "delete-product-variant",
		Method:        http.MethodDelete,
		Path:          groceriesPrefix + "/variants/{id}",
		Summary:       "Delete a product variant",
		Description:   "Receipt lines that used it keep their raw text and lose the variant.",
		Tags:          tags,
		DefaultStatus: http.StatusNoContent,
	}, func(ctx context.Context, in *idInput) (*struct{}, error) {
		id, err := parseID(in.ID)
		if err != nil {
			return nil, err
		}
		if err := h.svc.DeleteVariant(ctx, id); err != nil {
			return nil, h.fail(ctx, err)
		}
		return nil, nil
	})
}
