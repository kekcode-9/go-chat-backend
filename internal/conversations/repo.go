package conversations

import (
	"context"
	"errors"

	"github.com/kekcode-9/go-chat-backend/internal/platform/models"

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

// GetAllUserConversations returns one row per participant.
// The service layer is responsible for grouping the rows by conversation.
func (r *Repository) GetAllUserConversations(
	ctx context.Context,
	userID uuid.UUID,
) ([]ConversationParticipantRow, error) {
	rows, err := r.db.Query(
		ctx,
		`
		SELECT
			c.id,
			u.id,
			u.user_name
		FROM conversations c
		INNER JOIN conversation_participants cp_filter
			ON cp_filter.conversation_id = c.id
		INNER JOIN conversation_participants cp
			ON cp.conversation_id = c.id
		INNER JOIN users u
			ON u.id = cp.user_id
		WHERE
			cp_filter.user_id = $1
			AND cp_filter.status = 'active'
		ORDER BY
			c.created_at DESC,
			u.user_name ASC
		`,
		userID,
	)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var result []ConversationParticipantRow

	for rows.Next() {
		var row ConversationParticipantRow

		err := rows.Scan(
			&row.ConversationID,
			&row.ParticipantID,
			&row.ParticipantName,
		)
		if err != nil {
			return nil, err
		}

		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// CreateNewConversation inserts a new conversation and returns its ID.
func (r *Repository) CreateNewConversation(
	ctx context.Context,
	tx pgx.Tx,
	req NewConversation,
) (uuid.UUID, error) {
	var conversationID uuid.UUID

	err := tx.QueryRow(
		ctx,
		`
		INSERT INTO conversations (
			type,
			created_by
		)
		VALUES ($1, $2)
		RETURNING id
		`,
		req.Type,
		req.CreatedBy,
	).Scan(&conversationID)

	if err != nil {
		return uuid.Nil, err
	}

	return conversationID, nil
}

// CreateNewParticipant inserts a participant into a conversation.
// Database defaults populate status, last_read_message_id,
// last_read_mssg_seq, muted, archived and pinned flags.
func (r *Repository) CreateNewParticipant(
	ctx context.Context,
	tx pgx.Tx,
	req NewParticipant,
) error {
	_, err := tx.Exec(
		ctx,
		`
		INSERT INTO conversation_participants (
			conversation_id,
			user_id,
			role
		)
		VALUES ($1, $2, $3)
		`,
		req.ConversationID,
		req.UserID,
		req.Role,
	)

	if err != nil {
		return err
	}

	return nil
}

func (r *Repository) FindDirectConversationBetweenUsers(
	ctx context.Context,
	userID1 uuid.UUID,
	userID2 uuid.UUID,
) (uuid.UUID, error) {
	rows, err := r.db.Query(
		ctx,
		`
		SELECT c.id
		FROM conversations c
		JOIN conversation_participants cp1
			ON cp1.conversation_id = c.id
		JOIN conversation_participants cp2
			ON cp2.conversation_id = c.id
		WHERE
			c.type = 'direct'
			AND cp1.user_id = $1
			AND cp1.status = 'active'
			AND cp2.user_id = $2
			AND cp2.status = 'active'
		LIMIT 1;
		`,
		userID1,
		userID2,
	)

	if err != nil {
		return uuid.Nil, err
	}

	defer rows.Close()

	if rows.Next() {
		var conversationID uuid.UUID
		if err := rows.Scan(&conversationID); err != nil {
			return uuid.Nil, err
		}
		return conversationID, nil
	}

	return uuid.Nil, nil // No direct conversation found
}

func (r *Repository) VerifyConversationParticipant(
	ctx context.Context,
	userID uuid.UUID,
	conversationID uuid.UUID,
) (uuid.UUID, error) {
	var participantID uuid.UUID

	err := r.db.QueryRow(
		ctx,
		`
			SELECT user_id
			FROM conversation_participants
			WHERE 
				user_id = $1
				AND conversation_id = $2
				AND status = 'active'
		`,
		userID,
		conversationID,
	).Scan(&participantID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrUserNotActiveParticipant
		}
		return uuid.Nil, err
	}

	return participantID, nil
}

// -----------------------------------------------------------------------------
// Returns all ACTIVE participants of a conversation.
// -----------------------------------------------------------------------------

func (r *Repository) FindConversationParticipants(
	ctx context.Context,
	conversationID uuid.UUID,
) ([]models.ConversationParticipant, error) {

	rows, err := r.db.Query(
		ctx,
		`
		SELECT
			cp.user_id,
			u.user_name
		FROM conversation_participants cp
		JOIN users u
			ON u.id = cp.user_id
		WHERE
			cp.conversation_id = $1
			AND cp.status = 'active'
		`,
		conversationID,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var participants []models.ConversationParticipant

	for rows.Next() {

		var participant models.ConversationParticipant

		if err := rows.Scan(
			&participant.UserID,
			&participant.UserName,
		); err != nil {
			return nil, err
		}

		participants = append(participants, participant)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return participants, nil
}
