package groceries

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

const maxPackSize = 999999999.0

// Variant is a brand and pack of a product, e.g. Hacendado 6x1 l.
type Variant struct {
	ID        uuid.UUID  `json:"id"`
	ProductID uuid.UUID  `json:"product_id"`
	Brand     string     `json:"brand" example:"Hacendado" doc:"\"Generic\" for unbranded products"`
	PackSize  *float64   `json:"pack_size" nullable:"true" doc:"Contents of the pack in the product's compare unit. Null when sold loose: the quantity of a receipt line is the amount"`
	PackLabel *string    `json:"pack_label" nullable:"true" example:"4x125 g"`
	StoreID   *uuid.UUID `json:"store_id" nullable:"true" format:"uuid" doc:"Store whose own brand this is"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type CreateVariantInput struct {
	Brand     string   `json:"brand" minLength:"1" maxLength:"100"`
	PackSize  *float64 `json:"pack_size,omitempty" exclusiveMinimum:"0" maximum:"999999999" doc:"Up to 3 decimals; omit when sold loose"`
	PackLabel *string  `json:"pack_label,omitempty" maxLength:"50"`
	StoreID   *string  `json:"store_id,omitempty" format:"uuid"`
}

type UpdateVariantInput struct {
	Brand         *string  `json:"brand,omitempty" minLength:"1" maxLength:"100"`
	PackSize      *float64 `json:"pack_size,omitempty" exclusiveMinimum:"0" maximum:"999999999"`
	ClearPackSize bool     `json:"clear_pack_size,omitempty" doc:"Make the variant loose (no pack size); use instead of pack_size"`
	PackLabel     *string  `json:"pack_label,omitempty" maxLength:"50" doc:"Empty string clears it"`
	StoreID       *string  `json:"store_id,omitempty" doc:"Store id; empty string clears it"`
}

func variantFromModel(v store.ProductVariant) Variant {
	return Variant{
		ID: v.ID, ProductID: v.ProductID, Brand: v.Brand, PackSize: floatFromNumeric(v.PackSize),
		PackLabel: v.PackLabel, StoreID: v.StoreID, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
}

func validatePackSize(v *float64) (pgtype.Numeric, error) {
	if v == nil {
		return pgtype.Numeric{}, nil
	}
	if *v <= 0 || *v > maxPackSize {
		return pgtype.Numeric{}, Invalid("pack_size", "must be greater than 0")
	}
	return numericFromFloat(*v)
}

func (s *Service) checkStore(ctx context.Context, id *uuid.UUID) error {
	if id == nil {
		return nil
	}
	if _, err := s.q.GetGroceryStore(ctx, *id); err != nil {
		if isNoRows(err) {
			return Invalid("store_id", "unknown store")
		}
		return err
	}
	return nil
}

func (s *Service) ListVariants(ctx context.Context, productID uuid.UUID) ([]Variant, error) {
	if _, err := s.GetProduct(ctx, productID); err != nil {
		return nil, err
	}
	rows, err := s.q.ListProductVariants(ctx, productID)
	if err != nil {
		return nil, err
	}
	out := make([]Variant, len(rows))
	for i, r := range rows {
		out[i] = variantFromModel(r)
	}
	return out, nil
}

func (s *Service) CreateVariant(ctx context.Context, productID uuid.UUID, in CreateVariantInput) (Variant, error) {
	brand, err := normalizeName("brand", in.Brand)
	if err != nil {
		return Variant{}, err
	}
	size, err := validatePackSize(in.PackSize)
	if err != nil {
		return Variant{}, err
	}
	storeID, err := optionalID("store_id", in.StoreID)
	if err != nil {
		return Variant{}, err
	}
	if _, err := s.GetProduct(ctx, productID); err != nil {
		return Variant{}, err
	}
	if err := s.checkStore(ctx, storeID); err != nil {
		return Variant{}, err
	}
	row, err := s.q.CreateProductVariant(ctx, store.CreateProductVariantParams{
		ProductID: productID, Brand: brand, PackSize: size, PackLabel: emptyToNil(in.PackLabel), StoreID: storeID,
	})
	if pgErrorCode(err) == pgUniqueViolation {
		return Variant{}, Conflict("this product already has a variant with this brand and pack size")
	}
	if err != nil {
		return Variant{}, err
	}
	return variantFromModel(row), nil
}

func (s *Service) UpdateVariant(ctx context.Context, id uuid.UUID, in UpdateVariantInput) (Variant, error) {
	if in.Brand != nil {
		brand, err := normalizeName("brand", *in.Brand)
		if err != nil {
			return Variant{}, err
		}
		in.Brand = &brand
	}
	if in.ClearPackSize && in.PackSize != nil {
		return Variant{}, Invalid("clear_pack_size", "cannot be combined with pack_size")
	}
	size, err := validatePackSize(in.PackSize)
	if err != nil {
		return Variant{}, err
	}
	storeID, err := optionalID("store_id", in.StoreID)
	if err != nil {
		return Variant{}, err
	}
	if err := s.checkStore(ctx, storeID); err != nil {
		return Variant{}, err
	}
	row, err := s.q.UpdateProductVariant(ctx, store.UpdateProductVariantParams{
		ID: id, Brand: in.Brand,
		SetPackSize: in.PackSize != nil || in.ClearPackSize, PackSize: size,
		SetPackLabel: in.PackLabel != nil, PackLabel: emptyToNil(in.PackLabel),
		SetStore: in.StoreID != nil, StoreID: storeID,
	})
	if isNoRows(err) {
		return Variant{}, NotFound("variant")
	}
	if pgErrorCode(err) == pgUniqueViolation {
		return Variant{}, Conflict("this product already has a variant with this brand and pack size")
	}
	if err != nil {
		return Variant{}, err
	}
	return variantFromModel(row), nil
}

// DeleteVariant removes a variant. Receipt lines keep their raw text and
// lose the variant; aliases pointing at it are removed.
func (s *Service) DeleteVariant(ctx context.Context, id uuid.UUID) error {
	n, err := s.q.DeleteProductVariant(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return NotFound("variant")
	}
	return nil
}
