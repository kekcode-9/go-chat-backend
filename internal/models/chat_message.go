package models

import (
	"time"
	"github.com/google/uuid"
)

// This message is written into the RouteMessage of WsConManager by the
// MessageService --> OutMssgHandler() method
type ChatMessage struct{
	MessageID uuid.UUID `json:"message_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Sequence_no uint64 `json:"sequence_no"`

	SenderUserID uuid.UUID `json:"sender_user_id"`
	SenderName string `json:"sender_name"`

	TargetDeviceIDs []uuid.UUID `json:"target_device_ids"`

	Payload string `json:"payload"`

	Timestamp time.Time `json:timestamp`
}