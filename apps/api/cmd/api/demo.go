package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/demo"
)

// demoCommand runs the `demo` subcommands. `seed` fills a workspace owned by
// an existing user with fictional data. It needs shell access to the server
// and its database; there is deliberately no HTTP equivalent.
func demoCommand(ctx context.Context, seeder *demo.Seeder, out io.Writer, args []string) error {
	const usage = `usage: api demo seed --email EMAIL [--workspace "Demo"] [--locale en|es] [--reset]`
	if len(args) == 0 || args[0] != "seed" {
		return errors.New(usage)
	}
	flags := flag.NewFlagSet("demo seed", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	email := flags.String("email", "", "email of the existing user who owns the workspace")
	name := flags.String("workspace", demo.DefaultWorkspaceName, "name of the workspace to create")
	locale := flags.String("locale", demo.LocaleEN, "language of the seeded text: en or es")
	reset := flags.Bool("reset", false, "delete the user's workspace with this name and seed it again")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() > 0 || *email == "" {
		return errors.New(usage)
	}

	summary, err := seeder.Seed(ctx, demo.Options{Email: *email, Workspace: *name, Locale: *locale, Reset: *reset})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "seeded workspace %q (%s): %d accounts, %d categories, %d transactions, %d budgets, %d recurring items, %d stores, %d receipts, %d products, %d price checks\n",
		*name, summary.WorkspaceID, summary.Accounts, summary.Categories, summary.Transactions, summary.Budgets,
		summary.Recurring, summary.Stores, summary.Receipts, summary.Products, summary.PriceChecks)
	return err
}
