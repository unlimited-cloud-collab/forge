package config

import "testing"

func TestLoadUsesDefaultValues(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("DATABASE_URL", "")

	cfg := Load()

	if cfg.Port != "8080" {
		t.Fatalf("expected default port %q, got %q", "8080", cfg.Port)
	}

	expectedDatabaseURL := "postgres://forge:forge_dev_password@localhost:5432/forge"

	if cfg.DatabaseURL != expectedDatabaseURL {
		t.Fatalf(
			"expected default database URL %q, got %q",
			expectedDatabaseURL,
			cfg.DatabaseURL,
		)
	}
}

func TestLoadUsesEnvironmentValues(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/testdb")

	cfg := Load()

	if cfg.Port != "9090" {
		t.Fatalf("expected port %q, got %q", "9090", cfg.Port)
	}

	expectedDatabaseURL := "postgres://test:test@localhost:5432/testdb"

	if cfg.DatabaseURL != expectedDatabaseURL {
		t.Fatalf(
			"expected database URL %q, got %q",
			expectedDatabaseURL,
			cfg.DatabaseURL,
		)
	}
}
