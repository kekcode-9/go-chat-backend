package message

import (
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------
// GET all messages for a conversation
// ---------------------------------------
type Message struct {
	MessageID      uuid.UUID `json:"message_id"`
	ConversationID uuid.UUID `json:"conversation_id"`
	Sequence_no    int64     `json:"sequence_no"`

	SenderUserID uuid.UUID `json:"sender_user_id"`
	SenderName   string    `json:"sender_name"`

	Payload string `json:"payload"`

	Timestamp time.Time `json:"timestamp"`
}

type GetConversationMessagesResponse struct {
	ConversationID uuid.UUID `json:"conversation_id"`
	Limit          uuid.UUID `json:"limit"`
	Messages       []Message `json:"messages"`
}

type PostReadReceiptRequest struct {
	MessageID  uuid.UUID `json:"message_id"`
	SequenceNo int64     `json:"sequence_no"`
}

// -----------------------------------------------------------------------------
// Internal request to create a new message.
// -----------------------------------------------------------------------------

type CreateMessageReq struct {
	ConversationID   uuid.UUID
	SenderUserID     uuid.UUID
	ClientMessageID  *uuid.UUID
	ReplyToMessageID *uuid.UUID
	Content          string
}

// -----------------------------------------------------------------------------
// Returned after successfully creating the message.
// -----------------------------------------------------------------------------

type CreateMessageResp struct {
	MessageID      uuid.UUID
	ConversationID uuid.UUID
	SequenceNo     int64
	CreatedAt      time.Time
}

// -----------------------------------------------------------------------------
// Result returned after allocating the next sequence number.
// -----------------------------------------------------------------------------

type AllocateSequenceResp struct {
	SequenceNo int64
}

// -----------------------------------------------------------------------------
// Internal representation of a newly persisted message.
// Used by the service before publishing to Redis.
// -----------------------------------------------------------------------------

type PersistedMessage struct {
	MessageID        uuid.UUID
	ConversationID   uuid.UUID
	SequenceNo       int64
	SenderUserID     uuid.UUID
	Content          string
	CreatedAt        time.Time
	ClientMessageID  *uuid.UUID
	ReplyToMessageID *uuid.UUID
}
