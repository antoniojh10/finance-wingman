package httpapi

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
)

// seedExport fills the owner's workspace with one of everything and returns
// the ids needed to check the export.
func seedExport(t *testing.T, api *testAPI) (account, savings finance.Account, category finance.Category) {
	t.Helper()
	account = api.createAccount("Checking", "MXN", 150050)
	savings = api.createAccount("Savings USD", "USD", 0)
	category = api.createCategory("Food", "expense")
	api.createTransaction(map[string]any{
		"type": "expense", "account_id": account.ID, "amount": 12345, "category_id": category.ID,
		"description": "=HYPERLINK(\"http://evil.test\")", "occurred_on": "2026-01-15",
	})
	api.createTransaction(map[string]any{
		"type": "transfer", "account_id": account.ID, "amount": 20000,
		"destination_account_id": savings.ID, "destination_amount": 1100, "occurred_on": "2026-01-16",
	})
	api.createRecurring(map[string]any{
		"name": "Rent", "type": "expense", "account_id": account.ID, "amount": 800000,
		"interval_unit": "month", "start_on": "2026-01-01", "category_id": category.ID,
	})
	api.setBudgets("2026-01", budgetItem(category, "MXN", 50000))
	return account, savings, category
}

type exportDoc struct {
	SchemaVersion int    `json:"schema_version"`
	ExportedAt    string `json:"exported_at"`
	Workspace     struct{ ID, Name string }
	Members       []map[string]any `json:"members"`
	Accounts      []map[string]any `json:"accounts"`
	Categories    []map[string]any `json:"categories"`
	Recurring     []map[string]any `json:"recurring_items"`
	Budgets       []map[string]any `json:"budgets"`
	Transactions  []map[string]any `json:"transactions"`
}

func TestExportJSON(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account, savings, _ := seedExport(t, api)

	res := api.do(http.MethodGet, "/api/v1/export?format=json", nil).expect(http.StatusOK)
	var doc exportDoc
	if err := json.Unmarshal(res.Body, &doc); err != nil {
		t.Fatalf("export is not valid JSON: %v\n%s", err, res.Body)
	}
	if doc.SchemaVersion != ExportSchemaVersion || doc.ExportedAt == "" {
		t.Fatalf("unexpected header: %+v", doc)
	}
	if doc.Workspace.ID != api.workspace.ID.String() || doc.Workspace.Name != "Home" {
		t.Fatalf("unexpected workspace: %+v", doc.Workspace)
	}
	if len(doc.Members) != 1 || doc.Members[0]["email"] != "owner@example.com" || doc.Members[0]["role"] != "owner" {
		t.Fatalf("unexpected members: %+v", doc.Members)
	}
	if len(doc.Accounts) != 2 || len(doc.Categories) != 1 || len(doc.Recurring) != 1 || len(doc.Budgets) != 1 || len(doc.Transactions) != 2 {
		t.Fatalf("unexpected counts: %s", res.Body)
	}

	// Amounts are decimals, not minor units.
	byName := map[string]map[string]any{}
	for _, a := range doc.Accounts {
		byName[a["name"].(string)] = a
	}
	checking := byName["Checking"]
	if checking["initial_balance"] != 1500.5 || checking["currency"] != "MXN" || checking["id"] != account.ID.String() {
		t.Fatalf("unexpected account: %+v", checking)
	}
	if !strings.Contains(string(res.Body), `"initial_balance":1500.50`) {
		t.Fatalf("decimals must keep their scale: %s", res.Body)
	}
	if byName["Savings USD"]["id"] != savings.ID.String() {
		t.Fatalf("unexpected savings: %+v", byName["Savings USD"])
	}

	// Transactions come newest first; the transfer carries both sides.
	transfer, expense := doc.Transactions[0], doc.Transactions[1]
	if transfer["type"] != "transfer" || transfer["amount"] != 200.0 || transfer["destination_amount"] != 11.0 ||
		transfer["destination_currency"] != "USD" || transfer["destination_account_name"] != "Savings USD" {
		t.Fatalf("unexpected transfer: %+v", transfer)
	}
	if expense["amount"] != 123.45 || expense["category_name"] != "Food" || expense["account_name"] != "Checking" ||
		expense["occurred_on"] != "2026-01-15" || expense["destination_amount"] != nil {
		t.Fatalf("unexpected expense: %+v", expense)
	}
	if doc.Recurring[0]["amount"] != 8000.0 || doc.Recurring[0]["name"] != "Rent" || doc.Recurring[0]["category_name"] != "Food" {
		t.Fatalf("unexpected recurring item: %+v", doc.Recurring[0])
	}
	if b := doc.Budgets[0]; b["amount"] != 500.0 || b["month"] != "2026-01" || b["currency"] != "MXN" || b["category_name"] != "Food" {
		t.Fatalf("unexpected budget: %+v", b)
	}
}

func TestExportDefaultsToJSON(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	rec := api.rawGet("/api/v1/export")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("unexpected response: %d %s", rec.Code, rec.Header())
	}
	if d := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(d, "attachment; filename=\"finance-wingman-export-") || !strings.HasSuffix(d, `.json"`) {
		t.Fatalf("unexpected disposition %q", d)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("export must not be cached: %s", rec.Header())
	}
	var doc exportDoc
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("empty export is not valid JSON: %v\n%s", err, rec.Body)
	}
	if len(doc.Transactions) != 0 || len(doc.Accounts) != 0 || len(doc.Members) != 1 {
		t.Fatalf("unexpected empty export: %s", rec.Body)
	}
}

func (a *testAPI) rawGet(path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+a.token)
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	return rec
}

func readZip(t *testing.T, data []byte) map[string][][]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("export is not a valid zip: %v", err)
	}
	files := map[string][][]string{}
	for _, f := range zr.File {
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			t.Fatal(err)
		}
		rows, err := csv.NewReader(bytes.NewReader(raw)).ReadAll()
		if err != nil {
			t.Fatalf("%s is not valid CSV: %v", f.Name, err)
		}
		files[f.Name] = rows
	}
	return files
}

func TestExportCSV(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account, _, _ := seedExport(t, api)

	rec := api.rawGet("/api/v1/export?format=csv")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("unexpected response: %d %s", rec.Code, rec.Header())
	}
	if d := rec.Header().Get("Content-Disposition"); !strings.HasSuffix(d, `.zip"`) {
		t.Fatalf("unexpected disposition %q", d)
	}
	files := readZip(t, rec.Body.Bytes())
	for name, rows := range map[string]int{
		"members.csv": 2, "accounts.csv": 3, "categories.csv": 2, "transactions.csv": 3, "recurring_items.csv": 2, "budgets.csv": 2,
	} {
		if len(files[name]) != rows {
			t.Fatalf("%s: expected %d rows including the header, got %v", name, rows, files[name])
		}
	}
	if len(files) != 6 {
		t.Fatalf("unexpected files: %v", files)
	}

	col := func(file, column string) int {
		for i, h := range files[file][0] {
			if h == column {
				return i
			}
		}
		t.Fatalf("%s has no column %s", file, column)
		return -1
	}
	accounts := files["accounts.csv"]
	var checking []string
	for _, row := range accounts[1:] {
		if row[col("accounts.csv", "id")] == account.ID.String() {
			checking = row
		}
	}
	if checking == nil || checking[col("accounts.csv", "initial_balance")] != "1500.50" || checking[col("accounts.csv", "currency")] != "MXN" {
		t.Fatalf("unexpected accounts: %v", accounts)
	}

	txs := files["transactions.csv"]
	transfer, expense := txs[1], txs[2]
	if transfer[col("transactions.csv", "destination_amount")] != "11.00" || transfer[col("transactions.csv", "amount")] != "200.00" ||
		transfer[col("transactions.csv", "destination_currency")] != "USD" || transfer[col("transactions.csv", "category_id")] != "" {
		t.Fatalf("unexpected transfer row: %v", transfer)
	}
	if expense[col("transactions.csv", "amount")] != "123.45" || expense[col("transactions.csv", "category_name")] != "Food" {
		t.Fatalf("unexpected expense row: %v", expense)
	}
	// Spreadsheets would run a description that starts with "=".
	if got := expense[col("transactions.csv", "description")]; got != "'=HYPERLINK(\"http://evil.test\")" {
		t.Fatalf("formula was not neutralized: %q", got)
	}
	if b := files["budgets.csv"][1]; b[col("budgets.csv", "amount")] != "500.00" || b[col("budgets.csv", "month")] != "2026-01" {
		t.Fatalf("unexpected budget row: %v", b)
	}
	if r := files["recurring_items.csv"][1]; r[col("recurring_items.csv", "amount")] != "8000.00" || r[col("recurring_items.csv", "total_payments")] != "" {
		t.Fatalf("unexpected recurring row: %v", r)
	}
}

// TestExportPagesThroughAllTransactions covers workspaces with more
// transactions than one page.
func TestExportPagesThroughAllTransactions(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account := api.createAccount("Checking", "MXN", 0)
	const n = finance.MaxPageSize*2 + 5
	inputs := make([]finance.TransactionInput, n)
	for i := range inputs {
		inputs[i] = finance.TransactionInput{Type: finance.TypeIncome, AccountID: account.ID, Amount: 100, OccurredOn: "2026-02-01"}
	}
	for start := 0; start < n; start += 100 {
		if _, err := api.svc.CreateTransactions(api.ctx, inputs[start:min(start+100, n)]); err != nil {
			t.Fatal(err)
		}
	}

	var doc exportDoc
	api.do(http.MethodGet, "/api/v1/export", nil).expect(http.StatusOK).decode(&doc)
	seen := map[any]bool{}
	for _, tx := range doc.Transactions {
		seen[tx["id"]] = true
	}
	if len(doc.Transactions) != n || len(seen) != n {
		t.Fatalf("expected %d distinct transactions, got %d (%d distinct)", n, len(doc.Transactions), len(seen))
	}
	files := readZip(t, api.rawGet("/api/v1/export?format=csv").Body.Bytes())
	if len(files["transactions.csv"]) != n+1 {
		t.Fatalf("expected %d CSV rows, got %d", n+1, len(files["transactions.csv"]))
	}
}

func TestExportIsolatedPerWorkspace(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	account, _, category := seedExport(t, api)
	other := api.newUser("other@example.com", "Other")
	other.createAccount("Other checking", "EUR", 0)

	for _, format := range []string{"json", "csv"} {
		rec := other.rawGet("/api/v1/export?format=" + format)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", format, rec.Code)
		}
		body := rec.Body.Bytes()
		if format == "csv" {
			var all strings.Builder
			for name, rows := range readZip(t, body) {
				for _, row := range rows {
					all.WriteString(strings.Join(row, ",") + "\n")
				}
				_ = name
			}
			body = []byte(all.String())
		}
		for _, secret := range []string{account.ID.String(), category.ID.String(), "Food", "Rent", "HYPERLINK", "owner@example.com", "Savings"} {
			if strings.Contains(string(body), secret) {
				t.Fatalf("%s export of another workspace leaks %q", format, secret)
			}
		}
		if !strings.Contains(string(body), "Other checking") || !strings.Contains(string(body), "other@example.com") {
			t.Fatalf("%s export misses the user's own data: %s", format, body)
		}
	}
}

func TestExportAnyMemberCanExport(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	seedExport(t, api)
	base := "/api/v1/workspaces/" + api.workspace.ID.String()
	api.do(http.MethodPost, base+"/invitations", map[string]any{"email": "bob@example.com"}).expect(http.StatusCreated)
	var session auth.Session
	api.as("").do(http.MethodPost, "/api/v1/invitations/accept", map[string]any{"token": api.mail.LastInvitationToken(t)}).
		expect(http.StatusOK).decode(&session)

	var doc exportDoc
	api.as(session.Token).do(http.MethodGet, "/api/v1/export", nil).expect(http.StatusOK).decode(&doc)
	if len(doc.Members) != 2 || len(doc.Accounts) != 2 || len(doc.Transactions) != 2 {
		t.Fatalf("a member must get the whole workspace: %+v", doc)
	}
}

func TestExportErrors(t *testing.T) {
	t.Parallel()
	api := newTestAPI(t)
	api.as("").do(http.MethodGet, "/api/v1/export", nil).expectError(http.StatusUnauthorized)
	api.as("nope").do(http.MethodGet, "/api/v1/export?format=csv", nil).expectError(http.StatusUnauthorized)
	api.do(http.MethodGet, "/api/v1/export?format=xml", nil).expectError(http.StatusUnprocessableEntity)
}
