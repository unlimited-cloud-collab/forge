package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestUsersTable(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		databaseURL = "postgres://forge:forge_dev_password@localhost:5432/forge"
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	defer conn.Close(context.Background())

	var tableExists bool

	err = conn.QueryRow(
		ctx,
		`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name = 'users'
		)
		`,
	).Scan(&tableExists)

	if err != nil {
		t.Fatalf("check users table: %v", err)
	}

	if !tableExists {
		t.Fatal("expected users table to exist")
	}
}

func TestUsersColumns(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		databaseURL = "postgres://forge:forge_dev_password@localhost:5432/forge"
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	defer conn.Close(context.Background())

	expectedColumns := map[string]bool{
		"id":            false,
		"email":         false,
		"password_hash": false,
		"created_at":    false,
		"updated_at":    false,
	}

	rows, err := conn.Query(
		ctx,
		`
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'users'
		`,
	)
	if err != nil {
		t.Fatalf("query users columns: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var column string

		if err := rows.Scan(&column); err != nil {
			t.Fatalf("scan column: %v", err)
		}

		if _, ok := expectedColumns[column]; ok {
			expectedColumns[column] = true
		}
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("iterate columns: %v", err)
	}

	for column, found := range expectedColumns {
		if !found {
			t.Errorf("expected users column %q", column)
		}
	}
}

func TestUsersCanStoreUser(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		databaseURL = "postgres://forge:forge_dev_password@localhost:5432/forge"
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	defer conn.Close(context.Background())

	userID := uuid.New()
	testEmail := "task22-" + userID.String() + "@example.com"

	_, err = conn.Exec(
		ctx,
		`
		INSERT INTO users (
			id,
			email,
			password_hash
		)
		VALUES ($1, $2, $3)
		`,
		userID,
		testEmail,
		"test-password-hash",
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	t.Cleanup(func() {
		_, _ = conn.Exec(
			context.Background(),
			`DELETE FROM users WHERE id = $1`,
			userID,
		)
	})

	var email string

	err = conn.QueryRow(
		ctx,
		`SELECT email FROM users WHERE id = $1`,
		userID,
	).Scan(&email)

	if err != nil {
		t.Fatalf("query inserted user: %v", err)
	}

	if email != testEmail {
		t.Fatalf(
			"expected email %q, got %q",
			"task22@example.com",
			email,
		)
	}
}
