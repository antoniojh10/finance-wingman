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
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := load(envFrom(map[string]string{
		"APP_ENV":          "production",
		"PORT":             "9000",
		"DATABASE_URL":     "postgres://localhost/db",
		"MIGRATE_ON_START": "false",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Env != "production" || cfg.Port != 9000 || cfg.MigrateOnStart {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
}

func TestLoadInvalid(t *testing.T) {
	tests := map[string]map[string]string{
		"missing database url": {},
		"invalid port":         {"DATABASE_URL": "x", "PORT": "abc"},
		"port out of range":    {"DATABASE_URL": "x", "PORT": "70000"},
		"invalid migrate flag": {"DATABASE_URL": "x", "MIGRATE_ON_START": "maybe"},
	}
	for name, env := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := load(envFrom(env)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
