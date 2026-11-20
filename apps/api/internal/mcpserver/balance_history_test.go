package mcpserver

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

func dayOffset(offset int) string {
	return time.Now().UTC().AddDate(0, 0, offset).Format(time.DateOnly)
}

func (h *harness) incomeOn(account finance.Account, date string, amount int64) {
	h.t.Helper()
	if _, err := h.svc.CreateTransaction(h.ctx, finance.TransactionInput{Type: finance.TypeIncome, AccountID: account.ID, Amount: amount, OccurredOn: date}); err != nil {
		h.t.Fatal(err)
	}
}

func TestGetBalanceHistory(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	card := h.account("Card", "MXN")
	h.incomeOn(card, dayOffset(-3), 100000)
	h.expenseOn(card, nil, dayOffset(-1), 2500)

	var out balanceHistoryOut
	text := h.mustCall("get_balance_history", map[string]any{"from": dayOffset(-3), "to": dayOffset(-1)}, &out)
	if len(out.Currencies) != 1 || out.Accounts != nil {
		t.Fatalf("unexpected output: %+v", out)
	}
	mxn := out.Currencies[0]
	if mxn.Currency != "MXN" || len(mxn.Days) != 3 || len(mxn.Projection) != 0 {
		t.Fatalf("unexpected currency: %+v", mxn)
	}
	if mxn.Days[0].Balance != "1000.00" || mxn.Days[0].Income != "1000.00" || mxn.Days[2].Balance != "975.00" || mxn.Days[2].Expense != "25.00" {
		t.Fatalf("unexpected days: %+v", mxn.Days)
	}
	hd := mxn.Summary
	if hd == nil || hd.StartBalance != "1000.00" || hd.EndBalance != "975.00" || hd.MinBalance != "975.00" || hd.MinDate != dayOffset(-1) ||
		hd.MaxBalance != "1000.00" || hd.MaxDate != dayOffset(-3) || hd.ProjectedEndBalance != nil {
		t.Fatalf("unexpected header: %+v", hd)
	}
	if !strings.Contains(text, "MXN") || !strings.Contains(text, "lowest 975.00") {
		t.Fatalf("unexpected text: %s", text)
	}
}

func TestGetBalanceHistoryDefaultRangeAndProjection(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	card := h.account("Card", "MXN")
	today := time.Now().UTC()
	first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1)
	h.incomeOn(card, dayOffset(-2), 100000)
	h.expenseOn(card, nil, dayOffset(-1), 3000)

	var out balanceHistoryOut
	h.mustCall("get_balance_history", nil, &out)
	if out.From != first.Format(time.DateOnly) || out.To != last.Format(time.DateOnly) {
		t.Fatalf("the default range is the current month: %s..%s", out.From, out.To)
	}
	mxn := out.Currencies[0]
	if len(mxn.Days) != today.Day() {
		t.Fatalf("days should run up to today: %d", len(mxn.Days))
	}
	wantProjected := last.After(today.Truncate(24 * time.Hour))
	if wantProjected {
		if len(mxn.Projection) != last.Day()-today.Day() || mxn.Summary.ProjectedEndBalance == nil || mxn.ProjectionBasis.AverageDailyExpense != "1.00" {
			t.Fatalf("expected a projection: %+v", mxn)
		}
		if mxn.Projection[len(mxn.Projection)-1].Date != out.To {
			t.Fatalf("projection must reach the end of the range: %+v", mxn.Projection)
		}
	}
}

func TestGetBalanceHistoryProjectionBasis(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	card := h.account("Card", "MXN")
	h.incomeOn(card, dayOffset(-1), 100000)
	h.mustCall("create_recurring", map[string]any{
		"name": "Gym", "type": "expense", "amount": 50, "frequency": "month", "start_date": dayOffset(3),
	}, nil)

	var out balanceHistoryOut
	text := h.mustCall("get_balance_history", map[string]any{"from": dayOffset(-1), "to": dayOffset(5)}, &out)
	mxn := out.Currencies[0]
	if len(mxn.Days) != 2 || len(mxn.Projection) != 5 {
		t.Fatalf("unexpected series: %+v", mxn)
	}
	if len(mxn.ProjectionBasis.Recurring) != 1 || mxn.ProjectionBasis.Recurring[0].Name != "Gym" ||
		mxn.ProjectionBasis.Recurring[0].Amount != "-50.00" || mxn.ProjectionBasis.Recurring[0].AppliedOn != dayOffset(3) {
		t.Fatalf("unexpected basis: %+v", mxn.ProjectionBasis)
	}
	if mxn.Projection[1].Balance != "1000.00" || mxn.Projection[2].Balance != "950.00" || *mxn.Summary.ProjectedEndBalance != "950.00" || mxn.Summary.MinBalance != "950.00" {
		t.Fatalf("unexpected projection: %+v %+v", mxn.Projection, mxn.Summary)
	}
	if !strings.Contains(text, "Projected balance on "+dayOffset(5)+": 950.00") {
		t.Fatalf("unexpected text: %s", text)
	}
}

func TestGetBalanceHistoryByAccount(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	card := h.account("Card", "MXN")
	cash := h.account("Cash", "EUR")
	h.incomeOn(card, dayOffset(-1), 1000)
	h.incomeOn(cash, dayOffset(-1), 500)

	var out balanceHistoryOut
	h.mustCall("get_balance_history", map[string]any{"from": dayOffset(-1), "to": dayOffset(0), "by_account": true}, &out)
	if len(out.Currencies) != 2 || len(out.Accounts) != 2 {
		t.Fatalf("expected two currencies and accounts: %+v", out)
	}
	for _, a := range out.Accounts {
		if len(a.Days) != 2 || (a.Name == "Card" && a.Days[1].Balance != "10.00") || (a.Name == "Cash" && a.Days[1].Balance != "5.00") {
			t.Fatalf("unexpected account series: %+v", a)
		}
	}
}

func TestGetBalanceHistoryOwnerFilter(t *testing.T) {
	t.Parallel()
	h, luis, _ := ownersHarness(t)
	mine := h.ownedAccount("Mine", luis)
	joint := h.ownedAccount("Joint", uuid.Nil)
	h.incomeOn(mine, dayOffset(-1), 1000)
	h.incomeOn(joint, dayOffset(-1), 4000)

	for owner, want := range map[string]string{"": "50.00", "me": "10.00", "shared": "40.00"} {
		var out balanceHistoryOut
		h.mustCall("get_balance_history", map[string]any{"owner": owner, "from": dayOffset(-1), "to": dayOffset(-1)}, &out)
		if got := out.Currencies[0].Days[0].Balance; got != want {
			t.Fatalf("owner %q: balance %s, want %s", owner, got, want)
		}
	}
	h.mustFail("get_balance_history", map[string]any{"owner": "nobody"}, "no workspace member matches")
}

func TestGetBalanceHistoryErrors(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.account("Card", "MXN")
	h.mustFail("get_balance_history", map[string]any{"from": dayOffset(-200), "to": dayOffset(0)}, "use a range of at most 92 days")
	h.mustFail("get_balance_history", map[string]any{"from": dayOffset(0), "to": dayOffset(-1)}, "must not be before from")
	h.mustFail("get_balance_history", map[string]any{"from": "yesterday"}, "from")
}

func TestGetBalanceHistoryWithoutAccounts(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	var out balanceHistoryOut
	text := h.mustCall("get_balance_history", nil, &out)
	if len(out.Currencies) != 0 || !strings.Contains(text, "No accounts") {
		t.Fatalf("unexpected output: %+v %s", out, text)
	}
}
