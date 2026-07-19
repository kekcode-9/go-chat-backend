package message

import (
	"context"
	"time"

	"github.com/kekcode-9/go-chat-backend/internal/models"
	"github.com/kekcode-9/go-chat-backend/internal/repository"
	"github.com/kekcode-9/go-chat-backend/internal/websocket"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

type MessageService struct{
	BackendID string

	repo *repository.MockRepository

	redis *goredis.Client

	wsConManager *websocket.WsConManager
}

func NewMessageService(
	backendID string,
	repo *repository.MockRepository,
	redisClient *goredis.Client,
	wsConManager *websocket.WsConManager,
) *MessageService {
	return &MessageService{
		BackendID: backendID,
		repo: repo,
		redis: redisClient,
		wsConManager: wsConManager,
	}
}

// ---------------------------------------------
// Handle incoming message from senders
// ---------------------------------------------

func (m *MessageService) InMssgHandler(
	payload string,
	timestamp time.Time,
	senderUserID uuid.UUID,
	senderDeviceID uuid.UUID,
	conversationID uuid.UUID,
) error {
	// ------------------------------------------------------------------
	// Placeholder.
	//
	// Eventually this method will:
	//
	// 1. Allocate sequence number.
	// 2. Insert message.
	// 3. Find conversation participants.
	// 4. Find participant devices.
	// 5. Ask Redis which backend owns each device.
	// 6. Group devices by backend.
	// 7. Publish one ChatMessage per backend.
	// ------------------------------------------------------------------

	ctx := context.Background()

	// ------------------------------------------------------------------
	// Find conversation participants
	// ------------------------------------------------------------------

	userIDs, err := m.repo.FindConversationParticipants(conversationID)

	if err != nil {
		return err
	}

	// ------------------------------------------------------------------
	// Find all participant devices
	// ------------------------------------------------------------------

	userDevices, err := m.repo.FindDevicesForUsers(userIDs)

	if err != nil {
		return err
	}

	// --------------------------------------------------
	// Add sender's other devices.
	// (In this mock repository they are already present
	// because the sender is a conversation participant.
	// Keeping this comment because later the DB version
	// will explicitly exclude senderDeviceID.)
	// --------------------------------------------------

	// backendID -> []deviceID
	// For grouping devices by the backends they are connected to
	backendTargets := make(map[string][]uuid.UUID)

	for _, devices := range userDevices {

		for _, deviceID := range devices {
			// Skip the device that originated the message.
			// The frontend already rendered it.
			if deviceID == senderDeviceID {
				continue
			}

			backendID, err := m.redis.HGet(
				ctx,
				"DeviceConRegistry",
				deviceID.String(),
			).Result()

			if err != nil {
				continue
			}

			backendTargets[backendID] = append(
				backendTargets[backendID],
				deviceID,
			)
		}
	}

	// --------------------------------------------------
	// Publish one message per backend
	// --------------------------------------------------

	for backendID, targetDevices := range backendTargets {
		mssg := models.ChatMessage {
			MessageID: uuid.New(),

			ConversationID: conversationID,

			Sequence_no: 1, // mocked

			SenderUserID: senderUserID,

			SenderName: "Mock User",

			TargetDeviceIDs: targetDevices,

			Payload: payload,

			Timestamp: timestamp,
		}

		if err := m.publishToBackend(
			ctx,
			backendID,
			mssg,
		); err != nil {
			return err
		}
	}

	return nil
}

// --------------------------------------------
// Handle outbound messages to receivers
// --------------------------------------------

func (m *MessageService) OutMssgHandler(
	message models.ChatMessage,
) {
	m.wsConManager.RouteMessage <- message
}

func (m *MessageService) publishToBackend(
	ctx context.Context,
	backendID string,
	message models.ChatMessage,
) error {

	return m.Publish(
		ctx,
		"backend:"+backendID,
		message,
	)
}