package demo_test

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/demo"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/groceries"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/testutil"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

type fixture struct {
	seeder     *demo.Seeder
	workspaces *workspace.Service
	finance    *finance.Service
	groceries  *groceries.Service
	user       auth.User
}

const ownerEmail = "ana@example.com"

func newFixture(t *testing.T) *fixture {
	t.Helper()
	pool := testutil.NewDatabase(t, true)
	mail := &testutil.MailRecorder{}
	authSvc := auth.NewService(pool, mail, auth.Config{WebBaseURL: "http://web.test"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	workspaces := workspace.NewService(pool, mail, workspace.Config{WebBaseURL: "http://web.test"})
	fin := finance.NewService(pool, nil)
	gro := groceries.NewService(pool, fin)
	user, err := authSvc.AddUser(context.Background(), ownerEmail, "Ana")
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{
		seeder:     &demo.Seeder{Auth: authSvc, Workspaces: workspaces, Finance: fin, Groceries: gro},
		workspaces: workspaces, finance: fin, groceries: gro, user: user,
	}
}

// ctx acts on the workspace as the owner.
func (f *fixture) ctx(id uuid.UUID) context.Context {
	return finance.WithActor(db.WithWorkspace(context.Background(), id), f.user.ID)
}

func (f *fixture) seed(t *testing.T, opts demo.Options) demo.Summary {
	t.Helper()
	if opts.Email == "" {
		opts.Email = ownerEmail
	}
	summary, err := f.seeder.Seed(context.Background(), opts)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return summary
}

func TestSeedCreatesTheWorkspace(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	summary := f.seed(t, demo.Options{})

	want := demo.Summary{Accounts: 6, Categories: 13, Budgets: 8, Recurring: 5, Stores: 3, Receipts: 10, Products: 13, PriceChecks: 8}
	got := summary
	got.WorkspaceID, got.Transactions = uuid.Nil, 0
	if got != want {
		t.Fatalf("unexpected counts:\n got %+v\nwant %+v", got, want)
	}
	t.Logf("seeded transactions: %d", summary.Transactions)
	if summary.Transactions < 100 {
		t.Fatalf("expected months of transactions, got %d", summary.Transactions)
	}

	list, err := f.workspaces.List(context.Background(), f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "Demo" || list[0].Role != workspace.RoleOwner || list[0].ID != summary.WorkspaceID {
		t.Fatalf("the user should own a workspace named Demo, got %+v", list)
	}

	ctx := f.ctx(summary.WorkspaceID)
	accounts, err := f.finance.ListAccounts(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	currencies := map[string]int{}
	var shared, card int
	for _, a := range accounts {
		currencies[a.Currency]++
		if a.Owner == nil {
			shared++
		}
		if a.Type == "credit_card" {
			card++
		}
		if a.Type != "credit_card" && a.Balance <= 0 {
			t.Errorf("account %q should hold money, balance %d", a.Name, a.Balance)
		}
	}
	if currencies["EUR"] != 5 || currencies["MXN"] != 1 || shared != 1 || card != 1 {
		t.Fatalf("unexpected accounts: %+v", accounts)
	}
}

func TestSeedDataIsConsistent(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	summary := f.seed(t, demo.Options{})
	ctx := f.ctx(summary.WorkspaceID)

	today := f.finance.Today().Format("2006-01-02")
	page, err := f.finance.ListTransactions(ctx, finance.TransactionFilter{Limit: finance.MaxPageSize})
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]int{}
	var uncategorized int
	for _, tx := range page.Items {
		types[tx.Type]++
		if tx.OccurredOn > today {
			t.Errorf("transaction %q is dated in the future: %s", tx.Description, tx.OccurredOn)
		}
		if tx.Type != "transfer" && tx.CategoryID == nil {
			uncategorized++
		}
	}
	if types["expense"] == 0 || types["income"] == 0 || types["transfer"] == 0 {
		t.Fatalf("expected expenses, income and transfers, got %v", types)
	}
	if uncategorized == 0 {
		t.Error("expected a few uncategorized transactions")
	}

	// Recurring items: at least one overdue, one due this week, the rest paid.
	upcoming, err := f.finance.UpcomingRecurring(ctx, 7)
	if err != nil {
		t.Fatal(err)
	}
	var overdue, pending int
	for _, u := range upcoming {
		switch u.Status {
		case finance.PeriodOverdue:
			overdue++
		case finance.PeriodPending:
			pending++
		}
	}
	if overdue != 1 || pending < 1 {
		t.Fatalf("expected one overdue and at least one pending item, got %d overdue and %d pending: %+v", overdue, pending, upcoming)
	}

	// The detector finds a pattern that is not tracked yet.
	suggestions, err := f.finance.ListRecurringSuggestions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(suggestions) == 0 {
		t.Error("expected the recurring detector to suggest something")
	}

	// Receipts are linked to expenses and every line has a product.
	receipts, err := f.groceries.ListReceipts(ctx, groceries.ListReceiptsParams{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range receipts {
		if r.TransactionID == nil || r.UnassignedCount != 0 || r.LinesTotal != r.Total {
			t.Errorf("receipt %s should be linked, assigned and add up: %+v", r.PurchasedOn, r)
		}
	}

	// Budgets: this month has an over-budget and a near-budget category when
	// something was spent on them.
	status, err := f.finance.BudgetStatus(ctx, f.finance.Today().Format("2006-01"), finance.OwnerFilter{})
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, c := range status.Currencies {
		for _, line := range c.Categories {
			if c.Currency == "EUR" && line.CategoryName != nil && line.Spent > 0 && line.Amount != nil {
				states[*line.CategoryName] = line.State
			}
		}
	}
	if s, ok := states["Restaurants"]; ok && s != finance.BudgetOver {
		t.Errorf("restaurants should be over budget, got %q", s)
	}
	if s, ok := states["Leisure"]; ok && s != finance.BudgetNear {
		t.Errorf("leisure should be near its budget, got %q", s)
	}

	// Store comparison has data.
	checks, err := f.groceries.ListPriceChecks(ctx, groceries.ListPriceChecksParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(checks) != summary.PriceChecks {
		t.Fatalf("expected %d price checks, got %d", summary.PriceChecks, len(checks))
	}
}

func TestSeedRefusesAnExistingWorkspaceWithoutReset(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	first := f.seed(t, demo.Options{})

	_, err := f.seeder.Seed(context.Background(), demo.Options{Email: ownerEmail})
	if err == nil || !strings.Contains(err.Error(), "--reset") {
		t.Fatalf("expected an error telling how to reset, got %v", err)
	}
	list, err := f.workspaces.List(context.Background(), f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != first.WorkspaceID {
		t.Fatalf("the existing workspace must be left alone, got %+v", list)
	}
	page, err := f.finance.ListTransactions(f.ctx(first.WorkspaceID), finance.TransactionFilter{Limit: 1})
	if err != nil || int(page.Total) != first.Transactions {
		t.Fatalf("the existing data must be left alone: %v, %d transactions", err, page.Total)
	}
}

func TestSeedResetReplacesOnlyThatWorkspace(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()

	// Another workspace of the same user, with the same name as a different
	// user's one, and one the user does not own.
	keep, err := f.workspaces.Create(ctx, f.user.ID, "Home")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.finance.CreateAccount(f.ctx(keep.ID), finance.CreateAccountInput{Name: "Mine", Type: "checking", Currency: "EUR", InitialBalance: 12345}); err != nil {
		t.Fatal(err)
	}

	first := f.seed(t, demo.Options{})
	second := f.seed(t, demo.Options{Reset: true})
	third := f.seed(t, demo.Options{Reset: true})

	if second.WorkspaceID == first.WorkspaceID || third.WorkspaceID == second.WorkspaceID {
		t.Fatal("reset should create a new workspace")
	}
	if second != withID(first, second.WorkspaceID) || third != withID(first, third.WorkspaceID) {
		t.Fatalf("resetting should give the same counts:\n first %+v\nsecond %+v\n third %+v", first, second, third)
	}

	list, err := f.workspaces.List(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[uuid.UUID]string{}
	for _, m := range list {
		ids[m.ID] = m.Name
	}
	if len(ids) != 2 || ids[keep.ID] != "Home" || ids[third.WorkspaceID] != "Demo" {
		t.Fatalf("expected only Home and the latest Demo, got %v", ids)
	}
	accounts, err := f.finance.ListAccounts(f.ctx(keep.ID), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].Name != "Mine" || accounts[0].Balance != 12345 {
		t.Fatalf("the other workspace must be untouched, got %+v", accounts)
	}
}

func TestSeedNeverDeletesARealWorkspace(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()

	real, err := f.workspaces.Create(ctx, f.user.ID, "Home")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.finance.CreateAccount(f.ctx(real.ID), finance.CreateAccountInput{Name: "Mine", Type: "checking", Currency: "EUR", InitialBalance: 999}); err != nil {
		t.Fatal(err)
	}

	for _, reset := range []bool{false, true} {
		_, err := f.seeder.Seed(ctx, demo.Options{Email: ownerEmail, Workspace: "Home", Reset: reset})
		if err == nil || !strings.Contains(err.Error(), "not created by the demo seed") {
			t.Fatalf("reset=%v: expected a refusal, got %v", reset, err)
		}
		accounts, err := f.finance.ListAccounts(f.ctx(real.ID), false)
		if err != nil || len(accounts) != 1 || accounts[0].Balance != 999 {
			t.Fatalf("reset=%v: the real workspace must be intact: %v %+v", reset, err, accounts)
		}
	}
	list, err := f.workspaces.List(ctx, f.user.ID)
	if err != nil || len(list) != 1 || list[0].ID != real.ID {
		t.Fatalf("no workspace may be created or deleted: %v %+v", err, list)
	}
}

func withID(s demo.Summary, id uuid.UUID) demo.Summary {
	s.WorkspaceID = id
	return s
}

func TestSeedResetOnlyTouchesWorkspacesOwnedByTheUser(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()

	// Somebody else's workspace is named Demo too, and the user is just a
	// member of it.
	other, err := f.seeder.Auth.AddUser(ctx, "bob@example.com", "Bob")
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := f.workspaces.Create(ctx, other.ID, "Demo")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.finance.CreateAccount(finance.WithActor(db.WithWorkspace(ctx, theirs.ID), other.ID), finance.CreateAccountInput{Name: "Theirs", Type: "checking", Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}

	f.seed(t, demo.Options{Reset: true})

	list, err := f.workspaces.List(ctx, other.ID)
	if err != nil || len(list) != 1 || list[0].ID != theirs.ID {
		t.Fatalf("another user's workspace must survive: %v %+v", err, list)
	}
	accounts, err := f.finance.ListAccounts(finance.WithActor(db.WithWorkspace(ctx, theirs.ID), other.ID), false)
	if err != nil || len(accounts) != 1 {
		t.Fatalf("another user's data must survive: %v %+v", err, accounts)
	}
}

func TestSeedCustomNameAndSpanish(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	summary := f.seed(t, demo.Options{Workspace: "  Casa demo ", Locale: "es"})

	list, err := f.workspaces.List(context.Background(), f.user.ID)
	if err != nil || len(list) != 1 || list[0].Name != "Casa demo" {
		t.Fatalf("unexpected workspaces: %v %+v", err, list)
	}
	ctx := f.ctx(summary.WorkspaceID)
	categories, err := f.finance.ListCategories(ctx, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, c := range categories {
		names[c.Name] = true
	}
	if !names["Alquiler"] || !names["Nómina"] || names["Rent"] {
		t.Fatalf("categories should be in Spanish, got %v", names)
	}
	groups, err := f.groceries.ListGroups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var spanish bool
	for _, g := range groups {
		spanish = spanish || g.Name == "Lácteos y huevos"
	}
	if !spanish {
		t.Fatalf("product groups should be in Spanish, got %+v", groups)
	}
	if summary.Receipts != 10 || summary.Products != 13 {
		t.Fatalf("unexpected counts %+v", summary)
	}
}

func TestSeedErrors(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := context.Background()

	for name, opts := range map[string]demo.Options{
		"unknown email":      {Email: "nobody@example.com"},
		"empty email":        {},
		"unsupported locale": {Email: ownerEmail, Locale: "fr"},
	} {
		if _, err := f.seeder.Seed(ctx, opts); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	list, err := f.workspaces.List(ctx, f.user.ID)
	if err != nil || len(list) != 0 {
		t.Fatalf("failed runs must not create workspaces: %v %+v", err, list)
	}
}
