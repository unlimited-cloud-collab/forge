package migrations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

const advisoryLockKey int64 = 732451982

var migrationFilenamePattern = regexp.MustCompile(`^([0-9]+)_[a-z0-9_]+\.up\.sql$`)

type migration struct {
	version int64
	name    string
	sql     string
}

func Run(ctx context.Context, conn *pgx.Conn) error {
	migrationsDir, err := defaultMigrationsDir()
	if err != nil {
		return fmt.Errorf("locate migrations directory: %w", err)
	}

	migrations, err := discoverMigrations(migrationsDir)
	if err != nil {
		return fmt.Errorf("discover migrations: %w", err)
	}

	if len(migrations) == 0 {
		return fmt.Errorf("no up migrations found in %s", migrationsDir)
	}

	if err := acquireAdvisoryLock(ctx, conn); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(
			context.Background(),
			`SELECT pg_advisory_unlock($1)`,
			advisoryLockKey,
		)
	}()

	for _, migration := range migrations {
		applied, err := migrationApplied(ctx, conn, migration.version)
		if err != nil {
			return fmt.Errorf(
				"check migration %d (%s): %w",
				migration.version,
				migration.name,
				err,
			)
		}

		if applied {
			continue
		}

		if err := applyMigration(ctx, conn, migration); err != nil {
			return fmt.Errorf(
				"apply migration %d (%s): %w",
				migration.version,
				migration.name,
				err,
			)
		}
	}

	return nil
}

func defaultMigrationsDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	dir := wd

	for {
		internalDir := filepath.Join(dir, "internal")
		migrationsDir := filepath.Join(dir, "migrations")

		if isDirectory(internalDir) && isDirectory(migrationsDir) {
			return migrationsDir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return "", fmt.Errorf("repository migrations directory not found")
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

func discoverMigrations(dir string) ([]migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	migrations := make([]migration, 0)

	seenVersions := make(map[int64]string)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		if !strings.HasSuffix(name, ".sql") {
			continue
		}

		if strings.HasSuffix(name, ".down.sql") {
			continue
		}

		if !strings.HasSuffix(name, ".up.sql") {
			return nil, fmt.Errorf(
				"invalid migration filename %q: expected NNN_description.up.sql",
				name,
			)
		}

		matches := migrationFilenamePattern.FindStringSubmatch(name)
		if matches == nil {
			return nil, fmt.Errorf(
				"invalid migration filename %q: expected NNN_description.up.sql",
				name,
			)
		}

		version, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf(
				"parse migration version from %q: %w",
				name,
				err,
			)
		}

		if version <= 0 {
			return nil, fmt.Errorf(
				"migration %q has invalid version %d",
				name,
				version,
			)
		}

		if previous, exists := seenVersions[version]; exists {
			return nil, fmt.Errorf(
				"duplicate migration version %d: %q and %q",
				version,
				previous,
				name,
			)
		}

		seenVersions[version] = name

		path := filepath.Join(dir, name)

		sql, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		if strings.TrimSpace(string(sql)) == "" {
			return nil, fmt.Errorf("migration %q is empty", name)
		}

		migrations = append(migrations, migration{
			version: version,
			name:    name,
			sql:     string(sql),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	return migrations, nil
}

func acquireAdvisoryLock(ctx context.Context, conn *pgx.Conn) error {
	_, err := conn.Exec(
		ctx,
		`SELECT pg_advisory_lock($1)`,
		advisoryLockKey,
	)

	return err
}

func migrationApplied(
	ctx context.Context,
	conn *pgx.Conn,
	version int64,
) (bool, error) {
	var tableExists bool

	err := conn.QueryRow(
		ctx,
		`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name = 'schema_migrations'
		)
		`,
	).Scan(&tableExists)
	if err != nil {
		return false, fmt.Errorf("check migration table: %w", err)
	}

	if !tableExists {
		return false, nil
	}

	var applied bool

	err = conn.QueryRow(
		ctx,
		`
		SELECT EXISTS (
			SELECT 1
			FROM schema_migrations
			WHERE version = $1
		)
		`,
		version,
	).Scan(&applied)
	if err != nil {
		return false, fmt.Errorf("query migration history: %w", err)
	}

	return applied, nil
}

func applyMigration(
	ctx context.Context,
	conn *pgx.Conn,
	m migration,
) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, m.sql); err != nil {
		return fmt.Errorf("execute migration SQL: %w", err)
	}

	// Migration 001 creates schema_migrations itself. Therefore its
	// history row can only be inserted after its CREATE TABLE succeeds.
	_, err = tx.Exec(
		ctx,
		`
		INSERT INTO schema_migrations (version)
		VALUES ($1)
		`,
		m.version,
	)
	if err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
