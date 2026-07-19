package message

import (
	"context"
	"encoding/json"
	"log"

	"github.com/kekcode-9/go-chat-backend/internal/models"
)

func (m *MessageService) Subscriber(ctx context.Context) {
	channel := "backend:" + m.BackendID

	pubsub := m.redis.Subscribe(ctx, channel)

	defer pubsub.Close()

	log.Printf("Subscribed to %s", channel)

	ch := pubsub.Channel()

	for {
		select {
		case <- ctx.Done():
			return

		case redisMsg := <-ch:
			var chatMessage models.ChatMessage

			err := json.Unmarshal(
				[]byte(redisMsg.Payload),
				&chatMessage,
			)

			if err != nil {
				log.Println("redis unmarshal:", err)
				continue
			}

			m.OutMssgHandler(chatMessage)
		}
	}
}