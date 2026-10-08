package finance

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type Category struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name" example:"Groceries"`
	Kind      string    `json:"kind" enum:"expense,income"`
	Color     *string   `json:"color" example:"#22c55e"`
	Icon      *string   `json:"icon" example:"shopping-cart"`
	Archived  bool      `json:"archived"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateCategoryInput struct {
	Name  string  `json:"name" minLength:"1" maxLength:"100"`
	Kind  string  `json:"kind" enum:"expense,income"`
	Color *string `json:"color,omitempty" pattern:"^#[0-9a-fA-F]{6}$"`
	Icon  *string `json:"icon,omitempty" maxLength:"50"`
}

type UpdateCategoryInput struct {
	Name *string `json:"name,omitempty" minLength:"1" maxLength:"100"`
	// An empty string clears the color or icon.
	Color    *string `json:"color,omitempty" doc:"Hex color (#rrggbb); empty string clears it"`
	Icon     *string `json:"icon,omitempty" maxLength:"50" doc:"Icon name; empty string clears it"`
	Archived *bool   `json:"archived,omitempty"`
}

func categoryFromModel(c store.Category) Category {
	return Category{
		ID:        c.ID,
		Name:      c.Name,
		Kind:      c.Kind,
		Color:     c.Color,
		Icon:      c.Icon,
		Archived:  c.ArchivedAt != nil,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func validateKind(field, kind string) error {
	if kind != "expense" && kind != "income" {
		return Invalid(field, "must be expense or income")
	}
	return nil
}

func validateColor(color *string) error {
	if color != nil && *color != "" && !colorPattern.MatchString(*color) {
		return Invalid("color", "must be a hex color like #22c55e")
	}
	return nil
}

func emptyToNil(s *string) *string {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func (s *Service) ListCategories(ctx context.Context, kind *string, includeArchived bool) ([]Category, error) {
	if kind != nil {
		if err := validateKind("kind", *kind); err != nil {
			return nil, err
		}
	}
	rows, err := s.q.ListCategories(ctx, store.ListCategoriesParams{Kind: kind, IncludeArchived: includeArchived})
	if err != nil {
		return nil, err
	}
	out := make([]Category, len(rows))
	for i, r := range rows {
		out[i] = categoryFromModel(r)
	}
	return out, nil
}

func (s *Service) GetCategory(ctx context.Context, id uuid.UUID) (Category, error) {
	c, err := s.q.GetCategory(ctx, id)
	if isNoRows(err) {
		return Category{}, NotFound("category")
	}
	if err != nil {
		return Category{}, err
	}
	return categoryFromModel(c), nil
}

func (s *Service) CreateCategory(ctx context.Context, in CreateCategoryInput) (Category, error) {
	name, err := normalizeName("name", in.Name)
	if err != nil {
		return Category{}, err
	}
	if err := validateKind("kind", in.Kind); err != nil {
		return Category{}, err
	}
	if err := validateColor(in.Color); err != nil {
		return Category{}, err
	}
	var c store.Category
	err = s.withTx(ctx, func(tx *Service) error {
		var err error
		c, err = tx.q.CreateCategory(ctx, store.CreateCategoryParams{
			Name:  name,
			Kind:  in.Kind,
			Color: emptyToNil(in.Color),
			Icon:  emptyToNil(in.Icon),
		})
		if pgErrorCode(err) == pgUniqueViolation {
			return Conflict("an active " + in.Kind + " category with this name already exists")
		}
		if err != nil {
			return err
		}
		return tx.record(ctx, ActionCategoryCreated, EntityCategory, c.ID, ActivityDetails{Type: c.Kind})
	})
	if err != nil {
		return Category{}, err
	}
	return categoryFromModel(c), nil
}

func (s *Service) UpdateCategory(ctx context.Context, id uuid.UUID, in UpdateCategoryInput) (Category, error) {
	if in.Name != nil {
		name, err := normalizeName("name", *in.Name)
		if err != nil {
			return Category{}, err
		}
		in.Name = &name
	}
	if err := validateColor(in.Color); err != nil {
		return Category{}, err
	}
	cur, err := s.GetCategory(ctx, id)
	if err != nil {
		return Category{}, err
	}
	var changed []string
	add := func(differs bool, field string) {
		if differs {
			changed = append(changed, field)
		}
	}
	add(in.Name != nil && *in.Name != cur.Name, "name")
	add(in.Color != nil && !equalPtr(emptyToNil(in.Color), cur.Color), "color")
	add(in.Icon != nil && !equalPtr(emptyToNil(in.Icon), cur.Icon), "icon")

	var c store.Category
	err = s.withTx(ctx, func(tx *Service) error {
		var err error
		c, err = tx.q.UpdateCategory(ctx, store.UpdateCategoryParams{
			ID:       id,
			Name:     in.Name,
			SetColor: in.Color != nil,
			Color:    emptyToNil(in.Color),
			SetIcon:  in.Icon != nil,
			Icon:     emptyToNil(in.Icon),
			Archived: in.Archived,
		})
		if isNoRows(err) {
			return NotFound("category")
		}
		if pgErrorCode(err) == pgUniqueViolation {
			return Conflict("an active category with this name already exists")
		}
		if err != nil {
			return err
		}
		details := ActivityDetails{Type: c.Kind}
		if len(changed) > 0 {
			d := details
			d.Changed = changed
			if err := tx.record(ctx, ActionCategoryUpdated, EntityCategory, id, d); err != nil {
				return err
			}
		}
		if in.Archived != nil && *in.Archived != cur.Archived {
			action := ActionCategoryUnarchived
			if *in.Archived {
				action = ActionCategoryArchived
			}
			return tx.record(ctx, action, EntityCategory, id, details)
		}
		return nil
	})
	if err != nil {
		return Category{}, err
	}
	return categoryFromModel(c), nil
}

// DeleteCategory removes a category; its transactions become uncategorized.
func (s *Service) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	category, err := s.GetCategory(ctx, id)
	if err != nil {
		return err
	}
	return s.withTx(ctx, func(tx *Service) error {
		affected, err := tx.q.DeleteCategory(ctx, id)
		if err != nil {
			return err
		}
		if affected == 0 {
			return NotFound("category")
		}
		return tx.record(ctx, ActionCategoryDeleted, EntityCategory, id, ActivityDetails{Type: category.Kind})
	})
}
