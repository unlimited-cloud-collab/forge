package database

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testDatabaseURL() string {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" {
		return databaseURL
	}

	return "postgres://forge:forge_dev_password@localhost:5432/forge"
}

func testDatabasePool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(
		ctx,
		testDatabaseURL(),
	)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}

	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	return pool
}

func TestUserRepositoryCreateAndGetByEmail(t *testing.T) {
	pool := testDatabasePool(t)
	repository := NewUserRepository(pool)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	user := User{
		ID:           uuid.New(),
		Email:        "task23-" + uuid.NewString() + "@example.com",
		PasswordHash: "test-password-hash",
	}

	if err := repository.Create(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM users WHERE id = $1`,
			user.ID,
		)
	})

	got, err := repository.GetByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("get user by email: %v", err)
	}

	if got.ID != user.ID {
		t.Fatalf(
			"expected user ID %q, got %q",
			user.ID,
			got.ID,
		)
	}

	if got.Email != user.Email {
		t.Fatalf(
			"expected email %q, got %q",
			user.Email,
			got.Email,
		)
	}

	if got.PasswordHash != user.PasswordHash {
		t.Fatalf(
			"expected password hash %q, got %q",
			user.PasswordHash,
			got.PasswordHash,
		)
	}
}

func TestUserRepositoryGetByEmailNotFound(t *testing.T) {
	pool := testDatabasePool(t)
	repository := NewUserRepository(pool)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	email := "missing-" + uuid.NewString() + "@example.com"

	_, err := repository.GetByEmail(ctx, email)
	if err == nil {
		t.Fatal("expected user not found error")
	}

	if err != ErrUserNotFound {
		t.Fatalf(
			"expected ErrUserNotFound, got %v",
			err,
		)
	}
}
