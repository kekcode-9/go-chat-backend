package conversations

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConversationService struct {
	repo *Repository
	Repo *Repository
}

func NewConversationService(
	db *pgxpool.Pool,
) *ConversationService {
	repo := NewRepository(db)

	return &ConversationService{
		repo: repo,
		Repo: repo,
	}
}

func (c *ConversationService) getUserConversations(
	req GetConversationsRequest,
) (*GetConversationsResponse, error) {
	ctx := context.Background()

	rows, err := c.repo.GetAllUserConversations(
		ctx,
		req.UserID,
	)

	if err != nil {
		return nil, ErrFailedToFetchConversations
	}

	// Group rows by conversation.
	conversationMap := make(map[uuid.UUID]*ConversationSummary)

	for _, row := range rows {
		conversation, exists := conversationMap[row.ConversationID]

		if !exists {
			conversation = &ConversationSummary{
				ConversationID: row.ConversationID,
				Participants:   make([]ConversationParticipant, 0),
			}

			conversationMap[row.ConversationID] = conversation
		}

		conversation.Participants = append(
			conversation.Participants,
			ConversationParticipant{
				ParticipantID:   row.ParticipantID,
				ParticipantName: row.ParticipantName,
			},
		)
	}

	response := &GetConversationsResponse{
		Conversations: make([]ConversationSummary, 0, len(conversationMap)),
	}

	for _, conversation := range conversationMap {
		response.Conversations = append(
			response.Conversations,
			*conversation,
		)
	}

	return response, nil
}

func (c *ConversationService) createConversation(
	req CreateConversationRequest,
) (*CreateConversationResponse, error) {
	ctx := context.Background()

	tx, err := c.repo.db.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer tx.Rollback(ctx)

	// Prevent users from creating a conversation with themselves.
	if req.RequestingUserID == req.OtherUserID {
		return nil, ErrCannotStartConversationWithSelf
	}

	// Default to direct conversation.
	conversationType := req.Type
	if conversationType == "" {
		conversationType = "direct"
	}

	// Only direct and group conversations are supported.
	if conversationType != "direct" && conversationType != "group" {
		return nil, ErrInvalidConversationType
	}

	if conversationType == "direct" {
		// Check if a direct conversation already exists between the two users.
		existingConversationID, err := c.repo.FindDirectConversationBetweenUsers(
			ctx,
			req.RequestingUserID,
			req.OtherUserID,
		)

		if err != nil {
			return nil, err
		}

		if existingConversationID != uuid.Nil {
			return nil, ErrDirectConversationAlreadyExists
		}
	}

	conversationID, err := c.repo.CreateNewConversation(
		ctx,
		tx,
		NewConversation{
			Type:      conversationType,
			CreatedBy: req.RequestingUserID,
		},
	)

	if err != nil {
		return nil, err
	}

	// Creator becomes admin.
	err = c.repo.CreateNewParticipant(
		ctx,
		tx,
		NewParticipant{
			ConversationID: conversationID,
			UserID:         req.RequestingUserID,
			Role:           "admin",
		},
	)

	if err != nil {
		return nil, err
	}

	// Other user joins as member.
	err = c.repo.CreateNewParticipant(
		ctx,
		tx,
		NewParticipant{
			ConversationID: conversationID,
			UserID:         req.OtherUserID,
			Role:           "member",
		},
	)

	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &CreateConversationResponse{
		ConversationID: conversationID,
	}, nil
}
