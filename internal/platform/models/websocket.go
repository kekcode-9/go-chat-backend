package models

import (
	"github.com/google/uuid"
)

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
