package finance

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// MaxBatchSize is the maximum number of items accepted by a batch create.
const MaxBatchSize = 100

// withTx runs fn against a Service bound to a single database transaction.
// The transaction is committed when fn returns nil and rolled back otherwise.
func (s *Service) withTx(ctx context.Context, fn func(tx *Service) error) error {
	dbTx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = dbTx.Rollback(ctx) }()

	scoped := *s
	scoped.q = s.q.WithTx(dbTx)
	if err := fn(&scoped); err != nil {
		return err
	}
	return dbTx.Commit(ctx)
}

func checkBatchSize(n int) error {
	if n == 0 {
		return Invalid("items", "must contain at least one item")
	}
	if n > MaxBatchSize {
		return Invalid("items", fmt.Sprintf("must contain at most %d items, got %d", MaxBatchSize, n))
	}
	return nil
}

// itemError points a domain error at the failing batch item, e.g.
// "items[3].name: must not be empty". Non-domain errors pass through.
func itemError(index int, err error) error {
	var domainErr *Error
	if !errors.As(err, &domainErr) {
		return err
	}
	prefix := fmt.Sprintf("items[%d]", index)
	out := *domainErr
	switch {
	case domainErr.Field != "":
		out.Field = prefix + "." + domainErr.Field
	case domainErr.Kind == KindConflict:
		out.Message = prefix + ": " + domainErr.Message
	default:
		out.Field = prefix
	}
	return &out
}

// firstSeen detects duplicate keys inside a batch.
type firstSeen map[string]int

func (f firstSeen) check(index int, key string) error {
	if first, dup := f[key]; dup {
		return Invalid(fmt.Sprintf("items[%d].name", index),
			fmt.Sprintf("duplicates items[%d].name in this batch", first))
	}
	f[key] = index
	return nil
}

// CreateCategories creates all categories atomically: if any item is
// invalid or conflicts, nothing is created.
func (s *Service) CreateCategories(ctx context.Context, items []CreateCategoryInput) ([]Category, error) {
	if err := checkBatchSize(len(items)); err != nil {
		return nil, err
	}
	seen := firstSeen{}
	for i, in := range items {
		name, err := normalizeName("name", in.Name)
		if err != nil {
			return nil, itemError(i, err)
		}
		if err := seen.check(i, strings.ToLower(name)+"|"+in.Kind); err != nil {
			return nil, err
		}
	}
	out := make([]Category, 0, len(items))
	err := s.withTx(ctx, func(tx *Service) error {
		for i, in := range items {
			c, err := tx.CreateCategory(ctx, in)
			if err != nil {
				return itemError(i, err)
			}
			out = append(out, c)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CreateAccounts creates all accounts atomically.
func (s *Service) CreateAccounts(ctx context.Context, items []CreateAccountInput) ([]Account, error) {
	if err := checkBatchSize(len(items)); err != nil {
		return nil, err
	}
	seen := firstSeen{}
	for i, in := range items {
		name, err := normalizeName("name", in.Name)
		if err != nil {
			return nil, itemError(i, err)
		}
		if err := seen.check(i, strings.ToLower(name)); err != nil {
			return nil, err
		}
	}
	out := make([]Account, 0, len(items))
	err := s.withTx(ctx, func(tx *Service) error {
		for i, in := range items {
			a, err := tx.CreateAccount(ctx, in)
			if err != nil {
				return itemError(i, err)
			}
			out = append(out, a)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CreateTransactions creates all transactions atomically.
func (s *Service) CreateTransactions(ctx context.Context, items []TransactionInput) ([]Transaction, error) {
	if err := checkBatchSize(len(items)); err != nil {
		return nil, err
	}
	out := make([]Transaction, 0, len(items))
	err := s.withTx(ctx, func(tx *Service) error {
		for i, in := range items {
			t, err := tx.CreateTransaction(ctx, in)
			if err != nil {
				return itemError(i, err)
			}
			out = append(out, t)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
