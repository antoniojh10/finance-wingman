package finance

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

// RecurringSuggestion is a recurring pattern detected among unlinked
// transactions, ready to be accepted as a recurring item.
type RecurringSuggestion struct {
	Key            string      `json:"key" doc:"Opaque identifier to accept or dismiss the suggestion. Stable while the pattern is the same account, type and description"`
	Name           string      `json:"name" example:"Spotify" doc:"Description of the most recent matching transaction"`
	Type           string      `json:"type" enum:"expense,income"`
	AccountID      uuid.UUID   `json:"account_id"`
	AccountName    string      `json:"account_name"`
	Currency       string      `json:"currency" example:"MXN"`
	MinorUnits     int         `json:"minor_units" example:"2"`
	CategoryID     *uuid.UUID  `json:"category_id" nullable:"true" doc:"Most common category of the matching transactions"`
	CategoryName   *string     `json:"category_name"`
	Amount         int64       `json:"amount" doc:"Estimated amount of each occurrence (median of the matches), in minor units"`
	Frequency      string      `json:"frequency" enum:"weekly,monthly,yearly"`
	IntervalUnit   string      `json:"interval_unit" enum:"week,month,year"`
	IntervalCount  int         `json:"interval_count"`
	StartOn        string      `json:"start_on" format:"date" doc:"First due date the accepted item will have"`
	NextDueOn      *string     `json:"next_due_on" format:"date" nullable:"true" doc:"Next due date on or after today"`
	TransactionIDs []uuid.UUID `json:"transaction_ids" doc:"Matching transactions that accepting links to the new item, oldest first"`
	Confidence     float64     `json:"confidence" minimum:"0" maximum:"1" doc:"0 to 1: more occurrences, regular dates and steady amounts score higher"`
}

// SuggestionKeyInput identifies a suggestion to dismiss.
type SuggestionKeyInput struct {
	Key string `json:"key" minLength:"1" doc:"Key of a suggestion"`
}

// AcceptSuggestionInput accepts a suggestion, optionally overriding its
// name and amount.
type AcceptSuggestionInput struct {
	Key        string     `json:"key" minLength:"1" doc:"Key of a suggestion"`
	Name       string     `json:"name,omitempty" maxLength:"100" doc:"Name of the item. Defaults to the suggested name"`
	Amount     *int64     `json:"amount,omitempty" minimum:"1" maximum:"1000000000000000" doc:"Estimated amount in minor units. Defaults to the suggested amount"`
	CategoryID *uuid.UUID `json:"category_id,omitempty" format:"uuid" doc:"Category whose kind matches the type. Defaults to the suggested category"`
}

// RecurringMatch is the best active recurring item for a transaction.
type RecurringMatch struct {
	Item        *RecurringItem `json:"item" doc:"Best matching active item; null when none matches"`
	PeriodDueOn *string        `json:"period_due_on" format:"date" nullable:"true" doc:"Due date the transaction would settle if linked"`
}

type suggestionRow struct {
	detection
	AccountName  string
	Currency     string
	MinorUnits   int
	CategoryID   *uuid.UUID
	CategoryName *string
	Name         string
}

func encodeSuggestionKey(k groupKey) string {
	raw := k.Type + "|" + k.AccountID.String() + "|" + k.Description
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeSuggestionKey(key string) (groupKey, error) {
	raw, err := base64.RawURLEncoding.DecodeString(key)
	if err != nil {
		return groupKey{}, Invalid("key", "not a valid suggestion key; use the key of a listed suggestion")
	}
	parts := strings.SplitN(string(raw), "|", 3)
	if len(parts) != 3 || (parts[0] != TypeExpense && parts[0] != TypeIncome) || parts[2] == "" {
		return groupKey{}, Invalid("key", "not a valid suggestion key; use the key of a listed suggestion")
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return groupKey{}, Invalid("key", "not a valid suggestion key; use the key of a listed suggestion")
	}
	return groupKey{AccountID: id, Type: parts[0], Description: parts[2]}, nil
}

// detectSuggestions loads the unlinked transactions of the lookback window
// and runs the heuristic, leaving out dismissed groups.
func (s *Service) detectSuggestions(ctx context.Context) ([]suggestionRow, error) {
	today := s.Today()
	rows, err := s.q.ListRecurringCandidates(ctx, store.ListRecurringCandidatesParams{
		FromDate: today.AddDate(0, -maxLookbackMonths, 0),
		ToDate:   today,
	})
	if err != nil {
		return nil, err
	}
	type meta struct {
		accountName, currency string
		minorUnits            int
		categories            map[uuid.UUID]string
	}
	metas := map[uuid.UUID]*meta{}
	cands := make([]candidate, len(rows))
	for i, r := range rows {
		m := metas[r.AccountID]
		if m == nil {
			m = &meta{accountName: r.AccountName, currency: r.Currency, minorUnits: int(r.MinorUnits), categories: map[uuid.UUID]string{}}
			metas[r.AccountID] = m
		}
		c := candidate{
			ID: r.ID, AccountID: r.AccountID, Type: r.Type, Description: strings.TrimSpace(r.Description),
			OccurredOn: r.OccurredOn, Amount: r.Amount,
		}
		if r.CategoryID != uuid.Nil {
			id := r.CategoryID
			c.CategoryID = &id
			m.categories[id] = r.CategoryName
		}
		cands[i] = c
	}

	dismissed, err := s.q.ListDismissedSuggestions(ctx)
	if err != nil {
		return nil, err
	}
	skip := map[groupKey]bool{}
	for _, d := range dismissed {
		skip[groupKey{AccountID: d.AccountID, Type: d.Type, Description: d.Description}] = true
	}

	var out []suggestionRow
	for _, d := range detectRecurring(cands, today) {
		if skip[d.Group] {
			continue
		}
		m := metas[d.Group.AccountID]
		row := suggestionRow{
			detection: d, AccountName: m.accountName, Currency: m.currency, MinorUnits: m.minorUnits,
			Name: truncateRunes(d.Chain[len(d.Chain)-1].Description, 100),
		}
		if cat := mostCommonCategory(d.Chain); cat != nil {
			name := m.categories[*cat]
			row.CategoryID, row.CategoryName = cat, &name
		}
		out = append(out, row)
	}
	return out, nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n]))
}

func (s *Service) suggestionOut(r suggestionRow) RecurringSuggestion {
	startOn := suggestedStartOn(r.detection)
	sched := Schedule{Unit: r.Spec.unit, Count: 1, StartOn: startOn}
	ids := make([]uuid.UUID, len(r.Chain))
	for i, c := range r.Chain {
		ids[i] = c.ID
	}
	out := RecurringSuggestion{
		Key:            encodeSuggestionKey(r.Group),
		Name:           r.Name,
		Type:           r.Group.Type,
		AccountID:      r.Group.AccountID,
		AccountName:    r.AccountName,
		Currency:       r.Currency,
		MinorUnits:     r.MinorUnits,
		CategoryID:     r.CategoryID,
		CategoryName:   r.CategoryName,
		Amount:         r.Amount,
		Frequency:      r.Spec.name,
		IntervalUnit:   r.Spec.unit,
		IntervalCount:  1,
		StartOn:        formatDate(startOn),
		TransactionIDs: ids,
		Confidence:     r.Confidence,
	}
	if next, ok := sched.NextDueOn(s.Today()); ok {
		d := formatDate(next)
		out.NextDueOn = &d
	}
	return out
}

// ListRecurringSuggestions detects recurring patterns among unlinked
// expenses and incomes, on demand. Dismissed patterns are left out; linked
// transactions are never considered. Ordered by confidence.
func (s *Service) ListRecurringSuggestions(ctx context.Context) ([]RecurringSuggestion, error) {
	rows, err := s.detectSuggestions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RecurringSuggestion, len(rows))
	for i, r := range rows {
		out[i] = s.suggestionOut(r)
	}
	return out, nil
}

// AcceptRecurringSuggestion creates the recurring item of a suggestion and
// links its matching transactions, in one database transaction. Each
// transaction settles the due date closest to its date. The detection is
// run again inside the transaction, so only the key travels from the
// client. A name clash with another recurring item fails like any create.
func (s *Service) AcceptRecurringSuggestion(ctx context.Context, in AcceptSuggestionInput) (RecurringItem, error) {
	key, err := decodeSuggestionKey(in.Key)
	if err != nil {
		return RecurringItem{}, err
	}
	var item RecurringItem
	err = s.withTx(ctx, func(tx *Service) error {
		rows, err := tx.detectSuggestions(ctx)
		if err != nil {
			return err
		}
		var found *suggestionRow
		for i := range rows {
			if rows[i].Group == key {
				found = &rows[i]
				break
			}
		}
		if found == nil {
			return NotFound("suggestion")
		}
		sug := tx.suggestionOut(*found)
		name := sug.Name
		if strings.TrimSpace(in.Name) != "" {
			name = in.Name
		}
		amount := sug.Amount
		if in.Amount != nil {
			amount = *in.Amount
		}
		categoryID := sug.CategoryID
		if in.CategoryID != nil {
			categoryID = in.CategoryID
		}
		created, err := tx.CreateRecurringItem(ctx, CreateRecurringItemInput{
			Name:          name,
			Type:          sug.Type,
			AccountID:     sug.AccountID,
			CategoryID:    categoryID,
			Amount:        amount,
			IntervalUnit:  sug.IntervalUnit,
			IntervalCount: sug.IntervalCount,
			StartOn:       sug.StartOn,
		})
		if err != nil {
			return err
		}
		sched := created.schedule()
		for _, c := range found.Chain {
			period := sched.ClosestDueOn(c.OccurredOn)
			rec, err := tx.q.GetTransactionRecord(ctx, c.ID)
			if err != nil {
				return err
			}
			if err := tx.q.LinkTransactionToRecurring(ctx, store.LinkTransactionToRecurringParams{
				ID: c.ID, RecurringID: &created.ID, RecurringDueOn: &period,
			}); err != nil {
				return err
			}
			if err := tx.recordRecurringLink(ctx, rec, &created.ID, &period); err != nil {
				return err
			}
		}
		item, err = tx.GetRecurringItem(ctx, created.ID)
		return err
	})
	return item, err
}

// DismissRecurringSuggestion hides a suggestion for good. It is idempotent
// and works whether or not the pattern is currently detected.
func (s *Service) DismissRecurringSuggestion(ctx context.Context, in SuggestionKeyInput) error {
	key, err := decodeSuggestionKey(in.Key)
	if err != nil {
		return err
	}
	p := store.DismissSuggestionParams{AccountID: key.AccountID, Type: key.Type, Description: key.Description}
	if actor, ok := ActorFrom(ctx); ok {
		p.DismissedBy = &actor
	}
	if err := s.q.DismissSuggestion(ctx, p); err != nil {
		if pgErrorCode(err) == pgForeignKeyViolation {
			return Invalid("key", "account of the suggestion no longer exists")
		}
		return err
	}
	return nil
}

// MatchRecurringItem returns the best active recurring item for a
// transaction, for "link this to Netflix?" prompts. Only unlinked expenses
// and incomes qualify, and the item must have the same type and account and
// a name that matches the normalized description (equal, or one contains
// the other). Among those, an amount within +-15% of the item's estimate
// comes first, then the smallest amount difference and the closest due
// date. It returns an empty match when nothing fits.
func (s *Service) MatchRecurringItem(ctx context.Context, txID uuid.UUID) (RecurringMatch, error) {
	rec, err := s.q.GetTransactionRecord(ctx, txID)
	if isNoRows(err) {
		return RecurringMatch{}, NotFound("transaction")
	}
	if err != nil {
		return RecurringMatch{}, err
	}
	desc := normalizeDescription(rec.Description)
	if rec.RecurringID != nil || desc == "" || (rec.Type != TypeExpense && rec.Type != TypeIncome) {
		return RecurringMatch{}, nil
	}
	active := StatusActive
	items, err := s.ListRecurringItems(ctx, RecurringFilter{Status: &active, Type: &rec.Type})
	if err != nil {
		return RecurringMatch{}, err
	}

	type scored struct {
		item     RecurringItem
		inRange  bool
		amount   int64
		dueDelta int
		due      time.Time
	}
	var best *scored
	for _, it := range items {
		if it.AccountID != rec.AccountID || !namesMatch(desc, normalizeDescription(it.Name)) {
			continue
		}
		due := it.schedule().ClosestDueOn(rec.OccurredOn)
		diff := rec.Amount - it.Amount
		if diff < 0 {
			diff = -diff
		}
		c := scored{item: it, inRange: withinAmountTolerance(rec.Amount, it.Amount), amount: diff, due: due, dueDelta: absInt(daysBetween(due, rec.OccurredOn))}
		better := best == nil
		if !better {
			switch {
			case c.inRange != best.inRange:
				better = c.inRange
			case c.amount != best.amount:
				better = c.amount < best.amount
			case c.dueDelta != best.dueDelta:
				better = c.dueDelta < best.dueDelta
			default:
				better = strings.ToLower(c.item.Name) < strings.ToLower(best.item.Name)
			}
		}
		if better {
			cc := c
			best = &cc
		}
	}
	if best == nil {
		return RecurringMatch{}, nil
	}
	d := formatDate(best.due)
	return RecurringMatch{Item: &best.item, PeriodDueOn: &d}, nil
}

// namesMatch reports whether two normalized texts are equal or one
// contains the other (the shorter one at least 3 characters long).
func namesMatch(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	short, long := a, b
	if len(short) > len(long) {
		short, long = long, short
	}
	return len(short) >= 3 && strings.Contains(long, short)
}
