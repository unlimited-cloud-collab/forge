package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = errors.New("user not found")

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
}

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{
		pool: pool,
	}
}

func (r *UserRepository) Create(
	ctx context.Context,
	user User,
) error {
	_, err := r.pool.Exec(
		ctx,
		`
		INSERT INTO users (
			id,
			email,
			password_hash
		)
		VALUES ($1, $2, $3)
		`,
		user.ID,
		user.Email,
		user.PasswordHash,
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	return nil
}

func (r *UserRepository) GetByEmail(
	ctx context.Context,
	email string,
) (User, error) {
	var user User

	err := r.pool.QueryRow(
		ctx,
		`
		SELECT
			id,
			email,
			password_hash
		FROM users
		WHERE email = $1
		`,
		email,
	).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrUserNotFound
		}

		return User{}, fmt.Errorf("get user by email: %w", err)
	}

	return user, nil
}
