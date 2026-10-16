package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/groceries"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/money"
)

// --- Tool inputs and outputs ---

type receiptLineArgs struct {
	Text     string   `json:"text" jsonschema:"The line exactly as printed on the receipt, e.g. LECHE ENT. HACENDADO 1L. It is how the store's lines are recognised next time"`
	Quantity *float64 `json:"quantity,omitempty" jsonschema:"Units or weight bought (up to 3 decimals). Defaults to 1"`
	Amount   float64  `json:"amount" jsonschema:"What was paid for the line after any discount, as a decimal in the receipt currency, e.g. 1.25"`
	Discount *float64 `json:"discount,omitempty" jsonschema:"Discount already subtracted from amount, as a positive decimal. Optional"`
}

type addReceiptArgs struct {
	Store         string            `json:"store" jsonschema:"The store chain (not the branch), e.g. Mercadona. Use an existing name from list_grocery_catalog"`
	NewStore      bool              `json:"new_store,omitempty" jsonschema:"Set to true only after the user confirms this is a store that does not exist yet"`
	Date          string            `json:"date" jsonschema:"Purchase date in YYYY-MM-DD format, as printed on the receipt"`
	Total         float64           `json:"total" jsonschema:"Receipt total as a decimal in the receipt currency, e.g. 54.30"`
	Currency      string            `json:"currency,omitempty" jsonschema:"ISO 4217 code. Optional when account or transaction_id is given (their currency is used)"`
	Lines         []receiptLineArgs `json:"lines,omitempty" jsonschema:"The receipt lines in printed order (up to 300)"`
	TransactionID string            `json:"transaction_id,omitempty" jsonschema:"Link an existing expense (id from list_transactions). Do not combine with account"`
	Account       string            `json:"account,omitempty" jsonschema:"Create the expense in this account (name or ID, as in add_expense) and link it. Do not combine with transaction_id. Without either, the receipt is stored without a transaction"`
	Category      string            `json:"category,omitempty" jsonschema:"Expense category for the created expense (only with account). Defaults to the Groceries category when it exists"`
	Source        string            `json:"source,omitempty" jsonschema:"How the receipt was entered: photo, digital or manual. Defaults to manual"`
}

type receiptLineOut struct {
	Position  int     `json:"position" jsonschema:"1-based line number, used by assign_receipt_lines"`
	Text      string  `json:"text"`
	Quantity  float64 `json:"quantity"`
	Amount    string  `json:"amount"`
	Discount  string  `json:"discount,omitempty"`
	Match     string  `json:"match,omitempty" jsonschema:"alias (recognised from an earlier receipt), given or none (needs a product)"`
	Product   string  `json:"product,omitempty"`
	Brand     string  `json:"brand,omitempty"`
	PackLabel string  `json:"pack_label,omitempty"`
}

type receiptWarningOut struct {
	Code    string `json:"code" jsonschema:"total_mismatch or lines_mismatch"`
	Message string `json:"message"`
}

type receiptOut struct {
	ID              string              `json:"id"`
	Store           string              `json:"store"`
	Date            string              `json:"date"`
	Currency        string              `json:"currency"`
	Total           string              `json:"total"`
	Source          string              `json:"source"`
	TransactionID   string              `json:"transaction_id,omitempty"`
	LinesTotal      string              `json:"lines_total"`
	UnassignedCount int                 `json:"unassigned_count" jsonschema:"Lines without a product yet"`
	Warnings        []receiptWarningOut `json:"warnings"`
	Lines           []receiptLineOut    `json:"lines"`
	Hint            string              `json:"hint,omitempty"`
}

type assignmentArgs struct {
	Line        int      `json:"line" jsonschema:"Line position (1-based) as returned by add_receipt or get_receipt"`
	Product     string   `json:"product" jsonschema:"What the line is, without brand or pack, e.g. Whole milk. Reuse an existing product name from list_grocery_catalog; whole and semi-skimmed milk are different products"`
	Brand       string   `json:"brand,omitempty" jsonschema:"Brand, e.g. Hacendado. Defaults to Generic for unbranded products"`
	PackSize    *float64 `json:"pack_size,omitempty" jsonschema:"Contents of one pack in the product's compare_unit (kg, l or unit), e.g. 1 for a 1 l bottle or 0.5 for 500 g. Omit when sold loose"`
	PackLabel   string   `json:"pack_label,omitempty" jsonschema:"Pack description, e.g. 6x1 l or 4x125 g"`
	CompareUnit string   `json:"compare_unit,omitempty" jsonschema:"kg, l or unit. Required only when the product is new"`
	Group       string   `json:"group,omitempty" jsonschema:"Product group name from list_grocery_catalog (e.g. Produce, Dairy & eggs, Other). Required when the product is new; for an existing product without a group it sets it, and a different group is reported but not applied (use update_grocery_product)"`
}

type assignReceiptLinesArgs struct {
	ReceiptID   string           `json:"receipt_id" jsonschema:"Receipt id returned by add_receipt or list_receipts"`
	Assignments []assignmentArgs `json:"assignments" jsonschema:"One entry per line to assign (up to 300). All apply or none does"`
}

type listCatalogArgs struct {
	Query string `json:"query,omitempty" jsonschema:"Optional text to filter products by name or brand, case-insensitive"`
}

type catalogVariantOut struct {
	Brand     string   `json:"brand"`
	PackSize  *float64 `json:"pack_size,omitempty"`
	PackLabel string   `json:"pack_label,omitempty"`
}

type catalogProductOut struct {
	Name        string              `json:"name"`
	Group       string              `json:"group,omitempty"`
	CompareUnit string              `json:"compare_unit"`
	Variants    []catalogVariantOut `json:"variants"`
}

type catalogGroupOut struct {
	Name   string `json:"name"`
	IsFood bool   `json:"is_food"`
}

type catalogOut struct {
	Stores   []string            `json:"stores"`
	Groups   []catalogGroupOut   `json:"groups"`
	Products []catalogProductOut `json:"products"`
}

type listReceiptsArgs struct {
	Month          string `json:"month,omitempty" jsonschema:"Optional month in YYYY-MM format"`
	Store          string `json:"store,omitempty" jsonschema:"Optional store name"`
	UnassignedOnly bool   `json:"unassigned_only,omitempty" jsonschema:"Only receipts that still have lines without a product"`
}

type receiptSummaryOut struct {
	ID              string `json:"id"`
	Store           string `json:"store"`
	Date            string `json:"date"`
	Currency        string `json:"currency"`
	Total           string `json:"total"`
	Source          string `json:"source"`
	TransactionID   string `json:"transaction_id,omitempty"`
	LineCount       int    `json:"line_count"`
	UnassignedCount int    `json:"unassigned_count"`
}

type receiptsOut struct {
	Receipts []receiptSummaryOut `json:"receipts"`
}

type getReceiptArgs struct {
	ID string `json:"id" jsonschema:"Receipt id"`
}

func (s *Server) registerGroceriesTools() {
	if s.groc == nil {
		return
	}
	addTool(s, &mcp.Tool{
		Name:  "add_receipt",
		Title: "Add grocery receipt",
		Description: "Record a supermarket receipt read from a photo or typed by the user: store, date, total and lines. " +
			"With account it also creates the expense; with transaction_id it links an existing expense. " +
			"Lines are matched to products through earlier receipts of the same store; the response lists the lines still without a product.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args addReceiptArgs) (*mcp.CallToolResult, receiptOut, error) {
		return s.addReceipt(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:  "assign_receipt_lines",
		Title: "Assign receipt lines to products",
		Description: "Tell which product (and brand, pack, group) receipt lines are. Use it after add_receipt reports unmatched lines, once the user confirmed the proposed names. " +
			"The mapping is remembered for the store, so the same text matches automatically on the next receipt. Products and brands that do not exist yet are created.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args assignReceiptLinesArgs) (*mcp.CallToolResult, receiptOut, error) {
		return s.assignReceiptLines(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "list_grocery_catalog",
		Title:       "List grocery catalog",
		Description: "List the stores, product groups and products (with their brand and pack variants) already known. Check it before proposing product names so existing ones are reused instead of inventing new ones.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listCatalogArgs) (*mcp.CallToolResult, catalogOut, error) {
		return s.listGroceryCatalog(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "list_receipts",
		Title:       "List grocery receipts",
		Description: "List grocery receipts, newest first, with their total and how many lines still lack a product. Optionally filter by month and store.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listReceiptsArgs) (*mcp.CallToolResult, receiptsOut, error) {
		return s.listReceipts(ctx, args)
	})

	addTool(s, &mcp.Tool{
		Name:        "get_receipt",
		Title:       "Get grocery receipt",
		Description: "Show one receipt with all its lines, their products and any warnings.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getReceiptArgs) (*mcp.CallToolResult, receiptOut, error) {
		return s.getReceipt(ctx, args)
	})
}

// --- Handlers ---

// groceriesErr turns a groceries domain error into a plain error the model
// can act on.
func groceriesErr(err error) error {
	var ge *groceries.Error
	if errors.As(err, &ge) {
		return errors.New(ge.Error())
	}
	return friendly(err)
}

func (s *Server) addReceipt(ctx context.Context, args addReceiptArgs) (*mcp.CallToolResult, receiptOut, error) {
	if strings.TrimSpace(args.Date) == "" {
		return nil, receiptOut{}, errors.New("date is required: use the purchase date printed on the receipt (YYYY-MM-DD)")
	}
	txRef := strings.TrimSpace(args.TransactionID)
	accountRef := strings.TrimSpace(args.Account)
	if txRef != "" && accountRef != "" {
		return nil, receiptOut{}, errors.New("pass either transaction_id (link an existing expense) or account (create the expense), not both")
	}
	if accountRef == "" && strings.TrimSpace(args.Category) != "" {
		return nil, receiptOut{}, errors.New("category only applies together with account, because it is the category of the created expense")
	}

	in := groceries.CreateReceiptInput{PurchasedOn: args.Date, Source: args.Source}
	storeName, err := s.resolveStore(ctx, args.Store, args.NewStore)
	if err != nil {
		return nil, receiptOut{}, err
	}
	in.StoreName = &storeName

	currency := strings.ToUpper(strings.TrimSpace(args.Currency))
	switch {
	case accountRef != "":
		account, err := s.resolveAccount(ctx, accountRef, "account")
		if err != nil {
			return nil, receiptOut{}, err
		}
		if currency != "" && currency != account.Currency {
			return nil, receiptOut{}, fmt.Errorf("currency %s does not match account %s, which is in %s; omit currency or use an account in %s", currency, accountLabel(account), account.Currency, currency)
		}
		currency = account.Currency
		expense := groceries.CreateExpenseInput{AccountID: account.ID.String()}
		if strings.TrimSpace(args.Category) != "" {
			category, err := s.resolveCategory(ctx, args.Category, finance.TypeExpense)
			if err != nil {
				return nil, receiptOut{}, err
			}
			id := category.ID.String()
			expense.CategoryID = &id
		}
		in.CreateExpense = &expense
	case txRef != "":
		id, err := uuid.Parse(txRef)
		if err != nil {
			return nil, receiptOut{}, errors.New("transaction_id is not a valid id; take it from list_transactions")
		}
		tx, err := s.finance.GetTransaction(ctx, id)
		if err != nil {
			return nil, receiptOut{}, fmt.Errorf("transaction_id: %w; take the id of an expense from list_transactions", friendly(err))
		}
		if currency != "" && currency != tx.Currency {
			return nil, receiptOut{}, fmt.Errorf("currency %s does not match the transaction, which is in %s", currency, tx.Currency)
		}
		currency = tx.Currency
		linked := id.String()
		in.TransactionID = &linked
	}
	if currency == "" {
		return nil, receiptOut{}, errors.New("currency is required (ISO 4217 code, e.g. EUR) when neither account nor transaction_id is given")
	}
	cur, err := s.findCurrency(ctx, currency)
	if err != nil {
		return nil, receiptOut{}, err
	}
	in.Currency = cur.Code

	total, err := toMinor(args.Total, cur.MinorUnits, cur.Code, "total")
	if err != nil {
		return nil, receiptOut{}, err
	}
	in.Total = total
	for i, l := range args.Lines {
		amount, err := decimalToMinor(l.Amount, cur.MinorUnits, cur.Code, fmt.Sprintf("lines[%d].amount", i))
		if err != nil {
			return nil, receiptOut{}, err
		}
		line := groceries.ReceiptLineInput{RawText: l.Text, Quantity: l.Quantity, Amount: amount}
		if l.Discount != nil {
			discount, err := decimalToMinor(*l.Discount, cur.MinorUnits, cur.Code, fmt.Sprintf("lines[%d].discount", i))
			if err != nil {
				return nil, receiptOut{}, err
			}
			line.Discount = &discount
		}
		in.Lines = append(in.Lines, line)
	}

	receipt, err := s.groc.CreateReceipt(ctx, in)
	if err != nil {
		return nil, receiptOut{}, groceriesErr(err)
	}
	out := receiptToOut(receipt, cur.MinorUnits, s.groupNames(ctx))
	msg := fmt.Sprintf("Recorded receipt from %s on %s for %s %s (id %s)", out.Store, out.Date, out.Total, out.Currency, out.ID)
	if out.TransactionID != "" {
		msg += ", linked to expense " + out.TransactionID
	}
	msg += fmt.Sprintf(". %d of %d lines need a product.", out.UnassignedCount, len(out.Lines))
	for _, w := range out.Warnings {
		msg += "\nWarning: " + w.Message
	}
	if out.Hint != "" {
		msg += "\n" + out.Hint
	}
	return text(msg), out, nil
}

// resolveStore returns the name of an existing store (case-insensitive), or
// the given name when it is new and the user confirmed it (or no store
// exists yet).
func (s *Server) resolveStore(ctx context.Context, ref string, allowNew bool) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", errors.New("store is required: the supermarket chain on the receipt, e.g. Mercadona (see list_grocery_catalog for the known stores)")
	}
	stores, err := s.groc.ListStores(ctx, false)
	if err != nil {
		return "", groceriesErr(err)
	}
	for _, st := range stores {
		if strings.EqualFold(st.Name, ref) || st.ID.String() == ref {
			return st.Name, nil
		}
	}
	if allowNew || len(stores) == 0 {
		return ref, nil
	}
	names := make([]string, len(stores))
	for i, st := range stores {
		names[i] = st.Name
	}
	return "", fmt.Errorf("no store named %q; known stores: %s. A store is a chain, not a branch: use one of them if it is the same chain, or, if the user confirms it is a new store, call again with new_store=true", ref, strings.Join(names, ", "))
}

func (s *Server) assignReceiptLines(ctx context.Context, args assignReceiptLinesArgs) (*mcp.CallToolResult, receiptOut, error) {
	id, err := uuid.Parse(strings.TrimSpace(args.ReceiptID))
	if err != nil {
		return nil, receiptOut{}, errors.New("receipt_id is not a valid id; take it from add_receipt or list_receipts")
	}
	if len(args.Assignments) == 0 {
		return nil, receiptOut{}, errors.New("assignments must contain at least one item")
	}
	receipt, err := s.groc.GetReceipt(ctx, id)
	if err != nil {
		return nil, receiptOut{}, groceriesErr(err)
	}
	byPosition := make(map[int]uuid.UUID, len(receipt.Lines))
	for _, l := range receipt.Lines {
		byPosition[int(l.Position)] = l.ID
	}
	planner, err := s.newGroupPlanner(ctx)
	if err != nil {
		return nil, receiptOut{}, err
	}
	in := groceries.AssignLinesInput{}
	for i := range args.Assignments {
		a := args.Assignments[i]
		lineID, ok := byPosition[a.Line]
		if !ok {
			return nil, receiptOut{}, fmt.Errorf("assignments[%d].line: this receipt has lines 1 to %d, got %d", i, len(receipt.Lines), a.Line)
		}
		if strings.TrimSpace(a.Product) == "" {
			return nil, receiptOut{}, fmt.Errorf("assignments[%d].product is required: name the product (see list_grocery_catalog for existing ones)", i)
		}
		if err := planner.plan(fmt.Sprintf("assignments[%d]", i), a.Product, a.Group); err != nil {
			return nil, receiptOut{}, err
		}
		item := groceries.AssignLineInput{LineID: lineID.String(), Product: &a.Product, PackSize: a.PackSize}
		if a.Brand != "" {
			item.Brand = &a.Brand
		}
		if a.PackLabel != "" {
			item.PackLabel = &a.PackLabel
		}
		if a.CompareUnit != "" {
			item.CompareUnit = &a.CompareUnit
		}
		if a.Group != "" {
			item.Group = &a.Group
		}
		in.Assignments = append(in.Assignments, item)
	}
	updated, err := s.groc.AssignLines(ctx, id, in)
	if err != nil {
		return nil, receiptOut{}, fmt.Errorf("nothing was assigned: %w", s.assignError(ctx, err))
	}
	if err := s.applyGroups(ctx, planner); err != nil {
		return nil, receiptOut{}, fmt.Errorf("the lines were assigned but a group could not be set: %w; use update_grocery_product", err)
	}
	out := receiptToOut(updated, s.minorUnits(ctx, updated.Currency), s.groupNames(ctx))
	msg := fmt.Sprintf("Assigned %d lines. %d of %d lines of this receipt still need a product.", len(in.Assignments), out.UnassignedCount, len(out.Lines))
	for _, n := range planner.notes {
		msg += "\n" + n
	}
	if out.Hint != "" {
		msg += "\n" + out.Hint
	}
	return text(msg), out, nil
}

// assignError adds the valid groups to errors about unknown ones when the
// service message does not list them already.
func (s *Server) assignError(ctx context.Context, err error) error {
	err = groceriesErr(err)
	msg := err.Error()
	if strings.Contains(msg, "unknown product group") && !strings.Contains(msg, "valid groups") {
		if groups, gerr := s.groc.ListGroups(ctx); gerr == nil {
			names := make([]string, len(groups))
			for i, g := range groups {
				names[i] = g.Name
			}
			return fmt.Errorf("%s; valid groups: %s", msg, strings.Join(names, ", "))
		}
	}
	return err
}

func (s *Server) listGroceryCatalog(ctx context.Context, args listCatalogArgs) (*mcp.CallToolResult, catalogOut, error) {
	stores, err := s.groc.ListStores(ctx, false)
	if err != nil {
		return nil, catalogOut{}, groceriesErr(err)
	}
	groups, err := s.groc.ListGroups(ctx)
	if err != nil {
		return nil, catalogOut{}, groceriesErr(err)
	}
	products, err := s.groc.ListProducts(ctx, groceries.ListProductsParams{})
	if err != nil {
		return nil, catalogOut{}, groceriesErr(err)
	}
	query := strings.ToLower(strings.TrimSpace(args.Query))
	out := catalogOut{Stores: []string{}, Groups: []catalogGroupOut{}, Products: []catalogProductOut{}}
	for _, st := range stores {
		out.Stores = append(out.Stores, st.Name)
	}
	for _, g := range groups {
		out.Groups = append(out.Groups, catalogGroupOut{Name: g.Name, IsFood: g.IsFood})
	}
	for _, p := range products {
		variants, err := s.groc.ListVariants(ctx, p.ID)
		if err != nil {
			return nil, catalogOut{}, groceriesErr(err)
		}
		if query != "" && !strings.Contains(strings.ToLower(p.Name), query) {
			matches := false
			for _, v := range variants {
				matches = matches || strings.Contains(strings.ToLower(v.Brand), query)
			}
			if !matches {
				continue
			}
		}
		po := catalogProductOut{Name: p.Name, CompareUnit: p.CompareUnit, Variants: []catalogVariantOut{}}
		if p.GroupName != nil {
			po.Group = *p.GroupName
		}
		for _, v := range variants {
			vo := catalogVariantOut{Brand: v.Brand, PackSize: v.PackSize}
			if v.PackLabel != nil {
				vo.PackLabel = *v.PackLabel
			}
			po.Variants = append(po.Variants, vo)
		}
		out.Products = append(out.Products, po)
	}
	var msg strings.Builder
	fmt.Fprintf(&msg, "%d stores, %d groups, %d products.", len(out.Stores), len(out.Groups), len(out.Products))
	if len(out.Stores) > 0 {
		fmt.Fprintf(&msg, "\nStores: %s.", strings.Join(out.Stores, ", "))
	}
	var food, nonFood []string
	for _, g := range out.Groups {
		if g.IsFood {
			food = append(food, g.Name)
		} else {
			nonFood = append(nonFood, g.Name)
		}
	}
	fmt.Fprintf(&msg, "\nFood groups: %s.\nNon-food groups: %s.", strings.Join(food, ", "), strings.Join(nonFood, ", "))
	if len(out.Products) == 0 {
		msg.WriteString(" No products yet.")
	}
	for _, p := range out.Products {
		group := p.Group
		if group == "" {
			group = "no group"
		}
		fmt.Fprintf(&msg, "\n- %s (%s, per %s)", p.Name, group, p.CompareUnit)
	}
	return text(msg.String()), out, nil
}

func (s *Server) listReceipts(ctx context.Context, args listReceiptsArgs) (*mcp.CallToolResult, receiptsOut, error) {
	params := groceries.ListReceiptsParams{UnassignedOnly: args.UnassignedOnly}
	if m := strings.TrimSpace(args.Month); m != "" {
		if _, err := time.Parse("2006-01", m); err != nil {
			return nil, receiptsOut{}, fmt.Errorf("month %q is not valid; use YYYY-MM, e.g. 2026-10", m)
		}
		params.Month = m
	}
	if ref := strings.TrimSpace(args.Store); ref != "" {
		stores, err := s.groc.ListStores(ctx, true)
		if err != nil {
			return nil, receiptsOut{}, groceriesErr(err)
		}
		names := make([]string, len(stores))
		for i, st := range stores {
			names[i] = st.Name
			if strings.EqualFold(st.Name, ref) || st.ID.String() == ref {
				id := st.ID
				params.StoreID = &id
			}
		}
		if params.StoreID == nil {
			return nil, receiptsOut{}, fmt.Errorf("no store named %q; known stores: %s", ref, strings.Join(names, ", "))
		}
	}
	rows, err := s.groc.ListReceipts(ctx, params)
	if err != nil {
		return nil, receiptsOut{}, groceriesErr(err)
	}
	out := receiptsOut{Receipts: make([]receiptSummaryOut, len(rows))}
	units := map[string]int{}
	for i, r := range rows {
		mu, ok := units[r.Currency]
		if !ok {
			mu = s.minorUnits(ctx, r.Currency)
			units[r.Currency] = mu
		}
		out.Receipts[i] = receiptSummaryOut{
			ID: r.ID.String(), Store: r.StoreName, Date: r.PurchasedOn, Currency: r.Currency, Total: money.Format(r.Total, mu),
			Source: r.Source, LineCount: int(r.LineCount), UnassignedCount: int(r.UnassignedCount),
		}
		if r.TransactionID != nil {
			out.Receipts[i].TransactionID = r.TransactionID.String()
		}
	}
	return text(fmt.Sprintf("%d receipts.", len(out.Receipts))), out, nil
}

func (s *Server) getReceipt(ctx context.Context, args getReceiptArgs) (*mcp.CallToolResult, receiptOut, error) {
	id, err := uuid.Parse(strings.TrimSpace(args.ID))
	if err != nil {
		return nil, receiptOut{}, errors.New("id is not a valid receipt id; take it from list_receipts")
	}
	receipt, err := s.groc.GetReceipt(ctx, id)
	if err != nil {
		return nil, receiptOut{}, groceriesErr(err)
	}
	out := receiptToOut(receipt, s.minorUnits(ctx, receipt.Currency), s.groupNames(ctx))
	msg := fmt.Sprintf("Receipt from %s on %s for %s %s: %d lines, %d without a product.", out.Store, out.Date, out.Total, out.Currency, len(out.Lines), out.UnassignedCount)
	for _, w := range out.Warnings {
		msg += "\nWarning: " + w.Message
	}
	if out.Hint != "" {
		msg += "\n" + out.Hint
	}
	return text(msg), out, nil
}

// minorUnits returns the decimal places of a currency, 2 when unknown.
func (s *Server) minorUnits(ctx context.Context, code string) int {
	if c, err := s.findCurrency(ctx, code); err == nil {
		return c.MinorUnits
	}
	return 2
}

func receiptToOut(r groceries.Receipt, minorUnits int, groupNames string) receiptOut {
	out := receiptOut{
		ID: r.ID.String(), Store: r.StoreName, Date: r.PurchasedOn, Currency: r.Currency,
		Total: money.Format(r.Total, minorUnits), Source: r.Source, LinesTotal: money.Format(r.LinesTotal, minorUnits),
		UnassignedCount: int(r.UnassignedCount), Warnings: []receiptWarningOut{}, Lines: make([]receiptLineOut, len(r.Lines)),
	}
	if r.TransactionID != nil {
		out.TransactionID = r.TransactionID.String()
	}
	var unmatched []string
	for i, l := range r.Lines {
		lo := receiptLineOut{
			Position: int(l.Position), Text: l.RawText, Quantity: l.Quantity, Amount: money.Format(l.Amount, minorUnits), Match: l.Match,
		}
		if l.Discount > 0 {
			lo.Discount = money.Format(l.Discount, minorUnits)
		}
		if l.ProductName != nil {
			lo.Product = *l.ProductName
		}
		if l.Brand != nil {
			lo.Brand = *l.Brand
		}
		if l.PackLabel != nil {
			lo.PackLabel = *l.PackLabel
		}
		if l.VariantID == nil {
			unmatched = append(unmatched, fmt.Sprintf("%d (%s)", l.Position, l.RawText))
		}
		out.Lines[i] = lo
	}
	for _, w := range r.Warnings {
		wo := receiptWarningOut{Code: w.Code}
		switch w.Code {
		case groceries.WarningTotalMismatch:
			if w.ReceiptTotal != nil && w.TransactionAmount != nil {
				wo.Message = fmt.Sprintf("the receipt total (%s) differs from the linked expense (%s); tell the user so they can fix whichever is wrong.",
					money.Format(*w.ReceiptTotal, minorUnits), money.Format(*w.TransactionAmount, minorUnits))
			}
		case groceries.WarningLinesMismatch:
			if w.ReceiptTotal != nil && w.LinesTotal != nil {
				wo.Message = fmt.Sprintf("the lines add up to %s but the receipt total is %s; some lines may be missing or misread (discounts, bags, deposits), review them with the user.",
					money.Format(*w.LinesTotal, minorUnits), money.Format(*w.ReceiptTotal, minorUnits))
			}
		}
		out.Warnings = append(out.Warnings, wo)
	}
	if len(unmatched) > 0 {
		out.Hint = fmt.Sprintf("Lines without a product: %s. Check list_grocery_catalog, propose a product, brand, pack and group for each to the user, and wait for their confirmation before calling assign_receipt_lines. Do not assign guesses without confirmation.", strings.Join(unmatched, "; "))
		if groupNames != "" {
			out.Hint += " A new product needs a group; valid groups: " + groupNames + " (Other is the fallback)."
		}
	}
	return out
}
