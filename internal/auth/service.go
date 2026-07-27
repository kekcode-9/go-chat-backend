package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kekcode-9/go-chat-backend/internal/platform/config"
)

var jwtSecret []byte

func Init(cfg *config.Config) {
	jwtSecret = cfg.JWTSecret
}

type AuthService struct {
	repo      *Repository
	userStore UserStore
}

func NewAuthService(
	db *pgxpool.Pool,
	userStore UserStore,
) *AuthService {
	repo := NewRepository(db)

	return &AuthService{
		repo:      repo,
		userStore: userStore,
	}
}

func (a *AuthService) signup(
	userName string,
	deviceName string,
	deviceType string,
	email string,
	password string,
) (*SignupResponse, error) {
	ctx := context.Background()

	tx, err := a.repo.db.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer tx.Rollback(ctx)

	existingUser, err := a.userStore.GetUserByEmail(
		ctx,
		tx,
		email,
	)

	if err != nil {
		return nil, err
	}

	if existingUser != nil {
		return nil, ErrEmailAlreadyExists
	}

	passHash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}

	userID, err := a.userStore.CreateUser(
		ctx,
		tx,
		userName,
		email,
		passHash,
	)
	if err != nil {
		return nil, err
	}

	deviceID, err := a.repo.createNewDevice(
		ctx,
		tx,
		userID,
		deviceName,
		deviceType,
	)
	if err != nil {
		return nil, err
	}

	sessionID := uuid.New()

	accessToken, err := generateAccessToken(
		userID,
		deviceID,
	)
	if err != nil {
		return nil, err
	}

	refreshToken, err := generateRefreshToken()
	if err != nil {
		return nil, err
	}

	_, err = a.repo.createRefreshSession(
		ctx,
		tx,
		userID,
		deviceID,
		sessionID,
		refreshToken,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &SignupResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (a *AuthService) login(
	email string,
	password string,
	deviceID *uuid.UUID,
	deviceName *string,
	deviceType *string,
) (*LoginResponse, error) {
	ctx := context.Background()

	tx, err := a.repo.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	existingUser, err := a.userStore.GetUserByEmail(
		ctx,
		tx,
		email,
	)
	if err != nil {
		return nil, err
	}

	if existingUser == nil {
		return nil, ErrUserNotFound
	}

	if !checkPassword(password, existingUser.Password) {
		return nil, ErrInvalidCredentials
	}

	if deviceID == nil {
		// First login from this device.
		id, err := a.repo.createNewDevice(
			ctx,
			tx,
			existingUser.ID,
			*deviceName,
			*deviceType,
		)
		if err != nil {
			return nil, err
		}

		deviceID = &id
	} else {
		// Verify that this device belongs to the user.
		deviceExists, err := a.repo.findDevice(
			ctx,
			tx,
			existingUser.ID,
			*deviceID,
		)
		if err != nil {
			return nil, err
		}

		if !deviceExists {
			// Client supplied an unknown device ID.
			return nil, ErrUnknownDevice
		} else {
			// Revoke any previous session for this device.
			_, err = a.repo.revokeDeviceSession(
				ctx,
				tx,
				existingUser.ID,
				*deviceID,
			)
			if err != nil {
				return nil, err
			}
		}
	}

	sessionID := uuid.New()

	accessToken, err := generateAccessToken(
		existingUser.ID,
		*deviceID,
	)
	if err != nil {
		return nil, err
	}

	refreshToken, err := generateRefreshToken()
	if err != nil {
		return nil, err
	}

	_, err = a.repo.createRefreshSession(
		ctx,
		tx,
		existingUser.ID,
		*deviceID,
		sessionID,
		refreshToken,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (a *AuthService) refresh(
	refreshToken string,
) (*RefreshResponse, error) {
	ctx := context.Background()

	tx, err := a.repo.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// -----------------------------------------------
	// Find existing session
	// -----------------------------------------------
	session, err := a.repo.findRefreshSession(
		ctx,
		tx,
		refreshToken,
	)
	if err != nil {
		return nil, err
	}

	// No session found
	if session == nil {
		return nil, ErrInvalidRefreshToken
	}

	// -----------------------------------------------
	// Expired session
	// -----------------------------------------------
	if session.ExpiresAt.Before(time.Now()) {
		_, err = a.repo.revokeRefreshSession(
			ctx,
			tx,
			session.SessionID,
		)
		if err != nil {
			return nil, err
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}

		return nil, ErrExpiredRefreshToken
	}

	// -----------------------------------------------
	// Refresh token reuse detected
	// -----------------------------------------------
	if session.Revoked {
		_, err = a.repo.revokeAllUserSessions(
			ctx,
			tx,
			session.UserID,
		)
		if err != nil {
			return nil, err
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}

		return nil, ErrReusedRefreshToken
	}

	// -----------------------------------------------
	// Normal refresh rotation
	// -----------------------------------------------
	_, err = a.repo.revokeRefreshSession(
		ctx,
		tx,
		session.SessionID,
	)
	if err != nil {
		return nil, err
	}

	sessionID := uuid.New()

	accessToken, err := generateAccessToken(
		session.UserID,
		session.DeviceID,
	)
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := generateRefreshToken()
	if err != nil {
		return nil, err
	}

	_, err = a.repo.createRefreshSession(
		ctx,
		tx,
		session.UserID,
		session.DeviceID,
		sessionID,
		newRefreshToken,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &RefreshResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (a *AuthService) logout(
	userID uuid.UUID,
	deviceID uuid.UUID,
) error {
	ctx := context.Background()

	tx, err := a.repo.db.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	_, err = a.repo.revokeDeviceSession(
		ctx,
		tx,
		userID,
		deviceID,
	)

	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}
