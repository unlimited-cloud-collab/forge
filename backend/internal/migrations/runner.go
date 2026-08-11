package migrations

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const migrationVersion int64 = 1

const migrationSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version BIGINT PRIMARY KEY,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS forge_migration_marker (
	id INTEGER PRIMARY KEY
);
`

func Run(ctx context.Context, conn *pgx.Conn) error {
	if _, err := conn.Exec(ctx, migrationSQL); err != nil {
		return fmt.Errorf("create migration tables: %w", err)
	}

	var exists bool

	err := conn.QueryRow(
		ctx,
		`SELECT EXISTS (
			SELECT 1
			FROM schema_migrations
			WHERE version = $1
		)`,
		migrationVersion,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check migration version: %w", err)
	}

	if exists {
		return nil
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, migrationSQL); err != nil {
		return fmt.Errorf("run migration: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`INSERT INTO schema_migrations (version) VALUES ($1)`,
		migrationVersion,
	); err != nil {
		return fmt.Errorf("record migration version: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}

	return nil
}