package message

import (
	"context"
	"errors"
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

	userStore UserStore

	conversationStore ConversationStore

	redis *goredis.Client

	wsConManager *websocket.WsConManager
}

func NewMessageService(
	backendID string,
	db *pgxpool.Pool,
	redisClient *goredis.Client,
	wsConManager *websocket.WsConManager,
	userStore UserStore,
	conversationStore ConversationStore,
) *MessageService {
	repo := &Repository{
		db: db,
	}
	return &MessageService{
		BackendID:         backendID,
		repo:              repo,
		userStore:         userStore,
		conversationStore: conversationStore,
		redis:             redisClient,
		wsConManager:      wsConManager,
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
	clientMssgID uuid.UUID,
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
			ConversationID:  conversationID,
			SenderUserID:    senderUserID,
			Content:         payload,
			ClientMessageID: &clientMssgID,
		},
		sequenceNo,
	)

	if err != nil {
		if errors.Is(err, errors.New("message exists")) {
			// Message already exists, so we can safely ignore this duplicate.
			return err
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	// ------------------------------------------------------------------
	// Find conversation participants.
	// ------------------------------------------------------------------

	userIDs, err := m.conversationStore.FindConversationParticipants(
		ctx,
		conversationID,
	)

	if err != nil {
		return err
	}

	// ------------------------------------------------------------------
	// Find all active devices for those participants.
	// ------------------------------------------------------------------

	userDevices, err := m.userStore.FindDevicesForUsers(
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
	// Publish exactly one OutgoingMessage per backend.
	// ------------------------------------------------------------------

	for backendID, targetDevices := range backendTargets {

		chatMessage := models.OutgoingMessage{
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
	message models.OutgoingMessage,
) {
	m.wsConManager.RouteMessage <- message
}

func (m *MessageService) publishToBackend(
	ctx context.Context,
	backendID string,
	message models.OutgoingMessage,
) error {

	return m.Publish(
		ctx,
		"backend:"+backendID,
		message,
	)
}

// ---------------------------------------------
// Other logics
// ---------------------------------------------
func (m *MessageService) getMessages(
	conversationID uuid.UUID,
	limit int,
	seqNo *int64,
	getAfter bool,
) ([]Message, error) {
	/*
	* if seqNo doesnt exist, call m.repo.getLatestMessages
	* if seqNo exists and getAfter true call m.repo.getMessagesAfter
	* if seqNo exists and getAfter false call m.repo.getMessagesBefore
	 */
	ctx := context.Background()

	tx, err := m.repo.db.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer tx.Rollback(ctx)

	var messages []Message

	if seqNo == nil {
		messages, err = m.repo.getLatestMessages(
			ctx,
			tx,
			conversationID,
			limit,
		)
	} else if getAfter {
		messages, err = m.repo.getMessagesAfter(
			ctx,
			tx,
			conversationID,
			limit,
			*seqNo,
		)
	} else {
		messages, err = m.repo.getMessagesBefore(
			ctx,
			tx,
			conversationID,
			limit,
			*seqNo,
		)
	}

	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return messages, nil
}
