package conversations

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type ConversationService struct {
	Repo        *Repository
	redisClient *redis.Client
}

func NewConversationService(
	db *pgxpool.Pool,
	redisClient *redis.Client,
) *ConversationService {
	repo := NewRepository(db)

	return &ConversationService{
		Repo:        repo,
		redisClient: redisClient,
	}
}

func (c *ConversationService) getUserConversations(
	req GetConversationsRequest,
) (*GetConversationsResponse, error) {
	ctx := context.Background()

	rows, err := c.Repo.GetAllUserConversations(
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

	tx, err := c.Repo.db.Begin(ctx)
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
		existingConversationID, err := c.Repo.FindDirectConversationBetweenUsers(
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

	conversationID, err := c.Repo.CreateNewConversation(
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
	err = c.Repo.CreateNewParticipant(
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
	err = c.Repo.CreateNewParticipant(
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

func (c *ConversationService) getParticipants(
	conversationID uuid.UUID,
	callerUserID uuid.UUID,
) (*ConversationParticipants, error) {
	ctx := context.Background()

	verifiedUser, err := c.Repo.VerifyConversationParticipant(
		ctx,
		callerUserID,
		conversationID,
	)

	if err != nil {
		if errors.Is(err, ErrUserNotActiveParticipant) {
			return nil, ErrUserNotActiveParticipant
		}
		return nil, err
	}

	if verifiedUser == uuid.Nil {
		return nil, ErrUserNotActiveParticipant
	}

	participants, err := c.Repo.FindConversationParticipants(
		ctx,
		conversationID,
	)

	if err != nil {
		return nil, err
	}

	var resParticipants []ParticipantWithPresence

	for _, participant := range participants {
		if participant.UserID == callerUserID {
			continue
		}

		_, err := c.redisClient.Get(
			ctx,
			"presence:user:"+participant.UserID.String(),
		).Result()

		isOnline := true

		if err != nil {
			if errors.Is(err, redis.Nil) {
				isOnline = false
			} else {
				// Some real Redis/client error happened
				return nil, err
			}
		}

		resParticipants = append(resParticipants, ParticipantWithPresence{
			ParticipantID:   participant.UserID,
			ParticipantName: participant.UserName,
			IsOnline:        isOnline,
		})
	}

	return &ConversationParticipants{
		Participants: resParticipants,
	}, nil
}
