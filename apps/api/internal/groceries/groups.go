package groceries

import (
	"context"
	"regexp"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

var colorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// ProductGroup classifies products. is_food tells food from household
// groups.
type ProductGroup struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name" example:"Dairy & eggs"`
	DefaultKey *string   `json:"default_key" nullable:"true" example:"dairy_eggs" doc:"Stable key of a default group (clients may translate it); null once renamed and for custom groups"`
	IsFood     bool      `json:"is_food"`
	Color      *string   `json:"color" example:"#22c55e"`
	Position   int32     `json:"position"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type CreateProductGroupInput struct {
	Name     string  `json:"name" minLength:"1" maxLength:"100"`
	IsFood   bool    `json:"is_food" doc:"True for food, false for household and personal care"`
	Color    *string `json:"color,omitempty" pattern:"^#[0-9a-fA-F]{6}$"`
	Position *int32  `json:"position,omitempty" minimum:"0" doc:"Defaults to the end of the list"`
}

type UpdateProductGroupInput struct {
	Name     *string `json:"name,omitempty" minLength:"1" maxLength:"100"`
	IsFood   *bool   `json:"is_food,omitempty"`
	Color    *string `json:"color,omitempty" doc:"Hex color (#rrggbb); empty string clears it"`
	Position *int32  `json:"position,omitempty" minimum:"0"`
}

// defaultGroups are created the first time a workspace lists its groups.
// Names are plain user data from then on; keys let clients translate them
// until the group is renamed.
var defaultGroups = []struct {
	key    string
	name   string
	isFood bool
	color  string
}{
	{"produce", "Produce", true, "#22c55e"},
	{"meat_fish", "Meat & fish", true, "#ef4444"},
	{"dairy_eggs", "Dairy & eggs", true, "#3b82f6"},
	{"bakery", "Bakery", true, "#f59e0b"},
	{"pantry", "Pantry", true, "#a16207"},
	{"frozen", "Frozen", true, "#06b6d4"},
	{"snacks_sweets", "Snacks & sweets", true, "#ec4899"},
	{"drinks", "Drinks", true, "#0ea5e9"},
	{"alcohol", "Alcohol", true, "#8b5cf6"},
	{"cleaning_household", "Cleaning & household", false, "#14b8a6"},
	{"personal_care", "Personal care", false, "#f472b6"},
	{"other", "Other", false, "#6b7280"},
}

func groupFromModel(g store.ProductGroup) ProductGroup {
	return ProductGroup{
		ID: g.ID, Name: g.Name, DefaultKey: g.DefaultKey, IsFood: g.IsFood, Color: g.Color, Position: g.Position,
		CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt,
	}
}

func validateColor(color *string) error {
	if color != nil && *color != "" && !colorPattern.MatchString(*color) {
		return Invalid("color", "must be a hex color like #22c55e")
	}
	return nil
}

// ListGroups returns the groups, creating the defaults when the workspace
// has none yet. Seeding is idempotent and safe under concurrency: inserts
// that hit an existing name are ignored.
func (s *Service) ListGroups(ctx context.Context) ([]ProductGroup, error) {
	n, err := s.q.CountProductGroups(ctx)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		err := s.withTx(ctx, func(q *store.Queries) error {
			for i, g := range defaultGroups {
				color, key := g.color, g.key
				if err := q.SeedProductGroup(ctx, store.SeedProductGroupParams{Name: g.name, IsFood: g.isFood, Color: &color, Position: int32(i), DefaultKey: &key}); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	rows, err := s.q.ListProductGroups(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ProductGroup, len(rows))
	for i, r := range rows {
		out[i] = groupFromModel(r)
	}
	return out, nil
}

func (s *Service) GetGroup(ctx context.Context, id uuid.UUID) (ProductGroup, error) {
	row, err := s.q.GetProductGroup(ctx, id)
	if isNoRows(err) {
		return ProductGroup{}, NotFound("product group")
	}
	if err != nil {
		return ProductGroup{}, err
	}
	return groupFromModel(row), nil
}

func (s *Service) CreateGroup(ctx context.Context, in CreateProductGroupInput) (ProductGroup, error) {
	name, err := normalizeName("name", in.Name)
	if err != nil {
		return ProductGroup{}, err
	}
	if err := validateColor(in.Color); err != nil {
		return ProductGroup{}, err
	}
	if in.Position != nil && *in.Position < 0 {
		return ProductGroup{}, Invalid("position", "must not be negative")
	}
	// Create the defaults first so a custom group never blocks them.
	if _, err := s.ListGroups(ctx); err != nil {
		return ProductGroup{}, err
	}
	row, err := s.q.CreateProductGroup(ctx, store.CreateProductGroupParams{
		Name: name, IsFood: in.IsFood, Color: emptyToNil(in.Color), Position: in.Position,
	})
	if pgErrorCode(err) == pgUniqueViolation {
		return ProductGroup{}, Conflict("a product group with this name already exists")
	}
	if err != nil {
		return ProductGroup{}, err
	}
	return groupFromModel(row), nil
}

func (s *Service) UpdateGroup(ctx context.Context, id uuid.UUID, in UpdateProductGroupInput) (ProductGroup, error) {
	if in.Name != nil {
		name, err := normalizeName("name", *in.Name)
		if err != nil {
			return ProductGroup{}, err
		}
		in.Name = &name
	}
	if err := validateColor(in.Color); err != nil {
		return ProductGroup{}, err
	}
	if in.Position != nil && *in.Position < 0 {
		return ProductGroup{}, Invalid("position", "must not be negative")
	}
	row, err := s.q.UpdateProductGroup(ctx, store.UpdateProductGroupParams{
		ID: id, Name: in.Name, IsFood: in.IsFood, SetColor: in.Color != nil, Color: emptyToNil(in.Color), Position: in.Position,
	})
	if isNoRows(err) {
		return ProductGroup{}, NotFound("product group")
	}
	if pgErrorCode(err) == pgUniqueViolation {
		return ProductGroup{}, Conflict("a product group with this name already exists")
	}
	if err != nil {
		return ProductGroup{}, err
	}
	return groupFromModel(row), nil
}
