package users

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kekcode-9/go-chat-backend/internal/platform/models"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

/*
users table schema:
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_name TEXT NOT NULL,
    email TEXT UNIQUE,
    password TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)
*/

func (r *Repository) CreateUser(
	ctx context.Context,
	tx pgx.Tx,
	userName string,
	userEmail string,
	passHash string,
) (uuid.UUID, error) {
	var userID uuid.UUID

	err := tx.QueryRow(
		ctx,
		`
		INSERT INTO users (
			user_name,
			email,
			password
		)
		VALUES($1, $2, $3)
		RETURNING id
		`,
		userName,
		userEmail,
		passHash,
	).Scan(&userID)

	if err != nil {
		return uuid.Nil, err
	}

	return userID, nil
}

func (r *Repository) GetUserByEmail(
	ctx context.Context,
	tx pgx.Tx,
	email string,
) (*models.User, error) {
	var user models.User

	err := tx.QueryRow(
		ctx,
		`
		SELECT
			id,
			user_name,
			email,
			password,
			created_at
		FROM users
		WHERE email = $1
		`,
		email,
	).Scan(
		&user.ID,
		&user.UserName,
		&user.Email,
		&user.Password,
		&user.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}
