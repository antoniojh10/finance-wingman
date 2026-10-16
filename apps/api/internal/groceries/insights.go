package groceries

import (
	"context"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

// Unit prices are always expressed in minor units per compare unit (kg, l or
// unit) and are kept as floats; only display rounds them. The price of an
// observation is:
//
//	receipt line: amount_minor / (quantity * coalesce(pack_size, 1))
//	price check:  price_minor / coalesce(pack_size, 1)
//
// (see the grocery_unit_prices view). The display unit price multiplies it by
// the product's display multiplier (12: priced per dozen).
const (
	// CompareWindowDays is how far back the store comparison and the basket
	// look for observations.
	CompareWindowDays = 180
	// StaleAfterDays flags an observation as outdated.
	StaleAfterDays = 60

	insightDateLayout = "2006-01-02"
	monthLayout       = "2006-01"
	topProductsLimit  = 10
	historyMonths     = 12
	maxBasketMonths   = 12
)

// Observation sources.
const (
	SourceReceipt    = "receipt"
	SourcePriceCheck = "price_check"
)

func formatInsightDate(t time.Time) string { return t.Format(insightDateLayout) }

func roundMinor(v float64) int64 { return int64(math.Round(v)) }

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func currencyFilter(currency string) (*string, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		return nil, nil
	}
	if len(currency) != 3 {
		return nil, Invalid("currency", "must be a 3-letter ISO 4217 code")
	}
	return &currency, nil
}

func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// --- Breakdown ---

// Breakdown is the spending of a calendar month, one entry per currency.
type Breakdown struct {
	Month      string              `json:"month" example:"2026-09"`
	Currencies []CurrencyBreakdown `json:"currencies"`
}

type CurrencyBreakdown struct {
	Currency            string  `json:"currency" example:"EUR"`
	TotalMinor          int64   `json:"total_minor" doc:"Sum of the receipt totals of the month"`
	ReceiptCount        int64   `json:"receipt_count"`
	AverageReceiptMinor int64   `json:"average_receipt_minor" doc:"Receipt total divided by the receipt count, rounded"`
	LinesTotalMinor     int64   `json:"lines_total_minor" doc:"Sum of the receipt lines (what the group and coverage figures are based on)"`
	AssignedMinor       int64   `json:"assigned_minor" doc:"Lines already mapped to a product"`
	UnassignedMinor     int64   `json:"unassigned_minor" doc:"Lines not mapped to a product yet"`
	CoveragePct         float64 `json:"coverage_pct" doc:"assigned_minor as a percentage of lines_total_minor"`
	FoodMinor           int64   `json:"food_minor" doc:"Lines of products in food groups"`
	NonFoodMinor        int64   `json:"non_food_minor" doc:"Lines of products in non-food groups"`
	// GroceriesCategoryMinor is the expense total of the "Groceries"
	// category in the month, to compare with the receipts; null when the
	// workspace has no such category.
	GroceriesCategoryMinor *int64             `json:"groceries_category_minor" nullable:"true" doc:"Expenses of the Groceries category in the month, null when the category does not exist"`
	ByGroup                []BreakdownGroup   `json:"by_group"`
	ByStore                []BreakdownStore   `json:"by_store"`
	TopProducts            []BreakdownProduct `json:"top_products" doc:"Top 10 products by spend"`
}

type BreakdownGroup struct {
	GroupID    *uuid.UUID `json:"group_id" nullable:"true" format:"uuid" doc:"Null: lines without a product or products without a group"`
	Name       *string    `json:"name" nullable:"true"`
	DefaultKey *string    `json:"default_key" nullable:"true" doc:"Default key of the group while it keeps its default name"`
	Color      *string    `json:"color" nullable:"true"`
	IsFood     *bool      `json:"is_food" nullable:"true"`
	TotalMinor int64      `json:"total_minor"`
	SharePct   float64    `json:"share_pct" doc:"Percentage of lines_total_minor"`
}

type BreakdownStore struct {
	StoreID      uuid.UUID `json:"store_id"`
	Name         string    `json:"name"`
	TotalMinor   int64     `json:"total_minor" doc:"Sum of the receipt totals"`
	ReceiptCount int64     `json:"receipt_count"`
}

type BreakdownProduct struct {
	ProductID   uuid.UUID `json:"product_id"`
	Name        string    `json:"name"`
	CompareUnit string    `json:"compare_unit" enum:"kg,l,unit"`
	Quantity    float64   `json:"quantity" doc:"Bought amount in compare units"`
	SpendMinor  int64     `json:"spend_minor"`
}

// Breakdown returns the spending of month (YYYY-MM, the current month when
// empty), grouped per currency.
func (s *Service) Breakdown(ctx context.Context, month, currency string) (Breakdown, error) {
	cur, err := currencyFilter(currency)
	if err != nil {
		return Breakdown{}, err
	}
	start := monthStart(s.insightsToday())
	if strings.TrimSpace(month) != "" {
		start, err = time.Parse(monthLayout, strings.TrimSpace(month))
		if err != nil {
			return Breakdown{}, Invalid("month", "must be a month in YYYY-MM format")
		}
	}
	end := start.AddDate(0, 1, 0)

	totals, err := s.q.GroceryReceiptTotals(ctx, store.GroceryReceiptTotalsParams{FromDate: start, ToDate: end, Currency: cur})
	if err != nil {
		return Breakdown{}, err
	}
	groupRows, err := s.q.GroceryLineTotalsByGroup(ctx, store.GroceryLineTotalsByGroupParams{FromDate: start, ToDate: end, Currency: cur})
	if err != nil {
		return Breakdown{}, err
	}
	productRows, err := s.q.GroceryProductSpend(ctx, store.GroceryProductSpendParams{FromDate: start, ToDate: end, Currency: cur})
	if err != nil {
		return Breakdown{}, err
	}
	hasCategory, err := s.q.GroceryExpenseCategoryExists(ctx)
	if err != nil {
		return Breakdown{}, err
	}
	var categoryTotals map[string]int64
	if hasCategory {
		rows, err := s.q.GroceryCategoryExpenses(ctx, store.GroceryCategoryExpensesParams{FromDate: start, ToDate: end, Currency: cur})
		if err != nil {
			return Breakdown{}, err
		}
		categoryTotals = make(map[string]int64, len(rows))
		for _, r := range rows {
			categoryTotals[r.Currency] = r.TotalMinor
		}
	}

	byCurrency := map[string]*CurrencyBreakdown{}
	entry := func(code string) *CurrencyBreakdown {
		c, ok := byCurrency[code]
		if !ok {
			c = &CurrencyBreakdown{Currency: code, ByGroup: []BreakdownGroup{}, ByStore: []BreakdownStore{}, TopProducts: []BreakdownProduct{}}
			byCurrency[code] = c
		}
		return c
	}
	for _, r := range totals {
		c := entry(r.Currency)
		c.TotalMinor += r.TotalMinor
		c.ReceiptCount += r.ReceiptCount
		c.ByStore = append(c.ByStore, BreakdownStore{StoreID: r.StoreID, Name: r.StoreName, TotalMinor: r.TotalMinor, ReceiptCount: r.ReceiptCount})
	}
	for _, r := range groupRows {
		c := entry(r.Currency)
		c.LinesTotalMinor += r.TotalMinor
		if r.Assigned {
			c.AssignedMinor += r.TotalMinor
			if r.GroupIsFood != nil {
				if *r.GroupIsFood {
					c.FoodMinor += r.TotalMinor
				} else {
					c.NonFoodMinor += r.TotalMinor
				}
			}
		} else {
			c.UnassignedMinor += r.TotalMinor
		}
		// Unassigned lines and products without a group share the null group.
		if i := slices.IndexFunc(c.ByGroup, func(g BreakdownGroup) bool { return sameID(g.GroupID, r.GroupID) }); i >= 0 {
			c.ByGroup[i].TotalMinor += r.TotalMinor
		} else {
			c.ByGroup = append(c.ByGroup, BreakdownGroup{
				GroupID: r.GroupID, Name: r.GroupName, DefaultKey: r.GroupDefaultKey, Color: r.GroupColor, IsFood: r.GroupIsFood, TotalMinor: r.TotalMinor,
			})
		}
	}
	for _, r := range productRows { // already ordered by spend
		c := entry(r.Currency)
		if len(c.TopProducts) < topProductsLimit {
			c.TopProducts = append(c.TopProducts, BreakdownProduct{
				ProductID: r.ProductID, Name: r.ProductName, CompareUnit: r.CompareUnit, Quantity: round3(r.Quantity), SpendMinor: r.SpendMinor,
			})
		}
	}
	for code := range categoryTotals {
		entry(code) // a currency with Groceries expenses but no receipts still shows up
	}

	out := Breakdown{Month: start.Format(monthLayout), Currencies: make([]CurrencyBreakdown, 0, len(byCurrency))}
	for _, c := range byCurrency {
		if c.ReceiptCount > 0 {
			c.AverageReceiptMinor = roundMinor(float64(c.TotalMinor) / float64(c.ReceiptCount))
		}
		if c.LinesTotalMinor > 0 {
			c.CoveragePct = round1(float64(c.AssignedMinor) / float64(c.LinesTotalMinor) * 100)
		}
		for i := range c.ByGroup {
			if c.LinesTotalMinor > 0 {
				c.ByGroup[i].SharePct = round1(float64(c.ByGroup[i].TotalMinor) / float64(c.LinesTotalMinor) * 100)
			}
		}
		slices.SortStableFunc(c.ByGroup, func(a, b BreakdownGroup) int { return int(b.TotalMinor - a.TotalMinor) })
		slices.SortStableFunc(c.ByStore, func(a, b BreakdownStore) int { return int(b.TotalMinor - a.TotalMinor) })
		if hasCategory {
			total := categoryTotals[c.Currency]
			c.GroceriesCategoryMinor = &total
		}
		out.Currencies = append(out.Currencies, *c)
	}
	slices.SortFunc(out.Currencies, func(a, b CurrencyBreakdown) int { return strings.Compare(a.Currency, b.Currency) })
	return out, nil
}

func sameID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
