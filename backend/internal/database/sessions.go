package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrSessionNotFound    = errors.New("session not found")
	ErrSessionTokenTaken  = errors.New("session token already exists")
	ErrSessionUserNotFound = errors.New("session user not found")
)

type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
	CreatedAt time.Time
}

type SessionRepository struct {
	pool *pgxpool.Pool
}

func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{
		pool: pool,
	}
}

func (r *SessionRepository) Create(
	ctx context.Context,
	session Session,
) error {
	_, err := r.pool.Exec(
		ctx,
		`
		INSERT INTO sessions (
			id,
			user_id,
			token_hash,
			expires_at,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5)
		`,
		session.ID,
		session.UserID,
		session.TokenHash,
		session.ExpiresAt,
		session.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) {
			switch {
			case pgErr.Code == "23505" &&
				pgErr.ConstraintName == "sessions_token_hash_unique":
				return ErrSessionTokenTaken

			case pgErr.Code == "23503" &&
				pgErr.ConstraintName == "sessions_user_id_fk":
				return ErrSessionUserNotFound
			}
		}

		return fmt.Errorf("create session: %w", err)
	}

	return nil
}

func (r *SessionRepository) GetByTokenHash(
	ctx context.Context,
	tokenHash []byte,
	now time.Time,
) (Session, error) {
	var session Session

	err := r.pool.QueryRow(
		ctx,
		`
		SELECT
			id,
			user_id,
			token_hash,
			expires_at,
			created_at
		FROM sessions
		WHERE token_hash = $1
		  AND expires_at > $2
		`,
		tokenHash,
		now,
	).Scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&session.ExpiresAt,
		&session.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrSessionNotFound
		}

		return Session{}, fmt.Errorf("get session by token hash: %w", err)
	}

	return session, nil
}

func (r *SessionRepository) DeleteExpired(
	ctx context.Context,
	now time.Time,
) (int64, error) {
	result, err := r.pool.Exec(
		ctx,
		`
		DELETE FROM sessions
		WHERE expires_at <= $1
		`,
		now,
	)
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}

	return result.RowsAffected(), nil
}
