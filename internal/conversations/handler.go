package conversations

import (
	"net/http"

	"github.com/kekcode-9/go-chat-backend/internal/auth"
)

func (c *ConversationService) RegisterRoutes(
	mux *http.ServeMux,
) {
	// To fetch all conversations of the user
	mux.Handle("GET /conversations", auth.AuthMiddleware(http.HandlerFunc(c.getConversationsHandler)))

	mux.Handle("POST /conversations", auth.AuthMiddleware(http.HandlerFunc(c.postConversationHandler)))

	mux.Handle("POST /conversations/participant", auth.AuthMiddleware(http.HandlerFunc(c.postParticipantHandler)))

	// To fetch all messages of a conversation
	mux.Handle("GET /conversations/{id}/messages", auth.AuthMiddleware(http.HandlerFunc(c.getMessagesHandler)))

	// request to remove one participant from a group type conversation
	mux.Handle("DELETE /conversations/participant", auth.AuthMiddleware(http.HandlerFunc(c.removeGroupParticipantHandler)))

	// request to delete entire conversation
	mux.Handle("DELETE /conversations", auth.AuthMiddleware(http.HandlerFunc(c.deleteConversationHandler)))

	mux.Handle("DELETE /conversations/leave", auth.AuthMiddleware(http.HandlerFunc(c.leaveConversationHandler)))
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
	* Get the user_id from the jwt context
	* JOIN conversation_participants and conversations tables on
	* conversations.id = conversation_participants.conversation_id
	* WHERE user_id = conversation_participants.user_id
	* Send back the list
	 */
}

func (c *ConversationService) postConversationHandler(w http.ResponseWriter, r *http.Request) {
	/*
	* Get user_id of requesting user from jwt context
	* Get user_id of the other user from request body
	* create entry in conversations table and an entry in conversation_participants
	* table for each of the users
	* In conversation_participants the last_read_message_id would be initially Null
	* and the last_read_message_seq will be initially 0
	 */
}

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
