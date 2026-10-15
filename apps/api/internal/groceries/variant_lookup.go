package groceries

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

// VariantRef names a product variant by its descriptive fields, the way a
// receipt line assignment does.
type VariantRef struct {
	Product     string
	Brand       string
	PackSize    *float64
	PackLabel   string
	CompareUnit string
	Group       string
}

// EnsureVariant returns the id of the variant described by ref, creating the
// product (which needs CompareUnit) and the variant when they do not exist.
func (s *Service) EnsureVariant(ctx context.Context, ref VariantRef) (uuid.UUID, error) {
	if strings.TrimSpace(ref.Product) == "" {
		return uuid.Nil, Invalid("product", "is required")
	}
	a := AssignLineInput{Product: &ref.Product, PackSize: ref.PackSize}
	if ref.Brand != "" {
		a.Brand = &ref.Brand
	}
	if ref.PackLabel != "" {
		a.PackLabel = &ref.PackLabel
	}
	if ref.CompareUnit != "" {
		a.CompareUnit = &ref.CompareUnit
	}
	if ref.Group != "" {
		a.Group = &ref.Group
	}
	var groups []ProductGroup
	if ref.Group != "" {
		var err error
		if groups, err = s.ListGroups(ctx); err != nil {
			return uuid.Nil, err
		}
	}
	var id uuid.UUID
	err := s.withTx(ctx, func(q *store.Queries) error {
		v, err := resolveVariant(ctx, q, a, groups, func(name string) string { return name })
		if err != nil {
			return err
		}
		id = *v
		return nil
	})
	return id, err
}
