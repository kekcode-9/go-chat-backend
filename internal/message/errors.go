package message

import "errors"

var (
	// Conversation does not exist.
	ErrConversationNotFound = errors.New(
		"conversation not found",
	)

	// Sender is not an active participant.
	ErrNotConversationParticipant = errors.New(
		"user is not an active participant of the conversation",
	)

	// Empty message body.
	ErrEmptyMessage = errors.New(
		"message content cannot be empty",
	)

	// Duplicate client-generated UUID.
	ErrDuplicateClientMessage = errors.New(
		"duplicate client message",
	)

	// Failed to allocate sequence number.
	ErrSequenceAllocationFailed = errors.New(
		"failed to allocate message sequence number",
	)

	// Failed to persist message.
	ErrCreateMessageFailed = errors.New(
		"failed to create message",
	)
)
