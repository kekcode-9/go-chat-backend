package message

import (
	"context"
	"time"

	"github.com/kekcode-9/go-chat-backend/internal/platform/models"
	"github.com/kekcode-9/go-chat-backend/internal/websocket"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

type MessageService struct {
	BackendID string

	repo *Repository

	redis *goredis.Client

	wsConManager *websocket.WsConManager
}

func NewMessageService(
	backendID string,
	db *pgxpool.Pool,
	redisClient *goredis.Client,
	wsConManager *websocket.WsConManager,
) *MessageService {
	repo := &Repository{
		db: db,
	}
	return &MessageService{
		BackendID:    backendID,
		repo:         repo,
		redis:        redisClient,
		wsConManager: wsConManager,
	}
}

func (m *MessageService) Run(ctx context.Context) {
	m.Subscriber(ctx)
}

// ---------------------------------------------
// Handle read receipts from receivers
// ---------------------------------------------

func (m *MessageService) ReadReceiptHandler(
	conversationID uuid.UUID,
	sequenceNo int64,
	mssgID uuid.UUID,
	senderUserID uuid.UUID,
) error {
	ctx := context.Background()

	// ------------------------------------------------------------------
	// Persist the read receipt inside a transaction.
	// ------------------------------------------------------------------

	tx, err := m.repo.db.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	if err := m.repo.CreateReadReceipt(
		ctx,
		tx,
		conversationID,
		senderUserID,
		mssgID,
		sequenceNo,
	); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
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

	ctx := context.Background()

	// ------------------------------------------------------------------
	// Persist the message inside a transaction.
	// ------------------------------------------------------------------

	tx, err := m.repo.db.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	sequenceNo, err := m.repo.AllocateSequenceNumber(
		ctx,
		tx,
		conversationID,
	)

	if err != nil {
		return err
	}

	createResp, err := m.repo.CreateMessage(
		ctx,
		tx,
		CreateMessageReq{
			ConversationID: conversationID,
			SenderUserID:   senderUserID,
			Content:        payload,
		},
		sequenceNo,
	)

	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	// ------------------------------------------------------------------
	// Find conversation participants.
	// ------------------------------------------------------------------

	userIDs, err := m.repo.FindConversationParticipants(
		ctx,
		conversationID,
	)

	if err != nil {
		return err
	}

	// ------------------------------------------------------------------
	// Find all active devices for those participants.
	// ------------------------------------------------------------------

	userDevices, err := m.repo.FindDevicesForUsers(
		ctx,
		userIDs,
	)

	if err != nil {
		return err
	}

	// ------------------------------------------------------------------
	// Group target devices by backend.
	// ------------------------------------------------------------------

	backendTargets := make(map[string][]uuid.UUID)

	for _, devices := range userDevices {

		for _, deviceID := range devices {

			// Don't echo back to the originating device.
			if deviceID == senderDeviceID {
				continue
			}

			backendID, err := m.redis.HGet(
				ctx,
				"DeviceConRegistry",
				deviceID.String(),
			).Result()

			if err != nil {
				// Device is currently offline.
				continue
			}

			backendTargets[backendID] = append(
				backendTargets[backendID],
				deviceID,
			)
		}
	}

	// ------------------------------------------------------------------
	// Publish exactly one ChatMessage per backend.
	// ------------------------------------------------------------------

	for backendID, targetDevices := range backendTargets {

		chatMessage := models.ChatMessage{
			MessageID: createResp.MessageID,

			ConversationID: createResp.ConversationID,

			Sequence_no: createResp.SequenceNo,

			SenderUserID: senderUserID,

			// TODO: Replace with actual sender name lookup.
			SenderName: "",

			TargetDeviceIDs: targetDevices,

			Payload: payload,

			Timestamp: timestamp,
		}

		if err := m.publishToBackend(
			ctx,
			backendID,
			chatMessage,
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
