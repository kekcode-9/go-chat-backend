package server

import "net/http"

func registerConversationRoutes(
	mux *http.ServeMux,
) {
	// To fetch all conversations of the user
	mux.HandleFunc("GET /conversations/", func(w http.ResponseWriter, r *http.Request) {
		/*
		* Get the user_id from the jwt context
		* JOIN conversation_participants and conversations tables on
		* conversations.id = conversation_participants.conversation_id
		* WHERE user_id = conversation_participants.user_id
		* Send back the list
		 */
	})

	mux.HandleFunc("POST /conversations/", func(w http.ResponseWriter, r *http.Request) {
		/*
		* Get user_id of requesting user from jwt context
		* Get user_id of the other user from request body
		* create entry in conversations table and an entry in conversation_participants
		* table for each of the users
		* In conversation_participants the last_read_message_id would be initially Null
		* and the last_read_message_seq will be initially 0
		 */
	})

	mux.HandleFunc("POST /conversations/participant/", func(w http.ResponseWriter, r *http.Request) {
		/*
		* Get user_id of requesting user from jwt context
		* Get user_id of the other user from request body
		* Get the conversation_id from the request body
		* Change the type of the conversation in conversations table to "group"
		* Add new participant in conversation_participants table with last_read_message_id Null
		* and last_read_mssg_seq 0
		 */
	})

	// To fetch all messages of a conversation
	mux.HandleFunc("GET /conversations/{id}/messages", func(w http.ResponseWriter, r *http.Request) {
		/*
		* if query params have a from_seq value then fetch from that message sequence
		* else fetch all messages for the conversation
		 */
	})

	// request to remove one participant from a group type conversation
	mux.HandleFunc("DELETE /conversations/participant/", func(w http.ResponseWriter, r *http.Request) {
		// For a group conversation only the admin can do this
	})

	// request to delete entire conversation
	mux.HandleFunc("DELETE /conversations/", func(w http.ResponseWriter, r *http.Request) {
		// For a group conversation only admin can do this
	})

	mux.HandleFunc("DELETE /conversations/leave/", func(w http.ResponseWriter, r *http.Request) {
		/*
		* Only valid for group conversation
		* Find user_id in jwt context
		* remove conversation_participant entry for this user
		 */
	})
}
