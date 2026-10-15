package groceries

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

const maxShoppingListItems = 100

// ShoppingListItem is one product of a list with the amount to buy in the
// product's compare unit (kg, l or unit).
type ShoppingListItem struct {
	Product  string
	Quantity *float64
}

// ShoppingListLine is a known product of the list.
type ShoppingListLine struct {
	ProductID   uuid.UUID `json:"product_id"`
	Name        string    `json:"name"`
	CompareUnit string    `json:"compare_unit" enum:"kg,l,unit"`
	Quantity    float64   `json:"quantity" doc:"Amount in compare units"`
}

// UnknownListItem is a requested name that matches no product.
type UnknownListItem struct {
	Product     string   `json:"product"`
	Suggestions []string `json:"suggestions" doc:"Existing products with a similar name"`
}

type ShoppingListSplitLine struct {
	ProductID      uuid.UUID `json:"product_id"`
	Name           string    `json:"name"`
	StoreID        uuid.UUID `json:"store_id"`
	StoreName      string    `json:"store_name"`
	Quantity       float64   `json:"quantity"`
	UnitPriceMinor float64   `json:"unit_price_minor"`
	CostMinor      int64     `json:"cost_minor"`
}

type ShoppingListSplit struct {
	CostMinor int64                   `json:"cost_minor"`
	Lines     []ShoppingListSplitLine `json:"lines"`
}

type CurrencyShoppingList struct {
	Currency        string            `json:"currency"`
	Stores          []BasketStore     `json:"stores" doc:"Cost of the whole list per store using its cheapest variant of each product"`
	BestSingleStore *BasketBestStore  `json:"best_single_store,omitempty" doc:"Cheapest store pricing every product"`
	Split           ShoppingListSplit `json:"split" doc:"Each priced product bought at its cheapest store"`
}

type ShoppingList struct {
	Items      []ShoppingListLine     `json:"items"`
	Unknown    []UnknownListItem      `json:"unknown"`
	Unpriced   []BasketMissing        `json:"unpriced" doc:"Known products without any recent price"`
	Currencies []CurrencyShoppingList `json:"currencies"`
}

// PriceShoppingList prices a list of products at every store, using the
// latest observation of the last 180 days. Names that match no product are
// returned with similar existing names.
func (s *Service) PriceShoppingList(ctx context.Context, items []ShoppingListItem, currency string) (ShoppingList, error) {
	if len(items) == 0 {
		return ShoppingList{}, Invalid("items", "must contain at least one item")
	}
	if len(items) > maxShoppingListItems {
		return ShoppingList{}, Invalid("items", "must contain at most 100 items")
	}
	cur, err := currencyFilter(currency)
	if err != nil {
		return ShoppingList{}, err
	}
	products, err := s.ListProducts(ctx, ListProductsParams{})
	if err != nil {
		return ShoppingList{}, err
	}
	out := ShoppingList{Items: []ShoppingListLine{}, Unknown: []UnknownListItem{}, Unpriced: []BasketMissing{}, Currencies: []CurrencyShoppingList{}}
	qty := map[uuid.UUID]float64{}
	for _, it := range items {
		name := strings.TrimSpace(it.Product)
		if name == "" {
			return ShoppingList{}, Invalid("items", "product is required")
		}
		q := 1.0
		if it.Quantity != nil {
			q = *it.Quantity
		}
		if q <= 0 || q > 999999999 {
			return ShoppingList{}, Invalid("items", "quantity must be greater than 0")
		}
		idx := slices.IndexFunc(products, func(p Product) bool { return strings.EqualFold(p.Name, name) })
		if idx < 0 {
			out.Unknown = append(out.Unknown, UnknownListItem{Product: name, Suggestions: suggestProducts(products, name)})
			continue
		}
		p := products[idx]
		if _, seen := qty[p.ID]; !seen {
			out.Items = append(out.Items, ShoppingListLine{ProductID: p.ID, Name: p.Name, CompareUnit: p.CompareUnit})
		}
		qty[p.ID] += q
	}
	for i := range out.Items {
		out.Items[i].Quantity = round3(qty[out.Items[i].ProductID])
	}
	if len(out.Items) == 0 {
		return out, nil
	}

	prices, _, err := s.latestObservations(ctx, cur, nil, true)
	if err != nil {
		return ShoppingList{}, err
	}
	pricedAnywhere := map[uuid.UUID]bool{}
	byCurrency := groupByCurrency(prices)
	for _, code := range sortedKeys(byCurrency) {
		cheapest := map[uuid.UUID]map[uuid.UUID]store.GroceryLatestUnitPricesRow{}
		var priced []store.GroceryLatestUnitPricesRow
		for _, r := range byCurrency[code] {
			if _, ok := qty[r.ProductID]; !ok {
				continue
			}
			priced = append(priced, r)
			pricedAnywhere[r.ProductID] = true
			m, ok := cheapest[r.ProductID]
			if !ok {
				m = map[uuid.UUID]store.GroceryLatestUnitPricesRow{}
				cheapest[r.ProductID] = m
			}
			if prev, ok := m[r.StoreID]; !ok || r.UnitPriceMinor < prev.UnitPriceMinor {
				m[r.StoreID] = r
			}
		}
		if len(priced) == 0 {
			continue
		}
		cl := CurrencyShoppingList{Currency: code, Stores: []BasketStore{}, Split: ShoppingListSplit{Lines: []ShoppingListSplitLine{}}}
		storeOrder := storesOf(priced)
		var bestCost float64
		for _, st := range storeOrder {
			bs := BasketStore{StoreID: st.ID, Name: st.Name, MissingProducts: []BasketMissing{}}
			var cost float64
			for _, it := range out.Items {
				if r, ok := cheapest[it.ProductID][st.ID]; ok {
					cost += r.UnitPriceMinor * qty[it.ProductID]
					bs.PricedCount++
				} else {
					bs.MissingProducts = append(bs.MissingProducts, BasketMissing{ProductID: it.ProductID, Name: it.Name})
				}
			}
			bs.CostMinor = roundMinor(cost)
			bs.Complete = len(bs.MissingProducts) == 0
			cl.Stores = append(cl.Stores, bs)
			if bs.Complete && (cl.BestSingleStore == nil || cost < bestCost) {
				bestCost = cost
				cl.BestSingleStore = &BasketBestStore{StoreID: st.ID, Name: st.Name, CostMinor: bs.CostMinor}
			}
		}
		var splitCost float64
		for _, it := range out.Items {
			var best *store.GroceryLatestUnitPricesRow
			for _, st := range storeOrder { // ties go to the first store by name
				if r, ok := cheapest[it.ProductID][st.ID]; ok && (best == nil || r.UnitPriceMinor < best.UnitPriceMinor) {
					best = &r
				}
			}
			if best == nil {
				continue
			}
			cost := best.UnitPriceMinor * qty[it.ProductID]
			splitCost += cost
			cl.Split.Lines = append(cl.Split.Lines, ShoppingListSplitLine{
				ProductID: it.ProductID, Name: it.Name, StoreID: best.StoreID, StoreName: best.StoreName,
				Quantity: it.Quantity, UnitPriceMinor: best.UnitPriceMinor, CostMinor: roundMinor(cost),
			})
		}
		cl.Split.CostMinor = roundMinor(splitCost)
		out.Currencies = append(out.Currencies, cl)
	}
	for _, it := range out.Items {
		if !pricedAnywhere[it.ProductID] {
			out.Unpriced = append(out.Unpriced, BasketMissing{ProductID: it.ProductID, Name: it.Name})
		}
	}
	return out, nil
}

// suggestProducts returns up to five product names that contain, or are
// contained in, the requested name, or share a word of at least 3 letters.
func suggestProducts(products []Product, name string) []string {
	needle := strings.ToLower(name)
	words := strings.Fields(needle)
	out := []string{}
	for _, p := range products {
		lower := strings.ToLower(p.Name)
		match := strings.Contains(lower, needle) || strings.Contains(needle, lower)
		for _, w := range words {
			if len([]rune(w)) >= 3 && strings.Contains(lower, w) {
				match = true
			}
		}
		if match {
			out = append(out, p.Name)
			if len(out) == 5 {
				break
			}
		}
	}
	return out
}
