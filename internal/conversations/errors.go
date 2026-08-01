package conversations

import "errors"

var (
	ErrFailedToFetchConversations = errors.New(
		"failed to fetch conversations",
	)

	ErrUnauthorized = errors.New(
		"unauthorized",
	)

	ErrConversationCreationFailed = errors.New(
		"failed to create conversation",
	)

	ErrParticipantCreationFailed = errors.New(
		"failed to add conversation participant",
	)

	ErrCannotStartConversationWithSelf = errors.New(
		"cannot start conversation with yourself",
	)

	ErrInvalidConversationType = errors.New(
		"invalid conversation type",
	)

	ErrDirectConversationAlreadyExists = errors.New(
		"direct conversation already exists between the two users",
	)
)
