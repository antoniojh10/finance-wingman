package groceries

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

const maxNoteLength = 500

// PriceCheck is a price seen in a store without buying.
type PriceCheck struct {
	ID               uuid.UUID `json:"id"`
	StoreID          uuid.UUID `json:"store_id"`
	StoreName        string    `json:"store_name"`
	VariantID        uuid.UUID `json:"variant_id"`
	ProductID        uuid.UUID `json:"product_id"`
	ProductName      string    `json:"product_name"`
	Brand            string    `json:"brand"`
	PackLabel        *string   `json:"pack_label" nullable:"true"`
	CompareUnit      string    `json:"compare_unit" enum:"kg,l,unit"`
	ObservedOn       string    `json:"observed_on" format:"date"`
	PriceMinor       int64     `json:"price_minor" doc:"Price of the whole pack (of one compare unit when the variant is loose)"`
	Currency         string    `json:"currency"`
	UnitPriceMinor   float64   `json:"unit_price_minor" doc:"price_minor divided by the pack size, per compare unit"`
	DisplayUnitPrice float64   `json:"display_unit_price" doc:"unit_price_minor times the product's display multiplier"`
	Note             *string   `json:"note" nullable:"true"`
	CreatedAt        time.Time `json:"created_at"`
}

type CreatePriceCheckInput struct {
	StoreID    string  `json:"store_id" format:"uuid"`
	VariantID  string  `json:"variant_id" format:"uuid"`
	ObservedOn string  `json:"observed_on,omitempty" format:"date" doc:"Defaults to today; cannot be in the future"`
	PriceMinor int64   `json:"price_minor" minimum:"1" doc:"Price of the whole pack in minor units"`
	Currency   string  `json:"currency" minLength:"3" maxLength:"3" example:"EUR"`
	Note       *string `json:"note,omitempty" maxLength:"500"`
}

type ListPriceChecksParams struct {
	VariantID *uuid.UUID
	ProductID *uuid.UUID
	StoreID   *uuid.UUID
}

func priceCheckFromRow(r store.GetPriceCheckDetailRow) PriceCheck {
	return PriceCheck{
		ID: r.ID, StoreID: r.StoreID, StoreName: r.StoreName, VariantID: r.VariantID, ProductID: r.ProductID,
		ProductName: r.ProductName, Brand: r.Brand, PackLabel: r.PackLabel, CompareUnit: r.CompareUnit,
		ObservedOn: formatInsightDate(r.ObservedOn), PriceMinor: r.PriceMinor, Currency: r.Currency,
		UnitPriceMinor: r.UnitPriceMinor, DisplayUnitPrice: r.UnitPriceMinor * float64(r.DisplayMultiplier),
		Note: r.Note, CreatedAt: r.CreatedAt,
	}
}

func (s *Service) ListPriceChecks(ctx context.Context, p ListPriceChecksParams) ([]PriceCheck, error) {
	rows, err := s.q.ListPriceCheckDetails(ctx, store.ListPriceCheckDetailsParams{
		VariantID: p.VariantID, ProductID: p.ProductID, StoreID: p.StoreID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]PriceCheck, len(rows))
	for i, r := range rows {
		out[i] = priceCheckFromRow(store.GetPriceCheckDetailRow(r))
	}
	return out, nil
}

func (s *Service) CreatePriceCheck(ctx context.Context, in CreatePriceCheckInput) (PriceCheck, error) {
	storeID, err := parseIDField("store_id", in.StoreID)
	if err != nil {
		return PriceCheck{}, err
	}
	variantID, err := parseIDField("variant_id", in.VariantID)
	if err != nil {
		return PriceCheck{}, err
	}
	if in.PriceMinor <= 0 {
		return PriceCheck{}, Invalid("price_minor", "must be greater than 0")
	}
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if _, err := s.q.GetCurrency(ctx, currency); isNoRows(err) {
		return PriceCheck{}, Invalid("currency", "unsupported currency "+in.Currency)
	} else if err != nil {
		return PriceCheck{}, err
	}
	today := s.insightsToday()
	observed := today
	if strings.TrimSpace(in.ObservedOn) != "" {
		observed, err = time.Parse(insightDateLayout, strings.TrimSpace(in.ObservedOn))
		if err != nil {
			return PriceCheck{}, Invalid("observed_on", "must be a date in YYYY-MM-DD format")
		}
		if observed.After(today) {
			return PriceCheck{}, Invalid("observed_on", "cannot be in the future")
		}
	}
	note := emptyToNil(in.Note)
	if note != nil && len([]rune(*note)) > maxNoteLength {
		return PriceCheck{}, Invalid("note", "must be at most 500 characters")
	}
	st, err := s.q.GetGroceryStore(ctx, storeID)
	if isNoRows(err) {
		return PriceCheck{}, Invalid("store_id", "unknown store")
	} else if err != nil {
		return PriceCheck{}, err
	}
	if st.ArchivedAt != nil {
		return PriceCheck{}, Invalid("store_id", "store is archived")
	}
	if _, err := s.q.GetProductVariant(ctx, variantID); isNoRows(err) {
		return PriceCheck{}, Invalid("variant_id", "unknown variant")
	} else if err != nil {
		return PriceCheck{}, err
	}
	row, err := s.q.InsertPriceCheck(ctx, store.InsertPriceCheckParams{
		StoreID: storeID, VariantID: variantID, ObservedOn: observed, PriceMinor: in.PriceMinor, Currency: currency, Note: note,
	})
	if err != nil {
		return PriceCheck{}, err
	}
	detail, err := s.q.GetPriceCheckDetail(ctx, row.ID)
	if err != nil {
		return PriceCheck{}, err
	}
	return priceCheckFromRow(detail), nil
}

func (s *Service) DeletePriceCheck(ctx context.Context, id uuid.UUID) error {
	n, err := s.q.DeletePriceCheck(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return NotFound("price check")
	}
	return nil
}

func parseIDField(field, raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return uuid.Nil, Invalid(field, "must be a valid UUID")
	}
	return id, nil
}

// --- Product prices ---

// ProductPrices is the price picture of one product: latest price per
// variant and store, and the monthly history.
type ProductPrices struct {
	ProductID         uuid.UUID       `json:"product_id"`
	Name              string          `json:"name"`
	CompareUnit       string          `json:"compare_unit" enum:"kg,l,unit"`
	DisplayMultiplier int32           `json:"display_multiplier"`
	Variants          []VariantPrices `json:"variants"`
}

type VariantPrices struct {
	VariantID uuid.UUID     `json:"variant_id"`
	Brand     string        `json:"brand"`
	PackLabel *string       `json:"pack_label" nullable:"true"`
	PackSize  *float64      `json:"pack_size" nullable:"true"`
	StoreID   *uuid.UUID    `json:"store_id" nullable:"true" format:"uuid" doc:"Store whose own brand this is"`
	Latest    []StorePrice  `json:"latest" doc:"Latest price per store and currency"`
	History   []PriceSeries `json:"history" doc:"Average unit price per month over the last 12 months"`
}

type StorePrice struct {
	StoreID          uuid.UUID `json:"store_id"`
	StoreName        string    `json:"store_name"`
	Currency         string    `json:"currency"`
	UnitPriceMinor   float64   `json:"unit_price_minor"`
	DisplayUnitPrice float64   `json:"display_unit_price"`
	ObservedOn       string    `json:"observed_on" format:"date"`
	Source           string    `json:"source" enum:"receipt,price_check"`
	Stale            bool      `json:"stale"`
}

type PriceSeries struct {
	StoreID   uuid.UUID    `json:"store_id"`
	StoreName string       `json:"store_name"`
	Currency  string       `json:"currency"`
	Points    []PricePoint `json:"points"`
}

type PricePoint struct {
	Month            string  `json:"month" example:"2026-09"`
	UnitPriceMinor   float64 `json:"unit_price_minor" doc:"Average of the month's observations"`
	DisplayUnitPrice float64 `json:"display_unit_price"`
	Observations     int64   `json:"observations"`
}

// ProductPrices returns the latest price of every variant of a product at
// each store plus its monthly price history (last 12 months, the current one
// included). Currencies are never mixed: each entry carries its currency.
func (s *Service) ProductPrices(ctx context.Context, id uuid.UUID) (ProductPrices, error) {
	product, err := s.GetProduct(ctx, id)
	if err != nil {
		return ProductPrices{}, err
	}
	variants, err := s.q.ListProductVariants(ctx, id)
	if err != nil {
		return ProductPrices{}, err
	}
	latest, today, err := s.latestObservations(ctx, nil, &id, false)
	if err != nil {
		return ProductPrices{}, err
	}
	history, err := s.q.GroceryProductPriceHistory(ctx, store.GroceryProductPriceHistoryParams{
		ProductID: id, Since: monthStart(today).AddDate(0, -(historyMonths - 1), 0), Until: today,
	})
	if err != nil {
		return ProductPrices{}, err
	}
	mult := float64(product.DisplayMultiplier)
	out := ProductPrices{
		ProductID: product.ID, Name: product.Name, CompareUnit: product.CompareUnit,
		DisplayMultiplier: product.DisplayMultiplier, Variants: make([]VariantPrices, 0, len(variants)),
	}
	type seriesKey struct {
		variant, store uuid.UUID
		currency       string
	}
	names := map[uuid.UUID]string{}
	for _, v := range variants {
		vp := VariantPrices{
			VariantID: v.ID, Brand: v.Brand, PackLabel: v.PackLabel, PackSize: floatFromNumeric(v.PackSize), StoreID: v.StoreID,
			Latest: []StorePrice{}, History: []PriceSeries{},
		}
		for _, r := range latest {
			if r.VariantID == nil || *r.VariantID != v.ID {
				continue
			}
			names[r.StoreID] = r.StoreName
			c := priceCellFromRow(r, today)
			vp.Latest = append(vp.Latest, StorePrice{
				StoreID: r.StoreID, StoreName: r.StoreName, Currency: r.Currency, UnitPriceMinor: c.UnitPriceMinor,
				DisplayUnitPrice: c.DisplayUnitPrice, ObservedOn: c.ObservedOn, Source: c.Source, Stale: c.Stale,
			})
		}
		slices.SortFunc(vp.Latest, func(a, b StorePrice) int {
			if c := strings.Compare(strings.ToLower(a.StoreName), strings.ToLower(b.StoreName)); c != 0 {
				return c
			}
			return strings.Compare(a.Currency, b.Currency)
		})
		series := map[seriesKey]int{}
		for _, h := range history {
			if h.VariantID == nil || *h.VariantID != v.ID {
				continue
			}
			k := seriesKey{v.ID, h.StoreID, h.Currency}
			i, ok := series[k]
			if !ok {
				vp.History = append(vp.History, PriceSeries{StoreID: h.StoreID, StoreName: names[h.StoreID], Currency: h.Currency, Points: []PricePoint{}})
				i = len(vp.History) - 1
				series[k] = i
			}
			vp.History[i].Points = append(vp.History[i].Points, PricePoint{
				Month: h.Month, UnitPriceMinor: h.AvgUnitPriceMinor, DisplayUnitPrice: h.AvgUnitPriceMinor * mult, Observations: h.Observations,
			})
		}
		slices.SortFunc(vp.History, func(a, b PriceSeries) int {
			if c := strings.Compare(strings.ToLower(a.StoreName), strings.ToLower(b.StoreName)); c != 0 {
				return c
			}
			return strings.Compare(a.Currency, b.Currency)
		})
		out.Variants = append(out.Variants, vp)
	}
	return out, nil
}
