package conversations

import (
	"github.com/google/uuid"
)

// ---------------------------------------
// Get Conversations
// ---------------------------------------

type GetConversationsRequest struct {
	UserID uuid.UUID
}

type ConversationParticipant struct {
	ParticipantID   uuid.UUID `json:"participant_id"`
	ParticipantName string    `json:"participant_name"`
}

type ConversationSummary struct {
	ConversationID uuid.UUID                 `json:"conversation_id"`
	Participants   []ConversationParticipant `json:"participants"`
}

type GetConversationsResponse struct {
	Conversations []ConversationSummary `json:"conversations"`
}

// ---------------------------------------
// Create Conversation
// ---------------------------------------

type CreateConversationRequest struct {
	RequestingUserID uuid.UUID
	OtherUserID      uuid.UUID `json:"other_user_id"`
	Type             string    `json:"type"`
}

type CreateConversationResponse struct {
	ConversationID uuid.UUID `json:"conversation_id"`
}

// ---------------------------------------
// Get conversation participants
// ---------------------------------------

type ParticipantWithPresence struct {
	ParticipantID   uuid.UUID `json:"participant_id"`
	ParticipantName string    `json:"participant_name"`
	IsOnline        bool      `json:"is_online"`
}

type ConversationParticipants struct {
	Participants []ParticipantWithPresence `json:"participants"`
}

// ---------------------------------------
// Repository models
// ---------------------------------------

// Represents a single row returned by the SQL JOIN.
// Multiple rows may belong to the same conversation.
type ConversationParticipantRow struct {
	ConversationID  uuid.UUID
	ParticipantID   uuid.UUID
	ParticipantName string
}

type NewConversation struct {
	Type      string
	CreatedBy uuid.UUID
}

type NewParticipant struct {
	ConversationID uuid.UUID
	UserID         uuid.UUID
	Role           string
}
