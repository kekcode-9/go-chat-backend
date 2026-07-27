package message

import (
	"time"

	"github.com/google/uuid"
)

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
