package message

import (
	"context"

	"github.com/google/uuid"

	"github.com/kekcode-9/go-chat-backend/internal/platform/models"
)

type UserStore interface {
	FindDevicesForUsers(
		ctx context.Context,
		userIDs []uuid.UUID,
	) (map[uuid.UUID][]uuid.UUID, error)
}

type ConversationStore interface {
	FindConversationParticipants(
		ctx context.Context,
		conversationID uuid.UUID,
	) ([]models.ConversationParticipant, error)
}
