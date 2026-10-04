// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
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
}

// Load reads the configuration from the environment, applying defaults where
// a value is optional.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	cfg := Config{
		Env:            valueOr(getenv("APP_ENV"), "development"),
		Port:           8080,
		DatabaseURL:    getenv("DATABASE_URL"),
		MigrateOnStart: true,
		Location:       time.UTC,
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

	return cfg, errors.Join(errs...)
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
