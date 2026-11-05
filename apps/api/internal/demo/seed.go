// Package demo seeds a workspace with realistic fictional data for
// screenshots and a public demo. It goes through the same services as the
// REST API and MCP tools, so business rules and row-level security apply.
package demo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/groceries"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

// Supported locales for the seeded text.
const (
	LocaleEN = "en"
	LocaleES = "es"
)

// DefaultWorkspaceName is the name of the seeded workspace unless told
// otherwise.
const DefaultWorkspaceName = "Demo"

// historyMonths is how many months of transactions are seeded, the current
// one included.
const historyMonths = 6

const currencyEUR = "EUR"

// Options configure Seed.
type Options struct {
	// Email is the existing user who owns the workspace.
	Email string
	// Workspace is the workspace name. Defaults to DefaultWorkspaceName.
	Workspace string
	// Locale is the language of the seeded text: en (default) or es.
	Locale string
	// Reset deletes the owner's workspace with the same name and creates it
	// again. Without it an existing workspace is an error.
	Reset bool
}

// Summary counts what was seeded.
type Summary struct {
	WorkspaceID  uuid.UUID
	Accounts     int
	Categories   int
	Transactions int
	Budgets      int
	Recurring    int
	Stores       int
	Receipts     int
	Products     int
	PriceChecks  int
}

// Seeder builds demo workspaces.
type Seeder struct {
	Auth       *auth.Service
	Workspaces *workspace.Service
	Finance    *finance.Service
	Groceries  *groceries.Service
}

// run holds the state of one seeding.
type run struct {
	*Seeder
	locale     string
	today      time.Time
	accounts   map[string]finance.Account
	categories map[string]finance.Category
	// cardSpend is what was charged to the credit card, by months ago.
	cardSpend map[int]int64
	summary   Summary
}

// Seed creates the demo workspace owned by the user with opts.Email.
func (s *Seeder) Seed(ctx context.Context, opts Options) (Summary, error) {
	locale := strings.ToLower(strings.TrimSpace(opts.Locale))
	if locale == "" {
		locale = LocaleEN
	}
	if locale != LocaleEN && locale != LocaleES {
		return Summary{}, fmt.Errorf("unsupported locale %q: use %s or %s", opts.Locale, LocaleEN, LocaleES)
	}
	name := strings.TrimSpace(opts.Workspace)
	if name == "" {
		name = DefaultWorkspaceName
	}
	user, err := s.findUser(ctx, opts.Email)
	if err != nil {
		return Summary{}, err
	}

	// Only workspaces this user owns with exactly this name are considered:
	// no other workspace is ever touched.
	memberships, err := s.Workspaces.List(ctx, user.ID)
	if err != nil {
		return Summary{}, err
	}
	for _, m := range memberships {
		if m.Name != name || m.Role != workspace.RoleOwner {
			continue
		}
		if !opts.Reset {
			return Summary{}, fmt.Errorf("workspace %q already exists; pass --reset to delete it and seed it again", name)
		}
		if err := s.Workspaces.DeleteNow(ctx, user.ID, m.ID); err != nil {
			return Summary{}, fmt.Errorf("delete workspace %q: %w", name, err)
		}
	}

	w, err := s.Workspaces.Create(ctx, user.ID, name)
	if err != nil {
		return Summary{}, err
	}
	r := &run{
		Seeder:     s,
		locale:     locale,
		today:      s.Finance.Today(),
		accounts:   map[string]finance.Account{},
		categories: map[string]finance.Category{},
		cardSpend:  map[int]int64{},
	}
	r.summary.WorkspaceID = w.ID
	wctx := finance.WithActor(db.WithWorkspace(ctx, w.ID), user.ID)
	if err := r.seed(wctx); err != nil {
		// Do not leave a half-seeded workspace behind.
		_ = s.Workspaces.DeleteNow(context.WithoutCancel(ctx), user.ID, w.ID)
		return Summary{}, err
	}
	return r.summary, nil
}

func (s *Seeder) findUser(ctx context.Context, email string) (auth.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return auth.User{}, errors.New("an email is required")
	}
	users, err := s.Auth.ListUsers(ctx)
	if err != nil {
		return auth.User{}, err
	}
	for _, u := range users {
		if strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return auth.User{}, fmt.Errorf("no user with email %q: add them first with `api users add`", email)
}

func (r *run) seed(ctx context.Context) error {
	steps := []func(context.Context) error{
		r.seedAccounts, r.seedCategories, r.seedTransactions, r.seedRecurring,
		r.seedGroceries, r.seedCardPayments, r.seedBudgets, r.seedPriceChecks, r.count,
	}
	for _, step := range steps {
		if err := step(ctx); err != nil {
			return err
		}
	}
	return nil
}

// --- dates ---

// monthsAgo returns the day of the month that many months ago, clamped to
// the month's length, and whether it is not after today.
func (r *run) monthsAgo(months, day int) (time.Time, bool) {
	first := time.Date(r.today.Year(), r.today.Month()-time.Month(months), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1).Day()
	d := first.AddDate(0, 0, min(day, last)-1)
	return d, !d.After(r.today)
}

// monthOffset is how many months before the current month t is.
func (r *run) monthOffset(t time.Time) int {
	return (r.today.Year()*12 + int(r.today.Month())) - (t.Year()*12 + int(t.Month()))
}

func (r *run) historyStart() time.Time {
	d, _ := r.monthsAgo(historyMonths-1, 1)
	return d
}

func allMonths() []int {
	out := make([]int, historyMonths)
	for i := range out {
		out[i] = historyMonths - 1 - i
	}
	return out
}

func iso(t time.Time) string { return t.Format("2006-01-02") }

// vary changes base by up to spread percent, deterministically.
func vary(base int64, spread, seed int) int64 {
	if spread == 0 {
		return base
	}
	pct := (seed*7919+13)%(2*spread+1) - spread
	return max(base+base*int64(pct)/100, 1)
}

// --- accounts and categories ---

func (r *run) seedAccounts(ctx context.Context) error {
	asOf := r.historyStart().AddDate(0, 0, -1)
	for _, a := range accounts {
		owner := finance.OwnerMe
		if a.shared {
			owner = finance.OwnerShared
		}
		created, err := r.Finance.CreateAccount(ctx, finance.CreateAccountInput{
			Name: a.name.in(r.locale), Type: a.typ, Currency: a.currency,
			InitialBalance: a.opening, BalanceAsOf: iso(asOf), Owner: owner,
		})
		if err != nil {
			return fmt.Errorf("create account %q: %w", a.name[0], err)
		}
		r.accounts[a.key] = created
	}
	return nil
}

func (r *run) seedCategories(ctx context.Context) error {
	for _, c := range categories {
		color, icon := c.color, c.icon
		created, err := r.Finance.CreateCategory(ctx, finance.CreateCategoryInput{
			Name: c.name.in(r.locale), Kind: c.kind, Color: &color, Icon: &icon,
		})
		if err != nil {
			return fmt.Errorf("create category %q: %w", c.name[0], err)
		}
		r.categories[c.key] = created
	}
	return nil
}

// --- transactions ---

func (r *run) categoryID(key string) *uuid.UUID {
	if key == "" {
		return nil
	}
	id := r.categories[key].ID
	return &id
}

// addTransaction creates a transaction and tracks credit card spending.
func (r *run) addTransaction(ctx context.Context, in finance.TransactionInput) error {
	created, err := r.Finance.CreateTransaction(ctx, in)
	if err != nil {
		return fmt.Errorf("create transaction %q (%s): %w", in.Description, in.OccurredOn, err)
	}
	r.trackCard(created)
	return nil
}

func (r *run) trackCard(t finance.Transaction) {
	if t.Type != "expense" || t.AccountID != r.accounts[accCard].ID {
		return
	}
	day, err := time.Parse("2006-01-02", t.OccurredOn)
	if err != nil {
		return
	}
	r.cardSpend[r.monthOffset(day)] += t.Amount
}

func (r *run) seedTransactions(ctx context.Context) error {
	for i, spec := range transactions {
		months := spec.months
		if months == nil {
			months = allMonths()
		}
		n := 0
		for _, off := range months {
			for _, day := range spec.days {
				date, ok := r.monthsAgo(off, day)
				if !ok {
					continue
				}
				desc := spec.descs[n%len(spec.descs)]
				if err := r.addTransaction(ctx, finance.TransactionInput{
					Type:        spec.kind,
					AccountID:   r.accounts[spec.account].ID,
					Amount:      vary(spec.amount, spec.spread, off*97+day*13+i*31+n),
					CategoryID:  r.categoryID(spec.category),
					Description: desc.in(r.locale),
					OccurredOn:  iso(date),
				}); err != nil {
					return err
				}
				n++
			}
		}
	}
	for _, t := range transfers {
		months := t.months
		if months == nil {
			months = allMonths()
		}
		for _, off := range months {
			date, ok := r.monthsAgo(off, t.day)
			if !ok {
				continue
			}
			if err := r.addTransfer(ctx, t.from, t.to, t.desc, t.amount, t.toAmount, date); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *run) addTransfer(ctx context.Context, from, to string, desc text, amount, toAmount int64, date time.Time) error {
	in := finance.TransactionInput{
		Type:                 "transfer",
		AccountID:            r.accounts[from].ID,
		Amount:               amount,
		DestinationAccountID: ptr(r.accounts[to].ID),
		Description:          desc.in(r.locale),
		OccurredOn:           iso(date),
	}
	if toAmount != 0 {
		in.DestinationAmount = &toAmount
	}
	return r.addTransaction(ctx, in)
}

// seedCardPayments pays each month's card spending from the checking
// account the next month, so the card balance stays close to the current
// month's spending.
func (r *run) seedCardPayments(ctx context.Context) error {
	for off := historyMonths - 1; off >= 1; off-- {
		spend := r.cardSpend[off]
		date, ok := r.monthsAgo(off-1, cardPaymentDay)
		if spend == 0 || !ok {
			continue
		}
		if err := r.addTransfer(ctx, accChecking, accCard, cardPaymentDesc, spend, 0, date); err != nil {
			return err
		}
	}
	return nil
}

// --- recurring items ---

// recurringStart returns the first due date of a monthly item (five months
// before the target due date) and the due date to leave unpaid, if any.
func (r *run) recurringStart(spec recurringSpec) (start, unpaid time.Time) {
	target := time.Date(r.today.Year(), r.today.Month(), spec.day, 0, 0, 0, 0, time.UTC)
	switch {
	case spec.dueIn > 0:
		// Day 29-31 would clamp in short months; pick a day that does not.
		for k := spec.dueIn; ; k++ {
			if target = r.today.AddDate(0, 0, k); target.Day() <= 28 {
				break
			}
		}
	case spec.overdueBy > 0:
		for k := spec.overdueBy; ; k++ {
			if target = r.today.AddDate(0, 0, -k); target.Day() <= 28 {
				break
			}
		}
		unpaid = target
	}
	start = time.Date(target.Year(), target.Month()-(historyMonths-1), target.Day(), 0, 0, 0, 0, time.UTC)
	return start, unpaid
}

func (r *run) seedRecurring(ctx context.Context) error {
	for _, spec := range recurring {
		start, unpaid := r.recurringStart(spec)
		item, err := r.Finance.CreateRecurringItem(ctx, finance.CreateRecurringItemInput{
			Name: spec.name.in(r.locale), Type: spec.kind, AccountID: r.accounts[spec.account].ID,
			CategoryID: r.categoryID(spec.category), Amount: spec.amount,
			IntervalUnit: finance.UnitMonth, IntervalCount: 1, StartOn: iso(start),
		})
		if err != nil {
			return fmt.Errorf("create recurring item %q: %w", spec.name[0], err)
		}
		schedule := finance.Schedule{Unit: finance.UnitMonth, Count: 1, StartOn: start}
		for _, due := range schedule.DueDatesBetween(start, r.today) {
			if due.Equal(unpaid) || due.Before(r.historyStart()) {
				continue
			}
			paid, err := r.Finance.RegisterRecurringPayment(ctx, item.ID, finance.RecurringPaymentInput{Period: iso(due), Date: iso(due)})
			if err != nil {
				return fmt.Errorf("pay recurring item %q for %s: %w", spec.name[0], iso(due), err)
			}
			r.trackCard(paid)
		}
	}
	return nil
}

// --- groceries ---

// groupNamesES are the Spanish names of the default product groups.
var groupNamesES = map[string]string{
	"Produce":              "Frutas y verduras",
	"Meat & fish":          "Carne y pescado",
	"Dairy & eggs":         "Lácteos y huevos",
	"Bakery":               "Panadería",
	"Pantry":               "Despensa",
	"Frozen":               "Congelados",
	"Snacks & sweets":      "Snacks y dulces",
	"Drinks":               "Bebidas",
	"Alcohol":              "Alcohol",
	"Cleaning & household": "Limpieza y hogar",
	"Personal care":        "Cuidado personal",
	"Other":                "Otros",
}

func (r *run) groupName(english string) string {
	if r.locale == LocaleES {
		return groupNamesES[english]
	}
	return english
}

func productByKey(key string) (productSpec, bool) {
	for _, p := range products {
		if p.key == key {
			return p, true
		}
	}
	return productSpec{}, false
}

func storeIndex(name string) int {
	for i, s := range stores {
		if s == name {
			return i
		}
	}
	return -1
}

func (r *run) rawText(p productSpec) string {
	text := strings.ToUpper(p.name.in(r.locale))
	if p.label != "" {
		text += " " + strings.ToUpper(p.label)
	}
	return text
}

func (r *run) seedGroceries(ctx context.Context) error {
	groups, err := r.Groceries.ListGroups(ctx)
	if err != nil {
		return err
	}
	if r.locale == LocaleES {
		for _, g := range groups {
			name, ok := groupNamesES[g.Name]
			if !ok {
				continue
			}
			if _, err := r.Groceries.UpdateGroup(ctx, g.ID, groceries.UpdateProductGroupInput{Name: &name}); err != nil {
				return fmt.Errorf("rename product group %q: %w", g.Name, err)
			}
		}
	}
	for _, name := range stores {
		if _, err := r.Groceries.CreateStore(ctx, groceries.CreateStoreInput{Name: name}); err != nil {
			return fmt.Errorf("create store %q: %w", name, err)
		}
	}

	sources := []string{"photo", "digital", "manual"}
	for i, spec := range receipts {
		date := r.today.AddDate(0, 0, -spec.daysAgo)
		storeIdx := storeIndex(spec.store)
		lines := make([]groceries.ReceiptLineInput, len(spec.items))
		var total int64
		for j, it := range spec.items {
			p, ok := productByKey(it.product)
			if !ok {
				return fmt.Errorf("receipt %d: unknown product %q", i, it.product)
			}
			amount := vary(int64(float64(p.prices[storeIdx])*it.qty+0.5), 3, i*17+j*5)
			qty := it.qty
			lines[j] = groceries.ReceiptLineInput{RawText: r.rawText(p), Quantity: &qty, Amount: amount}
			total += amount
		}
		storeName := spec.store
		receipt, err := r.Groceries.CreateReceipt(ctx, groceries.CreateReceiptInput{
			StoreName:   &storeName,
			PurchasedOn: iso(date),
			Currency:    currencyEUR,
			Total:       total,
			Source:      sources[i%len(sources)],
			Lines:       lines,
			CreateExpense: &groceries.CreateExpenseInput{
				AccountID:  r.accounts[spec.account].ID.String(),
				CategoryID: ptr(r.categories[catGroceries].ID.String()),
			},
		})
		if err != nil {
			return fmt.Errorf("create receipt from %s on %s: %w", spec.store, iso(date), err)
		}
		if spec.account == accCard {
			r.cardSpend[r.monthOffset(date)] += total
		}
		assignments := make([]groceries.AssignLineInput, len(spec.items))
		for j, it := range spec.items {
			p, _ := productByKey(it.product)
			a := groceries.AssignLineInput{
				LineID:      receipt.Lines[j].ID.String(),
				Product:     ptr(p.name.in(r.locale)),
				Brand:       ptr(p.brand),
				CompareUnit: ptr(p.unit),
				Group:       ptr(r.groupName(p.group)),
			}
			if p.packSize > 0 {
				a.PackSize = ptr(p.packSize)
				a.PackLabel = ptr(p.label)
			}
			assignments[j] = a
		}
		if _, err := r.Groceries.AssignLines(ctx, receipt.ID, groceries.AssignLinesInput{Assignments: assignments}); err != nil {
			return fmt.Errorf("assign receipt lines: %w", err)
		}
	}
	return nil
}

func (r *run) seedPriceChecks(ctx context.Context) error {
	storeList, err := r.Groceries.ListStores(ctx, false)
	if err != nil {
		return err
	}
	storeIDs := map[string]uuid.UUID{}
	for _, s := range storeList {
		storeIDs[s.Name] = s.ID
	}
	variants, err := r.variantIDs(ctx)
	if err != nil {
		return err
	}
	for _, c := range priceChecks {
		p, ok := productByKey(c.product)
		if !ok || storeIndex(c.store) < 0 {
			return fmt.Errorf("price check: unknown product %q or store %q", c.product, c.store)
		}
		variantID, ok := variants[p.name.in(r.locale)]
		if !ok {
			return fmt.Errorf("price check: product %q has no variant", p.name[0])
		}
		in := groceries.CreatePriceCheckInput{
			StoreID:    storeIDs[c.store].String(),
			VariantID:  variantID.String(),
			ObservedOn: iso(r.today.AddDate(0, 0, -c.daysAgo)),
			PriceMinor: p.prices[storeIndex(c.store)],
			Currency:   currencyEUR,
		}
		if note := c.note.in(r.locale); note != "" {
			in.Note = &note
		}
		if _, err := r.Groceries.CreatePriceCheck(ctx, in); err != nil {
			return fmt.Errorf("create price check for %q: %w", p.name[0], err)
		}
	}
	return nil
}

// variantIDs maps product names to the id of their variant.
func (r *run) variantIDs(ctx context.Context) (map[string]uuid.UUID, error) {
	list, err := r.Groceries.ListProducts(ctx, groceries.ListProductsParams{})
	if err != nil {
		return nil, err
	}
	out := map[string]uuid.UUID{}
	for _, p := range list {
		variants, err := r.Groceries.ListVariants(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		if len(variants) > 0 {
			out[p.Name] = variants[0].ID
		}
	}
	return out, nil
}

// --- budgets ---

func (r *run) seedBudgets(ctx context.Context) error {
	oldest := r.historyStart().Format("2006-01")
	items := make([]finance.BudgetItemInput, len(budgets))
	for i, b := range budgets {
		amount := b.amount
		items[i] = finance.BudgetItemInput{CategoryID: r.categories[b.category].ID, Currency: currencyEUR, Amount: &amount}
	}
	if _, err := r.Finance.SetBudgets(ctx, oldest, items); err != nil {
		return fmt.Errorf("set budgets: %w", err)
	}
	r.summary.Budgets = len(budgets)

	// Adjust this month from what is spent so far: one category over its
	// budget, one close to it.
	current := r.today.Format("2006-01")
	status, err := r.Finance.BudgetStatus(ctx, current, finance.OwnerFilter{})
	if err != nil {
		return fmt.Errorf("read budget status: %w", err)
	}
	spent := map[uuid.UUID]int64{}
	for _, c := range status.Currencies {
		if c.Currency != currencyEUR {
			continue
		}
		for _, line := range c.Categories {
			if line.CategoryID != nil {
				spent[*line.CategoryID] = line.Spent
			}
		}
	}
	var adjust []finance.BudgetItemInput
	if s := spent[r.categories[overBudgetCategory].ID]; s > 0 {
		adjust = append(adjust, finance.BudgetItemInput{CategoryID: r.categories[overBudgetCategory].ID, Currency: currencyEUR, Amount: ptr(max(s*85/100, 1))})
	}
	if s := spent[r.categories[nearBudgetCategory].ID]; s > 0 {
		adjust = append(adjust, finance.BudgetItemInput{CategoryID: r.categories[nearBudgetCategory].ID, Currency: currencyEUR, Amount: ptr(max(s*100/88, 1))})
	}
	if len(adjust) > 0 {
		if _, err := r.Finance.SetBudgets(ctx, current, adjust); err != nil {
			return fmt.Errorf("adjust budgets: %w", err)
		}
	}
	return nil
}

// --- summary ---

func (r *run) count(ctx context.Context) error {
	r.summary.Accounts = len(accounts)
	r.summary.Categories = len(categories)
	r.summary.Recurring = len(recurring)
	r.summary.Stores = len(stores)
	page, err := r.Finance.ListTransactions(ctx, finance.TransactionFilter{Limit: 1})
	if err != nil {
		return err
	}
	r.summary.Transactions = int(page.Total)
	receiptList, err := r.Groceries.ListReceipts(ctx, groceries.ListReceiptsParams{})
	if err != nil {
		return err
	}
	r.summary.Receipts = len(receiptList)
	productList, err := r.Groceries.ListProducts(ctx, groceries.ListProductsParams{})
	if err != nil {
		return err
	}
	r.summary.Products = len(productList)
	checks, err := r.Groceries.ListPriceChecks(ctx, groceries.ListPriceChecksParams{})
	if err != nil {
		return err
	}
	r.summary.PriceChecks = len(checks)
	return nil
}

func ptr[T any](v T) *T { return &v }
