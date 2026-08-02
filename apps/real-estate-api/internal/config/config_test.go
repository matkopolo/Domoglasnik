package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://example")
	t.Setenv("DATABASE_MAX_CONNECTIONS", "")
	t.Setenv("API_ALLOWED_ORIGINS", " https://one.example,https://two.example ")
	t.Setenv("LOG_LEVEL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Address != ":8080" || cfg.DatabaseMaxConnections != 10 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[0] != "https://one.example" {
		t.Fatalf("unexpected origins: %v", cfg.AllowedOrigins)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error")
	}
}

func TestLoadRejectsInvalidPoolSize(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgresql://example")
	t.Setenv("DATABASE_MAX_CONNECTIONS", "zero")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected an error")
	}
}
