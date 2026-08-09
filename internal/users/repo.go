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

func (r *Repository) BlockUser(
	ctx context.Context,
	tx pgx.Tx,
	blockerID uuid.UUID,
	blockedID uuid.UUID,
) error {
	var exists bool

	err := tx.QueryRow(
		ctx,
		`
		SELECT EXISTS (
			SELECT 1
			FROM users
			WHERE id = $1
		)
		`,
		blockedID,
	).Scan(&exists)
	if err != nil {
		return err
	}

	if !exists {
		return ErrUserNotFound
	}

	_, err = tx.Exec(
		ctx,
		`
		INSERT INTO blocked_users (
			blocker_id,
			blocked_id
		)
		VALUES ($1, $2)
		`,
		blockerID,
		blockedID,
	)
	if err != nil {
		return err
	}

	return nil
}

// -----------------------------------------------------------------------------
// Returns all non-revoked devices grouped by user.
//
// userID -> []deviceID
// -----------------------------------------------------------------------------

func (r *Repository) FindDevicesForUsers(
	ctx context.Context,
	userIDs []uuid.UUID,
) (map[uuid.UUID][]uuid.UUID, error) {

	if len(userIDs) == 0 {
		return map[uuid.UUID][]uuid.UUID{}, nil
	}

	rows, err := r.db.Query(
		ctx,
		`
		SELECT
			user_id,
			id
		FROM devices
		WHERE
			user_id = ANY($1)
			AND revoked_at IS NULL
		`,
		userIDs,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	result := make(map[uuid.UUID][]uuid.UUID)

	for rows.Next() {

		var (
			userID   uuid.UUID
			deviceID uuid.UUID
		)

		if err := rows.Scan(
			&userID,
			&deviceID,
		); err != nil {
			return nil, err
		}

		result[userID] = append(
			result[userID],
			deviceID,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
