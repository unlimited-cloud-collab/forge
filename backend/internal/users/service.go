package users

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"forge/internal/database"
)

const dummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

var (
	ErrInvalidEmail       = errors.New("invalid email")
	ErrInvalidPassword    = errors.New("invalid password")
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type AuthenticatedUser struct {
	ID    uuid.UUID
	Email string
}

type UserRepository interface {
	Create(context.Context, database.User) error
	GetByEmail(context.Context, string) (database.User, error)
}

type Service struct {
	repository UserRepository
}

func NewService(repository UserRepository) *Service {
	return &Service{
		repository: repository,
	}
}

func (s *Service) Register(
	ctx context.Context,
	email string,
	password string,
) (database.User, error) {
	email = strings.TrimSpace(email)

	if email == "" {
		return database.User{}, ErrInvalidEmail
	}

	if password == "" {
		return database.User{}, ErrInvalidPassword
	}

	_, err := s.repository.GetByEmail(ctx, email)
	if err == nil {
		return database.User{}, ErrEmailTaken
	}

	if !errors.Is(err, database.ErrUserNotFound) {
		return database.User{}, fmt.Errorf("check existing user: %w", err)
	}

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return database.User{}, fmt.Errorf("hash password: %w", err)
	}

	user := database.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(passwordHash),
	}

	if err := s.repository.Create(ctx, user); err != nil {
		if errors.Is(err, database.ErrUserEmailTaken) {
			return database.User{}, ErrEmailTaken
		}

		return database.User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

func (s *Service) Authenticate(
	ctx context.Context,
	email string,
	password string,
) (AuthenticatedUser, error) {
	email = strings.TrimSpace(email)

	if email == "" || password == "" {
		return AuthenticatedUser{}, ErrInvalidCredentials
	}

	user, err := s.repository.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, database.ErrUserNotFound) {
			// Perform the same expensive password operation used for
			// existing users so the missing-user path does not return
			// immediately based on account existence.
			_ = bcrypt.CompareHashAndPassword(
				[]byte(dummyPasswordHash),
				[]byte(password),
			)

			return AuthenticatedUser{}, ErrInvalidCredentials
		}

		return AuthenticatedUser{}, fmt.Errorf(
			"get user by email: %w",
			err,
		)
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	); err != nil {
		return AuthenticatedUser{}, ErrInvalidCredentials
	}

	return AuthenticatedUser{
		ID:    user.ID,
		Email: user.Email,
	}, nil
}
