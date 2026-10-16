package httpapi

import (
	"io"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/groceries"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/store"
)

// insightsNow is the fixed "today" of the insights tests.
var insightsNow = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

// withGroceriesClock returns a client whose groceries service believes it is
// 2026-10-15.
func (a *testAPI) withGroceriesClock() *testAPI {
	c := *a
	c.handler = NewHandler(Deps{
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		DB:         a.pool,
		Auth:       a.auth,
		Finance:    a.svc,
		Groceries:  groceries.NewService(a.pool, nil).WithClock(func() time.Time { return insightsNow }),
		Workspaces: a.workspaces,
	})
	return &c
}

func near(t *testing.T, what string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-6 {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
}

func day(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}

func numeric(s string) pgtype.Numeric {
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		panic(err)
	}
	return n
}

type seedLine struct {
	raw      string
	qty      string
	amount   int64
	discount int64
	variant  *uuid.UUID
}

func (a *testAPI) seedReceipt(storeID uuid.UUID, date, currency string, lines ...seedLine) {
	a.t.Helper()
	q := store.New(a.pool)
	var total int64
	for _, l := range lines {
		total += l.amount
	}
	r, err := q.InsertGroceryReceipt(a.ctx, store.InsertGroceryReceiptParams{
		StoreID: storeID, PurchasedOn: day(date), Currency: currency, TotalMinor: total, Source: "manual",
	})
	if err != nil {
		a.t.Fatal(err)
	}
	for i, l := range lines {
		if _, err := q.InsertGroceryReceiptLine(a.ctx, store.InsertGroceryReceiptLineParams{
			ReceiptID: r.ID, Position: int32(i + 1), RawText: l.raw, Quantity: numeric(l.qty),
			AmountMinor: l.amount, DiscountMinor: l.discount, VariantID: l.variant,
		}); err != nil {
			a.t.Fatal(err)
		}
	}
}

func (a *testAPI) createGroceryVariant(productID uuid.UUID, body map[string]any) groceries.Variant {
	a.t.Helper()
	var v groceries.Variant
	a.do(http.MethodPost, gp+"/products/"+productID.String()+"/variants", body).expect(http.StatusCreated).decode(&v)
	return v
}

func (a *testAPI) createPriceCheck(store, variant uuid.UUID, date string, price int64, currency string) groceries.PriceCheck {
	a.t.Helper()
	var pc groceries.PriceCheck
	a.do(http.MethodPost, gp+"/price-checks", map[string]any{
		"store_id": store, "variant_id": variant, "observed_on": date, "price_minor": price, "currency": currency,
	}).expect(http.StatusCreated).decode(&pc)
	return pc
}

// groceryScenario seeds two stores with receipts and a third one with price
// checks only. Unit prices (minor units per compare unit):
//
//	Whole milk  Mercadona Hacendado 1 l 95 (Sep), 90 (Aug); Lidl Milbona 1 l 92 (Sep), 89 (Jul)
//	Eggs (dozen) Mercadona 12 pack 20/egg, Lidl 22.5/egg (Jul, stale)
//	Bananas     Mercadona loose 180 (Sep, 30 discount), 170 (Aug); Lidl 170
//	Napkins     Lidl 75
//	Olive oil   Mercadona Hacendado 895; Lidl Carbonell 799 (price check)
type groceryScenario struct {
	api                                     *testAPI
	mercadona, lidl, dia                    groceries.Store
	milk, eggs, bananas, napkins, oil       groceries.Product
	hacendado, milbona, pascual1, pascual15 groceries.Variant
	eggs12, bananasLoose, napkinsGeneric    groceries.Variant
	hacendadoOil, carbonell                 groceries.Variant
}

func newGroceryScenario(t *testing.T) *groceryScenario {
	t.Helper()
	api := newTestAPI(t).withGroceriesClock()
	s := &groceryScenario{api: api}
	s.mercadona = api.createGroceryStore("Mercadona")
	s.lidl = api.createGroceryStore("Lidl")
	s.dia = api.createGroceryStore("Dia")
	groups := map[string]uuid.UUID{}
	for _, g := range api.groceryGroups() {
		groups[g.Name] = g.ID
	}
	product := func(name, unit, group string, multiplier int) groceries.Product {
		body := map[string]any{"name": name, "compare_unit": unit, "display_multiplier": multiplier}
		if group != "" {
			body["group_id"] = groups[group]
		}
		return api.createGroceryProduct(body)
	}
	s.milk = product("Whole milk", "l", "Dairy & eggs", 1)
	s.eggs = product("Eggs", "unit", "Dairy & eggs", 12)
	s.bananas = product("Bananas", "kg", "Produce", 1)
	s.napkins = product("Napkins", "unit", "", 1)
	s.oil = product("Olive oil", "l", "Pantry", 1)

	s.hacendado = api.createGroceryVariant(s.milk.ID, map[string]any{"brand": "Hacendado", "pack_size": 1, "pack_label": "1 l", "store_id": s.mercadona.ID})
	s.milbona = api.createGroceryVariant(s.milk.ID, map[string]any{"brand": "Milbona", "pack_size": 1, "pack_label": "1 l", "store_id": s.lidl.ID})
	s.pascual1 = api.createGroceryVariant(s.milk.ID, map[string]any{"brand": "Pascual", "pack_size": 1, "pack_label": "1 l"})
	s.pascual15 = api.createGroceryVariant(s.milk.ID, map[string]any{"brand": "Pascual", "pack_size": 1.5, "pack_label": "1.5 l"})
	s.eggs12 = api.createGroceryVariant(s.eggs.ID, map[string]any{"brand": "Generic", "pack_size": 12, "pack_label": "12 eggs"})
	s.bananasLoose = api.createGroceryVariant(s.bananas.ID, map[string]any{"brand": "Generic"})
	s.napkinsGeneric = api.createGroceryVariant(s.napkins.ID, map[string]any{"brand": "Generic"})
	s.hacendadoOil = api.createGroceryVariant(s.oil.ID, map[string]any{"brand": "Hacendado", "pack_size": 1, "store_id": s.mercadona.ID})
	s.carbonell = api.createGroceryVariant(s.oil.ID, map[string]any{"brand": "Carbonell", "pack_size": 1})

	id := func(v groceries.Variant) *uuid.UUID { return &v.ID }
	// September: a receipt per store; one unassigned line.
	api.seedReceipt(s.mercadona.ID, "2026-09-05", "EUR",
		seedLine{"LECHE ENT. HACENDADO", "6", 570, 0, id(s.hacendado)},
		seedLine{"HUEVOS 12", "2", 480, 0, id(s.eggs12)},
		seedLine{"PLATANO", "1.5", 270, 30, id(s.bananasLoose)},
		seedLine{"ACEITE OLIVA 1L", "1", 895, 0, id(s.hacendadoOil)},
		seedLine{"MISC", "1", 100, 0, nil},
	)
	api.seedReceipt(s.lidl.ID, "2026-09-12", "EUR",
		seedLine{"MILBONA LECHE", "4", 368, 0, id(s.milbona)},
		seedLine{"SERVILLETAS", "2", 150, 0, id(s.napkinsGeneric)},
		seedLine{"BANANAS", "2", 340, 0, id(s.bananasLoose)},
	)
	api.seedReceipt(s.mercadona.ID, "2026-08-10", "EUR",
		seedLine{"LECHE ENT. HACENDADO", "6", 540, 0, id(s.hacendado)},
		seedLine{"PLATANO", "3", 510, 0, id(s.bananasLoose)},
	)
	api.seedReceipt(s.lidl.ID, "2026-07-20", "EUR",
		seedLine{"MILBONA LECHE", "3", 267, 0, id(s.milbona)},
		seedLine{"HUEVOS 12", "1", 270, 0, id(s.eggs12)},
	)
	// A second currency.
	api.seedReceipt(s.mercadona.ID, "2026-09-20", "USD",
		seedLine{"MILK", "1", 150, 0, id(s.hacendado)},
	)
	// Price checks: fresh, stale and outside the 180 day window.
	api.createPriceCheck(s.mercadona.ID, s.pascual1.ID, "2026-10-01", 139, "EUR")
	api.createPriceCheck(s.dia.ID, s.pascual15.ID, "2026-07-01", 195, "EUR")
	api.createPriceCheck(s.lidl.ID, s.pascual1.ID, "2026-03-01", 99, "EUR")
	api.createPriceCheck(s.lidl.ID, s.carbonell.ID, "2026-10-02", 799, "EUR")
	api.createPriceCheck(s.mercadona.ID, s.hacendado.ID, "2026-08-20", 100, "EUR")
	return s
}

func TestGroceryInsightsRequireAuthentication(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t).as("")
	id := uuid.NewString()
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, gp + "/price-checks"}, {http.MethodPost, gp + "/price-checks"},
		{http.MethodDelete, gp + "/price-checks/" + id}, {http.MethodGet, gp + "/breakdown"},
		{http.MethodGet, gp + "/compare"}, {http.MethodGet, gp + "/basket"},
		{http.MethodGet, gp + "/products/" + id + "/prices"},
	} {
		api.do(c.method, c.path, nil).expect(http.StatusUnauthorized)
	}
}

func TestGroceryBreakdown(t *testing.T) {
	t.Parallel()
	s := newGroceryScenario(t)
	api := s.api

	var b groceries.Breakdown
	b = groceries.Breakdown{}
	api.do(http.MethodGet, gp+"/breakdown?month=2026-09", nil).expect(http.StatusOK).decode(&b)
	if b.Month != "2026-09" || len(b.Currencies) != 2 {
		t.Fatalf("unexpected breakdown %+v", b)
	}
	eur := b.Currencies[0]
	if eur.Currency != "EUR" || eur.TotalMinor != 3173 || eur.ReceiptCount != 2 || eur.AverageReceiptMinor != 1587 {
		t.Fatalf("unexpected EUR totals %+v", eur)
	}
	if eur.LinesTotalMinor != 3173 || eur.AssignedMinor != 3073 || eur.UnassignedMinor != 100 {
		t.Fatalf("unexpected coverage %+v", eur)
	}
	near(t, "coverage", eur.CoveragePct, 96.8)
	if eur.FoodMinor != 2923 || eur.NonFoodMinor != 0 {
		t.Fatalf("unexpected food split %+v", eur)
	}
	if eur.GroceriesCategoryMinor != nil {
		t.Fatalf("no Groceries category yet, got %v", *eur.GroceriesCategoryMinor)
	}

	// Groups sorted by spend; unassigned lines and group-less products share
	// the null group.
	type group struct {
		name  string
		total int64
		share float64
	}
	want := []group{{"Dairy & eggs", 1418, 44.7}, {"Pantry", 895, 28.2}, {"Produce", 610, 19.2}, {"", 250, 7.9}}
	if len(eur.ByGroup) != len(want) {
		t.Fatalf("unexpected groups %+v", eur.ByGroup)
	}
	for i, w := range want {
		g := eur.ByGroup[i]
		name := ""
		if g.Name != nil {
			name = *g.Name
		}
		if name != w.name || g.TotalMinor != w.total || (w.name == "" && g.GroupID != nil) {
			t.Fatalf("group %d: got %+v, want %+v", i, g, w)
		}
		near(t, "share", g.SharePct, w.share)
		if (w.name != "") != (g.DefaultKey != nil) {
			t.Fatalf("group %d: default key must be set for default groups only, got %+v", i, g)
		}
	}

	if len(eur.ByStore) != 2 || eur.ByStore[0].Name != "Mercadona" || eur.ByStore[0].TotalMinor != 2315 || eur.ByStore[0].ReceiptCount != 1 ||
		eur.ByStore[1].Name != "Lidl" || eur.ByStore[1].TotalMinor != 858 {
		t.Fatalf("unexpected stores %+v", eur.ByStore)
	}

	type top struct {
		name  string
		qty   float64
		spend int64
	}
	wantTop := []top{{"Whole milk", 10, 938}, {"Olive oil", 1, 895}, {"Bananas", 3.5, 610}, {"Eggs", 24, 480}, {"Napkins", 2, 150}}
	if len(eur.TopProducts) != len(wantTop) {
		t.Fatalf("unexpected top products %+v", eur.TopProducts)
	}
	for i, w := range wantTop {
		p := eur.TopProducts[i]
		if p.Name != w.name || p.SpendMinor != w.spend {
			t.Fatalf("top %d: got %+v, want %+v", i, p, w)
		}
		near(t, "quantity", p.Quantity, w.qty)
	}

	usd := b.Currencies[1]
	if usd.Currency != "USD" || usd.TotalMinor != 150 || usd.ReceiptCount != 1 || usd.AverageReceiptMinor != 150 || usd.CoveragePct != 100 {
		t.Fatalf("unexpected USD %+v", usd)
	}

	// Currency filter.
	var only groceries.Breakdown
	api.do(http.MethodGet, gp+"/breakdown?month=2026-09&currency=usd", nil).expect(http.StatusOK).decode(&only)
	if len(only.Currencies) != 1 || only.Currencies[0].Currency != "USD" {
		t.Fatalf("currency filter: %+v", only)
	}

	// The Groceries category total sits next to the receipts.
	cat := api.createCategory("Groceries", "expense")
	eurAccount := api.createAccount("Checking", "EUR", 0)
	usdAccount := api.createAccount("Dollars", "USD", 0)
	tx := func(account uuid.UUID, amount int64, date string) {
		api.createTransaction(map[string]any{"type": "expense", "account_id": account, "amount": amount, "category_id": cat.ID, "occurred_on": date})
	}
	tx(eurAccount.ID, 2000, "2026-09-05")
	tx(eurAccount.ID, 1100, "2026-09-30")
	tx(eurAccount.ID, 999, "2026-10-01") // other month
	tx(usdAccount.ID, 150, "2026-09-20")
	b = groceries.Breakdown{}
	api.do(http.MethodGet, gp+"/breakdown?month=2026-09", nil).expect(http.StatusOK).decode(&b)
	if b.Currencies[0].GroceriesCategoryMinor == nil || *b.Currencies[0].GroceriesCategoryMinor != 3100 ||
		b.Currencies[1].GroceriesCategoryMinor == nil || *b.Currencies[1].GroceriesCategoryMinor != 150 {
		t.Fatalf("unexpected category totals %+v", b.Currencies)
	}

	// The default month is the current one (October: nothing bought, but the
	// category has an expense).
	b = groceries.Breakdown{}
	api.do(http.MethodGet, gp+"/breakdown", nil).expect(http.StatusOK).decode(&b)
	if b.Month != "2026-10" || len(b.Currencies) != 1 || b.Currencies[0].Currency != "EUR" ||
		b.Currencies[0].TotalMinor != 0 || *b.Currencies[0].GroceriesCategoryMinor != 999 {
		t.Fatalf("unexpected current month %+v", b)
	}
	// A month without data is empty.
	b = groceries.Breakdown{}
	api.do(http.MethodGet, gp+"/breakdown?month=2025-01", nil).expect(http.StatusOK).decode(&b)
	if len(b.Currencies) != 0 {
		t.Fatalf("expected no currencies, got %+v", b.Currencies)
	}

	for _, q := range []string{"month=2026-13", "month=september", "month=2026-9", "currency=EU"} {
		api.do(http.MethodGet, gp+"/breakdown?"+q, nil).expect(http.StatusUnprocessableEntity)
	}
}

func TestGroceryCompare(t *testing.T) {
	t.Parallel()
	s := newGroceryScenario(t)
	api := s.api

	var c groceries.Comparison
	c = groceries.Comparison{}
	api.do(http.MethodGet, gp+"/compare", nil).expect(http.StatusOK).decode(&c)
	if c.Level != "product" || len(c.Currencies) != 2 || c.Currencies[0].Currency != "EUR" || c.Currencies[1].Currency != "USD" {
		t.Fatalf("unexpected comparison %+v", c)
	}
	eur := c.Currencies[0]
	// Columns: stores with observations in the window, by name.
	if len(eur.Stores) != 3 || eur.Stores[0].ID != s.dia.ID || eur.Stores[1].ID != s.lidl.ID || eur.Stores[2].ID != s.mercadona.ID {
		t.Fatalf("unexpected stores %+v", eur.Stores)
	}
	if len(eur.Rows) != 5 {
		t.Fatalf("unexpected rows %+v", eur.Rows)
	}
	row := func(name string) groceries.CompareRow {
		for _, r := range eur.Rows {
			if r.ProductName == name {
				return r
			}
		}
		t.Fatalf("no row for %s", name)
		return groceries.CompareRow{}
	}
	cell := func(r groceries.CompareRow, st groceries.Store) groceries.PriceCell {
		c, ok := r.Cells[st.ID.String()]
		if !ok {
			t.Fatalf("%s has no price at %s", r.ProductName, st.Name)
		}
		return c
	}

	milk := row("Whole milk")
	// Mercadona: cheapest of Hacendado 95 (latest Sep, not the 100 check of
	// August nor the 90 receipt of August) and Pascual 1 l 139.
	m := cell(milk, s.mercadona)
	near(t, "milk mercadona", m.UnitPriceMinor, 95)
	if m.VariantID != s.hacendado.ID || m.Source != "receipt" || m.ObservedOn != "2026-09-05" || m.Stale {
		t.Fatalf("unexpected cell %+v", m)
	}
	// Lidl: Milbona 92; the Pascual check of March is outside the window.
	l := cell(milk, s.lidl)
	near(t, "milk lidl", l.UnitPriceMinor, 92)
	if l.VariantID != s.milbona.ID {
		t.Fatalf("unexpected cell %+v", l)
	}
	// Dia: only a stale price check, 195 for 1.5 l.
	d := cell(milk, s.dia)
	near(t, "milk dia", d.UnitPriceMinor, 130)
	if d.VariantID != s.pascual15.ID || d.Source != "price_check" || !d.Stale || d.ObservedOn != "2026-07-01" {
		t.Fatalf("unexpected cell %+v", d)
	}
	if milk.BestStoreID == nil || *milk.BestStoreID != s.lidl.ID || milk.MaxDiffPct == nil {
		t.Fatalf("unexpected best store %+v", milk)
	}
	near(t, "milk diff", *milk.MaxDiffPct, 41.3) // 130 / 92 - 1
	if milk.Brand != nil || milk.CompareUnit != "l" {
		t.Fatalf("unexpected row %+v", milk)
	}

	// Eggs: priced per dozen; Lidl's receipt of July 20 is 87 days old.
	eggs := row("Eggs")
	e := cell(eggs, s.mercadona)
	near(t, "eggs mercadona", e.UnitPriceMinor, 20)
	near(t, "eggs mercadona dozen", e.DisplayUnitPrice, 240)
	if e.Stale {
		t.Fatalf("fresh eggs flagged stale: %+v", e)
	}
	el := cell(eggs, s.lidl)
	near(t, "eggs lidl", el.UnitPriceMinor, 22.5)
	near(t, "eggs lidl dozen", el.DisplayUnitPrice, 270)
	if !el.Stale || len(eggs.Cells) != 2 || eggs.BestStoreID == nil || *eggs.BestStoreID != s.mercadona.ID {
		t.Fatalf("unexpected eggs %+v", eggs)
	}
	near(t, "eggs diff", *eggs.MaxDiffPct, 12.5)

	// Bananas: loose, the latest Mercadona line (Sep, after a discount) wins
	// over August's 170.
	bananas := row("Bananas")
	near(t, "bananas mercadona", cell(bananas, s.mercadona).UnitPriceMinor, 180)
	near(t, "bananas lidl", cell(bananas, s.lidl).UnitPriceMinor, 170)
	near(t, "bananas diff", *bananas.MaxDiffPct, 5.9)

	// A single store: no best store and no difference.
	napkins := row("Napkins")
	near(t, "napkins lidl", cell(napkins, s.lidl).UnitPriceMinor, 75)
	if napkins.BestStoreID != nil || napkins.MaxDiffPct != nil || len(napkins.Cells) != 1 {
		t.Fatalf("unexpected napkins %+v", napkins)
	}

	oil := row("Olive oil")
	near(t, "oil mercadona", cell(oil, s.mercadona).UnitPriceMinor, 895)
	near(t, "oil lidl", cell(oil, s.lidl).UnitPriceMinor, 799)
	near(t, "oil diff", *oil.MaxDiffPct, 12.0)

	// Currencies are never mixed.
	if usd := c.Currencies[1]; len(usd.Stores) != 1 || len(usd.Rows) != 1 || usd.Rows[0].ProductName != "Whole milk" ||
		usd.Rows[0].Cells[s.mercadona.ID.String()].UnitPriceMinor != 150 {
		t.Fatalf("unexpected USD %+v", usd)
	}
	c = groceries.Comparison{}
	api.do(http.MethodGet, gp+"/compare?currency=USD", nil).expect(http.StatusOK).decode(&c)
	if len(c.Currencies) != 1 || c.Currencies[0].Currency != "USD" {
		t.Fatalf("currency filter: %+v", c)
	}

	// Brand level: only manufacturer brands in at least two stores. Store
	// brands (Hacendado, Milbona) and generic variants are excluded.
	c = groceries.Comparison{}
	api.do(http.MethodGet, gp+"/compare?level=brand", nil).expect(http.StatusOK).decode(&c)
	if c.Level != "brand" || len(c.Currencies) != 1 || c.Currencies[0].Currency != "EUR" || len(c.Currencies[0].Rows) != 1 {
		t.Fatalf("unexpected brand comparison %+v", c)
	}
	brand := c.Currencies[0]
	pascual := brand.Rows[0]
	if pascual.ProductName != "Whole milk" || pascual.Brand == nil || *pascual.Brand != "Pascual" || len(pascual.Cells) != 2 {
		t.Fatalf("unexpected brand row %+v", pascual)
	}
	near(t, "pascual mercadona", pascual.Cells[s.mercadona.ID.String()].UnitPriceMinor, 139)
	near(t, "pascual dia", pascual.Cells[s.dia.ID.String()].UnitPriceMinor, 130)
	if pascual.BestStoreID == nil || *pascual.BestStoreID != s.dia.ID {
		t.Fatalf("unexpected best %+v", pascual)
	}
	near(t, "pascual diff", *pascual.MaxDiffPct, 6.9)
	if len(brand.Stores) != 2 {
		t.Fatalf("unexpected brand stores %+v", brand.Stores)
	}

	// Archived stores and archived products drop out.
	api.do(http.MethodPatch, gp+"/stores/"+s.dia.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK)
	api.do(http.MethodPatch, gp+"/products/"+s.napkins.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK)
	c = groceries.Comparison{}
	api.do(http.MethodGet, gp+"/compare?currency=EUR", nil).expect(http.StatusOK).decode(&c)
	if len(c.Currencies[0].Stores) != 2 || len(c.Currencies[0].Rows) != 4 {
		t.Fatalf("unexpected comparison after archiving %+v", c.Currencies[0])
	}
	if _, ok := c.Currencies[0].Rows[3].Cells[s.dia.ID.String()]; ok {
		t.Fatal("archived store still compared")
	}

	for _, q := range []string{"level=family", "currency=EU"} {
		api.do(http.MethodGet, gp+"/compare?"+q, nil).expect(http.StatusUnprocessableEntity)
	}
}

func TestGroceryBasket(t *testing.T) {
	t.Parallel()
	s := newGroceryScenario(t)
	api := s.api

	var b groceries.Basket
	b = groceries.Basket{}
	api.do(http.MethodGet, gp+"/basket?currency=EUR", nil).expect(http.StatusOK).decode(&b)
	// Last 3 full months: July, August and September.
	if b.Months != 3 || b.From != "2026-07-01" || b.To != "2026-09-30" || len(b.Currencies) != 1 {
		t.Fatalf("unexpected basket %+v", b)
	}
	eur := b.Currencies[0]

	type product struct {
		name string
		qty  float64
		paid int64
	}
	wantProducts := []product{
		{"Bananas", 2.167, 373}, {"Eggs", 12, 250}, {"Napkins", 0.667, 50}, {"Olive oil", 0.333, 298}, {"Whole milk", 6.333, 582},
	}
	if len(eur.Products) != len(wantProducts) {
		t.Fatalf("unexpected products %+v", eur.Products)
	}
	for i, w := range wantProducts {
		p := eur.Products[i]
		if p.Name != w.name || p.PaidMinor != w.paid {
			t.Fatalf("product %d: got %+v, want %+v", i, p, w)
		}
		near(t, w.name+" quantity", p.MonthlyQuantity, w.qty)
	}
	if eur.PaidMinor != 1553 { // (1120 + 750 + 150 + 895 + 1745) / 3
		t.Fatalf("paid: got %d", eur.PaidMinor)
	}

	// Stores: Dia, Lidl, Mercadona. Mercadona has no napkins, Lidl is the
	// only one with every product, Dia only prices milk.
	if len(eur.Stores) != 3 {
		t.Fatalf("unexpected stores %+v", eur.Stores)
	}
	dia, lidl, mercadona := eur.Stores[0], eur.Stores[1], eur.Stores[2]
	if dia.Name != "Dia" || dia.CostMinor != 823 || dia.Complete || dia.PricedCount != 1 || len(dia.MissingProducts) != 4 { // 130 * 19/3
		t.Fatalf("unexpected Dia %+v", dia)
	}
	if lidl.Name != "Lidl" || lidl.CostMinor != 1537 || !lidl.Complete || lidl.PricedCount != 5 || len(lidl.MissingProducts) != 0 {
		// 170*6.5/3 + 22.5*12 + 75*2/3 + 799/3 + 92*19/3
		t.Fatalf("unexpected Lidl %+v", lidl)
	}
	if mercadona.Name != "Mercadona" || mercadona.CostMinor != 1530 || mercadona.Complete || mercadona.PricedCount != 4 ||
		len(mercadona.MissingProducts) != 1 || mercadona.MissingProducts[0].Name != "Napkins" {
		// 180*6.5/3 + 20*12 + 895/3 + 95*19/3: cheaper but incomplete
		t.Fatalf("unexpected Mercadona %+v", mercadona)
	}
	if eur.BestSingleStore == nil || eur.BestSingleStore.StoreID != s.lidl.ID || eur.BestSingleStore.CostMinor != 1537 {
		t.Fatalf("unexpected best single store %+v", eur.BestSingleStore)
	}

	// Split: bananas, napkins, oil and milk at Lidl, eggs at Mercadona.
	if eur.Split.CostMinor != 1507 || len(eur.Split.Lines) != 5 || len(eur.Split.UnpricedProducts) != 0 {
		t.Fatalf("unexpected split %+v", eur.Split)
	}
	wantStores := map[string]string{"Bananas": "Lidl", "Eggs": "Mercadona", "Napkins": "Lidl", "Olive oil": "Lidl", "Whole milk": "Lidl"}
	for _, l := range eur.Split.Lines {
		if wantStores[l.Name] != l.StoreName {
			t.Fatalf("split line %+v, want store %s", l, wantStores[l.Name])
		}
	}

	// One month only (September): bananas 1.5 + 2 kg, no eggs from July.
	b = groceries.Basket{}
	api.do(http.MethodGet, gp+"/basket?months=1&currency=EUR", nil).expect(http.StatusOK).decode(&b)
	if b.From != "2026-09-01" || b.To != "2026-09-30" || len(b.Currencies[0].Products) != 5 {
		t.Fatalf("unexpected one-month basket %+v", b)
	}
	near(t, "bananas", b.Currencies[0].Products[0].MonthlyQuantity, 3.5)
	if b.Currencies[0].PaidMinor != 3173-100-0 { // lines minus the unassigned one
		t.Fatalf("paid: got %d", b.Currencies[0].PaidMinor)
	}

	// Both currencies by default; the USD one prices milk at Mercadona only.
	b = groceries.Basket{}
	api.do(http.MethodGet, gp+"/basket", nil).expect(http.StatusOK).decode(&b)
	if len(b.Currencies) != 2 || b.Currencies[1].Currency != "USD" {
		t.Fatalf("unexpected basket currencies %+v", b.Currencies)
	}
	usd := b.Currencies[1]
	// 1 l bought in September (outside the default window? no: inside).
	if len(usd.Products) != 1 || usd.BestSingleStore == nil || usd.BestSingleStore.CostMinor != 50 || usd.Split.CostMinor != 50 {
		t.Fatalf("unexpected USD basket %+v", usd)
	}

	// No complete store: no best single store.
	api.do(http.MethodPatch, gp+"/stores/"+s.lidl.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK)
	b = groceries.Basket{}
	api.do(http.MethodGet, gp+"/basket?currency=EUR", nil).expect(http.StatusOK).decode(&b)
	if b.Currencies[0].BestSingleStore != nil {
		t.Fatalf("unexpected best single store %+v", b.Currencies[0].BestSingleStore)
	}
	// Napkins are now unpriced everywhere and the split leaves them out.
	if split := b.Currencies[0].Split; len(split.UnpricedProducts) != 1 || split.UnpricedProducts[0].Name != "Napkins" || len(split.Lines) != 4 {
		t.Fatalf("unexpected split %+v", split)
	}

	for _, q := range []string{"months=0", "months=13", "months=abc", "currency=EU"} {
		api.do(http.MethodGet, gp+"/basket?"+q, nil).expect(http.StatusUnprocessableEntity)
	}
}

func TestGroceryProductPrices(t *testing.T) {
	t.Parallel()
	s := newGroceryScenario(t)
	api := s.api

	var p groceries.ProductPrices
	p = groceries.ProductPrices{}
	api.do(http.MethodGet, gp+"/products/"+s.milk.ID.String()+"/prices", nil).expect(http.StatusOK).decode(&p)
	if p.Name != "Whole milk" || p.CompareUnit != "l" || len(p.Variants) != 4 {
		t.Fatalf("unexpected prices %+v", p)
	}
	variant := func(id uuid.UUID) groceries.VariantPrices {
		for _, v := range p.Variants {
			if v.VariantID == id {
				return v
			}
		}
		t.Fatalf("variant %s missing", id)
		return groceries.VariantPrices{}
	}

	// Hacendado: the Mercadona history averages August's receipt (90) with
	// the price check (100); USD is a separate series.
	h := variant(s.hacendado.ID)
	if len(h.Latest) != 2 || h.Latest[0].Currency != "EUR" || h.Latest[1].Currency != "USD" {
		t.Fatalf("unexpected latest %+v", h.Latest)
	}
	near(t, "hacendado latest", h.Latest[0].UnitPriceMinor, 95)
	if h.Latest[0].StoreName != "Mercadona" || h.Latest[0].ObservedOn != "2026-09-05" || h.Latest[0].Source != "receipt" || h.Latest[0].Stale {
		t.Fatalf("unexpected latest %+v", h.Latest[0])
	}
	if len(h.History) != 2 || h.History[0].Currency != "EUR" || len(h.History[0].Points) != 2 {
		t.Fatalf("unexpected history %+v", h.History)
	}
	aug, sep := h.History[0].Points[0], h.History[0].Points[1]
	if aug.Month != "2026-08" || aug.Observations != 2 || sep.Month != "2026-09" || sep.Observations != 1 {
		t.Fatalf("unexpected points %+v", h.History[0].Points)
	}
	near(t, "august average", aug.UnitPriceMinor, 95)
	near(t, "september average", sep.UnitPriceMinor, 95)
	if usd := h.History[1]; usd.Currency != "USD" || len(usd.Points) != 1 || usd.Points[0].Month != "2026-09" || usd.Points[0].UnitPriceMinor != 150 {
		t.Fatalf("unexpected USD history %+v", usd)
	}

	// Pascual 1 l: no window for the latest price (March at Lidl is stale)
	// but within the last 12 months for the history.
	pa := variant(s.pascual1.ID)
	if len(pa.Latest) != 2 || pa.Latest[0].StoreName != "Lidl" || !pa.Latest[0].Stale || pa.Latest[1].StoreName != "Mercadona" || pa.Latest[1].Stale {
		t.Fatalf("unexpected Pascual latest %+v", pa.Latest)
	}
	near(t, "pascual lidl", pa.Latest[0].UnitPriceMinor, 99)
	if len(pa.History) != 2 || pa.History[0].Points[0].Month != "2026-03" || pa.History[1].Points[0].Month != "2026-10" {
		t.Fatalf("unexpected Pascual history %+v", pa.History)
	}

	// A variant without observations is listed empty.
	empty := variant(s.milbona.ID)
	if len(empty.Latest) != 1 || empty.Latest[0].StoreName != "Lidl" {
		t.Fatalf("unexpected Milbona %+v", empty)
	}
	near(t, "milbona", empty.Latest[0].UnitPriceMinor, 92)
	if len(empty.History[0].Points) != 2 || empty.History[0].Points[0].Month != "2026-07" {
		t.Fatalf("unexpected Milbona history %+v", empty.History)
	}

	// Dozen-priced product: the display price is per dozen.
	p = groceries.ProductPrices{}
	api.do(http.MethodGet, gp+"/products/"+s.eggs.ID.String()+"/prices", nil).expect(http.StatusOK).decode(&p)
	if p.DisplayMultiplier != 12 || len(p.Variants) != 1 || len(p.Variants[0].Latest) != 2 {
		t.Fatalf("unexpected eggs %+v", p)
	}
	near(t, "eggs mercadona", p.Variants[0].Latest[1].DisplayUnitPrice, 240)
	near(t, "eggs lidl", p.Variants[0].History[0].Points[0].DisplayUnitPrice, 270)

	api.do(http.MethodGet, gp+"/products/"+uuid.NewString()+"/prices", nil).expect(http.StatusNotFound)
	api.do(http.MethodGet, gp+"/products/nope/prices", nil).expect(http.StatusUnprocessableEntity)
}

func TestGroceryPriceChecks(t *testing.T) {
	t.Parallel()
	s := newGroceryScenario(t)
	api := s.api

	pc := api.createPriceCheck(s.lidl.ID, s.pascual15.ID, "2026-10-10", 195, "eur")
	if pc.Currency != "EUR" || pc.PriceMinor != 195 || pc.StoreName != "Lidl" || pc.Brand != "Pascual" || pc.ProductName != "Whole milk" || pc.ObservedOn != "2026-10-10" {
		t.Fatalf("unexpected price check %+v", pc)
	}
	near(t, "unit price", pc.UnitPriceMinor, 130)

	// Loose variants: the price is per compare unit. Defaults to today.
	var loose groceries.PriceCheck
	api.do(http.MethodPost, gp+"/price-checks", map[string]any{
		"store_id": s.dia.ID, "variant_id": s.bananasLoose.ID, "price_minor": 189, "currency": "EUR", "note": "  on offer ",
	}).expect(http.StatusCreated).decode(&loose)
	if loose.ObservedOn != "2026-10-15" || loose.Note == nil || *loose.Note != "on offer" {
		t.Fatalf("unexpected check %+v", loose)
	}
	near(t, "loose unit price", loose.UnitPriceMinor, 189)

	// A dozen-priced pack: 270 for 12 eggs is 22.5 per egg, 270 per dozen.
	eggs := api.createPriceCheck(s.dia.ID, s.eggs12.ID, "2026-10-14", 270, "EUR")
	near(t, "egg unit price", eggs.UnitPriceMinor, 22.5)
	near(t, "egg display price", eggs.DisplayUnitPrice, 270)

	var list struct{ Items []groceries.PriceCheck }
	list.Items = nil
	api.do(http.MethodGet, gp+"/price-checks", nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 8 || list.Items[0].ID != loose.ID {
		t.Fatalf("unexpected list (%d) %+v", len(list.Items), list.Items[0])
	}
	list.Items = nil
	api.do(http.MethodGet, gp+"/price-checks?variant_id="+s.pascual1.ID.String(), nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 2 {
		t.Fatalf("variant filter: %d", len(list.Items))
	}
	list.Items = nil
	api.do(http.MethodGet, gp+"/price-checks?store_id="+s.dia.ID.String(), nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 3 {
		t.Fatalf("store filter: %d", len(list.Items))
	}
	list.Items = nil
	api.do(http.MethodGet, gp+"/price-checks?product_id="+s.oil.ID.String(), nil).expect(http.StatusOK).decode(&list)
	if len(list.Items) != 1 || list.Items[0].Brand != "Carbonell" {
		t.Fatalf("product filter: %+v", list.Items)
	}
	api.do(http.MethodGet, gp+"/price-checks?store_id=nope", nil).expect(http.StatusUnprocessableEntity)

	valid := func() map[string]any {
		return map[string]any{"store_id": s.lidl.ID, "variant_id": s.pascual1.ID, "observed_on": "2026-10-10", "price_minor": 100, "currency": "EUR"}
	}
	invalid := map[string]func(map[string]any){
		"zero price":      func(b map[string]any) { b["price_minor"] = 0 },
		"negative price":  func(b map[string]any) { b["price_minor"] = -5 },
		"bad currency":    func(b map[string]any) { b["currency"] = "ZZZ" },
		"short currency":  func(b map[string]any) { b["currency"] = "EU" },
		"future date":     func(b map[string]any) { b["observed_on"] = "2026-10-16" },
		"malformed date":  func(b map[string]any) { b["observed_on"] = "yesterday" },
		"unknown store":   func(b map[string]any) { b["store_id"] = uuid.NewString() },
		"unknown variant": func(b map[string]any) { b["variant_id"] = uuid.NewString() },
		"bad store id":    func(b map[string]any) { b["store_id"] = "nope" },
		"missing variant": func(b map[string]any) { delete(b, "variant_id") },
		"note too long":   func(b map[string]any) { b["note"] = strings.Repeat("x", 501) },
		"archived store": func(b map[string]any) {
			b["store_id"] = s.dia.ID
			api.do(http.MethodPatch, gp+"/stores/"+s.dia.ID.String(), map[string]any{"archived": true}).expect(http.StatusOK)
		},
	}
	for name, mutate := range invalid {
		body := valid()
		mutate(body)
		if r := api.do(http.MethodPost, gp+"/price-checks", body); r.Status != http.StatusUnprocessableEntity {
			t.Fatalf("%s: got %d %s", name, r.Status, r.Body)
		}
	}

	api.do(http.MethodDelete, gp+"/price-checks/"+loose.ID.String(), nil).expect(http.StatusNoContent)
	api.do(http.MethodDelete, gp+"/price-checks/"+loose.ID.String(), nil).expect(http.StatusNotFound)
	api.do(http.MethodDelete, gp+"/price-checks/nope", nil).expect(http.StatusUnprocessableEntity)
}

func TestGroceryInsightsAreIsolatedByWorkspace(t *testing.T) {
	t.Parallel()
	s := newGroceryScenario(t)
	other := s.api.newUser("other@example.com", "Other")

	for _, path := range []string{"/price-checks", "/breakdown?month=2026-09", "/compare", "/basket"} {
		body := string(other.do(http.MethodGet, gp+path, nil).expect(http.StatusOK).Body)
		for _, secret := range []string{"Mercadona", "Lidl", "Whole milk", s.mercadona.ID.String()} {
			if strings.Contains(body, secret) {
				t.Fatalf("GET %s leaks %q: %s", path, secret, body)
			}
		}
	}
	other.do(http.MethodGet, gp+"/products/"+s.milk.ID.String()+"/prices", nil).expect(http.StatusNotFound)

	// Another workspace's store or variant cannot be referenced.
	other.do(http.MethodPost, gp+"/price-checks", map[string]any{
		"store_id": s.mercadona.ID, "variant_id": s.hacendado.ID, "price_minor": 100, "currency": "EUR",
	}).expect(http.StatusUnprocessableEntity)

	// A check of the first workspace cannot be deleted from the other one.
	var list struct{ Items []groceries.PriceCheck }
	s.api.do(http.MethodGet, gp+"/price-checks", nil).expect(http.StatusOK).decode(&list)
	other.do(http.MethodDelete, gp+"/price-checks/"+list.Items[0].ID.String(), nil).expect(http.StatusNotFound)

	// The view applies row-level security to direct queries as well.
	var n int
	if err := other.pool.QueryRow(other.ctx, "SELECT count(*) FROM grocery_unit_prices").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("other workspace sees %d unit prices", n)
	}
	if err := s.api.pool.QueryRow(s.api.ctx, "SELECT count(*) FROM grocery_unit_prices").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 17 { // 12 receipt lines with a variant (all but MISC) + 5 checks
		t.Fatalf("owner sees %d unit prices", n)
	}
}
