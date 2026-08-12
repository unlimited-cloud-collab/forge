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

var (
	ErrInvalidEmail    = errors.New("invalid email")
	ErrInvalidPassword = errors.New("invalid password")
	ErrEmailTaken      = errors.New("email already registered")
)

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
		return database.User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}
