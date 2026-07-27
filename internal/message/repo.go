package message

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{
		db: db,
	}
}

// -----------------------------------------------------------------------------
// Atomically allocate the next message sequence number.
//
// Must be called inside an existing transaction.
// -----------------------------------------------------------------------------

func (r *Repository) AllocateSequenceNumber(
	ctx context.Context,
	tx pgx.Tx,
	conversationID uuid.UUID,
) (int64, error) {

	var sequenceNo int64

	err := tx.QueryRow(
		ctx,
		`
		UPDATE conversations
		SET next_sequence_no = next_sequence_no + 1
		WHERE id = $1
		RETURNING next_sequence_no - 1
		`,
		conversationID,
	).Scan(&sequenceNo)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrConversationNotFound
		}

		return 0, err
	}

	return sequenceNo, nil
}

// -----------------------------------------------------------------------------
// Persist the message.
//
// Must be called inside an existing transaction.
// -----------------------------------------------------------------------------

func (r *Repository) CreateMessage(
	ctx context.Context,
	tx pgx.Tx,
	req CreateMessageReq,
	sequenceNo int64,
) (*CreateMessageResp, error) {

	resp := &CreateMessageResp{}

	err := tx.QueryRow(
		ctx,
		`
		INSERT INTO messages (
			sender_id,
			conversation_id,
			content,
			reply_to_message_id,
			client_message_id,
			sequence_no
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING
			id,
			conversation_id,
			sequence_no,
			created_at
		`,
		req.SenderUserID,
		req.ConversationID,
		req.Content,
		req.ReplyToMessageID,
		req.ClientMessageID,
		sequenceNo,
	).Scan(
		&resp.MessageID,
		&resp.ConversationID,
		&resp.SequenceNo,
		&resp.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// -----------------------------------------------------------------------------
// Returns all ACTIVE participants of a conversation.
// -----------------------------------------------------------------------------

func (r *Repository) FindConversationParticipants(
	ctx context.Context,
	conversationID uuid.UUID,
) ([]uuid.UUID, error) {

	rows, err := r.db.Query(
		ctx,
		`
		SELECT user_id
		FROM conversation_participants
		WHERE
			conversation_id = $1
			AND status = 'active'
		`,
		conversationID,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var userIDs []uuid.UUID

	for rows.Next() {

		var userID uuid.UUID

		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}

		userIDs = append(userIDs, userID)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return userIDs, nil
}

// -----------------------------------------------------------------------------
// Returns all non-revoked devices grouped by user.
//
// userID -> []deviceID
// -----------------------------------------------------------------------------

func (r *Repository) FindDevicesForUsers(
	ctx context.Context,
	userIDs []uuid.UUID,
) (map[uuid.UUID][]uuid.UUID, error) {

	if len(userIDs) == 0 {
		return map[uuid.UUID][]uuid.UUID{}, nil
	}

	rows, err := r.db.Query(
		ctx,
		`
		SELECT
			user_id,
			id
		FROM devices
		WHERE
			user_id = ANY($1)
			AND revoked_at IS NULL
		`,
		userIDs,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	result := make(map[uuid.UUID][]uuid.UUID)

	for rows.Next() {

		var (
			userID   uuid.UUID
			deviceID uuid.UUID
		)

		if err := rows.Scan(
			&userID,
			&deviceID,
		); err != nil {
			return nil, err
		}

		result[userID] = append(
			result[userID],
			deviceID,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
