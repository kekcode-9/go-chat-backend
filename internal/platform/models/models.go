package models

import (
	"time"

	"github.com/google/uuid"
)

// ----------------------------------------
// User query model
// ----------------------------------------
type User struct {
	ID        uuid.UUID
	UserName  string
	Email     string
	Password  string
	CreatedAt time.Time
}

// -----------------------------------------------------------------
// DTO's (Data Transfer Objects) for the websocket protocol
// -----------------------------------------------------------------

// Payload expected from the frontend.
//
// The sender/device IDs are NOT included because
// those are already known from the authenticated
// websocket connection.
type IncomingMessage struct {
	Type             string    `json:"type"` // "message" | "typing" | "read_receipt" | "edit_message" | "delete_message"
	ConversationID   uuid.UUID `json:"conversation_id"`
	Payload          string    `json:"payload"`
	ReadMessageID    uuid.UUID `json:"read_message_id"`
	ReadMessageSeqNo int64     `json:"read_message_seq_no"`
	ClientMessageID  uuid.UUID `json:"client_message_id"` // For idempotency
}

// This message is written into the RouteMessage of WsConManager by the
// MessageService --> OutMssgHandler() method
type OutgoingMessage struct {
	MessageID      uuid.UUID `json:"message_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Sequence_no    int64     `json:"sequence_no"`

	SenderUserID uuid.UUID `json:"sender_user_id"`
	SenderName   string    `json:"sender_name"`

	TargetDeviceIDs []uuid.UUID `json:"target_device_ids"`

	Payload string `json:"payload"`

	Timestamp time.Time `json:"timestamp"`
}

// For later ---------------------------------------------------
type TypingEvent struct {
	ConversationID uuid.UUID `json:"conversation_id"`
}

type ReadReceipt struct {
	ConversationID uuid.UUID `json:"conversation_id"`
	MessageID      uuid.UUID `json:"message_id"`
}

type EditMessageRequest struct {
	MessageID uuid.UUID `json:"message_id"`
	Content   string    `json:"content"`
}

type DeleteMessageRequest struct {
	MessageID uuid.UUID `json:"message_id"`
}
