package config

import "testing"

func envFrom(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := load(envFrom(map[string]string{"DATABASE_URL": "postgres://localhost/db"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Env != "development" || cfg.Port != 8080 || !cfg.MigrateOnStart {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.Email.From != `"Finance Wingman" <no-reply@localhost>` {
		t.Fatalf("unexpected default sender: %s", cfg.Email.From)
	}
	if cfg.LoginEmailsPerHour != 5 {
		t.Fatalf("unexpected login rate limit: %d", cfg.LoginEmailsPerHour)
	}
	if cfg.PublicURL != "http://localhost:8080" {
		t.Fatalf("unexpected public URL: %s", cfg.PublicURL)
	}
	if cfg.PprofAddr != "" {
		t.Fatalf("pprof should be disabled by default: %q", cfg.PprofAddr)
	}
	if cfg.InitialWorkspaceName != "Finance Wingman" {
		t.Fatalf("unexpected initial workspace name: %q", cfg.InitialWorkspaceName)
	}
	if cfg.WebBaseURL != "http://localhost:3000" || cfg.Email.Provider != "log" || cfg.Email.SMTPPort != 1025 || len(cfg.InitialUsers) != 0 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := load(envFrom(map[string]string{
		"APP_ENV":                "production",
		"PORT":                   "9000",
		"DATABASE_URL":           "postgres://localhost/db",
		"MIGRATE_ON_START":       "false",
		"APP_TIMEZONE":           "America/Mexico_City",
		"WEB_BASE_URL":           "https://app.example.com",
		"PUBLIC_URL":             "https://api.example.com/",
		"INITIAL_USERS":          " ana@example.com:Ana , bob@example.com ,",
		"INITIAL_WORKSPACE_NAME": " Home ",
		"EMAIL_PROVIDER":         "resend",
		"RESEND_API_KEY":         "re_123",
		"SMTP_PORT":              "2525",
		"EMAIL_FROM":             "hi@example.com",
		"EMAIL_FROM_NAME":        "Wingman",
		"PPROF_ADDR":             "localhost:6060",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Env != "production" || cfg.Port != 9000 || cfg.MigrateOnStart || cfg.Location.String() != "America/Mexico_City" {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
	if cfg.PublicURL != "https://api.example.com" || cfg.WebBaseURL != "https://app.example.com" || cfg.Email.Provider != "resend" || cfg.Email.SMTPPort != 2525 || cfg.Email.From != `"Wingman" <hi@example.com>` {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
	if cfg.PprofAddr != "localhost:6060" {
		t.Fatalf("unexpected pprof address: %q", cfg.PprofAddr)
	}
	if cfg.InitialWorkspaceName != "Home" {
		t.Fatalf("unexpected initial workspace name: %q", cfg.InitialWorkspaceName)
	}
	want := []InitialUser{{Email: "ana@example.com", Name: "Ana"}, {Email: "bob@example.com"}}
	if len(cfg.InitialUsers) != 2 || cfg.InitialUsers[0] != want[0] || cfg.InitialUsers[1] != want[1] {
		t.Fatalf("unexpected initial users: %+v", cfg.InitialUsers)
	}
}

func TestLoadInvalid(t *testing.T) {
	tests := map[string]map[string]string{
		"missing database url": {},
		"invalid port":         {"DATABASE_URL": "x", "PORT": "abc"},
		"port out of range":    {"DATABASE_URL": "x", "PORT": "70000"},
		"invalid migrate flag": {"DATABASE_URL": "x", "MIGRATE_ON_START": "maybe"},
		"invalid time zone":    {"DATABASE_URL": "x", "APP_TIMEZONE": "Mars/Base"},
		"unknown provider":     {"DATABASE_URL": "x", "EMAIL_PROVIDER": "carrier-pigeon"},
		"resend without key":   {"DATABASE_URL": "x", "EMAIL_PROVIDER": "resend"},
		"invalid smtp port":    {"DATABASE_URL": "x", "SMTP_PORT": "x"},
		"invalid login limit":  {"DATABASE_URL": "x", "LOGIN_EMAILS_PER_HOUR": "0"},
		"public url with path": {"DATABASE_URL": "x", "PUBLIC_URL": "https://api.example.com/v1"},
		"public url no scheme": {"DATABASE_URL": "x", "PUBLIC_URL": "api.example.com"},
		"pprof without port":   {"DATABASE_URL": "x", "PPROF_ADDR": "localhost"},
		"pprof on public port": {"DATABASE_URL": "x", "PPROF_ADDR": ":8080"},
		"production log email": {"DATABASE_URL": "x", "APP_ENV": "production", "PUBLIC_URL": "https://a.example", "WEB_BASE_URL": "https://w.example"},
		"production http url":  {"DATABASE_URL": "x", "APP_ENV": "production", "EMAIL_PROVIDER": "smtp", "WEB_BASE_URL": "https://w.example"},
		"production http web":  {"DATABASE_URL": "x", "APP_ENV": "production", "EMAIL_PROVIDER": "smtp", "PUBLIC_URL": "https://a.example"},
	}
	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := load(envFrom(env)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
