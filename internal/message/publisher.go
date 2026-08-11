package message

import (
	"context"
	"encoding/json"

	"github.com/kekcode-9/go-chat-backend/internal/platform/models"
)

func (m *MessageService) Publish(
	ctx context.Context,
	channel string,
	message models.OutgoingMessage,
) error {
	payload, err := json.Marshal(message)

	if err != nil {
		return err
	}

	return m.redis.Publish(
		ctx,
		channel,
		payload,
	).Err()
}
