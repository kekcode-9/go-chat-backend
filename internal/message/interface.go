package message

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kekcode-9/go-chat-backend/internal/platform/models"
)

type UserStore interface {
	FindDevicesForUsers(
		ctx context.Context,
		userIDs []uuid.UUID,
	) (map[uuid.UUID][]uuid.UUID, error)
}

type ConversationStore interface {
	GetConversationParticipantState(
		ctx context.Context,
		tx pgx.Tx,
		userID uuid.UUID,
		conversationID uuid.UUID,
	) (*models.ConversationParticipantState, error)

	FindConversationParticipants(
		ctx context.Context,
		conversationID uuid.UUID,
	) ([]models.ConversationParticipant, error)
}
