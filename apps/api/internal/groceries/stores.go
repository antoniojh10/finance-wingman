package groceries

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

// Store is a supermarket chain.
type Store struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name" example:"Mercadona"`
	Archived  bool      `json:"archived"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateStoreInput struct {
	Name string `json:"name" minLength:"1" maxLength:"100"`
}

type UpdateStoreInput struct {
	Name     *string `json:"name,omitempty" minLength:"1" maxLength:"100"`
	Archived *bool   `json:"archived,omitempty"`
}

func storeFromModel(s store.GroceryStore) Store {
	return Store{ID: s.ID, Name: s.Name, Archived: s.ArchivedAt != nil, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
}

func (s *Service) ListStores(ctx context.Context, includeArchived bool) ([]Store, error) {
	rows, err := s.q.ListGroceryStores(ctx, includeArchived)
	if err != nil {
		return nil, err
	}
	out := make([]Store, len(rows))
	for i, r := range rows {
		out[i] = storeFromModel(r)
	}
	return out, nil
}

func (s *Service) GetStore(ctx context.Context, id uuid.UUID) (Store, error) {
	row, err := s.q.GetGroceryStore(ctx, id)
	if isNoRows(err) {
		return Store{}, NotFound("store")
	}
	if err != nil {
		return Store{}, err
	}
	return storeFromModel(row), nil
}

func (s *Service) CreateStore(ctx context.Context, in CreateStoreInput) (Store, error) {
	name, err := normalizeName("name", in.Name)
	if err != nil {
		return Store{}, err
	}
	row, err := s.q.CreateGroceryStore(ctx, name)
	if pgErrorCode(err) == pgUniqueViolation {
		return Store{}, Conflict("an active store with this name already exists")
	}
	if err != nil {
		return Store{}, err
	}
	return storeFromModel(row), nil
}

func (s *Service) UpdateStore(ctx context.Context, id uuid.UUID, in UpdateStoreInput) (Store, error) {
	if in.Name != nil {
		name, err := normalizeName("name", *in.Name)
		if err != nil {
			return Store{}, err
		}
		in.Name = &name
	}
	row, err := s.q.UpdateGroceryStore(ctx, store.UpdateGroceryStoreParams{ID: id, Name: in.Name, Archived: in.Archived})
	if isNoRows(err) {
		return Store{}, NotFound("store")
	}
	if pgErrorCode(err) == pgUniqueViolation {
		return Store{}, Conflict("an active store with this name already exists")
	}
	if err != nil {
		return Store{}, err
	}
	return storeFromModel(row), nil
}
