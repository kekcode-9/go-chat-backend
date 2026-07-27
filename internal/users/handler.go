package users

import (
	"net/http"

	"github.com/kekcode-9/go-chat-backend/internal/auth"
)

func (u *UserService) RegisterRoutes(
	mux *http.ServeMux,
) {

	// for searching users to chat with
	mux.Handle("GET /users/", auth.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		/*
		* look for r.URL.Query.Get("email") or r.URL.Query.GET("username")
		* search users table using email / username
		* send back list of matching users
		 */
	})))

	mux.Handle("POST /users/block/", auth.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		/*
		* Get user_id of the requesting user from jwt context
		* find user_id of the to-be-blocked user in the request body
		* Add new entry to blocked_users table
		 */
	})))
}
