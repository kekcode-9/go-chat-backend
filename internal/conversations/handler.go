package conversations

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/kekcode-9/go-chat-backend/internal/auth"
)

func (c *ConversationService) RegisterRoutes(
	mux *http.ServeMux,
) {
	// To fetch all conversations of the user
	mux.HandleFunc("GET /conversations", c.getConversationsHandler)

	mux.HandleFunc("POST /conversations", c.postConversationHandler)

	mux.HandleFunc("POST /conversations/participant", c.postParticipantHandler)

	// To fetch all messages of a conversation
	mux.HandleFunc("GET /conversations/{id}/messages", c.getMessagesHandler)

	// request to remove one participant from a group type conversation
	mux.HandleFunc("DELETE /conversations/participant", c.removeGroupParticipantHandler)

	// request to delete entire conversation
	mux.HandleFunc("DELETE /conversations", c.deleteConversationHandler)

	mux.HandleFunc("DELETE /conversations/leave", c.leaveConversationHandler)
}

// getConversationsHandler godoc
//
// @Summary Get all conversations of a user
// @Description Returns all conversations that the authenticated user participates in.
// @Tags Conversations
// @Accept json
// @Produce json
// @Success 200 {array} ConversationResponse
// @Failure 401 {object} ErrorResponse
// @Router /conversations [get]
func (c *ConversationService) getConversationsHandler(w http.ResponseWriter, r *http.Request) {
	/*
	* To fetch all conversations of the user
	* Get the user_id from the jwt context
	* Find all conversations where this user is a participant
	* Send back a list like:
	* [{ conversation_id: uuid, participants: [ {participant_id: uuid, participant_name: string } ] }]
	 */
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(
			w,
			"missing auth claims",
			http.StatusUnauthorized,
		)
		return
	}

	req := GetConversationsRequest{
		UserID: claims.UserID,
	}

	resp, err := c.getUserConversations(req)
	if err != nil {
		switch {
		case errors.Is(err, ErrFailedToFetchConversations):
			http.Error(
				w,
				err.Error(),
				http.StatusInternalServerError,
			)
		default:
			http.Error(
				w,
				"internal server error",
				http.StatusInternalServerError,
			)
		}
		return
	}

	w.Header().Set(
		"Content-Type",
		"application/json",
	)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf(
			"failed to encode get conversations response: %v",
			err,
		)
	}
}

// For starting direct conversation with another usre
// this call will be made when one user clicks of the start conversation button beside another user's name
// A conversation can be started first and after that first message is sent
func (c *ConversationService) postConversationHandler(w http.ResponseWriter, r *http.Request) {
	/*
	* Get user_id of requesting user from jwt context
	* Get user_id of the other user from request body
	* Get conversation type from the request body which is optional. If not present the default type is "direct"
	* create entry in conversations table and an entry in conversation_participants
	* table for each of the users
	* In conversation_participants the last_read_message_id would be initially Null
	* and the last_read_message_seq will be initially 0
	 */
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(
			w,
			"missing auth claims",
			http.StatusUnauthorized,
		)
		return
	}

	var req CreateConversationRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(
			w,
			"invalid request body",
			http.StatusBadRequest,
		)
		return
	}

	req.RequestingUserID = claims.UserID

	resp, err := c.createConversation(req)
	if err != nil {
		switch {
		case errors.Is(err, ErrCannotStartConversationWithSelf):
			http.Error(
				w,
				err.Error(),
				http.StatusBadRequest,
			)

		case errors.Is(err, ErrInvalidConversationType):
			http.Error(
				w,
				err.Error(),
				http.StatusBadRequest,
			)

		default:
			log.Printf("create conversation failed: %v", err)

			http.Error(
				w,
				"internal server error",
				http.StatusInternalServerError,
			)
		}

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("failed to encode create conversation response: %v", err)
	}
}

// adding a participant to a group
func (c *ConversationService) postParticipantHandler(w http.ResponseWriter, r *http.Request) {
	/*
	* Get user_id of requesting user from jwt context
	* Get user_id of the other user from request body
	* Get the conversation_id from the request body
	* Change the type of the conversation in conversations table to "group"
	* Add new participant in conversation_participants table with last_read_message_id Null
	* and last_read_mssg_seq 0
	 */
}

func (c *ConversationService) getMessagesHandler(w http.ResponseWriter, r *http.Request) {
	/*
	* if query params have a from_seq value then fetch from that message sequence
	* else fetch all messages for the conversation
	 */
}

func (c *ConversationService) removeGroupParticipantHandler(w http.ResponseWriter, r *http.Request) {
	// For a group conversation only the admin can do this
}

func (c *ConversationService) deleteConversationHandler(w http.ResponseWriter, r *http.Request) {
	// For a group conversation only admin can do this
}

func (c *ConversationService) leaveConversationHandler(w http.ResponseWriter, r *http.Request) {
	/*
	* Only valid for group conversation
	* Find user_id in jwt context
	* remove conversation_participant entry for this user
	 */
}
