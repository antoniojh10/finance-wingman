package groceries

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

const maxDisplayMultiplier = 1000

// CompareUnits are the units unit prices are compared in.
var CompareUnits = []string{"kg", "l", "unit"}

// Product is something bought regardless of brand or pack, e.g. whole milk.
// Whole and semi-skimmed milk are different products.
type Product struct {
	ID                uuid.UUID  `json:"id"`
	Name              string     `json:"name" example:"Whole milk"`
	GroupID           *uuid.UUID `json:"group_id" nullable:"true" format:"uuid"`
	GroupName         *string    `json:"group_name" nullable:"true"`
	GroupColor        *string    `json:"group_color" nullable:"true"`
	CompareUnit       string     `json:"compare_unit" enum:"kg,l,unit" doc:"Unit the prices of its variants are compared in"`
	DisplayMultiplier int32      `json:"display_multiplier" doc:"Unit prices are shown for this many compare units (12: priced per dozen)"`
	VariantCount      int64      `json:"variant_count"`
	Archived          bool       `json:"archived"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type CreateProductInput struct {
	Name              string  `json:"name" minLength:"1" maxLength:"100"`
	GroupID           *string `json:"group_id,omitempty" format:"uuid"`
	CompareUnit       string  `json:"compare_unit" enum:"kg,l,unit"`
	DisplayMultiplier *int32  `json:"display_multiplier,omitempty" minimum:"1" maximum:"1000" doc:"Defaults to 1"`
}

type UpdateProductInput struct {
	Name              *string `json:"name,omitempty" minLength:"1" maxLength:"100"`
	GroupID           *string `json:"group_id,omitempty" doc:"Group id; empty string removes the group"`
	CompareUnit       *string `json:"compare_unit,omitempty" enum:"kg,l,unit"`
	DisplayMultiplier *int32  `json:"display_multiplier,omitempty" minimum:"1" maximum:"1000"`
	Archived          *bool   `json:"archived,omitempty"`
}

type ListProductsParams struct {
	IncludeArchived bool
	GroupID         *uuid.UUID
}

func (p Product) withArchived(at *time.Time) Product {
	p.Archived = at != nil
	return p
}

func productFromRow(r store.GetGroceryProductRow) Product {
	return Product{
		ID: r.ID, Name: r.Name, GroupID: r.GroupID, GroupName: r.GroupName, GroupColor: r.GroupColor,
		CompareUnit: r.CompareUnit, DisplayMultiplier: r.DisplayMultiplier, VariantCount: r.VariantCount,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}.withArchived(r.ArchivedAt)
}

func validateCompareUnit(unit string) error {
	if !slices.Contains(CompareUnits, unit) {
		return Invalid("compare_unit", "must be one of "+strings.Join(CompareUnits, ", "))
	}
	return nil
}

func validateMultiplier(m *int32) error {
	if m != nil && (*m < 1 || *m > maxDisplayMultiplier) {
		return Invalid("display_multiplier", "must be between 1 and 1000")
	}
	return nil
}

// optionalID parses an optional id; the second result tells whether a
// value was given (an empty string means "none").
func optionalID(field string, raw *string) (*uuid.UUID, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	id, err := uuid.Parse(strings.TrimSpace(*raw))
	if err != nil {
		return nil, Invalid(field, "must be a valid UUID")
	}
	return &id, nil
}

// checkGroup makes sure a referenced group exists in the workspace.
func (s *Service) checkGroup(ctx context.Context, id *uuid.UUID) error {
	if id == nil {
		return nil
	}
	if _, err := s.q.GetProductGroup(ctx, *id); err != nil {
		if isNoRows(err) {
			return Invalid("group_id", "unknown product group")
		}
		return err
	}
	return nil
}

func (s *Service) ListProducts(ctx context.Context, p ListProductsParams) ([]Product, error) {
	rows, err := s.q.ListGroceryProducts(ctx, store.ListGroceryProductsParams{IncludeArchived: p.IncludeArchived, GroupID: p.GroupID})
	if err != nil {
		return nil, err
	}
	out := make([]Product, len(rows))
	for i, r := range rows {
		out[i] = productFromRow(store.GetGroceryProductRow(r))
	}
	return out, nil
}

func (s *Service) GetProduct(ctx context.Context, id uuid.UUID) (Product, error) {
	row, err := s.q.GetGroceryProduct(ctx, id)
	if isNoRows(err) {
		return Product{}, NotFound("product")
	}
	if err != nil {
		return Product{}, err
	}
	return productFromRow(row), nil
}

func (s *Service) CreateProduct(ctx context.Context, in CreateProductInput) (Product, error) {
	name, err := normalizeName("name", in.Name)
	if err != nil {
		return Product{}, err
	}
	if err := validateCompareUnit(in.CompareUnit); err != nil {
		return Product{}, err
	}
	if err := validateMultiplier(in.DisplayMultiplier); err != nil {
		return Product{}, err
	}
	groupID, err := optionalID("group_id", in.GroupID)
	if err != nil {
		return Product{}, err
	}
	if err := s.checkGroup(ctx, groupID); err != nil {
		return Product{}, err
	}
	multiplier := int32(1)
	if in.DisplayMultiplier != nil {
		multiplier = *in.DisplayMultiplier
	}
	id, err := s.q.CreateGroceryProduct(ctx, store.CreateGroceryProductParams{
		Name: name, GroupID: groupID, CompareUnit: in.CompareUnit, DisplayMultiplier: multiplier,
	})
	if pgErrorCode(err) == pgUniqueViolation {
		return Product{}, Conflict("an active product with this name already exists")
	}
	if err != nil {
		return Product{}, err
	}
	return s.GetProduct(ctx, id)
}

func (s *Service) UpdateProduct(ctx context.Context, id uuid.UUID, in UpdateProductInput) (Product, error) {
	if in.Name != nil {
		name, err := normalizeName("name", *in.Name)
		if err != nil {
			return Product{}, err
		}
		in.Name = &name
	}
	if in.CompareUnit != nil {
		if err := validateCompareUnit(*in.CompareUnit); err != nil {
			return Product{}, err
		}
	}
	if err := validateMultiplier(in.DisplayMultiplier); err != nil {
		return Product{}, err
	}
	groupID, err := optionalID("group_id", in.GroupID)
	if err != nil {
		return Product{}, err
	}
	if err := s.checkGroup(ctx, groupID); err != nil {
		return Product{}, err
	}
	_, err = s.q.UpdateGroceryProduct(ctx, store.UpdateGroceryProductParams{
		ID: id, Name: in.Name, SetGroup: in.GroupID != nil, GroupID: groupID,
		CompareUnit: in.CompareUnit, DisplayMultiplier: in.DisplayMultiplier, Archived: in.Archived,
	})
	if isNoRows(err) {
		return Product{}, NotFound("product")
	}
	if pgErrorCode(err) == pgUniqueViolation {
		return Product{}, Conflict("an active product with this name already exists")
	}
	if err != nil {
		return Product{}, err
	}
	return s.GetProduct(ctx, id)
}
