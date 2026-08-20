package sessions

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64" 
	"fmt"
	"time"

	"github.com/google/uuid"

	"forge/internal/database"
)

const (
	TokenEntropyBytes = 32
	SessionLifetime    = 24 * time.Hour
)

var ErrSessionNotFound = database.ErrSessionNotFound

type Repository interface {
	Create(context.Context, database.Session) error
	GetByTokenHash(
		context.Context,
		[]byte,
		time.Time,
	) (database.Session, error)
}

type CreatedSession struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Token     string
	ExpiresAt time.Time
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{
		repository: repository,
	}
}

func (s *Service) Create(
	ctx context.Context,
	userID uuid.UUID,
) (CreatedSession, error) {
	token, err := generateToken()
	if err != nil {
		return CreatedSession{}, fmt.Errorf(
			"generate session token: %w",
			err,
		)
	}

	now := time.Now().UTC()

	session := database.Session{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: HashToken(token),
		CreatedAt: now,
		ExpiresAt: now.Add(SessionLifetime),
	}

	if err := s.repository.Create(ctx, session); err != nil {
		return CreatedSession{}, fmt.Errorf(
			"create session: %w",
			err,
		)
	}

	return CreatedSession{
		ID:        session.ID,
		UserID:    session.UserID,
		Token:     token,
		ExpiresAt: session.ExpiresAt,
	}, nil
}

func (s *Service) GetByToken(
	ctx context.Context,
	token string,
) (database.Session, error) {
	if token == "" {
		return database.Session{}, ErrSessionNotFound
	}

	session, err := s.repository.GetByTokenHash(
		ctx,
		HashToken(token),
		time.Now().UTC(),
	)
	if err != nil {
		return database.Session{}, fmt.Errorf(
			"get session: %w",
			err,
		)
	}

	return session, nil
}

func generateToken() (string, error) {
	buffer := make([]byte, TokenEntropyBytes)

	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func HashToken(token string) []byte {
	hash := sha256.Sum256([]byte(token))

	return hash[:]
}