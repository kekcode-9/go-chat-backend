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
		ON CONFLICT (sender_id, client_message_id) DO NOTHING
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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("message exists")
		}
		return nil, err
	}

	return resp, nil
}

// -----------------------------------------------------------------------------
// Update read receipt related detaila for the participant in the conversation.
// -----------------------------------------------------------------------------

func (r *Repository) CreateReadReceipt(
	ctx context.Context,
	tx pgx.Tx,
	conversationID uuid.UUID,
	userID uuid.UUID,
	messageID uuid.UUID,
	sequenceNo int64,
) error {
	// find the participant record for the user in the conversation
	var participantID uuid.UUID

	err := tx.QueryRow(
		ctx,
		`
		SELECT id
		FROM conversation_participants
		WHERE
			conversation_id = $1
			AND user_id = $2
		`,
		conversationID,
		userID,
	).Scan(&participantID)

	if err != nil {
		return err
	}

	// update the participant record with the read receipt details

	// dont update if new sequenceNo is lower than the existing last_read_mssg_seq
	_, err = tx.Exec(
		ctx,
		`
		UPDATE conversation_participants
		SET last_read_message_id = $1,
			last_read_mssg_seq = $2
		WHERE id = $3
			AND COALESCE(last_read_mssg_seq, 0) < $2
		`,
		messageID,
		sequenceNo,
		participantID,
	)

	if err != nil {
		return err
	}

	return nil
}

// ------------------------------------------------------------------------------
// Get all messages for a conversation up to a limit
// ------------------------------------------------------------------------------

func (r *Repository) getLatestMessages(
	ctx context.Context,
	tx pgx.Tx,
	conversationID uuid.UUID,
	limit int,
) ([]Message, error) {
	rows, err := tx.Query(
		ctx,
		`
			SELECT *
			FROM (
				SELECT
					m.id,
					m.conversation_id,
					m.sequence_no,
					m.sender_id,
					m.content,
					m.created_at,
					u.user_name
				FROM messages m
				JOIN users u
					ON m.sender_id = u.id
				WHERE
					m.conversation_id = $1
				ORDER BY m.sequence_no DESC
				LIMIT $2
			) latest
			ORDER BY latest.sequence_no ASC
		`,
		conversationID,
		limit,
	)

	if err != nil {
		return nil, err
	}

	return scanMessages(rows)
}

func (r *Repository) getMessagesBefore(
	ctx context.Context,
	tx pgx.Tx,
	conversationID uuid.UUID,
	limit int,
	seqNo int64,
) ([]Message, error) {
	rows, err := tx.Query(
		ctx,
		`
			SELECT *
			FROM (
				SELECT
					m.id,
					m.conversation_id,
					m.sequence_no,
					m.sender_id,
					m.content,
					m.created_at,
					u.user_name
				FROM messages m
				JOIN users u
					ON m.sender_id = u.id
				WHERE
					m.conversation_id = $1
					AND m.sequence_no < $2
				ORDER BY m.sequence_no DESC
				LIMIT $3
			) latest
			ORDER BY latest.sequence_no ASC
		`,
		conversationID,
		seqNo,
		limit,
	)

	if err != nil {
		return nil, err
	}

	return scanMessages(rows)
}

func (r *Repository) getMessagesAfter(
	ctx context.Context,
	tx pgx.Tx,
	conversationID uuid.UUID,
	limit int,
	seqNo int64,
) ([]Message, error) {
	rows, err := tx.Query(
		ctx,
		`
		SELECT
				m.id,
				m.conversation_id,
				m.sequence_no,
				m.sender_id,
				m.content,
				m.created_at,
				u.user_name
			FROM messages m
			JOIN users u
				ON m.sender_id = u.id
			WHERE
				m.conversation_id = $1
				AND m.sequence_no > $2
			ORDER BY m.sequence_no ASC
			LIMIT $3
		`,
		conversationID,
		seqNo,
		limit,
	)

	if err != nil {
		return nil, err
	}

	return scanMessages(rows)
}

func scanMessages(rows pgx.Rows) ([]Message, error) {
	defer rows.Close()

	var messages []Message

	for rows.Next() {
		var message Message

		err := rows.Scan(
			&message.MessageID,
			&message.ConversationID,
			&message.Sequence_no,
			&message.SenderUserID,
			&message.Payload,
			&message.Timestamp,
			&message.SenderName,
		)
		if err != nil {
			return nil, err
		}

		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}
