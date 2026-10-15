package groceries

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

// StoreRef identifies a store column of a comparison.
type StoreRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// PriceCell is the best latest price of a product (or brand) at one store.
type PriceCell struct {
	StoreID          uuid.UUID `json:"store_id"`
	VariantID        uuid.UUID `json:"variant_id" doc:"Variant with the lowest unit price at this store"`
	Brand            string    `json:"brand"`
	PackLabel        *string   `json:"pack_label" nullable:"true"`
	PackSize         *float64  `json:"pack_size" nullable:"true"`
	UnitPriceMinor   float64   `json:"unit_price_minor" doc:"Minor units per compare unit"`
	DisplayUnitPrice float64   `json:"display_unit_price" doc:"unit_price_minor times the product's display multiplier"`
	ObservedOn       string    `json:"observed_on" format:"date"`
	Source           string    `json:"source" enum:"receipt,price_check"`
	Stale            bool      `json:"stale" doc:"Observed more than 60 days ago"`
}

func priceCellFromRow(r store.GroceryLatestUnitPricesRow, today time.Time) PriceCell {
	variantID := uuid.Nil
	if r.VariantID != nil {
		variantID = *r.VariantID
	}
	return PriceCell{
		StoreID: r.StoreID, VariantID: variantID, Brand: r.Brand, PackLabel: r.PackLabel, PackSize: floatFromNumeric(r.PackSize),
		UnitPriceMinor: r.UnitPriceMinor, DisplayUnitPrice: r.UnitPriceMinor * float64(r.DisplayMultiplier),
		ObservedOn: formatInsightDate(r.ObservedOn), Source: r.Source,
		Stale: r.ObservedOn.Before(today.AddDate(0, 0, -StaleAfterDays)),
	}
}

// latestObservations returns the latest observation per (store, variant,
// currency) of the last CompareWindowDays days (or of all time for a single
// product).
func (s *Service) latestObservations(ctx context.Context, currency *string, productID *uuid.UUID, activeStoresOnly bool) ([]store.GroceryLatestUnitPricesRow, time.Time, error) {
	today := s.insightsToday()
	since := today.AddDate(0, 0, -CompareWindowDays)
	if productID != nil {
		since = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	rows, err := s.q.GroceryLatestUnitPrices(ctx, store.GroceryLatestUnitPricesParams{
		Since: since, Until: today, Currency: currency, ProductID: productID, ActiveStoresOnly: activeStoresOnly,
	})
	return rows, today, err
}

// cheapestPerStore keeps, for every store, the row with the lowest unit price.
func cheapestPerStore(rows []store.GroceryLatestUnitPricesRow) map[uuid.UUID]store.GroceryLatestUnitPricesRow {
	best := map[uuid.UUID]store.GroceryLatestUnitPricesRow{}
	for _, r := range rows {
		if cur, ok := best[r.StoreID]; !ok || r.UnitPriceMinor < cur.UnitPriceMinor {
			best[r.StoreID] = r
		}
	}
	return best
}

func storesOf(rows []store.GroceryLatestUnitPricesRow) []StoreRef {
	seen := map[uuid.UUID]bool{}
	var out []StoreRef
	for _, r := range rows {
		if !seen[r.StoreID] {
			seen[r.StoreID] = true
			out = append(out, StoreRef{ID: r.StoreID, Name: r.StoreName})
		}
	}
	slices.SortFunc(out, func(a, b StoreRef) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return out
}

func groupByCurrency(rows []store.GroceryLatestUnitPricesRow) map[string][]store.GroceryLatestUnitPricesRow {
	out := map[string][]store.GroceryLatestUnitPricesRow{}
	for _, r := range rows {
		out[r.Currency] = append(out[r.Currency], r)
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// --- Store comparison ---

// Comparison levels.
const (
	LevelProduct = "product"
	LevelBrand   = "brand"
)

type Comparison struct {
	Level      string               `json:"level" enum:"product,brand"`
	Currencies []CurrencyComparison `json:"currencies"`
}

type CurrencyComparison struct {
	Currency string       `json:"currency"`
	Stores   []StoreRef   `json:"stores" doc:"Columns: non-archived stores with prices in the last 180 days"`
	Rows     []CompareRow `json:"rows"`
}

type CompareRow struct {
	ProductID         uuid.UUID            `json:"product_id"`
	ProductName       string               `json:"product_name"`
	Brand             *string              `json:"brand" nullable:"true" doc:"Set at brand level"`
	CompareUnit       string               `json:"compare_unit" enum:"kg,l,unit"`
	DisplayMultiplier int32                `json:"display_multiplier"`
	Cells             map[string]PriceCell `json:"cells" doc:"Best price per store id; stores without a price are absent"`
	BestStoreID       *uuid.UUID           `json:"best_store_id" nullable:"true" format:"uuid" doc:"Cheapest store; null when only one store has a price"`
	MaxDiffPct        *float64             `json:"max_diff_pct" nullable:"true" doc:"(most expensive / cheapest - 1) * 100 over the stores with a price"`
}

// Compare builds the store price matrix. At product level every product with
// observations in the last 180 days gets, per store, its cheapest variant
// (latest observation of each variant). At brand level there is a row per
// (product, brand) priced in at least two stores; store brands and the
// generic (unbranded) variants are excluded.
func (s *Service) Compare(ctx context.Context, level, currency string) (Comparison, error) {
	if level == "" {
		level = LevelProduct
	}
	if level != LevelProduct && level != LevelBrand {
		return Comparison{}, Invalid("level", "must be product or brand")
	}
	cur, err := currencyFilter(currency)
	if err != nil {
		return Comparison{}, err
	}
	rows, today, err := s.latestObservations(ctx, cur, nil, true)
	if err != nil {
		return Comparison{}, err
	}
	out := Comparison{Level: level, Currencies: []CurrencyComparison{}}
	byCurrency := groupByCurrency(rows)
	for _, code := range sortedKeys(byCurrency) {
		cr := byCurrency[code]
		if level == LevelBrand {
			cr = slices.DeleteFunc(slices.Clone(cr), func(r store.GroceryLatestUnitPricesRow) bool {
				return r.VariantStoreID != nil || strings.EqualFold(r.Brand, genericBrand)
			})
		}
		cc := CurrencyComparison{Currency: code, Stores: storesOf(cr), Rows: []CompareRow{}}
		type key struct {
			product uuid.UUID
			brand   string
		}
		groups := map[key][]store.GroceryLatestUnitPricesRow{}
		var order []key
		for _, r := range cr {
			k := key{product: r.ProductID}
			if level == LevelBrand {
				k.brand = strings.ToLower(r.Brand)
			}
			if _, ok := groups[k]; !ok {
				order = append(order, k)
			}
			groups[k] = append(groups[k], r)
		}
		for _, k := range order {
			g := groups[k]
			best := cheapestPerStore(g)
			if level == LevelBrand && len(best) < 2 {
				continue
			}
			first := g[0]
			row := CompareRow{
				ProductID: first.ProductID, ProductName: first.ProductName, CompareUnit: first.CompareUnit,
				DisplayMultiplier: first.DisplayMultiplier, Cells: make(map[string]PriceCell, len(best)),
			}
			if level == LevelBrand {
				brand := first.Brand
				row.Brand = &brand
			}
			var lowest, highest float64
			var lowestStore uuid.UUID
			for _, st := range cc.Stores { // deterministic: ties go to the first store by name
				r, ok := best[st.ID]
				if !ok {
					continue
				}
				row.Cells[st.ID.String()] = priceCellFromRow(r, today)
				if len(row.Cells) == 1 || r.UnitPriceMinor < lowest {
					lowest, lowestStore = r.UnitPriceMinor, st.ID
				}
				if len(row.Cells) == 1 || r.UnitPriceMinor > highest {
					highest = r.UnitPriceMinor
				}
			}
			if len(row.Cells) > 1 {
				row.BestStoreID = &lowestStore
				diff := round1((highest/lowest - 1) * 100)
				row.MaxDiffPct = &diff
			}
			cc.Rows = append(cc.Rows, row)
		}
		slices.SortStableFunc(cc.Rows, func(a, b CompareRow) int {
			if c := strings.Compare(strings.ToLower(a.ProductName), strings.ToLower(b.ProductName)); c != 0 {
				return c
			}
			if a.Brand != nil && b.Brand != nil {
				return strings.Compare(strings.ToLower(*a.Brand), strings.ToLower(*b.Brand))
			}
			return 0
		})
		if level == LevelBrand { // columns: only stores with a compared brand
			cc.Stores = slices.DeleteFunc(cc.Stores, func(st StoreRef) bool {
				return !slices.ContainsFunc(cc.Rows, func(r CompareRow) bool { _, ok := r.Cells[st.ID.String()]; return ok })
			})
		}
		if len(cc.Rows) > 0 {
			out.Currencies = append(out.Currencies, cc)
		}
	}
	return out, nil
}

const genericBrand = "generic"

// --- Basket ---

type Basket struct {
	Months     int              `json:"months"`
	From       string           `json:"from" format:"date" doc:"First day of the first full month in the basket window"`
	To         string           `json:"to" format:"date" doc:"Last day of the last full month in the basket window"`
	Currencies []CurrencyBasket `json:"currencies"`
}

type CurrencyBasket struct {
	Currency        string           `json:"currency"`
	Products        []BasketProduct  `json:"products"`
	PaidMinor       int64            `json:"paid_minor" doc:"Average monthly spend on the basket products"`
	Stores          []BasketStore    `json:"stores"`
	BestSingleStore *BasketBestStore `json:"best_single_store,omitempty" doc:"Cheapest store among those pricing every product"`
	Split           BasketSplit      `json:"split"`
}

type BasketProduct struct {
	ProductID         uuid.UUID `json:"product_id"`
	Name              string    `json:"name"`
	CompareUnit       string    `json:"compare_unit" enum:"kg,l,unit"`
	DisplayMultiplier int32     `json:"display_multiplier"`
	MonthlyQuantity   float64   `json:"monthly_quantity" doc:"Average bought amount per month, in compare units"`
	PaidMinor         int64     `json:"paid_minor" doc:"Average monthly spend on the product"`
}

type BasketMissing struct {
	ProductID uuid.UUID `json:"product_id"`
	Name      string    `json:"name"`
}

type BasketStore struct {
	StoreID         uuid.UUID       `json:"store_id"`
	Name            string          `json:"name"`
	CostMinor       int64           `json:"cost_minor" doc:"Monthly cost of the priced products using the store's cheapest variant of each"`
	Complete        bool            `json:"complete" doc:"True when every basket product has a price at this store; otherwise cost_minor is not comparable"`
	PricedCount     int             `json:"priced_count"`
	MissingProducts []BasketMissing `json:"missing_products"`
}

type BasketBestStore struct {
	StoreID   uuid.UUID `json:"store_id"`
	Name      string    `json:"name"`
	CostMinor int64     `json:"cost_minor"`
}

type BasketSplit struct {
	CostMinor        int64             `json:"cost_minor" doc:"Monthly cost buying each priced product at its cheapest store"`
	Lines            []BasketSplitLine `json:"lines"`
	UnpricedProducts []BasketMissing   `json:"unpriced_products" doc:"Products without a price at any store, left out of the split"`
}

type BasketSplitLine struct {
	ProductID      uuid.UUID `json:"product_id"`
	Name           string    `json:"name"`
	StoreID        uuid.UUID `json:"store_id"`
	StoreName      string    `json:"store_name"`
	UnitPriceMinor float64   `json:"unit_price_minor"`
	CostMinor      int64     `json:"cost_minor"`
}

// Basket values what the household buys in a typical month. The basket is
// the average monthly quantity of every product over the last `months` full
// calendar months (the current month is excluded). A store is priced with
// its cheapest variant of each product, using the latest observation of the
// last 180 days; stores missing some products are flagged and never chosen as
// best single store. The split buys each product at its cheapest store.
func (s *Service) Basket(ctx context.Context, months int, currency string) (Basket, error) {
	if months == 0 {
		months = 3
	}
	if months < 1 || months > maxBasketMonths {
		return Basket{}, Invalid("months", "must be between 1 and 12")
	}
	cur, err := currencyFilter(currency)
	if err != nil {
		return Basket{}, err
	}
	end := monthStart(s.insightsToday())
	start := end.AddDate(0, -months, 0)
	usage, err := s.q.GroceryBasketUsage(ctx, store.GroceryBasketUsageParams{FromDate: start, ToDate: end, Currency: cur})
	if err != nil {
		return Basket{}, err
	}
	prices, _, err := s.latestObservations(ctx, cur, nil, true)
	if err != nil {
		return Basket{}, err
	}
	out := Basket{Months: months, From: formatInsightDate(start), To: formatInsightDate(end.AddDate(0, 0, -1)), Currencies: []CurrencyBasket{}}

	usageByCurrency := map[string][]store.GroceryBasketUsageRow{}
	for _, u := range usage {
		usageByCurrency[u.Currency] = append(usageByCurrency[u.Currency], u)
	}
	pricesByCurrency := groupByCurrency(prices)
	n := float64(months)
	for _, code := range sortedKeys(usageByCurrency) {
		cb := CurrencyBasket{Currency: code, Products: []BasketProduct{}, Stores: []BasketStore{}}
		qty := map[uuid.UUID]float64{}
		var paidTotal int64
		for _, u := range usageByCurrency[code] {
			qty[u.ProductID] = u.Quantity / n
			paidTotal += u.PaidMinor
			cb.Products = append(cb.Products, BasketProduct{
				ProductID: u.ProductID, Name: u.ProductName, CompareUnit: u.CompareUnit, DisplayMultiplier: u.DisplayMultiplier,
				MonthlyQuantity: round3(u.Quantity / n), PaidMinor: roundMinor(float64(u.PaidMinor) / n),
			})
		}
		cb.PaidMinor = roundMinor(float64(paidTotal) / n)

		// cheapest[product][store] = row of the cheapest variant at the store.
		cheapest := map[uuid.UUID]map[uuid.UUID]store.GroceryLatestUnitPricesRow{}
		var priced []store.GroceryLatestUnitPricesRow
		for _, r := range pricesByCurrency[code] {
			if _, inBasket := qty[r.ProductID]; !inBasket {
				continue
			}
			priced = append(priced, r)
			m, ok := cheapest[r.ProductID]
			if !ok {
				m = map[uuid.UUID]store.GroceryLatestUnitPricesRow{}
				cheapest[r.ProductID] = m
			}
			if cur, ok := m[r.StoreID]; !ok || r.UnitPriceMinor < cur.UnitPriceMinor {
				m[r.StoreID] = r
			}
		}

		storeCost := map[uuid.UUID]float64{}
		for _, st := range storesOf(priced) {
			bs := BasketStore{StoreID: st.ID, Name: st.Name, MissingProducts: []BasketMissing{}}
			var cost float64
			for _, p := range cb.Products {
				if r, ok := cheapest[p.ProductID][st.ID]; ok {
					cost += r.UnitPriceMinor * qty[p.ProductID]
					bs.PricedCount++
				} else {
					bs.MissingProducts = append(bs.MissingProducts, BasketMissing{ProductID: p.ProductID, Name: p.Name})
				}
			}
			bs.CostMinor = roundMinor(cost)
			bs.Complete = len(bs.MissingProducts) == 0
			storeCost[st.ID] = cost
			cb.Stores = append(cb.Stores, bs)
			if bs.Complete && (cb.BestSingleStore == nil || cost < storeCost[cb.BestSingleStore.StoreID]) {
				cb.BestSingleStore = &BasketBestStore{StoreID: st.ID, Name: st.Name, CostMinor: bs.CostMinor}
			}
		}

		cb.Split = BasketSplit{Lines: []BasketSplitLine{}, UnpricedProducts: []BasketMissing{}}
		var splitCost float64
		storeOrder := storesOf(priced)
		for _, p := range cb.Products {
			var best *store.GroceryLatestUnitPricesRow
			for _, st := range storeOrder { // ties go to the first store by name
				if r, ok := cheapest[p.ProductID][st.ID]; ok && (best == nil || r.UnitPriceMinor < best.UnitPriceMinor) {
					best = &r
				}
			}
			if best == nil {
				cb.Split.UnpricedProducts = append(cb.Split.UnpricedProducts, BasketMissing{ProductID: p.ProductID, Name: p.Name})
				continue
			}
			cost := best.UnitPriceMinor * qty[p.ProductID]
			splitCost += cost
			cb.Split.Lines = append(cb.Split.Lines, BasketSplitLine{
				ProductID: p.ProductID, Name: p.Name, StoreID: best.StoreID, StoreName: best.StoreName,
				UnitPriceMinor: best.UnitPriceMinor, CostMinor: roundMinor(cost),
			})
		}
		cb.Split.CostMinor = roundMinor(splitCost)
		out.Currencies = append(out.Currencies, cb)
	}
	return out, nil
}
