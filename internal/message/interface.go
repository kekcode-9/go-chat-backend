package message

import (
	"context"

	"github.com/google/uuid"
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
	) ([]uuid.UUID, error)
}
