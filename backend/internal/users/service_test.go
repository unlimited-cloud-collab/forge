package users

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"forge/internal/database"
)

type fakeUserRepository struct {
	users map[string]database.User
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{
		users: make(map[string]database.User),
	}
}

func (r *fakeUserRepository) Create(
	_ context.Context,
	user database.User,
) error {
	if _, exists := r.users[user.Email]; exists {
		return errors.New("duplicate email")
	}

	r.users[user.Email] = user

	return nil
}

func (r *fakeUserRepository) GetByEmail(
	_ context.Context,
	email string,
) (database.User, error) {
	user, exists := r.users[email]
	if !exists {
		return database.User{}, database.ErrUserNotFound
	}

	return user, nil
}

func TestRegisterCreatesUserWithHashedPassword(t *testing.T) {
	repository := newFakeUserRepository()
	service := NewService(repository)

	user, err := service.Register(
		context.Background(),
		" user@example.com ",
		"correct horse battery staple",
	)
	if err != nil {
		t.Fatalf("register user: %v", err)
	}

	if user.ID == uuid.Nil {
		t.Fatal("expected generated user ID")
	}

	if user.Email != "user@example.com" {
		t.Fatalf(
			"expected normalized email %q, got %q",
			"user@example.com",
			user.Email,
		)
	}

	if user.PasswordHash == "correct horse battery staple" {
		t.Fatal("password must not be stored in plaintext")
	}

	if err := bcryptCompare(
		user.PasswordHash,
		"correct horse battery staple",
	); err != nil {
		t.Fatalf("password hash does not match password: %v", err)
	}
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	repository := newFakeUserRepository()
	service := NewService(repository)

	_, err := service.Register(
		context.Background(),
		"user@example.com",
		"password-one",
	)
	if err != nil {
		t.Fatalf("first registration: %v", err)
	}

	_, err = service.Register(
		context.Background(),
		"user@example.com",
		"password-two",
	)
	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf(
			"expected ErrEmailTaken, got %v",
			err,
		)
	}
}

func TestRegisterRejectsEmptyEmail(t *testing.T) {
	service := NewService(newFakeUserRepository())

	_, err := service.Register(
		context.Background(),
		"   ",
		"password",
	)
	if !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf(
			"expected ErrInvalidEmail, got %v",
			err,
		)
	}
}

func TestRegisterRejectsEmptyPassword(t *testing.T) {
	service := NewService(newFakeUserRepository())

	_, err := service.Register(
		context.Background(),
		"user@example.com",
		"",
	)
	if !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf(
			"expected ErrInvalidPassword, got %v",
			err,
		)
	}
}

func TestRegisterPropagatesRepositoryFailure(t *testing.T) {
	repository := &failingUserRepository{}
	service := NewService(repository)

	_, err := service.Register(
		context.Background(),
		"user@example.com",
		"password",
	)
	if err == nil {
		t.Fatal("expected repository error")
	}
}

type failingUserRepository struct{}

func (r *failingUserRepository) Create(
	context.Context,
	database.User,
) error {
	return errors.New("database unavailable")
}

func (r *failingUserRepository) GetByEmail(
	context.Context,
	string,
) (database.User, error) {
	return database.User{}, database.ErrUserNotFound
}

func bcryptCompare(hash string, password string) error {
	return bcrypt.CompareHashAndPassword(
		[]byte(hash),
		[]byte(password),
	)
}

func TestRegisterTranslatesDuplicateEmailFromRepository(
	t *testing.T,
) {
	repository := &duplicateEmailRepository{}
	service := NewService(repository)

	_, err := service.Register(
		context.Background(),
		"user@example.com",
		"password",
	)

	if !errors.Is(err, ErrEmailTaken) {
		t.Fatalf(
			"expected ErrEmailTaken, got %v",
			err,
		)
	}
}

type duplicateEmailRepository struct{}

func (r *duplicateEmailRepository) Create(
	context.Context,
	database.User,
) error {
	return database.ErrUserEmailTaken
}

func (r *duplicateEmailRepository) GetByEmail(
	context.Context,
	string,
) (database.User, error) {
	return database.User{}, database.ErrUserNotFound
}

func TestAuthenticateWithValidCredentials(t *testing.T) {
	repository := newFakeUserRepository()
	service := NewService(repository)

	_, err := service.Register(
		context.Background(),
		"user@example.com",
		"correct horse battery staple",
	)
	if err != nil {
		t.Fatalf("register user: %v", err)
	}

	user, err := service.Authenticate(
		context.Background(),
		"user@example.com",
		"correct horse battery staple",
	)
	if err != nil {
		t.Fatalf("authenticate user: %v", err)
	}

	if user.Email != "user@example.com" {
		t.Fatalf(
			"expected email %q, got %q",
			"user@example.com",
			user.Email,
		)
	}
}

func TestAuthenticateRejectsWrongPassword(t *testing.T) {
	repository := newFakeUserRepository()
	service := NewService(repository)

	_, err := service.Register(
		context.Background(),
		"user@example.com",
		"correct password",
	)
	if err != nil {
		t.Fatalf("register user: %v", err)
	}

	_, err = service.Authenticate(
		context.Background(),
		"user@example.com",
		"wrong password",
	)

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf(
			"expected ErrInvalidCredentials, got %v",
			err,
		)
	}
}

func TestAuthenticateRejectsUnknownUser(t *testing.T) {
	service := NewService(newFakeUserRepository())

	_, err := service.Authenticate(
		context.Background(),
		"missing@example.com",
		"some password",
	)

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf(
			"expected ErrInvalidCredentials, got %v",
			err,
		)
	}
}
