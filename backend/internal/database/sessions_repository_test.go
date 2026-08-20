package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func createTestUser(
	t *testing.T,
	ctx context.Context,
) User {
	t.Helper()

	user := User{
		ID:           uuid.New(),
		Email:        "session-" + uuid.NewString() + "@example.com",
		PasswordHash: "test-password-hash",
	}

	repository := NewUserRepository(testDatabasePool(t))

	if err := repository.Create(ctx, user); err != nil {
		t.Fatalf("create test user: %v", err)
	}

	return user
}

func TestSessionRepositoryCreateAndGetByTokenHash(t *testing.T) {
	pool := testDatabasePool(t)
	userRepository := NewUserRepository(pool)
	repository := NewSessionRepository(pool)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	user := User{
		ID:           uuid.New(),
		Email:        "session-" + uuid.NewString() + "@example.com",
		PasswordHash: "test-password-hash",
	}

	if err := userRepository.Create(ctx, user); err != nil {
		t.Fatalf("create test user: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM users WHERE id = $1`,
			user.ID,
		)
	})

	now := time.Now().UTC()

	session := Session{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: []byte("test-token-hash"),
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}

	if err := repository.Create(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	got, err := repository.GetByTokenHash(
		ctx,
		session.TokenHash,
		now,
	)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}

	if got.ID != session.ID {
		t.Fatalf(
			"expected session ID %q, got %q",
			session.ID,
			got.ID,
		)
	}

	if got.UserID != session.UserID {
		t.Fatalf(
			"expected user ID %q, got %q",
			session.UserID,
			got.UserID,
		)
	}
}

func TestSessionRepositoryRejectsExpiredSession(t *testing.T) {
	pool := testDatabasePool(t)
	userRepository := NewUserRepository(pool)
	repository := NewSessionRepository(pool)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	user := User{
		ID:           uuid.New(),
		Email:        "expired-" + uuid.NewString() + "@example.com",
		PasswordHash: "test-password-hash",
	}

	if err := userRepository.Create(ctx, user); err != nil {
		t.Fatalf("create test user: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM users WHERE id = $1`,
			user.ID,
		)
	})

	now := time.Now().UTC()

	session := Session{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: []byte("expired-token-hash"),
		CreatedAt: now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-time.Hour),
	}

	if err := repository.Create(ctx, session); err != nil {
		t.Fatalf("create expired session: %v", err)
	}

	_, err := repository.GetByTokenHash(
		ctx,
		session.TokenHash,
		now,
	)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf(
			"expected ErrSessionNotFound, got %v",
			err,
		)
	}
}

func TestSessionRepositoryRejectsDuplicateTokenHash(t *testing.T) {
	pool := testDatabasePool(t)
	userRepository := NewUserRepository(pool)
	repository := NewSessionRepository(pool)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	user := User{
		ID:           uuid.New(),
		Email:        "duplicate-session-" + uuid.NewString() + "@example.com",
		PasswordHash: "test-password-hash",
	}

	if err := userRepository.Create(ctx, user); err != nil {
		t.Fatalf("create test user: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM users WHERE id = $1`,
			user.ID,
		)
	})

	now := time.Now().UTC()

	first := Session{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: []byte("duplicate-token-hash"),
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}

	if err := repository.Create(ctx, first); err != nil {
		t.Fatalf("create first session: %v", err)
	}

	second := first
	second.ID = uuid.New()

	err := repository.Create(ctx, second)
	if !errors.Is(err, ErrSessionTokenTaken) {
		t.Fatalf(
			"expected ErrSessionTokenTaken, got %v",
			err,
		)
	}
}

func TestSessionRepositoryRejectsUnknownUser(t *testing.T) {
	pool := testDatabasePool(t)
	repository := NewSessionRepository(pool)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	now := time.Now().UTC()

	session := Session{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		TokenHash: []byte("unknown-user-token"),
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}

	err := repository.Create(ctx, session)
	if !errors.Is(err, ErrSessionUserNotFound) {
		t.Fatalf(
			"expected ErrSessionUserNotFound, got %v",
			err,
		)
	}
}

func TestSessionRepositoryDeleteExpired(t *testing.T) {
	pool := testDatabasePool(t)
	userRepository := NewUserRepository(pool)
	repository := NewSessionRepository(pool)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	user := User{
		ID:           uuid.New(),
		Email:        "cleanup-" + uuid.NewString() + "@example.com",
		PasswordHash: "test-password-hash",
	}

	if err := userRepository.Create(ctx, user); err != nil {
		t.Fatalf("create test user: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM users WHERE id = $1`,
			user.ID,
		)
	})

	now := time.Now().UTC()

	expired := Session{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: []byte("cleanup-expired-token"),
		CreatedAt: now.Add(-2 * time.Hour),
		ExpiresAt: now.Add(-time.Hour),
	}

	active := Session{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: []byte("cleanup-active-token"),
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
	}

	if err := repository.Create(ctx, expired); err != nil {
		t.Fatalf("create expired session: %v", err)
	}

	if err := repository.Create(ctx, active); err != nil {
		t.Fatalf("create active session: %v", err)
	}

	deleted, err := repository.DeleteExpired(ctx, now)
	if err != nil {
		t.Fatalf("delete expired sessions: %v", err)
	}

	if deleted != 1 {
		t.Fatalf(
			"expected 1 deleted session, got %d",
			deleted,
		)
	}

	if _, err := repository.GetByTokenHash(
		ctx,
		expired.TokenHash,
		now,
	); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected expired session to be deleted, got %v", err)
	}

	if _, err := repository.GetByTokenHash(
		ctx,
		active.TokenHash,
		now,
	); err != nil {
		t.Fatalf("active session must remain available: %v", err)
	}
}

func TestSessionRepositoryIndexesExist(t *testing.T) {
	pool := testDatabasePool(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	var count int

	err := pool.QueryRow(
		ctx,
		`
		SELECT COUNT(*)
		FROM pg_indexes
		WHERE schemaname = 'public'
		  AND tablename = 'sessions'
		  AND indexname IN (
			  'sessions_user_id_idx',
			  'sessions_expires_at_idx'
		  )
		`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("check session indexes: %v", err)
	}

	if count != 2 {
		t.Fatalf(
			"expected 2 session indexes, got %d",
			count,
		)
	}
}