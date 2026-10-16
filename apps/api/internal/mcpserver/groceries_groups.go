package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/groceries"
)

// groupPlanner decides, before anything is written, what a requested group
// means for each product: required for new products, applied to existing
// products without one, and reported when it conflicts with the current one.
type groupPlanner struct {
	groups   []groceries.ProductGroup
	products map[string]groceries.Product // by lower-case name
	pending  map[string]string            // new products of this request -> group name
	// set lists existing products that get a group after the write succeeds.
	set   map[string]groceries.ProductGroup
	notes []string
}

func (s *Server) newGroupPlanner(ctx context.Context) (*groupPlanner, error) {
	groups, err := s.groc.ListGroups(ctx)
	if err != nil {
		return nil, groceriesErr(err)
	}
	products, err := s.groc.ListProducts(ctx, groceries.ListProductsParams{})
	if err != nil {
		return nil, groceriesErr(err)
	}
	p := &groupPlanner{
		groups: groups, products: map[string]groceries.Product{}, pending: map[string]string{},
		set: map[string]groceries.ProductGroup{},
	}
	for _, pr := range products {
		p.products[strings.ToLower(pr.Name)] = pr
	}
	return p, nil
}

func (p *groupPlanner) names() string {
	names := make([]string, len(p.groups))
	for i, g := range p.groups {
		names[i] = g.Name
	}
	return strings.Join(names, ", ")
}

func (p *groupPlanner) find(name string) (groceries.ProductGroup, bool) {
	for _, g := range p.groups {
		if strings.EqualFold(g.Name, name) {
			return g, true
		}
	}
	return groceries.ProductGroup{}, false
}

// plan checks one product reference; label names it in errors (e.g.
// "assignments[0]").
func (p *groupPlanner) plan(label, product, group string) error {
	product, group = strings.TrimSpace(product), strings.TrimSpace(group)
	key := strings.ToLower(product)
	var wanted groceries.ProductGroup
	if group != "" {
		g, ok := p.find(group)
		if !ok {
			return fmt.Errorf("%s.group: unknown product group %q; valid groups: %s", label, group, p.names())
		}
		wanted = g
	}
	if existing, ok := p.products[key]; ok {
		if group == "" {
			return nil
		}
		switch {
		case existing.GroupName == nil:
			p.set[key] = wanted
			p.products[key] = withGroup(existing, wanted)
			p.notes = append(p.notes, fmt.Sprintf("Set the group of %s to %s.", existing.Name, wanted.Name))
		case strings.EqualFold(*existing.GroupName, wanted.Name):
		default:
			p.notes = append(p.notes, fmt.Sprintf("Kept %s for %s instead of %s; use update_grocery_product to change it.", *existing.GroupName, existing.Name, wanted.Name))
		}
		return nil
	}
	if current, ok := p.pending[key]; ok {
		if !strings.EqualFold(current, wanted.Name) && group != "" {
			p.notes = append(p.notes, fmt.Sprintf("Kept %s for %s instead of %s; use update_grocery_product to change it.", current, product, wanted.Name))
		}
		return nil
	}
	if group == "" {
		return fmt.Errorf("%s.group is required to create the new product %q: ask the user which group fits and use one of: %s (Other is the fallback). If the product already exists, check its exact name in list_grocery_catalog", label, product, p.names())
	}
	p.pending[key] = wanted.Name
	return nil
}

func withGroup(pr groceries.Product, g groceries.ProductGroup) groceries.Product {
	id, name := g.ID, g.Name
	pr.GroupID, pr.GroupName = &id, &name
	return pr
}

// applyGroups writes the planned groups of existing products.
func (s *Server) applyGroups(ctx context.Context, p *groupPlanner) error {
	for key, g := range p.set {
		id := g.ID.String()
		if _, err := s.groc.UpdateProduct(ctx, p.products[key].ID, groceries.UpdateProductInput{GroupID: &id}); err != nil {
			return groceriesErr(err)
		}
	}
	return nil
}

// groupNames lists the group names for hints; empty when they cannot be read.
func (s *Server) groupNames(ctx context.Context) string {
	groups, err := s.groc.ListGroups(ctx)
	if err != nil {
		return ""
	}
	names := make([]string, len(groups))
	for i, g := range groups {
		names[i] = g.Name
	}
	return strings.Join(names, ", ")
}

// --- update_grocery_product ---

type updateProductArgs struct {
	Product string `json:"product" jsonschema:"Current name of the product (case-insensitive), as in list_grocery_catalog"`
	NewName string `json:"new_name,omitempty" jsonschema:"New name for the product. Optional"`
	Group   string `json:"group,omitempty" jsonschema:"Group to move the product to (a name from list_grocery_catalog). Optional"`
}

type updateProductOut struct {
	Name  string `json:"name"`
	Group string `json:"group,omitempty"`
}

func (s *Server) registerGroceryProductTools() {
	if s.groc == nil {
		return
	}
	addTool(s, &mcp.Tool{
		Name:  "update_grocery_product",
		Title: "Update grocery product",
		Description: "Rename a grocery product and/or change its group. Use it to fix a product's group (assign_receipt_lines and add_price_check keep the group a product already has). " +
			"Confirm the change with the user first. Brands, packs and receipt lines are untouched.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args updateProductArgs) (*mcp.CallToolResult, updateProductOut, error) {
		return s.updateGroceryProduct(ctx, args)
	})
}

func (s *Server) updateGroceryProduct(ctx context.Context, args updateProductArgs) (*mcp.CallToolResult, updateProductOut, error) {
	name := strings.TrimSpace(args.Product)
	newName := strings.TrimSpace(args.NewName)
	group := strings.TrimSpace(args.Group)
	if name == "" {
		return nil, updateProductOut{}, errors.New("product is required: the current name of the product (see list_grocery_catalog)")
	}
	if newName == "" && group == "" {
		return nil, updateProductOut{}, errors.New("nothing to change: pass new_name and/or group")
	}
	p, err := s.newGroupPlanner(ctx)
	if err != nil {
		return nil, updateProductOut{}, err
	}
	product, ok := p.products[strings.ToLower(name)]
	if !ok {
		suggestions, _ := s.groc.SuggestProducts(ctx, name)
		msg := fmt.Sprintf("no product named %q", name)
		if len(suggestions) > 0 {
			msg += "; similar products: " + strings.Join(suggestions, ", ")
		}
		return nil, updateProductOut{}, errors.New(msg + ". Check list_grocery_catalog for the exact name")
	}
	in := groceries.UpdateProductInput{}
	if newName != "" {
		in.Name = &newName
	}
	if group != "" {
		g, ok := p.find(group)
		if !ok {
			return nil, updateProductOut{}, fmt.Errorf("unknown product group %q; valid groups: %s", group, p.names())
		}
		id := g.ID.String()
		in.GroupID = &id
	}
	updated, err := s.groc.UpdateProduct(ctx, product.ID, in)
	if err != nil {
		var ge *groceries.Error
		if errors.As(err, &ge) && strings.Contains(ge.Error(), "already exists") {
			return nil, updateProductOut{}, fmt.Errorf("cannot rename %s to %q: %s; pick another name or use the existing product", product.Name, newName, ge.Error())
		}
		return nil, updateProductOut{}, groceriesErr(err)
	}
	out := updateProductOut{Name: updated.Name}
	groupText := "no group"
	if updated.GroupName != nil {
		out.Group = *updated.GroupName
		groupText = "group " + out.Group
	}
	return text(fmt.Sprintf("Updated %s: now %s, %s.", product.Name, updated.Name, groupText)), out, nil
}
