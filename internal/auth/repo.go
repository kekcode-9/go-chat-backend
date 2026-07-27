package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) createNewDevice(
	ctx context.Context,
	tx pgx.Tx,
	userID uuid.UUID,
	deviceName string,
	deviceType string,
) (uuid.UUID, error) {
	var deviceID uuid.UUID

	err := tx.QueryRow(
		ctx,
		`
		INSERT INTO devices (
			user_id,
			device_name,
			device_type,
			last_seen_at
		)
		VALUES ($1, $2, $3, NOW())
		RETURNING id
		`,
		userID,
		deviceName,
		deviceType,
	).Scan(&deviceID)

	if err != nil {
		return uuid.Nil, err
	}

	return deviceID, nil
}

func (r *Repository) findDevice(
	ctx context.Context,
	tx pgx.Tx,
	userID uuid.UUID,
	deviceID uuid.UUID,
) (bool, error) {
	var id uuid.UUID

	err := tx.QueryRow(
		ctx,
		`
		SELECT id
		FROM devices
		WHERE
			user_id = $1 AND
			id = $2
		`,
		userID,
		deviceID,
	).Scan(&id)

	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *Repository) createRefreshSession(
	ctx context.Context,
	tx pgx.Tx,
	userID uuid.UUID,
	deviceID uuid.UUID,
	sessionID uuid.UUID,
	refreshToken string,
) (uuid.UUID, error) {
	var id uuid.UUID

	hash := sha256.Sum256([]byte(refreshToken))
	refreshTokenHash := hex.EncodeToString(hash[:])

	err := tx.QueryRow(
		ctx,
		`
		INSERT INTO refresh_sessions (
			user_id,
			device_id,
			session_id,
			refresh_token_hash,
			expires_at
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
		`,
		userID,
		deviceID,
		sessionID,
		refreshTokenHash,
		time.Now().Add(30*24*time.Hour),
	).Scan(&id)

	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (r *Repository) findRefreshSession(
	ctx context.Context,
	tx pgx.Tx,
	refreshToken string,
) (*RefreshSession, error) {
	var refreshSession RefreshSession

	hash := sha256.Sum256([]byte(refreshToken))
	refreshTokenHash := hex.EncodeToString(hash[:])

	err := tx.QueryRow(
		ctx,
		`
		SELECT
			user_id,
			device_id,
			session_id,
			expires_at,
			revoked
		FROM refresh_sessions
		WHERE refresh_token_hash = $1
		`,
		refreshTokenHash,
	).Scan(
		&refreshSession.UserID,
		&refreshSession.DeviceID,
		&refreshSession.SessionID,
		&refreshSession.ExpiresAt,
		&refreshSession.Revoked,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &refreshSession, nil
}

func (r *Repository) revokeRefreshSession(
	ctx context.Context,
	tx pgx.Tx,
	sessionID uuid.UUID,
) (uuid.UUID, error) {
	var id uuid.UUID

	err := tx.QueryRow(
		ctx,
		`
		UPDATE refresh_sessions
		SET
			revoked = TRUE,
			last_used_at = NOW()
		WHERE session_id = $1
		RETURNING id
		`,
		sessionID,
	).Scan(&id)

	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (r *Repository) revokeAllUserSessions(
	ctx context.Context,
	tx pgx.Tx,
	userID uuid.UUID,
) ([]uuid.UUID, error) {
	rows, err := tx.Query(
		ctx,
		`
			UPDATE refresh_sessions
			SET
				revoked = TRUE,
				last_used_at = NOW()
			WHERE 
				user_id = $1 AND
				revoked = FALSE
			RETURNING id
		`,
		userID,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var ids []uuid.UUID

	for rows.Next() {
		var id uuid.UUID

		if err := rows.Scan(&id); err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ids, nil
}

func (r *Repository) revokeDeviceSession(
	ctx context.Context,
	tx pgx.Tx,
	userID uuid.UUID,
	deviceID uuid.UUID,
) (uuid.UUID, error) {
	var id uuid.UUID

	err := tx.QueryRow(
		ctx,
		`
		UPDATE refresh_sessions
		SET
			revoked = TRUE,
			last_used_at = NOW()
		WHERE
			user_id = $1 AND
			device_id = $2 AND
			revoked = FALSE
		RETURNING id
		`,
		userID,
		deviceID,
	).Scan(&id)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No matching device found.
			return uuid.Nil, nil
		}
		return uuid.Nil, err
	}

	return id, nil
}
