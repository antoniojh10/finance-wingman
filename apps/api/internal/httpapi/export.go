package httpapi

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/workspace"
)

// ExportSchemaVersion is the version of the JSON export document. Bump it
// when a field is removed or changes meaning; adding fields is compatible.
const ExportSchemaVersion = 1

const (
	exportFormatJSON = "json"
	exportFormatCSV  = "csv"
)

type exportHandlers struct {
	svc        *finance.Service
	workspaces *workspace.Service
	logger     *slog.Logger
}

type exportInput struct {
	Format string `query:"format" enum:"json,csv" default:"json" doc:"json: one document. csv: a zip with one CSV file per entity"`
}

type exportOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	CacheControl       string `header:"Cache-Control"`
	Body               func(ctx huma.Context)
}

type exportWorkspace struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type exportMember struct {
	Name     string    `json:"name"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

// registerExport adds the data export of the workspace the session acts on.
// Any member can export: members already see and manage all the data.
func registerExport(api huma.API, svc *finance.Service, workspaces *workspace.Service, logger *slog.Logger) {
	h := &exportHandlers{svc: svc, workspaces: workspaces, logger: logger}
	binary := &huma.Schema{Type: "string", Format: "binary"}

	huma.Register(api, huma.Operation{
		OperationID: "export-data",
		Method:      http.MethodGet,
		Path:        apiPrefix + "/export",
		Summary:     "Export all the data of the workspace",
		Description: fmt.Sprintf("Downloads accounts, categories, transactions, recurring items, budgets and member names/emails of the current workspace. Amounts are decimals in the currency of their row, not minor units. format=json returns one document with schema_version %d; format=csv returns a zip with one CSV file per entity (members, accounts, categories, transactions, recurring_items, budgets). Any member can export. The response is streamed.", ExportSchemaVersion),
		Tags:        []string{"Export"},
		Responses: map[string]*huma.Response{
			"200": {
				Description: "The export file",
				Content: map[string]*huma.MediaType{
					"application/json": {Schema: binary},
					"application/zip":  {Schema: binary},
				},
			},
		},
	}, h.export)
}

func (h *exportHandlers) export(ctx context.Context, in *exportInput) (*exportOutput, error) {
	session, ok := sessionFrom(ctx)
	if !ok || session.Workspace == nil {
		return nil, huma.Error409Conflict(errNoWorkspace)
	}
	members, err := h.workspaces.Members(ctx, session.User.ID, session.Workspace.ID)
	if err != nil {
		return nil, toHTTPError(ctx, h.logger, err)
	}
	snap, err := h.svc.ExportSnapshot(ctx)
	if err != nil {
		return nil, toHTTPError(ctx, h.logger, err)
	}
	meta := exportMeta{
		workspace:  exportWorkspace{ID: session.Workspace.ID, Name: session.Workspace.Name},
		members:    make([]exportMember, len(members)),
		snapshot:   snap,
		exportedAt: time.Now().UTC().Truncate(time.Second),
	}
	for i, m := range members {
		meta.members[i] = exportMember{Name: m.Name, Email: m.Email, Role: m.Role, JoinedAt: m.JoinedAt}
	}

	date := h.svc.Today().Format(time.DateOnly)
	out := &exportOutput{CacheControl: "no-store"}
	var write func(w io.Writer, transactions transactionSource) error
	switch in.Format {
	case exportFormatCSV:
		out.ContentType = "application/zip"
		out.ContentDisposition = `attachment; filename="finance-wingman-export-` + date + `.zip"`
		write = meta.writeCSV
	default:
		out.ContentType = "application/json"
		out.ContentDisposition = `attachment; filename="finance-wingman-export-` + date + `.json"`
		write = meta.writeJSON
	}
	out.Body = func(hctx huma.Context) {
		// The status and headers are already sent, so a failure here can only
		// be logged; the truncated file is not valid JSON/zip, so the client
		// can tell the export is incomplete.
		if err := write(hctx.BodyWriter(), func(fn func([]finance.ExportTransaction) error) error {
			return h.svc.ExportTransactions(hctx.Context(), fn)
		}); err != nil {
			h.logger.ErrorContext(hctx.Context(), "export data", "error", err)
		}
	}
	return out, nil
}

// transactionSource feeds pages of transactions to a writer.
type transactionSource func(fn func([]finance.ExportTransaction) error) error

type exportMeta struct {
	workspace  exportWorkspace
	members    []exportMember
	snapshot   finance.ExportSnapshot
	exportedAt time.Time
}

// writeJSON writes one document; transactions come last and are encoded as
// they are read.
func (m exportMeta) writeJSON(w io.Writer, transactions transactionSource) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	field := func(name string, v any) error {
		if _, err := fmt.Fprintf(w, "%s:", strconv.Quote(name)); err != nil {
			return err
		}
		return enc.Encode(v)
	}
	if _, err := io.WriteString(w, "{"); err != nil {
		return err
	}
	for _, f := range []struct {
		name  string
		value any
	}{
		{"schema_version", ExportSchemaVersion},
		{"exported_at", m.exportedAt},
		{"workspace", m.workspace},
		{"members", m.members},
		{"accounts", m.snapshot.Accounts},
		{"categories", m.snapshot.Categories},
		{"recurring_items", m.snapshot.Recurring},
		{"budgets", m.snapshot.Budgets},
	} {
		if err := field(f.name, f.value); err != nil {
			return err
		}
		if _, err := io.WriteString(w, ","); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(w, `"transactions":[`); err != nil {
		return err
	}
	first := true
	err := transactions(func(batch []finance.ExportTransaction) error {
		for _, t := range batch {
			if !first {
				if _, err := io.WriteString(w, ","); err != nil {
					return err
				}
			}
			first = false
			if err := enc.Encode(t); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, "]}\n")
	return err
}

// writeCSV writes a zip with one CSV file per entity.
func (m exportMeta) writeCSV(w io.Writer, transactions transactionSource) error {
	zw := zip.NewWriter(w)
	file := func(name string, header []string) (*csv.Writer, error) {
		f, err := zw.Create(name + ".csv")
		if err != nil {
			return nil, err
		}
		cw := csv.NewWriter(f)
		return cw, cw.Write(header)
	}
	table := func(name string, header []string, rows [][]string) error {
		cw, err := file(name, header)
		if err != nil {
			return err
		}
		if err := cw.WriteAll(rows); err != nil {
			return err
		}
		return cw.Error()
	}

	memberRows := make([][]string, len(m.members))
	for i, x := range m.members {
		memberRows[i] = []string{text(x.Name), x.Email, x.Role, x.JoinedAt.Format(time.RFC3339)}
	}
	if err := table("members", []string{"name", "email", "role", "joined_at"}, memberRows); err != nil {
		return err
	}

	accountRows := make([][]string, len(m.snapshot.Accounts))
	for i, a := range m.snapshot.Accounts {
		accountRows[i] = []string{
			a.ID.String(), text(a.Name), a.Type, a.Currency, string(a.InitialBalance), a.BalanceAsOf,
			string(a.Balance), optional(a.OwnerEmail), strconv.FormatBool(a.Archived),
			a.CreatedAt.Format(time.RFC3339), a.UpdatedAt.Format(time.RFC3339),
		}
	}
	if err := table("accounts", []string{
		"id", "name", "type", "currency", "initial_balance", "balance_as_of", "balance",
		"owner_email", "archived", "created_at", "updated_at",
	}, accountRows); err != nil {
		return err
	}

	categoryRows := make([][]string, len(m.snapshot.Categories))
	for i, c := range m.snapshot.Categories {
		categoryRows[i] = []string{
			c.ID.String(), text(c.Name), c.Kind, optional(c.Color), optional(c.Icon),
			strconv.FormatBool(c.Archived), c.CreatedAt.Format(time.RFC3339), c.UpdatedAt.Format(time.RFC3339),
		}
	}
	if err := table("categories", []string{
		"id", "name", "kind", "color", "icon", "archived", "created_at", "updated_at",
	}, categoryRows); err != nil {
		return err
	}

	cw, err := file("transactions", []string{
		"id", "type", "occurred_on", "account_id", "account_name", "currency", "amount",
		"destination_account_id", "destination_account_name", "destination_currency", "destination_amount",
		"category_id", "category_name", "description", "recurring_id", "recurring_due_on",
		"created_by_email", "created_at", "updated_at",
	})
	if err != nil {
		return err
	}
	err = transactions(func(batch []finance.ExportTransaction) error {
		for _, t := range batch {
			row := []string{
				t.ID.String(), t.Type, t.OccurredOn, t.AccountID.String(), text(t.AccountName), t.Currency, string(t.Amount),
				optional(uuidString(t.DestinationAccountID)), optional(textPtr(t.DestinationAccountName)),
				optional(t.DestinationCurrency), optionalNumber(t.DestinationAmount),
				optional(uuidString(t.CategoryID)), optional(textPtr(t.CategoryName)), text(t.Description),
				optional(uuidString(t.RecurringID)), optional(t.RecurringDueOn),
				optional(t.CreatedByEmail), t.CreatedAt.Format(time.RFC3339), t.UpdatedAt.Format(time.RFC3339),
			}
			if err := cw.Write(row); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}

	recurringRows := make([][]string, len(m.snapshot.Recurring))
	for i, r := range m.snapshot.Recurring {
		total := ""
		if r.TotalPayments != nil {
			total = strconv.Itoa(*r.TotalPayments)
		}
		recurringRows[i] = []string{
			r.ID.String(), text(r.Name), r.Type, r.AccountID.String(), text(r.AccountName), r.Currency,
			optional(uuidString(r.CategoryID)), optional(textPtr(r.CategoryName)), string(r.Amount), text(r.Notes),
			r.IntervalUnit, strconv.Itoa(r.IntervalCount), r.StartOn, total, r.Status,
			r.CreatedAt.Format(time.RFC3339), r.UpdatedAt.Format(time.RFC3339),
		}
	}
	if err := table("recurring_items", []string{
		"id", "name", "type", "account_id", "account_name", "currency", "category_id", "category_name",
		"amount", "notes", "interval_unit", "interval_count", "start_on", "total_payments", "status",
		"created_at", "updated_at",
	}, recurringRows); err != nil {
		return err
	}

	budgetRows := make([][]string, len(m.snapshot.Budgets))
	for i, b := range m.snapshot.Budgets {
		budgetRows[i] = []string{b.CategoryID.String(), text(b.CategoryName), b.Currency, b.Month, optionalNumber(b.Amount)}
	}
	if err := table("budgets", []string{"category_id", "category_name", "currency", "month", "amount"}, budgetRows); err != nil {
		return err
	}
	return zw.Close()
}

// text neutralizes spreadsheet formulas: a cell that starts with =, +, -, @,
// tab or carriage return is evaluated by Excel and Sheets, so free text is
// prefixed with an apostrophe. Numbers and dates are never passed through it.
func text(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func textPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := text(*s)
	return &t
}

func optional(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func optionalNumber(n *json.Number) string {
	if n == nil {
		return ""
	}
	return string(*n)
}

func uuidString(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}
