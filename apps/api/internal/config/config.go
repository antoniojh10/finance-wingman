// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	// Embed the time zone database so APP_TIMEZONE works in minimal containers.
	_ "time/tzdata"
)

type Config struct {
	Env            string
	Port           int
	DatabaseURL    string
	MigrateOnStart bool
	// Location resolves "today" and default periods (APP_TIMEZONE).
	Location *time.Location
	// PublicURL is the public base URL of this API (OAuth issuer, MCP resource).
	PublicURL string
	// WebBaseURL is the public URL of the Next.js app, used in magic links.
	WebBaseURL string
	// InitialUsers are granted access on startup ("email:Name,email2").
	InitialUsers []InitialUser
	// LoginEmailsPerHour caps sign-in emails per user (LOGIN_EMAILS_PER_HOUR).
	LoginEmailsPerHour int
	Email              EmailConfig
}

type InitialUser struct {
	Email string
	Name  string
}

type EmailConfig struct {
	// Provider is "smtp", "resend", or "log" (print emails to the log).
	Provider string
	// From is the formatted sender, built from EMAIL_FROM_NAME and EMAIL_FROM.
	From         string
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	ResendAPIKey string
}

// Load reads the configuration from the environment, applying defaults where
// a value is optional.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Env:                valueOr(getenv("APP_ENV"), "development"),
		Port:               8080,
		DatabaseURL:        getenv("DATABASE_URL"),
		MigrateOnStart:     true,
		Location:           time.UTC,
		PublicURL:          strings.TrimRight(valueOr(getenv("PUBLIC_URL"), "http://localhost:8080"), "/"),
		WebBaseURL:         valueOr(getenv("WEB_BASE_URL"), "http://localhost:3000"),
		InitialUsers:       parseInitialUsers(getenv("INITIAL_USERS")),
		LoginEmailsPerHour: 5,
		Email: EmailConfig{
			Provider:     valueOr(getenv("EMAIL_PROVIDER"), "log"),
			From:         formatFrom(valueOr(getenv("EMAIL_FROM_NAME"), "Finance Wingman"), valueOr(getenv("EMAIL_FROM"), "no-reply@localhost")),
			SMTPHost:     valueOr(getenv("SMTP_HOST"), "localhost"),
			SMTPPort:     1025,
			SMTPUsername: getenv("SMTP_USERNAME"),
			SMTPPassword: getenv("SMTP_PASSWORD"),
			ResendAPIKey: getenv("RESEND_API_KEY"),
		},
	}

	var errs []error

	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}

	if raw := getenv("PORT"); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil || port < 1 || port > 65535 {
			errs = append(errs, fmt.Errorf("PORT must be a valid port number, got %q", raw))
		} else {
			cfg.Port = port
		}
	}

	if raw := getenv("MIGRATE_ON_START"); raw != "" {
		migrate, err := strconv.ParseBool(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("MIGRATE_ON_START must be a boolean, got %q", raw))
		} else {
			cfg.MigrateOnStart = migrate
		}
	}

	if raw := getenv("APP_TIMEZONE"); raw != "" {
		loc, err := time.LoadLocation(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("APP_TIMEZONE must be an IANA time zone, got %q", raw))
		} else {
			cfg.Location = loc
		}
	}

	if raw := getenv("LOGIN_EMAILS_PER_HOUR"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			errs = append(errs, fmt.Errorf("LOGIN_EMAILS_PER_HOUR must be a positive integer, got %q", raw))
		} else {
			cfg.LoginEmailsPerHour = n
		}
	}

	if raw := getenv("SMTP_PORT"); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil || port < 1 || port > 65535 {
			errs = append(errs, fmt.Errorf("SMTP_PORT must be a valid port number, got %q", raw))
		} else {
			cfg.Email.SMTPPort = port
		}
	}

	if u, err := url.Parse(cfg.PublicURL); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.Path != "" {
		errs = append(errs, fmt.Errorf("PUBLIC_URL must be an http(s) origin without a path, got %q", cfg.PublicURL))
	}

	switch cfg.Email.Provider {
	case "log", "smtp":
	case "resend":
		if cfg.Email.ResendAPIKey == "" {
			errs = append(errs, errors.New("RESEND_API_KEY is required when EMAIL_PROVIDER=resend"))
		}
	default:
		errs = append(errs, fmt.Errorf("EMAIL_PROVIDER must be log, smtp or resend, got %q", cfg.Email.Provider))
	}

	return cfg, errors.Join(errs...)
}

// formatFrom builds an RFC 5322 "Name <address>" sender.
func formatFrom(name, address string) string {
	return (&mail.Address{Name: name, Address: address}).String()
}

func parseInitialUsers(raw string) []InitialUser {
	var users []InitialUser
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		email, name, _ := strings.Cut(entry, ":")
		users = append(users, InitialUser{Email: strings.TrimSpace(email), Name: strings.TrimSpace(name)})
	}
	return users
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
