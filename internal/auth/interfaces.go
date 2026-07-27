package auth

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kekcode-9/go-chat-backend/internal/platform/models"
)

type UserStore interface {
	CreateUser(
		ctx context.Context,
		tx pgx.Tx,
		userName string,
		userEmail string,
		passHash string,
	) (uuid.UUID, error)

	GetUserByEmail(
		ctx context.Context,
		tx pgx.Tx,
		email string,
	) (*models.User, error)
}
