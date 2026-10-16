package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/groceries"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/money"
)

// --- Tool inputs and outputs ---

type addPriceCheckArgs struct {
	Store       string   `json:"store" jsonschema:"The store chain (not the branch), e.g. Mercadona. Use an existing name from list_grocery_catalog"`
	NewStore    bool     `json:"new_store,omitempty" jsonschema:"Set to true only after the user confirms this is a store that does not exist yet"`
	Product     string   `json:"product" jsonschema:"What the product is, without brand or pack, e.g. Whole milk. Reuse an existing product name from list_grocery_catalog"`
	Brand       string   `json:"brand,omitempty" jsonschema:"Brand, e.g. Hacendado. Defaults to Generic for unbranded products"`
	PackSize    *float64 `json:"pack_size,omitempty" jsonschema:"Contents of the pack in the product's compare_unit (kg, l or unit), e.g. 1 for a 1 l bottle or 0.5 for 500 g. Omit when sold loose (price per compare unit)"`
	PackLabel   string   `json:"pack_label,omitempty" jsonschema:"Pack description, e.g. 6x1 l or 4x125 g"`
	Price       float64  `json:"price" jsonschema:"Shelf price of the whole pack as a decimal, e.g. 1.15"`
	Currency    string   `json:"currency,omitempty" jsonschema:"ISO 4217 code. Defaults to the currency of the latest receipt; required when there are no receipts"`
	Date        string   `json:"date,omitempty" jsonschema:"Date the price was seen, YYYY-MM-DD. Defaults to today"`
	CompareUnit string   `json:"compare_unit,omitempty" jsonschema:"kg, l or unit. Required only when the product is new"`
	Group       string   `json:"group,omitempty" jsonschema:"Product group name from list_grocery_catalog (e.g. Produce, Other). Required when the product is new; for an existing product without a group it sets it, and a different group is reported but not applied (use update_grocery_product)"`
}

type priceCheckOut struct {
	ID        string `json:"id"`
	Store     string `json:"store"`
	Product   string `json:"product"`
	Brand     string `json:"brand"`
	PackLabel string `json:"pack_label,omitempty"`
	Date      string `json:"date"`
	Price     string `json:"price" jsonschema:"Price of the whole pack"`
	Currency  string `json:"currency"`
	UnitPrice string `json:"unit_price" jsonschema:"Decimal price per unit, e.g. 1.150"`
	Unit      string `json:"unit" jsonschema:"Unit of unit_price, e.g. EUR/l"`
}

type breakdownArgs struct {
	Month    string `json:"month,omitempty" jsonschema:"Month in YYYY-MM format. Defaults to the current month"`
	Currency string `json:"currency,omitempty" jsonschema:"Only this ISO 4217 currency. Defaults to all"`
}

type breakdownGroupOut struct {
	Name     string  `json:"name" jsonschema:"Group name, or Ungrouped for lines without a product or group"`
	IsFood   *bool   `json:"is_food,omitempty"`
	Total    string  `json:"total"`
	SharePct float64 `json:"share_pct"`
}

type breakdownStoreOut struct {
	Store        string `json:"store"`
	Total        string `json:"total"`
	ReceiptCount int64  `json:"receipt_count"`
}

type breakdownProductOut struct {
	Product  string  `json:"product"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
	Spend    string  `json:"spend"`
}

type breakdownCurrencyOut struct {
	Currency          string                `json:"currency"`
	Total             string                `json:"total" jsonschema:"Sum of the receipt totals of the month"`
	ReceiptCount      int64                 `json:"receipt_count"`
	AverageReceipt    string                `json:"average_receipt"`
	CoveragePct       float64               `json:"coverage_pct" jsonschema:"Share of receipt lines already assigned to a product; the group and product figures only cover those"`
	Unassigned        string                `json:"unassigned" jsonschema:"Lines without a product yet"`
	Food              string                `json:"food"`
	NonFood           string                `json:"non_food"`
	GroceriesCategory string                `json:"groceries_category,omitempty" jsonschema:"Expenses of the Groceries category in the month, to compare with the receipts"`
	ByGroup           []breakdownGroupOut   `json:"by_group"`
	ByStore           []breakdownStoreOut   `json:"by_store"`
	TopProducts       []breakdownProductOut `json:"top_products"`
}

type breakdownOut struct {
	Month      string                 `json:"month"`
	Currencies []breakdownCurrencyOut `json:"currencies"`
}

type compareStoresArgs struct {
	Products []string `json:"products,omitempty" jsonschema:"Only these products (case-insensitive names). Defaults to every product with recent prices"`
	Level    string   `json:"level,omitempty" jsonschema:"product (default: cheapest variant per store) or brand (same brand across stores, store brands excluded)"`
	Currency string   `json:"currency,omitempty" jsonschema:"Only this ISO 4217 currency. Defaults to all"`
}

type storePriceOut struct {
	Store      string `json:"store"`
	Brand      string `json:"brand"`
	PackLabel  string `json:"pack_label,omitempty"`
	UnitPrice  string `json:"unit_price" jsonschema:"Decimal price per unit of the row, e.g. 1.150"`
	Unit       string `json:"unit" jsonschema:"Unit of unit_price, e.g. EUR/l"`
	ObservedOn string `json:"observed_on"`
	Source     string `json:"source" jsonschema:"receipt or price_check"`
	Stale      bool   `json:"stale,omitempty" jsonschema:"Seen more than 60 days ago"`
}

type compareRowOut struct {
	Product       string          `json:"product"`
	Brand         string          `json:"brand,omitempty"`
	Prices        []storePriceOut `json:"prices" jsonschema:"One per store with a price, cheapest first"`
	CheapestStore string          `json:"cheapest_store,omitempty" jsonschema:"Omitted when only one store has a price"`
	MaxDiffPct    *float64        `json:"max_diff_pct,omitempty" jsonschema:"How much more the most expensive store charges than the cheapest, in percent"`
}

type compareCurrencyOut struct {
	Currency string          `json:"currency"`
	Stores   []string        `json:"stores"`
	Rows     []compareRowOut `json:"rows"`
}

type compareOut struct {
	Level           string               `json:"level"`
	Currencies      []compareCurrencyOut `json:"currencies"`
	UnknownProducts []string             `json:"unknown_products,omitempty" jsonschema:"Requested names that match no product"`
}

type shoppingItemArgs struct {
	Product  string   `json:"product" jsonschema:"Product name as in list_grocery_catalog, e.g. Whole milk"`
	Quantity *float64 `json:"quantity,omitempty" jsonschema:"Amount in the product's compare_unit (kg, l or unit), e.g. 6 for six litres. Defaults to 1"`
}

type priceShoppingListArgs struct {
	Items    []shoppingItemArgs `json:"items" jsonschema:"The shopping list (up to 100 items)"`
	Currency string             `json:"currency,omitempty" jsonschema:"Only this ISO 4217 currency. Defaults to all currencies with prices"`
}

type shoppingItemOut struct {
	Product  string  `json:"product"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit" jsonschema:"kg, l or unit"`
}

type shoppingStoreOut struct {
	Store    string   `json:"store"`
	Cost     string   `json:"cost" jsonschema:"Cost of the priced products at this store; only comparable when complete"`
	Complete bool     `json:"complete" jsonschema:"True when this store has a price for every known product on the list"`
	Missing  []string `json:"missing,omitempty" jsonschema:"Products without a price at this store"`
}

type shoppingSplitLineOut struct {
	Product   string  `json:"product"`
	Store     string  `json:"store"`
	Quantity  float64 `json:"quantity"`
	UnitPrice string  `json:"unit_price"`
	Unit      string  `json:"unit"`
	Cost      string  `json:"cost"`
}

type shoppingCurrencyOut struct {
	Currency        string                 `json:"currency"`
	Stores          []shoppingStoreOut     `json:"stores" jsonschema:"Cheapest first among complete stores"`
	BestSingleStore string                 `json:"best_single_store,omitempty" jsonschema:"Cheapest store that has a price for every product"`
	BestSingleCost  string                 `json:"best_single_cost,omitempty"`
	Split           []shoppingSplitLineOut `json:"split" jsonschema:"Buying each product at its cheapest store"`
	SplitCost       string                 `json:"split_cost"`
}

type unknownItemOut struct {
	Product     string   `json:"product"`
	Suggestions []string `json:"suggestions" jsonschema:"Existing products with a similar name; ask the user which one they mean"`
}

type shoppingListOut struct {
	Items      []shoppingItemOut     `json:"items" jsonschema:"The known products of the list, with repeated products added up"`
	Unknown    []unknownItemOut      `json:"unknown,omitempty" jsonschema:"Names that match no product; they are not priced"`
	Unpriced   []string              `json:"unpriced,omitempty" jsonschema:"Known products with no price in the last 180 days"`
	Currencies []shoppingCurrencyOut `json:"currencies"`
}

func (s *Server) registerGroceryInsightsTools() {
	if s.groc == nil {
		return
	}
	addTool(s, &mcp.Tool{
		Name:  "add_price_check",
		Title: "Add price check",
		Description: "Record a shelf price seen in a store without buying (e.g. from the phone while shopping). " +
			"The product, brand and pack are matched by name and created when new; it then counts in the store comparison.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args addPriceCheckArgs) (*mcp.CallToolResult, priceCheckOut, error) {
		return s.addPriceCheck(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "get_grocery_breakdown",
		Title:       "Get grocery breakdown",
		Description: "Grocery spending of a month from receipts, per currency: total, receipts, food vs non-food, by product group, by store and top products. Group and product figures cover only lines already assigned to a product (see coverage_pct).",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args breakdownArgs) (*mcp.CallToolResult, breakdownOut, error) {
		return s.groceryBreakdown(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:  "compare_stores",
		Title: "Compare store prices",
		Description: "Compare unit prices (per kg, l or unit) of products across stores using the latest price seen in the last 180 days, from receipts and price checks. " +
			"Answers where a product is cheapest. Prices are decimals with their unit, e.g. 1.150 EUR/l.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args compareStoresArgs) (*mcp.CallToolResult, compareOut, error) {
		return s.compareStores(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:  "price_shopping_list",
		Title: "Price shopping list",
		Description: "Price a shopping list at every store: the cost of the whole list per store, the cheapest single store and the cost of buying each product where it is cheapest. " +
			"Uses the latest prices of the last 180 days. Names that match no product are listed back with similar existing products.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args priceShoppingListArgs) (*mcp.CallToolResult, shoppingListOut, error) {
		return s.priceShoppingList(ctx, args)
	})
}

// --- Helpers ---

// unitPrice formats a unit price given in minor units as a decimal with one
// more digit than the currency, e.g. 1.150 for EUR.
func unitPrice(minor float64, minorUnits int) string {
	return strconv.FormatFloat(minor/math.Pow10(minorUnits), 'f', minorUnits+1, 64)
}

// unitLabel is the unit of a unit price: EUR/l, or EUR/12 unit when the
// product is priced per dozen.
func unitLabel(currency, compareUnit string, multiplier int32) string {
	if multiplier > 1 {
		return fmt.Sprintf("%s/%d %s", currency, multiplier, compareUnit)
	}
	return currency + "/" + compareUnit
}

// currencyArg validates an optional currency argument.
func (s *Server) currencyArg(ctx context.Context, raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	cur, err := s.findCurrency(ctx, raw)
	if err != nil {
		return "", err
	}
	return cur.Code, nil
}

type minorUnitsCache struct {
	s     *Server
	ctx   context.Context
	cache map[string]int
}

func (s *Server) newMinorUnitsCache(ctx context.Context) *minorUnitsCache {
	return &minorUnitsCache{s: s, ctx: ctx, cache: map[string]int{}}
}

func (c *minorUnitsCache) get(code string) int {
	if mu, ok := c.cache[code]; ok {
		return mu
	}
	mu := c.s.minorUnits(c.ctx, code)
	c.cache[code] = mu
	return mu
}

// --- add_price_check ---

func (s *Server) addPriceCheck(ctx context.Context, args addPriceCheckArgs) (*mcp.CallToolResult, priceCheckOut, error) {
	if strings.TrimSpace(args.Product) == "" {
		return nil, priceCheckOut{}, errors.New("product is required: name what the product is (see list_grocery_catalog for existing ones)")
	}
	storeName, err := s.resolveStore(ctx, args.Store, args.NewStore)
	if err != nil {
		return nil, priceCheckOut{}, err
	}

	currency := strings.ToUpper(strings.TrimSpace(args.Currency))
	if currency == "" {
		receipts, err := s.groc.ListReceipts(ctx, groceries.ListReceiptsParams{})
		if err != nil {
			return nil, priceCheckOut{}, groceriesErr(err)
		}
		if len(receipts) == 0 {
			return nil, priceCheckOut{}, errors.New("currency is required (ISO 4217 code, e.g. EUR) because there are no receipts to take it from")
		}
		currency = receipts[0].Currency
	}
	cur, err := s.findCurrency(ctx, currency)
	if err != nil {
		return nil, priceCheckOut{}, err
	}
	price, err := toMinor(args.Price, cur.MinorUnits, cur.Code, "price")
	if err != nil {
		return nil, priceCheckOut{}, err
	}
	if d := strings.TrimSpace(args.Date); d != "" {
		if _, err := time.Parse(time.DateOnly, d); err != nil {
			return nil, priceCheckOut{}, fmt.Errorf("date %q is not valid; use YYYY-MM-DD, e.g. 2026-10-10", d)
		}
	}

	planner, err := s.newGroupPlanner(ctx)
	if err != nil {
		return nil, priceCheckOut{}, err
	}
	if err := planner.plan("product", args.Product, args.Group); err != nil {
		return nil, priceCheckOut{}, err
	}
	variantID, err := s.groc.EnsureVariant(ctx, groceries.VariantRef{
		Product: args.Product, Brand: args.Brand, PackSize: args.PackSize, PackLabel: args.PackLabel,
		CompareUnit: args.CompareUnit, Group: args.Group,
	})
	if err != nil {
		return nil, priceCheckOut{}, s.assignError(ctx, err)
	}
	stores, err := s.groc.ListStores(ctx, false)
	if err != nil {
		return nil, priceCheckOut{}, groceriesErr(err)
	}
	storeID := ""
	for _, st := range stores {
		if strings.EqualFold(st.Name, storeName) {
			storeID = st.ID.String()
		}
	}
	if storeID == "" { // confirmed new store
		created, err := s.groc.CreateStore(ctx, groceries.CreateStoreInput{Name: storeName})
		if err != nil {
			return nil, priceCheckOut{}, groceriesErr(err)
		}
		storeID = created.ID.String()
	}

	pc, err := s.groc.CreatePriceCheck(ctx, groceries.CreatePriceCheckInput{
		StoreID: storeID, VariantID: variantID.String(), ObservedOn: args.Date, PriceMinor: price, Currency: cur.Code,
	})
	if err != nil {
		return nil, priceCheckOut{}, groceriesErr(err)
	}
	out := priceCheckOut{
		ID: pc.ID.String(), Store: pc.StoreName, Product: pc.ProductName, Brand: pc.Brand, Date: pc.ObservedOn,
		Price: money.Format(pc.PriceMinor, cur.MinorUnits), Currency: pc.Currency,
		UnitPrice: unitPrice(pc.DisplayUnitPrice, cur.MinorUnits),
	}
	if pc.PackLabel != nil {
		out.PackLabel = *pc.PackLabel
	}
	out.Unit = unitLabel(pc.Currency, pc.CompareUnit, displayMultiplier(pc))
	if err := s.applyGroups(ctx, planner); err != nil {
		return nil, priceCheckOut{}, fmt.Errorf("the price was recorded but the group could not be set: %w; use update_grocery_product", err)
	}
	msg := fmt.Sprintf("Recorded %s %s for %s %s at %s on %s: %s %s.", out.Price, out.Currency, out.Product, out.Brand, out.Store, out.Date, out.UnitPrice, out.Unit)
	for _, n := range planner.notes {
		msg += "\n" + n
	}
	return text(msg), out, nil
}

// displayMultiplier recovers the multiplier of a price check from its two
// unit prices.
func displayMultiplier(pc groceries.PriceCheck) int32 {
	if pc.UnitPriceMinor <= 0 {
		return 1
	}
	return int32(math.Round(pc.DisplayUnitPrice / pc.UnitPriceMinor))
}

// --- get_grocery_breakdown ---

func (s *Server) groceryBreakdown(ctx context.Context, args breakdownArgs) (*mcp.CallToolResult, breakdownOut, error) {
	month := strings.TrimSpace(args.Month)
	if month != "" {
		if _, err := time.Parse("2006-01", month); err != nil {
			return nil, breakdownOut{}, fmt.Errorf("month %q is not valid; use YYYY-MM, e.g. 2026-10", month)
		}
	}
	currency, err := s.currencyArg(ctx, args.Currency)
	if err != nil {
		return nil, breakdownOut{}, err
	}
	b, err := s.groc.Breakdown(ctx, month, currency)
	if err != nil {
		return nil, breakdownOut{}, groceriesErr(err)
	}
	units := s.newMinorUnitsCache(ctx)
	out := breakdownOut{Month: b.Month, Currencies: []breakdownCurrencyOut{}}
	for _, c := range b.Currencies {
		mu := units.get(c.Currency)
		co := breakdownCurrencyOut{
			Currency: c.Currency, Total: money.Format(c.TotalMinor, mu), ReceiptCount: c.ReceiptCount,
			AverageReceipt: money.Format(c.AverageReceiptMinor, mu), CoveragePct: c.CoveragePct,
			Unassigned: money.Format(c.UnassignedMinor, mu), Food: money.Format(c.FoodMinor, mu), NonFood: money.Format(c.NonFoodMinor, mu),
			ByGroup: []breakdownGroupOut{}, ByStore: []breakdownStoreOut{}, TopProducts: []breakdownProductOut{},
		}
		if c.GroceriesCategoryMinor != nil {
			co.GroceriesCategory = money.Format(*c.GroceriesCategoryMinor, mu)
		}
		for _, g := range c.ByGroup {
			name := "Ungrouped"
			if g.Name != nil {
				name = *g.Name
			}
			co.ByGroup = append(co.ByGroup, breakdownGroupOut{Name: name, IsFood: g.IsFood, Total: money.Format(g.TotalMinor, mu), SharePct: g.SharePct})
		}
		for _, st := range c.ByStore {
			co.ByStore = append(co.ByStore, breakdownStoreOut{Store: st.Name, Total: money.Format(st.TotalMinor, mu), ReceiptCount: st.ReceiptCount})
		}
		for _, p := range c.TopProducts {
			co.TopProducts = append(co.TopProducts, breakdownProductOut{Product: p.Name, Quantity: p.Quantity, Unit: p.CompareUnit, Spend: money.Format(p.SpendMinor, mu)})
		}
		out.Currencies = append(out.Currencies, co)
	}
	if len(out.Currencies) == 0 {
		return text(fmt.Sprintf("No grocery receipts in %s.", b.Month)), out, nil
	}
	msg := fmt.Sprintf("Grocery spending in %s:", b.Month)
	for _, c := range out.Currencies {
		msg += fmt.Sprintf(" %s %s in %d receipts (%.0f%% of lines assigned to products);", c.Total, c.Currency, c.ReceiptCount, c.CoveragePct)
	}
	return text(strings.TrimSuffix(msg, ";") + "."), out, nil
}

// --- compare_stores ---

func (s *Server) compareStores(ctx context.Context, args compareStoresArgs) (*mcp.CallToolResult, compareOut, error) {
	level := strings.ToLower(strings.TrimSpace(args.Level))
	if level != "" && level != groceries.LevelProduct && level != groceries.LevelBrand {
		return nil, compareOut{}, errors.New("level must be product or brand")
	}
	currency, err := s.currencyArg(ctx, args.Currency)
	if err != nil {
		return nil, compareOut{}, err
	}
	cmp, err := s.groc.Compare(ctx, level, currency)
	if err != nil {
		return nil, compareOut{}, groceriesErr(err)
	}

	wanted := map[string]bool{}
	for _, p := range args.Products {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
			wanted[p] = true
		}
	}
	out := compareOut{Level: cmp.Level, Currencies: []compareCurrencyOut{}}
	found := map[string]bool{}
	for _, c := range cmp.Currencies {
		names := make(map[string]string, len(c.Stores))
		co := compareCurrencyOut{Currency: c.Currency, Stores: []string{}, Rows: []compareRowOut{}}
		for _, st := range c.Stores {
			names[st.ID.String()] = st.Name
			co.Stores = append(co.Stores, st.Name)
		}
		for _, r := range c.Rows {
			key := strings.ToLower(r.ProductName)
			if len(wanted) > 0 && !wanted[key] {
				continue
			}
			found[key] = true
			ro := compareRowOut{Product: r.ProductName, Prices: []storePriceOut{}, MaxDiffPct: r.MaxDiffPct}
			if r.Brand != nil {
				ro.Brand = *r.Brand
			}
			mu := s.minorUnits(ctx, c.Currency)
			unit := unitLabel(c.Currency, r.CompareUnit, r.DisplayMultiplier)
			for _, st := range c.Stores { // columns are sorted by name; sorted by price below
				cell, ok := r.Cells[st.ID.String()]
				if !ok {
					continue
				}
				po := storePriceOut{
					Store: st.Name, Brand: cell.Brand, UnitPrice: unitPrice(cell.DisplayUnitPrice, mu), Unit: unit,
					ObservedOn: cell.ObservedOn, Source: cell.Source, Stale: cell.Stale,
				}
				if cell.PackLabel != nil {
					po.PackLabel = *cell.PackLabel
				}
				ro.Prices = append(ro.Prices, po)
			}
			sortPricesByUnitPrice(ro.Prices, r.Cells, c.Stores)
			if r.BestStoreID != nil {
				ro.CheapestStore = names[r.BestStoreID.String()]
			}
			co.Rows = append(co.Rows, ro)
		}
		if len(co.Rows) > 0 {
			out.Currencies = append(out.Currencies, co)
		}
	}
	for _, p := range args.Products {
		if k := strings.ToLower(strings.TrimSpace(p)); k != "" && !found[k] {
			out.UnknownProducts = append(out.UnknownProducts, strings.TrimSpace(p))
		}
	}

	switch {
	case len(out.Currencies) == 0 && len(out.UnknownProducts) > 0:
		return text(fmt.Sprintf("No prices for %s. Check the exact product names with list_grocery_catalog; products need a receipt or add_price_check in the last 180 days.", strings.Join(out.UnknownProducts, ", "))), out, nil
	case len(out.Currencies) == 0:
		return text("No prices to compare yet: record receipts with assigned products or use add_price_check."), out, nil
	}
	msg := "Compared products in " + strconv.Itoa(len(out.Currencies)) + " currencies."
	if len(out.Currencies) == 1 {
		msg = fmt.Sprintf("Compared %d products across %d stores in %s.", len(out.Currencies[0].Rows), len(out.Currencies[0].Stores), out.Currencies[0].Currency)
	}
	if len(out.UnknownProducts) > 0 {
		msg += " No prices for: " + strings.Join(out.UnknownProducts, ", ") + "."
	}
	return text(msg), out, nil
}

// sortPricesByUnitPrice orders the prices of a row from cheapest to most
// expensive, keeping store-name order for ties.
func sortPricesByUnitPrice(prices []storePriceOut, cells map[string]groceries.PriceCell, stores []groceries.StoreRef) {
	byName := make(map[string]float64, len(prices))
	for _, st := range stores {
		if cell, ok := cells[st.ID.String()]; ok {
			byName[st.Name] = cell.UnitPriceMinor
		}
	}
	// Insertion sort: rows have a handful of stores and it is stable.
	for i := 1; i < len(prices); i++ {
		for j := i; j > 0 && byName[prices[j].Store] < byName[prices[j-1].Store]; j-- {
			prices[j], prices[j-1] = prices[j-1], prices[j]
		}
	}
}

// --- price_shopping_list ---

func (s *Server) priceShoppingList(ctx context.Context, args priceShoppingListArgs) (*mcp.CallToolResult, shoppingListOut, error) {
	if len(args.Items) == 0 {
		return nil, shoppingListOut{}, errors.New("items must contain at least one item, e.g. {\"product\": \"Whole milk\", \"quantity\": 6}")
	}
	currency, err := s.currencyArg(ctx, args.Currency)
	if err != nil {
		return nil, shoppingListOut{}, err
	}
	items := make([]groceries.ShoppingListItem, len(args.Items))
	for i, it := range args.Items {
		if strings.TrimSpace(it.Product) == "" {
			return nil, shoppingListOut{}, fmt.Errorf("items[%d].product is required: name a product (see list_grocery_catalog)", i)
		}
		if it.Quantity != nil && *it.Quantity <= 0 {
			return nil, shoppingListOut{}, fmt.Errorf("items[%d].quantity must be greater than zero", i)
		}
		items[i] = groceries.ShoppingListItem{Product: it.Product, Quantity: it.Quantity}
	}
	list, err := s.groc.PriceShoppingList(ctx, items, currency)
	if err != nil {
		return nil, shoppingListOut{}, groceriesErr(err)
	}

	units := s.newMinorUnitsCache(ctx)
	out := shoppingListOut{Items: []shoppingItemOut{}, Currencies: []shoppingCurrencyOut{}}
	byID := map[string]groceries.ShoppingListLine{}
	for _, l := range list.Items {
		byID[l.ProductID.String()] = l
		out.Items = append(out.Items, shoppingItemOut{Product: l.Name, Quantity: l.Quantity, Unit: l.CompareUnit})
	}
	for _, u := range list.Unknown {
		out.Unknown = append(out.Unknown, unknownItemOut{Product: u.Product, Suggestions: u.Suggestions})
	}
	for _, u := range list.Unpriced {
		out.Unpriced = append(out.Unpriced, u.Name)
	}
	for _, c := range list.Currencies {
		mu := units.get(c.Currency)
		co := shoppingCurrencyOut{Currency: c.Currency, Stores: []shoppingStoreOut{}, Split: []shoppingSplitLineOut{}, SplitCost: money.Format(c.Split.CostMinor, mu)}
		for _, st := range c.Stores {
			so := shoppingStoreOut{Store: st.Name, Cost: money.Format(st.CostMinor, mu), Complete: st.Complete}
			for _, m := range st.MissingProducts {
				so.Missing = append(so.Missing, m.Name)
			}
			co.Stores = append(co.Stores, so)
		}
		// Complete stores first, cheapest first.
		for i := 1; i < len(co.Stores); i++ {
			for j := i; j > 0 && storeBefore(c.Stores[j], c.Stores[j-1]); j-- {
				co.Stores[j], co.Stores[j-1] = co.Stores[j-1], co.Stores[j]
				c.Stores[j], c.Stores[j-1] = c.Stores[j-1], c.Stores[j]
			}
		}
		if c.BestSingleStore != nil {
			co.BestSingleStore = c.BestSingleStore.Name
			co.BestSingleCost = money.Format(c.BestSingleStore.CostMinor, mu)
		}
		for _, l := range c.Split.Lines {
			item := byID[l.ProductID.String()]
			// The split line carries the unit price per compare unit.
			co.Split = append(co.Split, shoppingSplitLineOut{
				Product: l.Name, Store: l.StoreName, Quantity: l.Quantity,
				UnitPrice: unitPrice(l.UnitPriceMinor, mu), Unit: c.Currency + "/" + item.CompareUnit, Cost: money.Format(l.CostMinor, mu),
			})
		}
		out.Currencies = append(out.Currencies, co)
	}

	var msg strings.Builder
	fmt.Fprintf(&msg, "Priced %d of %d list items.", len(out.Items), len(args.Items))
	for _, c := range out.Currencies {
		if c.BestSingleStore != "" {
			fmt.Fprintf(&msg, " In %s: cheapest single store %s at %s; buying each product where it is cheapest costs %s.", c.Currency, c.BestSingleStore, c.BestSingleCost, c.SplitCost)
		} else {
			fmt.Fprintf(&msg, " In %s: no store prices every product; buying each where it is cheapest costs %s.", c.Currency, c.SplitCost)
		}
	}
	if len(out.Unknown) > 0 {
		names := make([]string, len(out.Unknown))
		for i, u := range out.Unknown {
			names[i] = u.Product
		}
		fmt.Fprintf(&msg, " Not priced, no such product: %s; ask the user which existing product they mean (see suggestions) or use list_grocery_catalog.", strings.Join(names, ", "))
	}
	if len(out.Unpriced) > 0 {
		fmt.Fprintf(&msg, " No recent price for: %s.", strings.Join(out.Unpriced, ", "))
	}
	return text(msg.String()), out, nil
}

// storeBefore orders complete stores before incomplete ones, then by cost.
func storeBefore(a, b groceries.BasketStore) bool {
	if a.Complete != b.Complete {
		return a.Complete
	}
	return a.Complete && a.CostMinor < b.CostMinor
}
