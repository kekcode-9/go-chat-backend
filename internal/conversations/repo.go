package conversations

import (
	"context"

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

/*
CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    type TEXT NOT NULL DEFAULT 'direct' CHECK (type IN ('direct', 'group')),
    title TEXT,
    description TEXT,
    avatar_url TEXT,
    created_by UUID NOT NULL REFERENCES users(id),
    next_sequence_no BIGINT NOT NULL DEFAULT 1
);
*/

/*
CREATE TABLE conversation_participants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('member', 'admin')),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'left', 'removed')),
    conversation_id UUID NOT NULL REFERENCES conversations(id),
    user_id UUID NOT NULL REFERENCES users(id),
    left_at TIMESTAMPTZ,
    removed_at TIMESTAMPTZ,
    removed_by UUID REFERENCES users(id),
    is_muted BOOLEAN NOT NULL DEFAULT FALSE,
    is_archived BOOLEAN NOT NULL DEFAULT FALSE,
    is_pinned BOOLEAN NOT NULL DEFAULT FALSE,
    last_read_message_id UUID REFERENCES messages(id),
    last_read_mssg_seq BIGINT,
    UNIQUE (conversation_id, user_id)
);

CREATE INDEX idx_conv_participants_user ON conversation_participants (user_id);
*/

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
