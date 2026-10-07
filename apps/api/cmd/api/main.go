// Command api runs the Finance Wingman HTTP API and its maintenance tasks.
//
// Usage:
//
//	api serve                     start the HTTP server, OAuth and MCP (default)
//	api openapi                   print the REST API OpenAPI document
//	api migrate up|down|status    manage database migrations
//	api users list                list users with access
//	api users add EMAIL [NAME]    grant access to an email address
//	api users remove EMAIL        revoke access
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/antoniojh10/finance-wingman/apps/api/internal/auth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/config"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/db"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/finance"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/httpapi"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/mail"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/mcpserver"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/oauth"
	"github.com/antoniojh10/finance-wingman/apps/api/internal/telemetry"
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

	// Commands that need neither configuration nor a database.
	if len(args) > 0 && args[0] == "openapi" {
		return writeOpenAPI(os.Stdout)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	shutdownTelemetry, err := telemetry.Setup(ctx, os.Getenv, httpapi.Version)
	if err != nil {
		return fmt.Errorf("set up telemetry: %w", err)
	}
	// Emails printed by the log sender contain magic links; they only go to
	// stdout, never to the telemetry backend.
	mailLogger := logger
	if telemetry.Enabled(os.Getenv) {
		logger = slog.New(telemetry.LogHandler(logger.Handler(), slog.LevelInfo))
	}
	defer func() {
		// The signal context is already done here; give exporters time to flush.
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTelemetry(flushCtx); err != nil {
			logger.Warn("flush telemetry", "error", err)
		}
	}()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	authSvc := auth.NewService(pool, newMailSender(cfg.Email, mailLogger), auth.Config{WebBaseURL: cfg.WebBaseURL, MaxChallengesPerHour: cfg.LoginEmailsPerHour}, logger)

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
		for _, u := range cfg.InitialUsers {
			if _, err := authSvc.AddUser(ctx, u.Email, u.Name); err != nil {
				return fmt.Errorf("add initial user %q: %w", u.Email, err)
			}
		}
		financeSvc := finance.NewService(pool, cfg.Location)
		oauthSrv := oauth.NewServer(pool, authSvc, oauth.Config{Issuer: cfg.PublicURL}, logger)
		mcpHandler := mcpserver.New(financeSvc, httpapi.Version).Handler(authSvc, cfg.PublicURL, oauthSrv.ResourceMetadataURL(), logger)
		go purgeExpiredPeriodically(ctx, logger, authSvc.PurgeExpired, oauthSrv.PurgeExpired)
		if cfg.PprofAddr != "" {
			go servePprof(ctx, cfg.PprofAddr, logger)
		}
		logger.Info("telemetry", "otlp_export", telemetry.Enabled(os.Getenv))
		handler := httpapi.NewHandler(httpapi.Deps{
			Logger:  logger,
			DB:      pool,
			Auth:    authSvc,
			Finance: financeSvc,
			OAuth:   oauthSrv,
			MCP:     mcpHandler,
		})
		return serve(ctx, cfg, logger, handler)
	case "migrate":
		if len(args) < 2 {
			return errors.New("usage: api migrate <up|down|status>")
		}
		return migrate(ctx, pool, args[1])
	case "users":
		return users(ctx, authSvc, args[1:])
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

// writeOpenAPI prints the REST API's OpenAPI document. Services are only
// needed to register routes, so zero values are enough.
func writeOpenAPI(w io.Writer) error {
	spec, err := httpapi.OpenAPI(httpapi.Deps{
		Logger:  slog.New(slog.DiscardHandler),
		Auth:    &auth.Service{},
		Finance: &finance.Service{},
	})
	if err != nil {
		return err
	}
	_, err = w.Write(spec)
	return err
}

func newMailSender(cfg config.EmailConfig, logger *slog.Logger) mail.Sender {
	switch cfg.Provider {
	case "smtp":
		return mail.SMTPSender{Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername, Password: cfg.SMTPPassword, From: cfg.From}
	case "resend":
		return mail.ResendSender{APIKey: cfg.ResendAPIKey, From: cfg.From}
	default:
		return mail.LogSender{Logger: logger}
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

// servePprof exposes the runtime profiler on its own listener so it is never
// reachable through the public port.
func servePprof(ctx context.Context, addr string, logger *slog.Logger) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()
	logger.Info("pprof listening", "addr", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Warn("pprof server stopped", "error", err)
	}
}

func purgeExpiredPeriodically(ctx context.Context, logger *slog.Logger, purgers ...func(context.Context) error) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		for _, purge := range purgers {
			if err := purge(ctx); err != nil && ctx.Err() == nil {
				logger.Warn("purge expired auth records", "error", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
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

func users(ctx context.Context, svc *auth.Service, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: api users <list|add|remove>")
	}
	switch args[0] {
	case "list":
		list, err := svc.ListUsers(ctx)
		if err != nil {
			return err
		}
		for _, u := range list {
			fmt.Printf("%s\t%s\t%s\n", u.Email, u.Name, u.Locale)
		}
	case "add":
		if len(args) < 2 {
			return errors.New("usage: api users add EMAIL [NAME]")
		}
		u, err := svc.AddUser(ctx, args[1], strings.Join(args[2:], " "))
		if err != nil {
			return err
		}
		fmt.Printf("granted access to %s\n", u.Email)
	case "remove":
		if len(args) < 2 {
			return errors.New("usage: api users remove EMAIL")
		}
		if err := svc.RemoveUser(ctx, args[1]); err != nil {
			return err
		}
		fmt.Printf("revoked access for %s\n", args[1])
	default:
		return fmt.Errorf("unknown users command %q", args[0])
	}
	return nil
}
