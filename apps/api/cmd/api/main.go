// Command api runs the Finance Wingman HTTP API and its database migrations.
//
// Usage:
//
//	api serve            start the HTTP server (default)
//	api migrate up       apply pending migrations
//	api migrate down     roll back the last migration
//	api migrate status   list migrations and their state
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/config"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(os.Args[1:], logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(args []string, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	command := "serve"
	if len(args) > 0 {
		command = args[0]
	}

	switch command {
	case "serve":
		if cfg.MigrateOnStart {
			applied, err := db.MigrateUp(ctx, pool)
			if err != nil {
				return err
			}
			logger.Info("migrations applied", "count", applied)
		}
		return serve(ctx, cfg, logger, httpapi.NewHandler(httpapi.Deps{Logger: logger, DB: pool}))
	case "migrate":
		if len(args) < 2 {
			return errors.New("usage: api migrate <up|down|status>")
		}
		return migrate(ctx, pool, args[1])
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func serve(ctx context.Context, cfg config.Config, logger *slog.Logger, handler http.Handler) error {
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", server.Addr, "env", cfg.Env)
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

func migrate(ctx context.Context, pool *pgxpool.Pool, command string) error {
	m, err := db.NewMigrator(pool)
	if err != nil {
		return err
	}
	defer m.Close()

	switch command {
	case "up":
		applied, err := m.Up(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("applied %d migration(s)\n", applied)
	case "down":
		return m.Down(ctx)
	case "status":
		status, err := m.Status(ctx)
		if err != nil {
			return err
		}
		for _, s := range status {
			fmt.Printf("%-8s %s\n", s.State, s.Source.Path)
		}
	default:
		return fmt.Errorf("unknown migrate command %q", command)
	}
	return nil
}
