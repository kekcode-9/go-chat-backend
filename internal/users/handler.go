package users

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

func (u *UserService) RegisterRoutes(
	mux *http.ServeMux,
) {

	mux.HandleFunc("GET /users/", u.userLookupHandler)

	mux.HandleFunc("POST /users/block/", u.blockUserHandler)
}

// userLookupHandler godoc
//
// @Summary Search users
// @Description Searches users by email or username.
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param email query string false "Email search term"
// @Param username query string false "Username search term"
// @Success 200 {object} UserLookupResponse
// @Failure 400 {string} string "missing query"
// @Failure 401 {string} string "missing or invalid access token"
// @Failure 404 {string} string "user not found"
// @Failure 500 {string} string "internal server error"
// @Router /users/ [get]
func (u *UserService) userLookupHandler(w http.ResponseWriter, r *http.Request) {
	/*
	* look for r.URL.Query.Get("email") or r.URL.Query.GET("username")
	* search users table using email / username
	* send back list of matching users
	 */

	email := r.URL.Query().Get("email")
	userName := r.URL.Query().Get("username")

	resp, err := u.LookupUser(
		email,
		userName,
	)

	if err != nil {
		switch {
		case errors.Is(err, ErrMissingQuery):
			http.Error(
				w,
				err.Error(),
				http.StatusBadRequest,
			)

		case errors.Is(err, ErrUserNotFound):
			http.Error(
				w,
				err.Error(),
				http.StatusNotFound,
			)

		default:
			log.Printf("user lookup failed: %v", err)

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
		log.Printf("failed to encode user lookup response: %v", err)
	}
}

func (u *UserService) blockUserHandler(w http.ResponseWriter, r *http.Request) {
	/*
	* Get user_id of the requesting user from jwt context
	* find user_id of the to-be-blocked user in the request body
	* Add new entry to blocked_users table
	 */
}
