package config

import "testing"

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("DATABASE_URL", "")

	cfg := Load()

	if cfg.Port != defaultPort {
		t.Fatalf(
			"expected default port %q, got %q",
			defaultPort,
			cfg.Port,
		)
	}

	if cfg.DatabaseURL != defaultDatabaseURL {
		t.Fatalf(
			"expected default database URL %q, got %q",
			defaultDatabaseURL,
			cfg.DatabaseURL,
		)
	}
}

func TestLoadUsesEnvironmentValues(t *testing.T) {
	const (
		expectedPort        = "9090"
		expectedDatabaseURL = "postgres://test:test@localhost:5432/testdb"
	)

	t.Setenv("PORT", expectedPort)
	t.Setenv("DATABASE_URL", expectedDatabaseURL)

	cfg := Load()

	if cfg.Port != expectedPort {
		t.Fatalf(
			"expected port %q, got %q",
			expectedPort,
			cfg.Port,
		)
	}

	if cfg.DatabaseURL != expectedDatabaseURL {
		t.Fatalf(
			"expected database URL %q, got %q",
			expectedDatabaseURL,
			cfg.DatabaseURL,
		)
	}
}

func TestLoadDoesNotRequireEnvironmentVariables(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("DATABASE_URL", "")

	cfg := Load()

	if cfg.Port == "" {
		t.Fatal("expected port to have a value")
	}

	if cfg.DatabaseURL == "" {
		t.Fatal("expected database URL to have a value")
	}
}

func TestLoadUsesSessionCookieSecureEnvironmentValue(t *testing.T) {
	t.Setenv("SESSION_COOKIE_SECURE", "true")

	cfg := Load()

	if !cfg.SessionCookieSecure {
		t.Fatal("expected session cookie secure to be enabled")
	}
}

func TestLoadDisablesSessionCookieSecureByDefault(t *testing.T) {
	t.Setenv("SESSION_COOKIE_SECURE", "")

	cfg := Load()

	if cfg.SessionCookieSecure {
		t.Fatal("expected session cookie secure to be disabled by default")
	}
}