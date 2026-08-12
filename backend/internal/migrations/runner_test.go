package migrations

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverMigrationsOrdersByVersion(t *testing.T) {
	dir := t.TempDir()

	writeMigration(t, dir, "003_add_products.up.sql")
	writeMigration(t, dir, "001_create_schema_migrations.up.sql")
	writeMigration(t, dir, "002_create_users.up.sql")

	got, err := discoverMigrations(dir)
	if err != nil {
		t.Fatalf("discover migrations: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 migrations, got %d", len(got))
	}

	expected := []int64{1, 2, 3}

	for i, migration := range got {
		if migration.version != expected[i] {
			t.Fatalf(
				"migration %d: expected version %d, got %d",
				i,
				expected[i],
				migration.version,
			)
		}
	}
}

func TestDiscoverMigrationsRejectsMalformedFilename(t *testing.T) {
	dir := t.TempDir()

	writeMigration(t, dir, "001_create_users.sql")

	_, err := discoverMigrations(dir)
	if err == nil {
		t.Fatal("expected malformed migration filename error")
	}
}

func TestDiscoverMigrationsRejectsDuplicateVersions(t *testing.T) {
	dir := t.TempDir()

	writeMigration(t, dir, "001_create_users.up.sql")
	writeMigration(t, dir, "001_create_other.up.sql")

	_, err := discoverMigrations(dir)
	if err == nil {
		t.Fatal("expected duplicate migration version error")
	}
}

func TestDiscoverMigrationsRejectsEmptyMigration(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "001_empty.up.sql")

	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write migration: %v", err)
	}

	_, err := discoverMigrations(dir)
	if err == nil {
		t.Fatal("expected empty migration error")
	}
}

func TestDefaultMigrationsDirExists(t *testing.T) {
	dir, err := defaultMigrationsDir()
	if err != nil {
		t.Fatalf("find migrations directory: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat migrations directory: %v", err)
	}

	if !info.IsDir() {
		t.Fatalf("expected migrations path to be a directory")
	}
}

func writeMigration(t *testing.T, dir, name string) {
	t.Helper()

	path := filepath.Join(dir, name)

	if err := os.WriteFile(
		path,
		[]byte("SELECT 1;"),
		0o644,
	); err != nil {
		t.Fatalf("write migration %s: %v", name, err)
	}
}

func TestMigrationContextCanBeCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if ctx.Err() == nil {
		t.Fatal("expected cancelled context")
	}
}
