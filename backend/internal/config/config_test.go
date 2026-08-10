package config

import (
	"os"
	"testing"
)

func TestLoadUsesDefaultPort(t *testing.T) {
	t.Setenv("PORT", "")

	cfg := Load()

	if cfg.Port != "8080" {
		t.Fatalf("expected default port %q, got %q", "8080", cfg.Port)
	}
}

func TestLoadUsesEnvironmentPort(t *testing.T) {
	t.Setenv("PORT", "9090")

	cfg := Load()

	if cfg.Port != "9090" {
		t.Fatalf("expected port %q, got %q", "9090", cfg.Port)
	}
}

func TestLoadDoesNotDependOnProcessEnvironmentAfterSetup(t *testing.T) {
	originalPort, existed := os.LookupEnv("PORT")

	t.Cleanup(func() {
		if existed {
			_ = os.Setenv("PORT", originalPort)
		} else {
			_ = os.Unsetenv("PORT")
		}
	})

	_ = os.Setenv("PORT", "7070")

	cfg := Load()

	if cfg.Port != "7070" {
		t.Fatalf("expected port %q, got %q", "7070", cfg.Port)
	}
}
