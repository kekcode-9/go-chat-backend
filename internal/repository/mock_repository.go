package repository

import (
	"fmt"

	"github.com/google/uuid"
)

type MockRepository struct {
	// Hardcoded conversation participants.
	// conversation_id -> []user_id
	conversationParticipants map[uuid.UUID][]uuid.UUID

	// Hardcoded user devices.
	// user_id -> []device_id
	userDevices map[uuid.UUID][]uuid.UUID

	// Convenience IDs exposed for testing.
	AliceUserID uuid.UUID
	BobUserID   uuid.UUID

	AliceAndroid uuid.UUID

	BobMacbook uuid.UUID
	BobIPhone  uuid.UUID

	ConversationID uuid.UUID
}

func NewMockRepository() *MockRepository {
	aliceUser := uuid.New()
	bobUser := uuid.New()

	aliceAndroid := uuid.New()

	bobMacbook := uuid.New()
	bobIPhone := uuid.New()

	conversation := uuid.New()

	return &MockRepository{
		AliceUserID: aliceUser,
		BobUserID:   bobUser,

		AliceAndroid: aliceAndroid,

		BobMacbook: bobMacbook,
		BobIPhone:  bobIPhone,

		ConversationID: conversation,

		conversationParticipants: map[uuid.UUID][]uuid.UUID{
			conversation: {
				aliceUser,
				bobUser,
			},
		},

		userDevices: map[uuid.UUID][]uuid.UUID{
			aliceUser: {
				aliceAndroid,
			},

			bobUser: {
				bobMacbook,
				bobIPhone,
			},
		},
	}
}

// ----------------------------------------------------------------------
// Conversation queries
// ----------------------------------------------------------------------

func (r *MockRepository) FindConversationParticipants(
	conversationID uuid.UUID,
) ([]uuid.UUID, error) {

	users, ok := r.conversationParticipants[conversationID]

	if !ok {
		return nil, fmt.Errorf("conversation not found")
	}

	return users, nil
}

// ----------------------------------------------------------------------
// Device queries
// ----------------------------------------------------------------------

func (r *MockRepository) FindDevicesForUsers(
	userIDs []uuid.UUID,
) (map[uuid.UUID][]uuid.UUID, error) {

	result := make(map[uuid.UUID][]uuid.UUID)

	for _, userID := range userIDs {

		devices, ok := r.userDevices[userID]

		if !ok {
			continue
		}

		result[userID] = devices
	}

	return result, nil
}