package sessions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"forge/internal/database"
)

type fakeSessionRepository struct {
	session database.Session
	err     error
}

func (r *fakeSessionRepository) Create(
	_ context.Context,
	session database.Session,
) error {
	if r.err != nil {
		return r.err
	}

	r.session = session

	return nil
}

func (r *fakeSessionRepository) GetByTokenHash(
	_ context.Context,
	tokenHash []byte,
	now time.Time,
) (database.Session, error) {
	if r.err != nil {
		return database.Session{}, r.err
	}

	if !equalBytes(tokenHash, r.session.TokenHash) {
		return database.Session{}, database.ErrSessionNotFound
	}

	if !r.session.ExpiresAt.After(now) {
		return database.Session{}, database.ErrSessionNotFound
	}

	return r.session, nil
}

func TestGenerateTokenUsesStrongRandomToken(t *testing.T) {
	first, err := generateToken()
	if err != nil {
		t.Fatalf("generate first token: %v", err)
	}

	second, err := generateToken()
	if err != nil {
		t.Fatalf("generate second token: %v", err)
	}

	if first == second {
		t.Fatal("expected independently generated tokens to differ")
	}

	if len(first) < 40 {
		t.Fatalf(
			"expected encoded token to be sufficiently long, got %d characters",
			len(first),
		)
	}
}

func TestHashToken(t *testing.T) {
	token := "test-session-token"

	first := HashToken(token)
	second := HashToken(token)

	if !equalBytes(first, second) {
		t.Fatal("expected hashing the same token to be deterministic")
	}

	if len(first) != 32 {
		t.Fatalf(
			"expected SHA-256 hash to be 32 bytes, got %d",
			len(first),
		)
	}

	if equalBytes(first, HashToken("different-token")) {
		t.Fatal("different tokens must not produce the same hash")
	}
}

func TestCreateSession(t *testing.T) {
	repository := &fakeSessionRepository{}
	service := NewService(repository)

	userID := uuid.New()

	session, err := service.Create(
		context.Background(),
		userID,
	)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if session.ID == uuid.Nil {
		t.Fatal("expected session ID")
	}

	if session.UserID != userID {
		t.Fatalf(
			"expected user ID %q, got %q",
			userID,
			session.UserID,
		)
	}

	if session.Token == "" {
		t.Fatal("expected session token")
	}

	if len(repository.session.TokenHash) != 32 {
		t.Fatalf("expected 32-byte token hash")
	}

	if equalBytes(
		repository.session.TokenHash,
		[]byte(session.Token),
	) {
		t.Fatal("repository must not receive the raw session token")
	}

	if !repository.session.ExpiresAt.After(repository.session.CreatedAt) {
		t.Fatal("session must expire after creation")
	}

	expectedLifetime := repository.session.ExpiresAt.Sub(
		repository.session.CreatedAt,
	)

	if expectedLifetime != SessionLifetime {
		t.Fatalf(
			"expected lifetime %v, got %v",
			SessionLifetime,
			expectedLifetime,
		)
	}
}

func TestGetByToken(t *testing.T) {
	token := "valid-session-token"
	userID := uuid.New()

	repository := &fakeSessionRepository{
		session: database.Session{
			ID:        uuid.New(),
			UserID:    userID,
			TokenHash: HashToken(token),
			CreatedAt: time.Now().UTC().Add(-time.Hour),
			ExpiresAt: time.Now().UTC().Add(time.Hour),
		},
	}

	service := NewService(repository)

	got, err := service.GetByToken(
		context.Background(),
		token,
	)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}

	if got.UserID != userID {
		t.Fatalf(
			"expected user ID %q, got %q",
			userID,
			got.UserID,
		)
	}
}

func TestGetByTokenRejectsEmptyToken(t *testing.T) {
	service := NewService(&fakeSessionRepository{})

	_, err := service.GetByToken(
		context.Background(),
		"",
	)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf(
			"expected ErrSessionNotFound, got %v",
			err,
		)
	}
}

func TestCreatePropagatesRepositoryFailure(t *testing.T) {
	repositoryError := errors.New("database unavailable")

	service := NewService(&fakeSessionRepository{
		err: repositoryError,
	})

	_, err := service.Create(
		context.Background(),
		uuid.New(),
	)
	if err == nil {
		t.Fatal("expected repository failure")
	}

	if !errors.Is(err, repositoryError) {
		t.Fatalf(
			"expected repository error to be preserved, got %v",
			err,
		)
	}
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}